#!/bin/bash
set -euo pipefail

# Usage: autoresearch-log.sh <status> <metric_value> <description> [options]
# Options:
#   --commit <hash>        Git commit hash (7 chars). Auto-detected if omitted.
#   --metrics '{"k":v}'    Secondary metrics JSON object.
#   --asi '{"k":"v"}'      Actionable Side Information JSON object.
#
# Status values: keep, discard, crash, checks_failed
# On keep: git add -A && git commit
# On discard/crash/checks_failed: stage session files, revert code changes

STATUS="${1:?Usage: autoresearch-log.sh <status> <metric_value> <description> [--commit hash] [--metrics json] [--asi json]}"
METRIC="${2:?Usage: autoresearch-log.sh <status> <metric_value> <description>}"
DESCRIPTION="${3:?Usage: autoresearch-log.sh <status> <metric_value> <description>}"
shift 3

# Parse optional args
COMMIT=""
METRICS_JSON=""
ASI_JSON=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --commit) COMMIT="$2"; shift 2 ;;
    --metrics) METRICS_JSON="$2"; shift 2 ;;
    --asi) ASI_JSON="$2"; shift 2 ;;
    *) echo "ERROR: Unknown option: $1" >&2; exit 1 ;;
  esac
done

# Validate status
case "$STATUS" in
  keep|discard|crash|checks_failed) ;;
  *) echo "ERROR: status must be keep|discard|crash|checks_failed, got '$STATUS'" >&2; exit 1 ;;
esac

JSONL_FILE="autoresearch.jsonl"
if [ ! -f "$JSONL_FILE" ]; then
  echo "ERROR: $JSONL_FILE not found. Run autoresearch-init.sh first." >&2
  exit 1
fi

# Get current commit hash if not provided
if [ -z "$COMMIT" ]; then
  COMMIT=$(git rev-parse --short=7 HEAD 2>/dev/null || echo "0000000")
fi

# Calculate run number: count non-config lines
# Note: grep -c exits 1 when count is 0, so capture output separately from exit code
RUN=$(grep -cv '"type":"config"' "$JSONL_FILE" 2>/dev/null || true)
RUN=$(echo "$RUN" | tr -d '[:space:]')
RUN=${RUN:-0}

# Calculate current segment: count config lines minus 1
CONFIG_COUNT=$(grep -c '"type":"config"' "$JSONL_FILE" 2>/dev/null || true)
CONFIG_COUNT=$(echo "$CONFIG_COUNT" | tr -d '[:space:]')
CONFIG_COUNT=${CONFIG_COUNT:-1}
SEGMENT=$(( CONFIG_COUNT - 1 ))

# Read direction from last config line
DIRECTION=$(grep '"type":"config"' "$JSONL_FILE" | tail -1 | sed -n 's/.*"bestDirection":"\([^"]*\)".*/\1/p')
DIRECTION="${DIRECTION:-lower}"

METRIC_NAME=$(grep '"type":"config"' "$JSONL_FILE" | tail -1 | sed -n 's/.*"metricName":"\([^"]*\)".*/\1/p')

# --- Confidence scoring (MAD-based) ---
# Requires 3+ data points in current segment with metric > 0
CONFIDENCE="null"

# Get all metric values in current segment
SEGMENT_METRICS=$(awk -v seg="$SEGMENT" 'BEGIN { cfg_count = 0 }
  /"type":"config"/ { cfg_count++; next }
  {
    if (cfg_count - 1 == seg) {
      match($0, /"metric":([0-9.e+-]+)/, m)
      if (m[1] + 0 > 0) print m[1]
    }
  }
' "$JSONL_FILE" 2>/dev/null || true)

if [ -z "$SEGMENT_METRICS" ]; then
  POINT_COUNT=0
else
  POINT_COUNT=$(echo "$SEGMENT_METRICS" | wc -l | tr -d '[:space:]')
fi

if [ "$POINT_COUNT" -ge 3 ]; then
  # Compute confidence using awk
  CONFIDENCE=$(echo "$SEGMENT_METRICS" | awk -v direction="$DIRECTION" '
    BEGIN { n = 0 }
    { vals[n++] = $1 + 0 }
    END {
      if (n < 3) { print "null"; exit }

      # Save baseline (first chronological value) before sorting
      baseline = vals[0]

      # Find best value before sorting
      if (direction == "lower") {
        best = vals[0]
        for (i = 1; i < n; i++) if (vals[i] < best) best = vals[i]
      } else {
        best = vals[0]
        for (i = 1; i < n; i++) if (vals[i] > best) best = vals[i]
      }

      # Sort values for median calculation
      for (i = 0; i < n; i++)
        for (j = i+1; j < n; j++)
          if (vals[i] > vals[j]) { t = vals[i]; vals[i] = vals[j]; vals[j] = t }

      # Median
      if (n % 2 == 1) median = vals[int(n/2)]
      else median = (vals[n/2 - 1] + vals[n/2]) / 2

      # MAD (median absolute deviation)
      for (i = 0; i < n; i++) {
        d = vals[i] - median
        devs[i] = (d < 0) ? -d : d
      }
      # Sort deviations
      for (i = 0; i < n; i++)
        for (j = i+1; j < n; j++)
          if (devs[i] > devs[j]) { t = devs[i]; devs[i] = devs[j]; devs[j] = t }

      if (n % 2 == 1) mad = devs[int(n/2)]
      else mad = (devs[n/2 - 1] + devs[n/2]) / 2

      if (mad == 0) { print "null"; exit }

      delta = best - baseline
      if (delta < 0) delta = -delta
      printf "%.2f", delta / mad
    }
  ')
fi

# Build the result JSON line
TIMESTAMP=$(date +%s%3N 2>/dev/null || echo "$(date +%s)000")

# Start building JSON
RESULT_JSON=$(printf '{"run":%d,"commit":"%s","metric":%s,"status":"%s","description":"%s","timestamp":%s,"segment":%d,"confidence":%s' \
  "$RUN" "$COMMIT" "$METRIC" "$STATUS" "$DESCRIPTION" "$TIMESTAMP" "$SEGMENT" "$CONFIDENCE")

# Add secondary metrics if provided
if [ -n "$METRICS_JSON" ]; then
  RESULT_JSON="${RESULT_JSON},\"metrics\":${METRICS_JSON}"
fi

# Add ASI if provided
if [ -n "$ASI_JSON" ]; then
  RESULT_JSON="${RESULT_JSON},\"asi\":${ASI_JSON}"
fi

RESULT_JSON="${RESULT_JSON}}"

# Append to JSONL
echo "$RESULT_JSON" >> "$JSONL_FILE"

# Git operations based on status
case "$STATUS" in
  keep)
    git add -A
    COMMIT_MSG=$(printf '%s\n\nResult: {"status":"keep","%s":%s}' "$DESCRIPTION" "$METRIC_NAME" "$METRIC")
    git commit -m "$COMMIT_MSG" --quiet
    NEW_COMMIT=$(git rev-parse --short=7 HEAD)
    echo "--- Experiment logged: KEEP ---"
    echo "Run: $RUN"
    echo "Commit: $NEW_COMMIT"
    echo "$METRIC_NAME: $METRIC"
    ;;
  discard|crash|checks_failed)
    # Stage autoresearch session files (preserve them through revert)
    git add autoresearch.jsonl 2>/dev/null || true
    git add autoresearch.md 2>/dev/null || true
    git add autoresearch.ideas.md 2>/dev/null || true
    git add autoresearch.sh 2>/dev/null || true
    git add autoresearch.checks.sh 2>/dev/null || true
    # Revert all other changes
    git checkout -- . 2>/dev/null || true
    git clean -fd 2>/dev/null || true
    echo "--- Experiment logged: ${STATUS^^} ---"
    echo "Run: $RUN"
    echo "Reverted code changes. Session files preserved."
    echo "$METRIC_NAME: $METRIC"
    ;;
esac

# Report confidence
if [ "$CONFIDENCE" != "null" ]; then
  CONF_VAL=$(echo "$CONFIDENCE" | awk '{ if ($1 >= 2.0) print "strong"; else if ($1 >= 1.0) print "marginal"; else print "noise" }')
  echo "Confidence: ${CONFIDENCE}x ($CONF_VAL)"
else
  echo "Confidence: n/a (need 3+ data points)"
fi

# Report baseline delta
BASELINE=$(grep -v '"type":"config"' "$JSONL_FILE" | head -1 | sed -n 's/.*"metric":\([0-9.e+-]*\).*/\1/p' 2>/dev/null || echo "")
if [ -n "$BASELINE" ] && [ "$METRIC" != "0" ]; then
  DELTA=$(awk "BEGIN { printf \"%.2f\", (($METRIC - $BASELINE) / $BASELINE) * 100 }")
  echo "Delta from baseline: ${DELTA}%"
fi

echo "Total runs: $((RUN + 1))"
