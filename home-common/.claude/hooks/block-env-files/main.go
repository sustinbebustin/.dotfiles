package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
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

// Reader/exposer commands that print or send file contents somewhere visible.
var dangerousReaders = map[string]bool{
	"cat":  true,
	"less": true,
	"more": true,
	"head": true,
	"tail": true,
	"bat":  true,
	"nano": true,
	"vim":  true,
	"vi":   true,
	"code": true,
	"subl": true,
	"open": true,
	// Search/transform tools that can leak contents:
	"grep":  true,
	"egrep": true,
	"fgrep": true,
	"rg":    true,
	"awk":   true,
	"sed":   true,
	// Sourcing also exposes via the running shell:
	"source": true,
	".":      true,
	"export": true,
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

// looksLikeEnv returns true for basenames such as `.env`, `.env.local`,
// `.env-staging`, `prod.env`, and `app.env.local`.
func looksLikeEnv(base string) bool {
	if base == ".env" {
		return true
	}
	if strings.HasPrefix(base, ".env.") || strings.HasPrefix(base, ".env-") {
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

// checkBash inspects a parsed command for any dangerous-reader call whose
// arguments include a blocked env file path. Returns a non-empty reason if
// the command should be blocked.
func checkBash(cmd string) string {
	if cmd == "" {
		return ""
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return "" // unparseable -- fail open
	}
	var found string
	syntax.Walk(file, func(n syntax.Node) bool {
		if found != "" {
			return false
		}
		call, ok := n.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		name := wordLit(call.Args[0])
		if !dangerousReaders[name] {
			return true
		}
		for _, arg := range call.Args[1:] {
			lit := wordLit(arg)
			if isBlockedEnv(lit) {
				found = fmt.Sprintf("`%s %s` would expose a .env file. Use the .example/.sample/.template variant instead, or load the value through your secret manager.", name, lit)
				return false
			}
		}
		return true
	})
	return found
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
	json.NewEncoder(os.Stdout).Encode(out)
}

func emitDeny(reason string) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	json.NewEncoder(os.Stdout).Encode(out)
}
