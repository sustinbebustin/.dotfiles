# Claude Code Sound Notifications

Plays an alert on the Mac when a Claude Code permission prompt is left unanswered,
even though Claude Code runs on the remote dev VM.

## Why it works this way

Claude Code runs on the VM, which has no speakers. Nothing the hook does locally
can be heard. Rather than have the VM reach back to the Mac (which needs Remote
Login, a routable laptop, and VM-held credentials to the Mac), the alert rides the
ssh connection that already exists.

The Mac runs `soundnotify serve`, which listens on a unix socket and owns the
audio device. `ssh` remote-forwards that socket onto the *same path* on the VM, so
the hook client connects to `~/.claude/run/sound.sock` on either machine and does
not need to know which side it is on. No inbound firewall rules, no sshd on the
Mac, no credentials on the VM, and it works from any network the laptop is on.

With no ssh connection up, the socket is absent, the client is a silent no-op, and
the hook still exits 0.

## Delayed alerts

The hook does not play a sound when the permission prompt appears. It *arms* a
15-second timer keyed by Claude Code session id:

| Event | Action |
|-------|--------|
| `Notification` (`permission_prompt`) | arm `needs_input` for 15s |
| `PostToolUse` | disarm (the tool ran, so the prompt was approved) |
| `UserPromptSubmit` | disarm (you typed, so you are present) |
| `Stop` | disarm (turn ended, nothing is waiting) |

So the sound only reaches you when a prompt has actually sat unanswered, not the
instant it appears.

Known gap: approving a *single* tool that then runs longer than 15 seconds still
alerts, because nothing fires between approval and `PostToolUse`. Raise the delay
in `settings.json` if that happens often.

## Setup

### 1. Both machines: build and link

```bash
make -C ~/.dotfiles/home-common/.claude/hooks
dot stow
```

The binary is gitignored (`*-bin`) and built per-machine. The VM needs the client;
the Mac needs the server and the mp3 files under
`~/.claude/hooks/utils/audio/`.

### 2. VM: allow ssh to rebind the forwarded socket

Without this, only the first ssh connection can bind the socket and a reconnect
fails against the leftover file. Add to `/etc/ssh/sshd_config`:

```
StreamLocalBindUnlink yes
```

```bash
sudo systemctl reload ssh
```

Whichever connection attaches most recently then owns the socket, so Warp, Cursor
remote-ssh, and a plain terminal can come and go without breaking the alert.

### 3. VM: create the socket directory

`sshd` creates the socket but not its parent directory.

```bash
mkdir -p ~/.claude/run
```

### 4. Mac: forward the socket

In `~/.ssh/config` on the Mac, under the host entry for the VM:

```
Host dev
    RemoteForward /home/austin/.claude/run/sound.sock /Users/austin/.claude/run/sound.sock
```

Both paths are absolute and neither expands `~`. The first is the path on the VM,
the second the path on the Mac.

### 5. Mac: start the listener

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/local.claude-soundnotify.plist
```

It restarts at login and on crash. Logs go to `~/.claude/run/soundnotify.log`.

To stop it:

```bash
launchctl bootout gui/$(id -u)/local.claude-soundnotify
```

## Verifying

On the Mac, confirm the server is up:

```bash
~/.claude/hooks/soundnotify-bin play needs_input   # should be audible
```

Then, from an ssh session on the VM:

```bash
~/.claude/hooks/soundnotify-bin play needs_input
```

Silence plus `no sound server at ...` means the forward is not up: check that the
`RemoteForward` line applies to the host you connected to, and reconnect.

This message in the ssh session comes from the ssh client on the Mac, not the VM:

```
connect to /Users/austin/.claude/run/sound.sock port -2 failed: No such file or directory
```

It means the opposite: the forward *is* working and the Mac has no listener at the
other end. Check that the launchd agent is loaded (`launchctl print
gui/$(id -u)/local.claude-soundnotify`) and see `~/.claude/run/soundnotify.log`.
`port -2` is just how ssh renders a unix-socket destination.

`remote port forwarding failed for listen path ...` on connect means sshd could not
bind the socket, which is what `StreamLocalBindUnlink` above prevents. Note that
`sshd` runs as a persistent daemon here (`sshd -D`), so its children inherit the
config parsed at daemon start: `sshd -T` will report a new setting off disk while
live connections still use the old one until `systemctl reload ssh`.

## CLI

```
soundnotify serve                 listen on the sound socket and play alerts
soundnotify arm <sound> <delay>   play <sound> after <delay> unless disarmed
soundnotify disarm                cancel this session's pending alert
soundnotify play <sound>          play <sound> now
```

Client commands read the hook payload on stdin for the session id and always exit
0, so a broken sound setup can never block a tool call. Errors go to stderr.

A sound name resolves to `~/.claude/hooks/utils/audio/<name>.mp3` and must be
lowercase letters, digits, and underscores, so a request cannot escape that
directory. `CLAUDE_SOUND_SOCKET` overrides the socket path on both sides.

## Adding more alerts

Drop an mp3 into `home-common/.claude/hooks/utils/audio/` and reference it by
basename. `work_complete.mp3` is already there but deliberately unwired: on `Stop`
it would fire every turn. To use it, add a `Stop` hook running
`soundnotify-bin play work_complete` alongside the existing disarm.

To also alert when Claude is idle waiting for a prompt, add `idle_prompt` to the
`Notification` matcher in `settings.json`:

```json
"matcher": "permission_prompt|idle_prompt"
```
