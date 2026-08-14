# .zshenv (ZDOTDIR) - Environment for all shell types
# Sourced directly when ZDOTDIR is pre-set, or via ~/.zshenv delegation

# Load Cargo environment if it exists
[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"

# Essential environment variables
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
export EDITOR='nano'

# Opt out of Supabase CLI telemetry. Env-based so it survives Docker
# image/volume prunes that wipe the CLI's own `supabase telemetry disable` state.
export SUPABASE_TELEMETRY_DISABLED=1

# XDG directories
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"
export XDG_CACHE_HOME="$HOME/.cache"

# Initialize Homebrew/Linuxbrew (platform-aware).
# `brew shellenv` forks a process every shell (~20 ms on Mac). Cache the
# output and source the cached file; refresh when brew is upgraded.
_brew_bin=""
for _candidate in /opt/homebrew/bin/brew /usr/local/bin/brew /home/linuxbrew/.linuxbrew/bin/brew; do
    [ -x "$_candidate" ] && { _brew_bin="$_candidate"; break; }
done
if [ -n "$_brew_bin" ]; then
    _brew_cache="${XDG_CACHE_HOME:-$HOME/.cache}/zsh/brew-shellenv.zsh"
    if [ ! -s "$_brew_cache" ] || [ "$_brew_bin" -nt "$_brew_cache" ]; then
        mkdir -p "${_brew_cache%/*}"
        "$_brew_bin" shellenv > "$_brew_cache"
    fi
    . "$_brew_cache"
fi
unset _brew_bin _candidate _brew_cache

# ===== Path helpers =====
# Defined here rather than in .zshrc so PATH construction is available to
# every shell type, and so .zshrc can still use them.
path_append() {
  if [ -d "$1" ] && [[ ":$PATH:" != *":$1:"* ]]; then
    export PATH="$PATH:$1"
  fi
}

path_prepend() {
  if [ -d "$1" ] && [[ ":$PATH:" != *":$1:"* ]]; then
    export PATH="$1:$PATH"
  fi
}

# User bin directories
export PATH="$HOME/bin:$HOME/.local/bin:$PATH"
[ ! -d "$HOME/.local/bin" ] && mkdir -p "$HOME/.local/bin"

path_prepend "$HOME/go/bin"
path_prepend "$HOME/.qlty/bin"

# ===== Node toolchain =====
# pnpm is the single source of truth: the standalone installer provides pnpm
# (never corepack -- upstream advises against installing pnpm that way), and
# `pnpm runtime set` provides node. `dot init` sets both up.
#
# These live in .zshenv, not .zshrc, so non-interactive shells (scripts, git
# hooks, ssh commands, agent tooling) resolve the same node and the same
# registry auth as an interactive terminal does.
export PNPM_HOME="$XDG_DATA_HOME/pnpm"
case ":$PATH:" in
  *":$PNPM_HOME/bin:"*) ;;
  *) export PATH="$PNPM_HOME/bin:$PATH" ;;
esac

# Keep auth tokens (`pnpm login`) out of the tracked ~/.npmrc by redirecting
# the userconfig to an untracked file. The tracked dotfile is promoted to
# globalconfig so its settings still apply. pnpm honours both vars; non-auth
# pnpm settings live in ~/.config/pnpm/config.yaml instead.
export NPM_CONFIG_USERCONFIG="$HOME/.npmrc.local"
export NPM_CONFIG_GLOBALCONFIG="$HOME/.npmrc"

# ===== Bun =====
export BUN_INSTALL="$HOME/.bun"
case ":$PATH:" in
  *":$BUN_INSTALL/bin:"*) ;;
  *) [ -d "$BUN_INSTALL/bin" ] && export PATH="$BUN_INSTALL/bin:$PATH" ;;
esac
