# .zprofile - Executed for login shells

# Initialize Homebrew (platform-aware)
if [ -f /opt/homebrew/bin/brew ]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
elif [ -f /usr/local/bin/brew ]; then
    eval "$(/usr/local/bin/brew shellenv)"
elif [ -f /home/linuxbrew/.linuxbrew/bin/brew ]; then
    eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
fi

# User bin directories
export PATH="$HOME/bin:$HOME/.local/bin:$PATH"

if [ ! -d "$HOME/.local/bin" ]; then
    mkdir -p "$HOME/.local/bin"
fi

# libpq (PostgreSQL) - only if installed
if [ -d "$HOMEBREW_PREFIX/opt/libpq" ]; then
    export PATH="$HOMEBREW_PREFIX/opt/libpq/bin:$PATH"
    export LDFLAGS="-L$HOMEBREW_PREFIX/opt/libpq/lib"
    export CPPFLAGS="-I$HOMEBREW_PREFIX/opt/libpq/include"
    export PKG_CONFIG_PATH="$HOMEBREW_PREFIX/opt/libpq/lib/pkgconfig"
fi
