package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"backend/internal/db"
	"backend/internal/users"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type Transaction struct {
	PK string `dynamodbav:"PK" json:"-"`
	SK string `dynamodbav:"SK" json:"id"`

	GSI1PK string `dynamodbav:"GSI1PK" json:"-"`
	GSI1SK string `dynamodbav:"GSI1SK" json:"-"`

	UserSub   string  `dynamodbav:"UserSub" json:"-"`
	Amount    float64 `dynamodbav:"Amount" json:"amount"`
	Currency  string  `dynamodbav:"Currency" json:"currency"`
	Category  string  `dynamodbav:"Category" json:"category"`
	Note      string  `dynamodbav:"Note" json:"note"`
	CreatedAt string  `dynamodbav:"CreatedAt" json:"createdAt"`
}

type CreateTransactionRequest struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Category string  `json:"category"`
	Note     string  `json:"note"`
}

func userSub(req events.APIGatewayV2HTTPRequest) (string, string, error) {
	// For HTTP API JWT authorizer, claims are in:
	// req.RequestContext.Authorizer.JWT.Claims
	if req.RequestContext.Authorizer.JWT.Claims == nil {
		return "", "", errors.New("missing authorizer claims")
	}
	claims := req.RequestContext.Authorizer.JWT.Claims
	sub := strings.TrimSpace(claims["sub"])
	if sub == "" {
		return "", "", fmt.Errorf("missing sub")
	}
	email := strings.TrimSpace(claims["email"])
	return sub, email, nil
}

func jsonResp(status int, v any) (events.APIGatewayV2HTTPResponse, error) {
	b, _ := json.Marshal(v)
	return events.APIGatewayV2HTTPResponse{
		StatusCode: status,
		Headers: map[string]string{
			"content-type":                "application/json",
			"access-control-allow-origin": "*",
		},
		Body: string(b),
	}, nil
}

func errResp(status int, msg string) (events.APIGatewayV2HTTPResponse, error) {
	return jsonResp(status, map[string]any{
		"error": msg,
	})
}

func Transactions(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	table := db.TransactionsTableName()
	if strings.TrimSpace(table) == "" {
		return errResp(500, "TRANSACTIONS_TABLE is not set")
	}

	sub, email, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	client, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	awsCfg, _ := config.LoadDefaultConfig(ctx)
	snsClient := sns.NewFromConfig(awsCfg)

	// creates user topic + sends confirm email once
	users.EnsureUserEmailAlerts(ctx, client, snsClient, sub, email)

	switch req.RequestContext.HTTP.Method {
	case "GET":
		return listTransactions(ctx, client, table, sub, req)
	case "POST":
		return createTransaction(ctx, client, table, sub, req.Body)
	default:
		return errResp(405, "method not allowed")
	}
}

func listTransactions(ctx context.Context, client *dynamodb.Client, table, sub string, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	pk := fmt.Sprintf("USER#%s", sub)

	limit := int32(20)
	if s := strings.TrimSpace(req.QueryStringParameters["limit"]); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	var eks map[string]types.AttributeValue
	if token := strings.TrimSpace(req.QueryStringParameters["nextToken"]); token != "" {
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			return errResp(400, "invalid nextToken")
		}
		var m map[string]map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			return errResp(400, "invalid nextToken payload")
		}
		eks = map[string]types.AttributeValue{}
		for k, v := range m {
			if v["S"] != "" {
				eks[k] = &types.AttributeValueMemberS{Value: v["S"]}
			}
		}
	}

	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		ScanIndexForward:  aws.Bool(false),
		Limit:             aws.Int32(limit),
		ExclusiveStartKey: eks,
	})
	if err != nil {
		return errResp(500, "query failed")
	}

	var items []Transaction
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return errResp(500, "unmarshal failed")
	}

	var nextToken string
	if out.LastEvaluatedKey != nil && len(out.LastEvaluatedKey) > 0 {
		// encode as a tiny json map of {key: {S:"value"}} and base64url it
		m := map[string]map[string]string{}
		for k, av := range out.LastEvaluatedKey {
			if s, ok := av.(*types.AttributeValueMemberS); ok {
				m[k] = map[string]string{"S": s.Value}
			}
		}
		b, _ := json.Marshal(m)
		nextToken = base64.RawURLEncoding.EncodeToString(b)
	}

	return jsonResp(200, map[string]any{
		"items":     items,
		"nextToken": nextToken,
	})
}

func createTransaction(ctx context.Context, client *dynamodb.Client, table, sub, body string) (events.APIGatewayV2HTTPResponse, error) {
	var in CreateTransactionRequest
	if err := json.Unmarshal([]byte(body), &in); err != nil {
		return errResp(400, "invalid json body")
	}
	if in.Amount == 0 || strings.TrimSpace(in.Currency) == "" || strings.TrimSpace(in.Category) == "" {
		return errResp(400, "amount, currency, category are required")
	}

	now := time.Now().UTC()
	month := now.Format("2006-01") // YYYY-MM
	// SK can be time-based so sorting works
	sk := fmt.Sprintf("TX#%s", now.Format(time.RFC3339Nano))

	item := Transaction{
		PK: fmt.Sprintf("USER#%s", sub),
		SK: sk,

		GSI1PK: fmt.Sprintf("USER#%s#MONTH#%s", sub, month),
		GSI1SK: now.Format(time.RFC3339Nano),

		UserSub:   sub,
		Amount:    in.Amount,
		Currency:  strings.ToUpper(strings.TrimSpace(in.Currency)),
		Category:  strings.TrimSpace(in.Category),
		Note:      strings.TrimSpace(in.Note),
		CreatedAt: now.Format(time.RFC3339),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return errResp(500, "marshal failed")
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      av,
	})
	if err != nil {
		return errResp(500, "put failed")
	}

	return jsonResp(201, item)
}
