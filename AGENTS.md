# dotfiles

Stow-based dotfiles; `home/` mirrors `$HOME`. Layout, commands, and the stow model are in `README.md`; read `docs/architecture.md` before touching `dot`, zsh startup files, the Node toolchain, or the hooks.

## Editing live config

Everything under `home/` is symlinked into `$HOME`, so an edit here is **live** the moment it is saved -- in this session and every other one.

- The global agent config for every project is shared between Claude Code and Factory Droid: `home/.agents/` holds what both read (`AGENTS.md`, `skills/`, `agents/`, `commands/`, `hooks/`); `home/.claude/` and `home/.factory/` hold only what one harness reads (`rules/` is Claude-only). `home/.claude/CLAUDE.md` is a symlink to `home/.agents/AGENTS.md`; edit `AGENTS.md`. This file governs only this repo.
- Droid gets the agents as droids that `dot stow` generates into `~/.factory/droids`; edit the agent, never the droid. Agents that list the `Agent` tool are skipped, since droids can't spawn subagents.
- `home/.claude/settings.json` and `home/.factory/settings.json` are gitignored (still stowed and live). Settings this setup needs go in the `settings.base.json` beside each, which is not live until `dot stow` merges it into `settings.json`; personal ones (`autoMode`, model, UI toggles) go only in `settings.json`.
- Stow links per file (`--no-folding`), so a new file under `home/` is dead until `dot stow` runs. Skills link per folder and `commands/` and `agents/` as whole dirs: a new file in an existing skill, a new command, or a new agent is live at once in Claude Code; a new skill needs `dot stow`, and so does a new or changed agent for Droid.
- Skill category dirs (`skills/<category>/<name>/`) are flattened when published, so a skill name must be unique across all categories.
- Skills listed in `skills/vendored-skills.json` are third-party copies; add, update, and delete them with `/vendor-skills`. A hand edit to one needs a matching `localChanges` note in the catalog, or the next update may drop it.
- A file that must stay out of `$HOME` goes in `home/.stow-local-ignore`, mirrored re-anchored in `home/.claude/.stow-local-ignore` (the `~/.claude-<account>` stows). Keep the two in step.
- The repo is public. Machine-specific or secret values go in gitignored local files (`~/.agents/hooks/config.json`, `~/.npmrc.local`), with a tracked `.example` recording the shape.

## Checks

- `dot`: `shellcheck dot` stays clean, and the script must parse under bash 3.2 (stock macOS).
- Hooks and the droid generator (`home/.agents/hooks/`): `make check` is the gate. A change to what the guards allow shows up in `make golden`; review that diff.

## Commits

Conventional commits scoped by area: `feat(skills): ...`, `fix(hooks): ...`, `chore(claude): ...`, `feat(dot): ...`.
