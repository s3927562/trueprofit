# Daily Metrics Pipeline

This repository contains scripts used to generate daily metrics data, format it into both Parquet and CSV, and upload the results to Amazon S3. It also includes utilities for creating dummy shop data in DynamoDB.

## Prerequisites

Ensure the following tools are installed and configured:

### 1. AWS CLI
- Install the AWS CLI.
- Run `aws configure` and provide your IAM **Access Key ID** and **Secret Access Key**.

### 2. Python
- Install Python (3.9+ recommended).
- Install dependencies:

```bash
pip install -r requirements.txt
```

## Usage

### Step 1 — Configure the Scripts
Each script contains configuration variables (AWS region, table names, S3 bucket name, etc.).
Update these values to match your environment before running.

### Step 2 — Run the Scripts in Order

Execute the scripts using:

```bash
python <script_file>
```

Run them in the following sequence:

1. **create_and_insert_dummy_shops.py**  
   Generates dummy shop entries (including base revenue) and inserts them into DynamoDB.
   Saves generated shop metadata to a local file for downstream jobs.

2. **generate_and_upload_daily_metrics.py**  
   Generates daily metrics using the saved dummy shops, exports both Parquet and CSV formats, and uploads them to S3.
