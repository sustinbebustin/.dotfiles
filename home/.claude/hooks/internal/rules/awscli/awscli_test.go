package awscli

import (
	"testing"

	"claude-hooks/internal/hook"
)

func TestCheck(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		{"plain aws call asks", `aws s3 ls`, "ask"},
		{"absolute path still resolves to aws", `/usr/local/bin/aws s3 ls`, "ask"},
		{"env assignment does not hide the command", `AWS_PROFILE=x aws s3 ls`, "ask"},
		{"nested in a pipeline", `echo x | aws sts get-caller-identity`, "ask"},
		{"nested in a subshell", `(aws s3 ls)`, "ask"},
		{"behind &&", `true && aws s3 ls`, "ask"},
		{"inside a command substitution", `echo $(aws sts get-caller-identity)`, "ask"},
		{"sudo does not hide the command", `sudo aws s3 ls`, "ask"},
		{"sudo flags are stepped over", `sudo -u deploy aws s3 ls`, "ask"},
		{"env wrapper does not hide the command", `env AWS_PROFILE=x aws s3 ls`, "ask"},
		{"backslash quoting does not hide the command", `\aws s3 ls`, "ask"},
		{"unrelated command is allowed", `echo hi`, "allow"},
		{"sudo of something else is allowed", `sudo ls -la`, "allow"},
		{"aws as an argument is not a call", `echo aws`, "allow"},
		{"a command merely prefixed with aws is not aws", `awslogs get x`, "allow"},
		{"unparseable command is allowed", `echo "unterminated`, "allow"},
		{"empty command is allowed", ``, "allow"},

		// Loopback endpoints cannot reach an AWS account.
		{"loopback endpoint flag is allowed", `aws --endpoint-url http://127.0.0.1:4566 sqs list-queues`, "allow"},
		{"localhost endpoint flag after the command is allowed", `aws sqs list-queues --endpoint-url http://localhost:4566`, "allow"},
		{"endpoint flag with equals is allowed", `aws s3 ls --endpoint-url=http://s3.localhost:4566`, "allow"},
		{"ipv6 loopback is allowed", `aws sts get-caller-identity --endpoint-url http://[::1]:4566`, "allow"},
		{"remote endpoint flag asks", `aws s3 ls --endpoint-url https://s3.amazonaws.com`, "ask"},
		{"host merely starting with localhost asks", `aws s3 ls --endpoint-url http://localhost.evil.com`, "ask"},
		{"endpoint flag from a variable asks", `aws s3 ls --endpoint-url "$EP"`, "ask"},
		{"endpoint flag without a value asks", `aws s3 ls --endpoint-url`, "ask"},
		{"flag outranks a remote env endpoint", `AWS_ENDPOINT_URL=https://x.example.com aws s3 ls --endpoint-url http://127.0.0.1:1`, "allow"},
		{"loopback prefix assignment is allowed", `AWS_ENDPOINT_URL=http://127.0.0.1:4566 aws sqs list-queues`, "allow"},
		{"loopback env wrapper is allowed", `env AWS_ENDPOINT_URL=http://localhost:4566 aws sqs list-queues`, "allow"},
		{"remote prefix assignment asks", `AWS_ENDPOINT_URL=https://sqs.us-east-1.amazonaws.com aws sqs list-queues`, "ask"},
		{"export then call is allowed", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; aws sqs list-queues`, "allow"},
		{"export && call is allowed", `export AWS_ACCESS_KEY_ID=test AWS_ENDPOINT_URL=http://localhost:4566 && aws s3 ls && aws sqs list-queues`, "allow"},
		{"call before the export asks", `aws s3 ls; export AWS_ENDPOINT_URL=http://127.0.0.1:4566`, "ask"},
		{"export inside a subshell does not count", `(export AWS_ENDPOINT_URL=http://127.0.0.1:4566); aws s3 ls`, "ask"},
		{"export from a variable asks", `export AWS_ENDPOINT_URL=$EP; aws s3 ls`, "ask"},
		{"remote service endpoint poisons the env route", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566 AWS_ENDPOINT_URL_S3=https://s3.amazonaws.com; aws s3 ls`, "ask"},
		{"loopback service endpoint keeps the env route", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566 AWS_ENDPOINT_URL_S3=http://localhost:4566; aws s3 ls`, "allow"},
		{"ignoring configured endpoints asks", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; AWS_IGNORE_CONFIGURED_ENDPOINT_URLS=true aws s3 ls`, "ask"},
		{"unset after export asks", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; unset AWS_ENDPOINT_URL; aws s3 ls`, "ask"},
		{"one remote call among local ones asks", `aws sqs list-queues --endpoint-url http://127.0.0.1:1 && aws s3 ls`, "ask"},
		{"non-http scheme asks", `aws s3 ls --endpoint-url file://127.0.0.1/x`, "ask"},

		// Version and help print locally.
		{"version is allowed", `aws --version`, "allow"},
		{"help is allowed", `aws help`, "allow"},
		{"service help is allowed", `aws s3 help`, "allow"},
		{"command help is allowed", `aws s3api put-object help`, "allow"},
		{"help after a real argument asks", `aws s3 rm s3://bucket/key help`, "ask"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(hook.NewRequest("Bash", "", "", tc.cmd)).Decision.String()
			if got != tc.decision {
				t.Errorf("Check(%q) = %s, want %s", tc.cmd, got, tc.decision)
			}
		})
	}
}

func TestHookEnvironment(t *testing.T) {
	const flagCmd = `aws s3 ls --endpoint-url http://127.0.0.1:4566`
	const envCmd = `AWS_ENDPOINT_URL=http://127.0.0.1:4566 aws s3 ls`

	t.Run("remote service endpoint poisons the env route", func(t *testing.T) {
		t.Setenv("AWS_ENDPOINT_URL_S3", "https://s3.amazonaws.com")
		if got := Check(hook.NewRequest("Bash", "", "", envCmd)).Decision; got != hook.Ask {
			t.Errorf("Check(%q) = %s, want ask", envCmd, got)
		}
		if got := Check(hook.NewRequest("Bash", "", "", flagCmd)).Decision; got != hook.Allow {
			t.Errorf("Check(%q) = %s, want allow: the flag outranks the environment", flagCmd, got)
		}
	})

	t.Run("ignoring configured endpoints poisons the env route", func(t *testing.T) {
		t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true")
		if got := Check(hook.NewRequest("Bash", "", "", envCmd)).Decision; got != hook.Ask {
			t.Errorf("Check(%q) = %s, want ask", envCmd, got)
		}
	})
}

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "aws s3 ls")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
