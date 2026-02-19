# .zshrc - Configuration for interactive shells

# ===== Key bindings =====
bindkey '^[^?' backward-kill-word  # Option+Backspace: delete word

# ===== Completions =====
autoload -Uz compinit
compinit

# ===== Path Helpers =====
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

# ===== PATH =====
path_prepend "$HOME/.local/bin"
path_prepend "$HOME/go/bin"
path_prepend "$HOME/.bun/bin"
path_prepend "$HOME/.qlty/bin"
path_prepend "$HOME/.opencode/bin"

# ===== Node.js =====
export NVM_DIR="$HOME/.nvm"
# Load nvm from homebrew (macOS) or standard location
if [ -s "$HOMEBREW_PREFIX/opt/nvm/nvm.sh" ]; then
  \. "$HOMEBREW_PREFIX/opt/nvm/nvm.sh"
  [ -s "$HOMEBREW_PREFIX/opt/nvm/etc/bash_completion.d/nvm" ] && \. "$HOMEBREW_PREFIX/opt/nvm/etc/bash_completion.d/nvm"
elif [ -s "$NVM_DIR/nvm.sh" ]; then
  \. "$NVM_DIR/nvm.sh"
fi

path_prepend "$HOME/.npm-packages/bin"

# ===== Bun =====
export BUN_INSTALL="$HOME/.bun"
[ -s "$BUN_INSTALL/_bun" ] && source "$BUN_INSTALL/_bun"

# ===== SSH Agent (macOS) =====
if [[ "$OSTYPE" == "darwin"* ]]; then
  export APPLE_SSH_ADD_BEHAVIOR=macos
  ssh-add --apple-load-keychain > /dev/null 2>&1
fi

# ===== Aliases =====
alias l="ls"
alias ll="ls -al"
alias c="clear"
alias t="eza . --tree --level=1"
alias tt="eza . --tree --level=2"
alias ttt="eza . --tree --level=3"

# macOS-only aliases
if [[ "$OSTYPE" == "darwin"* ]]; then
  alias o="open ."
  alias code="open -a Cursor"
fi

# Git
alias gpl='git pull'
alias gaa='git add .'
alias gcm='git commit -m'
alias gpsh='git push'
alias gss='git status -s'
alias gs='echo ""; echo "*********************************************"; echo -e "   DO NOT FORGET TO PULL BEFORE COMMITTING"; echo "*********************************************"; echo ""; git status'

# ===== Colored man pages =====
export LESS_TERMCAP_mb=$'\e[1;31m'
export LESS_TERMCAP_md=$'\e[1;36m'
export LESS_TERMCAP_me=$'\e[0m'
export LESS_TERMCAP_se=$'\e[0m'
export LESS_TERMCAP_so=$'\e[1;44;33m'
export LESS_TERMCAP_ue=$'\e[0m'
export LESS_TERMCAP_us=$'\e[1;32m'

# ===== Plugins (via homebrew) =====
if [ -n "$HOMEBREW_PREFIX" ]; then
  [ -f "$HOMEBREW_PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh" ] && \
    source "$HOMEBREW_PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh"
  [ -f "$HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh" ] && \
    source "$HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh"
fi

# ===== Prompt =====
eval "$(starship init zsh)"
