package rules

import (
	"strings"
	"testing"

	"claude-hooks/internal/hook"
)

func rule(name string, tools []string, check func(hook.Request) hook.Verdict) Rule {
	return Rule{Name: name, Tools: tools, Check: check}
}

func allow(hook.Request) hook.Verdict { return hook.Allowed() }

// TestApplySkipsRulesThatDoNotMatchTheTool pins per-rule tool filtering: a rule
// that does not list the tool is not consulted, so it cannot contribute a
// verdict. The single settings.json matcher is the union of every rule's tools,
// so rules do see tools they did not ask for.
func TestApplySkipsRulesThatDoNotMatchTheTool(t *testing.T) {
	ran := false
	rs := []Rule{rule("bash-only", []string{"Bash"}, func(hook.Request) hook.Verdict {
		ran = true
		return hook.Denied("should not be reached")
	})}

	got := Apply(rs, hook.NewRequest("Read", "/etc/hosts", "", ""))
	if ran {
		t.Error("a rule ran for a tool it does not list")
	}
	if got.Decision != hook.Allow {
		t.Errorf("decision = %q, want allow", got.Decision)
	}
}

func TestApplyMergesApplicableRules(t *testing.T) {
	rs := []Rule{
		rule("a", []string{"Bash"}, func(hook.Request) hook.Verdict { return hook.Asked("ask-reason") }),
		rule("b", []string{"Bash"}, allow),
		rule("c", []string{"Bash"}, func(hook.Request) hook.Verdict { return hook.Denied("deny-reason") }),
		rule("d", []string{"Read"}, func(hook.Request) hook.Verdict { return hook.Denied("wrong tool") }),
	}

	got := Apply(rs, hook.NewRequest("Bash", "", "", "ls"))
	if got.Decision != hook.Deny {
		t.Fatalf("decision = %q, want deny", got.Decision)
	}
	if got.Reason != "deny-reason" {
		t.Errorf("reason = %q, want only the winning rule's reason", got.Reason)
	}
}

// TestPanickingRuleDeniesAndDoesNotSilenceTheRest is the reason checkSafely
// exists: every rule shares one process, and an unrecovered panic would emit no
// decision at all, which lets the command through unchecked.
func TestPanickingRuleDeniesAndDoesNotSilenceTheRest(t *testing.T) {
	after := false
	rs := []Rule{
		rule("boom", []string{"Bash"}, func(hook.Request) hook.Verdict {
			panic("index out of range")
		}),
		rule("after", []string{"Bash"}, func(hook.Request) hook.Verdict {
			after = true
			return hook.Allowed()
		}),
	}

	got := Apply(rs, hook.NewRequest("Bash", "", "", "ls"))
	if got.Decision != hook.Deny {
		t.Fatalf("decision = %q, want deny -- a guard that crashed must not allow the command", got.Decision)
	}
	if !strings.Contains(got.Reason, "boom") {
		t.Errorf("reason does not name the failing guard: %s", got.Reason)
	}
	if !strings.Contains(got.Reason, "index out of range") {
		t.Errorf("reason does not carry the panic value: %s", got.Reason)
	}
	if !after {
		t.Error("a rule after the panicking one did not run")
	}
}

// TestPanickingRuleLosesToARealDeny keeps the containment message from crowding
// out a guard that actually has something to say.
func TestPanickingRuleLosesToARealDeny(t *testing.T) {
	rs := []Rule{
		rule("boom", []string{"Bash"}, func(hook.Request) hook.Verdict { panic("boom") }),
		rule("real", []string{"Bash"}, func(hook.Request) hook.Verdict { return hook.Denied("[BLOCKED] rm -rf /") }),
	}

	got := Apply(rs, hook.NewRequest("Bash", "", "", "ls"))
	if !strings.Contains(got.Reason, "[BLOCKED] rm -rf /") {
		t.Errorf("the real deny reason was lost: %s", got.Reason)
	}
}

func TestByName(t *testing.T) {
	for _, r := range All() {
		got, ok := ByName(r.Name)
		if !ok {
			t.Errorf("ByName(%q) did not find a registered rule", r.Name)
			continue
		}
		if got.Name != r.Name {
			t.Errorf("ByName(%q) returned %q", r.Name, got.Name)
		}
	}
	if _, ok := ByName("no-such-rule"); ok {
		t.Error("ByName found a rule that is not registered")
	}
}

// TestAllReturnsACopy guards the registry against a caller that reorders or
// overwrites the slice it gets back.
func TestAllReturnsACopy(t *testing.T) {
	got := All()
	if len(got) == 0 {
		t.Fatal("no rules registered")
	}
	original := got[0].Name
	got[0] = Rule{Name: "clobbered"}
	if All()[0].Name != original {
		t.Error("All() exposes the registry; a caller can overwrite it")
	}
}

func TestEveryRuleIsUsable(t *testing.T) {
	for _, r := range All() {
		if r.Name == "" {
			t.Error("a rule has no name")
		}
		if len(r.Tools) == 0 {
			t.Errorf("rule %q lists no tools, so it never runs", r.Name)
		}
		if r.Check == nil {
			t.Errorf("rule %q has no Check", r.Name)
		}
	}
}
