package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	} `json:"hookSpecificOutput"`
}

type verdict struct {
	decision string
	reason   string
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(verdict{decision: "allow"})
		return
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil || in.ToolName != "Bash" || in.ToolInput.Command == "" {
		emit(verdict{decision: "allow"})
		return
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(in.ToolInput.Command), "")
	if err != nil {
		emit(verdict{decision: "allow"})
		return
	}

	for _, stmt := range file.Stmts {
		if v, hit := evaluate(stmt.Cmd); hit {
			emit(v)
			return
		}
	}
	emit(verdict{decision: "allow"})
}

func evaluate(cmd syntax.Command) (verdict, bool) {
	switch c := cmd.(type) {
	case *syntax.CallExpr:
		return checkCall(c)
	case *syntax.BinaryCmd:
		if v, hit := evaluate(c.X.Cmd); hit {
			return v, true
		}
		return evaluate(c.Y.Cmd)
	case *syntax.Subshell:
		for _, s := range c.Stmts {
			if v, hit := evaluate(s.Cmd); hit {
				return v, true
			}
		}
	case *syntax.Block:
		for _, s := range c.Stmts {
			if v, hit := evaluate(s.Cmd); hit {
				return v, true
			}
		}
	}
	return verdict{}, false
}

func checkCall(c *syntax.CallExpr) (verdict, bool) {
	if len(c.Args) == 0 {
		return verdict{}, false
	}
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = wordLit(a)
	}

	switch args[0] {
	case "git":
		return checkGit(args[1:])
	case "gh":
		return checkGh(args[1:])
	}
	return verdict{}, false
}

func checkGit(args []string) (verdict, bool) {
	sub, rest := subcommand(args, gitTopLevelFlags)
	switch sub {
	case "push":
		return verdict{decision: "ask", reason: "git push detected - allow?"}, true
	case "merge":
		return verdict{decision: "deny", reason: "[BLOCKED] git merge - branch merges not allowed"}, true
	case "rebase":
		return verdict{decision: "deny", reason: "[BLOCKED] git rebase - rebasing not allowed"}, true
	case "reset":
		if hasFlag(rest, "--hard") {
			return verdict{decision: "deny", reason: "[BLOCKED] git reset --hard - destructive reset not allowed"}, true
		}
	case "clean":
		return verdict{decision: "deny", reason: "[BLOCKED] git clean - file deletion not allowed"}, true
	case "branch":
		for _, a := range rest {
			if a == "-d" || a == "-D" || a == "--delete" {
				return verdict{decision: "deny", reason: "[BLOCKED] git branch delete not allowed"}, true
			}
		}
	case "checkout":
		for _, a := range rest {
			if a == "--" {
				return verdict{decision: "deny", reason: "[BLOCKED] git checkout -- discards changes, not allowed"}, true
			}
		}
	case "restore":
		return verdict{decision: "deny", reason: "[BLOCKED] git restore - discarding changes not allowed"}, true
	case "stash":
		if len(rest) > 0 && (rest[0] == "drop" || rest[0] == "clear") {
			return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] git stash %s - stash destruction not allowed", rest[0])}, true
		}
	case "tag":
		for _, a := range rest {
			if a == "-d" || a == "--delete" {
				return verdict{decision: "deny", reason: "[BLOCKED] git tag delete not allowed"}, true
			}
		}
	}
	return verdict{}, false
}

func checkGh(args []string) (verdict, bool) {
	if len(args) == 0 {
		return verdict{}, false
	}
	switch args[0] {
	case "pr":
		if len(args) > 1 {
			switch args[1] {
			case "merge":
				return verdict{decision: "deny", reason: "[BLOCKED] gh pr merge - PR merging not allowed"}, true
			case "close":
				return verdict{decision: "deny", reason: "[BLOCKED] gh pr close - PR closing not allowed"}, true
			}
		}
	case "issue":
		if len(args) > 1 && (args[1] == "close" || args[1] == "delete") {
			return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh issue %s not allowed", args[1])}, true
		}
	case "release":
		if len(args) > 1 && (args[1] == "create" || args[1] == "delete") {
			return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh release %s not allowed", args[1])}, true
		}
	case "repo":
		if len(args) > 1 && (args[1] == "delete" || args[1] == "rename") {
			return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh repo %s not allowed", args[1])}, true
		}
	case "api":
		for i, a := range args[1:] {
			if a == "-X" || a == "--method" {
				if i+2 < len(args) {
					m := strings.ToUpper(args[i+2])
					if m == "PUT" || m == "POST" || m == "PATCH" || m == "DELETE" {
						return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh api %s not allowed", m)}, true
					}
				}
			} else if strings.HasPrefix(a, "-X") && len(a) > 2 {
				m := strings.ToUpper(a[2:])
				if m == "PUT" || m == "POST" || m == "PATCH" || m == "DELETE" {
					return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh api %s not allowed", m)}, true
				}
			} else if strings.HasPrefix(a, "--method=") {
				m := strings.ToUpper(strings.TrimPrefix(a, "--method="))
				if m == "PUT" || m == "POST" || m == "PATCH" || m == "DELETE" {
					return verdict{decision: "deny", reason: fmt.Sprintf("[BLOCKED] gh api %s not allowed", m)}, true
				}
			}
		}
	}
	return verdict{}, false
}

// gitTopLevelFlags are flags that may appear before the git subcommand and
// take a value. They are skipped when looking for the subcommand.
var gitTopLevelFlags = map[string]bool{
	"-C":           true,
	"-c":           true,
	"--git-dir":    true,
	"--work-tree":  true,
	"--namespace":  true,
	"--exec-path":  true,
	"--config-env": true,
}

// subcommand returns the first non-flag token after skipping known flags
// (and their values where applicable), plus the remaining args after it.
func subcommand(args []string, flagsWithValue map[string]bool) (string, []string) {
	i := 0
	for i < len(args) {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			return a, args[i+1:]
		}
		// Flag with attached value (--foo=bar): just skip this token.
		if strings.Contains(a, "=") {
			i++
			continue
		}
		// Flag with separate value: skip both.
		if flagsWithValue[a] {
			i += 2
			continue
		}
		// Bare flag: skip just this token.
		i++
	}
	return "", nil
}

func hasFlag(args []string, want string) bool {
	for _, a := range args {
		if a == want || strings.HasPrefix(a, want+"=") {
			return true
		}
	}
	return false
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

func emit(v verdict) {
	out := hookOutput{}
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = v.decision
	out.HookSpecificOutput.PermissionDecisionReason = v.reason
	json.NewEncoder(os.Stdout).Encode(out)
}
