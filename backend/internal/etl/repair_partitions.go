package etl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

type PartitionRepair struct {
	ath *athena.Client
}

func NewPartitionRepair(cfg aws.Config) *PartitionRepair {
	return &PartitionRepair{ath: athena.NewFromConfig(cfg)}
}

func (h *PartitionRepair) Handle(ctx context.Context, _ events.CloudWatchEvent) (map[string]any, error) {
	db := strings.TrimSpace(os.Getenv("ATHENA_DATABASE"))
	wg := strings.TrimSpace(os.Getenv("ATHENA_WORKGROUP"))
	outS3 := strings.TrimSpace(os.Getenv("ATHENA_OUTPUT_S3"))
	table := strings.TrimSpace(os.Getenv("REPAIR_TABLE_NAME"))
	if table == "" {
		table = "daily_metrics"
	}

	if db == "" || wg == "" || outS3 == "" {
		return nil, fmt.Errorf("missing env: ATHENA_DATABASE/ATHENA_WORKGROUP/ATHENA_OUTPUT_S3")
	}

	sql := fmt.Sprintf("MSCK REPAIR TABLE %s", table)

	qid, err := startAthena(ctx, h.ath, sql, db, wg, outS3)
	if err != nil {
		return nil, err
	}

	state, reason, err := waitAthena(ctx, h.ath, qid, 120*time.Second, 900*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if state != athenatypes.QueryExecutionStateSucceeded {
		return nil, fmt.Errorf("repair failed: state=%s reason=%s qid=%s", state, reason, qid)
	}

	return map[string]any{
		"ok":    true,
		"table": table,
		"qid":   qid,
		"state": string(state),
	}, nil
}

func startAthena(ctx context.Context, c *athena.Client, sql, db, wg, outS3 string) (string, error) {
	out, err := c.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(sql),
		QueryExecutionContext: &athenatypes.QueryExecutionContext{
			Database: aws.String(db),
		},
		ResultConfiguration: &athenatypes.ResultConfiguration{
			OutputLocation: aws.String(outS3),
		},
		WorkGroup: aws.String(wg),
	})
	if err != nil {
		return "", fmt.Errorf("StartQueryExecution: %w", err)
	}
	return aws.ToString(out.QueryExecutionId), nil
}

func waitAthena(ctx context.Context, c *athena.Client, qid string, maxWait, poll time.Duration) (athenatypes.QueryExecutionState, string, error) {
	deadline := time.Now().Add(maxWait)

	for {
		if time.Now().After(deadline) {
			return athenatypes.QueryExecutionStateFailed, "timeout", fmt.Errorf("athena wait timeout qid=%s", qid)
		}
		out, err := c.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(qid),
		})
		if err != nil {
			return athenatypes.QueryExecutionStateFailed, "", fmt.Errorf("GetQueryExecution: %w", err)
		}
		st := out.QueryExecution.Status.State
		reason := aws.ToString(out.QueryExecution.Status.StateChangeReason)

		switch st {
		case athenatypes.QueryExecutionStateSucceeded,
			athenatypes.QueryExecutionStateFailed,
			athenatypes.QueryExecutionStateCancelled:
			return st, reason, nil
		default:
			time.Sleep(poll)
		}
	}
}
