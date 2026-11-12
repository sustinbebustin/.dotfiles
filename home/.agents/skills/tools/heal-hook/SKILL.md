---
name: heal-hook
description: Mine recent permission prompts and hook blocks, then autonomously loosen the PreToolUse guards wherever they stopped a provably safe call.
argument-hint: "[--since 7d] [--project text]"
disable-model-invocation: true
allowed-tools: Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/permission_asks.py *)
metadata:
  author: sustinbebustin
---

# Heal the hooks

The guards live in `~/.dotfiles/home/.agents/hooks` (Go; one package per rule under `internal/rules/`, shared shell parsing in `internal/shellast/`). Read `~/.dotfiles/docs/architecture.md` on the hooks before editing them. The binary is live as soon as `make build` runs, in every session.

The goal is the fewest prompts that still stop an agent from deleting, publishing, or reading what it should not. The run is autonomous: you decide every bucket, apply every heal, and hand the user a finished report, not a list of questions.

The stance throughout: a guard heals by learning to *prove* a call safe, never by trusting it. Every loosening keeps the guard failing closed on anything it cannot pin down, and a **silent allow** (a call now waved through that deletes, publishes or leaks something it should not) is the one outcome worse than a prompt. Doubt therefore always resolves to Keep: a prompt that survives costs the user a click; a silent allow can cost everything.

## Step 1: Mine

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/permission_asks.py --replay $ARGUMENTS
```

Defaults: last 7 days, every project. `--replay` runs each call through the current binary and omits those it now allows, so the report holds only calls that still stop. Groups:

- `approved` / `declined`: a human answered a prompt. `(raised outside the hooks ...)` groups came from Claude Code's own checks or a settings rule, not these guards; the fix there is a `permissions.allow` rule or nothing, so report them and leave them.
- `hook-denied`: a hook blocked the call outright and the agent had to retry. These cost no human time but waste turns, and a false positive here is still a bug.

`--json` emits one record per call when you need to slice further.

Done when you have the report and know which groups are hook-raised.

## Step 2: Triage

Sort every hook-raised group into one of three:

- **Heal**: the guard stopped a call it could have proven safe from the command text alone. Typical shapes: a target the resolver failed to follow (variables, loops, `cd`, braces), a word misread as a path or flag (jq filters, regexes, URLs), a wrapper or nested script the parser did not see through, a non-secret file matched as a credential (`.env.example`, a public key).
- **Keep**: the stop is the point. Publishing (push to main, `gh pr merge`, workflow dispatch), discarding unmerged work (`branch -D`), writes to real AWS, reads or writes of genuine secrets. A frequent prompt here is a config question (an `awsCli.profiles` or `dangerousRm.allowedRoots` entry in `~/.claude/hooks/config.json`), not a code change; report it, since what counts as safe there is the user's machine-specific call.
- **Rewrite hint**: unsafe as written but with a mechanical fix the agent can make itself; a deny whose reason spells out the rewrite saves the human the prompt (see `reasonChainVar` in `internal/rules/dangerousrm`). Apply these like heals.

A group earns Heal only when you can name the proof: the exact property of the command text that rules out every unsafe sibling. If you cannot state it in one sentence, it is Keep.

Bucket every group yourself. Reach for AskUserQuestion only when a group fires often, the code and transcripts leave it genuinely undecidable, and keeping it would leave a prompt the user hits many times a day; one question for that group, never a triage review.

Done when every hook-raised group has a bucket and every Heal has its one-sentence proof.

## Step 3: Heal, one group at a time

For each Heal and Rewrite-hint group, red-green:

1. Add the real command (or a minimal form of it) as a failing case in the rule's `_test.go`, plus its nearest unsafe sibling that must keep asking: the same shape pointed at a path, branch or profile it must not reach.
2. Make it pass in the narrowest place. Shared shell parsing goes in `shellast` so every rule sees through it the same way.
3. `make check` (format, lint, tests). Then `make golden` and read the `testdata/decisions.json` diff: every changed decision must be one you meant. Add a corpus case when the behaviour crosses rules (nested scripts, wrappers).

Done when each healed group has a passing test and `make check` exits 0.

## Step 4: Attack it

Loosening a guard is where silent allows come from. Dispatch two `vet-reviewer` agents in parallel on the uncommitted diff, split by area (for example: resolver and `shellast`; the other rules), each told to hunt commands that are now **allowed** but delete, publish or leak something, with a concrete command, cwd and config for every finding. Name the fail-closed stance and the rule's exemptions in the brief; leave your own conclusions out of it.

Turn every confirmed finding into a red test first, then fix. A heal you cannot make sound gets reverted, test and all, and its group moves back to Keep. Holes that predate your change are in scope when they sit in the code you touched; one you cannot close goes in the report.

Done when every finding is fixed, its heal reverted, or (pre-existing holes only) listed for the report.

## Step 5: Ship

1. `make check` and `make golden` (no unexplained corpus diff), then `make build` to make it live.
2. Re-run Step 1 with `--replay`: every healed group must be gone from the report.
3. Report: groups healed (with the example command and its proof), groups kept and why, heals reverted after Step 4, config changes to consider, and any pre-existing holes left open. Do not commit; the user runs `/commit` when ready.
