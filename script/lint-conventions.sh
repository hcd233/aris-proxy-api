#!/usr/bin/env bash
# lint-conventions.sh — compatibility wrapper for the Go AST convention checker.
set -euo pipefail

go run ./cmd/lintconv ./...
