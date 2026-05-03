# .zshrc - Configuration for interactive shells

# ===== Key bindings =====
bindkey '^[^?' backward-kill-word  # Option+Backspace: delete word

# ===== Completions =====
fpath=("$ZDOTDIR/functions" $fpath)
typeset -U fpath path  # dedupe fpath/path so compinit scans each dir once
autoload -Uz compinit
# Defer compinit until just after the first prompt is drawn. We register a
# zle file-descriptor handler on a no-op fd; zle invokes it once the line
# editor is active, so the prompt renders without waiting for compinit
# (~290 ms with this fpath). A `compdef` stub absorbs early calls from
# plugins so they don't error before the real compdef is loaded.
_compinit_deferred() {
  emulate -L zsh
  setopt extendedglob
  local zcache="${XDG_CACHE_HOME:-$HOME/.cache}/zsh"
  local zdump="$zcache/zcompdump"
  [[ -d "$zcache" ]] || mkdir -p "$zcache"
  if [[ -n $zdump(#qN.mh+24) ]]; then
    compinit -d "$zdump"
  else
    compinit -C -d "$zdump"
  fi
  if [[ -s "$zdump" && ( ! -s "${zdump}.zwc" || "$zdump" -nt "${zdump}.zwc" ) ]]; then
    zcompile "$zdump"
  fi
  # Replay any buffered compdef calls from plugins loaded eagerly.
  local cmd
  for cmd in "${_compdef_buffer[@]}"; do
    eval "compdef $cmd"
  done
  unset _compdef_buffer
  # Source brew plugins (autosuggestions, syntax-highlighting) lazily.
  local plugin
  for plugin in "${_zsh_brew_plugins_to_load[@]}"; do
    source "$plugin"
  done
  unset _zsh_brew_plugins_to_load
  unfunction _compinit_deferred 2>/dev/null
}
typeset -ga _compdef_buffer
compdef() { _compdef_buffer+=("$*") }

# Fire after the first prompt is rendered. zle -F triggers on fd activity;
# we use fd 0 inside zle, but the simpler path is `zle-line-init` which
# runs once when the line editor accepts input.
_compinit_kick() {
  zle -N zle-line-init
  _compinit_deferred
}
autoload -Uz add-zsh-hook
zle -N zle-line-init _compinit_kick

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

# ===== Node.js (lazy NVM) =====
# Sourcing nvm.sh costs ~110 ms. Defer it until a node-related command is
# actually used. The first invocation pays the load cost, subsequent ones
# are free.
export NVM_DIR="$HOME/.nvm"
_nvm_script=""
if [ -s "$HOMEBREW_PREFIX/opt/nvm/nvm.sh" ]; then
  _nvm_script="$HOMEBREW_PREFIX/opt/nvm/nvm.sh"
elif [ -s "$NVM_DIR/nvm.sh" ]; then
  _nvm_script="$NVM_DIR/nvm.sh"
fi
if [ -n "$_nvm_script" ]; then
  _nvm_completion="$HOMEBREW_PREFIX/opt/nvm/etc/bash_completion.d/nvm"
  _lazy_nvm() {
    unset -f nvm node npm npx pnpm yarn corepack 2>/dev/null
    \. "$_nvm_script"
    [ -s "$_nvm_completion" ] && \. "$_nvm_completion"
  }
  for _cmd in nvm node npm npx pnpm yarn corepack; do
    eval "${_cmd}() { _lazy_nvm; ${_cmd} \"\$@\"; }"
  done
  unset _cmd
fi

path_prepend "$HOME/.npm-packages/bin"

# ===== Bun =====
# Put the bun completion (37 KB) on fpath so compinit auto-discovers it
# instead of sourcing the whole file at every shell startup.
export BUN_INSTALL="$HOME/.bun"
[ -s "$BUN_INSTALL/_bun" ] && fpath=("$BUN_INSTALL" $fpath)

# ===== SSH Agent =====
if [[ "$OSTYPE" == "darwin"* ]]; then
  export APPLE_SSH_ADD_BEHAVIOR=macos
  # `ssh-add --apple-load-keychain` costs ~650 ms even when identities are
  # already loaded. Skip it once any identity is in the agent.
  ssh-add -l >/dev/null 2>&1 || ssh-add --apple-load-keychain >/dev/null 2>&1
elif [[ "$OSTYPE" == "linux"* ]]; then
  # Stabilize SSH_AUTH_SOCK behind a fixed symlink so tmux panes always
  # reach the forwarded agent after SSH reconnects (new socket path).
  if [[ -S "$SSH_AUTH_SOCK" && "$SSH_AUTH_SOCK" != "$HOME/.ssh/ssh_auth_sock" ]]; then
    ln -sf "$SSH_AUTH_SOCK" "$HOME/.ssh/ssh_auth_sock"
  fi
  export SSH_AUTH_SOCK="$HOME/.ssh/ssh_auth_sock"
fi

# ===== Aliases =====
alias l="ls"
alias ll="ls -al"
alias c="clear"
alias t="eza . --tree --level=1"
alias tt="eza . --tree --level=2"
alias ttt="eza . --tree --level=3"

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

# ===== Colored man pages (Dracula Pro) =====
export LESS_TERMCAP_mb=$'\e[1;38;2;149;128;255m'
export LESS_TERMCAP_md=$'\e[1;38;2;128;255;234m'
export LESS_TERMCAP_me=$'\e[0m'
export LESS_TERMCAP_se=$'\e[0m'
export LESS_TERMCAP_so=$'\e[1;38;2;248;248;242;48;2;69;65;88m'
export LESS_TERMCAP_ue=$'\e[0m'
export LESS_TERMCAP_us=$'\e[1;38;2;138;255;128m'

# ===== Linux/Arch plugins =====
if [[ "$OSTYPE" == "linux"* ]]; then
  [ -f /usr/share/zsh/plugins/zsh-autosuggestions/zsh-autosuggestions.zsh ] && \
    source /usr/share/zsh/plugins/zsh-autosuggestions/zsh-autosuggestions.zsh
  [ -f /usr/share/zsh/plugins/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh ] && \
    source /usr/share/zsh/plugins/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
  [ -f /usr/share/fzf/key-bindings.zsh ] && \
    source /usr/share/fzf/key-bindings.zsh
fi

# ===== Plugins (via homebrew) =====
# Source these lazily inside the deferred-compinit handler so they don't
# block the first prompt render (they cost ~90 ms together).
_zsh_brew_plugins_to_load=()
if [ -n "$HOMEBREW_PREFIX" ]; then
  [ -f "$HOMEBREW_PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh" ] && \
    _zsh_brew_plugins_to_load+=("$HOMEBREW_PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh")
  [ -f "$HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh" ] && \
    _zsh_brew_plugins_to_load+=("$HOMEBREW_PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh")
fi

# ===== Syntax highlighting (Dracula Pro) =====
# Declared up front so the deferred plugin source picks them up as soon as
# it loads. ZSH_HIGHLIGHT_STYLES must be an associative array for the
# subscript assignments to be valid.
typeset -gA ZSH_HIGHLIGHT_STYLES
ZSH_HIGHLIGHT_STYLES[command]='fg=#8AFF80'
ZSH_HIGHLIGHT_STYLES[builtin]='fg=#80FFEA'
ZSH_HIGHLIGHT_STYLES[alias]='fg=#8AFF80'
ZSH_HIGHLIGHT_STYLES[function]='fg=#8AFF80'
ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=#FF9580'
ZSH_HIGHLIGHT_STYLES[path]='fg=#F8F8F2,underline'
ZSH_HIGHLIGHT_STYLES[globbing]='fg=#9580FF'
ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=#FFFF80'
ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=#FFFF80'
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]='fg=#FFFF80'
ZSH_HIGHLIGHT_STYLES[back-quoted-argument]='fg=#FF80BF'
ZSH_HIGHLIGHT_STYLES[reserved-word]='fg=#FF80BF'
ZSH_HIGHLIGHT_STYLES[commandseparator]='fg=#FF80BF'
ZSH_HIGHLIGHT_STYLES[redirection]='fg=#9580FF'
ZSH_HIGHLIGHT_STYLES[comment]='fg=#7970A9'
ZSH_AUTOSUGGESTION_HIGHLIGHT_STYLE='fg=#504C67'

# ===== Prompt =====
# `eval "$(starship init zsh)"` is ~70 ms (mostly the eval). Cache the
# expanded output and source it; bytecode-compile it for good measure.
() {
  local sbin
  sbin=$(command -v starship 2>/dev/null) || return
  local cache="${XDG_CACHE_HOME:-$HOME/.cache}/zsh/starship-init.zsh"
  if [[ ! -s "$cache" || "$sbin" -nt "$cache" ]]; then
    mkdir -p "${cache%/*}"
    "$sbin" init zsh > "$cache"
  fi
  if [[ ! -s "${cache}.zwc" || "$cache" -nt "${cache}.zwc" ]]; then
    zcompile "$cache" 2>/dev/null
  fi
  source "$cache"
}

# ===== History =====
HISTFILE=~/.zsh_history
HISTSIZE=50000
SAVEHIST=50000
setopt SHARE_HISTORY HIST_IGNORE_DUPS HIST_IGNORE_SPACE HIST_VERIFY

# ===== Direnv =====
command -v direnv &>/dev/null && eval "$(direnv hook zsh)"

# ===== Zoxide =====
command -v zoxide &>/dev/null && eval "$(zoxide init zsh)"

# ===== lsd aliases (Arch) =====
if command -v lsd &>/dev/null; then
  alias ls='lsd'
  alias lt='lsd --tree'
fi

. "$HOME/.local/bin/env"

export QLTY_INSTALL="$HOME/.qlty"
