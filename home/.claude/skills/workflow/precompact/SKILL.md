---
name: precompact
description: Write a handoff to yourself before compacting, so the current work survives the summary.
disable-model-invocation: true
argument-hint: "What are we about to do next? (optional)"
metadata:
  author: sustinbebustin
---

The user is about to run `/compact` and then keep working with you in this same session. Compaction replaces the conversation with a summary: early instructions, tool output, and reasoning you have not written down are gone afterwards. Write the handoff that carries this work across that boundary.

Invoke the `handoff` skill with the Skill tool now, and follow it with these adjustments:

- **The reader is you, after compaction** — not a fresh agent on a new task. Write what you would be stuck without: the user's decisions and constraints as they stated them, what you have already established and ruled out, the exact state of in-flight work, and the next concrete action.
- **Omit the suggested-skills section.** The user drives what comes next.
- **Save the handoff last**, so it is the most recently modified file: compaction re-reads up to five files, most-recently-modified first, and this one earns a slot. Keep it under 5,000 tokens or it comes back as a bare path reference instead of content.
- **Report the absolute path** in your reply, so the user can point you back at it if the re-read misses it.

If the user passed arguments, they name what the session does after compacting; weight the handoff toward what that work needs.

Close by telling the user the handoff is written and they can run `/compact`.
