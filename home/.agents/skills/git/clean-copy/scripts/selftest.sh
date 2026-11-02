#!/usr/bin/env bash
# Self-test for clean-copy.sh. Builds throwaway repos in a temp dir, proves the
# claims the skill rests on, and cleans up.
#
# Run this if clean-copy's output looks wrong, or after editing it.
#   bash ~/.claude/skills/clean-copy/scripts/selftest.sh
set -u

CC="$(cd "$(dirname "$0")" && pwd)/clean-copy.sh"
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

newrepo() {
  local r="$TMP/$1"
  mkdir -p "$r"
  git -C "$r" init -q -b main .
  git -C "$r" config user.email t@t
  git -C "$r" config user.name t
  git -C "$r" config commit.gpgsign false
  echo "$r"
}

# ---------------------------------------------------------------------------
# Test 1: `git add` is NOT content-preserving under renormalization, and
# `git restore --source` is. This is the whole reason `stage` exists.
# ---------------------------------------------------------------------------
R="$(newrepo renorm)"
g() { git -C "$R" "$@"; }
g config core.autocrlf false

printf 'alpha\r\nbeta\r\n' >"$R/f.txt"
g add -A >/dev/null; g commit -qm base
MB="$(g rev-parse HEAD)"

# The branch declares the path as text but does not restage f.txt, so the
# source tree still holds the CRLF blob while `* text` is in effect.
g checkout -qb feature
printf '* text\n' >"$R/.gitattributes"
g add -- .gitattributes >/dev/null 2>&1
g commit -qm "branch: declare text" >/dev/null
SRC_BLOB="$(g rev-parse 'feature:f.txt')"

g switch -q -c feature-clean feature
g reset -q --mixed "$MB"
g add -- .gitattributes >/dev/null 2>&1
g add -- f.txt >/dev/null 2>&1
ADD_BLOB="$(g rev-parse ':f.txt')"
[ "$ADD_BLOB" != "$SRC_BLOB" ] && diverged=yes || diverged=no
check "$diverged" "git add diverges from the source blob under renormalization"

g restore --source=feature --staged --worktree -- f.txt
RES_BLOB="$(g rev-parse ':f.txt')"
[ "$RES_BLOB" = "$SRC_BLOB" ] && matched=yes || matched=no
check "$matched" "git restore --source reproduces the source blob exactly"

# ---------------------------------------------------------------------------
# Test 2: `git add` loses file modes when core.fileMode=false; restore does not.
# ---------------------------------------------------------------------------
R="$(newrepo mode)"
g() { git -C "$R" "$@"; }
printf '#!/bin/sh\necho hi\n' >"$R/s.sh"; chmod 644 "$R/s.sh"
g add -A >/dev/null; g commit -qm base
MB="$(g rev-parse HEAD)"
g checkout -qb feature
chmod 755 "$R/s.sh"; g add -A >/dev/null; g commit -qm "branch: chmod +x"
SRC_MODE="$(g ls-tree feature -- s.sh | awk '{print $1}')"

g switch -q -c feature-clean feature
g reset -q --mixed "$MB"
g config core.fileMode false
chmod 644 "$R/s.sh"
g add -- s.sh >/dev/null
[ "$(g ls-files -s -- s.sh | awk '{print $1}')" != "$SRC_MODE" ] && lost=yes || lost=no
check "$lost" "git add loses the exec bit under core.fileMode=false"

g restore --source=feature --staged --worktree -- s.sh
[ "$(g ls-files -s -- s.sh | awk '{print $1}')" = "$SRC_MODE" ] && kept=yes || kept=no
check "$kept" "git restore --source preserves the mode from the tree"

# ---------------------------------------------------------------------------
# Test 3: an ignored-but-tracked path is invisible to status, silently skipped
# by `git add -A`, and picked up correctly by restore.
# ---------------------------------------------------------------------------
R="$(newrepo ignored)"
g() { git -C "$R" "$@"; }
printf 'x\n' >"$R/base.txt"; g add -A >/dev/null; g commit -qm base
MB="$(g rev-parse HEAD)"
g checkout -qb feature
printf 'dist/\n' >"$R/.gitignore"
mkdir -p "$R/dist"; printf 'built\n' >"$R/dist/out.js"
g add -A >/dev/null; g add -f dist/out.js >/dev/null; g commit -qm "branch: ignored but tracked"

g switch -q -c feature-clean feature
g reset -q --mixed "$MB"
[ "$(g status --porcelain -uall | grep -c 'dist/out.js')" = "0" ] && hidden=yes || hidden=no
check "$hidden" "ignored-but-tracked path is invisible to git status"

g add -A >/dev/null
[ "$(g ls-files --cached -- dist/out.js | wc -l)" = "0" ] && skipped=yes || skipped=no
check "$skipped" "git add -A silently skips it"

g restore --source=feature --staged --worktree -- :/
[ "$(g ls-files --cached -- dist/out.js | wc -l)" = "1" ] && bypassed=yes || bypassed=no
check "$bypassed" "git restore --source bypasses ignore rules"

# ---------------------------------------------------------------------------
# Test 4: no-overlay restore removes a path the source deleted, rather than
# erroring on a pathspec absent from the source tree. If this ever fails, the
# `git rm --cached` fallback in `stage` becomes load-bearing.
# ---------------------------------------------------------------------------
R="$(newrepo deleted)"
g() { git -C "$R" "$@"; }
printf 'keep\n' >"$R/keep.txt"; printf 'doomed\n' >"$R/gone.txt"
g add -A >/dev/null; g commit -qm base
MB="$(g rev-parse HEAD)"
g checkout -qb feature
g rm -q gone.txt; g add -A >/dev/null; g commit -qm "branch: delete gone.txt"

g switch -q -c feature-clean feature
g reset -q --mixed "$MB"
g restore --source=feature --staged --worktree -- gone.txt 2>/dev/null && ok=yes || ok=no
check "$ok" "restore accepts a pathspec for a path the source deleted"
[ "$(g ls-files -- gone.txt | wc -l)" = "0" ] && removed=yes || removed=no
check "$removed" "and removes its index entry"

# ---------------------------------------------------------------------------
# Test 5: end to end through the guard -- add, modify, delete, rename, and a
# mode change, sliced into several commits, must land on an identical tree.
# ---------------------------------------------------------------------------
R="$(newrepo e2e)"
g() { git -C "$R" "$@"; }
mkdir -p "$R/src"
printf 'one\n' >"$R/src/keep.go"
printf 'old\n' >"$R/src/oldname.go"
printf 'bye\n' >"$R/src/doomed.go"
printf 'echo\n' >"$R/run.sh"; chmod 644 "$R/run.sh"
g add -A >/dev/null; g commit -qm base

g checkout -qb feature
printf 'one\ntwo\n' >"$R/src/keep.go"           # modify
printf 'brand new\n' >"$R/src/added.go"         # add
g rm -q src/doomed.go                           # delete
g mv src/oldname.go src/newname.go              # rename
chmod 755 "$R/run.sh"                           # mode change
g add -A >/dev/null; g commit -qm "everything at once"
SRC_TREE="$(g rev-parse 'feature^{tree}')"

out="$( (cd "$R" && bash "$CC" state main) 2>&1 )" && st=yes || st=no
check "$st" "state accepts a healthy branch"
echo "$out" | grep -q 'src/added.go' && listed=yes || listed=no
check "$listed" "state's work list names the added file"

# Partition it: three commits, staging only from the source tree.
g switch -q -c feature-clean feature
g reset -q --mixed main
(cd "$R" && bash "$CC" stage src/newname.go src/oldname.go >/dev/null) \
  && g commit -q --no-verify -m "refactor: rename oldname to newname"
(cd "$R" && bash "$CC" stage src/added.go src/keep.go >/dev/null) \
  && g commit -q --no-verify -m "feat: add the new thing"
(cd "$R" && bash "$CC" stage src/doomed.go run.sh >/dev/null) \
  && g commit -q --no-verify -m "chore: drop dead code, mark run.sh executable"

[ "$(g rev-parse 'HEAD^{tree}')" = "$SRC_TREE" ] && same=yes || same=no
check "$same" "sliced tree is identical to the source tree"

[ "$(g rev-list --count main..HEAD)" = "3" ] && count=yes || count=no
check "$count" "history has the three planned commits"

g log --diff-filter=R --oneline -M --name-status main..HEAD 2>/dev/null | grep -q '^R' \
  && renamed=yes || renamed=no
check "$renamed" "the rename renders as a rename, not delete+add"

vout="$( (cd "$R" && bash "$CC" verify) 2>&1 )" && vok=yes || vok=no
check "$vok" "verify passes on a correct partition"
echo "$vout" | grep -q 'tree identical' && gate=yes || gate=no
check "$gate" "verify reports the tree gate"

# ---------------------------------------------------------------------------
# Test 6: verify must FAIL when a slice is forgotten.
# ---------------------------------------------------------------------------
R="$(newrepo forgotten)"
g() { git -C "$R" "$@"; }
printf 'a\n' >"$R/a.txt"; g add -A >/dev/null; g commit -qm base
g checkout -qb feature
printf 'a2\n' >"$R/a.txt"; printf 'b\n' >"$R/b.txt"
g add -A >/dev/null; g commit -qm "two files"
(cd "$R" && bash "$CC" state main >/dev/null 2>&1)
g switch -q -c feature-clean feature
g reset -q --mixed main
(cd "$R" && bash "$CC" stage a.txt >/dev/null)   # b.txt deliberately forgotten
g commit -q --no-verify -m "partial"
(cd "$R" && bash "$CC" verify >/dev/null 2>&1) && vfail=no || vfail=yes
check "$vfail" "verify fails when a file was never staged"

# ---------------------------------------------------------------------------
# Test 7: state's refusals.
# ---------------------------------------------------------------------------
R="$(newrepo refuse)"
g() { git -C "$R" "$@"; }
printf 'a\n' >"$R/a.txt"; g add -A >/dev/null; g commit -qm base

(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses when the branch is the base"

g checkout -qb feature
printf 'b\n' >"$R/b.txt"; g add -A >/dev/null; g commit -qm work
printf 'dirty\n' >>"$R/a.txt"
(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses a dirty worktree"
g checkout -q -- a.txt

g branch feature-clean
(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses when the clean branch already exists"
g branch -q -D feature-clean

g checkout -q --detach
(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses on detached HEAD"
g checkout -q feature

g checkout -qb feature-clean-probe
g branch -m other-clean
(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses to run from a branch already named *-clean"
g checkout -q feature

# A branch whose commits net out to nothing must be refused, not silently
# "copied" into zero commits with a trivially passing tree gate. Cut from main,
# so the only content in range is the add that the remove cancels.
g checkout -q main
g checkout -qb noop
printf 'temp\n' >"$R/tmp.txt"; g add -A >/dev/null; g commit -qm add
g rm -q tmp.txt; g commit -qm remove >/dev/null
(cd "$R" && bash "$CC" state main >/dev/null 2>&1) && r=no || r=yes
check "$r" "state refuses a branch with commits but zero net change"

# ---------------------------------------------------------------------------
# Test 8: a stale local base must not silently widen the diff.
# ---------------------------------------------------------------------------
R="$(newrepo drift)"
g() { git -C "$R" "$@"; }
printf 'a\n' >"$R/a.txt"; g add -A >/dev/null; g commit -qm base
UP="$TMP/upstream.git"; git init -q --bare -b main "$UP"
g remote add origin "$UP"; g push -q origin main
# Someone else lands a commit; local main stays behind while the branch is cut
# from the newer remote tip.
g checkout -qb their-work
printf 'theirs\n' >"$R/theirs.txt"; g add -A >/dev/null; g commit -qm "their commit"
g push -q origin their-work:main
g fetch -q origin
g checkout -q -b feature origin/main
printf 'mine\n' >"$R/mine.txt"; g add -A >/dev/null; g commit -qm "my commit"

out="$( (cd "$R" && bash "$CC" state main) 2>&1 )"
echo "$out" | grep -q 'stale' && warned=yes || warned=no
check "$warned" "state warns that local main is stale"
echo "$out" | grep -q 'theirs.txt' && leaked=yes || leaked=no
check "$( [ "$leaked" = "no" ] && echo yes || echo no )" \
  "the work list excludes the other author's file"

# ---------------------------------------------------------------------------
# Test 9: a poly-repo change -- two sibling checkouts copied in one invocation,
# sliced into matching storylines, and verified together.
# ---------------------------------------------------------------------------
W="$TMP/poly"
mkdir -p "$W"
for name in frontend backend; do
  R="$(newrepo "poly/$name")"
  printf 'base\n' >"$R/base.txt"
  git -C "$R" add -A >/dev/null; git -C "$R" commit -qm base
  git -C "$R" checkout -qb feature
  printf 'schema\n' >"$R/schema.txt"
  printf 'wire\n' >"$R/wire.txt"
  git -C "$R" add -A >/dev/null; git -C "$R" commit -qm "everything at once"
done
FE="$W/frontend"; BE="$W/backend"

out="$( (cd "$W" && bash "$CC" state) 2>&1 )" && st=yes || st=no
check "$st" "state accepts a workspace of sibling repos"
echo "$out" | grep -q '## Repo: frontend' && both=yes || both=no
check "$both" "state reports each repo in its own block"
echo "$out" | grep -q '^- repos: backend frontend' && scoped=yes || scoped=no
check "$scoped" "state lists the discovered scope"

sout="$( (cd "$W" && bash "$CC" state frontend) 2>&1 )"
echo "$sout" | grep -q 'backend' && narrowed=no || narrowed=yes
check "$narrowed" "a repo token narrows the scope to that repo"

# Ambiguity is refused, not guessed: staging into the wrong repo is silent.
(cd "$W" && bash "$CC" stage schema.txt >/dev/null 2>&1) && amb=no || amb=yes
check "$amb" "stage refuses to guess which repo when several have state"

# A workspace holds repos that are not part of the change. Discovering one must
# not fail the run -- but naming it must, since the user asked for it.
R="$(newrepo poly/unrelated)"
printf 'x\n' >"$R/x.txt"
git -C "$R" add -A >/dev/null; git -C "$R" commit -qm base
nout="$( (cd "$W" && bash "$CC" state) 2>&1 )" && nok=yes || nok=no
check "$nok" "a neighbour repo with nothing to copy does not fail the run"
echo "$nout" | grep -q '\[SKIP\].*nothing to copy' && said=yes || said=no
check "$said" "and the skip is reported, not silent"
(cd "$W" && bash "$CC" state unrelated >/dev/null 2>&1) && named=no || named=yes
check "$named" "naming that same repo is still a refusal"
rm -rf "$R"

(cd "$W" && bash "$CC" state >/dev/null 2>&1)
for R in "$FE" "$BE"; do
  git -C "$R" switch -q -c feature-clean feature
  git -C "$R" reset -q --mixed main
done
# Same idea, same position, same subject in both histories.
(cd "$W" && bash "$CC" stage --repo frontend schema.txt >/dev/null)
(cd "$W" && bash "$CC" stage --repo backend schema.txt >/dev/null)
git -C "$FE" commit -q --no-verify -m "feat(quotes): add the schema"
git -C "$BE" commit -q --no-verify -m "feat(quotes): add the schema"
(cd "$W" && bash "$CC" stage --repo frontend wire.txt >/dev/null)
(cd "$W" && bash "$CC" stage --repo backend wire.txt >/dev/null)
git -C "$FE" commit -q --no-verify -m "feat(quotes): wire it up"
git -C "$BE" commit -q --no-verify -m "feat(quotes): wire it up"

vout="$( (cd "$W" && bash "$CC" verify) 2>&1 )" && vok=yes || vok=no
check "$vok" "verify passes over every repo in one call"
[ "$(echo "$vout" | grep -c 'tree identical')" = "2" ] && gates=yes || gates=no
check "$gates" "verify gates both repos, not just the first"
echo "$vout" | grep -q 'Storyline alignment' && aligned=yes || aligned=no
check "$aligned" "verify prints the cross-repo storyline alignment"
echo "$vout" | grep -qE '^ +2\. +frontend +feat\(quotes\): wire it up' && paired=yes || paired=no
check "$paired" "the alignment table pairs commits by position"
echo "$vout" | grep -q 'different position' && falsepos=yes || falsepos=no
check "$( [ "$falsepos" = "no" ] && echo yes || echo no )" \
  "an aligned storyline draws no ordering warning"

# The same idea told in a different order in each repo is the failure mode the
# table exists to catch; the tree gates cannot see it.
git -C "$BE" reset -q --hard HEAD~2
(cd "$W" && bash "$CC" stage --repo backend wire.txt >/dev/null)
git -C "$BE" commit -q --no-verify -m "feat(quotes): wire it up"
(cd "$W" && bash "$CC" stage --repo backend schema.txt >/dev/null)
git -C "$BE" commit -q --no-verify -m "feat(quotes): add the schema"
dout="$( (cd "$W" && bash "$CC" verify) 2>&1 )" && dok=yes || dok=no
check "$dok" "a reordered repo still passes its own tree gate"
echo "$dout" | grep -q 'different position' && drift=yes || drift=no
check "$drift" "and the alignment table flags the reordering"
git -C "$BE" reset -q --hard HEAD~2
(cd "$W" && bash "$CC" stage --repo backend schema.txt >/dev/null)
git -C "$BE" commit -q --no-verify -m "feat(quotes): add the schema"
(cd "$W" && bash "$CC" stage --repo backend wire.txt >/dev/null)
git -C "$BE" commit -q --no-verify -m "feat(quotes): wire it up"

# One repo failing must fail the whole run: a half-copied poly-repo change is
# worse than none, because the tree gate passed on the half that is done.
git -C "$BE" reset -q --soft HEAD~1
(cd "$W" && bash "$CC" verify >/dev/null 2>&1) && anyfail=no || anyfail=yes
check "$anyfail" "verify fails the run when any repo's tree diverges"
git -C "$BE" commit -q --no-verify -m "feat(quotes): wire it up"

(cd "$W" && bash "$CC" abort >/dev/null 2>&1) && aok=yes || aok=no
check "$aok" "abort unwinds every repo in one call"
left=0
for R in "$FE" "$BE"; do
  [ "$(git -C "$R" symbolic-ref --short HEAD)" = "feature" ] || left=1
  git -C "$R" rev-parse --verify -q refs/heads/feature-clean >/dev/null && left=1
done
check "$( [ "$left" = "0" ] && echo yes || echo no )" \
  "both repos are back on their source branch with the clean branch gone"

# ---------------------------------------------------------------------------
echo ""
echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ] || exit 1
