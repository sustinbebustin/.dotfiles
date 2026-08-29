---
name: update-meta-skills
description: Refresh the meta skills (create-agent-skills, create-sub-agents) against the latest Claude Code docs on skills and subagents.
disable-model-invocation: true
allowed-tools: Read, Edit, Glob, Grep, Skill
---

# Update Meta Skills

Keep the meta authoring skills current with the official Claude Code docs.

## Steps

1. **Read the last-reviewed versions.** Open the frontmatter of both skills and note `metadata.last_reviewed_version`:
   - `home/.claude/skills/meta/create-agent-skills/SKILL.md`
   - `home/.claude/skills/meta/create-sub-agents/SKILL.md`

   Get the installed version with `claude --version`. Use the lower of the two recorded versions as the cutoff — only changes released after it matter. If a skill's version already equals the installed version, it's current; review it only if asked.

2. **Pull fresh docs.** Invoke the `claude-code-docs` skill to get current information on agent skills, subagents, slash commands, and anything else these two skills cover (frontmatter fields, invocation control, scopes, permissions, lifecycle). That skill refreshes its own cache when the Claude Code version has changed — don't fetch or update docs yourself.

3. **Diff against the skills.** Compare the docs against the current SKILL.md content (and their `references/`, `templates/`, `workflows/`). Focus on what changed since the cutoff version: new or renamed frontmatter fields, changed defaults, new invocation/permission behavior, new built-in subagents, deprecations.

4. **Apply needed changes.** Edit the skills to reflect any additions or corrections. Keep edits surgical — match existing structure and tone. If nothing changed, make no edits.

5. **Bump the versions.** Set `metadata.last_reviewed_version` to the installed Claude Code version in any skill you reviewed (whether or not you edited content), so the next run knows the new cutoff.

6. **Report.** Briefly summarize what changed (or "no changes needed") per skill.
