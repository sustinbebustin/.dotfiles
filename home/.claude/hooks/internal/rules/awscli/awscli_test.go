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

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "aws s3 ls")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
