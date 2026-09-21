---
name: resume-subagents
description: Resume subagents (and their nested subagents) that stopped mid-task when session credits or rate limits ran out, instead of spawning fresh ones.
disable-model-invocation: true
metadata:
  author: sustinbebustin
---

# Resume Subagents

Pick up the interrupted work where it stopped. Every subagent that died mid-task is **resumed**, not replaced: a resumed agent keeps its full context, a fresh one starts blind.

## 1. Inventory the stopped agents

List every subagent you dispatched that never returned a final result. Their IDs and names are in your earlier Agent tool results; `ListAgents` shows the ones still addressable. Load `SendMessage` (and `ListAgents`) via `ToolSearch` if they appear only as deferred tools.

Done when every unfinished dispatch has an ID or name, or is marked unknown.

## 2. Resume each one with SendMessage

Send each stopped agent this message, filling in nothing but what is specific to it:

> Your run was interrupted when the session hit its usage limit. Resume your task exactly where you left off; don't restart completed work. If you had dispatched subagents of your own that stopped without returning, resume each of them with SendMessage and pass this same message on. Only if a resume fails, start a replacement seeded from the stopped agent's transcript at `<session-dir>/subagents/agent-<id>.jsonl`. Then finish your task and report as originally asked.

## 3. Replace only what cannot be resumed

When SendMessage fails for an agent (unknown ID, agent gone), rebuild its context from the transcript before dispatching a replacement:

- Transcripts live at `~/.claude/projects/<cwd-with-slashes-as-dashes>/${CLAUDE_SESSION_ID}/subagents/agent-<id>.jsonl`; the sibling `agent-<id>.meta.json` holds its `description` and `agentType`, which identifies it when the ID is unknown. Nested subagents land in the same directory.
- Read the transcript's tail to find the last completed action, files already changed, and findings so far.
- Brief the replacement with: the original task, what the transcript shows is already done, the exact next step, and the transcript path so it can read more itself.

Done when every stopped agent is either resumed or replaced with a transcript-seeded brief, and you report to the user which was which.
