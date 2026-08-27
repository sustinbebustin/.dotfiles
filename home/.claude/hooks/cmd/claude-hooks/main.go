// Command claude-hooks is the PreToolUse guard for Claude Code. It reads one
// hook payload on stdin, runs every registered rule that applies to the tool,
// and writes a single permission decision to stdout.
//
// Rules are registered in internal/rules; their verdicts are reduced to one by
// hook.Merge, worst wins.
//
// Usage:
//
//	claude-hooks           # run every applicable rule (the form settings.json invokes)
//	claude-hooks <rule>    # run one rule only, for debugging or a narrower hook
//	claude-hooks list      # print the rules, their tools, and the matcher
package main

import (
	"fmt"
	"os"
	"strings"

	"claude-hooks/internal/config"
	"claude-hooks/internal/hook"
	"claude-hooks/internal/rules"
)

const progName = "claude-hooks"

func main() {
	args := os.Args[1:]
	if len(args) == 1 && args[0] == "list" {
		list()
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
		hook.Render(progName, hook.Allowed())
		return
	}

	req.Config = loadConfig()

	hook.Render(progName, rules.Apply(selected, req))
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
	fmt.Printf("matcher: %s\n\n", rules.Matcher())
	for _, r := range rules.All() {
		fmt.Printf("  %-22s %s\n", r.Name, strings.Join(r.Tools, ", "))
	}
}
