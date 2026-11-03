package awscli

import (
	"strings"
	"testing"

	"agent-hooks/internal/config"
	"agent-hooks/internal/hook"
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
		{"looking aws up is not a call", `w=$(command -v aws); readlink -f "$w"`, "allow"},
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
		{"env -u drops the exported endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env -u AWS_ENDPOINT_URL aws s3 ls`, "ask"},
		{"env -i drops the exported endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env -i aws s3 ls`, "ask"},
		{"env -i in a cluster drops the exported endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env -iv aws s3 ls`, "ask"},
		{"env - drops the exported endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env - aws s3 ls`, "ask"},
		{"exec -c drops the exported endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; exec -c aws s3 ls`, "ask"},
		{"env -S hides what it drops", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env -S '-u AWS_ENDPOINT_URL' aws s3 ls`, "ask"},
		{"env -u after a loopback prefix drops it", `AWS_ENDPOINT_URL=http://127.0.0.1:4566 env -u AWS_ENDPOINT_URL aws s3 ls`, "ask"},
		{"inner env -u drops an outer env's endpoint", `env AWS_ENDPOINT_URL=http://127.0.0.1:4566 env -u AWS_ENDPOINT_URL aws s3 ls`, "ask"},
		{"partly variable env option drops the endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; X=unset=AWS_ENDPOINT_URL; env "--$X" aws s3 ls`, "ask"},
		{"partly variable env long option drops the endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env --$X aws s3 ls`, "ask"},
		{"env -u of another variable keeps the endpoint", `export AWS_ENDPOINT_URL=http://127.0.0.1:4566; env -u FOO aws s3 ls`, "allow"},
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

// clearAWSEnv empties every variable the rule reads from its own environment,
// so a developer's shell cannot change a test's outcome. An empty value counts
// as unset.
func clearAWSEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN",
	} {
		t.Setenv(k, "")
	}
}

var testPolicies = map[string]config.AWSPolicy{
	"rec":     config.AWSAllow,
	"work":    config.AWSReadOnly,
	"prod":    config.AWSDeny,
	"careful": config.AWSAsk,
}

func checkWithPolicies(cmd string) hook.Verdict {
	req := hook.NewRequest("Bash", "", "", cmd)
	req.Config = config.Config{AWSProfiles: testPolicies}
	return Check(req)
}

func TestProfilePolicies(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		// Policies.
		{"allow runs a write", `aws --profile rec sqs delete-queue --queue-url q`, "allow"},
		{"readOnly runs a read", `aws --profile work sqs get-queue-attributes --queue-url q`, "allow"},
		{"readOnly asks for a write", `aws --profile work sqs send-message --queue-url q --message-body b`, "ask"},
		{"deny blocks", `aws --profile prod s3 ls`, "deny"},
		{"ask asks", `aws --profile careful s3 ls`, "ask"},
		{"unlisted profile asks", `aws --profile other s3 ls`, "ask"},
		{"no profile is default, which is unlisted", `aws s3 ls`, "ask"},
		{"loopback outranks deny", `aws --profile prod s3 ls --endpoint-url http://127.0.0.1:1`, "allow"},
		{"remote endpoint flag falls to the policy", `aws --profile rec s3 ls --endpoint-url https://x.example.com`, "allow"},

		// Where the profile comes from.
		{"--profile=name form", `aws --profile=rec sqs delete-queue --queue-url q`, "allow"},
		{"last --profile wins", `aws --profile rec --profile prod s3 ls`, "deny"},
		{"prefix assignment", `AWS_PROFILE=rec aws sqs delete-queue --queue-url q`, "allow"},
		{"env wrapper", `env AWS_PROFILE=rec aws sqs delete-queue --queue-url q`, "allow"},
		{"earlier export", `export AWS_PROFILE=rec; aws sqs delete-queue --queue-url q`, "allow"},
		{"export && call", `export AWS_PROFILE=rec && aws sqs delete-queue --queue-url q`, "allow"},
		{"latest export before the call", `export AWS_PROFILE=prod; export AWS_PROFILE=rec; aws s3 ls`, "allow"},
		{"export after the call does not count", `aws s3 ls; export AWS_PROFILE=rec`, "ask"},
		{"prefix on another call does not count", `AWS_PROFILE=rec true; aws s3 ls`, "ask"},
		{"flag outranks the prefix", `AWS_PROFILE=rec aws --profile prod s3 ls`, "deny"},

		// Profile unknown, so ask.
		{"bare assignment", `AWS_PROFILE=rec; aws sqs delete-queue --queue-url q`, "ask"},
		{"export inside a subshell", `(export AWS_PROFILE=rec); aws s3 ls`, "ask"},
		{"export from a variable", `export AWS_PROFILE=$P; aws s3 ls`, "ask"},
		{"prefix from a variable", `AWS_PROFILE=$P aws s3 ls`, "ask"},
		{"--profile from an unknown variable", `aws --profile "$P" s3 ls`, "ask"},
		{"unset", `export AWS_PROFILE=rec; unset AWS_PROFILE; aws s3 ls`, "ask"},
		{"eval", `export AWS_PROFILE=rec; eval "$X"; aws s3 ls`, "ask"},
		{"default profile variable", `AWS_DEFAULT_PROFILE=rec aws s3 ls`, "ask"},
		{"config file redirect beats --profile", `AWS_CONFIG_FILE=/tmp/c aws --profile rec s3 ls`, "ask"},
		{"credentials beat AWS_PROFILE", `AWS_ACCESS_KEY_ID=k AWS_PROFILE=rec aws s3 ls`, "ask"},
		{"credentials do not beat --profile", `AWS_ACCESS_KEY_ID=k aws --profile rec sqs delete-queue --queue-url q`, "allow"},
		{"sudo hides the profile", `sudo aws --profile rec s3 ls`, "ask"},
		{"env -i drops AWS_PROFILE", `env -i AWS_PROFILE=rec aws s3 ls`, "ask"},
		{"env -i in a cluster drops AWS_PROFILE", `export AWS_PROFILE=rec; env -iv aws s3 ls`, "ask"},
		{"env - drops AWS_PROFILE", `export AWS_PROFILE=rec; env - aws s3 ls`, "ask"},
		{"exec -c drops AWS_PROFILE", `export AWS_PROFILE=rec; exec -c aws s3 ls`, "ask"},
		{"env -u AWS_PROFILE", `export AWS_PROFILE=rec; env -u AWS_PROFILE aws s3 ls`, "ask"},
		{"env -u AWS_PROFILE in a cluster", `export AWS_PROFILE=rec; env -vu AWS_PROFILE aws s3 ls`, "ask"},
		{"env -u HOME moves ~/.aws", `env -u HOME aws --profile rec s3 ls`, "ask"},
		{"env -u of an unknown name", `env -u "$V" aws --profile rec s3 ls`, "ask"},
		{"env -S hides its effect", `env -S 'x' aws --profile rec s3 ls`, "ask"},
		{"unknown env flag", `env --frobnicate aws --profile rec s3 ls`, "ask"},
		{"partly variable env option hides an unset", `export AWS_PROFILE=rec; X=unset=AWS_PROFILE; env "--$X" aws sqs delete-queue --queue-url q`, "ask"},
		{"env -u of an unrelated variable", `env -u AWS_ENDPOINT_URL aws sts get-caller-identity --profile rec`, "allow"},
		{"env --unset= of an unrelated variable", `env --unset=AWS_ENDPOINT_URL aws --profile rec sqs delete-queue --queue-url q`, "allow"},
		{"env -u attached of an unrelated variable", `env -uAWS_ENDPOINT_URL -u FOO aws --profile rec s3 ls`, "allow"},
		{"unquoted variable could add --profile", `aws --profile rec s3 ls $EXTRA`, "ask"},
		{"quoted unknown operand could be --profile=x", `aws --profile rec s3 cp "$F" s3://b/k`, "ask"},
		{"quoted unknown after literal text is fine", `aws --profile rec s3 cp "./$F" s3://b/k`, "allow"},
		{"ANSI-C quoting could spell --profile", `aws --profile rec s3 rb s3://b $'\x2d-profile=prod'`, "ask"},
		{"ANSI-C quoting before a glob could spell --profile", `aws --profile rec s3 rb s3://b $'\x2d'*`, "ask"},
		{"leading tilde expands HOME", `HOME=--profile=prod; aws --profile rec s3 rb s3://b ~`, "ask"},
		{"zsh extended glob could match --profile=x", `aws --profile rec s3 ls ^x`, "ask"},
		{"tilde inside a word stays literal", `aws --profile rec s3 cp s3://b/~x .`, "allow"},
		{"unquoted variable inside a word could add --profile", `aws --profile work s3 ls ${X}z`, "ask"},
		{"unquoted substitution could add --profile", `aws --profile work s3 ls $(cat f)`, "ask"},
		{"unquoted substitution inside a word could add --profile", `aws --profile work s3 ls s3://b/$(cat f)`, "ask"},
		{"unquoted glob could match a file named --profile=x", `aws --profile work s3 ls --*`, "ask"},
		{"quoted substitution is one argument", `aws --profile work s3 ls "s3://b/$(cat f)"`, "allow"},
		{"brace list could add --profile", `aws --profile work s3 ls --{profile=rec,x}`, "ask"},
		{"glob after literal text cannot make an option", `aws --profile work s3 ls s3://b/*`, "allow"},
		{"quoted glob is one argument", `aws --profile work logs describe-log-groups --query 'logGroups[?x]'`, "allow"},

		// An expansion in an option's value slot is denied with a rewrite hint
		// when `--opt="$X"` would pass; anything else keeps asking.
		{"loop variable as a value", `for f in -a; do aws --profile work lambda get-function --function-name $f; done`, "deny"},
		{"quoted variable as a value", `aws --profile work lambda get-function --function-name "$f"`, "deny"},
		{"quoted substitution as a value", `aws --profile work logs filter-log-events --log-group-name g --start-time "$(date +%s)"`, "deny"},
		{"unbound variable may be several values", `aws --profile work lambda get-function --function-name $f`, "ask"},
		{"value behind the option is fine", `aws --profile work lambda get-function --function-name="$f"`, "allow"},
		{"rewrite that would still ask", `aws --profile work sqs purge-queue --queue-url $Q`, "ask"},
		{"expansion not after an option", `aws --profile work s3 ls $X`, "ask"},
		{"expansion as the --profile value", `aws --profile $P s3 ls`, "ask"},
		{"expansion after literal text", `aws --profile work lambda get-function --function-name x$f`, "ask"},
		{"one fixable and one not", `aws --profile work lambda get-function --function-name $f $X`, "ask"},
		{"rewrite under an unlisted profile", `aws lambda get-function --function-name $f`, "ask"},
		{"arithmetic as a value", `S=$(( 1 * 1000 )); aws --profile work logs filter-log-events --log-group-name g --start-time $S`, "deny"},
		{"unquoted unknown variable may be several values", `aws --profile work ec2 describe-instances --instance-ids $IDS`, "ask"},
		{"unquoted substitution may be several values", `aws --profile work ec2 describe-instances --instance-ids $(cat ids)`, "ask"},
		{"quoted substitution as a value", `aws --profile work ec2 describe-instances --instance-ids "$(cat id)"`, "deny"},
		{"array as a value", `aws --profile work ec2 describe-instances --instance-ids "${ids[@]}"`, "ask"},
		{"positional parameters as a value", `aws --profile work lambda get-function --function-name $@`, "ask"},
		{"boolean global flag", `aws --profile work --no-paginate "$X" sqs list-queues`, "ask"},
		{"s3 boolean flag", `aws --profile work s3 ls --recursive "$P"`, "ask"},
		{"logs tail positional", `aws --profile work logs tail --follow "$G"`, "ask"},
		{"double dash is not an option", `aws --profile work s3 cp -- "$f" s3://b/`, "ask"},

		// A for-loop variable over literal items, read inside the loop body,
		// expands to those items' words, so none can be an option.
		{"loop items are operations", `export AWS_PROFILE=rec; for c in "sns list-topics" "ses get-send-quota"; do aws $c --max-items 1; done`, "allow"},
		{"loop item in a substitution in the body", `for c in "sns list-topics" ses; do r=$(aws --profile rec $c 2>&1); done`, "allow"},
		{"loop variable as a value is proven", `for f in a b; do aws --profile work lambda get-function --function-name $f; done`, "allow"},
		{"loop items before the operation leave readOnly unsure", `for c in "sns list-topics"; do aws --profile work $c; done`, "ask"},
		{"loop item word could be --profile", `for c in "s3 ls --profile=prod"; do aws --profile rec $c; done`, "ask"},
		{"loop item could be --endpoint-url", `for c in "--endpoint-url=http://127.0.0.1:1 s3 ls"; do aws --profile prod $c; done`, "ask"},
		{"loop item glob could match --profile=x", `for c in 's3 *'; do aws --profile rec $c; done`, "ask"},
		{"loop item brace list could add --profile", `for c in x{,--profile=prod}; do aws --profile rec s3 ls $c; done`, "ask"},
		{"ANSI-C loop item spells a dash", `for c in $'\x2d-profile=prod'; do aws --profile rec s3 ls $c; done`, "ask"},

		// Bindings the census must see, or it must give up on every variable.
		{"printf -v into an element", `for c in a; do printf -v 'c[0]' %s --profile=prod; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"read into an element", `for c in a; do read 'c[0]' <<< --profile=prod; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"printf -v into a variable name", `n=c; for c in a; do printf -v "$n" %s --profile=prod; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"quoted declare assignment", `for c in a; do declare "c=--profile=prod"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"quoted export assignment", `for c in a; do export "c=--profile=prod"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"quoted typeset assignment", `for c in a; do typeset "c=--profile=prod"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"declare from a variable", `for c in a; do declare "$X"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"indirect default assignment", `n=c; for c in a; do unset "$n"; : "${!n:=--profile=prod}"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"zsh set -A", `for c in a; do set -A c --profile=prod; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"zsh unconditional assignment", `for c in a; do : ${c::=--profile=prod}; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"quoted declare of IFS", `for c in ax--profile=prod; do declare "IFS=x"; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"printf -v of IFS by name", `n=IFS; for c in ax--profile=prod; do printf -v "$n" x; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"let assigns", `for c in a; do let c=-1; aws --profile rec s3 rb s3://b $c; done`, "ask"},
		{"rebinding past a loopback endpoint", `for c in a; do declare "c=--endpoint-url=https://x.example.com"; aws --profile prod --endpoint-url http://localhost:4566 s3 rb s3://b $c; done`, "ask"},
		{"rebinding under readOnly", `for c in a; do declare "c=--profile=prod"; aws --profile work sqs list-queues $c; done`, "ask"},
		{"quoted declare of an exported variable", `X=a; declare "X=--profile=prod"; aws --profile rec s3 rb s3://b $X`, "ask"},
		{"printf still allows when it binds nothing watched", `for c in a; do printf '%s\n' "$c"; aws --profile rec s3 ls $c; done`, "allow"},
		{"loop item from a variable", `for c in "$X"; do aws --profile rec s3 ls $c; done`, "ask"},
		{"loop over positional parameters", `for c; do aws --profile rec s3 ls $c; done`, "ask"},
		{"select is not a loop over its items", `select c in a; do aws --profile rec s3 ls $c; done`, "ask"},
		{"call after the loop", `for c in a; do :; done; aws --profile rec s3 ls $c`, "ask"},
		{"call before the loop", `aws --profile rec s3 ls $c; for c in a; do :; done`, "ask"},
		{"loop never runs, so the call is outside it", `false && for c in a; do :; done; aws --profile rec s3 ls $c`, "ask"},
		{"body reassigns the variable", `for c in a; do c=--profile=prod; aws --profile rec s3 ls $c; done`, "ask"},
		{"body reads into the variable", `for c in a; do read c; aws --profile rec s3 ls $c; done`, "ask"},
		{"two loops bind the variable", `for c in a; do aws --profile rec s3 ls $c; done; for c in --profile=prod; do :; done`, "ask"},
		{"IFS set anywhere", `IFS=x; for c in ax--profile=prod; do aws --profile rec s3 ls $c; done`, "ask"},
		{"modified expansion", `for c in a; do aws --profile rec s3 ls ${c:-"--profile=prod"}; done`, "ask"},
		{"eval can bind anything", `for c in a; do eval "$X"; aws --profile rec s3 ls $c; done`, "ask"},

		// Words that expand to several arguments even inside double quotes.
		{"quoted array after literal text", `ids=(i-1 --profile=prod); aws --profile work ec2 describe-instances --instance-ids="${ids[@]}"`, "ask"},
		{"quoted $@ after literal text", `aws --profile work s3 ls s3://b/x"$@"`, "ask"},
		{"any quoted subscript is refused", `aws --profile work ec2 describe-instances --instance-ids="${ids[0]}"`, "ask"},
		{"quoted scalar after literal text is one word", `aws --profile work ec2 describe-instances --instance-ids="$id"`, "allow"},

		// Variables the shell rebinds out of the census's sight are not known.
		{"literal variable is known", `X=a; aws --profile rec s3 ls $X`, "allow"},
		{"$_ is rebound by every command", `_=a; : --profile=prod; aws --profile rec s3 ls $_`, "ask"},
		{"nameref writes through", `X=a; declare -n r=X; r=--profile=prod; aws --profile rec s3 ls $X`, "ask"},
		{"printf -v attached", `X=a; printf -vX %s --profile=prod; aws --profile rec s3 ls $X`, "ask"},
		{"zsh print -v", `X=a; print -v X -- --profile=prod; aws --profile rec s3 ls $X`, "ask"},
		{"zsh vared", `X=a; vared X; aws --profile rec s3 ls $X`, "ask"},

		// The last --endpoint-url wins, as in the CLI.
		{"later remote endpoint wins", `aws --profile prod --endpoint-url http://localhost:4566 s3 rb s3://b --endpoint-url https://s3.amazonaws.com`, "deny"},
		{"expansion could add a remote endpoint", `aws --profile prod --endpoint-url http://localhost:4566 s3 rb s3://b $X`, "ask"},
		{"later local endpoint wins", `aws --profile prod --endpoint-url https://s3.amazonaws.com s3 ls --endpoint-url http://127.0.0.1:1`, "allow"},

		// Read-only boundaries under readOnly.
		{"describe", `aws --profile work ecs describe-services --cluster c`, "allow"},
		{"list", `aws --profile work sqs list-queues`, "allow"},
		{"head", `aws --profile work s3api head-object --bucket b --key k`, "allow"},
		{"filter-log-events", `aws --profile work logs filter-log-events --log-group-name g`, "allow"},
		{"s3 ls", `aws --profile work s3 ls s3://b`, "allow"},
		{"global flags before the service", `aws --profile work --region us-west-1 --output json --no-cli-pager sqs list-queues`, "allow"},
		{"get-secret-value asks", `aws --profile work secretsmanager get-secret-value --secret-id s`, "ask"},
		{"ssm get-parameter asks", `aws --profile work ssm get-parameter --name n`, "ask"},
		{"get-object asks", `aws --profile work s3api get-object --bucket b --key k out`, "ask"},
		{"credential minting asks", `aws --profile work ecr get-login-password`, "ask"},
		{"cognito-identity get asks", `aws --profile work cognito-identity get-id --identity-pool-id p`, "ask"},
		{"receive-message asks", `aws --profile work sqs receive-message --queue-url q`, "ask"},
		{"s3 cp asks", `aws --profile work s3 cp s3://b/k .`, "ask"},
		{"unknown flag before the operation asks", `aws --profile work --frobnicate x sqs list-queues`, "ask"},
		{"variable before the operation asks", `aws --profile work "$SVC" list-queues`, "ask"},
		{"service with no operation asks", `aws --profile work sqs`, "ask"},

		// Every call is judged.
		{"deny in a pipeline beats an allow", `aws --profile rec s3 ls | aws --profile prod s3 ls`, "deny"},
		{"one write among reads asks", `aws --profile work sqs list-queues && aws --profile work sqs purge-queue --queue-url q`, "ask"},

		// Commands that make no request.
		{"configure list", `aws configure list`, "allow"},
		{"configure list-profiles", `aws configure list-profiles`, "allow"},
		{"configure get still asks", `aws configure get aws_secret_access_key`, "ask"},
	}

	clearAWSEnv(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkWithPolicies(tc.cmd)
			if got.Decision.String() != tc.decision {
				t.Errorf("Check(%q) = %s (%s), want %s", tc.cmd, got.Decision, got.Reason, tc.decision)
			}
		})
	}
}

func TestProfileFromHookEnvironment(t *testing.T) {
	const write = `aws sqs delete-queue --queue-url q`

	t.Run("inherited AWS_PROFILE applies", func(t *testing.T) {
		clearAWSEnv(t)
		t.Setenv("AWS_PROFILE", "rec")
		if got := checkWithPolicies(write).Decision; got != hook.Allow {
			t.Errorf("Check(%q) = %s, want allow", write, got)
		}
	})

	t.Run("an export in the command outranks it", func(t *testing.T) {
		clearAWSEnv(t)
		t.Setenv("AWS_PROFILE", "rec")
		cmd := `export AWS_PROFILE=prod; ` + write
		if got := checkWithPolicies(cmd).Decision; got != hook.Deny {
			t.Errorf("Check(%q) = %s, want deny", cmd, got)
		}
	})

	t.Run("inherited credentials make AWS_PROFILE untrustworthy", func(t *testing.T) {
		clearAWSEnv(t)
		t.Setenv("AWS_PROFILE", "rec")
		t.Setenv("AWS_ACCESS_KEY_ID", "k")
		if got := checkWithPolicies(write).Decision; got != hook.Ask {
			t.Errorf("Check(%q) = %s, want ask", write, got)
		}
	})

	t.Run("inherited config file redirect", func(t *testing.T) {
		clearAWSEnv(t)
		t.Setenv("AWS_CONFIG_FILE", "/tmp/elsewhere")
		cmd := `aws --profile rec s3 ls`
		if got := checkWithPolicies(cmd).Decision; got != hook.Ask {
			t.Errorf("Check(%q) = %s, want ask", cmd, got)
		}
	})
}

// TestValueSlotHintSpellsTheRewrite pins that the denial gives the agent the
// exact replacement for each expansion, so it can fix the call itself.
func TestValueSlotHintSpellsTheRewrite(t *testing.T) {
	clearAWSEnv(t)
	cases := []struct{ cmd, want string }{
		{`for f in a -b; do aws --profile work lambda get-function --function-name $f; done`, `--function-name="$f"`},
		{`S=$(( 1 )); aws --profile work logs filter-log-events --log-group-name g --start-time $S`, `--start-time="$S"`},
		{`aws --profile work lambda get-function --function-name "$f"`, `--function-name="$f"`},
		{`aws --profile work logs filter-log-events --log-group-name g --start-time "$(date +%s)"`, `--start-time="$(date +%s)"`},
	}
	for _, tc := range cases {
		got := checkWithPolicies(tc.cmd)
		if got.Decision != hook.Deny || !strings.Contains(got.Reason, tc.want) {
			t.Errorf("Check(%q) = %s (%s), want deny naming %s", tc.cmd, got.Decision, got.Reason, tc.want)
		}
	}
}

func TestReasonsNameTheProfile(t *testing.T) {
	clearAWSEnv(t)
	cases := []struct{ cmd, want string }{
		{`aws --profile work sqs send-message --queue-url q`, `"sqs send-message" is not one`},
		{`aws --profile other s3 ls`, `profile "other", which has no policy`},
		{`aws --profile prod s3 ls`, `which the hook config denies`},
		{`AWS_DEFAULT_PROFILE=x aws s3 ls`, `cannot be determined: AWS_DEFAULT_PROFILE is set`},
	}
	for _, tc := range cases {
		if got := checkWithPolicies(tc.cmd).Reason; !strings.Contains(got, tc.want) {
			t.Errorf("Check(%q).Reason = %q, want it to contain %q", tc.cmd, got, tc.want)
		}
	}
}

func TestEndpointThroughVariable(t *testing.T) {
	cases := []struct {
		name     string
		cmd      string
		decision string
	}{
		{"loopback in a variable", `E=http://127.0.0.1:1; aws --endpoint-url $E s3 ls`, "allow"},
		{"quoted and braced", `E=http://localhost:1; aws s3 ls --endpoint-url "${E}"`, "allow"},
		{"equals form", `E=http://127.0.0.1:1; aws s3 ls --endpoint-url=$E`, "allow"},
		{"exported", `export E=http://127.0.0.1:1 && aws s3 ls --endpoint-url $E`, "allow"},
		{"flags held in a variable", `E="--endpoint-url http://127.0.0.1:1 --no-cli-pager"; aws sqs list-queues $E`, "allow"},
		{"used inside a subshell", `E=http://127.0.0.1:1; (aws --endpoint-url $E s3 ls)`, "allow"},
		{"remote in a variable", `E=https://s3.amazonaws.com; aws --endpoint-url $E s3 ls`, "ask"},
		{"assigned twice", `E=http://127.0.0.1:1; E=https://x.example.com; aws --endpoint-url $E s3 ls`, "ask"},
		{"assigned inside a subshell", `(E=http://127.0.0.1:1); aws --endpoint-url $E s3 ls`, "ask"},
		{"assigned in a conditional too", `E=http://127.0.0.1:1; if x; then E=https://x.example.com; fi; aws --endpoint-url $E s3 ls`, "ask"},
		{"from a substitution", `E=$(cmd); aws --endpoint-url $E s3 ls`, "ask"},
		{"assigned after the call", `aws --endpoint-url $E s3 ls; E=http://127.0.0.1:1`, "ask"},
		{"read can change it", `E=http://127.0.0.1:1; read E; aws --endpoint-url $E s3 ls`, "ask"},
		{"unset", `E=http://127.0.0.1:1; unset E; aws --endpoint-url $E s3 ls`, "ask"},
		{"eval could change it", `E=http://127.0.0.1:1; eval "$X"; aws --endpoint-url $E s3 ls`, "ask"},
		{"IFS changed", `IFS=:; E="--endpoint-url http://127.0.0.1:1"; aws s3 ls $E`, "ask"},
		{"prefix assignment is not the shell's value", `E=http://127.0.0.1:1 aws --endpoint-url $E s3 ls`, "ask"},
		{"modifier is refused", `E=http://127.0.0.1:1; aws --endpoint-url ${E:-https://x.example.com} s3 ls`, "ask"},
	}

	clearAWSEnv(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(hook.NewRequest("Bash", "", "", tc.cmd))
			if got.Decision.String() != tc.decision {
				t.Errorf("Check(%q) = %s (%s), want %s", tc.cmd, got.Decision, got.Reason, tc.decision)
			}
		})
	}
}

func TestNonBashIsIgnored(t *testing.T) {
	if got := Check(hook.NewRequest("Read", "", "", "aws s3 ls")); got.Decision != hook.Allow {
		t.Fatalf("Read payload = %q, want allow", got.Decision)
	}
}
