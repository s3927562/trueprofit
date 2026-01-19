package shopify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"backend/internal/db"
	"backend/internal/security"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// IntegrationItem mirrors DynamoDB structure.
type IntegrationItem struct {
	PK             string `dynamodbav:"PK"`
	SK             string `dynamodbav:"SK"`
	Shop           string `dynamodbav:"Shop"`
	AccessTokenEnc string `dynamodbav:"AccessTokenEnc"`
	Scope          string `dynamodbav:"Scope"`
	CreatedAt      string `dynamodbav:"CreatedAt"`
	LastSyncAt     string `dynamodbav:"LastSyncAt,omitempty"`
}

// LoadIntegrationAndDecryptToken loads the integration record from DynamoDB
// and decrypts the access token. Returns (plainAccessToken, integrationItem, error).
func LoadIntegrationAndDecryptToken(ctx context.Context, sub, shopDomain string) (string, *IntegrationItem, error) {
	if sub == "" {
		return "", nil, errors.New("missing sub")
	}
	if shopDomain == "" {
		return "", nil, errors.New("missing shop domain")
	}

	intTable := db.IntegrationsTableName()
	if strings.TrimSpace(intTable) == "" {
		return "", nil, errors.New("INTEGRATIONS_TABLE not configured")
	}

	pk := fmt.Sprintf("USER#%s", sub)
	sk := fmt.Sprintf("SHOPIFY#%s", shopDomain)

	ddb, err := db.NewDynamoClient(ctx)
	if err != nil {
		return "", nil, err
	}

	out, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(intTable),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return "", nil, err
	}
	if out.Item == nil {
		return "", nil, fmt.Errorf("shop not connected: %s", shopDomain)
	}

	var integ IntegrationItem
	if err := attributevalue.UnmarshalMap(out.Item, &integ); err != nil {
		return "", nil, err
	}

	enc := strings.TrimSpace(integ.AccessTokenEnc)
	if enc == "" {
		return "", nil, errors.New("no AccessTokenEnc on record")
	}

	keyB64 := os.Getenv("TOKEN_ENC_KEY_B64")
	if keyB64 == "" {
		return "", nil, errors.New("TOKEN_ENC_KEY_B64 not set")
	}

	key, err := security.LoadKeyFromBase64(keyB64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid TOKEN_ENC_KEY_B64: %w", err)
	}

	token, err := security.DecryptAESGCM(key, enc)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decrypt token: %w", err)
	}

	return token, &integ, nil
}
