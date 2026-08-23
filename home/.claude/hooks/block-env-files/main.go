// Command block-env-files is a PreToolUse hook that denies access to secret .env
// files. Read/Edit/Write are checked by their path argument; Bash commands are
// parsed and every word is inspected, so reading, copying, sourcing, or
// redirecting an env file is caught wherever it appears in the command. Example
// variants (.env.example, .sample, .template) stay allowed so the documented
// shape of a config remains readable.
package main

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"claude-hooks/internal/hookio"

	"mvdan.cc/sh/v3/syntax"
)

const hookName = "block-env-files"

func main() {
	in, err := hookio.Read()
	if err != nil {
		hookio.Render(hookName, hookio.Allowed())
	}

	filePath := in.ToolInput.FilePath
	if filePath == "" {
		filePath = in.ToolInput.Path
	}

	switch in.ToolName {
	case "Read":
		if isBlockedEnv(filePath) {
			hookio.Render(hookName, hookio.Denied(fmt.Sprintf("Access to .env files is blocked for security. Use .env.example as a reference. (path: %s)", filePath)))
		}
	case "Edit":
		if isBlockedEnv(filePath) {
			hookio.Render(hookName, hookio.Denied(fmt.Sprintf("Editing .env files is blocked for security. (path: %s)", filePath)))
		}
	case "Write":
		if isBlockedEnv(filePath) {
			hookio.Render(hookName, hookio.Denied(fmt.Sprintf("Writing to .env files is blocked for security. (path: %s)", filePath)))
		}
	case "Grep":
		if isBlockedEnv(filePath) {
			hookio.Render(hookName, hookio.Denied(fmt.Sprintf("Searching .env files is blocked for security. (path: %s)", filePath)))
		}
	case "Bash":
		if reason := checkBash(in.ToolInput.Command); reason != "" {
			hookio.Render(hookName, hookio.Denied(reason))
		}
	}
	hookio.Render(hookName, hookio.Allowed())
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
