package awscli

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/shellast"
)

// envResettingWrappers start the command with a fresh or filtered environment,
// and a different home directory, in ways their flags do not spell out.
var envResettingWrappers = map[string]bool{"sudo": true, "doas": true}

// wrapperEnv is what the wrappers in front of a call (env, exec, sudo ...) do
// to the environment it inherits.
type wrapperEnv struct {
	// unknown says why a wrapper's effect cannot be followed; "" when every
	// wrapper was read.
	unknown string
	// clears names the wrapper option that empties the inherited environment
	// (env -i, exec -c); "" when none does.
	clears string
	// unset are the variables env -u removes.
	unset map[string]bool
}

// drops reports whether the call may not inherit variable name.
func (e wrapperEnv) drops(name string) bool {
	return e.unknown != "" || e.clears != "" || e.unset[name]
}

// readWrappers reads the wrapper words that run a call. Anything it does not
// model -- env -S, an unknown option, a non-literal name -- sets unknown, so a
// caller relying on what the call inherits refuses to.
func readWrappers(wrappers []*syntax.Word) wrapperEnv {
	e := wrapperEnv{unset: map[string]bool{}}
	wrapper := ""
	for i := 0; i < len(wrappers); i++ {
		text := shellast.WordLit(wrappers[i])
		switch {
		case strings.HasPrefix(text, "-") && !shellast.IsLiteral(wrappers[i]):
			// WordLit keeps only the literal parts: `"--$X"` renders as `--`
			// though $X may spell unset=AWS_PROFILE.
			e.unknown = fmt.Sprintf("the wrapper option %q is built from a variable, so its effect cannot be followed", text)
		case text == "--":
		case text == "-" && wrapper == "env":
			e.clears = "env -"
		case len(text) > 1 && strings.HasPrefix(text, "-") && (wrapper == "env" || wrapper == "exec"):
			i = e.readOption(wrapper, wrappers, i)
		case strings.HasPrefix(text, "-"), strings.Contains(text, "="):
			// Another wrapper's flag (command -p), or an env assignment, which
			// the census reads.
		default:
			wrapper = shellast.CommandName(text)
			if envResettingWrappers[wrapper] {
				e.unknown = fmt.Sprintf("%s may reset the environment and home directory the profile comes from", text)
			}
		}
		if e.unknown != "" {
			return e
		}
	}
	return e
}

// readOption applies the env or exec option at words[i] and returns the index
// of the last word it consumed.
func (e *wrapperEnv) readOption(wrapper string, words []*syntax.Word, i int) int {
	w := words[i]
	text := shellast.WordLit(w)
	cannotFollow := func() int {
		e.unknown = fmt.Sprintf("%s %s may change the environment in a way the hook cannot follow", wrapper, text)
		return i
	}
	if !shellast.IsLiteral(w) {
		return cannotFollow()
	}
	// value returns the option's value: attached if non-empty, else the next
	// word, consumed.
	value := func(attached string) (string, bool) {
		if attached != "" {
			return attached, true
		}
		if i+1 >= len(words) || !shellast.IsLiteral(words[i+1]) {
			return "", false
		}
		i++
		return shellast.WordLit(words[i]), true
	}

	if strings.HasPrefix(text, "--") {
		name, attached, _ := strings.Cut(text, "=")
		switch {
		case wrapper != "env":
			return cannotFollow()
		case name == "--ignore-environment":
			e.clears = wrapper + " " + text
		case name == "--debug", name == "--null":
		case name == "--unset", name == "--chdir":
			v, ok := value(attached)
			if !ok {
				return cannotFollow()
			}
			if name == "--unset" {
				e.unset[v] = true
			}
		default:
			// --split-string, and anything else.
			return cannotFollow()
		}
		return i
	}

	for j := 1; j < len(text); j++ {
		c := text[j]
		switch {
		case wrapper == "env" && c == 'i', wrapper == "exec" && c == 'c':
			e.clears = wrapper + " " + text
		case wrapper == "env" && (c == 'v' || c == '0'), wrapper == "exec" && c == 'l':
		case wrapper == "env" && (c == 'u' || c == 'C'), wrapper == "exec" && c == 'a':
			v, ok := value(text[j+1:])
			if !ok {
				return cannotFollow()
			}
			if c == 'u' {
				e.unset[v] = true
			}
			return i
		default:
			// env -S, and anything else.
			return cannotFollow()
		}
	}
	return i
}
