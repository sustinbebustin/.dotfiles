// Package awscli prompts before any `aws` CLI invocation runs. Instead of a
// hard deny it returns ask, so the user approves each AWS command case by case.
// The whole shell tree is walked, so `aws` nested in pipelines, subshells,
// command substitutions, and && / || chains is still caught.
package awscli

import (
	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-aws-cli"

// Check asks for approval when req invokes the aws CLI.
func Check(req hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	if v, hit := shellast.FirstCall(file, checkAWS); hit {
		return v
	}
	return hook.Allowed()
}

// checkAWS returns an "ask" verdict when c invokes the `aws` CLI. Environment
// assignments preceding the command (e.g. `AWS_PROFILE=x aws ...`) are skipped
// so the aws binary is still recognized as the command word.
func checkAWS(c *syntax.CallExpr) (hook.Verdict, bool) {
	if len(c.Args) == 0 {
		return hook.Verdict{}, false
	}
	if cmd := shellast.CommandName(shellast.WordLit(c.Args[0])); cmd == "aws" {
		return hook.Asked("AWS CLI command detected. Allow?"), true
	}
	return hook.Verdict{}, false
}
