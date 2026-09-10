#!/usr/bin/env bash
# Repo-scoped @ file autocomplete for the dotfiles repo.
#
# The built-in suggester also surfaces ~/.claude paths, which are the stow
# symlink targets of home/.claude in this repo -- the same files under a second
# name, always ranked first. Suggest only paths under the repo itself.
#
# stdin:  {"query": "..."}  stdout: newline-separated repo-relative paths.

set -uo pipefail

query=$(jq -r '.query // ""')

cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0

{
  # Tracked + untracked files, honoring .gitignore.
  rg --files --hidden --glob '!.git' 2>/dev/null
  # Directories, so partial paths like "skills/git/" complete too.
  find . -mindepth 1 -type d -name .git -prune -o -mindepth 1 -type d -printf '%P/\n' 2>/dev/null
} |
  grep -v '^$' |
  if [ -n "$query" ]; then grep -i -F -- "$query"; else cat; fi |
  awk '{ print length($0), $0 }' |
  sort -n -k1,1 |
  cut -d' ' -f2- |
  head -15
