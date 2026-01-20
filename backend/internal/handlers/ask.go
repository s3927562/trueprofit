package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	bedrockruntime "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/glue"

	"backend/internal/nlq"
	"backend/internal/tenancy"
)

type AskHandler struct {
	cfg  aws.Config
	glue *glue.Client
	ddb  *dynamodb.Client
}

func NewAskHandler(cfg aws.Config) *AskHandler {
	return &AskHandler{
		cfg:  cfg,
		glue: glue.NewFromConfig(cfg),
		ddb:  dynamodb.NewFromConfig(cfg),
	}
}

type AskRequest struct {
	Question string   `json:"question"`
	ShopIDs  []string `json:"shop_ids,omitempty"` // optional subset
}

func (h *AskHandler) Handle(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Parse JSON body
	var body AskRequest
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		return jsonErr(http.StatusBadRequest, "invalid_json", err), nil
	}
	body.Question = strings.TrimSpace(body.Question)
	if body.Question == "" {
		return jsonErr(http.StatusBadRequest, "question_required", nil), nil
	}

	// Auth: get Cognito sub
	sub := ""
	if req.RequestContext.Authorizer.JWT.Claims != nil {
		sub = req.RequestContext.Authorizer.JWT.Claims["sub"]
	}
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return jsonErr(http.StatusUnauthorized, "missing_user_sub", nil), nil
	}

	// Tenant scoping: allowed shops for this user (via GSI_UserSub on ShopToUser table)
	allowedShopIDs, err := tenancy.GetAllowedShopsByUserSub(ctx, h.ddb, sub)
	if err != nil {
		return jsonErr(http.StatusInternalServerError, "shop_lookup_failed", err), nil
	}
	if len(allowedShopIDs) == 0 {
		return jsonOK(map[string]any{
			"type":  "no_shops",
			"error": "no shops connected to this user",
		}), nil
	}

	effectiveShopIDs := intersectAllowed(body.ShopIDs, allowedShopIDs)
	if len(effectiveShopIDs) == 0 {
		return jsonErr(http.StatusForbidden, "no_allowed_shops_in_request", nil), nil
	}
	allowedShopIDs = effectiveShopIDs

	// Load schema from Glue
	schema, err := nlq.LoadTableSchemaFromEnv(ctx, h.glue)
	if err != nil {
		return jsonErr(http.StatusInternalServerError, "glue_get_table_failed", err), nil
	}
	schemaText := nlq.CompactSchemaText(schema)

	// Config
	maxDays := 90
	if v := strings.TrimSpace(os.Getenv("NLQ_MAX_DAYS")); v != "" {
		// optional parse
		// strconv.Atoi
	}
	today := nlq.TodayISO()
	tz := "Asia/Ho_Chi_Minh"

	schemaHash := nlq.SchemaHash(schemaText)

	// Check cache
	ck := nlq.CacheKey{
		UserSub:    sub,
		Shops:      allowedShopIDs,
		Question:   body.Question,
		TodayISO:   today,
		MaxDays:    maxDays,
		SchemaHash: schemaHash,
	}

	if cached, ok, err := nlq.GetCached(ctx, h.ddb, ck); err == nil && ok {
		return jsonOK(map[string]any{
			"type":          "result",
			"cached":        true,
			"sql":           cached.SQL,
			"assumptions":   cached.Assumptions,
			"confidence":    cached.Confidence,
			"result":        nlq.ShapeResult(cached.Columns, cached.Rows),
			"query_id":      cached.QueryID,
			"scanned_bytes": cached.ScannedBytes,
			"exec_ms":       cached.ExecMs,
		}), nil
	}

	// Build prompt for Bedrock (Claude)
	prompt := nlq.BuildPrompt(nlq.LLMRequest{
		Question:        body.Question,
		AllowedShopIDs:  allowedShopIDs,
		MaxDaysLookback: maxDays,
		SchemaText:      schemaText,
		TodayISO:        today,
		DefaultTimezone: tz,
	})

	// Clients
	br := bedrockruntime.NewFromConfig(h.cfg)
	ath := athena.NewFromConfig(h.cfg)

	// Invoke LLM for initial SQL
	llmRes, err := nlq.InvokeBedrockClaude(ctx, br, prompt)
	if err != nil {
		return jsonErr(http.StatusInternalServerError, "bedrock_error", err), nil
	}

	// Clarification branch
	if llmRes.NeedsClarification {
		return jsonOK(map[string]any{
			"type":                "clarification",
			"clarifying_question": llmRes.ClarifyingQuestion,
			"assumptions":         llmRes.Assumptions,
			"confidence":          llmRes.Confidence,
		}), nil
	}

	// Validate initial SQL (Step 12 includes dt lookback bound)
	sqlValidate := nlq.ValidateOptions{
		AllowedShopIDs:  allowedShopIDs,
		RequireDTFilter: true,
		MaxDaysLookback: maxDays,
		TodayISO:        today,
	}
	if err := nlq.ValidateSQL(llmRes.SQL, sqlValidate); err != nil {
		return jsonOK(map[string]any{
			"type":        "sql_rejected",
			"reason":      err.Error(),
			"model_sql":   llmRes.SQL,
			"assumptions": llmRes.Assumptions,
			"confidence":  llmRes.Confidence,
		}), nil
	}

	// Athena run options
	athOpt := nlq.AthenaRunOptions{
		Database:       strings.TrimSpace(os.Getenv("ATHENA_DATABASE")),
		Workgroup:      strings.TrimSpace(os.Getenv("ATHENA_WORKGROUP")),
		OutputLocation: strings.TrimSpace(os.Getenv("ATHENA_OUTPUT_S3")),
		MaxWait:        25 * time.Second,
		PollInterval:   700 * time.Millisecond,
		MaxResultRows:  200,
	}

	// Execute with self-correction (2 fix attempts)
	finalLLM, athRes, runErr := nlq.ExecuteWithSelfCorrection(
		ctx,
		br,  // BedrockClient
		ath, // AthenaClient
		sqlValidate,
		athOpt,
		body.Question,
		schemaText,
		allowedShopIDs,
		maxDays,
		today,
		tz,
		llmRes,
		2, // max fix attempts
	)
	if runErr != nil {
		lastSQL := ""
		lastAssumptions := []string(nil)
		lastConfidence := 0.0
		if finalLLM != nil {
			lastSQL = finalLLM.SQL
			lastAssumptions = finalLLM.Assumptions
			lastConfidence = finalLLM.Confidence
		}
		return jsonOK(map[string]any{
			"type":        "athena_failed",
			"error":       runErr.Error(),
			"last_sql":    lastSQL,
			"assumptions": lastAssumptions,
			"confidence":  lastConfidence,
		}), nil
	}

	// Clarification after a fix attempt (rare, but allowed)
	if athRes == nil && finalLLM != nil && finalLLM.NeedsClarification {
		return jsonOK(map[string]any{
			"type":                "clarification",
			"clarifying_question": finalLLM.ClarifyingQuestion,
			"assumptions":         finalLLM.Assumptions,
			"confidence":          finalLLM.Confidence,
		}), nil
	}

	// Cache successful result
	_ = nlq.PutCached(ctx, h.ddb, ck, nlq.CachedResponse{
		SQL:          finalLLM.SQL,
		Columns:      athRes.Columns,
		Rows:         athRes.Rows,
		Assumptions:  finalLLM.Assumptions,
		Confidence:   finalLLM.Confidence,
		ScannedBytes: athRes.ScannedBytes,
		ExecMs:       athRes.ExecutionMs,
		QueryID:      athRes.QueryExecutionID,
	})

	// Success: return results
	return jsonOK(map[string]any{
		"type":          "result",
		"sql":           finalLLM.SQL,
		"assumptions":   finalLLM.Assumptions,
		"confidence":    finalLLM.Confidence,
		"result":        nlq.ShapeResult(athRes.Columns, athRes.Rows),
		"query_id":      athRes.QueryExecutionID,
		"scanned_bytes": athRes.ScannedBytes,
		"exec_ms":       athRes.ExecutionMs,
	}), nil
}

func jsonOK(v any) events.APIGatewayV2HTTPResponse {
	b, _ := json.Marshal(v)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(b),
	}
}

func jsonErr(status int, msg string, err error) events.APIGatewayV2HTTPResponse {
	resp := map[string]any{"error": msg}
	if err != nil {
		resp["detail"] = err.Error()
	}
	b, _ := json.Marshal(resp)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(b),
	}
}

func intersectAllowed(requested, allowed []string) []string {
	if len(requested) == 0 {
		return allowed
	}
	allowedSet := map[string]bool{}
	for _, a := range allowed {
		allowedSet[strings.ToLower(strings.TrimSpace(a))] = true
	}
	out := make([]string, 0, len(requested))
	seen := map[string]bool{}
	for _, r := range requested {
		r2 := strings.TrimSpace(r)
		if r2 == "" {
			continue
		}
		k := strings.ToLower(r2)
		if !allowedSet[k] || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, r2)
	}
	return out
}
