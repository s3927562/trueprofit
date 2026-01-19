#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

mkdir -p "${DIST_DIR}"

build_one () {
  local name="$1"      # health or hello
  local out_zip="${DIST_DIR}/${name}.zip"

  echo "==> Building ${name}..."
  rm -f "${out_zip}"

  # Lambda custom runtime expects an executable named "bootstrap"
  # Compile Linux binary and place as bootstrap
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT

  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o "${tmpdir}/bootstrap" "./cmd/${name}"

  (cd "${tmpdir}" && zip -q -r "${out_zip}" bootstrap)

  echo "==> Wrote ${out_zip}"
}

build_one health
build_one transactions
build_one summary
build_one shopify
build_one shopify-orders-worker
build_one shopify-refunds-worker
build_one shopify-emailer

echo "Done."
