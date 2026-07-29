#!/usr/bin/env bash
# Fail if total test coverage is below the required threshold.
# Usage: check-coverage.sh <coverage-profile> <min-percent>
set -euo pipefail

PROFILE="${1:-coverage.out}"
THRESHOLD="${2:-85}"

if [[ ! -f "$PROFILE" ]]; then
	echo "coverage profile not found: $PROFILE" >&2
	exit 1
fi

TOTAL=$(go tool cover -func="$PROFILE" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')

if [[ -z "$TOTAL" ]]; then
	echo "could not determine total coverage from $PROFILE" >&2
	exit 1
fi

echo "total coverage: ${TOTAL}% (required: ${THRESHOLD}%)"

if awk -v t="$TOTAL" -v m="$THRESHOLD" 'BEGIN { exit !(t < m) }'; then
	echo "FAIL: coverage ${TOTAL}% is below the required ${THRESHOLD}%" >&2
	exit 1
fi

echo "PASS: coverage gate satisfied"
