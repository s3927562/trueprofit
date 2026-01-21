# TrueProfit

The goal of this project is to create and deploy an AWS serverless analytics system that resembles the TrueProfit application. The system uses cloud-native services to provide analytics, calculate real profit metrics for online retailers, and ingest transaction and cost data. Users can query business data in plain English thanks to the AI-powered Text-to-SQL (Structured Query Language) feature.

You can use the application [here](https://d1gxtjiutpotmx.cloudfront.net).

## Repository structure

- `frontend-react/`: Vite + TypeScript web app
- `frontend-infra/`: Serverless Framework to deploy S3 + CloudFront that is used to host the web app
- `backend/`: Serverless Framework to deploy all services required + Go scripts for Lambda functions
- `dataset/`: Scripts to generate and upload testing data + testing data used for demo

---

## AWS Services Used

### Main architecture

- `API Gateway`: Backend endpoint to call Lambda functions
- `CloudFront`: Fast and secure access to the web app stored on S3
- `Cognito`: User sign-up and sign-in
- `DynamoDB`: Store transactions, Shopify integrations, and webhook deduplication
- `EventBridge`: Receive webhooks from Shopify and push to SQS queues
- `Lambda`: Run processing functions for new orders, refunds, and emails
- `S3`: Store the static web app
- `SNS`: Email users on new orders or refunds
- `SQS`: Push messages to Lambda functions
- `Systems Manager Parameter Store`: Store API Gateway link for Lambda functions to reference during runtime

### Advanced Feature (Text-to-SQL)

- `Athena`: Execute SQL queries on Glue database
- `Bedrock`: Create embeddings and generate SQL
- `Glue`: Store dataset metadata from S3 objects
- `Lambda`: Gather transactions from DynamoDB daily, generate final daily metrics, process user queries and send them to Bedrock
- `S3`: Store the main dataset
