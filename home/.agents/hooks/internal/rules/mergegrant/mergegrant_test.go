package mergegrant

import (
	"testing"

	"agent-hooks/internal/hook"
)

func TestCheck(t *testing.T) {
	const store = "/home/u/.local/state/agent-guard-grants"
	cases := []struct {
		name     string
		req      *hook.Request
		decision string
	}{
		{"write a grant", hook.NewRequest("Write", store+"/abc", "", ""), "ask"},
		{"edit a grant", hook.NewRequest("Edit", store+"/abc", "", ""), "ask"},
		{"write elsewhere", hook.NewRequest("Write", "/home/u/notes.md", "", ""), "allow"},
		// Reading the store shows only which sessions hold a grant.
		{"read a grant", hook.NewRequest("Read", store+"/abc", "", ""), "allow"},

		{"touch a grant", bash(`touch ` + store + `/abc`), "ask"},
		{"redirect into the store", bash(`: > ~/.local/state/agent-guard-grants/abc`), "ask"},
		{"store reached through a variable", bash(`d=~/.local/state/agent-guard-grants; touch "$d/abc"`), "ask"},
		{"store named in a nested script", bash(`bash -c 'touch ~/.local/state/agent-guard-grants/abc'`), "ask"},
		{"store named in a command the parser rejects", bash(`touch ` + store + `/abc; echo ${=x`), "ask"},

		{"run the minter", bash(`~/.agents/hooks/agent-guard-bin grant`), "ask"},
		{"pipe a payload to the minter", bash(`echo '{}' | ./agent-guard-bin grant`), "ask"},
		{"minter behind a wrapper", bash(`env X=1 agent-guard-bin grant`), "ask"},
		{"minter with an expanded mode", bash(`agent-guard-bin "$mode"`), "ask"},
		{"minter in a command the parser rejects", bash(`agent-guard-bin grant; echo ${=x`), "ask"},

		// The guard's other modes mint nothing.
		{"list the rules", bash(`./agent-guard-bin list`), "allow"},
		{"run one rule", bash(`./agent-guard-bin block-dangerous-git`), "allow"},
		{"another command with a grant argument", bash(`echo grant`), "allow"},
		{"unrelated command", bash(`ls ~/.local/state`), "allow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.req)
			if got.Decision.String() != tc.decision {
				t.Fatalf("decision = %q, want %q (reason: %s)", got.Decision, tc.decision, got.Reason)
			}
			if tc.decision == "ask" && got.Reason == "" {
				t.Error("an ask carries no reason")
			}
		})
	}
}

func bash(cmd string) *hook.Request { return hook.NewRequest("Bash", "", "", cmd) }
