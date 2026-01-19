package shopify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type webhookCreateReq struct {
	Webhook struct {
		Address string `json:"address"`
		Topic   string `json:"topic"`
		Format  string `json:"format"`
	} `json:"webhook"`
}

type webhookCreateResp struct {
	Webhook any `json:"webhook"`
	Errors  any `json:"errors"`
}

// Creates a Shopify webhook whose address is the EventBridge partner event source ARN.
// Shopify will then deliver events to Partner Event Source/Event Bus.
func CreateEventBridgeWebhook(ctx context.Context, shopDomain, apiVersion, accessToken, topic, eventSourceArn string) (string, error) {
	url := fmt.Sprintf("https://%s/admin/api/%s/webhooks.json", shopDomain, apiVersion)

	var payload webhookCreateReq
	payload.Webhook.Address = eventSourceArn
	payload.Webhook.Topic = topic
	payload.Webhook.Format = "json"

	b, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shopify-Access-Token", accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("create webhook failed: http %d: %s", res.StatusCode, string(raw))
	}

	// Shopify returns webhook id in the response body in many cases,
	// but we won't depend on it. We'll just return topic as an identifier.
	return topic, nil
}

// Subscribe a shop to all required topics.
func SubscribeEventBridgeTopics(ctx context.Context, shopDomain, apiVersion, accessToken, eventSourceArn string) (created []string, failed []map[string]string) {
	topics := []string{
		"orders/create",
		"orders/updated",
		"refunds/create",
	}

	for _, t := range topics {
		_, err := CreateEventBridgeWebhook(ctx, shopDomain, apiVersion, accessToken, t, eventSourceArn)
		if err != nil {
			failed = append(failed, map[string]string{"topic": t, "error": err.Error()})
			continue
		}
		created = append(created, t)
	}
	return created, failed
}
