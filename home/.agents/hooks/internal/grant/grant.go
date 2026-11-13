// Package grant holds the merge grant: the record that the user typed
// `/commit-push-pr` with a merge flag, which lets that session's `gh pr merge`
// run without the prompt block-dangerous-git otherwise raises.
//
// A grant is minted from a UserPromptExpansion payload (see Apply). That event
// fires only when a typed slash command is expanded, so the model invoking the
// skill through the Skill tool mints nothing, and neither does text that
// merely reads like the command: a subagent report, a task notification, a
// message from another session.
//
// A grant belongs to one session and lasts TTL. It is not single-use, because
// one invocation may merge several targets and retry a merge under another
// method. It is not tied to the prompt that minted it either: the CI watch
// wakes the session through a task notification, which the harness counts as a
// new prompt. Typing the skill again without a merge flag revokes it.
//
// The store is a directory of empty files named after the session, aged by
// their modification time. Anything running as the user can write there, so
// the grant is only as strong as the protect-merge-grant rule, which keeps the
// agent's own tool calls away from the directory and from the minting command.
package grant

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DirName is the store's directory name. It is distinctive on purpose:
// protect-merge-grant recognises the store by this name wherever it appears in
// a path.
const DirName = "agent-guard-grants"

// TTL is how long a grant lasts. It has to outlive a CI run, since the merge
// follows the watch; past it the merge asks again, which is the safe way to be
// wrong.
const TTL = time.Hour

// Skill is the slash command whose merge flags mint a grant.
const Skill = "commit-push-pr"

// Event is the hook event Apply acts on.
const Event = "UserPromptExpansion"

// Subcommand is the agent-guard argument that runs Apply: the command the
// Event is registered with is `agent-guard-bin grant`.
const Subcommand = "grant"

// Dir returns the store's location. It is fixed under the home directory
// rather than read from the environment, so the event that mints a grant and
// the one that checks it cannot disagree about where it is.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating the home directory to find the merge grants: %w", err)
	}
	return filepath.Join(home, ".local", "state", DirName), nil
}

// sessionID is the shape of a session identifier that may name a file. The
// harness sends a UUID; anything else is refused rather than joined into a
// path.
var sessionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// file returns the grant file for session, or false when session cannot name
// one.
func file(dir, session string) (string, bool) {
	if !sessionID.MatchString(session) {
		return "", false
	}
	return filepath.Join(dir, session), true
}

// Mint records a grant for session, starting at now, and removes the expired
// grants other sessions left behind.
func Mint(dir, session string, now time.Time) error {
	path, ok := file(dir, session)
	if !ok {
		return fmt.Errorf("session id %q cannot name a grant file (want letters, digits, - and _)", session)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating the merge grant directory: %w", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return fmt.Errorf("writing the merge grant: %w", err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		return fmt.Errorf("dating the merge grant at %s: %w", path, err)
	}
	return sweep(dir, now)
}

// sweep removes every expired grant in dir.
func sweep(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("listing the merge grants to clear expired ones: %w", err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !expired(info.ModTime(), now) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing the expired merge grant at %s: %w", path, err)
		}
	}
	return nil
}

// Revoke removes session's grant. Having none is not an error.
func Revoke(dir, session string) error {
	path, ok := file(dir, session)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing the merge grant at %s: %w", path, err)
	}
	return nil
}

// Live reports whether session holds a grant that has not expired at now. A
// session with no usable id holds none: Droid, or a payload without one.
func Live(dir, session string, now time.Time) (bool, error) {
	path, ok := file(dir, session)
	if !ok {
		return false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading the merge grant at %s: %w", path, err)
	}
	return info.Mode().IsRegular() && !expired(info.ModTime(), now), nil
}

// expired reports whether a grant minted at minted is over at now. One dated
// in the future is over too: nothing this package writes produces it.
func expired(minted, now time.Time) bool {
	age := now.Sub(minted)
	return age < 0 || age >= TTL
}

// expansion is the part of a UserPromptExpansion payload Apply reads.
type expansion struct {
	Event     string `json:"hook_event_name"`
	SessionID string `json:"session_id"`
	Command   string `json:"command_name"`
	Args      string `json:"command_args"`
}

// Apply reads one hook payload and updates the store: an expansion of Skill
// with a merge flag mints the session's grant, and one without revokes it.
// Any other payload changes nothing, so registering the command under the
// wrong event, or without a matcher, cannot mint a grant.
func Apply(r io.Reader, dir string, now time.Time) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading the hook payload from stdin: %w", err)
	}
	var in expansion
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("decoding the hook payload as JSON: %w", err)
	}
	if in.Event != Event || in.Command != Skill {
		return nil
	}
	if WantsMerge(in.Args) {
		return Mint(dir, in.SessionID, now)
	}
	return Revoke(dir, in.SessionID)
}

// WantsMerge reports whether the skill's arguments put it in merge mode. It
// has to agree with the skill's own gather-state.sh, which a test checks by
// running that script: flags count on the first line, before the last ` -- `
// note separator, and a no-push flag beside a merge flag is a conflict the
// skill stops on, so it merges nothing.
func WantsMerge(args string) bool {
	args = strings.TrimRight(args, "\n")
	switch {
	case args == "--" || strings.HasPrefix(args, "-- "):
		return false
	default:
		if i := strings.LastIndex(args, " -- "); i >= 0 {
			args = args[:i]
		}
	}
	line, _, _ := strings.Cut(args, "\n")

	merge, noPush := false, false
	for tok := range strings.FieldsSeq(line) {
		switch tok {
		case "--merge", "--bypass", "--merge-bypass":
			merge = true
		case "--no-push", "--np":
			noPush = true
		}
	}
	return merge && !noPush
}
