#!/usr/bin/env bash
set -euo pipefail

# Run all tests with race detector and coverage.
# Requires Linux or GOOS=linux cross-compilation environment.

GOOS=linux go test -race -coverprofile=coverage.txt ./...
go tool cover -func=coverage.txt | tail -1
