$ErrorActionPreference = "Stop"

$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$dist = Join-Path $root "dist"

if (!(Test-Path $dist)) { New-Item -ItemType Directory -Path $dist | Out-Null }

function Build-One([string]$name) {
    Write-Host "==> Building $name..."

    $tmp = Join-Path $root ".tmp"
    if (Test-Path $tmp) { Remove-Item $tmp -Recurse -Force }
    New-Item -ItemType Directory -Path $tmp | Out-Null

    $bootstrap = Join-Path $tmp "bootstrap"
    $zipPath = Join-Path $dist "$name.zip"

    if (Test-Path $zipPath) { Remove-Item $zipPath -Force }

    Push-Location $root
    $env:GOOS = "linux"
    $env:GOARCH = "arm64"
    $env:CGO_ENABLED = "0"

    go build -ldflags="-s -w" -o $bootstrap ("./cmd/" + $name)
    Pop-Location

    Compress-Archive -Path $bootstrap -DestinationPath $zipPath -Force

    Remove-Item $tmp -Recurse -Force

    Write-Host "==> Output: $zipPath"
}

Build-One "health"
Build-One "hello"
Build-One "transactions"
Build-One "summary"
Build-One "maintenance"
Build-One "shopify"
Build-One "shopify-orders-worker"
Build-One "shopify-refunds-worker"
Build-One "shopify-emailer"

Write-Host "Done."
