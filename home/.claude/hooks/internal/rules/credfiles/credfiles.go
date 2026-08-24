// Package credfiles guards credential files. Read/Edit/Write/Grep are checked
// by their path argument; for Bash, every command argument and every
// redirection target is classified, so `cat .env`, `cp ~/.aws/credentials
// /tmp`, `source .env`, and `tr a b < .env` are all caught.
//
// What that does not reach: a path that only ever appears as a variable
// assignment value or in a loop or case word list is not inspected, because
// the walk visits call arguments and redirects rather than every Word.
//
// Paths that are acted on fall into two tiers. Credential material -- env files, private keys,
// ~/.aws/credentials, .netrc, anything under ~/.ssh or ~/.gnupg -- is denied
// outright. Config files that merely tend to carry a token (.npmrc, ~/.kube,
// Terraform state) are sent to the human as an ask, because they are also
// edited routinely and a hard block would be wrong more often than right.
//
// Example variants (.env.example, .sample, .template) and public key material
// (*.pub, known_hosts, authorized_keys, certificates) stay allowed so the
// documented shape of a config remains readable.
package credfiles

import (
	"fmt"
	"path"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/shellast"
)

// Name identifies this rule to the dispatcher.
const Name = "block-credential-files"

// tier is how sensitive a path is: nothing to act on, worth a human's
// approval, or never allowed.
type tier int

const (
	tierNone tier = iota
	tierSensitive
	tierSecret
)

// Check inspects whichever surface the tool exposes: a path for the file tools,
// the whole command for Bash.
func Check(req hook.Request) hook.Verdict {
	switch req.ToolName {
	case "Read", "Edit", "Write", "Grep":
		return checkPath(req.ToolName, req.FilePath)
	case "Bash":
		return checkBash(req)
	}
	return hook.Allowed()
}

// verbs describes what each tool would do to the file, so the reason names the
// action that was actually attempted.
var verbs = map[string]string{
	"Read":  "Reading",
	"Edit":  "Editing",
	"Write": "Writing to",
	"Grep":  "Searching",
}

func checkPath(tool, p string) hook.Verdict {
	verb, ok := verbs[tool]
	if !ok {
		verb = "Accessing"
	}
	switch classify(p) {
	case tierSecret:
		return hook.Denied(fmt.Sprintf("%s credential files is blocked for security. Use the .example/.sample/.template variant, or load the value through your secret manager. (path: %s)", verb, p))
	case tierSensitive:
		return hook.Asked(fmt.Sprintf("%s %s needs approval: this file commonly holds an auth token. Allow only if you know this copy carries no secret.", verb, p))
	}
	return hook.Allowed()
}

// classify decides the tier of a path from its basename and its directory
// components. Directory components matter because the filename alone is often
// innocuous -- `credentials`, `config` -- and only the enclosing `.aws` or
// `.ssh` makes it a secret.
func classify(p string) tier {
	if p == "" {
		return tierNone
	}
	cleaned := path.Clean(strings.TrimSpace(p))
	base := path.Base(cleaned)
	switch base {
	case "", ".", "..", "/":
		return tierNone
	}
	if isSafeVariant(base) || isPublicMaterial(base) {
		return tierNone
	}
	if looksLikeEnv(base) || isSecretName(base) || hasSecretExt(base) || underAnyDir(cleaned, secretDirs) {
		return tierSecret
	}
	if isSensitiveName(base) || underAnyDir(cleaned, sensitiveDirs) {
		return tierSensitive
	}
	return tierNone
}

// looksLikeEnv reports whether base names an env file: `.env`, `.envrc`,
// `.env.local`, `.env-staging`, `prod.env`, `app.env.local`.
//
// The leading-dot `.env*` prefix deliberately covers direnv's `.envrc` and any
// other `.env`-prefixed dotfile: those carry secrets just like `.env` does, so
// matching the exact name `.env` alone would leave a hole.
func looksLikeEnv(base string) bool {
	if strings.HasPrefix(base, ".env") {
		return true
	}
	if strings.HasSuffix(base, ".env") {
		return true
	}
	if strings.Contains(base, ".env.") || strings.Contains(base, ".env-") {
		return true
	}
	return false
}

// secretNames are basenames that are credential material wherever they appear.
var secretNames = map[string]bool{
	"credentials":      true, // aws, and the conventional name elsewhere
	"credentials.json": true,
	"credentials.yml":  true,
	"credentials.yaml": true,
	".credentials":     true,
	".git-credentials": true,
	".netrc":           true,
	"_netrc":           true,
	".authinfo":        true,
	".pgpass":          true,
	".my.cnf":          true,
	".htpasswd":        true,
	".dockercfg":       true,
	"shadow":           true, // /etc/shadow
	"master.key":       true, // rails
	"secring.gpg":      true,
	"identity":         true, // ssh private key under its default alternate name
	".s3cfg":           true,
	".boto":            true,
	".rclone.conf":     true,
}

// secretWords name a file after what it holds. They only count on data-ish
// files: `token` on its own is an everyday grep argument, and `token.ts` is
// source code, but `tokens.json` is a credential store.
var secretWords = []string{
	"secret", "credential", "password", "passwd", "apikey", "api_key",
	"api-key", "token", "privatekey", "private_key", "private-key",
}

// dataExts are the formats a credential actually gets stored in.
var dataExts = map[string]bool{
	".json": true, ".yaml": true, ".yml": true, ".toml": true,
	".ini": true, ".conf": true, ".cfg": true, ".txt": true,
	".csv": true, ".xml": true, ".properties": true, ".plist": true,
}

// secretExts are formats that are key material by definition.
var secretExts = map[string]bool{
	".pem": true, ".key": true, ".p12": true, ".pfx": true, ".jks": true,
	".keystore": true, ".ppk": true, ".kdbx": true, ".keytab": true,
	".gpg": true, ".pkcs12": true, ".asc": true, ".ovpn": true,
}

func isSecretName(base string) bool {
	lower := strings.ToLower(base)
	if secretNames[lower] {
		return true
	}
	// Every OpenSSH default private key is `id_<type>`; the matching public
	// half ends in `.pub` and is exempted before this runs.
	if strings.HasPrefix(lower, "id_") {
		return true
	}
	if !dataExts[strings.ToLower(path.Ext(lower))] {
		return false
	}
	for _, word := range secretWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}

func hasSecretExt(base string) bool {
	return secretExts[strings.ToLower(path.Ext(base))]
}

// sensitiveNames hold tokens often enough to warrant a prompt, but are also
// ordinary config that gets edited by hand.
var sensitiveNames = map[string]bool{
	".npmrc":       true,
	".pypirc":      true,
	".gemrc":       true,
	".terraformrc": true,
	"kubeconfig":   true,
	".pip.conf":    true,
}

func isSensitiveName(base string) bool {
	lower := strings.ToLower(base)
	if sensitiveNames[lower] {
		return true
	}
	// Terraform state inlines resource attributes, passwords included.
	if strings.HasSuffix(lower, ".tfstate") || strings.HasSuffix(lower, ".tfstate.backup") {
		return true
	}
	if strings.HasSuffix(lower, ".tfvars") || strings.HasSuffix(lower, ".tfvars.json") {
		return true
	}
	return false
}

// secretDirs make every file beneath them credential material, whatever the
// file is called.
var secretDirs = map[string]bool{
	".ssh":        true,
	".aws":        true,
	".gnupg":      true,
	".azure":      true,
	"gcloud":      true, // ~/.config/gcloud
	".chef":       true,
	".subversion": true,
}

// sensitiveDirs hold registry and cluster credentials next to plain config.
var sensitiveDirs = map[string]bool{
	".docker": true,
	".kube":   true,
}

// underAnyDir reports whether any directory component of cleaned is in dirs.
func underAnyDir(cleaned string, dirs map[string]bool) bool {
	dir := path.Dir(cleaned)
	for part := range strings.SplitSeq(dir, "/") {
		if dirs[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

func isSafeVariant(base string) bool {
	for _, marker := range []string{".example", ".sample", ".template", ".dist"} {
		if strings.HasSuffix(base, marker) || strings.Contains(base, marker+".") {
			return true
		}
	}
	return false
}

// isPublicMaterial exempts the halves of a keypair that are meant to be shared,
// plus the SSH bookkeeping files that sit beside private keys.
func isPublicMaterial(base string) bool {
	lower := strings.ToLower(base)
	if strings.HasSuffix(lower, ".pub") {
		return true
	}
	switch {
	case strings.HasPrefix(lower, "known_hosts"), strings.HasPrefix(lower, "authorized_keys"):
		return true
	}
	switch path.Ext(lower) {
	case ".crt", ".cer", ".csr", ".pub":
		return true
	}
	return false
}

// checkBash inspects any command that references a credential path, whether as
// a command argument (`cat .env`, `xxd .envrc`, `cp ~/.aws/credentials /tmp`,
// `source .env`) or as a redirection target (`tr a b < .env`). There is
// intentionally no allowlist of "reader" commands: enumerating every binary
// that can read a file is a losing game, and there is no benign reason for a
// command to name a secret.
func checkBash(req hook.Request) hook.Verdict {
	file, ok := req.Shell.File()
	if !ok {
		if req.Shell.Status() != shellast.Unparseable {
			return hook.Allowed()
		}
		// Unparseable: fail CLOSED if the raw text names a credential path.
		// Better to over-block a malformed command than to leak on a parse
		// edge case the AST walk would have caught.
		if worst, _ := rawWorstTier(req.Command); worst == tierSecret {
			return hook.Denied("This command references a credential file but could not be parsed safely, so it is blocked. Use the .example/.sample/.template variant, or load the value through your secret manager.")
		}
		return hook.Allowed()
	}

	found := tierNone
	var foundPath string
	consider := func(lit string) {
		if t := classify(lit); t > found {
			found, foundPath = t, lit
		}
	}
	syntax.Walk(file, func(n syntax.Node) bool {
		if found == tierSecret {
			return false
		}
		switch x := n.(type) {
		case *syntax.CallExpr:
			for _, arg := range x.Args {
				consider(shellast.WordLit(arg))
			}
		case *syntax.Redirect:
			consider(shellast.WordLit(x.Word))
		}
		return true
	})
	return bashVerdict(found, foundPath)
}

func bashVerdict(t tier, p string) hook.Verdict {
	switch t {
	case tierSecret:
		return hook.Denied(fmt.Sprintf("Blocked: command references the credential file %q. Reading, copying, sourcing, or redirecting a secret is not allowed. Use the .example/.sample/.template variant, or load the value through your secret manager.", p))
	case tierSensitive:
		return hook.Asked(fmt.Sprintf("This command references %q, which commonly holds an auth token. Approve only if you know this copy carries no secret.", p))
	}
	return hook.Allowed()
}

// rawWorstTier is the fail-closed fallback for commands the shell parser
// rejects. It splits on whitespace and shell metacharacters and classifies each
// resulting token, returning the highest tier found and the token that reached
// it. A command with nothing to act on yields tierNone and an empty path.
func rawWorstTier(cmd string) (worst tier, worstPath string) {
	fields := strings.FieldsFunc(cmd, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ';', '|', '&', '<', '>', '(', ')', '"', '\'', '=':
			return true
		default:
			return false
		}
	})
	for _, f := range fields {
		if t := classify(f); t > worst {
			worst, worstPath = t, f
		}
	}
	return worst, worstPath
}
