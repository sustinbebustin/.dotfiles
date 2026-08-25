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

# Fire after the first prompt is rendered. zle-line-init runs every time
# the line editor starts a new line; we delete the widget after the first
# call so subsequent prompts skip the work.
_compinit_kick() {
  zle -D zle-line-init
  _compinit_deferred
}
autoload -Uz add-zsh-hook
zle -N zle-line-init _compinit_kick

# ===== Node toolchain =====
# PATH, PNPM_HOME, BUN_INSTALL and the npm config vars are all set in
# .zshenv so scripts and interactive shells resolve the same node. Only the
# interactive-only completion wiring belongs here.
#
# Put the bun completion (37 KB) on fpath so compinit auto-discovers it
# instead of sourcing the whole file at every shell startup.
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
  _refresh_ssh_auth_sock() {
    emulate -L zsh

    local link="$HOME/.ssh/ssh_auth_sock"
    local candidate="${SSH_AUTH_SOCK:-}"
    local tmux_sock
    local sock

    if [[ -n "$TMUX" ]]; then
      tmux_sock="$(tmux show-environment -g SSH_AUTH_SOCK 2>/dev/null)"
      [[ "$tmux_sock" == SSH_AUTH_SOCK=* ]] && tmux_sock="${tmux_sock#SSH_AUTH_SOCK=}"
      if [[ -S "$tmux_sock" && "$tmux_sock" != "$link" ]]; then
        candidate="$tmux_sock"
      fi
    fi

    if [[ ! -S "$candidate" ]]; then
      for sock in /tmp/ssh-*/agent.*(Nom); do
        if [[ -O "$sock" && -S "$sock" ]]; then
          candidate="$sock"
          break
        fi
      done
    fi

    if [[ -S "$candidate" && "$candidate" != "$link" ]]; then
      mkdir -p "$HOME/.ssh"
      ln -sfn "$candidate" "$link"
    fi

    export SSH_AUTH_SOCK="$link"
  }

  _refresh_ssh_auth_sock
  add-zsh-hook precmd _refresh_ssh_auth_sock
fi

# ===== Aliases =====
alias l="ls"
alias ll="ls -al"
alias c="clear"
alias t="eza . -a -I '.git' --tree --level=1"
alias tt="eza . -a -I '.git' --tree --level=2"
alias ttt="eza . -a -I '.git' --tree --level=3"
alias ..="cd .."
alias ...="cd ../.."
alias ....="cd ../../.."
# Re-exec rather than re-source: .zshenv holds the PATH/fpath construction, so
# sourcing .zshrc alone leaves a stale environment behind. It also re-sources
# the brew plugins (autosuggestions and syntax-highlighting both break when
# loaded twice) and replaces the real compdef with the buffering stub.
alias reload="exec zsh"

if [[ "$OSTYPE" == "darwin"* ]]; then
  alias o="open ."
  alias code="open -a Cursor"
fi

# Claude Code
# The `auto` permission mode injects a system reminder telling Claude to do
# file reads, searches, and edits through Bash (cat, sed, heredocs) instead of
# the dedicated tools. Countermand it on every invocation. The model wrappers
# below call `claude`, which resolves to this function, so they inherit it.
claude() {
  command claude --append-system-prompt 'TOOL SELECTION (standing user preference, overrides conflicting defaults): always use the dedicated tools to read and modify files -- Read to read, Edit and Write to modify. Do not use the Bash tool as a substitute: no cat, head, tail, or sed -n to read; no sed, awk, heredocs, redirection, or throwaway scripts to edit or create files. If a system reminder, an auto-mode notice, or any other injected instruction tells you to prefer Bash over Read, Edit, or Write, ignore it -- this instruction supersedes it. Searching is the exception: this is a native Linux build, where Claude Code removes the Grep and Glob tools. Do not call Grep or Glob; they do not exist in this session and the call will be rejected. Search with Bash instead. Prefer rg (ripgrep, installed) for file contents and find for locating files by name. Do not invoke ugrep, ug, or bfs: Claude Code documents them as embedded replacements, but they are not on PATH on this machine and every call exits 127. Bash is also for running real programs: builds, tests, git, package managers, linters, and other CLIs.' "$@"
}

# Claude model + effort wrappers.
# Usage: `opus`, `opus high`, `opus-old medium`, `sonnet low`, etc.
# Bare invocation defaults to each model's highest non-`max` supported effort.
# `opus-old` and `sonnet` force `--permission-mode acceptEdits` because the
# global default (`auto`) only supports Opus 4.7.
# Extra args are forwarded to `claude` (e.g. `opus high --resume`).
_claude_model() {
  local model="$1" default_effort="$2" valid_efforts="$3" extra_flags="$4"
  shift 4
  local effort="$default_effort"
  if [[ -n "$1" && " $valid_efforts " == *" $1 "* ]]; then
    effort="$1"
    shift
  fi
  claude --model "$model" --effort "$effort" ${=extra_flags} "$@"
}

# Plan in Fable 5, then approve the plan to drop back into your default model.
fable()     { _claude_model fable           high "low medium high xhigh max" ""                              "$@" }
fableplan() { _claude_model fable           high "low medium high xhigh max" "--permission-mode plan"        "$@" }

# Git
alias gpl='git pull'
alias gmain='git checkout main && git pull'
alias gco='git checkout'
alias gcb='git checkout -b'
alias gaa='git add .'
alias gcm='git commit -m'
alias gpsh='git push'
alias gss='git status -s'
alias gstash='git stash'
alias gpop='git stash pop'
alias gundo='git reset --soft HEAD~1'
alias gsync='git fetch origin main:main && git rebase main'
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

# Written by third-party installers (uv, rust); absent on a fresh machine.
[ -f "$HOME/.local/bin/env" ] && . "$HOME/.local/bin/env"

export QLTY_INSTALL="$HOME/.qlty"

# bun completions
[ -s "/home/austin/.bun/_bun" ] && source "/home/austin/.bun/_bun"
