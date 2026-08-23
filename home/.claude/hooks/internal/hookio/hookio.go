// Package hookio holds the parts of a PreToolUse hook that are the same across
// every hook in this tree: decoding the tool input and rendering a verdict onto
// stdout.
package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Decision is the verdict a hook reaches. Hooks return one of these; Render maps
// it to the wire format.
type Decision int

const (
	// Allow means the hook found nothing to act on.
	Allow Decision = iota
	// Ask means the action is risky enough that a human should approve it
	// case by case.
	Ask
	// Deny means the action is not permitted at all.
	Deny
)

// String renders a Decision as the name Claude Code uses on the wire. It is the
// spelling the hook test tables assert against.
func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Ask:
		return "ask"
	case Deny:
		return "deny"
	}
	return fmt.Sprintf("Decision(%d)", int(d))
}

// Verdict is a Decision plus the reason shown to the user and the model. Reason
// is required for Ask and Deny and ignored for Allow.
type Verdict struct {
	Decision Decision
	Reason   string
}

// Allowed is the verdict for "nothing to act on here".
func Allowed() Verdict { return Verdict{Decision: Allow} }

// Asked is the verdict for "a human should approve this".
func Asked(reason string) Verdict { return Verdict{Decision: Ask, Reason: reason} }

// Denied is the verdict for "this is not permitted".
func Denied(reason string) Verdict { return Verdict{Decision: Deny, Reason: reason} }

// Input is the subset of the PreToolUse payload these hooks read. tool_input
// varies by tool, so the fields here are the union of what the hooks in this
// tree look at.
type Input struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
}

// output is the PreToolUse wire format.
type output struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// Read decodes the hook payload from stdin.
//
// Every failure here is reported as "no usable input" rather than as a hard
// error. A hook that cannot read its input has no grounds to block anything,
// and the callers all treat this as an Allow.
func Read() (Input, error) {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return Input{}, fmt.Errorf("reading hook input from stdin: %w", err)
	}
	var in Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return Input{}, fmt.Errorf("decoding hook input as JSON: %w", err)
	}
	return in, nil
}

// Render writes v to stdout as a PreToolUse decision, then exits 0.
func Render(name string, v Verdict) {
	var out output
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.Decision.String()
	if v.Decision != Allow {
		out.HookSpecificOutput.PermissionDecisionReason = v.Reason
	}

	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		// A failed write would leave Claude Code with no decision at all, and a
		// guard that says nothing lets the action through unchecked. So this
		// fails closed: exit 2 blocks the tool call on PreToolUse and hands
		// stderr to the model as the reason.
		//
		// This covers a genuine write failure on the target (a full disk, EIO).
		// It does not cover stdout being closed outright: the Go runtime reopens
		// closed standard descriptors onto /dev/null before main runs, so the
		// write reports success and the decision is silently discarded. That
		// branch is therefore unreachable from a closed stdout and is left
		// unverified.
		fatalf(name, "could not write the hook decision (%v). Blocking this action rather than "+
			"allowing it unchecked; retry once stdout works.%s", err, reasonTail(v.Reason))
	}
	os.Exit(0)
}

func reasonTail(reason string) string {
	if reason == "" {
		return ""
	}
	return " Original reason: " + reason
}

// fatalf reports a hook-level failure on stderr and exits 2, which blocks the
// tool call on PreToolUse.
func fatalf(name, format string, args ...any) {
	fmt.Fprintf(os.Stderr, name+": "+format+"\n", args...)
	os.Exit(2)
}
