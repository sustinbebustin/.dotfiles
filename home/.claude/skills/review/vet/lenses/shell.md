# Lens: shell

Shell scripts and shell startup files. Run `shellcheck` on each changed script (read-only) and fold its warnings into your evidence; then look for what it cannot see. Respect the target shell the overlay or shebang names -- a script that must run under bash 3.2 (stock macOS) or POSIX `sh` is held to that.

## Targets

- **Quoting and splitting.** Unquoted expansions that word-split or glob, `$@` versus `$*`, `read` without `-r`, filenames with spaces or leading dashes reaching a command without `--`.
- **Strict-mode traps.** `set -e` disabled inside conditions, `&&` chains, and functions called from them; `local x=$(cmd)` masking the exit status; `set -u` on an empty array (`"${arr[@]}"` errors before bash 4.4); `pipefail` turning a reader that exits early (`head`, `grep -q`) into a SIGPIPE failure upstream.
- **Portability.** Bash 4+ features in a 3.2 script (associative arrays, `${x,,}`, `mapfile`, `|&`, `;&`, negative indices); GNU-only flags where BSD tools run (`sed -i` without a suffix, `grep -P`, `date -d`, `readlink -f`, `stat -c`, `xargs -r`, `find -printf`, sed's `I` flag); `[[ ]]` or arrays in a `sh` script.
- **Failure handling.** `cd` without a guard, temp files without `mktemp` and a cleanup `trap`, errors written to stdout, exit codes that report success after a failure, a pipeline whose failure goes unseen.
- **Injection.** `eval`, unvalidated input interpolated into a command, `sh -c` with user data spliced into the string.
- **Startup files.** A slow command or network call on every shell start, an exported variable that should be local to one tool, ordering assumptions between `.zshenv`, `.zprofile`, and `.zshrc`.
