# Tmux Keybindings

Prefix: `Ctrl+B`

## Splits and Windows

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix \`       | Split horizontally (side-by-side) |
| `prefix Enter`   | Split vertically (top/bottom)   |
| `prefix c`       | New window                      |

All splits and new windows preserve the current pane's working directory.

## Pane Management

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix x`       | Kill pane                       |
| `prefix m`       | Zoom/maximize pane              |

## Pane Resizing

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix -`       | Resize down 2 rows              |
| `prefix =`       | Resize up 2 rows                |
| `prefix ]`       | Resize right 2 columns          |
| `prefix [`       | Resize left 2 columns           |
| `prefix Delete`  | Tiled layout                    |

All resize bindings are repeatable (`-r`), so you can press the key multiple
times after a single prefix.

## Copy Mode (vim-style)

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix v`       | Enter copy mode                 |
| `v`              | Begin selection                 |
| `V`              | Select line                     |
| `Ctrl+v`         | Toggle rectangle selection      |
| `y`              | Copy selection to clipboard     |
| `Escape`         | Clear selection                 |
| `q`              | Exit copy mode                  |
| Mouse drag       | Copy selection to clipboard     |

Standard vim motions (`h/j/k/l`, `w/b`, `Ctrl+u/d`, `/` search) work in
copy mode.

## Config

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix r`       | Reload tmux config              |

## Useful Defaults (not remapped)

| Keybinding       | Action                          |
|------------------|---------------------------------|
| `prefix d`       | Detach session                  |
| `prefix s`       | List sessions                   |
| `prefix w`       | List windows (all sessions)     |
| `prefix ,`       | Rename current window           |
| `prefix $`       | Rename current session          |
| `prefix n`       | Next window                     |
| `prefix p`       | Previous window                 |
| `prefix 0-9`     | Switch to window by number      |
| `prefix arrow`   | Navigate panes                  |
| `prefix q`       | Show pane numbers (then press number to jump) |
| `prefix !`       | Break pane into new window      |

## Plugins

- **tmux-resurrect** -- save/restore sessions across restarts
- **tmux-continuum** -- auto-save sessions every 10 minutes, auto-restore on start
