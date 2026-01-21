#!/usr/bin/env bash

set -euo pipefail

APP_STAGE="${APP_STAGE:-dev}"   # or read from .env, or pass manually
AWS_REGION="${AWS_REGION:-us-east-1}"

get_export () {
  aws cloudformation list-exports \
    --region "$AWS_REGION" \
    --query "Exports[?Name=='$1'].Value | [0]" \
    --output text
}

echo "Fetching CloudFormation exports for stage: $APP_STAGE..."

CF_URL="$(get_export "TrueProfit-CloudFrontURL-${APP_STAGE}")"
BUCKET="$(get_export "TrueProfit-FrontendBucketName-${APP_STAGE}")"
DIST_ID="$(get_export "TrueProfit-CloudFrontDistributionId-${APP_STAGE}")"

API_BASE="$(get_export "TrueProfit-ApiBaseUrl-${APP_STAGE}")"
COGNITO_DOMAIN="$(get_export "TrueProfit-CognitoHostedDomainURL-${APP_STAGE}")"
USER_POOL_ID="$(get_export "TrueProfit-CognitoUserPoolId-${APP_STAGE}")"
CLIENT_ID="$(get_export "TrueProfit-CognitoUserPoolClientId-${APP_STAGE}")"

echo "Writing .env file..."
cat > .env <<EOF
VITE_COGNITO_DOMAIN=${COGNITO_DOMAIN}
VITE_COGNITO_CLIENT_ID=${CLIENT_ID}
VITE_COGNITO_REDIRECT_URI=${CF_URL}/callback
VITE_COGNITO_LOGOUT_URI=${CF_URL}/

VITE_API_BASE_URL=${API_BASE}
EOF

echo "S3 Bucket: $BUCKET"
echo "Distribution ID: $DIST_ID"
