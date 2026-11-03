package awscli

import (
	"errors"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
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

// profileSourceVars are the variables whose removal changes which profile a
// call runs under, or which ~/.aws defines it.
var profileSourceVars = []string{profileVar, defaultProfileVar, "HOME"}

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
// call's aws arguments and wrappers what the words that run it (sudo, env ...)
// do to its environment.
func (pe profileEnv) resolve(call *syntax.CallExpr, wrappers wrapperEnv, toks []token) (string, error) {
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
			return "", false, unknownProfile("the %s could expand to a --profile option",
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

// checkWrappers refuses wrappers that may hand aws a different profile
// environment from the one the hook sees. Unsetting any other variable
// (`env -u AWS_ENDPOINT_URL`) leaves the profile as it was.
func checkWrappers(e wrapperEnv) error {
	switch {
	case e.unknown != "":
		return unknownProfile("%s", e.unknown)
	case e.clears != "":
		return unknownProfile("%s clears the environment AWS_PROFILE comes from", e.clears)
	}
	for _, name := range profileSourceVars {
		if e.unset[name] {
			return unknownProfile("env -u %s changes where the profile comes from", name)
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
