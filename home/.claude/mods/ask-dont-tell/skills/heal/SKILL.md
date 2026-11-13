---
name: heal
description: Mine where ask-dont-tell nudged wrongly or stayed silent when it should have nudged, then autonomously fix its reviewer against labelled cases.
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

The two failures cost differently but both cost. A **miss** forces the user to type "what's next?" or "use AskUserQuestion". A **false nudge** spends a turn and teaches the user to ignore the mod. A heal trades neither for the other: a fix that flips any labelled case is not a fix.

## Step 1: Mine

```bash
uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py fired --replay $ARGUMENTS
uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py misses --replay --controls 30 $ARGUMENTS
```

- `fired`: each nudge sent, with the reviewed reply, the items named, the `outcome` of the nudged turn (`asked`, `acted`, `dismissed`: Claude said nothing was open, `ignored`), the user's next prompt, and `now`, the current reviewer's verdict. `LIKELY-FP` marks a dismissal or a user complaint.
- `misses`: prompts where the user nudged by hand (`ask`, `continue`, `affirm`: a bare "yes" to a prose offer) after a reply the mod let through, plus 30 `control` turns. `now=done` on a non-control is a live miss.

`--json` gives full records when you need the whole reply. Done when you have both reports.

## Step 2: Triage

Read every `LIKELY-FP` and every non-control `now=done` in full, then sort each into one bucket:

- **False nudge**: the reply left nothing anyone could act on now. Heal the reviewer.
- **Miss**: the reply left a question, offer, or actionable leftover in prose. Heal the reviewer.
- **Gate**: the reviewer judged correctly but the turn should never have been reviewed, or the nudge should have been dropped (a notification-driven turn, a reply superseded by a newer turn). Heal `register.ts`.
- **Agent fault**: the nudge was right and Claude dismissed or mishandled it. Heal `Verdict.nudge` only if its wording caused that, otherwise report it.
- **Correct**: the mod was right; the miner's regex misread the prompt (a redirect, a new task). Leave it.

A record earns a heal only when you can name the item kind the reply's pending thing should have had and why. If you cannot say it in one sentence, it is Correct. Read the controls the reviewer calls open: a false nudge among them is triaged like any other.

Done when every flagged record has a bucket and every heal has its one-sentence reason.

## Step 3: Heal, one group at a time

Group records that share a cause. For each reviewer group, red-green:

1. Add one case to `tests/reviewer-cases.json`: a minimal paraphrase of the real reply that keeps the shape that misled the reviewer, with `expect` and a one-line `why`. The repo is public: write it with no company, client, product, person, repo, URL, account id, or hostname from the transcript.
2. `uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py check --id <case-id> --runs 4`: it must go **red**. A case already green needs no heal; keep it only if it guards the boundary.
3. Fix in the narrowest place: an example inside the matching kind's definition in `Verdict.SYSTEM` before a new rule; a new kind (added to the `ItemKind` union, `ITEM_KINDS`, and `NUDGE_KINDS` if it nudges, with a `Verdict.parse` test) only when no existing kind fits. Switching `MODEL` in `register.ts` is the last resort, after wording fails twice.
4. `check --id <case-id> --runs 4` goes green: every run agrees with the label.

A gate group heals in `register.ts` with a failing test first in `tests/ask-dont-tell.test.ts`, then `claude plugin test ${CLAUDE_SKILL_DIR}/../..`.

Done when each healed group has a case or test that went red, then green.

## Step 4: Regress

1. `uv run ${CLAUDE_SKILL_DIR}/scripts/mine.py check --runs 4`: every case green on every run. A case that flips once is red: the reviewer is not deterministic, so one disagreement is a real error rate. Fix the wording, or revert the heal and move its group back to Correct.
2. Re-run both Step 1 commands. Healed records must flip (`now=done` for false nudges, `now=open` for misses), and the controls' open count must not rise without each new one being a real open item.

Done when `check` exits 0 and the replays show every heal landed with no new false nudges.

## Step 5: Ship

1. From the mod root: `claude plugin validate . --strict`, `claude plugin test .`, and `tsc -p .`.
2. Report: groups healed (the case id and its reason), records left as Correct or Agent fault and why, heals reverted in Step 4, and the replay numbers before and after. Do not commit; the user runs `/commit` when ready.
