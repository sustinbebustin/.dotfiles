package main

import "testing"

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
		reason   string
	}{
		{
			name:     "non-recursive rm is allowed",
			cmd:      `rm /etc/hosts`,
			decision: "allow",
		},
		{
			name:     "recursive rm outside tmp asks",
			cmd:      `rm -rf /Users/me/project/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "literal tmp path is exempt",
			cmd:      `rm -rf /tmp/probe`,
			decision: "allow",
		},
		{
			name:     "private tmp path is exempt",
			cmd:      `rm -rf /private/tmp/claude-501/session/scratchpad/x`,
			decision: "allow",
		},
		{
			name:     "dotdot escape from tmp still asks",
			cmd:      `rm -rf /tmp/../Users/me`,
			decision: "ask",
			reason:   reasonGeneric,
		},

		// Variable resolution: the behaviour this change adds.
		{
			name:     "variable pointing into tmp is exempt",
			cmd:      "R=/tmp/probe\nrm -rf \"$R\"",
			decision: "allow",
		},
		{
			name:     "variable with suffix into tmp is exempt",
			cmd:      "R=/private/tmp/scratch\nrm -rf \"$R/src\"",
			decision: "allow",
		},
		{
			name:     "braced variable into tmp is exempt",
			cmd:      "R=/tmp/probe\nrm -rf \"${R}/a\"",
			decision: "allow",
		},
		{
			name:     "variable pointing into a repo asks",
			cmd:      "R=/Users/me/repo\nrm -rf \"$R/src\"",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "reassignment uses the latest binding before the rm",
			cmd:      "R=/tmp/a\nR=/Users/me/repo\nrm -rf \"$R\"",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment after the rm does not apply",
			cmd:      "rm -rf \"$R\"\nR=/tmp/a",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "unknown variable fails closed",
			cmd:      `rm -rf "$UNSET/src"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "command substitution fails closed",
			cmd:      "rm -rf \"$(mktemp -d)\"",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "default-value expansion fails closed",
			cmd:      `rm -rf "${R:-/tmp/x}"`,
			decision: "ask",
			reason:   reasonGeneric,
		},

		// Scratchpad hint: same command creates paths beneath the targets.
		{
			name: "probe then cleanup outside tmp gets the scratchpad hint",
			cmd: "R=/Users/me/repo\n" +
				`mkdir -p "$R/src/Pricing/Domain" "$R/src/Deployment/Domain"` + "\n" +
				`cat > "$R/src/Pricing/Domain/Probe.php" <<'PHP'` + "\nx\nPHP\n" +
				`rm -rf "$R/src/Pricing" "$R/src/Deployment"`,
			decision: "ask",
			reason:   reasonSameCommand,
		},
		{
			name:     "redirect-created file gets the scratchpad hint",
			cmd:      "echo hi > /Users/me/repo/probe/a.txt\nrm -rf /Users/me/repo/probe",
			decision: "ask",
			reason:   reasonSameCommand,
		},
		{
			name:     "removal unrelated to created paths stays generic",
			cmd:      "mkdir -p /Users/me/repo/probe\nrm -rf /Users/me/repo/src",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "only one of two targets created keeps it generic",
			cmd:      "mkdir -p /Users/me/repo/probe/x\nrm -rf /Users/me/repo/probe /Users/me/repo/src",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "creation after the rm does not earn the hint",
			cmd:      "rm -rf /Users/me/repo/probe\nmkdir -p /Users/me/repo/probe/x",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "cp source is not treated as created",
			cmd:      "cp /Users/me/repo/src/a.txt /Users/me/other/b.txt\nrm -rf /Users/me/repo/src",
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "input redirect is not a creation",
			cmd:      "wc -l < /Users/me/repo/probe/a.txt\nrm -rf /Users/me/repo/probe",
			decision: "ask",
			reason:   reasonGeneric,
		},

		// Nesting: rm must still be found inside compound structures.
		{
			name:     "rm inside a subshell is caught",
			cmd:      `(cd /Users/me/repo && rm -rf src)`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "rm behind && is caught",
			cmd:      `true && rm -rf /Users/me/repo/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "long recursive flag is caught",
			cmd:      `rm --recursive --force /Users/me/repo/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "target after -- is a path not a flag",
			cmd:      `rm -rf -- /tmp/probe`,
			decision: "allow",
		},

		// Artifact directories.
		{
			name:     "symfony cache subdirectory is exempt",
			cmd:      `rm -rf var/cache/test`,
			decision: "allow",
		},
		{
			name:     "artifact directory itself is exempt",
			cmd:      `rm -rf node_modules`,
			decision: "allow",
		},
		{
			name:     "artifact directory nested in a monorepo is exempt",
			cmd:      `rm -rf apps/web/.next`,
			decision: "allow",
		},
		{
			name:     "leading dot-slash and glob are exempt",
			cmd:      `rm -rf ./var/cache/*`,
			decision: "allow",
		},
		{
			name:     "several artifact targets are exempt together",
			cmd:      `rm -rf .pytest_cache __pycache__ coverage`,
			decision: "allow",
		},
		{
			name:     "variable pointing at an artifact directory is exempt",
			cmd:      `R=var/cache; rm -rf "$R/test"`,
			decision: "allow",
		},
		{
			name:     "absolute system var cache still asks",
			cmd:      `rm -rf /var/cache`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "home-relative artifact path still asks",
			cmd:      `rm -rf ~/var/cache`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "dotdot escape from an artifact directory still asks",
			cmd:      `rm -rf node_modules/../src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "var without cache still asks",
			cmd:      `rm -rf var`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "generic output names still ask",
			cmd:      `rm -rf dist build out target vendor`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "one non-artifact target among artifacts still asks",
			cmd:      `rm -rf node_modules src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "artifact-suffixed name is not an artifact directory",
			cmd:      `rm -rf node_modules_backup`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "cd into a repo then clear its cache is exempt",
			cmd:      `(cd /Users/me/project && rm -rf var/cache/test && vendor/bin/phpunit)`,
			decision: "allow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluate(tc.cmd)
			if name := got.Decision.String(); name != tc.decision {
				t.Fatalf("decision = %q, want %q (reason: %s)", name, tc.decision, got.Reason)
			}
			if tc.reason != "" && got.Reason != tc.reason {
				t.Fatalf("reason = %q, want %q", got.Reason, tc.reason)
			}
		})
	}
}
