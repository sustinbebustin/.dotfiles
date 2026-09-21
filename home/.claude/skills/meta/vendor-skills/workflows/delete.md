# Delete

Input: a skill name, and optional `--project`.

- **`--project`:** confirm with AskUserQuestion, then remove `<git toplevel>/.claude/skills/<name>`.
- **Global:**
  1. Run `vendor.ts catalog show <name>`. If the skill isn't catalogued, it isn't vendored. Stop, and tell the user that personal skills are deleted by hand.
  2. Confirm with AskUserQuestion. Show `<root>/<localPath>`, and say whether `localChanges` is non-empty: those edits then survive only in git history.
  3. Remove `<root>/<localPath>`, then run `vendor.ts catalog remove <name>`.
  4. When `needsPublish` is true, run the publish step from SKILL.md. `dot stow` prunes the dangling link. The skill is gone when `~/.claude/skills/<name>` no longer exists.
