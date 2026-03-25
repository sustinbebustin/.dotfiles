#!/bin/bash
set -euo pipefail

# Usage: autoresearch-init.sh <name> <metric_name> [metric_unit] [metric_direction]
# Writes a config header to autoresearch.jsonl.
# On re-init (JSONL exists with results), appends a new segment header.

NAME="${1:?Usage: autoresearch-init.sh <name> <metric_name> [metric_unit] [direction]}"
METRIC_NAME="${2:?Usage: autoresearch-init.sh <name> <metric_name> [metric_unit] [direction]}"
METRIC_UNIT="${3:-}"
DIRECTION="${4:-lower}"

if [[ "$DIRECTION" != "lower" && "$DIRECTION" != "higher" ]]; then
  echo "ERROR: direction must be 'lower' or 'higher', got '$DIRECTION'" >&2
  exit 1
fi

JSONL_FILE="autoresearch.jsonl"
CONFIG_LINE=$(printf '{"type":"config","name":"%s","metricName":"%s","metricUnit":"%s","bestDirection":"%s"}' \
  "$NAME" "$METRIC_NAME" "$METRIC_UNIT" "$DIRECTION")

if [ ! -f "$JSONL_FILE" ]; then
  echo "$CONFIG_LINE" > "$JSONL_FILE"
  echo "--- Autoresearch initialized ---"
  echo "Session: $NAME"
  echo "Metric: $METRIC_NAME ($METRIC_UNIT, $DIRECTION is better)"
  echo "Segment: 0"
  echo "JSONL: $JSONL_FILE"
else
  # Check if there are any result entries (non-config lines)
  RESULT_COUNT=$(grep -cv '"type":"config"' "$JSONL_FILE" 2>/dev/null || true)
  RESULT_COUNT=$(echo "$RESULT_COUNT" | tr -d '[:space:]')
  RESULT_COUNT=${RESULT_COUNT:-0}
  echo "$CONFIG_LINE" >> "$JSONL_FILE"
  echo "--- Autoresearch re-initialized (new segment) ---"
  echo "Session: $NAME"
  echo "Metric: $METRIC_NAME ($METRIC_UNIT, $DIRECTION is better)"
  echo "Previous results: $RESULT_COUNT"
  echo "JSONL: $JSONL_FILE"
fi
