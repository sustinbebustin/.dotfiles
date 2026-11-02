---
name: create-mod
description: Build a personal Claude Code mod with hot reload, then install it globally or into one project.
metadata:
  author: sustinbebustin
---

# Creating Mods

A mod is a plugin whose `hooks/hooks.json` lists in-process hook modules. The bundled `plugin-authoring` skill owns the writing: the API, this build's types, examples, and hot reload. This skill owns the decisions around it and the move from its dev folder into place.

## Steps

1. **Scope.** Ask the user unless the request settles it:
   - **Global**: loads in every session.
   - **Project**: loads only in sessions started at one project's root.

   Done when the scope is named.

2. **Form of each piece.** A mod can register commands, tools, and subagent types in code, and can also ship plain `skills/` and `agents/` files beside it, like any plugin:
   - A command that answers by itself, with no turn for Claude: code.
   - A tool Claude calls: code.
   - A procedure Claude carries out, or guidance for how it reasons: a skill file. Invoke `create-skills`.
   - A subagent: an agent file by default. Invoke `create-agents`. Register it in code only when part of its definition is known at session start alone (the repo it runs in, what is present there), or when it must stay hidden behind the mod's own tool.

   Done when every piece has a form.

3. **Build.** Load `plugin-authoring` and build the mod in the dev folder it names, with hot reload. Set `author` in `plugin.json` to the output of `gh api user --jq .login`.

   Done when the user has exercised the mod in this session and `claude plugin validate <dir> --strict`, `claude plugin test <dir>`, and `tsc -p <dir>` pass.

4. **Install.** Move the folder out of the dev folder; a copy left there loads under the same name.
   - **Global**: move it to `home/.claude/mods/<name>/` in the dotfiles repo, then run `dot stow`, which links it into `~/.claude/mods` as one folder symlink. Stow's per-file links fail: the loader rejects a module whose real path lies outside the plugin folder.
   - **Project**: move it to `<project>/.claude/skills/<name>/`; no stow. It loads only in a session started at that root, not a subdirectory, after the folder is trusted. In a repo, gitignore the mod's `.claude-plugin/types/`, which Claude Code writes on load.

   Done when the dev folder holds no copy and the installed folder is in place.

5. **Verify.** Re-run validate and test against the installed folder, then start a new session there (for project scope, at the project root). Done when `claude plugin list` shows the mod loaded: `<name>@inline` for global, `<name>@skills-dir` for project.
