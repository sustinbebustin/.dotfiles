// Package supabaseremote guards Supabase CLI commands that write to a remote
// database. Commands that wipe data or delete a project (db reset, migration
// down, projects delete, branches delete) return a hard `deny` when they point
// at anything but the local stack; other remote writes (db push, migration
// up/repair/squash, db query, storage rm) return `ask`. Local work and every
// read, including a remote `db dump`, pass untouched.
//
// Most of these commands target the local stack unless given --linked,
// --project-ref or --db-url, so the danger is one copied flag away from a
// routine command. A few target the linked project when given nothing; those
// count as remote unless --local is passed.
//
// The CLI is recognised however it is launched: bare, by path, behind sudo or
// env, or through a package runner (`pnpm exec supabase`, `npx supabase`). The
// whole shell tree is walked, so a call nested anywhere is still caught.
package supabaseremote

import (
	"fmt"
	"net/url"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-supabase-remote"

// Check returns the first guarded call's verdict.
func Check(req *hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		return hook.Allowed()
	}
	if v, hit := shellast.FirstCall(file, checkCall); hit {
		return v
	}
	return hook.Allowed()
}

// defaultTarget is where a command points when no target flag is given.
type defaultTarget int

const (
	// localByDefault commands reach a remote database only through a flag.
	localByDefault defaultTarget = iota
	// remoteByDefault commands reach the linked project unless --local is
	// passed. db push says so in its help; migration repair and storage rm
	// do not state a default, so they are treated the same way to fail safe.
	remoteByDefault
	// alwaysRemote commands act on the Supabase platform, never the local stack.
	alwaysRemote
)

type guard struct {
	decision hook.Decision
	target   defaultTarget
	// effect completes "it <effect>" in the reason.
	effect string
}

// guarded maps a command path to its guard. Commands absent here are allowed.
var guarded = map[string]guard{
	"db reset":         {hook.Deny, localByDefault, "wipes all of its data and replays the local migrations over it"},
	"migration down":   {hook.Deny, localByDefault, "reverts its applied migrations, dropping the data they hold"},
	"projects delete":  {hook.Deny, alwaysRemote, "deletes the whole project and its database"},
	"branches delete":  {hook.Deny, alwaysRemote, "deletes a preview branch and its database"},
	"db push":          {hook.Ask, remoteByDefault, "applies the local migrations to its schema"},
	"migration up":     {hook.Ask, localByDefault, "applies the pending migrations to its schema"},
	"migration repair": {hook.Ask, remoteByDefault, "rewrites its migration history table"},
	"migration squash": {hook.Ask, localByDefault, "rewrites its migration history table"},
	"db query":         {hook.Ask, localByDefault, "runs arbitrary SQL against it"},
	"storage rm":       {hook.Ask, remoteByDefault, "deletes storage objects from it"},
}

func checkCall(c *syntax.CallExpr) (hook.Verdict, bool) {
	args, ok := supabaseArgs(c)
	if !ok || hasHelp(args) {
		return hook.Verdict{}, false
	}
	path := commandPath(args)
	g, ok := guarded[path]
	if !ok {
		return hook.Verdict{}, false
	}
	target, remote := remoteTarget(args, g.target)
	if !remote {
		return hook.Verdict{}, false
	}

	if g.decision == hook.Deny {
		return hook.Denied(fmt.Sprintf(
			"[BLOCKED] `supabase %s` would run against %s, a remote database: it %s. "+
				"Nothing was run. Only the local stack may be changed this way from Claude Code -- "+
				"drop the remote flag to target local. If the remote change is really intended, "+
				"the user has to run it themselves.",
			path, target, g.effect,
		)), true
	}
	return hook.Asked(fmt.Sprintf(
		"`supabase %s` would run against %s, a remote database: it %s. Pass --local to target the local stack instead. Allow?",
		path, target, g.effect,
	)), true
}

// runners launch a package's binary, so `pnpm exec supabase` or
// `npx supabase@x` runs the same CLI as `supabase` does.
var runners = map[string]bool{
	"npx": true, "pnpx": true, "bunx": true, "pnpm": true, "yarn": true, "bun": true, "npm": true,
}

// supabaseArgs returns the words following the supabase command word, and
// false when c does not run the Supabase CLI. Through a runner, everything up
// to the first `supabase` or `supabase@<version>` token is the runner's own.
func supabaseArgs(c *syntax.CallExpr) ([]string, bool) {
	name, operands := shellast.Invocation(c.Args, shellast.WordLit)
	args := make([]string, len(operands))
	for i, a := range operands {
		args[i] = shellast.WordLit(a)
	}

	cmd := shellast.CommandName(name)
	if cmd == "supabase" {
		return args, true
	}
	if !runners[cmd] {
		return nil, false
	}
	for i, a := range args {
		if a == "supabase" || strings.HasPrefix(a, "supabase@") {
			return args[i+1:], true
		}
	}
	return nil, false
}

// valueFlags take the following token as their value when not written with
// `=`. They are skipped so a value is never read as a subcommand word:
// `supabase --workdir db reset` names a directory, not the db command.
var valueFlags = map[string]bool{
	// Global.
	"--workdir": true, "--profile": true, "--network-id": true, "--dns-resolver": true,
	"--output": true, "-o": true, "--output-format": true, "--log-level": true, "--agent": true,
	// Target selection, which may sit between the subcommand words.
	"--project-ref": true, "--db-url": true, "--password": true, "-p": true,
}

// commandPath joins the first two positional words: "db reset", "projects
// delete". Every guarded command is exactly two deep.
func commandPath(args []string) string {
	var words []string
	for i := 0; i < len(args) && len(words) < 2; i++ {
		a := args[i]
		switch {
		case !strings.HasPrefix(a, "-"):
			words = append(words, a)
		case valueFlags[a]:
			i++
		}
	}
	return strings.Join(words, " ")
}

// remoteTarget describes the remote database args point at, and reports false
// when they point at the local stack. A remote flag wins over --local, since
// the CLI refuses the pair rather than choosing local.
func remoteTarget(args []string, def defaultTarget) (string, bool) {
	local := false
	for i := 0; i < len(args); i++ {
		name, val, attached := strings.Cut(args[i], "=")
		if !attached && valueFlags[name] && i+1 < len(args) {
			i++
			val = args[i]
		}
		switch name {
		case "--linked":
			if !attached || val == "true" {
				return "the linked project", true
			}
		case "--project-ref":
			return "project " + val, true
		case "--db-url":
			if host, remote := dbURLHost(val); remote {
				return host, true
			}
		case "--local":
			local = !attached || val == "true"
		}
	}

	switch def {
	case alwaysRemote:
		return "the Supabase platform", true
	case remoteByDefault:
		if !local {
			return "the linked project (this command's default without --local)", true
		}
	case localByDefault:
	}
	return "", false
}

// dbURLHost returns a --db-url's host for the reason, never the whole URL,
// which carries the password. A URL that cannot be read -- most often a
// variable the hook cannot expand -- counts as remote: the guard fails safe.
func dbURLHost(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "an unreadable --db-url (the hook cannot see where it points)", true
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return "", false
	}
	return "the database at " + u.Hostname(), true
}

func hasHelp(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}
