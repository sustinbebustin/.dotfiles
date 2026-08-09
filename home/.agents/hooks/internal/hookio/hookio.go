// Package hookio holds the parts of a PreToolUse hook that are the same across
// every hook in this tree: reading the harness flag, decoding the tool input,
// and rendering a verdict onto stdout.
//
// Claude Code and Codex share a hook contract -- same event names, same stdin
// envelope, same hookSpecificOutput shape, same exit-2 fallback -- but they do
// not share every permission decision. The differences are confined to Render
// below; everything upstream of it works in terms of a Verdict and never needs
// to know which harness is running.
package hookio

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// Harness is the agent CLI a hook is running under. The zero value is Claude,
// which is the harness these hooks were written for and the one they run under
// unless a config explicitly says otherwise.
type Harness int

const (
	Claude Harness = iota
	Codex
)

// Decision is the verdict a hook reaches, independent of how any harness spells
// it. Hooks return one of these; Render maps it to the wire format.
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

// String renders a Decision as the name both harnesses use on the wire. It is
// the spelling the hook test tables assert against.
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

// codexAskSuffix is appended when an Ask verdict has to be downgraded to a deny
// under Codex. Codex parses permissionDecision "ask" but marks the hook run
// failed and lets the tool run, so honoring Ask literally there would turn a
// guard into a no-op. Blocking is the safe direction, and the text has to tell
// the reader why the answer differs from the Claude one and how to get the
// command run anyway.
const codexAskSuffix = " (Codex cannot prompt for approval from a PreToolUse hook, so this is blocked " +
	"rather than allowed unchecked. Nothing has been changed. If this is intended, ask the user to run it themselves.)"

// Input is the subset of the PreToolUse payload these hooks read. Both harnesses
// send the same envelope; tool_input varies by tool, so the fields here are the
// union of what the hooks in this tree look at.
type Input struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
}

// output is the PreToolUse wire format. Both harnesses accept this shape.
type output struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// ParseHarness reads the -harness flag. It defaults to Claude so that a config
// which passes no flag -- which is every Claude Code config -- gets the behavior
// these hooks were written for.
//
// An unrecognized value is fatal rather than a fallback to the default: a guard
// that quietly runs in the wrong mode is worse than one that refuses to start,
// and a typo in a hooks config should be loud.
func ParseHarness(name string) Harness {
	var raw string
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&raw, "harness", "claude", "agent CLI this hook is running under: claude or codex")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatalf(name, "could not parse arguments (%v). Expected an optional -harness=claude|codex.", err)
	}
	switch raw {
	case "claude":
		return Claude
	case "codex":
		return Codex
	default:
		fatalf(name, "unknown -harness value %q. Expected \"claude\" or \"codex\".", raw)
		return Claude // unreachable; fatalf exits
	}
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

// Render writes v to stdout in the form the given harness understands, then
// exits 0.
//
// The two harness-specific rules:
//
//   - Allow is silent under Codex. Claude needs the explicit "allow" it has
//     always emitted, but under Codex an explicit allow bypasses its approval
//     flow, so every command a hook did not flag would be auto-approved. Saying
//     nothing leaves Codex's own approval policy in charge.
//   - Ask becomes Deny under Codex, per codexAskSuffix.
func Render(name string, h Harness, v Verdict) {
	var decision, reason string
	switch v.Decision {
	case Allow:
		if h == Codex {
			os.Exit(0)
		}
		decision = "allow"
	case Ask:
		if h == Codex {
			decision, reason = "deny", v.Reason+codexAskSuffix
			break
		}
		decision, reason = "ask", v.Reason
	case Deny:
		decision, reason = "deny", v.Reason
	}

	var out output
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = decision
	out.HookSpecificOutput.PermissionDecisionReason = reason

	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		// A failed write would leave the harness with no decision at all, and a
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
			"allowing it unchecked; retry once stdout works.%s", err, reasonTail(reason))
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
// tool call on PreToolUse in both harnesses.
func fatalf(name, format string, args ...any) {
	fmt.Fprintf(os.Stderr, name+": "+format+"\n", args...)
	os.Exit(2)
}
