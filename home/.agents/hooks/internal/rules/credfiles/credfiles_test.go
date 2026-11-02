package credfiles

import (
	"testing"

	"agent-hooks/internal/config"
	"agent-hooks/internal/hook"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		path string
		want tier
	}{
		// env files, the original scope
		{".env", tierSecret},
		{"/srv/app/.env.production", tierSecret},
		{".envrc", tierSecret},
		{"prod.env", tierSecret},
		{"app.env-staging", tierSecret},
		{".env.example", tierNone},
		{".env.sample", tierNone},
		{".env.template", tierNone},
		{".env.dist", tierNone},
		{".env.example.local", tierNone},
		{"PROD.ENV", tierSecret},
		{".Env.local", tierSecret},
		{".ENVRC", tierSecret},

		// patterns whose literal part can only mean a credential file
		{"credential*", tierSecret},
		{".netr?", tierSecret},
		{"cred*", tierSecret},
		{"src/*", tierNone},
		{"*", tierNone},
		{"*.go", tierNone},
		{"c*", tierNone}, // too short to mean anything

		// cloud and CLI credential stores
		{"/home/me/.aws/credentials", tierSecret},
		{"~/.aws/config", tierSecret},
		{"/home/me/.config/gcloud/application_default_credentials.json", tierSecret},
		{"/home/me/.azure/msal_token_cache.json", tierSecret},
		{".s3cfg", tierSecret},

		// ssh and gpg
		{"/home/me/.ssh/id_ed25519", tierSecret},
		{"id_rsa", tierSecret},
		{"~/.ssh/anything_at_all", tierSecret},
		{"~/.gnupg/secring.gpg", tierSecret},
		{"~/.ssh/id_ed25519.pub", tierNone},
		{"~/.ssh/known_hosts", tierNone},
		{"~/.ssh/authorized_keys", tierNone},

		// the credential directory named as the target, which reads all of it
		{"~/.ssh", tierSecret},
		{"/home/me/.aws", tierSecret},
		{"~/.gnupg/", tierSecret},
		{"~/.kube", tierSensitive},
		{"~/.config/gcloud", tierSecret},

		// inside a credential directory an example marker means nothing, but
		// public key material and the ask-tier directories are unaffected
		{"~/.ssh/id_rsa.template", tierSecret},
		{"~/.aws/credentials.example", tierSecret},
		{"~/.ssh/id_ed25519.pub", tierNone},
		{"~/.kube/config.example", tierNone},

		// key material by extension
		{"certs/server.key", tierSecret},
		{"certs/server.pem", tierSecret},
		{"vault.kdbx", tierSecret},
		{"client.p12", tierSecret},
		{"vpn/office.ovpn", tierSecret},
		{"certs/server.crt", tierNone},
		{"certs/server.csr", tierNone},

		// named-after-contents, but only on data formats
		{"secrets.yaml", tierSecret},
		{"config/tokens.json", tierSecret},
		{"db_password.txt", tierSecret},
		{"secrets.example.yaml", tierNone},
		{"token", tierNone},       // a bare grep argument
		{"useToken.ts", tierNone}, // source code
		{"secrets.ts", tierNone},

		// misc unix credential files
		{"/home/me/.netrc", tierSecret},
		{".git-credentials", tierSecret},
		{"/etc/shadow", tierSecret},
		{"config/master.key", tierSecret},

		// ask tier
		{"/home/me/.npmrc", tierSensitive},
		{"home/.npmrc", tierSensitive},
		{".pypirc", tierSensitive},
		{"~/.kube/config", tierSensitive},
		{"~/.docker/config.json", tierSensitive},
		{"infra/terraform.tfstate", tierSensitive},
		{"infra/prod.tfvars", tierSensitive},

		// SSH key names by algorithm, and words that merely start with id_
		{"id_ed25519_sk", tierSecret},
		{"id_rsa_work.old", tierSecret},
		{"id_ecdsa", tierSecret},
		{"docs/id_root-user.md", tierNone},
		{"id_string|Exception", tierNone},

		// filters and patterns that only look like they end in a key extension
		{".shapes | to_entries[] | .key", tierNone},
		{".shapes|to_entries[]|select(.value.type==\"operation\")|.key", tierNone},
		{"force|VersionId|object.Key", tierNone},
		{`SimpleNotificationService-[0-9a-f]+\.pem`, tierNone},
		{".key", tierSecret},
		{"server (1).key", tierSecret},

		// a URL's query string is not part of its path
		{"https://api.github.com/repos/o/r/git/trees/master?recursive=1", tierNone},
		{"https://example.com/certs/server.pem", tierSecret},
		{"file:///home/me/.aws/credentials", tierSecret},
		{`curl -s "https://x/y?z=1"; cat ~/.ssh/id_rsa`, tierSecret},
		{"u='h://x?'; print(open('/home/u/.aws/credentials').read())", tierSecret},
		{"https://x/?a/home/u/.ssh/id_rsa", tierNone}, // the shell passes this whole word as one URL

		// regex alternations and escapes are search patterns, not file names
		{`UPDATE|writeFile|process\.env`, tierNone},
		{"BASE|basePath|/api|prefix|process.env", tierNone},
		{`^\.env`, tierNone},
		{`~/.ssh/a|b`, tierSecret}, // a credential directory still outranks it

		// mws's port lease is rendered from tracked config
		{".mws-ports.env", tierNone},
		{"/home/me/ws/copy/.wt/04/.mws-ports.env", tierNone},
		{"~/.ssh/.mws-ports.env", tierSecret},
		{".mws-ports.env.local", tierSecret},

		// system apt keyrings hold public keys only
		{"/usr/share/keyrings/hashicorp-archive-keyring.gpg", tierNone},
		{"/etc/apt/keyrings/docker.asc", tierNone},
		{"/etc/apt/trusted.gpg.d/ubuntu.gpg", tierNone},
		{"/usr/share/keyrings/../../../home/me/.gnupg/secring.gpg", tierSecret},
		{"/home/me/usr/share/keyrings/x.gpg", tierSecret},
		{"usr/share/keyrings/x.gpg", tierSecret},

		// a custom-named key without an extension still reads as one
		{"deploy/id_deploy", tierSecret},
		{"keys/id_github", tierSecret},

		// ordinary paths
		{"", tierNone},
		{"main.go", tierNone},
		{"/etc/hosts", tierNone},
		{"README.md", tierNone},
		{"package.json", tierNone},
	}

	for _, tc := range cases {
		if got := classify(tc.path); got != tc.want {
			t.Errorf("classify(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestCheckBash(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		{name: "plain command is allowed", cmd: `ls -la`, decision: "allow"},
		{name: "reading env is denied", cmd: `cat .env`, decision: "deny"},
		{name: "reading aws credentials is denied", cmd: `cat ~/.aws/credentials`, decision: "deny"},
		{name: "copying a private key is denied", cmd: `cp ~/.ssh/id_ed25519 /tmp/k`, decision: "deny"},
		{name: "redirect from a key is denied", cmd: `base64 < server.pem`, decision: "deny"},
		{name: "quoted path is denied", cmd: `cat "$HOME/.netrc"`, decision: "deny"},
		{name: "example variant is allowed", cmd: `cat .env.example`, decision: "allow"},
		{name: "public key is allowed", cmd: `cat ~/.ssh/id_ed25519.pub`, decision: "allow"},
		{name: "grep for the word token is allowed", cmd: `grep -r token src`, decision: "allow"},
		{name: "recursive grep of the ssh dir is denied", cmd: `grep -r BEGIN ~/.ssh`, decision: "deny"},
		{name: "archiving the aws dir is denied", cmd: `tar -czf /tmp/x.tgz ~/.aws`, decision: "deny"},
		{name: "listing the kube dir asks", cmd: `ls ~/.kube`, decision: "ask"},
		{name: "npmrc asks", cmd: `cat ~/.npmrc`, decision: "ask"},
		{name: "deny wins over ask in one command", cmd: `cat ~/.npmrc ~/.aws/credentials`, decision: "deny"},
		{name: "unparseable command naming a secret is denied", cmd: `cat .env "unterminated`, decision: "deny"},
		{name: "unparseable command naming nothing is allowed", cmd: `echo "unterminated`, decision: "allow"},
		{name: "empty command is allowed", cmd: ``, decision: "allow"},

		// a glob that excludes the file names it to keep it out
		{name: "rg exclusion glob is allowed", cmd: `rg -n x --glob '!**/.env*' src`, decision: "allow"},
		{name: "rg short exclusion glob is allowed", cmd: `rg -n x -g '!.env' .`, decision: "allow"},
		{name: "rg attached exclusion glob is allowed", cmd: `rg -n x --glob='!*.tfstate' .`, decision: "allow"},
		{name: "rg iglob exclusion is allowed", cmd: `sudo rg x --iglob '!.env' .`, decision: "allow"},
		{name: "rg inclusion glob is denied", cmd: `rg -n x -g '.env*' .`, decision: "deny"},
		{name: "rg exclusion beside an inclusion is denied", cmd: `rg x -g '!.env' -g '.env' .`, decision: "deny"},
		{name: "rg double negation is denied", cmd: `rg x -g '!!.env' .`, decision: "deny"},
		{name: "rg exclusion with a secret operand is denied", cmd: `rg x -g '!.env' .env`, decision: "deny"},
		{name: "bang word outside a glob flag is denied", cmd: `rg x '!.env'`, decision: "deny"},
		{name: "another tool's -g is not an exclusion", cmd: `cat -g '!.env'`, decision: "deny"},
		{name: "find negated name is allowed", cmd: `find . -type f -not -name '.env*' -not -path '*/.git*'`, decision: "allow"},
		{name: "find bang name is allowed", cmd: `find d ! -name .env -print`, decision: "allow"},
		{name: "find positive name is denied", cmd: `find . -name .env -exec cat {} +`, decision: "deny"},
		{name: "find double negation is denied", cmd: `find . -not -not -name .env -exec cat {} +`, decision: "deny"},
		{name: "find negation inside a group is denied", cmd: `find . -not \( -not -name .env \) -exec cat {} +`, decision: "deny"},
		{name: "find negation beside -o is denied", cmd: `find . -not -name .env -o -exec cat {} +`, decision: "deny"},
		{name: "find negation beside -or is denied", cmd: `find . -not -name .env -or -print`, decision: "deny"},
		{name: "find negation beside a comma is denied", cmd: `find . -not -name .env , -exec cat {} +`, decision: "deny"},
		{name: "find prune is denied", cmd: `find . -path '*/.env*' -prune -o -type f -print`, decision: "deny"},

		// regex words
		{name: "regex alternation is allowed", cmd: `rg -n 'UPDATE|writeFile|process\.env' src`, decision: "allow"},
		{name: "regex beside a secret operand is denied", cmd: `rg -o '^\.env' .env`, decision: "deny"},
		{name: "unquoted escape still names the file", cmd: `cat \.env`, decision: "deny"},
		{name: "ANSI-C escape inside an env name is denied", cmd: `cat $'.env\x2dprod'`, decision: "deny"},
		{name: "ANSI-C escaped env name is denied", cmd: `cat $'\x2eenv'`, decision: "deny"},
		{name: "ANSI-C escaped aws path is denied", cmd: `cat $'\x2fhome\x2fme\x2f.aws\x2fcredentials'`, decision: "deny"},
		{name: "ANSI-C regex alternation is allowed", cmd: `rg $'a|process\\.env' src`, decision: "allow"},

		// a command name proves nothing about what runs: a function, alias or
		// another binary on PATH can take it
		{name: "test is not exempt", cmd: `test(){ cat "$1"; }; test .env`, decision: "deny"},
		{name: "touch is not exempt", cmd: `/tmp/x/touch .env`, decision: "deny"},
		{name: "echo of a secret name is denied", cmd: `echo .env | xargs cat`, decision: "deny"},
		{name: "ls of a secret is denied", cmd: `ls .env | xargs cat`, decision: "deny"},

		{name: "mws port lease is allowed", cmd: `cat .mws-ports.env | rg DB_PORT`, decision: "allow"},
		{name: "public apt keyring is allowed", cmd: `gpg --no-default-keyring --keyring /usr/share/keyrings/hashicorp-archive-keyring.gpg --list-keys`, decision: "allow"},
	}

	for _, tc := range cases {
		got := Check(hook.NewRequest("Bash", "", "", tc.cmd)).Decision.String()
		if got != tc.decision {
			t.Errorf("%s: Check(%q) = %s, want %s", tc.name, tc.cmd, got, tc.decision)
		}
	}
}

func TestCheckPath(t *testing.T) {
	cases := []struct {
		tool     string
		path     string
		decision string
	}{
		{"Read", ".env", "deny"},
		{"Edit", "~/.aws/credentials", "deny"},
		{"Write", "server.key", "deny"},
		{"Grep", "secrets.yaml", "deny"},
		{"Read", "~/.npmrc", "ask"},
		{"Read", ".env.example", "allow"},
		{"Write", "main.go", "allow"},
	}

	for _, tc := range cases {
		if got := checkPath(tc.tool, tc.path, false).Decision.String(); got != tc.decision {
			t.Errorf("checkPath(%s, %q) = %s, want %s", tc.tool, tc.path, got, tc.decision)
		}
	}
}

// TestRelaxedRoots pins what a configured relaxed root does and does not
// relax: the name-word heuristics go, credential material proper stays.
func TestRelaxedRoots(t *testing.T) {
	const root = "/home/tester/dev/emu"
	cases := []struct {
		name     string
		tool     string
		cwd      string
		input    string
		decision string
	}{
		{"model named after secrets manager", "Read", "", root + "/models/secrets-manager-2017-10-17.json", "allow"},
		{"fixture named credentials", "Write", "", root + "/testdata/credentials.json", "allow"},
		{"relative sdk credentials package from inside the root", "Bash", root, `rg -n x aws-sdk-go-v2/credentials`, "allow"},
		{"regex mentioning id_ from inside the root", "Bash", root, `rg -n "id_string|Exception" moto.py`, "allow"},
		{"shape named Credentials from inside the root", "Bash", root, `python3 -I shape.py sts.json AssumeRoleResponse Credentials`, "allow"},
		{"the .credentials dotfile stays denied", "Bash", root, `cat .credentials`, "deny"},
		{"env file in the root stays denied", "Read", "", root + "/.env", "deny"},
		{"key file in the root stays denied", "Read", "", root + "/tls/server.key", "deny"},
		{"netrc in the root stays denied", "Bash", root, `cat .netrc`, "deny"},
		{"aws credentials outside the root stay denied", "Bash", root, `cat ~/.aws/credentials`, "deny"},
		{"climbing out of the root is not relaxed", "Bash", root, `cat ../secrets.json`, "deny"},
		{"a neighbouring directory is not relaxed", "Read", "", root + "-old/secrets.json", "deny"},
		{"relative path with no cwd is not relaxed", "Bash", "", `cat secrets.json`, "deny"},
		{"a cd out of the root takes relative paths with it", "Bash", root, `cd /home/tester && cat secrets.json`, "deny"},
		{"a cd within the root keeps relative paths relaxed", "Bash", root, `cd ` + root + `/sub && cat secrets.json`, "allow"},
		{"an unknown cd leaves relative paths unplaced", "Bash", root, `cd "$D" && cat secrets.json`, "deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req *hook.Request
			if tc.tool == "Bash" {
				req = hook.NewRequest("Bash", "", "", tc.input)
			} else {
				req = hook.NewRequest(tc.tool, tc.input, "", "")
			}
			req.Cwd = tc.cwd
			req.Config = config.Config{RelaxedCredRoots: []string{root}}
			if got := Check(req).Decision.String(); got != tc.decision {
				t.Errorf("Check(%s %q, cwd %q) = %s, want %s", tc.tool, tc.input, tc.cwd, got, tc.decision)
			}
		})
	}
}

// TestGrepUsesPathField pins the fallback in hook.NewRequest: Grep names its
// argument `path`, not `file_path`.
func TestGrepUsesPathField(t *testing.T) {
	got := Check(hook.NewRequest("Grep", "", "/repo/.env", ""))
	if got.Decision != hook.Deny {
		t.Fatalf("Grep on a credential path = %q, want deny", got.Decision)
	}
}

// TestUnmatchedToolIsAllowed pins that a tool outside the rule's list is not
// judged, even when it carries a credential path.
func TestUnmatchedToolIsAllowed(t *testing.T) {
	got := Check(hook.NewRequest("Glob", "/repo/.env", "", ""))
	if got.Decision != hook.Allow {
		t.Fatalf("Glob payload = %q, want allow", got.Decision)
	}
}

// TestNestedShellScriptIsNotAPath keeps the reason truthful: the script a shell
// runs with -c is a program, and classifying the whole word as a path would
// report a credential file named "cat .env". The dispatcher checks that script
// on its own, where the real path is named.
func TestNestedShellScriptIsNotAPath(t *testing.T) {
	got := Check(hook.NewRequest("Bash", "", "", `sh -c 'cat .env'`))
	if got.Decision != hook.Allow {
		t.Errorf("decision = %q, want allow -- the outer call names no path itself (reason: %s)",
			got.Decision, got.Reason)
	}
}

// TestNestedShellScriptWithARealPathStillCounts pins the other side: only the
// script word is exempt, so a credential named as an operand of the shell
// itself is still caught.
func TestNestedShellScriptWithARealPathStillCounts(t *testing.T) {
	got := Check(hook.NewRequest("Bash", "", "", `sh -c 'ls' < .env`))
	if got.Decision != hook.Deny {
		t.Errorf("decision = %q, want deny", got.Decision)
	}
}
