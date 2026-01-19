package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/internal/db"
	"backend/internal/shopify"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type EBEvent struct {
	DetailType string         `json:"detail-type"`
	Source     string         `json:"source"`
	Time       string         `json:"time"`
	Detail     map[string]any `json:"detail"`
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSEventResponse, error) {
	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return events.SQSEventResponse{}, err
	}
	txTable := db.TransactionsTableName()

	failures := make([]events.SQSBatchItemFailure, 0)

	for _, rec := range sqsEvent.Records {
		if err := processOneRefund(ctx, ddb, txTable, rec.Body); err != nil {
			fmt.Printf("refunds-worker: msgId=%s failed: %v\n", rec.MessageId, err)
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
		}
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}

func processOneRefund(ctx context.Context, ddb *dynamodb.Client, txTable string, body string) error {
	var e EBEvent
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		return fmt.Errorf("unmarshal eb event: %w", err)
	}

	meta := asMap(pickAny(e.Detail, "metadata"))
	topic := pickString(meta, "X-Shopify-Topic")
	shopDomain := pickString(meta, "X-Shopify-Shop-Domain")
	webhookID := pickString(meta, "X-Shopify-Webhook-Id")

	if topic == "" || shopDomain == "" || !strings.HasPrefix(topic, "refunds/") {
		return nil
	}

	payload := pickAny(e.Detail, "payload")
	raw, _ := json.Marshal(payload)

	var refund map[string]any
	if err := json.Unmarshal(raw, &refund); err != nil {
		return fmt.Errorf("unmarshal refund payload: %w", err)
	}

	refundID := fmt.Sprintf("%v", pickAny(refund, "id"))
	if refundID == "" || refundID == "<nil>" {
		return fmt.Errorf("missing refund id")
	}

	amount, ok := findRefundAmount(refund)
	if !ok {
		return fmt.Errorf("cannot determine refund amount")
	}

	currency := pickString(refund, "currency")
	if currency == "" {
		currency = "USD"
	}

	createdAt := pickString(refund, "created_at", "processed_at", "updated_at")
	tm := parseShopifyTime(createdAt)
	month := tm.Format("2006-01")

	subs, err := shopify.UsersForShop(ctx, ddb, shopDomain)
	if err != nil {
		return fmt.Errorf("usersForShop: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	nowISO := time.Now().UTC().Format(time.RFC3339)
	for _, sub := range subs {
		_ = shopify.UpdateLastEvent(ctx, ddb, sub, shopDomain, nowISO, topic, webhookID)
	}

	for _, sub := range subs {
		txPK := fmt.Sprintf("USER#%s", sub)
		txSK := fmt.Sprintf("SHOPIFY#%s#REFUND#%s", shopDomain, refundID)

		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: txPK},
			"SK":        &types.AttributeValueMemberS{Value: txSK},
			"GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#MONTH#%s", sub, month)},
			"GSI1SK":    &types.AttributeValueMemberS{Value: tm.Format(time.RFC3339Nano)},
			"Amount":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", -1*amount)},
			"Currency":  &types.AttributeValueMemberS{Value: currency},
			"Category":  &types.AttributeValueMemberS{Value: "Shopify Refunds"},
			"Note":      &types.AttributeValueMemberS{Value: fmt.Sprintf("Refund %s (%s)", refundID, shopDomain)},
			"CreatedAt": &types.AttributeValueMemberS{Value: tm.Format(time.RFC3339)},
			"Source":    &types.AttributeValueMemberS{Value: "shopify"},
			"Shop":      &types.AttributeValueMemberS{Value: shopDomain},
			"Topic":     &types.AttributeValueMemberS{Value: topic},
			"RefundId":  &types.AttributeValueMemberS{Value: refundID},
		}

		_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(txTable),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
		})
		if err != nil {
			// If duplicate, treat as success; otherwise fail
			if !strings.Contains(err.Error(), "ConditionalCheckFailedException") {
				return fmt.Errorf("ddb put refund tx: %w", err)
			}
		}
	}

	return nil
}

func findRefundAmount(refund map[string]any) (float64, bool) {
	if txs, ok := pickAny(refund, "transactions").([]any); ok && len(txs) > 0 {
		sum := 0.0
		found := false
		for _, t := range txs {
			m, ok := t.(map[string]any)
			if !ok {
				continue
			}
			kind := strings.ToLower(fmt.Sprintf("%v", pickAny(m, "kind")))
			status := strings.ToLower(fmt.Sprintf("%v", pickAny(m, "status")))

			if kind != "" && kind != "refund" {
				continue
			}
			if status != "" && status != "success" {
				continue
			}
			if f, ok := parseFloatAny(pickAny(m, "amount")); ok {
				sum += f
				found = true
			}
		}
		if found {
			return sum, true
		}
	}

	if f, ok := parseFloatAny(pickAny(refund, "amount")); ok {
		return f, true
	}
	if f, ok := parseFloatAny(pickAny(refund, "total_refunded")); ok {
		return f, true
	}
	return 0, false
}

func parseFloatAny(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		if x == "" || x == "<nil>" {
			return 0, false
		}
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func parseShopifyTime(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
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
