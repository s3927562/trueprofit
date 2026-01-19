package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"backend/internal/db"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type MonthlySummary struct {
	Month      string             `json:"month"`
	Currency   string             `json:"currency"`
	Income     float64            `json:"income"`
	Expense    float64            `json:"expense"`
	Net        float64            `json:"net"`
	ByCategory map[string]float64 `json:"byCategory"`
	Count      int                `json:"count"`
}

func SummaryMonthly(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	month := strings.TrimSpace(req.QueryStringParameters["month"])
	if month == "" || len(month) != 7 || month[4] != '-' {
		return errResp(400, "month is required in format YYYY-MM")
	}

	table := db.TransactionsTableName()
	if strings.TrimSpace(table) == "" {
		return errResp(500, "TRANSACTIONS_TABLE is not set")
	}

	client, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	gsiPk := fmt.Sprintf("USER#%s#MONTH#%s", sub, month)

	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: gsiPk},
		},
		Limit: aws.Int32(500),
	})
	if err != nil {
		return errResp(500, "query failed")
	}

	var items []Transaction
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &items); err != nil {
		return errResp(500, "unmarshal failed")
	}

	if len(items) == 0 {
		return jsonResp(200, MonthlySummary{
			Month:      month,
			Currency:   "USD",
			Income:     0,
			Expense:    0,
			Net:        0,
			ByCategory: map[string]float64{},
			Count:      0,
		})
	}

	// For simplicity assume all same currency; production: group by currency
	currency := items[0].Currency
	sum := MonthlySummary{
		Month:      month,
		Currency:   currency,
		ByCategory: map[string]float64{},
		Count:      len(items),
	}

	for _, t := range items {
		if t.Currency != currency {
			// keep it simple for now
			return errResp(400, "multiple currencies in month not supported yet")
		}
		if t.Amount >= 0 {
			sum.Income += t.Amount
		} else {
			sum.Expense += math.Abs(t.Amount)
		}
		sum.ByCategory[t.Category] += t.Amount
	}

	sum.Net = sum.Income - sum.Expense

	// normalize ByCategory: show net contribution per category
	// (income positive, expense negative) already handled by Amount
	return jsonResp(200, sum)
}

var _ = errors.New // keep linter happy if needed
var _ = json.Marshal
