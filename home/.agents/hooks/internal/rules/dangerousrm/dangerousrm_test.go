package dangerousrm

import (
	"fmt"
	"testing"
	"time"

	"agent-hooks/internal/config"
	"agent-hooks/internal/hook"
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
			decision: "deny",
			reason:   reasonChainVar,
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
			name:     "an assignment inside an && chain reaches the rm later in that chain",
			cmd:      `cd /x && R=/tmp/probe && rm -rf "$R"`,
			decision: "allow",
		},
		{
			name:     "an assignment inside an && chain does not outlive it",
			cmd:      `true && R=/tmp/probe; rm -rf "$R"`,
			decision: "deny",
			reason:   reasonChainVar,
		},
		{
			name:     "a target safe only if an earlier chain ran is denied with a rewrite hint",
			cmd:      `cd /x && S=/tmp/s && make; rm -rf "$S/dist"`,
			decision: "deny",
			reason:   reasonChainVar,
		},
		{
			name:     "a target unsafe even if the chain ran still asks",
			cmd:      `cd /x && S=/Users/me/repo && make; rm -rf "$S/dist"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a target unsafe for another reason still asks",
			cmd:      `cd /x && S=/tmp/s && make; rm -rf "$S/dist" /Users/me/repo`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "an assignment before || does not reach the rm after it",
			cmd:      `true && R=/tmp/probe || rm -rf "$R"`,
			decision: "deny",
			reason:   reasonChainVar,
		},
		{
			name:     "an assignment inside an if body applies later in that body",
			cmd:      `if true; then R=/tmp/probe; rm -rf "$R"; fi`,
			decision: "allow",
		},
		{
			name:     "an assignment in the then branch does not reach the else branch",
			cmd:      `if true; then R=/tmp/probe; else rm -rf "$R"; fi`,
			decision: "ask",
			reason:   reasonGeneric,
		},

		// A binding that may or may not have been replaced is unknown, so a
		// scratch value bound earlier must not survive a conditional rebinding.
		{
			name:     "a conditional rebinding on the right of && hides an earlier tmp binding",
			cmd:      `R=/tmp/probe; true && R=/Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a rebinding in an if body hides an earlier tmp binding",
			cmd:      `R=/tmp/probe; if true; then R=/Users/me/repo; fi; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a rebinding in a loop body hides an earlier tmp binding after the loop",
			cmd:      `R=/tmp/probe; for d in a; do R=/Users/me/repo; done; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a rebinding later in a loop body reaches the rm on the next iteration",
			cmd:      `R=/tmp/probe; for d in a b; do rm -rf "$R"; R=/Users/me/repo; done`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a rebinding in the last pipeline stage persists under zsh",
			cmd:      `R=/tmp/probe; true | R=/Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "read rebinds its operands",
			cmd:      `R=/tmp/probe; read R; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "unset drops the binding",
			cmd:      `R=/tmp/probe; unset R; rm -rf "/tmp$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "eval can rebind anything",
			cmd:      `R=/tmp/probe; eval "$X"; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "source can rebind anything",
			cmd:      `R=/tmp/probe; . ./env.sh; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a nameref is not a literal binding",
			cmd:      `declare -n R=coverage; coverage=/Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a function can rebind a variable when it is called",
			cmd:      `f() { R=/Users/me/repo; }; R=/tmp/probe; f; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "an rm inside a function body cannot know the caller's bindings",
			cmd:      `R=/tmp/probe; f() { rm -rf "$R"; }; R=/Users/me/repo; f`,
			decision: "ask",
			reason:   reasonGeneric,
		},

		// Word splitting.
		{
			name:     "an unquoted value with spaces splits into several targets",
			cmd:      `R="/tmp/a /Users/me/repo"; rm -rf $R`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a quoted value with spaces is one target",
			cmd:      `R="/tmp/a /Users/me/repo"; rm -rf "$R"`,
			decision: "allow",
		},

		// for loops over literal lists.
		{
			name:     "a loop variable over tmp items is exempt",
			cmd:      `for d in /tmp/a /tmp/b; do rm -rf "$d/x"; done`,
			decision: "allow",
		},
		{
			name:     "every loop item must be exempt",
			cmd:      `for d in /tmp/a /Users/me/repo; do rm -rf "$d"; done`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "loop items resolve through earlier assignments",
			cmd:      `O=/tmp/s; for d in $O/a $O/b; do rm -rf $d/x; done`,
			decision: "allow",
		},
		{
			name:     "a body assignment built from the loop variable is exempt",
			cmd:      `O=/tmp/s; for c in a b; do D=$O/b-$c; rm -rf $D; mkdir -p $D; done`,
			decision: "allow",
		},
		{
			name:     "loop items split from an unquoted value fail closed",
			cmd:      `L="/tmp/a /Users/me/repo"; for d in $L; do rm -rf "$d"; done`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a loop over the positional parameters is unknown",
			cmd:      `for d; do rm -rf "/tmp/$d"; done`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "the loop variable is unknown after the loop",
			cmd:      `for d in /tmp/a; do :; done; rm -rf "$d"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "builtin export rebinds",
			cmd:      `R=/tmp/x; builtin export R=/Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "command declare rebinds",
			cmd:      `R=/tmp/x; command declare R=/Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "an assigning expansion rebinds",
			cmd:      `R=""; : ${R:=/Users/me/repo}; rm -rf /tmp/x $R`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "zsh print -v rebinds",
			cmd:      `R=/tmp/x; print -v R /Users/me/repo; rm -rf "$R"`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a changed IFS makes unquoted splitting unknown",
			cmd:      `IFS=/; R=/tmp/x; rm -rf $R`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "brace expansion can climb out of tmp",
			cmd:      `rm -rf /tmp/{a,../Users/me}`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a brace list of names under tmp is exempt",
			cmd:      `S=/tmp/s; rm -rf $S/{uibuild,uiout,ready.json}`,
			decision: "allow",
		},
		{
			name:     "several brace lists under tmp are exempt",
			cmd:      `rm -rf /tmp/{a,b}/{c,d}`,
			decision: "allow",
		},
		{
			name:     "a nested brace list fails closed",
			cmd:      `rm -rf /tmp/{a,{b,../c}}`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "an unbalanced brace fails closed",
			cmd:      `rm -rf /tmp/{a,b`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "brace expansion can climb out of an artifact directory",
			cmd:      `rm -rf node_modules/{x,../..}/src`,
			decision: "ask",
			reason:   reasonGeneric,
		},
		{
			name:     "a while body assignment applies later in that body",
			cmd:      `while read -r l; do R=/tmp/probe; rm -rf "$R"; done`,
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
			cmd:      "rm -rf \"$(git rev-parse --show-toplevel)\"",
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

		// A directory `mktemp -d` just made.
		{
			name:     "a fresh mktemp directory is exempt",
			cmd:      `d=$(mktemp -d); echo hi >"$d/log"; rm -rf "$d"`,
			decision: "allow",
		},
		{
			name:     "a quoted mktemp substitution is exempt",
			cmd:      `d="$(mktemp -d)"; rm -rf "$d"`,
			decision: "allow",
		},
		{
			name: "a server probe cleaning up its mktemp directory is exempt",
			cmd: `b=.build/us; d=$(mktemp -d); $b serve --data "$d" >"$d/log" 2>&1 & pid=$!; ` +
				`for i in $(seq 1 50); do curl -s http://127.0.0.1:45783/ && break; done; ` +
				`kill $pid; wait $pid 2>/dev/null; tail -3 "$d/log"; rm -rf "$d"`,
			decision: "allow",
		},
		{
			name:     "a path below a mktemp directory asks, since a failed mktemp leaves it at the root",
			cmd:      `d=$(mktemp -d); rm -rf "$d/x"`,
			decision: "ask",
		},
		{
			name:     "climbing out of a mktemp directory asks",
			cmd:      `d=$(mktemp -d); rm -rf "$d/.."`,
			decision: "ask",
		},
		{
			name:     "an unquoted mktemp directory may word-split and asks",
			cmd:      `d=$(mktemp -d); rm -rf $d`,
			decision: "ask",
		},
		{
			name:     "a mktemp variable rebound before the rm asks",
			cmd:      `d=$(mktemp -d); d=/home/me; rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "mktemp with other arguments asks",
			cmd:      `d=$(mktemp -d /home/me/x); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a mktemp dry run asks",
			cmd:      `d=$(mktemp -u); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "mktemp with a redirect asks",
			cmd:      `d=$(mktemp -d 2>/dev/null); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a mktemp value with a suffix asks",
			cmd:      `d=$(mktemp -d)/sub; rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a cd into a mktemp directory leaves relative targets unknown",
			cmd:      `d=$(mktemp -d); cd "$d" && rm -rf x`,
			decision: "ask",
		},

		// A mktemp the command may have replaced.
		{
			name:     "a mktemp function makes the substitution unknown",
			cmd:      `mktemp() { echo /home/me; }; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a mktemp function declared with the keyword makes the substitution unknown",
			cmd:      `function mktemp { echo /home/me; }; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a PATH change makes the substitution unknown",
			cmd:      `PATH=/home/me/bin:$PATH; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "an exported PATH makes the substitution unknown",
			cmd:      `export PATH=/home/me/bin; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a zsh path array change makes the substitution unknown",
			cmd:      `path=(/home/me/bin $path); d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a zsh functions entry makes the substitution unknown",
			cmd:      `functions[mktemp]='echo /home/me'; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a hashed command path makes the substitution unknown",
			cmd:      `hash -p /bin/echo mktemp; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "an alias makes the substitution unknown",
			cmd:      "alias mktemp='echo /home/me'\nd=$(mktemp -d); rm -rf \"$d\"",
			decision: "ask",
		},
		{
			name:     "an eval makes the substitution unknown",
			cmd:      `eval "$X"; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a nameref write makes the variable it names unknown",
			cmd:      `d=/tmp/x; declare -n r=d; r=/home/me; rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "a let assignment makes the variable unknown",
			cmd:      `d=/tmp/x; let d=0; rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "an arithmetic assignment makes the variable unknown",
			cmd:      `d=/tmp/x; (( d = 1 )); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "an arithmetic increment makes the variable unknown",
			cmd:      `d=/tmp/x; (( d++ )); rm -rf "$d"`,
			decision: "ask",
		},
		{
			name:     "arithmetic on another variable leaves the target known",
			cmd:      `d=/tmp/x; for ((i=0; i<3; i++)); do :; done; rm -rf "$d"`,
			decision: "allow",
		},
		{
			name:     "a nameref makes the substitution unknown",
			cmd:      `declare -n p=PATH; p=/home/me/bin; d=$(mktemp -d); rm -rf "$d"`,
			decision: "ask",
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

// TestDoublingVariablesStayCheap pins that a binding is resolved once, however
// often it is looked up: a chain of variables that each double the one before
// would otherwise cost 2^n lookups and stall the hook.
func TestDoublingVariablesStayCheap(t *testing.T) {
	cmd := "V0=''"
	for i := 1; i <= 40; i++ {
		cmd += fmt.Sprintf("; V%d=$V%d$V%d", i, i-1, i-1)
	}
	cmd += `; rm -rf "/tmp/x$V40"`
	done := make(chan hook.Verdict, 1)
	go func() { done <- evaluate(cmd) }()
	select {
	case got := <-done:
		if got.Decision != hook.Allow {
			t.Fatalf("decision = %q, want allow", got.Decision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("resolving 40 doubling variables took over 5s")
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
		{
			name:     "brace expansion is not modelled",
			cwd:      "/elsewhere",
			cmd:      `rm -rf /home/tester/dev/app/backend/{tmp,..}`,
			decision: "ask",
		},

		// A cd moves where a relative target lands.
		{
			name:     "a cd out of the root moves relative targets with it",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd /home/tester && rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "a cd in an earlier statement still moves relative targets",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd /home/tester; rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "a cd into the root places relative targets there",
			cwd:      "/elsewhere",
			cmd:      `cd /home/tester/dev/app/backend && rm -rf build`,
			decision: "allow",
		},
		{
			name:     "a dot-relative cd is followed",
			cwd:      "/home/tester/dev/app/backend",
			cmd:      `cd .. && rm -rf frontend/x`,
			decision: "allow",
		},
		{
			name:     "a dot-relative cd can leave the root",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd .. && rm -rf other/x`,
			decision: "ask",
		},
		{
			name:     "a bare relative cd may follow CDPATH and is unknown",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd src && rm -rf x`,
			decision: "ask",
		},
		{
			name:     "a cd through a variable is followed",
			cwd:      "/elsewhere",
			cmd:      `W=/home/tester/dev/app/wt; cd "$W" && rm -rf x`,
			decision: "allow",
		},
		{
			name:     "a cd in a subshell does not move the rm after it",
			cwd:      "/elsewhere",
			cmd:      `(cd /home/tester/dev/app); rm -rf x`,
			decision: "ask",
		},
		{
			name:     "a conditional cd leaves the directory unknown",
			cwd:      "/home/tester/dev/app",
			cmd:      `test -d x && cd /home/tester; rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "a function that cds leaves the directory unknown",
			cwd:      "/home/tester/dev/app",
			cmd:      `f() { cd /home/tester; }; f; rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "a computed cd leaves the directory unknown",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd "$(mktemp -d)" && rm -rf x`,
			decision: "ask",
		},
		{
			name:     "a bare cd goes home",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd && rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "env -C changes the directory too",
			cwd:      "/home/tester/dev/app",
			cmd:      `env -C /home/tester rm -rf dev`,
			decision: "ask",
		},
		{
			name:     "a cd that may fail leaves the start directory possible",
			cwd:      "/home/tester",
			cmd:      `cd /home/tester/dev/app/missing; rm -rf build`,
			decision: "ask",
		},
		{
			name:     "a cd heading a chain may have failed past the chain",
			cwd:      "/home/tester",
			cmd:      `cd /home/tester/dev/app && make; rm -rf build`,
			decision: "deny",
		},
		{
			name:     "a cd in an if condition may have failed",
			cwd:      "/home/tester",
			cmd:      `if cd /home/tester/dev/app; then :; fi; rm -rf build`,
			decision: "ask",
		},
		{
			name:     "a cd that may fail is fine when both directories are below the root",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd /home/tester/dev/app/wt; rm -rf build`,
			decision: "allow",
		},
		{
			name:     "a brace in a cd target is unknown",
			cwd:      "/home/tester/dev/app",
			cmd:      `cd /home/tester/dev/app/{x,y} && rm -rf build`,
			decision: "ask",
		},
		{
			name:     "an attached env -C value moves the rm",
			cwd:      "/home/tester/dev/app",
			cmd:      `env -C/ rm -rf home`,
			decision: "ask",
		},
		{
			name:     "an attached sudo -D value moves the rm",
			cwd:      "/home/tester/dev/app",
			cmd:      `sudo -D/ rm -rf home`,
			decision: "ask",
		},
		{
			name:     "sudo --chdir does not hide the rm",
			cwd:      "/home/tester/dev/app",
			cmd:      `sudo --chdir / rm -rf home`,
			decision: "ask",
		},
		{
			name:     "a cd then a loop over relative items below the root is exempt",
			cwd:      "/elsewhere",
			cmd:      `cd /home/tester/dev/app/wt && for d in a/b c/d; do test -d $d && rm -r $d/testdata; done`,
			decision: "allow",
		},

		// A cd into /tmp places relative targets there.
		{
			name:     "relative targets after a cd into a tmp scratchpad are exempt",
			cwd:      "/home/tester/other",
			cmd:      `S=/tmp/claude/scratchpad/binsize; cd $S && (diff -ru a b > k.patch; true) && wc -l k.patch && rm -rf proto proto.full && ls`,
			decision: "allow",
		},
		{
			name:     "a relative target climbing out of tmp still asks",
			cwd:      "/home/tester/other",
			cmd:      `cd /tmp/x && rm -rf ../../home/tester`,
			decision: "ask",
		},
		{
			name:     "a cd into tmp that may have failed leaves the start directory possible",
			cwd:      "/home/tester/other",
			cmd:      `cd /tmp/x; rm -rf proto`,
			decision: "ask",
		},
		{
			name:     "a relative target from a tmp cwd is exempt",
			cwd:      "/tmp/claude/scratchpad",
			cmd:      `rm -rf proto`,
			decision: "allow",
		},

		// A `..` resolves against the physical directory, which a symlink under
		// /tmp can put anywhere.
		{
			name:     "a relative target climbing through a tmp symlink asks",
			cwd:      "/tmp/x",
			cmd:      `rm -rf link/../proj`,
			decision: "ask",
		},
		{
			name:     "a relative target climbing from a tmp cwd asks",
			cwd:      "/tmp/link",
			cmd:      `rm -rf ../victim`,
			decision: "ask",
		},
		{
			name:     "a relative target climbing after a cd into tmp asks",
			cwd:      "/home/tester/other",
			cmd:      `cd /tmp/link && rm -rf ../proj`,
			decision: "ask",
		},
		{
			name:     "a physical cd climbing through tmp asks",
			cwd:      "/home/tester/other",
			cmd:      `cd -P /tmp/link/.. && rm -rf proj`,
			decision: "ask",
		},
		{
			name:     "a dot-relative physical cd from tmp asks",
			cwd:      "/home/tester/other",
			cmd:      `cd /tmp/link && cd -P .. && rm -rf proj`,
			decision: "ask",
		},

		// A cd the command may have replaced.
		{
			name:     "a cd function makes the directory unknown",
			cwd:      "/home/tester/other",
			cmd:      `cd() { :; }; cd /tmp/x && rm -rf proj`,
			decision: "ask",
		},
		{
			name:     "an alias makes the directory unknown",
			cwd:      "/home/tester/other",
			cmd:      "alias g='cd /home/tester'\ncd /tmp/x && g && rm -rf proj",
			decision: "ask",
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
