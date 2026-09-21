# dotfiles

Stow-based dotfiles; `home/` mirrors `$HOME`. Layout, commands, and the stow model are in `README.md`; read `docs/architecture.md` before touching `dot`, zsh startup files, the Node toolchain, or the hooks.

## Editing live config

Everything under `home/` is symlinked into `$HOME`, so an edit here is **live** the moment it is saved -- in this session and every other one.

- `home/.claude/CLAUDE.md`, `rules/`, `agents/`, `skills/`, and `settings.json` are the global agent config for every project. This file governs only this repo.
- Stow links per file (`--no-folding`), so a new file under `home/` is dead until `dot stow` runs. Skills link per folder: a new file in an existing skill is live at once; a new skill needs `dot stow`.
- Skill category dirs (`skills/<category>/<name>/`) are flattened when published, so a skill name must be unique across all categories.
- Skills listed in `skills/vendored-skills.json` are third-party copies; add, update, and delete them with `/vendor-skills`. A hand edit to one needs a matching `localChanges` note in the catalog, or the next update may drop it.
- A file that must stay out of `$HOME` goes in `home/.stow-local-ignore`, mirrored re-anchored in `home/.claude/.stow-local-ignore` (the `~/.claude-work` stow). Keep the two in step.
- The repo is public. Machine-specific or secret values go in gitignored local files (`~/.claude/hooks/config.json`, `~/.npmrc.local`), with a tracked `.example` recording the shape.

## Checks

- `dot`: `shellcheck dot` stays clean, and the script must parse under bash 3.2 (stock macOS).
- Hooks (`home/.claude/hooks/`): `make check` is the gate. A change to what the guards allow shows up in `make golden`; review that diff.

## Commits

Conventional commits scoped by area: `feat(skills): ...`, `fix(hooks): ...`, `chore(claude): ...`, `feat(dot): ...`.
