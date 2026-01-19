package tenancy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DDBClient interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

func GetAllowedShopsByUserSub(ctx context.Context, ddb DDBClient, userSub string) ([]string, error) {
	userSub = strings.TrimSpace(userSub)
	if userSub == "" {
		return nil, fmt.Errorf("empty userSub")
	}

	table := strings.TrimSpace(os.Getenv("SHOP_TO_USER_TABLE"))
	if table == "" {
		return nil, fmt.Errorf("missing SHOP_TO_USER_TABLE")
	}

	indexName := strings.TrimSpace(os.Getenv("SHOP_TO_USER_GSI_USERSUB"))
	if indexName == "" {
		indexName = "GSI_UserSub"
	}

	out, err := ddb.Query(ctx, &dynamodb.QueryInput{
		TableName: aws.String(table),
		IndexName: aws.String(indexName),
		KeyConditionExpression: aws.String("#u = :u"),
		ExpressionAttributeNames: map[string]string{
			"#u": "UserSub",
			"#s": "Shop",
		},
		ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
			":u": &ddbtypes.AttributeValueMemberS{Value: userSub},
		},
		ProjectionExpression: aws.String("#s"),
	})
	if err != nil {
		return nil, fmt.Errorf("dynamodb query GSI_UserSub failed: %w", err)
	}

	shops := make([]string, 0, len(out.Items))
	for _, it := range out.Items {
		if v, ok := it["Shop"]; ok {
			if sv, ok2 := v.(*ddbtypes.AttributeValueMemberS); ok2 {
				val := strings.TrimSpace(sv.Value)
				if val != "" {
					shops = append(shops, val)
				}
			}
		}
	}
	return uniqueStrings(shops), nil
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		k := strings.ToLower(strings.TrimSpace(v))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}
