package etl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
)

type Resp struct {
	Ok        bool   `json:"ok"`
	QueryID   string `json:"query_id,omitempty"`
	State     string `json:"state,omitempty"`
	Database  string `json:"database,omitempty"`
	Table     string `json:"table,omitempty"`
	Workgroup string `json:"workgroup,omitempty"`
	Output    string `json:"output,omitempty"`
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context) (Resp, error) {
	db := strings.TrimSpace(os.Getenv("ATHENA_DATABASE"))
	table := strings.TrimSpace(os.Getenv("ATHENA_TABLE"))
	workgroup := strings.TrimSpace(os.Getenv("ATHENA_WORKGROUP"))
	output := strings.TrimSpace(os.Getenv("ATHENA_OUTPUT")) // s3://bucket/prefix/

	if db == "" || table == "" || output == "" {
		return Resp{Ok: false}, fmt.Errorf("missing env: ATHENA_DATABASE, ATHENA_TABLE, ATHENA_OUTPUT are required")
	}
	if !strings.HasPrefix(output, "s3://") {
		return Resp{Ok: false}, fmt.Errorf("ATHENA_OUTPUT must start with s3://")
	}
	if workgroup == "" {
		workgroup = "primary"
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Resp{Ok: false}, err
	}
	ath := athena.NewFromConfig(cfg)

	q := fmt.Sprintf("MSCK REPAIR TABLE %s;", table)

	startOut, err := ath.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString: aws.String(q),
		QueryExecutionContext: &athenatypes.QueryExecutionContext{
			Database: aws.String(db),
		},
		WorkGroup: aws.String(workgroup),
		ResultConfiguration: &athenatypes.ResultConfiguration{
			OutputLocation: aws.String(output),
		},
	})
	if err != nil {
		return Resp{Ok: false}, fmt.Errorf("StartQueryExecution: %w", err)
	}

	qid := aws.ToString(startOut.QueryExecutionId)
	fmt.Printf("repair started: qid=%s db=%s table=%s wg=%s out=%s\n", qid, db, table, workgroup, output)

	// Poll until completion (short timeout)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		st, err := ath.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{
			QueryExecutionId: aws.String(qid),
		})
		if err != nil {
			return Resp{Ok: false, QueryID: qid}, fmt.Errorf("GetQueryExecution: %w", err)
		}
		state := string(st.QueryExecution.Status.State)
		if state == "SUCCEEDED" {
			fmt.Printf("repair succeeded: qid=%s\n", qid)
			return Resp{
				Ok:        true,
				QueryID:   qid,
				State:     state,
				Database:  db,
				Table:     table,
				Workgroup: workgroup,
				Output:    output,
			}, nil
		}
		if state == "FAILED" || state == "CANCELLED" {
			reason := ""
			if st.QueryExecution.Status.StateChangeReason != nil {
				reason = *st.QueryExecution.Status.StateChangeReason
			}
			return Resp{Ok: false, QueryID: qid, State: state}, fmt.Errorf("repair %s: %s", state, reason)
		}
		time.Sleep(2 * time.Second)
	}

	return Resp{Ok: false, QueryID: qid, State: "TIMEOUT"}, fmt.Errorf("repair timed out waiting for qid=%s", qid)
}
