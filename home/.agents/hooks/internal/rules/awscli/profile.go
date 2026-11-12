package awscli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/shellast"
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
	// repeated reports a position in a loop or function body, where a call can
	// run after statements written below it.
	repeated func(syntax.Pos) bool
}

func newProfileEnv(file *syntax.File, c census, environ []string) profileEnv {
	pe := profileEnv{census: c, hookVars: map[string]bool{}, repeated: shellast.RepeatedSpans(file)}
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
	if pe.census.Opaque {
		return "", unknownProfile("the command runs eval, source or another builtin that can set any variable, " +
			"so AWS_PROFILE and the files profiles are read from may have changed")
	}
	for _, name := range redirectingVars {
		if pe.census.Bound(name) || pe.hookVars[name] {
			return "", unknownProfile("%s is set, so profiles may come from another file", name)
		}
	}
	if pe.census.Bound(defaultProfileVar) || pe.hookVars[defaultProfileVar] {
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

// guardedFlags are the options that decide which account a call reaches.
var guardedFlags = []string{profileFlag, endpointFlag}

// splitFlag reads one argument as an option: its name, and the value attached
// with `=` if there is one. The CLI's argparse takes any unambiguous
// abbreviation of a global option, and no other global option starts with --p
// or --e, so every such prefix of a guarded flag is returned as that flag:
// `--prof x` and `--e=http://...` set the profile and the endpoint.
func splitFlag(text string) (flag, value string, attached bool) {
	flag, value, attached = strings.Cut(text, "=")
	if len(flag) > len("--") {
		for _, full := range guardedFlags {
			if strings.HasPrefix(full, flag) {
				return full, value, attached
			}
		}
	}
	return flag, value, attached
}

// setsGuardedFlag reports whether an argument starting with the literal text
// prefix is a guarded flag with its value attached, so whatever follows prefix
// in the word is that value.
func setsGuardedFlag(prefix string) bool {
	flag, _, attached := splitFlag(prefix)
	return attached && slices.Contains(guardedFlags, flag)
}

// mayBecomeGuardedFlag reports whether an argument starting with the literal
// text prefix and continuing with text the hook cannot know could be a guarded
// flag: prefix already is one awaiting its value (`--profile="$P"`), or it is
// the start of one's name, which the unknown text may complete (`--prof"$X"`,
// `"$X"` alone).
func mayBecomeGuardedFlag(prefix string) bool {
	if strings.Contains(prefix, "=") {
		return setsGuardedFlag(prefix)
	}
	if strings.Trim(prefix, "-") == "" {
		return true
	}
	for _, full := range guardedFlags {
		if strings.HasPrefix(full, prefix) {
			return true
		}
	}
	return false
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
		flag, value, attached := splitFlag(t.text)
		switch {
		case flag != profileFlag:
		case attached:
			name, found = value, true
		case i+1 >= len(toks) || !toks[i+1].known:
			return "", false, unknownProfile("the --profile value is not a literal")
		default:
			name, found = toks[i+1].text, true
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
	if pe.hookVars[name] || pe.census.Opaque {
		return true
	}
	for _, s := range pe.census.Sites[name] {
		if s.Kind != shellast.BindCall || s.Call == call {
			return true
		}
	}
	return false
}

// envProfile resolves AWS_PROFILE as call sees it: its own prefix or env
// wrapper, else the last top-level export before it, else the hook's
// environment. Any other way of setting it -- a bare assignment, one inside a
// subshell or conditional, unset, read, a value from a variable -- makes it
// unknown, and so does an export written below a call that may run after it.
func (pe profileEnv) envProfile(call *syntax.CallExpr) (string, error) {
	if pe.census.Opaque {
		return "", unknownProfile("eval or source could set AWS_PROFILE")
	}
	var own, exported *shellast.Site
	for i := range pe.census.Sites[profileVar] {
		s := &pe.census.Sites[profileVar][i]
		switch {
		case s.Kind == shellast.BindCall && s.Call != call:
			continue
		case s.Kind == shellast.BindCall:
			own = s
		case s.Kind == shellast.BindExport:
			switch {
			case call.Pos().After(s.End):
				exported = s
			case pe.repeated(call.Pos()):
				return "", unknownProfile("AWS_PROFILE is exported below this call, which sits in a loop or " +
					"function and so may run after that export")
			}
		default:
			return "", unknownProfile("AWS_PROFILE is set in a way the hook cannot follow (only a literal " +
				"`export AWS_PROFILE=name` or a prefix on the aws call counts)")
		}
		if !s.Literal {
			return "", unknownProfile("AWS_PROFILE is set from a variable or substitution")
		}
	}
	switch {
	case own != nil:
		return nonEmptyProfile(own.Value), nil
	case exported != nil:
		return nonEmptyProfile(exported.Value), nil
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
