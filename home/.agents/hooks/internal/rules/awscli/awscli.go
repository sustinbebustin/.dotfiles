// Package awscli decides whether an `aws` CLI invocation that could reach real
// AWS runs unprompted, asks, or is blocked. The whole shell tree is walked, so
// `aws` nested in pipelines, subshells, command substitutions, and && / ||
// chains is still caught, and every call in a command is judged: the strictest
// verdict wins.
//
// A call that provably targets a loopback endpoint runs unprompted: local
// emulators (moto, LocalStack, understudy) see a stream of such calls, and none
// of them can touch an AWS account. "Provably" means the endpoint is a loopback
// URL the CLI will actually use:
//
//   - an --endpoint-url flag, which outranks every other endpoint setting. Its
//     value may come through a variable the same command sets to one literal
//     value (`E=http://127.0.0.1:4566; aws --endpoint-url $E ...`; see vars.go);
//   - otherwise AWS_ENDPOINT_URL, set on the call itself (`AWS_ENDPOINT_URL=...
//     aws`, `env AWS_ENDPOINT_URL=... aws`) or exported by an earlier top-level
//     statement of the same command (`export AWS_ENDPOINT_URL=...; aws ...`).
//
// The environment route is refused whenever something could still redirect
// the call: a service-specific AWS_ENDPOINT_URL_<SERVICE> (which outranks the
// global variable) that is not loopback, AWS_IGNORE_CONFIGURED_ENDPOINT_URLS,
// an unset, or any non-literal or non-loopback assignment of an endpoint
// variable anywhere in the command or in the hook's own environment.
// `aws --version`, `aws [service [command]] help`, `aws configure list` and
// `aws configure list-profiles` make no requests and are allowed too.
//
// Any other call is judged by the policy the machine-local config gives its
// profile (config.AWSPolicy): allow, readOnly (see readonly.go), ask or deny.
// A profile the config does not name, or one the hook cannot determine (see
// profile.go), asks. A command that mentions aws but does not parse asks too.
//
// A function the command defines as a thin wrapper (`a() { aws --profile=x
// "$@"; }`) is seen through: each call of it is judged as the aws call it
// runs, provided every use of the function's name is such a call (see
// shellast.WrapperFuncs). Otherwise the body is judged as written, and its
// "$@" asks.
//
// An expansion the hook cannot read asks when it could add or complete a
// --profile or --endpoint-url option, abbreviations included (see vars.go).
// That covers the zsh forms the bash-mode parser reads as text (`$=name`).
// When quoting the value and attaching it to its option would settle that, the
// call is denied with the rewrite instead, so the agent fixes it unprompted.
//
// A command that defines its own aws function, or any alias, asks for every
// aws call in it: what `aws` runs there is no longer the CLI the call shows.
//
// Accepted gap: a function the command calls but does not define (one from
// the shell snapshot) could rebind a variable the census trusts. The hook
// cannot see such functions, so it assumes they do not.
package awscli

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"agent-hooks/internal/config"
	"agent-hooks/internal/hook"
	"agent-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-aws-cli"

const (
	endpointVar       = "AWS_ENDPOINT_URL"
	serviceEndpointPx = endpointVar + "_"
	ignoreEndpoints   = "AWS_IGNORE_CONFIGURED_ENDPOINT_URLS"
	endpointFlag      = "--endpoint-url"
)

// mentionsAWS matches raw command text that may run the aws CLI. It is for
// commands that did not parse, so it errs toward matching.
var mentionsAWS = regexp.MustCompile(`\baws\b`)

// Check judges every aws call in req and returns the strictest verdict.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		// A command the parser rejects may still run: zsh accepts syntax it
		// does not (`${c::=x}`), and that syntax can rebind what a call sees.
		if req.Shell.Status() == shellast.Unparseable && mentionsAWS.MatchString(req.Command) {
			return hook.Asked("AWS CLI call in a command the hook could not parse, so its profile and endpoint " +
				"cannot be checked. Allow?")
		}
		return hook.Allowed()
	}
	environ := os.Environ()
	c := takeCensus(file)
	s := scope{
		env:       scanEnv(file, environ),
		census:    c,
		profiles:  newProfileEnv(file, c, environ),
		policies:  req.Config.AWSProfiles,
		funcs:     awsWrapperFuncs(file, req.Command, c),
		redefined: redefinesAWS(file),
	}
	var verdicts []hook.Verdict
	syntax.Walk(file, func(n syntax.Node) bool {
		if call, isCall := n.(*syntax.CallExpr); isCall {
			if v, hit := s.checkAWS(call); hit {
				verdicts = append(verdicts, v)
			}
		}
		return true
	})
	return hook.Merge(verdicts)
}

// scope is everything a single call's check needs from the whole command.
type scope struct {
	env      endpointEnv
	census   census
	profiles profileEnv
	policies map[string]config.AWSPolicy
	// funcs are the functions the command defines as thin wrappers around aws.
	funcs shellast.WrapperFuncs
	// redefined is set when the command may give `aws` a meaning of its own.
	redefined bool
}

// redefinesAWS reports whether file defines a function named aws, or any
// alias. Either can make `aws args` run something the call does not show: the
// function's body may reach the CLI through a computed name, with its own
// profile, and zsh expands aliases in the commands that follow.
func redefinesAWS(file *syntax.File) bool {
	found := false
	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.FuncDecl:
			found = found || (x.Name != nil && x.Name.Value == "aws")
		case *syntax.CallExpr:
			name, _ := shellast.Invocation(x.Args, shellast.WordLit)
			found = found || shellast.CommandName(name) == "alias"
		}
		return !found
	})
	return found
}

// awsWrapperFuncs returns the wrapper functions of file (see
// shellast.WrapperFuncs) whose body calls aws directly. One that reaches aws
// through env or sudo is left out, so its body is judged as written.
func awsWrapperFuncs(file *syntax.File, src string, c census) shellast.WrapperFuncs {
	funcs := shellast.FindWrapperFuncs(file, src, c.Census)
	maps.DeleteFunc(funcs, func(_ string, w shellast.WrapperFunc) bool { return w.Command() != "aws" })
	return funcs
}

// endpointEnv is what the command's environment says about AWS_ENDPOINT_URL.
type endpointEnv struct {
	// poisoned means the environment route cannot be trusted at all: some
	// endpoint setting in the command or the hook's environment is dynamic,
	// remote, unset, or disabled.
	poisoned bool
	// exportedFrom is the end of the first top-level statement exporting a
	// loopback AWS_ENDPOINT_URL; calls after it inherit that endpoint. Valid
	// only when exported is true.
	exportedFrom syntax.Pos
	exported     bool
}

// checkAWS judges c when it invokes the aws CLI; hit is false for any other
// command. Environment assignments preceding the command and wrappers such as
// sudo and env are stepped over (see shellast.Invocation).
//
// A call of a wrapper function (see awsWrapperFuncs) is judged as the aws call
// its body runs, with the call's own arguments in place of "$@": the
// environment is the one the function is called in, so c stands for the aws
// call. The aws call in the body is then not judged a second time.
func (s *scope) checkAWS(c *syntax.CallExpr) (v hook.Verdict, hit bool) {
	if s.funcs.IsBody(c) {
		return hook.Verdict{}, false
	}
	var args []*syntax.Word
	wrapperArgs := 0
	if words, wrapped := s.funcs.Expand(c); wrapped {
		args = words[1:]
	} else {
		name, rest := shellast.Invocation(c.Args, shellast.WordLit)
		if shellast.CommandName(name) != "aws" {
			return hook.Verdict{}, false
		}
		if s.redefined {
			return hook.Asked("AWS CLI call in a command that defines its own aws function or an alias, " +
				"so what `aws` runs there cannot be checked. Allow?"), true
		}
		if makesNoRequest(rest) {
			return hook.Allowed(), true
		}
		args, wrapperArgs = rest, len(c.Args)-len(rest)-1
	}
	toks := s.census.expand(args, c.Pos())
	v = s.judge(c, wrapperArgs, toks)
	if v.Decision != hook.Ask {
		return v, true
	}
	// An expansion that only blocks the call by sitting in an option's value
	// slot has a mechanical fix; deny with it, so the agent rewrites the call
	// instead of a human approving it. The rewritten call is judged afresh when
	// it runs, so the hint can never let anything through.
	if fixed, hints, ok := s.census.valueSlotRewrite(toks); ok && s.judge(c, wrapperArgs, fixed).Decision == hook.Allow {
		return hook.Denied(fmt.Sprintf("Blocked: this AWS CLI call passes %s, which could "+
			"expand to a --profile or --endpoint-url option, so the profile cannot be confirmed. Nothing ran. "+
			"Quote each value and attach it to its option so it stays one argument: %s.",
			describeCount(len(hints)), strings.Join(hints, "; "))), true
	}
	return v, true
}

// judge applies the endpoint and profile rules to the call's tokens.
// The environment route is refused when a wrapper may drop AWS_ENDPOINT_URL:
// a loopback endpoint the call would not inherit, or one set by an outer env
// and unset by an inner one, leaves the CLI on real AWS.
func (s *scope) judge(c *syntax.CallExpr, wrapperArgs int, toks []token) hook.Verdict {
	wrappers := readWrappers(c.Args[:wrapperArgs])
	if local, found := flagEndpoint(toks); found {
		if local {
			return hook.Allowed()
		}
	} else if !s.env.poisoned && !wrappers.drops(endpointVar) && (callSetsLocalEndpoint(c, wrapperArgs) ||
		(s.env.exported && c.Pos().After(s.env.exportedFrom))) {
		return hook.Allowed()
	}

	profile, err := s.profiles.resolve(c, wrappers, toks)
	if err != nil {
		return hook.Asked(fmt.Sprintf("AWS CLI call, and %v. Allow?", err))
	}
	return s.applyPolicy(profile, toks)
}

func describeCount(n int) string {
	if n == 1 {
		return "an option value the shell may split or the CLI may read as an option"
	}
	return fmt.Sprintf("%d option values the shell may split or the CLI may read as options", n)
}

// positionalCommands are the high-level aws commands that take positional
// arguments, and boolean options among their flags, so a word after an option
// need not be its value.
var positionalCommands = map[string]bool{"logs tail": true, "logs start-live-tail": true}

// valueSlotRewrite rewrites every flag-capable token into `--opt="value"` and
// returns the rewritten tokens with a hint per rewrite. ok is false unless
// every such token can be rewritten, in a modelled API operation (no s3 or
// other command with positional arguments). One is rewritten when it is
// already `--opt=value` with the value unquoted (see attachedSource), or when
// it directly follows a literal `--opt` and its source is already one word
// (see quotedSource). Either way the option must take a value: not a global
// boolean, and not a guarded flag, whose values must be literal anyway.
func (c census) valueSlotRewrite(toks []token) (fixed []token, hints []string, ok bool) {
	op, err := findOperation(toks)
	if err != nil || op.service == "s3" || positionalCommands[op.String()] {
		return nil, nil, false
	}
	for i, t := range toks {
		if !t.mayBeFlags {
			fixed = append(fixed, t)
			continue
		}
		if opt, value, attached := attachedSource(t.word); attached {
			if !takesRewrittenValue(opt) {
				return nil, nil, false
			}
			fixed = append(fixed, token{text: opt + "="})
			hints = append(hints, fmt.Sprintf("`%s=%s`", opt, value))
			continue
		}
		if i == 0 {
			return nil, nil, false
		}
		// A known option is never itself rewritten, so it is still the last
		// token in fixed.
		opt := toks[i-1]
		value, quotable := c.quotedSource(t.word)
		if !quotable || !opt.known || strings.Contains(opt.text, "=") || !takesRewrittenValue(opt.text) {
			return nil, nil, false
		}
		fixed[len(fixed)-1] = token{text: opt.text + "="}
		hints = append(hints, fmt.Sprintf("`%s=%s`", opt.text, value))
	}
	return fixed, hints, len(hints) > 0
}

// takesRewrittenValue reports whether opt is a long option the rewrite may
// attach a quoted value to.
func takesRewrittenValue(opt string) bool {
	flag, _, _ := splitFlag(opt)
	return optionName.MatchString(opt) && !globalBoolFlags[opt] && !slices.Contains(guardedFlags, flag)
}

// optionName matches a long option as the CLI spells them.
var optionName = regexp.MustCompile(`^--[a-z0-9][a-z0-9-]*$`)

// plainValueText matches literal text that reads the same inside double quotes
// as outside them: nothing the shell expands, splits or unescapes.
var plainValueText = regexp.MustCompile(`^[A-Za-z0-9_./:@,+=%-]*$`)

// attachedSource reads a word written `--opt=value` whose value holds an
// unquoted variable or substitution, and returns the option with the value in
// double quotes. The author attached the value, so it is meant as one
// argument, and quoting it keeps it one; an array or other expansion that
// stays several words inside quotes is refused (see multiWord), as is a value
// with quoting of its own.
func attachedSource(w *syntax.Word) (opt, value string, ok bool) {
	if w == nil || len(w.Parts) < 2 {
		return "", "", false
	}
	first, isLit := w.Parts[0].(*syntax.Lit)
	if !isLit {
		return "", "", false
	}
	opt, head, attached := strings.Cut(first.Value, "=")
	if !attached || !plainValueText.MatchString(head) {
		return "", "", false
	}
	inner := &syntax.Word{Parts: []syntax.WordPart{&syntax.Lit{Value: head}}}
	for _, p := range w.Parts[1:] {
		switch x := p.(type) {
		case *syntax.Lit:
			if !plainValueText.MatchString(x.Value) {
				return "", "", false
			}
		case *syntax.ParamExp:
			if multiWord(x) {
				return "", "", false
			}
		case *syntax.CmdSubst, *syntax.ArithmExp:
		default:
			return "", "", false
		}
		inner.Parts = append(inner.Parts, p)
	}
	var sb strings.Builder
	if err := syntax.NewPrinter().Print(&sb, inner); err != nil {
		return "", "", false
	}
	return opt, `"` + sb.String() + `"`, true
}

// quotedSource returns w's source as one double-quoted word, when w is already
// meant as one value, so attaching it to its option keeps the meaning: a
// double-quoted string with no expansion that yields several words, or a bare
// $NAME the census binds only to single words (a loop item, an arithmetic
// result). An unquoted $(cmd) or other $NAME may be meant to split into
// several values (`--instance-ids $IDS`), so it is not rewritten.
func (c census) quotedSource(w *syntax.Word) (string, bool) {
	if w == nil || len(w.Parts) != 1 {
		return "", false
	}
	var sb strings.Builder
	if err := syntax.NewPrinter().Print(&sb, w); err != nil {
		return "", false
	}
	switch x := w.Parts[0].(type) {
	case *syntax.DblQuoted:
		for _, p := range x.Parts {
			if pe, ok := p.(*syntax.ParamExp); ok && multiWord(pe) {
				return "", false
			}
		}
		return sb.String(), true
	case *syntax.ParamExp:
		if x.Param != nil && !multiWord(x) && c.singleWord(x.Param.Value) {
			return `"` + sb.String() + `"`, true
		}
	}
	return "", false
}

// applyPolicy turns the profile's configured policy into a verdict for a call
// with arguments toks.
func (s *scope) applyPolicy(profile string, toks []token) hook.Verdict {
	policy, configured := s.policies[profile]
	switch {
	case !configured:
		return hook.Asked(fmt.Sprintf("AWS CLI call with profile %q, which has no policy in the hook config. Allow?",
			profile))
	case policy == config.AWSAllow:
		return hook.Allowed()
	case policy == config.AWSDeny:
		return hook.Denied(fmt.Sprintf("AWS CLI call with profile %q, which the hook config denies "+
			"(awsCli.profiles policy deny). Run it yourself if it is needed, or change the policy.", profile))
	case policy == config.AWSReadOnly:
		op, err := findOperation(toks)
		if err != nil {
			return hook.Asked(fmt.Sprintf("AWS CLI call with profile %q, which only runs reads unprompted, "+
				"and %s. Allow?", profile, err))
		}
		if isReadOnly(op) {
			return hook.Allowed()
		}
		return hook.Asked(fmt.Sprintf("AWS CLI call with profile %q, which only runs reads unprompted, "+
			"and %q is not one. Allow?", profile, op))
	}
	return hook.Asked(fmt.Sprintf("AWS CLI call with profile %q (policy ask). Allow?", profile))
}

// localCommands are aws commands that read only the local configuration, make
// no request, and print no secret.
var localCommands = map[string]bool{"configure list": true, "configure list-profiles": true}

// makesNoRequest reports whether the aws arguments only print the version or
// help (`aws --version`, `aws help`, `aws s3 help`, `aws s3 cp help`) or are one
// of localCommands.
func makesNoRequest(args []*syntax.Word) bool {
	if len(args) == 1 && shellast.WordLit(args[0]) == "--version" {
		return true
	}
	if len(args) == 2 && shellast.IsLiteral(args[0]) && shellast.IsLiteral(args[1]) &&
		localCommands[shellast.WordLit(args[0])+" "+shellast.WordLit(args[1])] {
		return true
	}
	if len(args) == 0 || len(args) > 3 {
		return false
	}
	for _, a := range args {
		if !shellast.IsLiteral(a) {
			return false
		}
		w := shellast.WordLit(a)
		if w == "" || strings.HasPrefix(w, "-") || strings.ContainsAny(w, ":/=") {
			return false
		}
	}
	return shellast.WordLit(args[len(args)-1]) == "help"
}

// flagEndpoint finds the --endpoint-url flag in the aws arguments; the last one
// wins, as in the CLI. found reports whether one is present; local whether its
// value is a known loopback URL. A token that may expand to options could add
// a later one, so it counts as a non-local endpoint.
func flagEndpoint(toks []token) (local, found bool) {
	for i, t := range toks {
		if t.mayBeFlags {
			return false, true
		}
		if !t.known {
			continue
		}
		flag, value, attached := splitFlag(t.text)
		switch {
		case flag != endpointFlag:
		case attached:
			local, found = isLoopbackURL(value), true
		default:
			local, found = i+1 < len(toks) && toks[i+1].known && isLoopbackURL(toks[i+1].text), true
		}
	}
	return local, found
}

// callSetsLocalEndpoint reports whether the call itself sets a literal loopback
// AWS_ENDPOINT_URL, as a prefix assignment or as an env-wrapper argument among
// the first wrapperArgs words.
func callSetsLocalEndpoint(c *syntax.CallExpr, wrapperArgs int) bool {
	for _, as := range c.Assigns {
		if as.Name != nil && as.Name.Value == endpointVar && as.Value != nil && literalLoopback(as.Value) {
			return true
		}
	}
	for i := 0; i < wrapperArgs && i < len(c.Args); i++ {
		w := c.Args[i]
		if v, ok := strings.CutPrefix(shellast.WordLit(w), endpointVar+"="); ok && shellast.IsLiteral(w) && isLoopbackURL(v) {
			return true
		}
	}
	return false
}

// scanEnv decides whether the environment route can be trusted for this
// command, and from where an exported loopback endpoint applies.
func scanEnv(file *syntax.File, environ []string) endpointEnv {
	var env endpointEnv
	for _, kv := range environ {
		k, v, _ := strings.Cut(kv, "=")
		if poisons(k, v, true) {
			env.poisoned = true
		}
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.Assign:
			if x.Name != nil && poisonsAssign(x) {
				env.poisoned = true
			}
		case *syntax.CallExpr:
			if callPoisons(x) {
				env.poisoned = true
			}
		}
		return true
	})
	for _, st := range shellast.TopLevel(file.Stmts) {
		if exportsLocalEndpoint(st) {
			env.exportedFrom, env.exported = st.End(), true
			break
		}
	}
	return env
}

// poisons reports whether setting name to value could send a call somewhere
// other than a loopback endpoint. literal is false when the value is computed
// by the shell and so unknown here.
func poisons(name, value string, literal bool) bool {
	switch {
	case name == ignoreEndpoints:
		return true
	case name == endpointVar || strings.HasPrefix(name, serviceEndpointPx):
		return !literal || !isLoopbackURL(value)
	}
	return false
}

func poisonsAssign(as *syntax.Assign) bool {
	if as.Value == nil {
		// `export AWS_ENDPOINT_URL` re-exports an unknown inherited value.
		return poisons(as.Name.Value, "", false)
	}
	return poisons(as.Name.Value, shellast.WordLit(as.Value), shellast.IsLiteral(as.Value))
}

// callPoisons catches endpoint settings that are not Assign nodes: `unset`, and
// NAME=value words handed to env.
func callPoisons(c *syntax.CallExpr) bool {
	name, rest := shellast.Invocation(c.Args, shellast.WordLit)
	if shellast.CommandName(name) == "unset" {
		for _, a := range rest {
			if poisons(shellast.WordLit(a), "", false) {
				return true
			}
		}
	}
	wrapperArgs := len(c.Args)
	if name != "" {
		wrapperArgs = len(c.Args) - len(rest) - 1
	}
	for i := 0; i < wrapperArgs; i++ {
		k, v, ok := strings.Cut(shellast.WordLit(c.Args[i]), "=")
		if ok && poisons(k, v, shellast.IsLiteral(c.Args[i])) {
			return true
		}
	}
	return false
}

// exportsLocalEndpoint reports whether st is `export AWS_ENDPOINT_URL=<literal
// loopback URL>`, possibly among other exports.
func exportsLocalEndpoint(st *syntax.Stmt) bool {
	d, ok := st.Cmd.(*syntax.DeclClause)
	if !ok || d.Variant == nil || d.Variant.Value != "export" || st.Background {
		return false
	}
	for _, as := range d.Args {
		if as.Name != nil && as.Name.Value == endpointVar && as.Value != nil && literalLoopback(as.Value) {
			return true
		}
	}
	return false
}

func literalLoopback(w *syntax.Word) bool {
	return shellast.IsLiteral(w) && isLoopbackURL(shellast.WordLit(w))
}

// isLoopbackURL reports whether raw is an http(s) URL whose host is localhost,
// a *.localhost name, or a loopback IP address.
func isLoopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
