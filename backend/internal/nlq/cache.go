package nlq

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type CacheClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

type CacheKey struct {
	UserSub    string
	Shops      []string
	Question   string
	TodayISO   string
	MaxDays    int
	SchemaHash string // optional but helps invalidate when schema changes
}

type CachedResponse struct {
	SQL          string           `json:"sql"`
	Columns      []string         `json:"columns"`
	Rows         []map[string]any `json:"rows"`
	Assumptions  []string         `json:"assumptions"`
	Confidence   float64          `json:"confidence"`
	ScannedBytes int64            `json:"scanned_bytes"`
	ExecMs       int64            `json:"exec_ms"`
	QueryID      string           `json:"query_id"`
}

func cacheTable() (string, error) {
	t := strings.TrimSpace(os.Getenv("NLQ_CACHE_TABLE"))
	if t == "" {
		return "", fmt.Errorf("missing NLQ_CACHE_TABLE")
	}
	return t, nil
}

func cacheTTLSeconds() int64 {
	v := strings.TrimSpace(os.Getenv("NLQ_CACHE_TTL_SECONDS"))
	if v == "" {
		return 600
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 600
	}
	return n
}

func NormalizeQuestion(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	// collapse whitespace
	q = strings.Join(strings.Fields(q), " ")
	return q
}

func ShopsKey(shops []string) string {
	// stable order for hashing
	cp := append([]string(nil), shops...)
	// small sort
	return strings.Join(cp, ",")
}

func HashKeyMaterial(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func MakeCachePK(userSub string) string {
	return "USER#" + userSub
}

func MakeCacheSK(k CacheKey) string {
	qn := NormalizeQuestion(k.Question)
	material := strings.Join([]string{
		"shops=" + ShopsKey(k.Shops),
		"today=" + k.TodayISO,
		"maxdays=" + fmt.Sprintf("%d", k.MaxDays),
		"schema=" + k.SchemaHash,
		"q=" + qn,
	}, "|")
	return "NLQ#" + HashKeyMaterial(material)
}

func GetCached(ctx context.Context, ddb CacheClient, key CacheKey) (*CachedResponse, bool, error) {
	table, err := cacheTable()
	if err != nil {
		return nil, false, err
	}
	pk := MakeCachePK(key.UserSub)
	sk := MakeCacheSK(key)

	out, err := ddb.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]ddbtypes.AttributeValue{
			"PK": &ddbtypes.AttributeValueMemberS{Value: pk},
			"SK": &ddbtypes.AttributeValueMemberS{Value: sk},
		},
		ConsistentRead: aws.Bool(false),
	})
	if err != nil {
		return nil, false, fmt.Errorf("cache GetItem: %w", err)
	}
	if out.Item == nil || len(out.Item) == 0 {
		return nil, false, nil
	}

	payloadAttr, ok := out.Item["Payload"].(*ddbtypes.AttributeValueMemberS)
	if !ok {
		return nil, false, nil
	}
	var resp CachedResponse
	if err := json.Unmarshal([]byte(payloadAttr.Value), &resp); err != nil {
		return nil, false, nil
	}
	return &resp, true, nil
}

func PutCached(ctx context.Context, ddb CacheClient, key CacheKey, resp CachedResponse) error {
	table, err := cacheTable()
	if err != nil {
		return err
	}
	pk := MakeCachePK(key.UserSub)
	sk := MakeCacheSK(key)

	b, _ := json.Marshal(resp)

	now := time.Now().UTC().Unix()
	exp := now + cacheTTLSeconds()

	_, err = ddb.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item: map[string]ddbtypes.AttributeValue{
			"PK":        &ddbtypes.AttributeValueMemberS{Value: pk},
			"SK":        &ddbtypes.AttributeValueMemberS{Value: sk},
			"ExpiresAt": &ddbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", exp)},
			"Payload":   &ddbtypes.AttributeValueMemberS{Value: string(b)},
			"CreatedAt": &ddbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", now)},
		},
	})
	if err != nil {
		return fmt.Errorf("cache PutItem: %w", err)
	}
	return nil
}

func SchemaHash(schemaText string) string {
	return HashKeyMaterial(schemaText)
}
