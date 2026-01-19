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

func UsersForShop(ctx context.Context, ddb *dynamodb.Client, shopDomain string) ([]string, error) {
	tbl := db.ShopToUserTableName()
	if strings.TrimSpace(tbl) == "" {
		return nil, fmt.Errorf("SHOP_TO_USER_TABLE not set")
	}

	pk := fmt.Sprintf("SHOP#%s", shopDomain)

	out, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tbl),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :u)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":u":  &types.AttributeValueMemberS{Value: "USER#"},
		},
	})
	if err != nil {
		return nil, err
	}

	var subs []string
	for _, it := range out.Items {
		if sk, ok := it["SK"].(*types.AttributeValueMemberS); ok {
			// SK=USER#sub
			s := strings.TrimPrefix(sk.Value, "USER#")
			if s != "" {
				subs = append(subs, s)
			}
		}
	}
	return subs, nil
}
