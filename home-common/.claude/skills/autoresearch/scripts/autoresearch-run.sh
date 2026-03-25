#!/bin/bash
set -euo pipefail

# Usage: autoresearch-run.sh [command] [timeout_seconds] [checks_timeout_seconds]
# Runs the benchmark command, extracts METRIC lines, optionally runs checks.
# Defaults: ./autoresearch.sh, 600s timeout, 300s checks timeout

COMMAND="${1:-./autoresearch.sh}"
TIMEOUT="${2:-600}"
CHECKS_TIMEOUT="${3:-300}"

# Guard: if autoresearch.sh exists, only allow running it
if [ -f "autoresearch.sh" ]; then
  # Strip env vars, wrappers (env, time, nice, nohup) to find the core command
  CORE_CMD="$COMMAND"
  CORE_CMD=$(echo "$CORE_CMD" | sed -E 's/^(\w+=\S*\s+)+//')
  CORE_CMD=$(echo "$CORE_CMD" | sed -E 's/^(env|time|nice|nohup)(\s+-\S+)*\s+//')
  # Check if core command is autoresearch.sh
  if ! echo "$CORE_CMD" | grep -qE '^(bash\s+(-\w+\s+)*)?(\.\/|\/[\w/.-]*\/)?autoresearch\.sh(\s|$)'; then
    echo "ERROR: autoresearch.sh exists -- you must run it instead of a custom command." >&2
    echo "Use: autoresearch-run.sh './autoresearch.sh'" >&2
    exit 1
  fi
fi

OUTPUT_FILE=$(mktemp /tmp/autoresearch-output.XXXXXX)
trap 'rm -f "$OUTPUT_FILE"' EXIT

echo "--- Running benchmark ---"
echo "Command: $COMMAND"
echo "Timeout: ${TIMEOUT}s"

# Run the command with timeout, capture exit code
BENCH_EXIT=0
SECONDS=0
timeout --kill-after=10 "$TIMEOUT" bash -c "$COMMAND" > "$OUTPUT_FILE" 2>&1 || BENCH_EXIT=$?
ELAPSED=$SECONDS

if [ "$BENCH_EXIT" -eq 124 ]; then
  echo ""
  echo "=== BENCHMARK TIMED OUT (${TIMEOUT}s) ==="
  echo "STATUS: timeout"
  echo "EXIT_CODE: 124"
  echo ""
  echo "--- Output (last 20 lines) ---"
  tail -20 "$OUTPUT_FILE"
  exit 1
fi

if [ "$BENCH_EXIT" -ne 0 ]; then
  echo ""
  echo "=== BENCHMARK CRASHED ==="
  echo "STATUS: crash"
  echo "EXIT_CODE: $BENCH_EXIT"
  echo "ELAPSED: ${ELAPSED}s"
  echo ""
  echo "--- Output (last 20 lines) ---"
  tail -20 "$OUTPUT_FILE"
  exit 1
fi

# Extract METRIC lines
echo ""
echo "=== BENCHMARK PASSED ==="
echo "STATUS: pass"
echo "EXIT_CODE: 0"
echo "ELAPSED: ${ELAPSED}s"
echo ""
echo "--- Metrics ---"
if grep -q '^METRIC ' "$OUTPUT_FILE"; then
  grep '^METRIC ' "$OUTPUT_FILE"
else
  echo "WARNING: No METRIC lines found in output"
fi

# Run checks if autoresearch.checks.sh exists
CHECKS_STATUS="skipped"
if [ -f "autoresearch.checks.sh" ]; then
  CHECKS_OUTPUT=$(mktemp /tmp/autoresearch-checks.XXXXXX)
  trap 'rm -f "$OUTPUT_FILE" "$CHECKS_OUTPUT"' EXIT

  echo ""
  echo "--- Running checks (${CHECKS_TIMEOUT}s timeout) ---"
  CHECKS_EXIT=0
  timeout --kill-after=10 "$CHECKS_TIMEOUT" bash autoresearch.checks.sh > "$CHECKS_OUTPUT" 2>&1 || CHECKS_EXIT=$?

  if [ "$CHECKS_EXIT" -eq 0 ]; then
    CHECKS_STATUS="pass"
    echo "CHECKS: pass"
  else
    CHECKS_STATUS="fail"
    echo "CHECKS: FAIL (exit code $CHECKS_EXIT)"
    echo ""
    echo "--- Checks output (last 80 lines) ---"
    tail -80 "$CHECKS_OUTPUT"
  fi
fi

echo ""
echo "--- Output (last 20 lines) ---"
tail -20 "$OUTPUT_FILE"

echo ""
echo "--- Summary ---"
echo "BENCH_STATUS: pass"
echo "CHECKS_STATUS: $CHECKS_STATUS"
echo "ELAPSED: ${ELAPSED}s"
echo "TOTAL_OUTPUT_LINES: $(wc -l < "$OUTPUT_FILE")"
