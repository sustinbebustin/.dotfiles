// Package hook holds the parts of a PreToolUse hook that are the same across
// every rule: decoding the tool payload into a Request, reducing the rules'
// verdicts into one (see Merge), and rendering it onto stdout.
package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"

	"claude-hooks/internal/config"
	"claude-hooks/internal/shellast"
)

// Decision is the verdict a rule reaches. Rules return one of these. They are
// ordered by severity, which is how Merge picks the winner.
type Decision int

const (
	// Allow means the rule found nothing to act on.
	Allow Decision = iota
	// Rewrite means the action may run, but only as the rule's replacement
	// command: an equivalent the guards allow.
	Rewrite
	// Ask means the action is risky enough that a human should approve it
	// case by case.
	Ask
	// Deny means the action is not permitted at all.
	Deny
)

// String names a Decision. Every name but "rewrite" is also its wire value; a
// rewrite goes out as an allow carrying the new input (see Encode). A value
// outside the enum renders as Decision(n), which no consumer accepts -- it is a
// bug signal, not a wire value.
func (d Decision) String() string {
	switch d {
	case Allow:
		return "allow"
	case Rewrite:
		return "rewrite"
	case Ask:
		return "ask"
	case Deny:
		return "deny"
	}
	return fmt.Sprintf("Decision(%d)", int(d))
}

// Verdict is a Decision plus the reason shown to the user and the model. Reason
// is required for Ask and Deny and ignored otherwise.
//
// Command and Note come from a rewrite: the replacement command, and what the
// model is told about it, since it sees only the result. A Rewrite always has
// them; an Ask may keep them from a rewrite it outranked (see Merge); an Allow
// or Deny never does.
type Verdict struct {
	Decision Decision
	Reason   string
	Command  string
	Note     string
}

// Allowed is the verdict for "nothing to act on here".
func Allowed() Verdict { return Verdict{Decision: Allow} }

// Rewritten is the verdict for "run command in place of the one sent".
func Rewritten(command, note string) Verdict {
	return Verdict{Decision: Rewrite, Command: command, Note: note}
}

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
	Cwd       json.RawMessage `json:"cwd"`
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
		HookEventName            string                     `json:"hookEventName"`
		PermissionDecision       string                     `json:"permissionDecision"`
		PermissionDecisionReason string                     `json:"permissionDecisionReason,omitempty"`
		UpdatedInput             map[string]json.RawMessage `json:"updatedInput,omitempty"`
		AdditionalContext        string                     `json:"additionalContext,omitempty"`
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
	// Cwd is the directory Claude Code ran the tool in, which is what a
	// relative path in the command is relative to. It is empty when the payload
	// carried none; a rule that resolves paths must still work without it.
	Cwd string
	// Config is the machine-local configuration, attached by the binary after
	// decoding. It is deliberately not a NewRequest parameter: the rule tests
	// that need it set it directly, and the rest get the zero value, which is
	// the no-config state.
	Config config.Config
	// toolInput is tool_input as sent, field by field. A rewrite must hand
	// back the whole input object, so the fields no rule reads are kept too.
	toolInput map[string]json.RawMessage
}

// NewRequest builds a Request from already-decoded fields. It is the seam the
// rule tests construct payloads through. Cwd and Config are set as fields
// rather than passed here, so this does not grow into a row of interchangeable
// string parameters.
//
// A Request is passed around by pointer: it carries the parsed shell tree and
// the configuration, and copying that into every rule is waste the rules have
// no use for. Nothing mutates it after this returns.
//
// The command is parsed as shell only for Bash. Nothing in the payload
// guarantees the tool is what the matcher said, and a `command` field on a
// non-Bash tool is not a shell command.
func NewRequest(toolName, filePath, path, command string) *Request {
	if filePath == "" {
		filePath = path
	}
	r := &Request{ToolName: toolName, FilePath: filePath, Command: command}
	if toolName == "Bash" {
		r.Shell = shellast.Parse(command)
	}
	return r
}

// Embedded returns one derived Request per script this command hands to a
// nested shell (see shellast.EmbeddedScripts), so a rule can check `bash -c
// 'rm -rf ~'` by running over the inner script as if it had arrived on its own.
// It returns nothing for a non-Bash tool, an unparsed command, or a command
// that starts no nested shell.
//
// Each derived Request carries the inner script as its Command and Shell, and
// keeps the rest of the payload -- tool, cwd, config -- from its parent. Giving
// the inner script its own Shell rather than splicing it into the parent tree is
// what keeps node positions meaningful: the rules that compare offsets, and the
// one that slices the reason out of Command, would both read a spliced subtree
// against the wrong text.
//
// The derived shell is a separate process from the parent's, and the parent's
// variables are not resolved into it. That is the safe direction: an unresolved
// target reads as non-scratch, which prompts.
func (r *Request) Embedded() []*Request {
	file, ok := r.Shell.File()
	if !ok {
		return nil
	}
	scripts := shellast.EmbeddedScripts(file)
	if len(scripts) == 0 {
		return nil
	}
	out := make([]*Request, 0, len(scripts))
	for _, script := range scripts {
		sub := *r
		sub.Command = script
		sub.Shell = shellast.Parse(script)
		out = append(out, &sub)
	}
	return out
}

// Read decodes the hook payload from stdin.
//
// Only a payload that is not JSON at all is an error, and it is reported as "no
// usable input" rather than as a hard failure: a hook that cannot read its input
// has no grounds to block anything, and the caller treats this as an Allow.
// Within a payload that does parse, a field of the wrong type is dropped on its
// own and the rest is still checked.
func Read(r io.Reader) (*Request, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading hook input from stdin: %w", err)
	}
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decoding hook input as JSON: %w", err)
	}
	fields := toolInputFields(in.ToolInput)
	req := NewRequest(
		jsonString(in.ToolName),
		jsonString(fields["file_path"]),
		jsonString(fields["path"]),
		jsonString(fields["command"]),
	)
	req.Cwd = jsonString(in.Cwd)
	req.toolInput = fields
	return req, nil
}

// Encode renders v as the PreToolUse wire bytes, including the trailing
// newline json.Encoder writes. It does no I/O, so tests can pin the exact
// bytes without running a binary.
//
// A rewrite goes out as an allow, or as an ask when it was outranked by one, so
// the prompt shows the command that will actually run. Its updatedInput is req's
// tool input with the command replaced: Claude Code swaps in the whole object,
// so every other field is carried over as sent. The note goes to the model as
// additionalContext. req may be nil when v carries no rewrite.
func Encode(v Verdict, req *Request) ([]byte, error) {
	var out output
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	switch v.Decision {
	case Allow:
		out.HookSpecificOutput.PermissionDecision = v.Decision.String()
	case Rewrite:
		out.HookSpecificOutput.PermissionDecision = Allow.String()
	default:
		out.HookSpecificOutput.PermissionDecision = v.Decision.String()
		out.HookSpecificOutput.PermissionDecisionReason = v.Reason
	}
	if v.Command != "" && v.Decision != Deny {
		command, err := json.Marshal(v.Command)
		if err != nil {
			return nil, err
		}
		fields := map[string]json.RawMessage{}
		if req != nil {
			maps.Copy(fields, req.toolInput)
		}
		fields["command"] = command
		out.HookSpecificOutput.UpdatedInput = fields
		out.HookSpecificOutput.AdditionalContext = v.Note
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Render writes v, reached on req, to stdout as a PreToolUse decision and
// exits: 0 when the decision was written, 2 when it could not be. It never
// returns. req may be nil when v is not a rewrite (see Encode).
func Render(name string, req *Request, v Verdict) {
	raw, err := Encode(v, req)
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
