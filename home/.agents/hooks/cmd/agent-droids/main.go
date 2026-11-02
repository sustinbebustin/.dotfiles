// Command agent-droids generates Factory Droid droids from the shared agents.
//
// Claude Code reads the agents in home/.agents/agents as they are. Droid wants
// its own frontmatter -- its tool IDs, BYOK model IDs, reasoningEffort -- in
// ~/.factory/droids, so this translates one into the other (see convert).
// `dot stow` runs it after building the hooks:
//
//	agent-droids <agents-dir> <droids-dir>
//
// Every file it writes carries generatedMarker, and it only ever overwrites or
// prunes files that do, so droids Droid installs itself, or ones written by
// hand, are never touched. Nothing is written unless every agent converts: one
// bad file fails the run with its path named.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const progName = "agent-droids"

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <agents-dir> <droids-dir>\n", progName)
		os.Exit(2)
	}
	report, err := generate(os.Args[1], os.Args[2])
	for _, line := range report {
		fmt.Println(line)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}
}

// generate converts every agent under agentsDir and syncs droidsDir to match,
// returning one report line per skip plus a summary.
func generate(agentsDir, droidsDir string) ([]string, error) {
	agentsDir, err := filepath.Abs(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("resolving the agents dir %s: %w", agentsDir, err)
	}
	results, err := convertAll(agentsDir)
	if err != nil {
		return nil, err
	}

	var report []string
	var droids []droid
	for _, r := range results {
		switch r := r.(type) {
		case droid:
			droids = append(droids, r)
		case skipped:
			report = append(report, fmt.Sprintf("skipped %s: %s", r.name, r.reason))
		}
	}

	written, err := write(droidsDir, droids)
	if err != nil {
		return report, err
	}
	pruned, err := prune(droidsDir, droids)
	if err != nil {
		return report, err
	}
	report = append(report, fmt.Sprintf("%d droid(s) in %s (%d updated, %d pruned)",
		len(droids), droidsDir, written, pruned))
	return report, nil
}

// write writes each droid that changed into dir, returning how many it wrote.
// Every target is checked before any is written, so a conflict leaves the dir
// as it was rather than half-synced.
func write(dir string, droids []droid) (int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("creating %s: %w", dir, err)
	}
	current := make(map[string]string, len(droids))
	for _, d := range droids {
		target := filepath.Join(dir, d.name+".md")
		existing, err := os.ReadFile(target)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", target, err)
		}
		if !isGenerated(string(existing)) {
			return 0, fmt.Errorf("refusing to overwrite %s: it was not generated from an agent. "+
				"Nothing was written; rename the agent %s or move that droid aside, then re-run", target, d.name)
		}
		current[d.name] = string(existing)
	}

	written := 0
	for _, d := range droids {
		if current[d.name] == d.content {
			continue
		}
		target := filepath.Join(dir, d.name+".md")
		if err := os.WriteFile(target, []byte(d.content), 0o644); err != nil {
			return written, fmt.Errorf("writing %s: %w", target, err)
		}
		written++
	}
	return written, nil
}

// convertAll converts every agent file under dir, in path order, failing on
// the first one that does not convert or on a name two agents share.
func convertAll(dir string) ([]result, error) {
	var results []result
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || filepath.Ext(path) != ".md" || strings.HasPrefix(e.Name(), "README") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		a, err := parseAgent(string(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		r, err := convert(a, path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if slices.ContainsFunc(results, func(o result) bool { return o.agentName() == r.agentName() }) {
			return fmt.Errorf("%s: agent name %q is already used by another agent", path, r.agentName())
		}
		results = append(results, r)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("converting the agents in %s: %w. No droids were written", dir, err)
	}
	return results, nil
}

// prune removes generated droids whose agent is gone or now skipped, leaving
// every file that does not carry the marker alone.
func prune(dir string, keep []droid) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("listing %s: %w", dir, err)
	}
	pruned := 0
	for _, e := range entries {
		name, ok := strings.CutSuffix(e.Name(), ".md")
		if !ok || e.IsDir() || slices.ContainsFunc(keep, func(d droid) bool { return d.name == name }) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return pruned, fmt.Errorf("reading %s: %w", path, err)
		}
		if !isGenerated(string(raw)) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return pruned, fmt.Errorf("removing stale droid %s: %w", path, err)
		}
		pruned++
	}
	return pruned, nil
}
