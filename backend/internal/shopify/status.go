package shopify

import (
	"context"
	"fmt"
	"strings"

	"backend/internal/db"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// UpdateLastEvent updates per-user, per-shop "last event received" fields on the integrations item.
// PK = USER#<sub>
// SK = SHOPIFY#<shopDomain>
func UpdateLastEvent(ctx context.Context, ddb *dynamodb.Client, userSub, shopDomain, atISO, topic, webhookID string) error {
	tbl := strings.TrimSpace(db.IntegrationsTableName())
	if tbl == "" {
		return fmt.Errorf("INTEGRATIONS_TABLE not set")
	}
	if strings.TrimSpace(userSub) == "" || strings.TrimSpace(shopDomain) == "" {
		return fmt.Errorf("missing userSub/shopDomain")
	}

	pk := fmt.Sprintf("USER#%s", userSub)
	sk := fmt.Sprintf("SHOPIFY#%s", shopDomain)

	// Only set webhook id if present (avoid storing empty string forever).
	updateExpr := "SET LastEventAt=:a, LastEventTopic=:t"
	exprVals := map[string]types.AttributeValue{
		":a": &types.AttributeValueMemberS{Value: atISO},
		":t": &types.AttributeValueMemberS{Value: topic},
	}

	if strings.TrimSpace(webhookID) != "" {
		updateExpr += ", LastEventWebhookId=:w"
		exprVals[":w"] = &types.AttributeValueMemberS{Value: webhookID}
	}

	_, err := ddb.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tbl),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprVals,
	})
	return err
}
