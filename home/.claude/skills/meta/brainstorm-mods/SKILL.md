---
name: brainstorm-mods
description: Brainstorm mods and plugins worth building, for one project or globally, then design and build the one you pick.
argument-hint: "global | project [path]"
disable-model-invocation: true
metadata:
  author: sustinbebustin
---

# Brainstorm Mods

Arguments: `$ARGUMENTS`

Two sources of ideas, kept apart on purpose: **evidence** (friction the user's own sessions show) and **invention** (what mods make possible that the user would never think to ask for). The inventor never sees the evidence, so its ideas are not anchored to what the user already does.

## Steps

1. **Scope.** `global` covers every session and the global setup (`~/.claude/`). `project` covers one project: the given path, or the working directory. With no scope in the arguments, ask with `AskUserQuestion` and wait.

2. **Profile.** Write a short profile of the user for the inventor. Keep it to what they are, not what they struggle with:
   - The stack: project manifests (`go.mod`, `package.json`, ...) for a project, or the languages across `~/dev` for global.
   - The setup that exists, by name only: skills, agents, mods, hooks, plugins (`~/.claude/` and, for a project, `<project>/.claude/`).
   - How they work, in two or three lines: worktrees, review pipelines, which services they query.

   Done when the profile fits in about 20 lines.

3. **Mine and invent.** Dispatch two `general-purpose` agents in one message, so they run in parallel.
   - **Evidence agent.** Brief: run `python3 ~/.claude/skills/brainstorm-mods/scripts/mine-sessions.py` (with `--root <project>` for project scope), read the existing setup, and return each recurring friction: what keeps happening, how often, a quoted example, and which existing tool, if any, already touches it. Return frictions, not solutions.
   - **Inventor agent.** Brief: the profile from step 2 and nothing about their sessions. Read `~/.claude/context/plugins__mods__overview.md`, `plugins__mods__reference.md`, `plugins__mods__api.md`, and `plugins__mods__interface.md` for everything a mod can do (run `bash ~/.claude/skills/claude-docs/scripts/refresh-if-outdated.sh` first). Invent at least 15 mods across different kinds of value: ambient awareness, delight and play, the model working in the background, memory across sessions, guarding and steering, things the user will not think to ask for. Each: one scenario of the user's day with it, and the capabilities it rests on. Every idea must be buildable with what the docs describe; wild is wanted, impossible is not.

4. **Cross-pollinate.** Once both have returned, send the evidence agent's frictions to the inventor with `SendMessage`. Its own list is already written, so the evidence can no longer anchor it. Brief: for each friction, the boldest fix a mod allows, not the obvious one, and which of its earlier ideas already answer one.

5. **Shortlist.** For each friction, weigh the inventor's fix against the plain one (skill, agent, hook, or one rule line) and keep whichever serves the user better; drop frictions an existing tool already handles. Present two groups, **Grounded** (with its evidence) and **Invented**, about six each, best first. Write each as a scenario in plain words: what the user sees and does. No API names.

   Done when the user picks one or more ideas through `AskUserQuestion` (`multiSelect: true`).

6. **Design.** Invoke `grilling` on the picked ideas. Done when it reaches shared understanding.

7. **Build.** Invoke `create-plugins` with the settled design and the scope from step 1.
