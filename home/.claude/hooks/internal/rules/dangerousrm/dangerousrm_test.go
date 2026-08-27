package dangerousrm

import (
	"testing"

	"claude-hooks/internal/config"
	"claude-hooks/internal/hook"
)

func evaluate(cmd string) hook.Verdict {
	return Check(hook.NewRequest("Bash", "", "", cmd))
}

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
			name:     "absolute path still resolves to rm",
			cmd:      `/bin/rm -rf /Users/me/project/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "sudo does not hide the rm",
			cmd:      `sudo rm -rf /Users/me/project/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "backslash quoting does not hide the rm",
			cmd:      `\rm -rf /Users/me/project/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "literal tmp path is exempt",
			cmd:      `rm -rf /tmp/probe`,
			decision: "allow",
		},
		{
			name:     "sudo keeps the tmp exemption",
			cmd:      `sudo rm -rf /tmp/probe`,
			decision: "allow",
		},
		{
			name:     "an escaped target still reads as its path",
			cmd:      `rm -rf /tmp/my\ probe`,
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

		// Variable resolution.
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

		// Only bindings the shell reaches unconditionally, in the shell that
		// runs the rm, may exempt a target.
		{
			name:     "assignment in a subshell does not persist",
			cmd:      `R=/Users/me/repo; (R=/tmp/probe); rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment in a pipeline stage does not persist",
			cmd:      `R=/Users/me/repo; R=/tmp/probe | true; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment in an if body may not run",
			cmd:      `R=/Users/me/repo; if true; then R=/tmp/probe; fi; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment in a for body may not run",
			cmd:      `R=/Users/me/repo; for d in a; do R=/tmp/probe; done; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment on the right of && may not run",
			cmd:      `R=/Users/me/repo; false && R=/tmp/probe; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a command prefix binds only for that command",
			cmd:      `R=/Users/me/repo; R=/tmp/probe true; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "background assignment does not persist",
			cmd:      `R=/Users/me/repo; R=/tmp/probe & rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "assignment on the left of && does persist",
			cmd:      `R=/tmp/probe && rm -rf "$R"`,
			decision: "allow",
		},
		{
			name:     "assignment in a brace block does persist",
			cmd:      `{ R=/tmp/probe; }; rm -rf "$R"`,
			decision: "allow",
		},
		{
			name:     "an exported assignment does persist",
			cmd:      `export R=/tmp/probe; rm -rf "$R"`,
			decision: "allow",
		},
		{
			name:     "a declared assignment does persist",
			cmd:      `declare R=/tmp/probe; rm -rf "$R"`,
			decision: "allow",
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

// TestNonBashIsIgnored pins that a payload from another tool is not a shell
// command, even when it has a command field.
func TestNonBashIsIgnored(t *testing.T) {
	got := Check(hook.NewRequest("Read", "", "", "rm -rf /repo/src"))
	if got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}

// allowedRoot is the configured root the tests below run against. Package
// config has already validated the roots a rule sees, so these are given in the
// absolute, cleaned form it produces.
const allowedRoot = "/home/tester/dev/app"

func evaluateIn(cwd, cmd string) hook.Verdict {
	req := hook.NewRequest("Bash", "", "", cmd)
	req.Cwd = cwd
	req.Config = config.Config{AllowedRmRoots: []string{allowedRoot}}
	return Check(req)
}

// TestAllowedRoots covers the machine-local exemption: a recursive rm below a
// configured root runs unprompted, and everything the resolver cannot place
// below one still asks.
func TestAllowedRoots(t *testing.T) {
	cases := []struct {
		name     string
		cwd      string
		cmd      string
		decision string
	}{
		{
			name:     "absolute target below the root is exempt",
			cwd:      "/elsewhere",
			cmd:      `rm -rf /home/tester/dev/app/backend/tmp`,
			decision: "allow",
		},
		{
			name:     "relative target resolves against the cwd",
			cwd:      "/home/tester/dev/app/backend",
			cmd:      `rm -rf build/artifacts`,
			decision: "allow",
		},
		{
			name:     "target through a variable is exempt",
			cwd:      "/elsewhere",
			cmd:      "B=/home/tester/dev/app/backend\nrm -rf $B/docs",
			decision: "allow",
		},
		{
			name:     "every target must be below a root",
			cwd:      "/home/tester/dev/app",
			cmd:      `rm -rf backend/tmp /etc/hosts`,
			decision: "ask",
		},
		{
			name:     "the root itself still asks",
			cwd:      "/elsewhere",
			cmd:      `rm -rf /home/tester/dev/app`,
			decision: "ask",
		},
		{
			name:     "a sibling sharing the root's prefix is not below it",
			cwd:      "/elsewhere",
			cmd:      `rm -rf /home/tester/dev/app-old/src`,
			decision: "ask",
		},
		{
			name:     "climbing out of the root is not below it",
			cwd:      "/home/tester/dev/app",
			cmd:      `rm -rf backend/../../other/src`,
			decision: "ask",
		},
		{
			name:     "a relative target with no cwd cannot be placed",
			cwd:      "",
			cmd:      `rm -rf backend/tmp`,
			decision: "ask",
		},
		{
			name:     "a tilde target is not expanded",
			cwd:      "/elsewhere",
			cmd:      `rm -rf ~/dev/app/backend/tmp`,
			decision: "ask",
		},
		{
			name:     "an unresolvable target is never exempt",
			cwd:      "/home/tester/dev/app",
			cmd:      `rm -rf "$(cat target)"`,
			decision: "ask",
		},
		{
			name:     "a non-recursive rm below a root was never asked about",
			cwd:      "/home/tester/dev/app",
			cmd:      `rm backend/tmp`,
			decision: "allow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateIn(tc.cwd, tc.cmd)
			if name := got.Decision.String(); name != tc.decision {
				t.Fatalf("decision = %q, want %q (reason: %s)", name, tc.decision, got.Reason)
			}
		})
	}
}

// TestNoConfigKeepsAsking pins that the exemption is opt-in: with no configured
// roots, the same commands decide exactly as they did before config existed.
func TestNoConfigKeepsAsking(t *testing.T) {
	for _, cmd := range []string{
		`rm -rf /home/tester/dev/app/backend/tmp`,
		`rm -rf backend/tmp`,
	} {
		req := hook.NewRequest("Bash", "", "", cmd)
		req.Cwd = "/home/tester/dev/app"
		if got := Check(req); got.Decision != hook.Ask {
			t.Errorf("%s = %q with no config, want ask", cmd, got.Decision)
		}
	}
}
