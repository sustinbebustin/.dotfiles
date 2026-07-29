#!/usr/bin/env bash
# Self-test for rebase-guard.sh. Builds throwaway repos in a temp dir, exercises
# the two things the tool exists to do, and cleans up.
#
# Run this if rebase-guard's output looks wrong, or after editing it.
#   bash ~/.claude/skills/rebase/scripts/selftest.sh
set -eu

GUARD="$(cd "$(dirname "$0")" && pwd)/rebase-guard.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0
check() {
  if [ "$1" = "yes" ]; then
    echo "[PASS] $2"; pass=$((pass + 1))
  else
    echo "[FAIL] $2"; fail=$((fail + 1))
  fi
}

# ---------------------------------------------------------------------------
# Test 1: a semantic conflict git merges clean must be flagged.
# ---------------------------------------------------------------------------
R="$TMP/semantic"; mkdir -p "$R/src"
g() { git -C "$R" "$@"; }
g init -q -b main .; g config user.email t@t; g config user.name t
printf 'package p\n\nfunc CalculateHvacPrice(x int) int { return x*2 }\nfunc CalculateRoofPrice(x int) int { return x*3 }\n' >"$R/src/pricing.go"
g add -A; g commit -qm base
BASE="$(g rev-parse HEAD)"

g checkout -qb feature
printf 'package p\n\nfunc GateHvac() int { return CalculateHvacPrice(10) }\n' >"$R/src/authz.go"
g add -A; g commit -qm "branch: gate"

g checkout -q main
printf 'package p\n\nfunc CalculateRoofPrice(x int) int { return x*3 }\n' >"$R/src/pricing.go"
g add -A; g commit -qm "main: remove hvac"

g checkout -q feature; g rebase -q main

[ -z "$(g status --porcelain)" ] && clean=yes || clean=no
check "$clean" "git rebases clean (no conflict raised) -- the premise of the test"

out="$(cd "$R" && bash "$GUARD" report main --pre "$BASE" 2>&1)"
echo "$out" | grep -q 'CalculateHvacPrice' && hit=yes || hit=no
check "$hit" "report flags the removed symbol the branch still calls"

echo "$out" | grep -q 'main: remove hvac' && landed=yes || landed=no
check "$landed" "report lists what landed on trunk"

# ---------------------------------------------------------------------------
# Test 2: snapshot/restore round-trips uncommitted work.
# ---------------------------------------------------------------------------
R="$TMP/backup"; mkdir -p "$R/src/nested"
g() { git -C "$R" "$@"; }
g init -q -b main .; g config user.email t@t; g config user.name t
printf 'original\n' >"$R/src/tracked.txt"
g add -A; g commit -qm base
g checkout -qb feature

printf 'original\nEDIT\n' >"$R/src/tracked.txt"
printf 'new\n' >"$R/src/untracked.txt"
printf 'deep\n' >"$R/src/nested/deep.txt"
before="$(cat "$R/src/tracked.txt" "$R/src/untracked.txt" "$R/src/nested/deep.txt")"

(cd "$R" && bash "$GUARD" snapshot main >/dev/null 2>&1)
g reset -q --hard HEAD
g clean -qfd
(cd "$R" && bash "$GUARD" restore >/dev/null 2>&1)

after="$(cat "$R/src/tracked.txt" "$R/src/untracked.txt" "$R/src/nested/deep.txt" 2>/dev/null || echo MISSING)"
[ "$before" = "$after" ] && ok=yes || ok=no
check "$ok" "snapshot/restore round-trips tracked edits + untracked files"

[ "$(g config --get rerere.enabled)" = "true" ] && rr=yes || rr=no
check "$rr" "snapshot enables rerere"

# ---------------------------------------------------------------------------
# Test 3: must not mistake an ancestor repo for the current one.
# ---------------------------------------------------------------------------
R="$TMP/nested-ws"; mkdir -p "$R/child"
git init -q -b main "$R" >/dev/null 2>&1
git -C "$R" config user.email t@t; git -C "$R" config user.name t
git init -q -b main "$R/child" >/dev/null 2>&1
git -C "$R/child" config user.email t@t; git -C "$R/child" config user.name t
printf 'x\n' >"$R/child/f.txt"
git -C "$R/child" add -A; git -C "$R/child" commit -qm base

mkdir -p "$R/workspace/child"
git init -q -b main "$R/workspace/child" >/dev/null 2>&1
git -C "$R/workspace/child" config user.email t@t; git -C "$R/workspace/child" config user.name t
printf 'y\n' >"$R/workspace/child/f.txt"
git -C "$R/workspace/child" add -A; git -C "$R/workspace/child" commit -qm base

# $R/workspace is NOT a repo root, but $R (its parent) is. Must scan children.
out="$(cd "$R/workspace" && bash "$GUARD" snapshot main 2>&1)"
echo "$out" | grep -q '### child' && scoped=yes || scoped=no
check "$scoped" "scans child repos instead of latching onto an ancestor repo"

echo
echo "$pass passed, $fail failed"
[ "$fail" = "0" ]
