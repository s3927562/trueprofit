package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/internal/db"
	"backend/internal/security"
	"backend/internal/shopify"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func ShopifyHandler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Route by path + method
	switch req.RawPath {
	case "/integrations/shopify/connect":
		return shopifyConnect(ctx, req)
	case "/integrations/shopify/callback":
		return shopifyCallback(ctx, req)
	case "/integrations/shopify/shops":
		if req.RequestContext.HTTP.Method == "GET" {
			return shopifyListShops(ctx, req)
		}
		if req.RequestContext.HTTP.Method == "DELETE" {
			return shopifyDisconnectShop(ctx, req)
		}
		return errResp(405, "method not allowed")
	case "/integrations/shopify/sync":
		if req.RequestContext.HTTP.Method == "POST" {
			return shopifySyncStub(ctx, req)
		}
		return errResp(405, "method not allowed")
	default:
		return errResp(404, "not found")
	}
}

func shopifyConnect(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Must be logged in (Cognito JWT authorizer)
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	shop := strings.ToLower(strings.TrimSpace(req.QueryStringParameters["shop"]))
	if !isValidShopDomain(shop) {
		return errResp(400, "invalid shop (expected like your-store.myshopify.com)")
	}

	state, err := randomState(24)
	if err != nil {
		return errResp(500, "failed to generate state")
	}

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	stateTable := db.OAuthStateTableName()
	if strings.TrimSpace(stateTable) == "" {
		return errResp(500, "OAUTH_STATE_TABLE not set")
	}

	exp := time.Now().UTC().Add(10 * time.Minute).Unix()

	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(stateTable),
		Item: map[string]types.AttributeValue{
			"State":          &types.AttributeValueMemberS{Value: state},
			"UserSub":        &types.AttributeValueMemberS{Value: sub},
			"Shop":           &types.AttributeValueMemberS{Value: shop},
			"ExpiresAtEpoch": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", exp)},
		},
	})
	if err != nil {
		return errResp(500, "failed to store oauth state")
	}

	apiKey := os.Getenv("SHOPIFY_API_KEY")
	scopes := strings.TrimSpace(os.Getenv("SHOPIFY_SCOPES"))
	redirectBase := strings.TrimRight(os.Getenv("SHOPIFY_REDIRECT_BASE"), "/")
	if apiKey == "" || scopes == "" || redirectBase == "" {
		return errResp(500, "missing SHOPIFY_* env vars")
	}

	redirectURI := redirectBase + "/integrations/shopify/callback"

	authorize := fmt.Sprintf("https://%s/admin/oauth/authorize", shop)
	u, _ := url.Parse(authorize)
	q := u.Query()
	q.Set("client_id", apiKey)
	q.Set("scope", scopes)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	u.RawQuery = q.Encode()

	return jsonResp(200, map[string]any{
		"authorizeUrl": u.String(),
	})
}

func shopifyCallback(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	params := req.QueryStringParameters

	shop := strings.ToLower(strings.TrimSpace(params["shop"]))
	code := strings.TrimSpace(params["code"])
	state := strings.TrimSpace(params["state"])
	hmacParam := strings.TrimSpace(params["hmac"])

	if !isValidShopDomain(shop) || code == "" || state == "" || hmacParam == "" {
		return errResp(400, "missing required oauth params")
	}

	secret := os.Getenv("SHOPIFY_API_SECRET")
	if secret == "" {
		return errResp(500, "SHOPIFY_API_SECRET not set")
	}
	if !verifyShopifyHMAC(params, secret, hmacParam) {
		return errResp(400, "invalid hmac")
	}

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	// Validate state
	stateTable := db.OAuthStateTableName()
	out, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(stateTable),
		Key: map[string]types.AttributeValue{
			"State": &types.AttributeValueMemberS{Value: state},
		},
	})
	if err != nil || out.Item == nil {
		return errResp(400, "invalid or expired state")
	}

	userSub := attrS(out.Item["UserSub"])
	shopFromState := attrS(out.Item["Shop"])
	if userSub == "" || shopFromState == "" || shopFromState != shop {
		return errResp(400, "state mismatch")
	}

	// Exchange code -> access token
	apiKey := os.Getenv("SHOPIFY_API_KEY")
	tokenURL := fmt.Sprintf("https://%s/admin/oauth/access_token", shop)

	body := map[string]string{
		"client_id":     apiKey,
		"client_secret": secret,
		"code":          code,
	}
	b, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(string(b)))
	httpReq.Header.Set("content-type", "application/json")

	httpRes, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return errResp(502, "token exchange failed")
	}
	defer httpRes.Body.Close()

	raw, _ := io.ReadAll(httpRes.Body)
	if httpRes.StatusCode < 200 || httpRes.StatusCode >= 300 {
		return errResp(502, fmt.Sprintf("token exchange failed: %s", string(raw)))
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil || tok.AccessToken == "" {
		return errResp(502, "invalid token response")
	}

	// Encrypt token before storing
	keyB64 := os.Getenv("TOKEN_ENC_KEY_B64")
	key, err := security.LoadKeyFromBase64(keyB64)
	if err != nil {
		return errResp(500, "invalid TOKEN_ENC_KEY_B64")
	}

	encTok, err := security.EncryptAESGCM(key, tok.AccessToken)
	if err != nil {
		return errResp(500, "failed to encrypt token")
	}

	intTable := db.IntegrationsTableName()
	if strings.TrimSpace(intTable) == "" {
		return errResp(500, "INTEGRATIONS_TABLE not set")
	}

	pk := fmt.Sprintf("USER#%s", userSub)
	sk := fmt.Sprintf("SHOPIFY#%s", shop)

	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(intTable),
		Item: map[string]types.AttributeValue{
			"PK":             &types.AttributeValueMemberS{Value: pk},
			"SK":             &types.AttributeValueMemberS{Value: sk},
			"Provider":       &types.AttributeValueMemberS{Value: "shopify"},
			"Shop":           &types.AttributeValueMemberS{Value: shop},
			"AccessTokenEnc": &types.AttributeValueMemberS{Value: encTok},
			"Scope":          &types.AttributeValueMemberS{Value: tok.Scope},
			"CreatedAt":      &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return errResp(500, "failed to store integration")
	}

	mapTable := os.Getenv("SHOP_TO_USER_TABLE")
	if mapTable != "" {
		shopPk := fmt.Sprintf("SHOP#%s", shop)
		shopSk := fmt.Sprintf("USER#%s", userSub)

		_, _ = ddb.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(mapTable),
			Item: map[string]types.AttributeValue{
				"PK":        &types.AttributeValueMemberS{Value: shopPk},
				"SK":        &types.AttributeValueMemberS{Value: shopSk},
				"Shop":      &types.AttributeValueMemberS{Value: shop},
				"UserSub":   &types.AttributeValueMemberS{Value: userSub},
				"CreatedAt": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			},
		})
	}

	// Subscribe this shop to required webhooks
	eventSourceArn := strings.TrimSpace(os.Getenv("SHOPIFY_EVENTBRIDGE_SOURCE_ARN"))
	apiVersion := strings.TrimSpace(os.Getenv("SHOPIFY_API_VERSION"))
	if apiVersion == "" {
		apiVersion = "2026-01"
	}
	shopify.SubscribeEventBridgeTopics(ctx, shop, apiVersion, tok.AccessToken, eventSourceArn)

	// one-time state cleanup
	_, _ = ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(stateTable),
		Key: map[string]types.AttributeValue{
			"State": &types.AttributeValueMemberS{Value: state},
		},
	})

	// Redirect back to frontend Shopify page
	fe := strings.TrimRight(os.Getenv("FRONTEND_BASE_URL"), "/")
	if fe == "" {
		fe = "/"
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 302,
		Headers: map[string]string{
			"location": fe + "/shopify?connected=1&shop=" + url.QueryEscape(shop),
		},
	}, nil
}

func shopifyListShops(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	intTable := db.IntegrationsTableName()
	if strings.TrimSpace(intTable) == "" {
		return errResp(500, "INTEGRATIONS_TABLE not set")
	}

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	pk := fmt.Sprintf("USER#%s", sub)

	out, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(intTable),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :pref)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":   &types.AttributeValueMemberS{Value: pk},
			":pref": &types.AttributeValueMemberS{Value: "SHOPIFY#"},
		},
		Limit: aws.Int32(50),
	})
	if err != nil {
		return errResp(500, "query failed")
	}

	type ShopItem struct {
		Shop               string `json:"shop"`
		Scope              string `json:"scope"`
		CreatedAt          string `json:"createdAt"`
		LastEventAt        string `json:"lastEventAt"`
		LastEventTopic     string `json:"lastEventTopic"`
		LastEventWebhookId string `json:"lastEventWebhookId"`
	}

	items := make([]ShopItem, 0, len(out.Items))
	for _, it := range out.Items {
		items = append(items, ShopItem{
			Shop:               attrS(it["Shop"]),
			Scope:              attrS(it["Scope"]),
			CreatedAt:          attrS(it["CreatedAt"]),
			LastEventAt:        attrS(it["LastEventAt"]),
			LastEventTopic:     attrS(it["LastEventTopic"]),
			LastEventWebhookId: attrS(it["LastEventWebhookId"]),
		})
	}

	return jsonResp(200, map[string]any{"items": items})
}

func shopifyDisconnectShop(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	shop := strings.ToLower(strings.TrimSpace(req.QueryStringParameters["shop"]))
	if !isValidShopDomain(shop) {
		return errResp(400, "invalid shop")
	}

	intTable := db.IntegrationsTableName()
	if strings.TrimSpace(intTable) == "" {
		return errResp(500, "INTEGRATIONS_TABLE not set")
	}

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	pk := fmt.Sprintf("USER#%s", sub)
	sk := fmt.Sprintf("SHOPIFY#%s", shop)

	_, err = ddb.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(intTable),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return errResp(500, "delete failed")
	}

	return jsonResp(200, map[string]any{"ok": true})
}

func shopifySyncStub(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	return shopifySyncReal(ctx, req)
}

type shopifyIntegrationItem struct {
	PK             string `dynamodbav:"PK"`
	SK             string `dynamodbav:"SK"`
	Shop           string `dynamodbav:"Shop"`
	AccessTokenEnc string `dynamodbav:"AccessTokenEnc"`
	Scope          string `dynamodbav:"Scope"`
	CreatedAt      string `dynamodbav:"CreatedAt"`
	LastSyncAt     string `dynamodbav:"LastSyncAt,omitempty"`
}

type shopifyMoney struct {
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currencyCode"`
}

type shopifyOrderNode struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	ProcessedAt   string `json:"processedAt"`
	UpdatedAt     string `json:"updatedAt"`
	TotalPriceSet struct {
		ShopMoney shopifyMoney `json:"shopMoney"`
	} `json:"totalPriceSet"`

	Refunds shopifyRefunds `json:"refunds"`
}

type shopifyOrdersPage struct {
	Orders struct {
		Edges []struct {
			Cursor string           `json:"cursor"`
			Node   shopifyOrderNode `json:"node"`
		} `json:"edges"`
		PageInfo struct {
			HasNextPage bool   `json:"hasNextPage"`
			EndCursor   string `json:"endCursor"`
		} `json:"pageInfo"`
	} `json:"orders"`
}

type shopifyRefundNode struct {
	Id               string `json:"id"`
	CreatedAt        string `json:"createdAt"`
	TotalRefundedSet struct {
		ShopMoney shopifyMoney `json:"shopMoney"`
	} `json:"totalRefundedSet"`
}

type shopifyRefunds struct {
	Edges []struct {
		Node shopifyRefundNode `json:"node"`
	} `json:"edges"`
}

func shopifySyncReal(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	shopDomain := strings.ToLower(strings.TrimSpace(req.QueryStringParameters["shop"]))
	if !isValidShopDomain(shopDomain) {
		return errResp(400, "invalid shop")
	}

	// optional limit per sync run
	limit := 50
	if s := strings.TrimSpace(req.QueryStringParameters["limit"]); s != "" {
		if n, e := strconv.Atoi(s); e == nil && n >= 1 && n <= 200 {
			limit = n
		}
	}

	intTable := db.IntegrationsTableName()
	txTable := db.TransactionsTableName()
	if strings.TrimSpace(intTable) == "" || strings.TrimSpace(txTable) == "" {
		return errResp(500, "tables not configured")
	}

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	pk := fmt.Sprintf("USER#%s", sub)
	sk := fmt.Sprintf("SHOPIFY#%s", shopDomain)

	accessToken, integ, err := shopify.LoadIntegrationAndDecryptToken(ctx, sub, shopDomain)
	if err != nil {
		return errResp(500, err.Error())
	}

	apiVersion := strings.TrimSpace(os.Getenv("SHOPIFY_API_VERSION"))
	if apiVersion == "" {
		apiVersion = "2026-01"
	}

	// Build query: sync orders updated after LastSyncAt (or last 30 days if never synced)
	// Shopify supports filtering in the orders query (query string)
	since := integ.LastSyncAt
	if since == "" {
		since = time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	}

	gqlQuery := `
query OrdersSync($first: Int!, $after: String, $q: String!) {
  orders(first: $first, after: $after, query: $q, sortKey: UPDATED_AT) {
    edges {
      cursor
      node {
        id
        name
        processedAt
        updatedAt
        totalPriceSet { shopMoney { amount currencyCode } }

        refunds(first: 20) {
          edges {
            node {
              id
              createdAt
              totalRefundedSet { shopMoney { amount currencyCode } }
            }
          }
        }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

	q := fmt.Sprintf("updated_at:>=%s", since)

	created := 0
	skipped := 0
	var endCursor *string = nil
	var newestUpdatedAt string = since

	for created+skipped < limit {
		first := 50
		if limit-(created+skipped) < first {
			first = limit - (created + skipped)
		}

		vars := map[string]any{
			"first": first,
			"after": endCursor,
			"q":     q,
		}

		resp, status, err := shopify.PostGraphQL[shopifyOrdersPage](ctx, shopDomain, apiVersion, accessToken, gqlQuery, vars)
		if err != nil {
			return errResp(502, "shopify request failed")
		}
		if status < 200 || status >= 300 {
			return errResp(502, fmt.Sprintf("shopify error status %d", status))
		}
		if len(resp.Errors) > 0 {
			msgs := make([]string, 0, len(resp.Errors))
			for _, e := range resp.Errors {
				if e.Extensions.Code != "" {
					msgs = append(msgs, e.Message+" ("+e.Extensions.Code+")")
				} else {
					msgs = append(msgs, e.Message)
				}
			}
			return jsonResp(502, map[string]any{
				"error":  "shopify graphql returned errors",
				"errors": msgs,
			})
		}

		edges := resp.Data.Orders.Edges
		if len(edges) == 0 {
			break
		}

		for _, e := range edges {
			o := e.Node

			// Track newest updatedAt to advance LastSyncAt
			if o.UpdatedAt != "" && o.UpdatedAt > newestUpdatedAt {
				newestUpdatedAt = o.UpdatedAt
			}

			// Parse amount
			amt, err := strconv.ParseFloat(o.TotalPriceSet.ShopMoney.Amount, 64)
			if err != nil {
				skipped++
				continue
			}

			// Use order processedAt as CreatedAt
			createdAt := o.ProcessedAt
			if createdAt == "" {
				createdAt = time.Now().UTC().Format(time.RFC3339)
			}

			// Create deterministic transaction key (idempotent)
			// Example: SHOPIFY#shop.myshopify.com#ORDER#<gid last segment>
			orderId := o.Id
			if i := strings.LastIndex(orderId, "/"); i >= 0 {
				orderId = orderId[i+1:]
			}

			txPK := fmt.Sprintf("USER#%s", sub)
			txSK := fmt.Sprintf("SHOPIFY#%s#ORDER#%s", shopDomain, orderId)

			// Also set GSI1 so monthly summary works
			tm, terr := time.Parse(time.RFC3339, createdAt)
			if terr != nil {
				tm = time.Now().UTC()
			}
			month := tm.UTC().Format("2006-01")

			item := map[string]types.AttributeValue{
				"PK":        &types.AttributeValueMemberS{Value: txPK},
				"SK":        &types.AttributeValueMemberS{Value: txSK},
				"GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#MONTH#%s", sub, month)},
				"GSI1SK":    &types.AttributeValueMemberS{Value: tm.UTC().Format(time.RFC3339Nano)},
				"Amount":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", amt)},
				"Currency":  &types.AttributeValueMemberS{Value: o.TotalPriceSet.ShopMoney.CurrencyCode},
				"Category":  &types.AttributeValueMemberS{Value: "Shopify Sales"},
				"Note":      &types.AttributeValueMemberS{Value: fmt.Sprintf("%s (%s)", o.Name, shopDomain)},
				"CreatedAt": &types.AttributeValueMemberS{Value: tm.UTC().Format(time.RFC3339)},
				"Source":    &types.AttributeValueMemberS{Value: "shopify"},
				"Shop":      &types.AttributeValueMemberS{Value: shopDomain},
				"OrderGid":  &types.AttributeValueMemberS{Value: o.Id},
				"OrderName": &types.AttributeValueMemberS{Value: o.Name},
				"UpdatedAt": &types.AttributeValueMemberS{Value: o.UpdatedAt},
			}

			_, putErr := ddb.PutItem(ctx, &dynamodb.PutItemInput{
				TableName:           aws.String(txTable),
				Item:                item,
				ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
			})
			if putErr != nil {
				// If already exists, treat as idempotent skip
				skipped++
			} else {
				created++
			}

			// Create refund transactions (negative amounts)
			for _, re := range o.Refunds.Edges {
				r := re.Node

				refAmt, err := strconv.ParseFloat(r.TotalRefundedSet.ShopMoney.Amount, 64)
				if err != nil || refAmt == 0 {
					continue
				}

				refId := r.Id
				if i := strings.LastIndex(refId, "/"); i >= 0 {
					refId = refId[i+1:]
				}

				refTime, terr := time.Parse(time.RFC3339, r.CreatedAt)
				if terr != nil {
					refTime = time.Now().UTC()
				}
				refMonth := refTime.UTC().Format("2006-01")

				refSK := fmt.Sprintf("SHOPIFY#%s#REFUND#%s", shopDomain, refId)

				refItem := map[string]types.AttributeValue{
					"PK":        &types.AttributeValueMemberS{Value: txPK},
					"SK":        &types.AttributeValueMemberS{Value: refSK},
					"GSI1PK":    &types.AttributeValueMemberS{Value: fmt.Sprintf("USER#%s#MONTH#%s", sub, refMonth)},
					"GSI1SK":    &types.AttributeValueMemberS{Value: refTime.UTC().Format(time.RFC3339Nano)},
					"Amount":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", -1*refAmt)},
					"Currency":  &types.AttributeValueMemberS{Value: r.TotalRefundedSet.ShopMoney.CurrencyCode},
					"Category":  &types.AttributeValueMemberS{Value: "Shopify Refunds"},
					"Note":      &types.AttributeValueMemberS{Value: fmt.Sprintf("%s refund (%s)", o.Name, shopDomain)},
					"CreatedAt": &types.AttributeValueMemberS{Value: refTime.UTC().Format(time.RFC3339)},
					"Source":    &types.AttributeValueMemberS{Value: "shopify"},
					"Shop":      &types.AttributeValueMemberS{Value: shopDomain},
					"OrderGid":  &types.AttributeValueMemberS{Value: o.Id},
					"OrderName": &types.AttributeValueMemberS{Value: o.Name},
					"RefundGid": &types.AttributeValueMemberS{Value: r.Id},
				}

				_, putErr := ddb.PutItem(ctx, &dynamodb.PutItemInput{
					TableName:           aws.String(txTable),
					Item:                refItem,
					ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
				})
				if putErr != nil {
					// already exists => ignore
				} else {
					created++
				}
			}

			if created+skipped >= limit {
				break
			}
		}

		if !resp.Data.Orders.PageInfo.HasNextPage || resp.Data.Orders.PageInfo.EndCursor == "" {
			break
		}
		c := resp.Data.Orders.PageInfo.EndCursor
		endCursor = &c
	}

	// Persist LastSyncAt per shop so next sync continues
	_, _ = ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(intTable),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET LastSyncAt = :t"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":t": &types.AttributeValueMemberS{Value: newestUpdatedAt},
		},
	})

	return jsonResp(200, map[string]any{
		"ok":         true,
		"shop":       shopDomain,
		"created":    created,
		"skipped":    skipped,
		"lastSyncAt": newestUpdatedAt,
	})
}

/** Helpers **/

func attrS(av types.AttributeValue) string {
	if s, ok := av.(*types.AttributeValueMemberS); ok {
		return s.Value
	}
	return ""
}

func isValidShopDomain(shop string) bool {
	if !strings.HasSuffix(shop, ".myshopify.com") {
		return false
	}
	if strings.Contains(shop, "/") || strings.Contains(shop, " ") {
		return false
	}
	return len(shop) >= len("a.myshopify.com")
}

func randomState(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func verifyShopifyHMAC(params map[string]string, secret, providedHex string) bool {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "hmac" || k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	msg := strings.Join(parts, "&")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(msg))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(strings.ToLower(providedHex)))
}

func marshalMiniJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
