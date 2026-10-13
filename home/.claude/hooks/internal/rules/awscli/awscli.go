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
// profile.go), asks.
package awscli

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/config"
	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-aws-cli"

const (
	endpointVar       = "AWS_ENDPOINT_URL"
	serviceEndpointPx = endpointVar + "_"
	ignoreEndpoints   = "AWS_IGNORE_CONFIGURED_ENDPOINT_URLS"
	endpointFlag      = "--endpoint-url"
)

// Check judges every aws call in req and returns the strictest verdict.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	environ := os.Environ()
	c := takeCensus(file)
	s := scope{
		env:      scanEnv(file, environ),
		census:   c,
		profiles: newProfileEnv(c, environ),
		policies: req.Config.AWSProfiles,
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
func (s *scope) checkAWS(c *syntax.CallExpr) (v hook.Verdict, hit bool) {
	name, rest := shellast.Invocation(c.Args, shellast.WordLit)
	if shellast.CommandName(name) != "aws" {
		return hook.Verdict{}, false
	}
	if makesNoRequest(rest) {
		return hook.Allowed(), true
	}
	toks := s.census.expand(rest, c.Pos())
	wrapperArgs := len(c.Args) - len(rest) - 1
	if local, found := flagEndpoint(toks); found {
		if local {
			return hook.Allowed(), true
		}
	} else if !s.env.poisoned && (callSetsLocalEndpoint(c, wrapperArgs) ||
		(s.env.exported && c.Pos().After(s.env.exportedFrom))) {
		return hook.Allowed(), true
	}

	profile, err := s.profiles.resolve(c, c.Args[:wrapperArgs], toks)
	if err != nil {
		return hook.Asked(fmt.Sprintf("AWS CLI call, and %v. Allow?", err)), true
	}
	return s.applyPolicy(profile, toks), true
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

// flagEndpoint finds an --endpoint-url flag in the aws arguments. found reports
// whether one is present; local whether its value is a known loopback URL.
func flagEndpoint(toks []token) (local, found bool) {
	for i, t := range toks {
		switch {
		case t.text == endpointFlag:
			if i+1 >= len(toks) {
				return false, true
			}
			next := toks[i+1]
			return next.known && isLoopbackURL(next.text), true
		case strings.HasPrefix(t.text, endpointFlag+"="):
			return t.known && isLoopbackURL(strings.TrimPrefix(t.text, endpointFlag+"=")), true
		}
	}
	return false, false
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
	for _, st := range topLevel(file.Stmts) {
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

// topLevel flattens the file's statements and && chains into the sequence the
// shell runs in order, so `export X=y && aws ...` counts the export as
// preceding the call. Statements inside subshells, functions and conditionals
// are not included: their exports may never reach a later call.
func topLevel(stmts []*syntax.Stmt) []*syntax.Stmt {
	var out []*syntax.Stmt
	var add func(st *syntax.Stmt)
	add = func(st *syntax.Stmt) {
		if b, ok := st.Cmd.(*syntax.BinaryCmd); ok && b.Op == syntax.AndStmt && !st.Negated && !st.Background {
			add(b.X)
			add(b.Y)
			return
		}
		out = append(out, st)
	}
	for _, st := range stmts {
		add(st)
	}
	return out
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
