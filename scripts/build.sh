#!/usr/bin/env bash
set -euo pipefail

# Build thrive binary for Linux.
# Usage: ./scripts/build.sh [output-path]

OUTPUT="${1:-./bin/thrive}"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o "$OUTPUT" ./cmd/thrive
echo "Built: $OUTPUT"
