#!/bin/bash
set -euo pipefail

# COVERAGE
#
# This script runs `go test` with coverage enabled and evaluates the resulting
# coverage percentage against a desired threshold.

THRESHOLD=90.0

# Run tests and capture the coverage line
line=$(go test ./... -cover | grep 'coverage:' | head -n1)

# Extract the percentage part, e.g. "17.4%"
coverage=$(echo "$line" | grep -E -o '[0-9]+([.][0-9]+)?%'| tr -d '%')

# Use bc for floating-point comparison, and capture just 0 or 1
result=$(echo "$coverage >= $THRESHOLD" | bc -l)

if [ "$result" -eq 1 ]; then
  echo "[92mCoverage ${coverage}% meets threshold (${THRESHOLD}%) ✔[0m"
else
  echo "[91mCoverage ${coverage}% does NOT meet threshold (${THRESHOLD}%) ✘"
  exit 1
fi