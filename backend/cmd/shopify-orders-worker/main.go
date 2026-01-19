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
		// Fail whole batch (infra issue)
		return events.SQSEventResponse{}, err
	}
	txTable := db.TransactionsTableName()

	failures := make([]events.SQSBatchItemFailure, 0)

	for _, rec := range sqsEvent.Records {
		if err := processOneOrder(ctx, ddb, txTable, rec.Body); err != nil {
			// Log + mark this message as failed so it retries (or goes to DLQ)
			fmt.Printf("orders-worker: msgId=%s failed: %v\n", rec.MessageId, err)
			failures = append(failures, events.SQSBatchItemFailure{ItemIdentifier: rec.MessageId})
		}
	}

	return events.SQSEventResponse{BatchItemFailures: failures}, nil
}

func processOneOrder(ctx context.Context, ddb *dynamodb.Client, txTable string, body string) error {
	var e EBEvent
	if err := json.Unmarshal([]byte(body), &e); err != nil {
		return fmt.Errorf("unmarshal eb event: %w", err)
	}

	meta := asMap(pickAny(e.Detail, "metadata"))
	topic := pickString(meta, "X-Shopify-Topic")
	shopDomain := pickString(meta, "X-Shopify-Shop-Domain")
	webhookID := pickString(meta, "X-Shopify-Webhook-Id")

	if topic == "" || shopDomain == "" || !strings.HasPrefix(topic, "orders/") {
		// Not ours; treat as success (should not happen due to filter)
		return nil
	}

	payload := pickAny(e.Detail, "payload")
	raw, _ := json.Marshal(payload)

	var order map[string]any
	if err := json.Unmarshal(raw, &order); err != nil {
		return fmt.Errorf("unmarshal order payload: %w", err)
	}

	orderID := fmt.Sprintf("%v", pickAny(order, "id"))
	if orderID == "" || orderID == "<nil>" {
		return fmt.Errorf("missing order id")
	}

	// More tolerant amount extraction: try multiple fields
	amount, currency, err := extractOrderTotal(order)
	if err != nil {
		return fmt.Errorf("extract amount: %w", err)
	}
	if currency == "" {
		currency = "USD"
	}

	createdAt := pickString(order, "processed_at", "created_at", "updated_at")
	tm := parseShopifyTime(createdAt)
	month := tm.Format("2006-01")

	name := pickString(order, "name")
	if name == "" {
		name = fmt.Sprintf("Order %s", orderID)
	}

	subs, err := shopify.UsersForShop(ctx, ddb, shopDomain)
	if err != nil {
		return fmt.Errorf("usersForShop: %w", err)
	}
	if len(subs) == 0 {
		// No users mapped; success (nothing to do)
		return nil
	}

	// UpdateLastEvent (non-fatal)
	nowISO := time.Now().UTC().Format(time.RFC3339)
	for _, sub := range subs {
		_ = shopify.UpdateLastEvent(ctx, ddb, sub, shopDomain, nowISO, topic, webhookID)
	}

	// Upsert per user
	for _, sub := range subs {
		txPK := fmt.Sprintf("USER#%s", sub)
		txSK := fmt.Sprintf("SHOPIFY#%s#ORDER#%s", shopDomain, orderID)

		item := map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: txPK},
			"SK":        &types.AttributeValueMemberS{Value: txSK},
			"GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#MONTH#%s", sub, month)},
			"GSI1SK":    &types.AttributeValueMemberS{Value: tm.Format(time.RFC3339Nano)},
			"Amount":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", amount)},
			"Currency":  &types.AttributeValueMemberS{Value: currency},
			"Category":  &types.AttributeValueMemberS{Value: "Shopify Sales"},
			"Note":      &types.AttributeValueMemberS{Value: fmt.Sprintf("%s (%s)", name, shopDomain)},
			"CreatedAt": &types.AttributeValueMemberS{Value: tm.Format(time.RFC3339)},
			"Source":    &types.AttributeValueMemberS{Value: "shopify"},
			"Shop":      &types.AttributeValueMemberS{Value: shopDomain},
			"Topic":     &types.AttributeValueMemberS{Value: topic},
			"OrderId":   &types.AttributeValueMemberS{Value: orderID},
			"OrderName": &types.AttributeValueMemberS{Value: name},
		}

		if _, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(txTable),
			Item:      item,
		}); err != nil {
			return fmt.Errorf("ddb put order tx: %w", err)
		}
	}

	return nil
}

func extractOrderTotal(order map[string]any) (amount float64, currency string, err error) {
	// 1) current_total_price (string)
	if s, ok := pickAny(order, "current_total_price").(string); ok && s != "" {
		f, e := strconv.ParseFloat(s, 64)
		if e == nil {
			return f, pickString(order, "currency"), nil
		}
	}
	// 2) total_price (string)
	if s, ok := pickAny(order, "total_price").(string); ok && s != "" {
		f, e := strconv.ParseFloat(s, 64)
		if e == nil {
			return f, pickString(order, "currency"), nil
		}
	}
	// 3) current_total_price_set.shop_money.amount
	if m, ok := pickAny(order, "current_total_price_set").(map[string]any); ok {
		if sm, ok := m["shop_money"].(map[string]any); ok {
			amtS, _ := sm["amount"].(string)
			curS, _ := sm["currency_code"].(string)
			if amtS != "" {
				f, e := strconv.ParseFloat(amtS, 64)
				if e == nil {
					return f, curS, nil
				}
			}
		}
	}
	// 4) total_price_set.shop_money.amount
	if m, ok := pickAny(order, "total_price_set").(map[string]any); ok {
		if sm, ok := m["shop_money"].(map[string]any); ok {
			amtS, _ := sm["amount"].(string)
			curS, _ := sm["currency_code"].(string)
			if amtS != "" {
				f, e := strconv.ParseFloat(amtS, 64)
				if e == nil {
					return f, curS, nil
				}
			}
		}
	}
	return 0, "", fmt.Errorf("no total price field found")
}

func parseShopifyTime(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	// Sometimes Shopify sends with timezone offset like 2026-01-18T10:21:02-05:00 (still RFC3339)
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
