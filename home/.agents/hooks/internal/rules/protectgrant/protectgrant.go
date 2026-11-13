// Package protectgrant keeps the agent from granting itself what only the
// user can. A session grant (see package grant) lets a session's merges,
// releases, workflow dispatches and pushes to main run unprompted, and it is
// meant to exist only because the user typed a skill and approved it. The
// store is an ordinary directory and the minter an ordinary command, so both
// are open to the agent's own tool calls; this rule sends those to the user as
// an ask.
//
// Two routes are covered: an Edit or Write whose path, or a Bash command whose
// text, names the store directory; and a Bash call that runs the guard binary's
// grant mode. The text match is deliberately blunt -- it also asks for a
// search or a commit message that merely mentions the directory -- because a
// path can reach a command through an assignment, a redirect or a nested
// script, and an ask costs one approval where a miss costs the guard.
//
// What that does not reach: a command that assembles the directory name from
// pieces, or a program other than the guard binary that writes the file. The
// agent runs as the user, so nothing here can make a grant unforgeable; the
// rule closes the routes that take no effort to find.
package protectgrant

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/grant"
	"agent-hooks/internal/hook"
	"agent-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "protect-grants"

// minter is the guard binary, whose grant mode mints and revokes grants.
const minter = "agent-guard-bin"

const storeReason = "This names the session grant store (" + grant.DirName + "). A grant there lets " +
	"merges, releases, workflow dispatches or pushes to main run unprompted for its session - allow?"

const mintReason = minter + " " + grant.Subcommand + " mints or revokes a session grant, which lets " +
	"merges, releases, workflow dispatches or pushes to main run unprompted for its session - allow?"

// minterText matches text that names the guard binary and then, as a whole
// word, its grant mode.
var minterText = regexp.MustCompile(regexp.QuoteMeta(minter) + `\b[\s\S]*?\b` + grant.Subcommand + `\b`)

// Check asks before a tool call that names the grant store or runs the minter.
func Check(req *hook.Request) hook.Verdict {
	switch req.ToolName {
	case "Edit", "Write":
		if strings.Contains(req.FilePath, grant.DirName) {
			return hook.Asked(storeReason)
		}
	case "Bash":
		if strings.Contains(req.Command, grant.DirName) {
			return hook.Asked(storeReason)
		}
		if mints(req) {
			return hook.Asked(mintReason)
		}
	}
	return hook.Allowed()
}

// mints reports whether the command runs the guard binary's grant mode. A
// command the parser rejects may still run, so there the text decides.
func mints(req *hook.Request) bool {
	file, ok := req.Shell.File()
	if !ok {
		return req.Shell.Status() == shellast.Unparseable && minterText.MatchString(req.Command)
	}
	_, hit := shellast.FirstCall(file, func(c *syntax.CallExpr) (struct{}, bool) {
		name, operands := shellast.Invocation(c.Args, shellast.WordLit)
		if shellast.CommandName(name) != minter {
			return struct{}{}, false
		}
		for _, o := range operands {
			// An operand that is not plain text could expand to the mode.
			if !shellast.IsLiteral(o) || shellast.WordLit(o) == grant.Subcommand {
				return struct{}{}, true
			}
		}
		return struct{}{}, false
	})
	return hit
}
