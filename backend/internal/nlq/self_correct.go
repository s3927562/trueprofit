package nlq

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type FixSQLRequest struct {
	OriginalQuestion string
	SchemaText       string
	AllowedShopIDs   []string
	MaxDaysLookback  int
	TodayISO         string
	Timezone         string

	PreviousSQL string
	AthenaError string
}

func BuildFixPrompt(r FixSQLRequest) string {
	today, _ := time.Parse("2006-01-02", r.TodayISO)
	dtMin := today.AddDate(0, 0, -r.MaxDaysLookback).Format("2006-01-02")

	shops := strings.Join(r.AllowedShopIDs, ", ")
	if shops == "" {
		shops = "(none)"
	}

	return fmt.Sprintf(`
FIX the SQL query.

CRITICAL RULES:
- Output JSON only.
- One SELECT only.
- shop_id must remain inside allowlist [%s].
- dt MUST have lower bound >= '%s'.
- schema + question must be respected.

SCHEMA:
%s

QUESTION:
%s

PREVIOUS SQL:
%s

ATHENA ERROR:
%s

Return JSON:
{
  "sql": "...",
  "confidence": 0.0,
  "assumptions": ["..."],
  "needs_clarification": false,
  "clarifying_question": null
}
`, shops, dtMin, r.SchemaText, r.OriginalQuestion, r.PreviousSQL, r.AthenaError)
}

func ExecuteWithSelfCorrection(
	ctx context.Context,
	bedrock BedrockClient,
	athena AthenaClient,
	sqlValidate ValidateOptions,
	athenaOpt AthenaRunOptions,
	question string,
	schemaText string,
	allowedShopIDs []string,
	maxDays int,
	todayISO string,
	timezone string,
	initialLLM *LLMResult,
	maxFixAttempts int,
) (*LLMResult, *AthenaResult, error) {

	if maxFixAttempts < 0 {
		maxFixAttempts = 0
	}

	// Attempt 0: initial SQL
	cur := *initialLLM
	if err := ValidateSQL(cur.SQL, sqlValidate); err != nil {
		return nil, nil, fmt.Errorf("initial sql rejected: %w", err)
	}
	res, err := RunAthenaQuery(ctx, athena, cur.SQL, athenaOpt)
	if err == nil {
		return &cur, res, nil
	}

	lastErr := err
	for attempt := 1; attempt <= maxFixAttempts; attempt++ {
		fixPrompt := BuildFixPrompt(FixSQLRequest{
			OriginalQuestion: question,
			SchemaText:       schemaText,
			AllowedShopIDs:   allowedShopIDs,
			MaxDaysLookback:  maxDays,
			TodayISO:         todayISO,
			Timezone:         timezone,
			PreviousSQL:      cur.SQL,
			AthenaError:      lastErr.Error(),
		})

		fixed, ferr := InvokeBedrockClaude(ctx, bedrock, fixPrompt)
		if ferr != nil {
			return nil, nil, fmt.Errorf("bedrock fix attempt %d failed: %w", attempt, ferr)
		}
		if fixed.NeedsClarification {
			// bubble up clarification
			return fixed, nil, nil
		}

		if err := ValidateSQL(fixed.SQL, sqlValidate); err != nil {
			lastErr = fmt.Errorf("fixed sql rejected: %w", err)
			cur = *fixed
			continue
		}

		// If model forgot dt lower bound, auto-inject dt >= dtMin
		today, _ := time.Parse("2006-01-02", todayISO)
		dtMin := today.AddDate(0, 0, -maxDays).Format("2006-01-02")

		if !strings.Contains(strings.ToLower(cur.SQL), "dt >=") &&
			!strings.Contains(strings.ToLower(cur.SQL), "dt between") {
			cur.SQL = fmt.Sprintf(
				"SELECT * FROM (%s) WHERE dt >= date '%s'",
				cur.SQL,
				dtMin,
			)
		}

		r2, err2 := RunAthenaQuery(ctx, athena, fixed.SQL, athenaOpt)
		if err2 == nil {
			return fixed, r2, nil
		}
		lastErr = err2
		cur = *fixed
	}

	return &cur, nil, fmt.Errorf("athena failed after retries: %w", lastErr)
}
