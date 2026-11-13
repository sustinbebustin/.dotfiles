// Command agent-guard is the PreToolUse guard for Claude Code and Factory
// Droid. It reads one hook payload on stdin, runs every registered rule that
// applies to the tool, and writes a single permission decision to stdout. Both
// harnesses share the wire format; Droid's tool names are translated on the way
// in (see hook.CanonicalTool).
//
// Rules are registered in internal/rules; their verdicts are reduced to one by
// hook.Merge, worst wins.
//
// Usage:
//
//	agent-guard           # run every applicable rule (the form settings.json invokes)
//	agent-guard <rule>    # run one rule only, for debugging or a narrower hook
//	agent-guard list      # print the rules, their tools, and the matchers
//	agent-guard grant     # mint or revoke the session's merge grant
//
// The grant form is not a PreToolUse hook. It is registered under
// UserPromptExpansion and reads that event's payload; see package grant.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"agent-hooks/internal/config"
	"agent-hooks/internal/grant"
	"agent-hooks/internal/hook"
	"agent-hooks/internal/rules"
)

const progName = "agent-guard"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "list" {
		list()
		return
	}
	if len(args) == 1 && args[0] == grant.Subcommand {
		applyGrant()
		return
	}

	selected, err := selectRules(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(2)
	}

	// A payload that cannot be read is no grounds to block anything.
	req, err := hook.Read(os.Stdin)
	if err != nil {
		hook.Render(progName, nil, hook.Allowed())
		return
	}

	req.Config = loadConfig()
	req.MergeGranted = mergeGranted(req.SessionID)

	hook.Render(progName, req, rules.Apply(selected, req))
}

// applyGrant handles one UserPromptExpansion payload: it mints or revokes the
// session's merge grant and prints nothing, since stdout from this event is
// added to the model's context.
//
// A failure exits 1, which the harness reports without blocking the prompt.
// Exit 2 would stop the skill from running at all over a grant it can do
// without.
func applyGrant() {
	dir, err := grant.Dir()
	if err == nil {
		err = grant.Apply(os.Stdin, dir, time.Now())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: could not update the merge grant: %v.\nThe skill still runs, and "+
			"`gh pr merge` asks for approval as it does without a grant.\n", progName, err)
		os.Exit(1)
	}
}

// mergeGranted reports whether the session holds a live merge grant, reporting
// a store that cannot be read on stderr and carrying on with none. Like a
// broken config, that costs a prompt and never skips one.
func mergeGranted(session string) bool {
	granted := false
	dir, err := grant.Dir()
	if err == nil {
		granted, err = grant.Live(dir, session, time.Now())
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v.\n`gh pr merge` asks for approval as it does without a grant.\n",
			progName, err)
		return false
	}
	return granted
}

// loadConfig reads the machine-local configuration, reporting a broken one on
// stderr and carrying on with none.
//
// This does not block. The configuration only ever widens what runs unprompted,
// so running without it costs extra approvals and never fewer -- whereas
// exiting non-zero here would block every tool call in the session over a
// misplaced comma.
func loadConfig() config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v.\nThe guards are still running, with no configured "+
			"exemptions, so affected commands prompt as they did before.\n", progName, err)
	}
	return cfg
}

func selectRules(args []string) ([]rules.Rule, error) {
	switch len(args) {
	case 0:
		return rules.All(), nil
	case 1:
		r, ok := rules.ByName(args[0])
		if !ok {
			return nil, fmt.Errorf("unknown rule %q. Known rules: %s (run `%s list` for details)",
				args[0], strings.Join(names(), ", "), progName)
		}
		return []rules.Rule{r}, nil
	default:
		return nil, fmt.Errorf("expected at most one rule name, got %d arguments", len(args))
	}
}

func names() []string {
	all := rules.All()
	out := make([]string, len(all))
	for i, r := range all {
		out[i] = r.Name
	}
	return out
}

func list() {
	fmt.Printf("matcher: %s\n", rules.Matcher())
	fmt.Printf("droid matcher: %s\n\n", rules.DroidMatcher())
	for _, r := range rules.All() {
		fmt.Printf("  %-22s %s\n", r.Name, strings.Join(r.Tools, ", "))
	}
}
