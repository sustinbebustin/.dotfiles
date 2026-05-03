# .zshenv (ZDOTDIR) - Environment for all shell types
# Sourced directly when ZDOTDIR is pre-set, or via ~/.zshenv delegation

# Load Cargo environment if it exists
[ -f "$HOME/.cargo/env" ] && . "$HOME/.cargo/env"

# Essential environment variables
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
export EDITOR='nano'

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

# User bin directories
export PATH="$HOME/bin:$HOME/.local/bin:$PATH"
[ ! -d "$HOME/.local/bin" ] && mkdir -p "$HOME/.local/bin"
