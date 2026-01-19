package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/internal/db"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func BackfillGSI(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	sub, _, err := userSub(req)
	if err != nil {
		return errResp(401, "unauthorized")
	}

	table := db.TransactionsTableName()
	if strings.TrimSpace(table) == "" {
		return errResp(500, "TRANSACTIONS_TABLE is not set")
	}

	client, err := db.NewDynamoClient(ctx)
	if err != nil {
		return errResp(500, "failed to init dynamodb")
	}

	// Query by PK to fetch user's items
	pk := fmt.Sprintf("USER#%s", sub)

	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		Limit: aws.Int32(200),
	})
	if err != nil {
		return errResp(500, "query failed")
	}

	updated := 0
	skipped := 0

	for _, item := range out.Items {
		// If already has GSI1PK, skip
		if _, ok := item["GSI1PK"]; ok {
			skipped++
			continue
		}

		createdAv, ok := item["CreatedAt"].(*types.AttributeValueMemberS)
		if !ok || strings.TrimSpace(createdAv.Value) == "" {
			// can't backfill without CreatedAt
			skipped++
			continue
		}

		// parse createdAt RFC3339
		t, err := time.Parse(time.RFC3339, createdAv.Value)
		if err != nil {
			skipped++
			continue
		}
		month := t.UTC().Format("2006-01")
		gsi1pk := fmt.Sprintf("USER#%s#MONTH#%s", sub, month)
		gsi1sk := t.UTC().Format(time.RFC3339Nano)

		// Need SK to update item
		skAv, ok := item["SK"].(*types.AttributeValueMemberS)
		if !ok || strings.TrimSpace(skAv.Value) == "" {
			skipped++
			continue
		}

		_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(table),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: skAv.Value},
			},
			UpdateExpression: aws.String("SET GSI1PK = :p, GSI1SK = :s"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":p": &types.AttributeValueMemberS{Value: gsi1pk},
				":s": &types.AttributeValueMemberS{Value: gsi1sk},
			},
		})
		if err != nil {
			// fail fast - easier to debug
			return errResp(500, "update failed")
		}

		updated++
	}

	return jsonResp(200, map[string]any{
		"updated": updated,
		"skipped": skipped,
		"note":    "Backfill only processes first 200 items in this simple version. Re-run if needed.",
	})
}
