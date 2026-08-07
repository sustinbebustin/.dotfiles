// Command block-env-files is a Claude Code PreToolUse hook that denies access to
// secret .env files. Read/Edit/Write are checked by their path argument; Bash
// commands are parsed and every word is inspected, so reading, copying,
// sourcing, or redirecting an env file is caught wherever it appears in the
// command. Example variants (.env.example, .sample, .template) stay allowed so
// the documented shape of a config remains readable.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
		Command  string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		emitAllow()
		return
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		emitAllow()
		return
	}

	filePath := in.ToolInput.FilePath
	if filePath == "" {
		filePath = in.ToolInput.Path
	}

	switch in.ToolName {
	case "Read":
		if isBlockedEnv(filePath) {
			emitDeny(fmt.Sprintf("Access to .env files is blocked for security. Use .env.example as a reference. (path: %s)", filePath))
			return
		}
	case "Edit":
		if isBlockedEnv(filePath) {
			emitDeny(fmt.Sprintf("Editing .env files is blocked for security. (path: %s)", filePath))
			return
		}
	case "Write":
		if isBlockedEnv(filePath) {
			emitDeny(fmt.Sprintf("Writing to .env files is blocked for security. (path: %s)", filePath))
			return
		}
	case "Grep":
		if isBlockedEnv(filePath) {
			emitDeny(fmt.Sprintf("Searching .env files is blocked for security. (path: %s)", filePath))
			return
		}
	case "Bash":
		if reason := checkBash(in.ToolInput.Command); reason != "" {
			emitDeny(reason)
			return
		}
	}
	emitAllow()
}

// isBlockedEnv returns true when the basename names an env file that is NOT
// one of the safe template variants (.example, .sample, .template).
func isBlockedEnv(p string) bool {
	if p == "" {
		return false
	}
	base := path.Base(p)
	if base == "" || base == "." || base == "/" {
		return false
	}
	if !looksLikeEnv(base) {
		return false
	}
	if isSafeEnv(base) {
		return false
	}
	return true
}

// looksLikeEnv returns true for basenames such as `.env`, `.envrc`,
// `.env.local`, `.env-staging`, `prod.env`, and `app.env.local`.
//
// The leading-dot `.env*` prefix deliberately covers direnv's `.envrc` and any
// other `.env`-prefixed dotfile -- those carry secrets just like `.env` does
// and were the gap that previously let `.envrc` leak.
func looksLikeEnv(base string) bool {
	if strings.HasPrefix(base, ".env") {
		return true
	}
	if strings.HasSuffix(base, ".env") {
		return true
	}
	if strings.Contains(base, ".env.") || strings.Contains(base, ".env-") {
		return true
	}
	return false
}

func isSafeEnv(base string) bool {
	for _, marker := range []string{".example", ".sample", ".template"} {
		if strings.HasSuffix(base, marker) || strings.Contains(base, marker+".") {
			return true
		}
	}
	return false
}

// checkBash blocks any command that references a blocked env file, whether as a
// command argument (`cat .env`, `xxd .envrc`, `cp .env /tmp`, `source .env`) or
// as a redirection target (`tr a b < .env`). There is intentionally no
// allowlist of "reader" commands: enumerating every binary that can read a file
// is a losing game, and there is no benign reason for a command to name a
// secret env file. Returns a non-empty reason if the command should be blocked.
func checkBash(cmd string) string {
	if cmd == "" {
		return ""
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		// Unparseable: fail CLOSED if the raw text names a blocked env file.
		// Better to over-block a malformed command than to leak on a parse
		// edge case the AST walk would have caught.
		if rawMentionsBlockedEnv(cmd) {
			return "This command references a .env file but could not be parsed safely, so it is blocked. Use the .example/.sample/.template variant, or load the value through your secret manager."
		}
		return ""
	}
	var found string
	syntax.Walk(file, func(n syntax.Node) bool {
		if found != "" {
			return false
		}
		switch x := n.(type) {
		case *syntax.CallExpr:
			for _, arg := range x.Args {
				if lit := wordLit(arg); isBlockedEnv(lit) {
					found = blockReason(lit)
					return false
				}
			}
		case *syntax.Redirect:
			if lit := wordLit(x.Word); isBlockedEnv(lit) {
				found = blockReason(lit)
				return false
			}
		}
		return true
	})
	return found
}

func blockReason(p string) string {
	return fmt.Sprintf("Blocked: command references the env file %q. Reading, copying, sourcing, or redirecting a secret env file is not allowed. Use the .example/.sample/.template variant, or load the value through your secret manager.", p)
}

// rawMentionsBlockedEnv is the fail-closed fallback for commands the shell
// parser rejects. It splits on whitespace and shell metacharacters and checks
// each resulting token against isBlockedEnv.
func rawMentionsBlockedEnv(cmd string) bool {
	fields := strings.FieldsFunc(cmd, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ';', '|', '&', '<', '>', '(', ')', '"', '\'', '=':
			return true
		default:
			return false
		}
	})
	return slices.ContainsFunc(fields, isBlockedEnv)
}

func wordLit(w *syntax.Word) string {
	if w == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range w.Parts {
		switch x := p.(type) {
		case *syntax.Lit:
			sb.WriteString(x.Value)
		case *syntax.SglQuoted:
			sb.WriteString(x.Value)
		case *syntax.DblQuoted:
			for _, dp := range x.Parts {
				if lit, ok := dp.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				}
			}
		}
	}
	return sb.String()
}

func emitAllow() {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "allow"
	write(out)
}

func emitDeny(reason string) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	write(out)
}

// write emits the decision as JSON on stdout. A failed write would leave Claude
// Code with no decision at all, and a guard hook that says nothing lets the
// access through unchecked. So this fails closed: exit 2 blocks the tool call
// on PreToolUse and hands stderr to Claude as the reason.
//
// This covers a genuine write failure on the target (a full disk, EIO). It does
// not cover stdout being closed outright: the Go runtime reopens closed standard
// descriptors onto /dev/null before main runs, so the write reports success and
// the decision is silently discarded. That branch is therefore unreachable from
// a closed stdout and is left unverified.
func write(out hookOutput) {
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "block-env-files: could not write the hook decision (%v). "+
			"Blocking this access rather than allowing it unchecked; retry once stdout works.\n", err)
		os.Exit(2)
	}
}
