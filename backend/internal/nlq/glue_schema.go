package nlq

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
)

type GlueClient interface {
	GetTable(ctx context.Context, params *glue.GetTableInput, optFns ...func(*glue.Options)) (*glue.GetTableOutput, error)
}

type TableSchema struct {
	Database   string
	Table      string
	Location   string
	Columns    []Column
	Partitions []Column
}

type Column struct {
	Name string
	Type string
}

func LoadTableSchemaFromEnv(ctx context.Context, c GlueClient) (*TableSchema, error) {
	db := strings.TrimSpace(os.Getenv("GLUE_DATABASE"))
	tbl := strings.TrimSpace(os.Getenv("DAILY_METRICS_TABLE"))
	if db == "" || tbl == "" {
		return nil, fmt.Errorf("missing env vars: GLUE_DATABASE and/or DAILY_METRICS_TABLE")
	}
	return LoadTableSchema(ctx, c, db, tbl)
}

func LoadTableSchema(ctx context.Context, c GlueClient, database, table string) (*TableSchema, error) {
	out, err := c.GetTable(ctx, &glue.GetTableInput{
		DatabaseName: aws.String(database),
		Name:         aws.String(table),
	})
	if err != nil {
		return nil, fmt.Errorf("glue GetTable %s.%s: %w", database, table, err)
	}

	ti := out.Table
	sd := ti.StorageDescriptor

	schema := &TableSchema{
		Database: database,
		Table:    aws.ToString(ti.Name),
		Location: aws.ToString(sd.Location),
	}

	cols := make([]Column, 0, len(sd.Columns))
	for _, col := range sd.Columns {
		cols = append(cols, Column{
			Name: aws.ToString(col.Name),
			Type: aws.ToString(col.Type),
		})
	}
	schema.Columns = cols

	parts := make([]Column, 0, len(ti.PartitionKeys))
	for _, p := range ti.PartitionKeys {
		parts = append(parts, Column{
			Name: aws.ToString(p.Name),
			Type: aws.ToString(p.Type),
		})
	}
	schema.Partitions = parts

	// Make prompt stable across runs
	sort.Slice(schema.Columns, func(i, j int) bool { return schema.Columns[i].Name < schema.Columns[j].Name })
	sort.Slice(schema.Partitions, func(i, j int) bool { return schema.Partitions[i].Name < schema.Partitions[j].Name })

	return schema, nil
}

// CompactSchemaText returns a prompt-ready schema block, e.g.:
//
// DATABASE trueprofit_analytics_dev
// TABLE daily_metrics ( ... )
// PARTITIONED BY (dt date, shop_id string)
// LOCATION s3://...
func CompactSchemaText(s *TableSchema) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("DATABASE %s\n", s.Database))
	b.WriteString(fmt.Sprintf("TABLE %s (\n", s.Table))

	for i, c := range s.Columns {
		comma := ","
		if i == len(s.Columns)-1 {
			comma = ""
		}
		b.WriteString(fmt.Sprintf("  %s %s%s\n", c.Name, c.Type, comma))
	}
	b.WriteString(")\n")

	if len(s.Partitions) > 0 {
		b.WriteString("PARTITIONED BY (")
		for i, p := range s.Partitions {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s %s", p.Name, p.Type))
		}
		b.WriteString(")\n")
	}

	if s.Location != "" {
		b.WriteString(fmt.Sprintf("LOCATION %s\n", s.Location))
	}

	return b.String()
}

// Optional: Glue column types sometimes include complex types;
func NormalizeGlueType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	// keep as-is for MVP
	return t
}

// (Optional) you can convert AWS Glue types to Athena-friendly types here if needed later.
func glueColumnToPromptColumn(c gluetypes.Column) Column {
	return Column{Name: aws.ToString(c.Name), Type: NormalizeGlueType(aws.ToString(c.Type))}
}
