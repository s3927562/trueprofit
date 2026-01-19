package users

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"backend/internal/db"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

func UserPK(sub string) string {
	return fmt.Sprintf("USER#%s", sub)
}

func shortHashSub(sub string) string {
	h := sha1.Sum([]byte(sub))
	// 8 bytes -> 16 hex chars, stable and short
	return hex.EncodeToString(h[:8])
}

// EnsureUserEmailAlerts ensures:
// - an SNS topic exists for the user
// - an email subscription exists for the user's email (user confirms once)
// - Users table stores AlertsTopicArn
//
// Returns topicArn.
func EnsureUserEmailAlerts(ctx context.Context, ddb *dynamodb.Client, snsClient *sns.Client, sub, email string) (string, error) {
	sub = strings.TrimSpace(sub)
	email = strings.TrimSpace(email)

	if sub == "" || email == "" {
		return "", nil
	}

	stage := strings.TrimSpace(os.Getenv("ALERTS_STAGE"))
	if stage == "" {
		stage = "dev"
	}

	// If already stored, reuse
	existing, _ := GetAlertsTopicArn(ctx, ddb, sub)
	if existing != "" {
		return existing, nil
	}

	// SNS topic names must be simple (no slashes, etc.)
	topicName := fmt.Sprintf("trueprofit-user-alerts-%s-%s", stage, shortHashSub(sub))

	ct, err := snsClient.CreateTopic(ctx, &sns.CreateTopicInput{
		Name: aws.String(topicName),
	})
	if err != nil {
		return "", err
	}
	topicArn := aws.ToString(ct.TopicArn)

	// Subscribe email (requires confirm link click once)
	_, err = snsClient.Subscribe(ctx, &sns.SubscribeInput{
		TopicArn: aws.String(topicArn),
		Protocol: aws.String("email"),
		Endpoint: aws.String(email),
	})
	if err != nil {
		return "", err
	}

	// Save to Users table (also store email)
	tbl := strings.TrimSpace(db.UsersTableName())
	if tbl != "" {
		_, _ = ddb.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(tbl),
			Item: map[string]types.AttributeValue{
				"PK":             &types.AttributeValueMemberS{Value: UserPK(sub)},
				"Email":          &types.AttributeValueMemberS{Value: email},
				"AlertsTopicArn": &types.AttributeValueMemberS{Value: topicArn},
				"UpdatedAt":      &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			},
		})
	}

	return topicArn, nil
}

func GetAlertsTopicArn(ctx context.Context, ddb *dynamodb.Client, sub string) (string, error) {
	tbl := strings.TrimSpace(db.UsersTableName())
	if tbl == "" || strings.TrimSpace(sub) == "" {
		return "", nil
	}

	out, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tbl),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: UserPK(sub)},
		},
	})
	if err != nil || out.Item == nil {
		return "", err
	}

	if v, ok := out.Item["AlertsTopicArn"].(*types.AttributeValueMemberS); ok {
		return v.Value, nil
	}
	return "", nil
}
