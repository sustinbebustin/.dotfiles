package awscli

import (
	"errors"
	"fmt"
	"strings"
)

// globalValueFlags are the CLI's global options that take a value, so the word
// after them is not the service or operation.
var globalValueFlags = map[string]bool{
	"--profile": true, "--region": true, "--output": true, "--query": true, endpointFlag: true,
	"--color": true, "--ca-bundle": true, "--cli-read-timeout": true, "--cli-connect-timeout": true,
	"--cli-binary-format": true,
}

// globalBoolFlags are the CLI's global options that take no value.
var globalBoolFlags = map[string]bool{
	"--debug": true, "--no-paginate": true, "--no-sign-request": true, "--no-verify-ssl": true,
	"--no-cli-pager": true, "--cli-auto-prompt": true, "--no-cli-auto-prompt": true,
}

// operation is the service and operation an aws call names, such as
// {"sqs", "get-queue-attributes"} or {"s3", "ls"}.
type operation struct {
	service, name string
}

func (o operation) String() string { return o.service + " " + o.name }

// findOperation returns the first two positional words, stepping over global
// options. Anything it cannot read for certain -- an unknown word, an
// unrecognised flag -- before the operation is an error, since it could change
// what runs.
func findOperation(toks []token) (operation, error) {
	var words []string
	for i := 0; i < len(toks) && len(words) < 2; i++ {
		t := toks[i]
		if !t.known {
			return operation{}, errors.New("a variable or substitution comes before the operation, so it is not known")
		}
		switch flag, _, hasValue := strings.Cut(t.text, "="); {
		case globalValueFlags[flag]:
			if !hasValue {
				i++
			}
		case globalBoolFlags[t.text]:
		case strings.HasPrefix(t.text, "-"):
			return operation{}, fmt.Errorf("the option %s comes before the operation, and the hook does not know "+
				"whether it takes a value", t.text)
		default:
			words = append(words, t.text)
		}
	}
	if len(words) < 2 {
		return operation{}, errors.New("no operation was found after the service")
	}
	return operation{service: words[0], name: words[1]}, nil
}

// readOnlyPrefixes start operations that only read. The CLI's naming is
// consistent enough that a prefix is a better list than one maintained by hand
// per service; the exceptions are in sensitiveReads.
var readOnlyPrefixes = []string{"describe-", "list-", "get-", "head-"}

// readOnlyExtras are reads whose names do not start with a read-only prefix.
var readOnlyExtras = map[string]bool{
	"logs filter-log-events": true,
	"logs tail":              true,
	"s3 ls":                  true,
}

// sensitiveReads match a read-only prefix but return secrets, mint
// credentials, or download data, so the readOnly policy still asks for them.
var sensitiveReads = map[string]bool{
	// Secret values.
	"secretsmanager get-secret-value":       true,
	"ssm get-parameter":                     true,
	"ssm get-parameters":                    true,
	"ssm get-parameters-by-path":            true,
	"ssm get-parameter-history":             true,
	"lightsail get-instance-access-details": true,
	"apigateway get-api-key":                true,
	"apigateway get-api-keys":               true,
	// Credentials and tokens.
	"sts get-session-token":                     true,
	"sts get-federation-token":                  true,
	"sso get-role-credentials":                  true,
	"ecr get-login-password":                    true,
	"ecr get-authorization-token":               true,
	"codeartifact get-authorization-token":      true,
	"ec2 get-password-data":                     true,
	"eks get-token":                             true,
	"redshift get-cluster-credentials":          true,
	"redshift get-cluster-credentials-with-iam": true,
	"redshift-serverless get-credentials":       true,
	"connect get-federation-token":              true,
	// Object data, written to a local file.
	"s3api get-object":         true,
	"s3api get-object-torrent": true,
}

// isReadOnly reports whether op only reads AWS state and returns nothing
// secret. Every cognito-identity get-* is excluded: get-id creates an identity
// and get-credentials-for-identity mints credentials.
func isReadOnly(op operation) bool {
	key := op.String()
	if sensitiveReads[key] || (op.service == "cognito-identity" && strings.HasPrefix(op.name, "get-")) {
		return false
	}
	if readOnlyExtras[key] {
		return true
	}
	for _, p := range readOnlyPrefixes {
		if strings.HasPrefix(op.name, p) {
			return true
		}
	}
	return false
}
