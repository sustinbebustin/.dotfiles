// Command block-aws-cli is a PreToolUse hook that prompts before any `aws` CLI
// invocation runs. It mirrors block-dangerous-rm: instead of a hard `deny`, it
// returns `ask` so the user approves each AWS command case by case. The whole
// shell tree is walked, so `aws` nested in pipelines, subshells, command
// substitutions, and && / || chains is still caught.
package main

import (
	"strings"

	"claude-hooks/internal/hookio"

	"mvdan.cc/sh/v3/syntax"
)

const hookName = "block-aws-cli"

func main() {
	in, err := hookio.Read()
	if err != nil || in.ToolName != "Bash" || in.ToolInput.Command == "" {
		hookio.Render(hookName, hookio.Allowed())
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(in.ToolInput.Command), "")
	if err != nil {
		hookio.Render(hookName, hookio.Allowed())
	}

	// Walk the whole tree so aws nested in pipelines, subshells, command
	// substitutions, && / || chains, etc. is still caught.
	var hit *hookio.Verdict
	syntax.Walk(file, func(n syntax.Node) bool {
		if hit != nil {
			return false
		}
		if call, ok := n.(*syntax.CallExpr); ok {
			if v, found := checkAWS(call); found {
				v := v
				hit = &v
				return false
			}
		}
		return true
	})
	if hit != nil {
		hookio.Render(hookName, *hit)
	}
	hookio.Render(hookName, hookio.Allowed())
}

// checkAWS returns an "ask" verdict when c invokes the `aws` CLI. Environment
// assignments preceding the command (e.g. `AWS_PROFILE=x aws ...`) are skipped
// so the aws binary is still recognized as the command word.
func checkAWS(c *syntax.CallExpr) (hookio.Verdict, bool) {
	if len(c.Args) == 0 {
		return hookio.Verdict{}, false
	}
	if cmd := commandName(wordLit(c.Args[0])); cmd == "aws" {
		return hookio.Asked("AWS CLI command detected. Allow?"), true
	}
	return hookio.Verdict{}, false
}

// commandName strips any leading path so `/usr/local/bin/aws` and `aws` both
// resolve to "aws".
func commandName(tok string) string {
	if i := strings.LastIndex(tok, "/"); i >= 0 {
		return tok[i+1:]
	}
	return tok
}

// wordLit renders the literal parts of a word, ignoring expansions (variables,
// command/arithmetic substitution).
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
