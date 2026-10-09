// Package awscli prompts before an `aws` CLI invocation that could reach real
// AWS. Instead of a hard deny it returns ask, so the user approves each such
// command case by case. The whole shell tree is walked, so `aws` nested in
// pipelines, subshells, command substitutions, and && / || chains is still
// caught.
//
// A call that provably targets a loopback endpoint runs unprompted: local
// emulators (moto, LocalStack, understudy) see a stream of such calls, and none
// of them can touch an AWS account. "Provably" means the endpoint is a literal
// loopback URL the CLI will actually use:
//
//   - an --endpoint-url flag, which outranks every other endpoint setting;
//   - otherwise AWS_ENDPOINT_URL, set on the call itself (`AWS_ENDPOINT_URL=...
//     aws`, `env AWS_ENDPOINT_URL=... aws`) or exported by an earlier top-level
//     statement of the same command (`export AWS_ENDPOINT_URL=...; aws ...`).
//
// The environment route is refused whenever something could still redirect
// the call: a service-specific AWS_ENDPOINT_URL_<SERVICE> (which outranks the
// global variable) that is not loopback, AWS_IGNORE_CONFIGURED_ENDPOINT_URLS,
// an unset, or any non-literal or non-loopback assignment of an endpoint
// variable anywhere in the command or in the hook's own environment.
// `aws --version` and `aws [service [command]] help` make no requests and are
// allowed too.
package awscli

import (
	"net"
	"net/url"
	"os"
	"strings"

	"mvdan.cc/sh/v3/syntax"

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

// Check asks for approval when req invokes the aws CLI against anything that
// is not provably a loopback endpoint.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	env := scanEnv(file, os.Environ())
	if v, hit := shellast.FirstCall(file, func(c *syntax.CallExpr) (hook.Verdict, bool) {
		return checkAWS(c, env)
	}); hit {
		return v
	}
	return hook.Allowed()
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

// checkAWS returns an "ask" verdict when c invokes the aws CLI and the call is
// not provably local. Environment assignments preceding the command and
// wrappers such as sudo and env are stepped over (see shellast.Invocation).
func checkAWS(c *syntax.CallExpr, env endpointEnv) (hook.Verdict, bool) {
	name, rest := shellast.Invocation(c.Args, shellast.WordLit)
	if shellast.CommandName(name) != "aws" {
		return hook.Verdict{}, false
	}
	if makesNoRequest(rest) {
		return hook.Verdict{}, false
	}
	if local, found := flagEndpoint(rest); found {
		if local {
			return hook.Verdict{}, false
		}
		return asked(), true
	}
	wrapperArgs := len(c.Args) - len(rest) - 1
	if !env.poisoned && (callSetsLocalEndpoint(c, wrapperArgs) || (env.exported && c.Pos().After(env.exportedFrom))) {
		return hook.Verdict{}, false
	}
	return asked(), true
}

func asked() hook.Verdict { return hook.Asked("AWS CLI command detected. Allow?") }

// makesNoRequest reports whether the aws arguments only print the version or
// help: `aws --version`, `aws help`, `aws s3 help`, `aws s3 cp help`.
func makesNoRequest(args []*syntax.Word) bool {
	if len(args) == 1 && shellast.WordLit(args[0]) == "--version" {
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
// whether one is present; local whether its value is a literal loopback URL.
func flagEndpoint(args []*syntax.Word) (local, found bool) {
	for i, a := range args {
		w := shellast.WordLit(a)
		switch {
		case w == endpointFlag:
			if i+1 >= len(args) {
				return false, true
			}
			return literalLoopback(args[i+1]), true
		case strings.HasPrefix(w, endpointFlag+"="):
			if !shellast.IsLiteral(a) {
				return false, true
			}
			return isLoopbackURL(strings.TrimPrefix(w, endpointFlag+"=")), true
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
