// Package grant holds session grants: the record that the user approved a
// skill's outward-facing steps up front, which lets that session run them
// without the prompts block-dangerous-git otherwise raises.
//
// There are two ways to get one, and both start from the user typing a slash
// command. Apply reads the hook payloads that carry them:
//
//   - /commit-push-pr typed with a merge flag grants PRMerge at once. The flag
//     is the approval.
//   - A skill named in the machine-local config (see Policy) grants its
//     operations in two steps. Typing it, in one of its roots, leaves a pending
//     grant that waives nothing. The grant goes live when the user then answers
//     a question dialog with the policy's confirming answer: for a deploy, the
//     "proceed" on the plan. Nothing is waived before the plan is approved.
//
// Both signals are the user's own. A UserPromptExpansion fires only when a
// typed slash command is expanded, so the model invoking the skill through the
// Skill tool mints nothing, and neither does text that merely reads like the
// command: a subagent report, a task notification, a message from another
// session. A question dialog is always shown, and its PostToolUse payload
// carries the option the user picked, whatever answer the model's call
// supplied.
//
// A grant belongs to one session and lasts TTL. It is not single-use, because
// one invocation may merge several targets or dispatch several workflows. It is
// not tied to the prompt that minted it either: a CI watch wakes the session
// through a task notification, which the harness counts as a new prompt.
// Typing /commit-push-pr without a merge flag revokes it, and typing a
// configured skill again starts over from pending.
//
// The store is a directory of small files named after the session, aged by
// their modification time. Anything running as the user can write there, so a
// grant is only as strong as the protect-grants rule, which keeps the
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
	"slices"
	"strings"
	"time"
)

// DirName is the store's directory name. It is distinctive on purpose:
// protect-grants recognises the store by this name wherever it appears in
// a path.
const DirName = "agent-guard-grants"

// TTL is how long a grant lasts, pending or live. It has to outlive a CI run,
// since a merge follows the watch; past it the commands ask again, which is the
// safe way to be wrong.
const TTL = time.Hour

// MergeSkill is the slash command whose merge flags grant PRMerge.
const MergeSkill = "commit-push-pr"

// The hook events Apply acts on, and the tool whose result confirms a pending
// grant.
const (
	ExpansionEvent = "UserPromptExpansion"
	AnswerEvent    = "PostToolUse"
	AnswerTool     = "AskUserQuestion"
)

// Subcommand is the agent-guard argument that runs Apply: the events are
// registered with the command `agent-guard-bin grant`.
const Subcommand = "grant"

// Op is one prompt a grant waives.
type Op string

// The operations a grant can waive. Each is the plain form of its command: a
// forced push, or a command the guard cannot read, asks whatever is granted.
const (
	// PRMerge is `gh pr merge`.
	PRMerge Op = "prMerge"
	// WorkflowRun is `gh workflow run`.
	WorkflowRun Op = "workflowRun"
	// ReleaseCreate is `gh release create`.
	ReleaseCreate Op = "releaseCreate"
	// PushMain is a `git push` that names main or master.
	PushMain Op = "pushMain"
)

// allOps are the operations config.json may name, in the order the error for
// an unknown one lists them.
var allOps = []Op{PRMerge, WorkflowRun, ReleaseCreate, PushMain}

// ParseOp returns the operation config.json spells as s.
func ParseOp(s string) (Op, bool) {
	if op := Op(s); slices.Contains(allOps, op) {
		return op, true
	}
	return "", false
}

// OpNames lists the spellings ParseOp accepts.
func OpNames() []string {
	names := make([]string, len(allOps))
	for i, op := range allOps {
		names[i] = string(op)
	}
	return names
}

// Ops is the set of operations a session has been granted. The zero value
// grants nothing.
type Ops []Op

// Has reports whether op is granted.
func (o Ops) Has(op Op) bool { return slices.Contains(o, op) }

// Policy is what typing one configured skill can grant.
type Policy struct {
	// Roots are absolute, cleaned directories. The skill counts only when
	// typed in a session working at or under one of them, so a skill of the
	// same name in another project grants nothing.
	Roots []string
	// Ops are the operations granted once the user confirms.
	Ops Ops
	// Confirm is how the confirming answer begins, compared without regard to
	// case: "Proceed" matches the options "Proceed" and "Proceed with the
	// prebuild flipped".
	Confirm string
}

// Policies maps a skill's slash-command name to its Policy.
type Policies map[string]Policy

// Dir returns the store's location. It is fixed under the home directory
// rather than read from the environment, so the event that mints a grant and
// the one that checks it cannot disagree about where it is.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating the home directory to find the session grants: %w", err)
	}
	return filepath.Join(home, ".local", "state", DirName), nil
}

// record is one session's grant as stored. It is pending while Confirm is set:
// the operations are waived only once an answer beginning with it arrives.
type record struct {
	Ops     Ops    `json:"ops"`
	Confirm string `json:"confirm,omitempty"`
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

// write stores rec as session's grant, dated now, and removes the expired
// grants other sessions left behind.
func write(dir, session string, rec record, now time.Time) error {
	path, ok := file(dir, session)
	if !ok {
		return fmt.Errorf("session id %q cannot name a grant file (want letters, digits, - and _)", session)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encoding the session grant: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating the session grant directory: %w", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("writing the session grant: %w", err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		return fmt.Errorf("dating the session grant at %s: %w", path, err)
	}
	return sweep(dir, now)
}

// sweep removes every expired grant in dir.
func sweep(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("listing the session grants to clear expired ones: %w", err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !expired(info.ModTime(), now) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing the expired session grant at %s: %w", path, err)
		}
	}
	return nil
}

// revoke removes session's grant. Having none is not an error.
func revoke(dir, session string) error {
	path, ok := file(dir, session)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing the session grant at %s: %w", path, err)
	}
	return nil
}

// read returns session's unexpired grant, pending or live. A session with no
// usable id holds none: Droid, or a payload without one.
func read(dir, session string, now time.Time) (rec record, ok bool, err error) {
	path, valid := file(dir, session)
	if !valid {
		return record{}, false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return record{}, false, nil
	}
	if err != nil {
		return record{}, false, fmt.Errorf("reading the session grant at %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || expired(info.ModTime(), now) {
		return record{}, false, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return record{}, false, fmt.Errorf("reading the session grant at %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return record{}, false, fmt.Errorf("decoding the session grant at %s: %w", path, err)
	}
	return rec, true, nil
}

// Live returns the operations session is granted at now. A pending grant
// yields none.
func Live(dir, session string, now time.Time) (Ops, error) {
	rec, ok, err := read(dir, session, now)
	if err != nil || !ok || rec.Confirm != "" {
		return nil, err
	}
	return rec.Ops, nil
}

// expired reports whether a grant dated minted is over at now. One dated in
// the future is over too: nothing this package writes produces it.
func expired(minted, now time.Time) bool {
	age := now.Sub(minted)
	return age < 0 || age >= TTL
}

// payload is the part of a hook payload Apply reads, across the events it
// handles. Answers holds raw values because a multi-select answer need not be
// a string, and one that is not must not fail the whole decode.
type payload struct {
	Event     string `json:"hook_event_name"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Command   string `json:"command_name"`
	Args      string `json:"command_args"`
	Tool      string `json:"tool_name"`
	Response  struct {
		Answers map[string]json.RawMessage `json:"answers"`
	} `json:"tool_response"`
}

// Apply reads one hook payload and updates the store.
//
// An expansion of MergeSkill with a merge flag grants PRMerge, and one without
// revokes the session's grant. An expansion of a skill in policies, typed at or
// under one of its roots, leaves a pending grant. A question dialog answered
// with a pending grant's confirming answer makes it live.
//
// Any other payload changes nothing, so registering the command under another
// event cannot mint a grant.
func Apply(r io.Reader, dir string, policies Policies, now time.Time) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading the hook payload from stdin: %w", err)
	}
	var in payload
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("decoding the hook payload as JSON: %w", err)
	}

	switch {
	case in.Event == ExpansionEvent && in.Command == MergeSkill:
		if WantsMerge(in.Args) {
			return write(dir, in.SessionID, record{Ops: Ops{PRMerge}}, now)
		}
		return revoke(dir, in.SessionID)
	case in.Event == ExpansionEvent:
		policy, ok := policies[in.Command]
		if !ok || !underARoot(in.Cwd, policy.Roots) {
			return nil
		}
		return write(dir, in.SessionID, record{Ops: policy.Ops, Confirm: policy.Confirm}, now)
	case in.Event == AnswerEvent && in.Tool == AnswerTool:
		rec, ok, err := read(dir, in.SessionID, now)
		if err != nil || !ok || rec.Confirm == "" || !confirms(in.Response.Answers, rec.Confirm) {
			return err
		}
		return write(dir, in.SessionID, record{Ops: rec.Ops}, now)
	}
	return nil
}

// confirms reports whether any answer the user gave begins with confirm,
// compared without regard to case.
func confirms(answers map[string]json.RawMessage, confirm string) bool {
	for _, raw := range answers {
		var answer string
		if json.Unmarshal(raw, &answer) != nil {
			continue
		}
		answer = strings.TrimSpace(answer)
		if len(answer) >= len(confirm) && strings.EqualFold(answer[:len(confirm)], confirm) {
			return true
		}
	}
	return false
}

// underARoot reports whether dir is one of roots or lies under one. A session
// with no working directory is under none.
func underARoot(dir string, roots []string) bool {
	if dir == "" {
		return false
	}
	dir = filepath.Clean(dir)
	for _, root := range roots {
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// WantsMerge reports whether MergeSkill's arguments put it in merge mode. It
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
