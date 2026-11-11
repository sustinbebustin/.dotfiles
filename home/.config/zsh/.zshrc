# .zshrc - Configuration for interactive shells

# ===== Key bindings =====
# Pin emacs keys: zsh otherwise switches to vi keys when EDITOR contains "vi".
bindkey -e
bindkey '^[^?' backward-kill-word  # Option+Backspace: delete word
bindkey '^[[1;5D' backward-word    # Ctrl+Left
bindkey '^[[1;5C' forward-word     # Ctrl+Right: also accepts one word of a suggestion
bindkey '^[[3~' delete-char        # Delete
# Home/End, in the three encodings terminals and multiplexers send.
bindkey '^[[H' beginning-of-line
bindkey '^[OH' beginning-of-line
bindkey '^[[1~' beginning-of-line
bindkey '^[[F' end-of-line
bindkey '^[OF' end-of-line
bindkey '^[[4~' end-of-line
# Up/Down walk only the history entries that start with what is already typed.
autoload -Uz up-line-or-beginning-search down-line-or-beginning-search
zle -N up-line-or-beginning-search
zle -N down-line-or-beginning-search
bindkey '^[[A' up-line-or-beginning-search
bindkey '^[OA' up-line-or-beginning-search
bindkey '^[[B' down-line-or-beginning-search
bindkey '^[OB' down-line-or-beginning-search

# ===== Completions =====
fpath=("$ZDOTDIR/functions" $fpath)
typeset -U fpath path  # dedupe fpath/path so compinit scans each dir once
# Completion menu: Tab opens a list navigable with the arrow keys; matching
# falls back from exact to case-insensitive to substring.
zmodload zsh/complist  # must load before compinit for `menu select`
zstyle ':completion:*' menu select
zstyle ':completion:*' matcher-list '' 'm:{a-zA-Z}={A-Za-z}' 'r:|[._-]=* r:|=*' 'l:|=* r:|=*'
zstyle ':completion:*' list-colors "${(s.:.)LS_COLORS}"
zstyle ':completion:*' group-name ''
zstyle ':completion:*:descriptions' format '%F{#7970A9}-- %d --%f'
zstyle ':completion:*:warnings' format '%F{#FF9580}no matches%f'
zstyle ':completion:*' squeeze-slashes true
zstyle ':completion:*' use-cache on
zstyle ':completion:*' cache-path "${XDG_CACHE_HOME:-$HOME/.cache}/zsh/zcompcache"
bindkey '^[[Z' reverse-menu-complete                # Shift+Tab
bindkey -M menuselect '^[[Z' reverse-menu-complete
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
  # zsh-autosuggestions wraps the editing widgets from a precmd hook, and the
  # first prompt's precmd ran before it was sourced. Wrap them now, or a new
  # shell shows no suggestions until its second prompt.
  (( $+functions[_zsh_autosuggest_start] )) && _zsh_autosuggest_start
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
# PATH, PNPM_HOME and BUN_INSTALL are all set in
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
  # Every shell uses one fixed link that ssh-agent-sock keeps on a live
  # agent. ssh refreshes it too (Match exec in ~/.ssh/config); the prompt
  # hook covers other agent clients such as ssh-add.
  export SSH_AUTH_SOCK="$HOME/.ssh/ssh_auth_sock"
  _ssh_agent_sock() { "$HOME/.local/bin/ssh-agent-sock" }
  _ssh_agent_sock
  add-zsh-hook precmd _ssh_agent_sock
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
# A leading effort token is consumed as `--effort`: `l`/`m`/`h`/`x` or the full
# word (`low` `medium` `high` `xhigh` `max`). Omit it and no flag is passed, so
# the effort from settings.json applies. Remaining args go straight to `claude`
# (e.g. `claude h --resume`).
_claude_run() {
  local model="$1" extra_flags="$2"
  shift 2
  local -a effort_flag model_flag
  case "$1" in
    l|low)    effort_flag=(--effort low);    shift ;;
    m|medium) effort_flag=(--effort medium); shift ;;
    h|high)   effort_flag=(--effort high);   shift ;;
    x|xhigh)  effort_flag=(--effort xhigh);  shift ;;
    max)      effort_flag=(--effort max);    shift ;;
  esac
  [[ -n "$model" ]] && model_flag=(--model "$model")
  command claude "${model_flag[@]}" "${effort_flag[@]}" ${=extra_flags} "$@"
}

claude()    { _claude_run ""      ""                      "$@" }
fable()     { _claude_run fable   ""                      "$@" }
# Plan in Fable 5, then approve the plan to drop back into your default model.
fableplan() { _claude_run fable   "--permission-mode plan" "$@" }

# Extra Claude accounts, registered per machine with `dot claude add <name>`.
# Each gets a function <name> whose CLAUDE_CONFIG_DIR, ~/.claude-<name>, holds
# its own credentials, so every login stays live at once; `dot` links the
# tracked config and shared session history into it.
#
# <name> takes an optional wrapper name, then the same effort token and flags
# the default wrappers take: `work`, `work h`, `work fable h`,
# `work fableplan x --resume`. zsh scopes the export to this call and its
# dynamic extent, so the wrapper it delegates to still sees it.
_claude_account_run() {
  local -x CLAUDE_CONFIG_DIR="$HOME/.claude-$1"
  shift
  case "$1" in
    claude|fable|fableplan)
      local wrapper="$1"
      shift
      "$wrapper" "$@"
      ;;
    *) claude "$@" ;;
  esac
}

() {
  local file="${XDG_CONFIG_HOME:-$HOME/.config}/dot/claude-accounts" name
  [[ -r "$file" ]] || return 0
  for name in ${(f)"$(<$file)"}; do
    # Same rule `dot` enforces; the name is spliced into a function body.
    if [[ ! "$name" =~ '^[a-z][a-z0-9-]*$' ]]; then
      print -u2 "zshrc: skipping invalid Claude account name '$name' in $file"
      continue
    fi
    if (( $+commands[$name] || $+functions[$name] || $+aliases[$name] || $+builtins[$name] )); then
      print -u2 "zshrc: Claude account '$name' would shadow an existing command; rename it with dot claude remove/add"
      continue
    fi
    functions[$name]="_claude_account_run $name \"\$@\""
  done
}

# Factory Droid
# Built-in skills can only be turned off per launch -- settings.json has no key
# for it. The flag is global, so it also rides along on `droid exec`, `resume`,
# and the management subcommands (mcp, plugin, ...), which all accept it.
droid() { command droid --disable-builtin-skills "$@" }

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
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE='fg=#7970A9'

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
setopt SHARE_HISTORY HIST_IGNORE_DUPS HIST_IGNORE_SPACE HIST_VERIFY EXTENDED_HISTORY

# ===== fzf =====
# Ctrl-T inserts a file path, Alt-C changes directory, `**<Tab>` completes
# fuzzily. Ctrl-R is rebound by atuin below.
if command -v fzf &>/dev/null; then
  if command -v fd &>/dev/null; then
    export FZF_DEFAULT_COMMAND='fd --type f --hidden --exclude .git'
    export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
    export FZF_ALT_C_COMMAND='fd --type d --hidden --exclude .git'
  fi
  # Dracula Pro
  export FZF_DEFAULT_OPTS='--height=40% --layout=reverse --border --color=fg:#F8F8F2,hl:#9580FF,fg+:#F8F8F2,bg+:#454158,hl+:#9580FF,info:#7970A9,prompt:#8AFF80,pointer:#FF80BF,marker:#FF80BF,spinner:#FFCA80,header:#7970A9,border:#454158'
  # Off a terminal (`zsh -ic` from a script or agent) there is no line editor
  # to bind keys in, and the script warns "can't change option: zle".
  [[ -t 0 ]] && eval "$(fzf --zsh)"
fi

# ===== Atuin =====
# Takes Ctrl-R from fzf (bound above, so this wins). The up arrow stays zsh's
# own prefix search rather than opening atuin.
if command -v atuin &>/dev/null; then
  # --disable-ai: `?` on an empty prompt would otherwise open atuin's AI mode.
  eval "$(atuin init zsh --disable-up-arrow --disable-ai)"
  # atuin init makes its own strategy -- the most recent match from anywhere,
  # failed commands included -- the only source of autosuggestions. Prefer a
  # command that succeeded in this directory, then atuin's match, then zsh
  # history in case atuin fails.
  _zsh_autosuggest_strategy_atuin_cwd() {
    # '$all-user' is a literal atuin author filter, not a shell variable.
    suggestion=$(ATUIN_QUERY="$1" atuin search --cmd-only --author '$all-user' \
      --cwd "$PWD" --exit 0 --limit 1 --search-mode prefix 2>/dev/null)
  }
  ZSH_AUTOSUGGEST_STRATEGY=(atuin_cwd atuin history)
fi

# ===== Direnv =====
# The hook bakes in direnv's resolved Cellar path, which `brew upgrade` deletes
# out from under running shells; call it through PATH instead.
command -v direnv &>/dev/null && eval "$(direnv hook zsh | sed "s|\$'[^']*/bin/direnv'|direnv|")"

# ===== Zoxide =====
command -v zoxide &>/dev/null && eval "$(zoxide init zsh)"

# Written by third-party installers (uv, rust); absent on a fresh machine.
[ -f "$HOME/.local/bin/env" ] && . "$HOME/.local/bin/env"

export QLTY_INSTALL="$HOME/.qlty"

# >>> mws shell-init >>>
eval "$(mws shell-init zsh)"
# <<< mws shell-init <<<
