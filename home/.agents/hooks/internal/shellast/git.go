package shellast

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// gitValueFlags are the git top-level flags that take the following word as
// their value. They are stepped over when looking for the subcommand.
var gitValueFlags = map[string]bool{
	"-C":           true,
	"-c":           true,
	"--git-dir":    true,
	"--work-tree":  true,
	"--namespace":  true,
	"--exec-path":  true,
	"--config-env": true,
}

// GitSubcommand returns the subcommand in args, the words after `git`, plus
// the words that follow it. name is empty when there is none.
func GitSubcommand(args []string) (name string, rest []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			return a, args[i+1:]
		case strings.Contains(a, "="):
			// A flag with its value attached (--git-dir=x).
		case gitValueFlags[a]:
			i++
		}
	}
	return "", nil
}

// Rebase is what a `git rebase` invocation asks for, as far as the guards
// care.
type Rebase struct {
	// Execs are the scripts --exec / -x hands to the shell after each commit.
	Execs []string
	// Interactive is --interactive / -i: the plan goes to an editor.
	Interactive bool
}

// rebaseValueShorthands are the `git rebase` shorthands that take a value:
// the rest of the bundle when there is any, the next word otherwise. -S takes
// only an attached value, so it is in the first group alone.
const (
	rebaseValueShorthands    = "sXC"
	rebaseAttachedShorthands = "S"
)

// ParseRebase reads the words after `rebase`. Both the guard on rebase flags
// and the extraction of exec scripts use it, so they cannot disagree about
// which word is a script.
func ParseRebase(rest []string) Rebase {
	var r Rebase
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		switch {
		case a == "--":
			return r
		case a == "--exec":
			if i+1 < len(rest) {
				r.Execs = append(r.Execs, rest[i+1])
				i++
			}
		case strings.HasPrefix(a, "--exec="):
			r.Execs = append(r.Execs, strings.TrimPrefix(a, "--exec="))
		case a == "--interactive":
			r.Interactive = true
		case strings.HasPrefix(a, "--"):
		case strings.HasPrefix(a, "-") && len(a) > 1:
			i += r.readShorthands(a[1:], rest[i+1:])
		}
	}
	return r
}

// readShorthands reads one bundle of shorthand flags and returns how many of
// the following words it consumed as a value.
func (r *Rebase) readShorthands(bundle string, next []string) int {
	for j, flag := range bundle {
		value := bundle[j+1:]
		switch {
		case flag == 'i':
			r.Interactive = true
		case flag == 'x':
			if value != "" {
				r.Execs = append(r.Execs, value)
				return 0
			}
			if len(next) > 0 {
				r.Execs = append(r.Execs, next[0])
				return 1
			}
			return 0
		case strings.ContainsRune(rebaseValueShorthands, flag):
			if value == "" && len(next) > 0 {
				return 1
			}
			return 0
		case strings.ContainsRune(rebaseAttachedShorthands, flag):
			return 0
		}
	}
	return 0
}

// rebaseExecScripts returns the exec scripts c hands to the shell when it is a
// `git rebase` whose arguments are all literal. Any expansion among them
// yields nothing: there is no script text to check, and the rebase guard asks
// for such a rebase anyway.
func rebaseExecScripts(c *syntax.CallExpr) []string {
	name, operands := Invocation(c.Args, WordLit)
	if CommandName(name) != "git" {
		return nil
	}
	args := make([]string, len(operands))
	for i, w := range operands {
		args[i] = WordLit(w)
	}
	sub, rest := GitSubcommand(args)
	if sub != "rebase" {
		return nil
	}
	// The scripts are re-parsed as shell, so they render the way the outer
	// shell hands them over (see scriptText); flags read the same either way.
	words := operands[len(operands)-len(rest):]
	texts := make([]string, len(words))
	for i, w := range words {
		if !IsLiteral(w) {
			return nil
		}
		texts[i] = scriptText(w)
	}
	return ParseRebase(texts).Execs
}
