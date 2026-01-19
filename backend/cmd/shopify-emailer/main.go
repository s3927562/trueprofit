package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/db"
	"backend/internal/shopify"
	"backend/internal/users"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type EBEvent struct {
	DetailType string         `json:"detail-type"`
	Source     string         `json:"source"`
	Time       string         `json:"time"`
	Detail     map[string]any `json:"detail"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) (any, error) {
	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return nil, err
	}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	snsClient := sns.NewFromConfig(awsCfg)

	sent := 0
	skipped := 0

	for _, rec := range sqsEvent.Records {
		var ev EBEvent
		if err := json.Unmarshal([]byte(rec.Body), &ev); err != nil {
			skipped++
			continue
		}

		meta := asMap(pickAny(ev.Detail, "metadata"))
		topic := pickString(meta, "X-Shopify-Topic")
		shopDomain := pickString(meta, "X-Shopify-Shop-Domain")
		webhookID := pickString(meta, "X-Shopify-Webhook-Id")

		if topic == "" || shopDomain == "" {
			skipped++
			continue
		}

		// shop -> users
		subs, err := shopify.UsersForShop(ctx, ddb, shopDomain)
		if err != nil || len(subs) == 0 {
			skipped++
			continue
		}

		subject, message := buildMessage(topic, shopDomain, webhookID, ev.Detail)

		for _, sub := range subs {
			userTopicArn, err := users.GetAlertsTopicArn(ctx, ddb, sub)
			if err != nil || strings.TrimSpace(userTopicArn) == "" {
				// user hasn't enabled/confirmed alerts
				continue
			}

			_, err = snsClient.Publish(ctx, &sns.PublishInput{
				TopicArn: aws.String(userTopicArn),
				Subject:  aws.String(subject),
				Message:  aws.String(message),
			})
			if err == nil {
				sent++
			}
		}
	}

	return map[string]any{"ok": true, "sent": sent, "skipped": skipped}, nil
}

func buildMessage(topic, shopDomain, webhookID string, detail map[string]any) (subject string, body string) {
	payload := asMap(pickAny(detail, "payload"))

	objID := fmt.Sprintf("%v", pickAny(payload, "id"))
	total := fmt.Sprintf("%v", pickAny(payload, "current_total_price", "total_price"))
	currency := pickString(payload, "currency")
	createdAt := pickString(payload, "created_at", "processed_at")

	subject = fmt.Sprintf("TrueProfit: %s (%s)", topic, shopDomain)

	lines := []string{
		"TrueProfit Shopify Event",
		"",
		fmt.Sprintf("Shop: %s", shopDomain),
		fmt.Sprintf("Topic: %s", topic),
	}
	if webhookID != "" {
		lines = append(lines, fmt.Sprintf("WebhookId: %s", webhookID))
	}
	if objID != "" && objID != "<nil>" {
		lines = append(lines, fmt.Sprintf("ObjectId: %s", objID))
	}
	if total != "" && total != "<nil>" {
		if currency == "" {
			currency = "USD"
		}
		lines = append(lines, fmt.Sprintf("Amount: %s %s", total, currency))
	}
	if createdAt != "" {
		lines = append(lines, fmt.Sprintf("CreatedAt: %s", createdAt))
	}
	lines = append(lines, "", fmt.Sprintf("ReceivedAt: %s", time.Now().UTC().Format(time.RFC3339)))

	body = strings.Join(lines, "\n")
	return subject, body
}

func pickString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func pickAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func asMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func main() { lambda.Start(handler) }
