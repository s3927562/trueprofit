// FILE: backend/internal/etl/daily_metrics.go
package etl

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// DailyMetricsRow matches the Glue table columns you created in Step 3.
type DailyMetricsRow struct {
	MerchantID       string  `parquet:"name=merchant_id, type=UTF8, encoding=PLAIN_DICTIONARY"`
	MetricDate       string  `parquet:"name=metric_date, type=UTF8"` // store as YYYY-MM-DD (Athena can cast to date)
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
// MVP behavior: Discover all distinct shops from SHOP_TO_USER_TABLE and write one Parquet row per shop for "today".
// (You will replace the metric calculations later with real aggregation from your transaction data.)
func (h *DailyMetricsETL) Handle(ctx context.Context, _ events.CloudWatchEvent) (map[string]any, error) {
	table := strings.TrimSpace(os.Getenv("SHOP_TO_USER_TABLE"))
	bucket := strings.TrimSpace(os.Getenv("ANALYTICS_BUCKET"))
	prefix := strings.TrimSpace(os.Getenv("DAILY_METRICS_PREFIX"))
	if prefix == "" {
		prefix = "daily_metrics/"
	}
	tzName := strings.TrimSpace(os.Getenv("ETL_TIMEZONE"))
	if tzName == "" {
		tzName = "Asia/Ho_Chi_Minh"
	}

	if table == "" {
		return nil, fmt.Errorf("missing env SHOP_TO_USER_TABLE")
	}
	if bucket == "" {
		return nil, fmt.Errorf("missing env ANALYTICS_BUCKET")
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("load timezone %s: %w", tzName, err)
	}
	now := time.Now().In(loc)
	dt := now.Format("2006-01-02") // partition dt=YYYY-MM-DD

	shops, err := h.listDistinctShops(ctx, table)
	if err != nil {
		return nil, err
	}
	if len(shops) == 0 {
		return map[string]any{"ok": true, "written": 0, "reason": "no shops found"}, nil
	}

	written := 0
	for _, shop := range shops {
		// For MVP, we write one row with zeros; replace with real aggregation later.
		row := DailyMetricsRow{
			MerchantID:       shop, // MVP: set merchant_id = shop
			MetricDate:       dt,
			GrossRevenue:     0,
			NetRevenue:       0,
			ProductCosts:     0,
			MarketingCosts:   0,
			FulfillmentCosts: 0,
			ProcessingFees:   0,
			OtherCosts:       0,
		}

		// Write to: daily_metrics/dt=YYYY-MM-DD/shop_id=<shop>/part-<rand>.parquet
		key := fmt.Sprintf("%sdt=%s/shop_id=%s/part-%s.parquet", ensureTrailingSlash(prefix), dt, shop, randHex(8))

		if err := h.writeOneParquetRowToS3(ctx, bucket, key, row); err != nil {
			return nil, fmt.Errorf("write parquet for shop=%s: %w", shop, err)
		}
		written++
	}

	return map[string]any{
		"ok":      true,
		"dt":      dt,
		"shops":   len(shops),
		"written": written,
		"bucket":  bucket,
		"prefix":  prefix,
	}, nil
}

// listDistinctShops scans SHOP_TO_USER_TABLE and extracts the "Shop" attribute.
// Your table items include: PK, SK, Shop, UserSub, CreatedAt (from your snippet).
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
						shops = append(shops, s) // ✅ preserve original
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

func bytesReader(b []byte) *bytesReadCloser {
	return &bytesReadCloser{b: b}
}

type bytesReadCloser struct {
	b []byte
	i int
}

func (r *bytesReadCloser) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
func (r *bytesReadCloser) Close() error { return nil }

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
