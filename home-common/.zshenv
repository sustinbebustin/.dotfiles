# .zshenv - Bootstrap for all shells
# Sets ZDOTDIR then delegates to $ZDOTDIR/.zshenv for actual env setup

export ZDOTDIR="$HOME/.config/zsh"
[ -f "$ZDOTDIR/.zshenv" ] && source "$ZDOTDIR/.zshenv"

# Sentry CLI completion (Linux dev box only).
[ -d "/home/austin/.local/share/zsh/site-functions" ] && \
  fpath=("/home/austin/.local/share/zsh/site-functions" $fpath)
