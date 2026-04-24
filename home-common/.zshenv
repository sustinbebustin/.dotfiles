# .zshenv - Bootstrap for all shells
# Sets ZDOTDIR then delegates to $ZDOTDIR/.zshenv for actual env setup

export ZDOTDIR="$HOME/.config/zsh"
[ -f "$ZDOTDIR/.zshenv" ] && source "$ZDOTDIR/.zshenv"

# sentry
fpath=("/home/austin/.local/share/zsh/site-functions" $fpath)
