package awscli

import (
	"errors"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/shellast"
)

const (
	profileVar        = "AWS_PROFILE"
	defaultProfileVar = "AWS_DEFAULT_PROFILE"
	profileFlag       = "--profile"
	defaultProfile    = "default"
)

// redirectingVars change what a profile name means: which file defines it.
var redirectingVars = []string{"AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE"}

// credentialVars supply credentials directly. The CLI uses them in preference to
// AWS_PROFILE, though not to an explicit --profile.
var credentialVars = []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN"}

// envResettingWrappers start the command with a fresh or filtered environment,
// so an inherited AWS_PROFILE may not reach it.
var envResettingWrappers = map[string]bool{"sudo": true, "doas": true}

// envResettingFlags are env's options that drop variables from the command's
// environment.
var envResettingFlags = []string{"-i", "--ignore-environment", "-u", "--unset", "-S", "--split-string"}

// profileEnv is what the command and the hook's own environment say about the
// profile, before looking at any one call.
type profileEnv struct {
	census census
	// inherited is AWS_PROFILE in the hook's environment, and hasInherited
	// whether it is set at all.
	inherited    string
	hasInherited bool
	// hookVars are the profile-relevant variables set in the hook's environment.
	hookVars map[string]bool
}

func newProfileEnv(c census, environ []string) profileEnv {
	pe := profileEnv{census: c, hookVars: map[string]bool{}}
	for _, kv := range environ {
		k, v, _ := strings.Cut(kv, "=")
		if k == profileVar && v != "" {
			pe.inherited, pe.hasInherited = v, true
		}
		if v != "" {
			pe.hookVars[k] = true
		}
	}
	return pe
}

// errProfileUnknown wraps every reason a call's profile cannot be determined.
var errProfileUnknown = errors.New("the profile it uses cannot be determined")

func unknownProfile(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{errProfileUnknown}, args...)...)
}

// resolve returns the profile call runs under, following the CLI's precedence:
// --profile, then AWS_PROFILE as the call sees it, then "default". toks are the
// call's aws arguments and wrappers the words that run it (sudo, env ...).
func (pe profileEnv) resolve(call *syntax.CallExpr, wrappers []*syntax.Word, toks []token) (string, error) {
	for _, name := range redirectingVars {
		if pe.census.bound(name) || pe.hookVars[name] {
			return "", unknownProfile("%s is set, so profiles may come from another file", name)
		}
	}
	if pe.census.bound(defaultProfileVar) || pe.hookVars[defaultProfileVar] {
		return "", unknownProfile("%s is set", defaultProfileVar)
	}

	// sudo also changes HOME, and with it which ~/.aws defines the profile, so
	// this comes before --profile.
	if err := checkWrappers(wrappers); err != nil {
		return "", err
	}
	if name, found, err := flagProfile(toks); err != nil || found {
		return name, err
	}
	for _, name := range credentialVars {
		if pe.credentialsReach(name, call) {
			return "", unknownProfile("%s is set, and the CLI prefers it to AWS_PROFILE", name)
		}
	}
	return pe.envProfile(call)
}

// flagProfile finds --profile in the call. The last one wins, as in the CLI. An
// unquoted expansion could add one, so it makes the profile unknown.
func flagProfile(toks []token) (name string, found bool, err error) {
	for i, t := range toks {
		if t.mayBeFlags {
			return "", false, unknownProfile("the unquoted %s could expand to a --profile option",
				describeWord(t.text))
		}
		if !t.known {
			continue
		}
		switch {
		case t.text == profileFlag:
			if i+1 >= len(toks) || !toks[i+1].known {
				return "", false, unknownProfile("the --profile value is not a literal")
			}
			name, found = toks[i+1].text, true
		case strings.HasPrefix(t.text, profileFlag+"="):
			name, found = strings.TrimPrefix(t.text, profileFlag+"="), true
		}
	}
	return name, found, nil
}

func describeWord(text string) string {
	if text == "" {
		return "variable"
	}
	return fmt.Sprintf("word %q", text)
}

// checkWrappers refuses wrappers that may hand aws a different environment
// from the one the hook sees.
func checkWrappers(wrappers []*syntax.Word) error {
	for _, w := range wrappers {
		text := shellast.WordLit(w)
		if envResettingWrappers[shellast.CommandName(text)] {
			return unknownProfile("%s may reset the environment and home directory the profile comes from", text)
		}
		for _, f := range envResettingFlags {
			if text == f || (strings.HasPrefix(f, "--") && strings.HasPrefix(text, f+"=")) {
				return unknownProfile("env %s may drop AWS_PROFILE", text)
			}
		}
	}
	return nil
}

// credentialsReach reports whether credential variable name may be in call's
// environment: set in the hook's environment, or bound anywhere in the command
// except on the prefix of some other call.
func (pe profileEnv) credentialsReach(name string, call *syntax.CallExpr) bool {
	if pe.hookVars[name] || pe.census.opaque {
		return true
	}
	for _, s := range pe.census.sites[name] {
		if s.kind != bindCall || s.call == call {
			return true
		}
	}
	return false
}

// envProfile resolves AWS_PROFILE as call sees it: its own prefix or env
// wrapper, else the last top-level export before it, else the hook's
// environment. Any other way of setting it -- a bare assignment, one inside a
// subshell or conditional, unset, read, a value from a variable -- makes it
// unknown.
func (pe profileEnv) envProfile(call *syntax.CallExpr) (string, error) {
	if pe.census.opaque {
		return "", unknownProfile("eval or source could set AWS_PROFILE")
	}
	var own, exported *site
	for i := range pe.census.sites[profileVar] {
		s := &pe.census.sites[profileVar][i]
		switch {
		case s.kind == bindCall && s.call != call:
			continue
		case s.kind == bindCall:
			own = s
		case s.kind == bindExport:
			if call.Pos().After(s.end) {
				exported = s
			}
		default:
			return "", unknownProfile("AWS_PROFILE is set in a way the hook cannot follow (only a literal " +
				"`export AWS_PROFILE=name` or a prefix on the aws call counts)")
		}
		if !s.literal {
			return "", unknownProfile("AWS_PROFILE is set from a variable or substitution")
		}
	}
	switch {
	case own != nil:
		return nonEmptyProfile(own.value), nil
	case exported != nil:
		return nonEmptyProfile(exported.value), nil
	case pe.hasInherited:
		return pe.inherited, nil
	}
	return defaultProfile, nil
}

// nonEmptyProfile maps an empty AWS_PROFILE, which the CLI ignores, to the
// default profile.
func nonEmptyProfile(name string) string {
	if name == "" {
		return defaultProfile
	}
	return name
}
