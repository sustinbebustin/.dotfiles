# Hyprland Keybindings

This documents all keybindings configured across Hyprland, xremap, and kanata.

`$mod` = `Super`

---

## Caps Lock (kanata)

Caps Lock is remapped at the kernel level via kanata:

- **Tap** -> Escape
- **Hold** -> Hyper (Ctrl+Shift+Alt+Super)

Config: `~/.config/kanata/kanata.kbd`

---

## Window Management

| Keybind              | Action               |
|----------------------|----------------------|
| Super + Return       | Open terminal (Ghostty) |
| Super + Q            | Close active window  |
| Super + Space        | App launcher (Walker) |
| Super + E            | File manager (Thunar) |
| Super + Shift + V    | Toggle floating      |
| Super + Shift + F    | Fullscreen           |
| Super + P            | Pseudo-tile          |
| Super + J            | Toggle split         |
| Super + Shift + M    | Exit Hyprland        |

## Focus

| Keybind              | Action               |
|----------------------|----------------------|
| Super + Arrow keys   | Move focus (l/r/u/d) |

## Move Windows

| Keybind                    | Action                    |
|----------------------------|---------------------------|
| Super + Shift + Arrow keys | Move window (l/r/u/d)     |

## Resize Windows

| Keybind                   | Action                        |
|---------------------------|-------------------------------|
| Super + Ctrl + Right      | Grow width (+20)              |
| Super + Ctrl + Left       | Shrink width (-20)            |
| Super + Ctrl + Up         | Shrink height (-20)           |
| Super + Ctrl + Down       | Grow height (+20)             |

## Mouse

| Keybind              | Action               |
|----------------------|----------------------|
| Super + Left click   | Move window          |
| Super + Right click  | Resize window        |

## Workspaces

| Keybind              | Action                      |
|----------------------|-----------------------------|
| Super + 1-9, 0       | Switch to workspace 1-10    |
| Super + Shift + 1-9, 0 | Move window to workspace 1-10 |

Workspaces 1-5 are persistent (always exist).

---

## Utilities

| Keybind              | Action                              |
|----------------------|-------------------------------------|
| Print                | Screenshot area (grimblast, copy+save) |
| Shift + Print        | Screenshot full output (grimblast, copy+save) |
| Super + Ctrl + Q     | Lock screen (hyprlock)              |
| Super + Shift + N    | Toggle notification center (swaync) |
| Super + Shift + R    | Toggle screen recording (wl-screenrec) |
| Super + Shift + S    | Blue light filter on (4500K)        |
| Super + Shift + Alt + S | Blue light filter off            |

## Media Keys

| Keybind              | Action               |
|----------------------|----------------------|
| XF86AudioMute        | Toggle mute          |
| XF86AudioRaiseVolume | Volume +5%           |
| XF86AudioLowerVolume | Volume -5%           |
| XF86AudioPlay        | Play/pause           |
| XF86AudioNext        | Next track           |
| XF86AudioPrev        | Previous track       |

---

## macOS-style Shortcuts (xremap)

xremap intercepts at the evdev level before Hyprland, remapping specific
Super+key combos to their Ctrl equivalents. Unremapped Super combos pass
through to Hyprland normally.

Config: `~/.config/xremap/config.yml`

### Terminal Apps

Terminals use Ctrl+Shift variants to avoid conflicts with SIGINT (Ctrl+C).

Applies to: Ghostty, Kitty, Alacritty, foot, WezTerm

| Keybind              | Maps to              | Action               |
|----------------------|----------------------|----------------------|
| Super + C            | Ctrl+Shift+C         | Copy                 |
| Super + V            | Ctrl+Shift+V         | Paste                |
| Super + X            | Ctrl+Shift+X         | Cut                  |
| Super + A            | Ctrl+Shift+A         | Select all           |
| Super + F            | Ctrl+Shift+F         | Find                 |
| Super + Z            | Ctrl+Shift+Z         | Undo                 |
| Super + T            | Ctrl+Shift+T         | New tab              |
| Super + W            | Ctrl+Shift+W         | Close tab            |
| Super + Backspace    | Ctrl+U               | Clear line           |
| Super + Left         | Home                 | Beginning of line    |
| Super + Right        | End                  | End of line          |
| Alt + Left           | Ctrl+Left            | Word back            |
| Alt + Right          | Ctrl+Right           | Word forward         |
| Alt + Backspace      | Ctrl+W               | Delete word back     |

### GUI Apps

Everything except terminals. Uses standard Ctrl variants.

| Keybind              | Maps to              | Action               |
|----------------------|----------------------|----------------------|
| Super + C            | Ctrl+C               | Copy                 |
| Super + V            | Ctrl+V               | Paste                |
| Super + X            | Ctrl+X               | Cut                  |
| Super + Z            | Ctrl+Z               | Undo                 |
| Super + Shift + Z    | Ctrl+Shift+Z         | Redo                 |
| Super + A            | Ctrl+A               | Select all           |
| Super + S            | Ctrl+S               | Save                 |
| Super + F            | Ctrl+F               | Find                 |
| Super + T            | Ctrl+T               | New tab              |
| Super + W            | Ctrl+W               | Close tab            |
| Super + Shift + T    | Ctrl+Shift+T         | Reopen tab           |
| Super + L            | Ctrl+L               | Focus address bar    |
| Super + R            | Ctrl+R               | Reload               |
| Super + N            | Ctrl+N               | New window           |
| Super + [            | Alt+Left             | Back                 |
| Super + ]            | Alt+Right            | Forward              |
| Super + Left         | Home                 | Beginning of line    |
| Super + Right        | End                  | End of line          |
| Super + Up           | Ctrl+Home            | Top of document      |
| Super + Down         | Ctrl+End             | Bottom of document   |
| Alt + Left           | Ctrl+Left            | Word back            |
| Alt + Right          | Ctrl+Right           | Word forward         |
| Alt + Backspace      | Ctrl+Backspace       | Delete word back     |
| Super + Backspace    | Ctrl+Shift+Backspace | Delete line back     |
