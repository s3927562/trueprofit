package shopify

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func DedupeTable() string {
	return os.Getenv("SHOPIFY_WEBHOOK_DEDUPE_TABLE")
}

// Returns (isDuplicate, error). If duplicate, caller should exit early.
func ClaimWebhook(ctx context.Context, ddb *dynamodb.Client, webhookID, shopDomain, topic string) (bool, error) {
	tbl := strings.TrimSpace(DedupeTable())
	if tbl == "" {
		// If not configured, don't block processing
		return false, nil
	}
	webhookID = strings.TrimSpace(webhookID)
	if webhookID == "" {
		return false, nil
	}

	// TTL: keep dedupe records for 7 days
	exp := time.Now().UTC().Add(7 * 24 * time.Hour).Unix()

	_, err := ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tbl),
		Item: map[string]types.AttributeValue{
			"PK":        &types.AttributeValueMemberS{Value: fmt.Sprintf("WH#%s", webhookID)},
			"Shop":      &types.AttributeValueMemberS{Value: shopDomain},
			"Topic":     &types.AttributeValueMemberS{Value: topic},
			"CreatedAt": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			"ExpiresAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", exp)},
		},
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})

	if err != nil {
		// Conditional check failed => already processed
		var cfe *types.ConditionalCheckFailedException
		if ok := errorAs(err, &cfe); ok {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

func errorAs(err error, target any) bool {
	switch t := target.(type) {
	case **types.ConditionalCheckFailedException:
		_, ok := err.(*types.ConditionalCheckFailedException)
		if ok {
			*t = err.(*types.ConditionalCheckFailedException)
		}
		return ok
	default:
		return false
	}
}
