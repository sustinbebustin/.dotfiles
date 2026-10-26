---
verify: shellcheck dot && make -C home/.claude/hooks check
deploy: everything under home/ is symlinked into $HOME, so a saved edit is live at once, in every session
deploy: a new file under home/ is dead until `dot stow` runs; a new skill folder likewise
---

## Baseline

Stow-based dotfiles: `home/` mirrors `$HOME`. `README.md` has the layout and commands; `docs/architecture.md` has the stow model, `dot`, zsh startup order, the Node toolchain, and the hooks.

## Project rules

- `dot` and every script under `home/.claude/skills/` run on stock macOS: bash 3.2 and BSD userland. A bash 4+ feature or a GNU-only flag is blocking.
- `home/.claude/settings.json` is gitignored. Settings the setup needs go in `settings.base.json`, merged by `dot stow`, which never deletes keys from the live file.
- Skill category folders are flattened when published: a skill name must be unique across `skills/*/`.
- A skill listed in `skills/vendored-skills.json` is a third-party copy: a hand edit needs a matching `localChanges` note.
- `home/.stow-local-ignore` and `home/.claude/.stow-local-ignore` mirror each other; a change to one without the other is a finding.
- The repo is public: machine-specific or secret values belong in gitignored local files with a tracked `.example`.
- Hooks (`home/.claude/hooks/`): `make check` is the gate, and a change to what a guard allows shows in `make golden`. Loosening a guard is `business_risk: possible`.
- `home/.claude/CLAUDE.md`, `rules/`, `agents/`, and `skills/` are global agent instructions for every project: review them as prompts -- an ambiguous or contradictory instruction is a defect.
