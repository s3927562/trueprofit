package nlq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	bedrockruntime "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockClient interface {
	InvokeModel(ctx context.Context, params *bedrockruntime.InvokeModelInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.InvokeModelOutput, error)
}

type LLMRequest struct {
	Question        string
	AllowedShopIDs  []string
	MaxDaysLookback int
	SchemaText      string
	TodayISO        string // e.g. 2026-01-19
	DefaultTimezone string // e.g. Asia/Ho_Chi_Minh (optional)
}

type LLMResult struct {
	SQL                string   `json:"sql"`
	Confidence         float64  `json:"confidence"`
	Assumptions        []string `json:"assumptions"`
	NeedsClarification bool     `json:"needs_clarification"`
	ClarifyingQuestion *string  `json:"clarifying_question"`
}

func BuildPrompt(r LLMRequest) string {
	shops := strings.Join(r.AllowedShopIDs, ", ")
	if shops == "" {
		shops = "(none)"
	}

	today, _ := time.Parse("2006-01-02", r.TodayISO)
	dtMin := today.AddDate(0, 0, -r.MaxDaysLookback).Format("2006-01-02")

	return fmt.Sprintf(`
You are a Text-to-SQL compiler for AWS Athena.

OUTPUT: valid JSON ONLY (never SQL alone).

CRITICAL RULES:
- One SELECT statement only, no semicolon, no comments.
- Use ONLY tables/columns in schema.
- shop_id must be restricted to this allowlist: [%s].
- dt must always have a lower bound >= '%s'.
  Example:
    dt >= date '%s'
    OR dt between date '%s' and date '%s'
- metric_date is a string 'YYYY-MM-DD' — cast as date when needed.
- NEVER remove dt filter.
- Prefer partition pruning: filter dt and shop_id.
- ALWAYS wrap aggregate functions using COALESCE(..., 0) so results never return NULL.
  For example:
    SUM(x)        => COALESCE(SUM(x), 0)
    AVG(x)        => COALESCE(AVG(x), 0)
    MAX(x)        => COALESCE(MAX(x), 0)
    MIN(x)        => COALESCE(MIN(x), 0)
    COUNT(x)      => COALESCE(COUNT(x), 0)
- When the user asks for total/aggregate values, return a single scalar column named appropriately (e.g., total_net_revenue).

TODAY: %s
DT_MIN_ALLOWED: %s
LOCAL_TIMEZONE: %s

SCHEMA:
%s

USER QUESTION:
%s

Return JSON:
{
  "sql": "...",
  "confidence": 0.0,
  "assumptions": ["..."],
  "needs_clarification": false,
  "clarifying_question": null
}
`, shops, dtMin, dtMin, dtMin, r.TodayISO, r.TodayISO, dtMin, r.DefaultTimezone, r.SchemaText, r.Question)
}

// InvokeBedrockClaude sends the prompt and parses Claude JSON output.
// This version uses the Anthropic-style payload commonly used in Bedrock for Claude models.
func InvokeBedrockClaude(ctx context.Context, c BedrockClient, prompt string) (*LLMResult, error) {
	modelID := strings.TrimSpace(os.Getenv("BEDROCK_MODEL_ID"))
	if modelID == "" {
		return nil, fmt.Errorf("missing env BEDROCK_MODEL_ID")
	}

	// Claude on Bedrock typically accepts this schema:
	// { "anthropic_version": "bedrock-2023-05-31", "max_tokens": ..., "messages": [...] }
	payload := map[string]any{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        700,
		"temperature":       0.0,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)

	out, err := c.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        body,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock InvokeModel: %w", err)
	}

	// Parse response:
	// Claude returns JSON like: { "content":[{"type":"text","text":"..."}], ... }
	var raw struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(out.Body, &raw); err != nil {
		return nil, fmt.Errorf("bedrock response unmarshal: %w", err)
	}

	var text string
	for _, c := range raw.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	text = strings.TrimSpace(text)

	// Sometimes the model wraps JSON in extra whitespace. We require pure JSON.
	// Try to extract the first JSON object.
	jsonStr := extractFirstJSONObject(text)
	if jsonStr == "" {
		return nil, fmt.Errorf("model did not return JSON object")
	}

	var res LLMResult
	if err := json.Unmarshal([]byte(jsonStr), &res); err != nil {
		return nil, fmt.Errorf("LLM JSON parse failed: %w; raw=%s", err, truncate(jsonStr, 800))
	}
	res.SQL = strings.TrimSpace(res.SQL)
	return &res, nil
}

func TodayISO() string {
	return time.Now().UTC().Format("2006-01-02")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// extractFirstJSONObject finds the first {...} block. MVP-safe; not a full JSON parser.
func extractFirstJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
