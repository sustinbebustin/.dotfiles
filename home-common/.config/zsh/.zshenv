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

# Initialize Homebrew/Linuxbrew (platform-aware)
if [ -f /opt/homebrew/bin/brew ]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
elif [ -f /usr/local/bin/brew ]; then
    eval "$(/usr/local/bin/brew shellenv)"
elif [ -f /home/linuxbrew/.linuxbrew/bin/brew ]; then
    eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
fi

# User bin directories
export PATH="$HOME/bin:$HOME/.local/bin:$PATH"
[ ! -d "$HOME/.local/bin" ] && mkdir -p "$HOME/.local/bin"
