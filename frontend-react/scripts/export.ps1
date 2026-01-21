#!/usr/bin/env pwsh
Set-StrictMode -Version Latest

# Allow passing from environment or default values
$APP_STAGE = $env:APP_STAGE
if (-not $APP_STAGE) { $APP_STAGE = "dev" }

$AWS_REGION = $env:AWS_REGION
if (-not $AWS_REGION) { $AWS_REGION = "us-east-1" }

function Get-ExportValue {
    param(
        [Parameter(Mandatory=$true)][string]$ExportName
    )

    $result = aws cloudformation list-exports `
        --region $AWS_REGION `
        --query "Exports[?Name=='$ExportName'].Value | [0]" `
        --output text

    return $result
}

Write-Host "Fetching CloudFormation exports for stage: $APP_STAGE..."

$CF_URL      = Get-ExportValue "TrueProfit-CloudFrontURL-$APP_STAGE"
$BUCKET      = Get-ExportValue "TrueProfit-FrontendBucketName-$APP_STAGE"
$DIST_ID     = Get-ExportValue "TrueProfit-CloudFrontDistributionId-$APP_STAGE"

$API_BASE    = Get-ExportValue "TrueProfit-ApiBaseUrl-$APP_STAGE"
$COGNITO_DOMAIN = Get-ExportValue "TrueProfit-CognitoHostedDomainURL-$APP_STAGE"
$USER_POOL_ID = Get-ExportValue "TrueProfit-CognitoUserPoolId-$APP_STAGE"
$CLIENT_ID    = Get-ExportValue "TrueProfit-CognitoUserPoolClientId-$APP_STAGE"

Write-Host "Writing .env file..."

@"
VITE_COGNITO_DOMAIN=$COGNITO_DOMAIN
VITE_COGNITO_CLIENT_ID=$CLIENT_ID
VITE_COGNITO_REDIRECT_URI=$CF_URL/callback
VITE_COGNITO_LOGOUT_URI=$CF_URL/

VITE_API_BASE_URL=$API_BASE
"@ | Set-Content -Path ".env" -Encoding UTF8

Write-Host "S3 Bucket: $BUCKET"
Write-Host "Distribution ID: $DIST_ID"
