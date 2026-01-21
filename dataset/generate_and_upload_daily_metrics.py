# generate_and_upload_daily_metrics.py
#
# Reads dummy_shops.txt (format: "<shop_domain>,<base_revenue>")
# Generates daily_metrics rows matching Glue schema
# then uploads Parquet partitions to S3 under:
#   s3://<BUCKET>/<S3_PREFIX>/dt=YYYY-MM-DD/shop_id=<shop>/part-....parquet

import os
import uuid
from datetime import datetime, timedelta

import boto3
import numpy as np
import pandas as pd
import pyarrow as pa
import pyarrow.parquet as pq

# CONFIG
AWS_REGION = "us-east-1"

S3_BUCKET = "trueprofit-analytics-dev-893677978594"
S3_PREFIX = "daily_metrics"

ATHENA_DATABASE = "trueprofit_analytics_dev"
ATHENA_TABLE = "daily_metrics"
ATHENA_WORKGROUP = "trueprofit-dev"
ATHENA_OUTPUT_S3 = f"s3://{S3_BUCKET}/athena-results/"

# Input shop file produced by create_dummy_shops.py
# Each line: shop_domain,base_revenue
SHOPS_FILE = "dummy_shops.txt"

# Local output
OUT_DIR = "daily_metrics"
CSV_NAME = "daily_metrics.csv"
PARQUET_DIRNAME = "parquet"

# Data generation knobs
DAYS = 90
NOISE_STD = 0.2
WEEKEND_FACTOR = 0.85

# Seasonal multipliers
SEASONAL = {
    11: 1.6,
    12: 1.4,
    1: 0.9,
    7: 0.85,
    8: 0.85,
}

# Cost ratio ranges (as portion of gross)
PRODUCT_COST_RANGE = (0.35, 0.45)
MARKETING_COST_RANGE = (0.15, 0.25)
FULFILLMENT_COST_RANGE = (0.08, 0.12)
PROCESSING_FEES_RANGE = (0.025, 0.035)
OTHER_COST_RANGE = (0.00, 0.03)

def read_shops_with_base_revenue(path: str):
    shops = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = [p.strip() for p in line.split(",")]
            if len(parts) != 2:
                raise ValueError(f"Invalid line in {path}: {line!r} (expected 'shop,base_revenue')")
            shop = parts[0]
            base_rev = float(parts[1])
            shops.append((shop, base_rev))
    if not shops:
        raise ValueError(f"No shops found in {path}")
    return shops


def ensure_dir(p: str):
    os.makedirs(p, exist_ok=True)


def generate_rows_for_shop_day(shop: str, base_rev: float, day):
    # day is a date object
    seasonal_factor = SEASONAL.get(day.month, 1.0)
    weekend_factor = WEEKEND_FACTOR if day.weekday() >= 5 else 1.0

    gross = base_rev * seasonal_factor * weekend_factor
    gross *= (1 + np.random.normal(0, NOISE_STD))
    gross = max(gross, 0.0)

    product_costs = gross * np.random.uniform(*PRODUCT_COST_RANGE)
    marketing_costs = gross * np.random.uniform(*MARKETING_COST_RANGE)
    fulfillment_costs = gross * np.random.uniform(*FULFILLMENT_COST_RANGE)
    processing_fees = gross * np.random.uniform(*PROCESSING_FEES_RANGE)
    other_costs = gross * np.random.uniform(*OTHER_COST_RANGE)

    net = gross - (product_costs + marketing_costs + fulfillment_costs + processing_fees + other_costs)

    day_str = day.strftime("%Y-%m-%d")
    return {
        # Glue columns
        "merchant_id": shop,
        "metric_date": day_str,
        "gross_revenue": round(gross, 2),
        "net_revenue": round(net, 2),
        "product_costs": round(product_costs, 2),
        "marketing_costs": round(marketing_costs, 2),
        "fulfillment_costs": round(fulfillment_costs, 2),
        "processing_fees": round(processing_fees, 2),
        "other_costs": round(other_costs, 2),

        # Partition helpers (not part of Glue columns)
        "dt": day_str,
        "shop_id": shop,
    }


def df_to_parquet_partitioned_and_upload(df: pd.DataFrame, local_parquet_root: str, s3_bucket: str, s3_prefix: str):
    """
    Writes parquet files to:
      <local_parquet_root>/dt=YYYY-MM-DD/shop_id=<shop>/part-....parquet
    Then uploads to:
      s3://bucket/<s3_prefix>/dt=.../shop_id=.../part-....parquet
    """
    s3 = boto3.client("s3", region_name=AWS_REGION)
    uploaded = 0

    # Ensure prefix formatting
    prefix = s3_prefix.strip().strip("/")
    # Local root
    ensure_dir(local_parquet_root)

    # Only write Glue columns into parquet (NOT dt/shop_id)
    glue_cols = [
        "merchant_id",
        "metric_date",
        "gross_revenue",
        "net_revenue",
        "product_costs",
        "marketing_costs",
        "fulfillment_costs",
        "processing_fees",
        "other_costs",
    ]

    grouped = df.groupby(["dt", "shop_id"], sort=False)
    for (dt_str, shop), g in grouped:
        part_dir = os.path.join(local_parquet_root, f"dt={dt_str}", f"shop_id={shop}")
        ensure_dir(part_dir)

        fname = f"part-{uuid.uuid4().hex[:12]}.parquet"
        local_path = os.path.join(part_dir, fname)

        table = pa.Table.from_pandas(g[glue_cols], preserve_index=False)
        pq.write_table(table, local_path, compression=None)

        s3_key = f"{prefix}/dt={dt_str}/shop_id={shop}/{fname}" if prefix else f"dt={dt_str}/shop_id={shop}/{fname}"
        s3.upload_file(local_path, s3_bucket, s3_key)
        uploaded += 1

    return uploaded


def run_athena_query(sql: str, db: str, workgroup: str, output_s3: str, region: str, timeout: int = 120) -> str:
    ath = boto3.client("athena", region_name=region)

    start = ath.start_query_execution(
        QueryString=sql,
        QueryExecutionContext={"Database": db},
        ResultConfiguration={"OutputLocation": output_s3},
        WorkGroup=workgroup,
    )
    qid = start["QueryExecutionId"]

    # poll
    deadline = datetime.now().timestamp() + timeout
    while datetime.now().timestamp() < deadline:
        res = ath.get_query_execution(QueryExecutionId=qid)
        state = res["QueryExecution"]["Status"]["State"]
        if state in ("SUCCEEDED", "FAILED", "CANCELLED"):
            if state != "SUCCEEDED":
                reason = res["QueryExecution"]["Status"].get("StateChangeReason", "")
                raise RuntimeError(f"Athena query {state}: {reason} (qid={qid})")
            return qid
        # ~1s sleep without importing time heavily (tiny)
        import time
        time.sleep(1)

    raise RuntimeError(f"Athena query timed out (qid={qid})")


def main():
    # Read shops + base revenue
    shops = read_shops_with_base_revenue(SHOPS_FILE)

    # Generate dataframe
    today = datetime.now().date()
    rows = []

    for shop, base_rev in shops:
        for i in range(DAYS):
            day = today - timedelta(days=i)
            rows.append(generate_rows_for_shop_day(shop, base_rev, day))

    df = pd.DataFrame(rows)

    # Local outputs
    ensure_dir(OUT_DIR)

    # Save CSV (readable) including partitions to help debugging
    csv_path = os.path.join(OUT_DIR, CSV_NAME)
    df.to_csv(csv_path, index=False)
    print("Saved CSV:", csv_path)

    # Save parquet partitioned + upload to S3
    local_parquet_root = os.path.join(OUT_DIR, PARQUET_DIRNAME)
    uploaded = df_to_parquet_partitioned_and_upload(df, local_parquet_root, S3_BUCKET, S3_PREFIX)

    print("Saved Parquet partitions locally:", local_parquet_root)
    print(f"Uploaded {uploaded} Parquet objects to s3://{S3_BUCKET}/{S3_PREFIX}/")

    # Run repair
    repair_sql = f"MSCK REPAIR TABLE {ATHENA_TABLE};"
    qid = run_athena_query(repair_sql, ATHENA_DATABASE, ATHENA_WORKGROUP, ATHENA_OUTPUT_S3, AWS_REGION)
    print(f"Ran Athena repair: {repair_sql}  (qid={qid})")


if __name__ == "__main__":
    main()
