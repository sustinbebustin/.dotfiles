---
name: heal
description: Review the nudges ask-dont-tell sent, find the false nudges, and autonomously fix its reviewer or gate against labelled cases.
argument-hint: "[--since 14d]"
disable-model-invocation: true
allowed-tools: Bash(uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py *)
metadata:
  author: sustinbebustin
---

# Heal ask-dont-tell

The mod lives two levels above this skill, at `${CLAUDE_SKILL_DIR}/../..`, a symlink into the dotfiles repo (`home/.claude/mods/ask-dont-tell`). Its parts:

- `hooks/register.ts`: the gate. Which turns get reviewed (main loop, interactive, answered, not a nudge's own turn), the reviewer `MODEL`, and when a nudge is dropped.
- `hooks/verdict.ts`: the judgement. `Verdict.SYSTEM` asks the reviewer to list each pending item with a kind; `NUDGE_KINDS` (`ask`, `offer`, `todo`) are the kinds that nudge; `Verdict.nudge` is the text Claude reads back.
- `tests/reviewer-cases.json`: labelled replies, each `expect`ing `open` or `done`. This is the regression set: every heal adds to it and none may break it.

Edits are live in every session at the mod's next reload. The run is autonomous: you decide every record, apply every heal, and hand the user a finished report.

The target is the **false nudge**: a nudge on a reply that left nothing anyone could act on now. Each one spends a turn and teaches the user to ignore the mod. Heal only the nudges that fired; the `open` cases in the regression set are the boundary every heal keeps, so a fix that silences a labelled real nudge is not a fix.

## Step 1: Mine

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py fired --replay $ARGUMENTS
```

Each record is one nudge sent: the reviewed reply, the items named, the `outcome` of the nudged turn (`asked`, `acted`, `dismissed`: Claude said nothing was open, `ignored`), the user's next prompt, and `now`, the current reviewer's verdict. `LIKELY-FP` marks a dismissal or a user complaint. `--json` gives full records when you need the whole reply.

Done when you have the report.

## Step 2: Triage

Read every record, `LIKELY-FP` first. The flag is a floor: a false nudge Claude obeyed (a pointless AskUserQuestion, an unrequested action) carries no flag, so `asked` and `acted` records are suspects too. Sort each into one bucket:

- **False nudge**: the reply left nothing anyone could act on now, yet the reviewer named an item. Heal the reviewer.
- **Gate**: the turn should never have been reviewed, or the nudge should have been dropped (a notification-driven turn, a reply superseded by a newer turn). Heal `register.ts`.
- **Agent fault**: the nudge was right and Claude dismissed or mishandled it. Heal `Verdict.nudge` only if its wording caused that, otherwise report it.
- **Correct**: the reply did leave a question, offer, or actionable leftover; the nudge earned its turn. Leave it.

A false nudge earns a heal only when you can say in one sentence why the item the reviewer named was not actionable now (already done, purely informational, the user's own future choice with nothing to ask). If you cannot, it is Correct. A record already at `now=done` may be healed by an earlier change; it still gets a case if Step 3's check goes red.

Done when every record has a bucket and every heal has its one-sentence reason.

## Step 3: Heal, one group at a time

Group records that share a cause. For each reviewer group, red-green:

1. Add one case to `tests/reviewer-cases.json`: a minimal paraphrase of the real reply that keeps the shape that misled the reviewer, with `expect: "done"` and a one-line `why`. The repo is public: write it with no company, client, product, person, repo, URL, account id, or hostname from the transcript.
2. `uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py check --id <case-id> --runs 4`: it must go **red**. A case already green needs no heal; keep it only if it guards the boundary.
3. Fix in the narrowest place: a counter-example inside the matching kind's definition in `Verdict.SYSTEM` (what that kind is not) before a new rule. Switching `MODEL` in `register.ts` is the last resort, after wording fails twice.
4. `check --id <case-id> --runs 4` goes green: every run agrees with the label.

A gate group heals in `register.ts` with a failing test first in `tests/ask-dont-tell.test.ts`, then `claude plugin test ${CLAUDE_SKILL_DIR}/../..`.

Done when each healed group has a case or test that went red, then green.

## Step 4: Regress

1. `uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py check --runs 4`: every case green on every run. A case that flips once is red: the reviewer is not deterministic, so one disagreement is a real error rate. An `open` case going red means the heal over-reached; tighten the wording, or revert the heal and move its group back to Correct.
2. Re-run the Step 1 command. Every healed record must read `now=done`, and every record triaged Correct must still read `now=open`.

Done when `check` exits 0 and the replay shows every heal landed with no Correct nudge silenced.

## Step 5: Ship

1. From the mod root: `claude plugin validate . --strict`, `claude plugin test .`, and `tsc -p .`.
2. Report: groups healed (the case id and its reason), records left as Correct or Agent fault and why, heals reverted in Step 4, and the replay's nudge count before and after. Do not commit; the user runs `/commit` when ready.
