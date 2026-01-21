package etl

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/writer"
)

// DailyMetricsRow matches the Glue table columns
type DailyMetricsRow struct {
	MerchantID       string  `parquet:"name=merchant_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	MetricDate       string  `parquet:"name=metric_date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"` // YYYY-MM-DD
	GrossRevenue     float64 `parquet:"name=gross_revenue, type=DOUBLE"`
	NetRevenue       float64 `parquet:"name=net_revenue, type=DOUBLE"`
	ProductCosts     float64 `parquet:"name=product_costs, type=DOUBLE"`
	MarketingCosts   float64 `parquet:"name=marketing_costs, type=DOUBLE"`
	FulfillmentCosts float64 `parquet:"name=fulfillment_costs, type=DOUBLE"`
	ProcessingFees   float64 `parquet:"name=processing_fees, type=DOUBLE"`
	OtherCosts       float64 `parquet:"name=other_costs, type=DOUBLE"`
}

type DailyMetricsETL struct {
	ddb *dynamodb.Client
	s3  *s3.Client
}

func NewDailyMetricsETL(cfg aws.Config) *DailyMetricsETL {
	return &DailyMetricsETL{
		ddb: dynamodb.NewFromConfig(cfg),
		s3:  s3.NewFromConfig(cfg),
	}
}

// Handle is triggered by EventBridge schedule.
//
// Behavior:
// - Discover shops from SHOP_TO_USER_TABLE
// - For each shop and each day in the backfill window, aggregate from TRANSACTIONS_TABLE
// - Write one Parquet row per (shop, dt) under:
//     daily_metrics/dt=YYYY-MM-DD/shop_id=<shop>/part-<rand>.parquet
//
// Env:
// - SHOP_TO_USER_TABLE (required)
// - TRANSACTIONS_TABLE (required)
// - ANALYTICS_BUCKET (required)
// - DAILY_METRICS_PREFIX (default "daily_metrics/")
// - ETL_TIMEZONE (default "Asia/Ho_Chi_Minh")
// - ETL_DAYS_BACK (default "1")  // number of days including today
func (h *DailyMetricsETL) Handle(ctx context.Context, _ events.CloudWatchEvent) (map[string]any, error) {
	mapTable := strings.TrimSpace(os.Getenv("SHOP_TO_USER_TABLE"))
	txTable := strings.TrimSpace(os.Getenv("TRANSACTIONS_TABLE"))

	bucket := strings.TrimSpace(os.Getenv("ANALYTICS_BUCKET"))
	prefix := strings.TrimSpace(os.Getenv("DAILY_METRICS_PREFIX"))
	if prefix == "" {
		prefix = "daily_metrics/"
	}

	tzName := strings.TrimSpace(os.Getenv("ETL_TIMEZONE"))
	if tzName == "" {
		tzName = "Asia/Ho_Chi_Minh"
	}

	daysBack := 1
	if v := strings.TrimSpace(os.Getenv("ETL_DAYS_BACK")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 90 {
			daysBack = n
		}
	}

	if mapTable == "" {
		return nil, fmt.Errorf("missing env SHOP_TO_USER_TABLE")
	}
	if txTable == "" {
		return nil, fmt.Errorf("missing env TRANSACTIONS_TABLE")
	}
	if bucket == "" {
		return nil, fmt.Errorf("missing env ANALYTICS_BUCKET")
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", tzName, err)
	}

	shops, err := h.listDistinctShops(ctx, mapTable)
	if err != nil {
		return nil, err
	}
	if len(shops) == 0 {
		return map[string]any{"ok": true, "written": 0, "reason": "no shops found"}, nil
	}

	now := time.Now().In(loc)
	written := 0
	totalTx := 0

	for i := 0; i < daysBack; i++ {
		day := now.AddDate(0, 0, -i)
		dtStr := day.Format("2006-01-02")

		for _, shop := range shops {
			gross, net, cnt, err := h.sumShopAmountsForDay(ctx, txTable, shop, dtStr)
			if err != nil {
				return nil, fmt.Errorf("sum tx for shop=%s dt=%s: %w", shop, dtStr, err)
			}

			// You asked to keep costs 0 for now.
			row := DailyMetricsRow{
				MerchantID:       shop, // MVP: merchant_id = shop
				MetricDate:       dtStr,
				GrossRevenue:     gross,
				NetRevenue:       net,
				ProductCosts:     0,
				MarketingCosts:   0,
				FulfillmentCosts: 0,
				ProcessingFees:   0,
				OtherCosts:       0,
			}

			key := fmt.Sprintf("%sdt=%s/shop_id=%s/part-%s.parquet",
				ensureTrailingSlash(prefix),
				dtStr,
				shop,
				randHex(8),
			)

			if err := h.writeOneParquetRowToS3(ctx, bucket, key, row); err != nil {
				return nil, fmt.Errorf("write parquet for shop=%s dt=%s: %w", shop, dtStr, err)
			}

			written++
			totalTx += cnt
		}
	}

	return map[string]any{
		"ok":        true,
		"shops":     len(shops),
		"days_back": daysBack,
		"written":   written,
		"tx_count":  totalTx,
		"bucket":    bucket,
		"prefix":    prefix,
	}, nil
}

// listDistinctShops scans SHOP_TO_USER_TABLE and extracts the "Shop" attribute.
func (h *DailyMetricsETL) listDistinctShops(ctx context.Context, table string) ([]string, error) {
	seen := map[string]bool{}
	shops := make([]string, 0, 64)

	var startKey map[string]ddbtypes.AttributeValue
	for {
		out, err := h.ddb.Scan(ctx, &dynamodb.ScanInput{
			TableName:            aws.String(table),
			ExclusiveStartKey:    startKey,
			ProjectionExpression: aws.String("#shop"),
			ExpressionAttributeNames: map[string]string{
				"#shop": "Shop",
			},
		})
		if err != nil {
			return nil, fmt.Errorf("dynamodb scan %s: %w", table, err)
		}

		for _, it := range out.Items {
			if v, ok := it["Shop"]; ok {
				if sv, ok2 := v.(*ddbtypes.AttributeValueMemberS); ok2 {
					s := strings.TrimSpace(sv.Value)
					if s == "" {
						continue
					}
					k := strings.ToLower(s)
					if !seen[k] {
						seen[k] = true
						shops = append(shops, s)
					}
				}
			}
		}

		if out.LastEvaluatedKey == nil || len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}
	return shops, nil
}

// sumShopAmountsForDay scans TRANSACTIONS_TABLE and sums Amount for one shop + one day.
// Works with your worker inserts:
// - Shop: "<domain>"
// - CreatedAt: RFC3339, so begins_with("YYYY-MM-DD") works
// - Amount: N string (positive sale / negative refund)
func (h *DailyMetricsETL) sumShopAmountsForDay(ctx context.Context, txTable, shop, dayYYYYMMDD string) (gross float64, net float64, count int, err error) {
	var startKey map[string]ddbtypes.AttributeValue

	for {
		out, err := h.ddb.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(txTable),
			ExclusiveStartKey: startKey,

			FilterExpression: aws.String("#shop = :shop AND begins_with(#createdAt, :day)"),
			ExpressionAttributeNames: map[string]string{
				"#shop":      "Shop",
				"#createdAt": "CreatedAt",
				"#amount":    "Amount",
			},
			ExpressionAttributeValues: map[string]ddbtypes.AttributeValue{
				":shop": &ddbtypes.AttributeValueMemberS{Value: shop},
				":day":  &ddbtypes.AttributeValueMemberS{Value: dayYYYYMMDD},
			},
			ProjectionExpression: aws.String("#shop, #createdAt, #amount"),
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("scan tx table: %w", err)
		}

		for _, it := range out.Items {
			av, ok := it["Amount"]
			if !ok {
				continue
			}
			nv, ok := av.(*ddbtypes.AttributeValueMemberN)
			if !ok {
				continue
			}
			amt, perr := strconv.ParseFloat(nv.Value, 64)
			if perr != nil {
				continue
			}

			if amt > 0 {
				gross += amt
			}
			net += amt
			count++
		}

		if out.LastEvaluatedKey == nil || len(out.LastEvaluatedKey) == 0 {
			break
		}
		startKey = out.LastEvaluatedKey
	}

	return gross, net, count, nil
}

func (h *DailyMetricsETL) writeOneParquetRowToS3(ctx context.Context, bucket, key string, row DailyMetricsRow) error {
	tmpDir := os.TempDir()
	localPath := filepath.Join(tmpDir, "daily_metrics_"+randHex(8)+".parquet")

	fw, err := local.NewLocalFileWriter(localPath)
	if err != nil {
		return fmt.Errorf("parquet file writer: %w", err)
	}

	pw, err := writer.NewParquetWriter(fw, new(DailyMetricsRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("parquet writer: %w", err)
	}
	pw.RowGroupSize = 128 * 1024 * 1024
	pw.PageSize = 8 * 1024
	pw.CompressionType = 0 // no snappy

	if err := pw.Write(row); err != nil {
		_ = pw.WriteStop()
		_ = fw.Close()
		return fmt.Errorf("parquet write row: %w", err)
	}
	if err := pw.WriteStop(); err != nil {
		_ = fw.Close()
		return fmt.Errorf("parquet write stop: %w", err)
	}
	if err := fw.Close(); err != nil {
		return fmt.Errorf("parquet close: %w", err)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read parquet tmp: %w", err)
	}
	defer func() { _ = os.Remove(localPath) }()

	_, err = h.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
		ACL:         s3types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("s3 putobject failed: %w", err)
	}
	return nil
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

func randHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
