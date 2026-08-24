package credfiles

import (
	"testing"

	"claude-hooks/internal/hook"
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
		if got := checkPath(tc.tool, tc.path).Decision.String(); got != tc.decision {
			t.Errorf("checkPath(%s, %q) = %s, want %s", tc.tool, tc.path, got, tc.decision)
		}
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
