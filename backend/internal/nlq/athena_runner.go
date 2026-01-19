package nlq

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

type AthenaClient interface {
	StartQueryExecution(ctx context.Context, params *athena.StartQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.StartQueryExecutionOutput, error)
	GetQueryExecution(ctx context.Context, params *athena.GetQueryExecutionInput, optFns ...func(*athena.Options)) (*athena.GetQueryExecutionOutput, error)
	GetQueryResults(ctx context.Context, params *athena.GetQueryResultsInput, optFns ...func(*athena.Options)) (*athena.GetQueryResultsOutput, error)
}

type AthenaRunOptions struct {
	Database       string
	Workgroup      string
	OutputLocation string // s3://.../athena-results/
	MaxWait        time.Duration
	PollInterval   time.Duration
	MaxResultRows  int // safety
	MaxResultBytes int // (not enforced in API; reserved)
}

type AthenaResult struct {
	QueryExecutionID string
	Columns          []string
	Rows             []map[string]any
	ScannedBytes     int64
	ExecutionMs      int64
}

type AthenaError struct {
	State            string
	Reason           string
	QueryExecutionID string
}

func (e *AthenaError) Error() string {
	if e.QueryExecutionID != "" {
		return fmt.Sprintf("athena %s: %s (qid=%s)", e.State, e.Reason, e.QueryExecutionID)
	}
	return fmt.Sprintf("athena %s: %s", e.State, e.Reason)
}

func RunAthenaQuery(ctx context.Context, c AthenaClient, sql string, opt AthenaRunOptions) (*AthenaResult, error) {
	if strings.TrimSpace(opt.Database) == "" {
		return nil, fmt.Errorf("missing athena database")
	}
	if strings.TrimSpace(opt.Workgroup) == "" {
		return nil, fmt.Errorf("missing athena workgroup")
	}
	if strings.TrimSpace(opt.OutputLocation) == "" {
		return nil, fmt.Errorf("missing athena output location")
	}
	if opt.MaxWait == 0 {
		opt.MaxWait = 25 * time.Second
	}
	if opt.PollInterval == 0 {
		opt.PollInterval = 700 * time.Millisecond
	}
	if opt.MaxResultRows == 0 {
		opt.MaxResultRows = 200
	}

	startOut, err := c.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(sql),
		QueryExecutionContext: &athenatypes.QueryExecutionContext{
			Database: aws.String(opt.Database),
		},
		ResultConfiguration: &athenatypes.ResultConfiguration{
			OutputLocation: aws.String(opt.OutputLocation),
		},
		WorkGroup: aws.String(opt.Workgroup),
	})
	if err != nil {
		return nil, fmt.Errorf("athena StartQueryExecution: %w", err)
	}
	qid := aws.ToString(startOut.QueryExecutionId)

	// Poll status
	deadline := time.Now().Add(opt.MaxWait)
	var exec *athenatypes.QueryExecution
	for {
		if time.Now().After(deadline) {
			return nil, &AthenaError{State: "TIMEOUT", Reason: "query timed out", QueryExecutionID: qid}
		}
		getOut, err := c.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(qid),
		})
		if err != nil {
			return nil, fmt.Errorf("athena GetQueryExecution: %w", err)
		}
		exec = getOut.QueryExecution
		state := exec.Status.State

		switch state {
		case athenatypes.QueryExecutionStateSucceeded:
			goto RESULTS
		case athenatypes.QueryExecutionStateFailed, athenatypes.QueryExecutionStateCancelled:
			reason := aws.ToString(exec.Status.StateChangeReason)
			return nil, &AthenaError{State: string(state), Reason: reason, QueryExecutionID: qid}
		default:
			time.Sleep(opt.PollInterval)
		}
	}

RESULTS:
	// Fetch results (first row is headers)
	var (
		nextToken *string
		allRows   []athenatypes.Row
		colInfo   []athenatypes.ColumnInfo
	)

	for {
		resOut, err := c.GetQueryResults(ctx, &athena.GetQueryResultsInput{
			QueryExecutionId: aws.String(qid),
			NextToken:        nextToken,
			MaxResults:       aws.Int32(1000),
		})
		if err != nil {
			return nil, fmt.Errorf("athena GetQueryResults: %w", err)
		}
		if colInfo == nil {
			colInfo = resOut.ResultSet.ResultSetMetadata.ColumnInfo
		}
		allRows = append(allRows, resOut.ResultSet.Rows...)
		if resOut.NextToken == nil || aws.ToString(resOut.NextToken) == "" {
			break
		}
		nextToken = resOut.NextToken

		// Safety: avoid huge result pulls
		if len(allRows) > opt.MaxResultRows+5 {
			break
		}
	}

	cols := make([]string, 0, len(colInfo))
	for _, c := range colInfo {
		cols = append(cols, aws.ToString(c.Name))
	}

	// Convert to rows of map[col]=value
	// Athena returns header row as first row
	outRows := make([]map[string]any, 0, minInt(opt.MaxResultRows, maxInt(0, len(allRows)-1)))

	for i, r := range allRows {
		if i == 0 {
			continue // header row
		}
		if len(outRows) >= opt.MaxResultRows {
			break
		}

		m := map[string]any{}
		for ci, d := range r.Data {
			if ci >= len(cols) {
				continue
			}
			v := aws.ToString(d.VarCharValue)
			m[cols[ci]] = coerceScalar(v)
		}
		outRows = append(outRows, m)
	}

	var scanned int64
	var execMs int64
	if exec != nil {
		if exec.Statistics != nil {
			scanned = aws.ToInt64(exec.Statistics.DataScannedInBytes)
			execMs = aws.ToInt64(exec.Statistics.EngineExecutionTimeInMillis)
		}
	}

	return &AthenaResult{
		QueryExecutionID: qid,
		Columns:          cols,
		Rows:             outRows,
		ScannedBytes:     scanned,
		ExecutionMs:      execMs,
	}, nil
}

func coerceScalar(v string) any {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	// Try int
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	// Try float
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	// Keep string
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
