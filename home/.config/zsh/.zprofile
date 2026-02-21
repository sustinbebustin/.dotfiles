# .zprofile - Executed for login shells
# Homebrew init and base PATH setup moved to .zshenv (runs for all shell types)

# libpq (PostgreSQL) - only if installed
if [ -d "$HOMEBREW_PREFIX/opt/libpq" ]; then
    export PATH="$HOMEBREW_PREFIX/opt/libpq/bin:$PATH"
    export LDFLAGS="-L$HOMEBREW_PREFIX/opt/libpq/lib"
    export CPPFLAGS="-I$HOMEBREW_PREFIX/opt/libpq/include"
    export PKG_CONFIG_PATH="$HOMEBREW_PREFIX/opt/libpq/lib/pkgconfig"
fi
