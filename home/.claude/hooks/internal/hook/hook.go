// Package hook holds the parts of a PreToolUse hook that are the same across
// every rule: decoding the tool payload into a Request, reducing the rules'
// verdicts into one (see Merge), and rendering it onto stdout.
package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"claude-hooks/internal/shellast"
)

// Decision is the verdict a rule reaches. Rules return one of these;
// [Decision.String] gives the name used on the wire.
type Decision int

const (
	// Allow means the rule found nothing to act on.
	Allow Decision = iota
	// Ask means the action is risky enough that a human should approve it
	// case by case.
	Ask
	// Deny means the action is not permitted at all.
	Deny
)

// String renders a Decision as the name Claude Code uses on the wire. A value
// outside the enum renders as Decision(n), which no consumer accepts -- it is a
// bug signal, not a wire value.
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

// input is the subset of the PreToolUse payload the rules read. tool_input
// varies by tool, so the fields the rules look at are pulled out of it
// individually.
//
// Everything is held as raw JSON and decoded field by field on purpose. A
// single typed struct decodes all-or-nothing, so one field of an unexpected
// type -- and tool_input is filled in by the model -- would fail the whole
// payload and hand the rules an empty Request. That is an allow, which means
// `{"command":"rm -rf /","file_path":123}` would walk past every guard.
type input struct {
	ToolName  json.RawMessage `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// jsonString decodes raw as a JSON string. A field that is absent, null, or of
// some other type yields "", which reads as "not supplied" -- the fields are
// independent, so an unusable one must not take its neighbours with it.
func jsonString(raw json.RawMessage) string {
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

// toolInputFields decodes tool_input into its raw fields. A tool_input that is
// not an object yields no fields rather than an error.
func toolInputFields(raw json.RawMessage) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &fields) != nil {
		return nil
	}
	return fields
}

// output is the PreToolUse wire format.
type output struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

// Request is one decoded PreToolUse payload. Shell is parsed once here rather
// than separately by each rule that needs it.
type Request struct {
	ToolName string
	// FilePath is tool_input.file_path, falling back to tool_input.path for
	// the tools that name the field that way (Grep).
	FilePath string
	Command  string
	Shell    shellast.Shell
}

// NewRequest builds a Request from already-decoded fields. It is the seam the
// rule tests construct payloads through.
//
// The command is parsed as shell only for Bash. Nothing in the payload
// guarantees the tool is what the matcher said, and a `command` field on a
// non-Bash tool is not a shell command.
func NewRequest(toolName, filePath, path, command string) Request {
	if filePath == "" {
		filePath = path
	}
	r := Request{ToolName: toolName, FilePath: filePath, Command: command}
	if toolName == "Bash" {
		r.Shell = shellast.Parse(command)
	}
	return r
}

// Read decodes the hook payload from stdin.
//
// Only a payload that is not JSON at all is an error, and it is reported as "no
// usable input" rather than as a hard failure: a hook that cannot read its input
// has no grounds to block anything, and the caller treats this as an Allow.
// Within a payload that does parse, a field of the wrong type is dropped on its
// own and the rest is still checked.
func Read(r io.Reader) (Request, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Request{}, fmt.Errorf("reading hook input from stdin: %w", err)
	}
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return Request{}, fmt.Errorf("decoding hook input as JSON: %w", err)
	}
	fields := toolInputFields(in.ToolInput)
	return NewRequest(
		jsonString(in.ToolName),
		jsonString(fields["file_path"]),
		jsonString(fields["path"]),
		jsonString(fields["command"]),
	), nil
}

// Encode renders v as the PreToolUse wire bytes, including the trailing
// newline json.Encoder writes. It does no I/O, so tests can pin the exact
// bytes without running a binary.
func Encode(v Verdict) ([]byte, error) {
	var out output
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.Decision.String()
	if v.Decision != Allow {
		out.HookSpecificOutput.PermissionDecisionReason = v.Reason
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Render writes v to stdout as a PreToolUse decision and exits: 0 when the
// decision was written, 2 when it could not be. It never returns.
func Render(name string, v Verdict) {
	raw, err := Encode(v)
	if err == nil {
		_, err = os.Stdout.Write(raw)
	}
	if err != nil {
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
