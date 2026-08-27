// Package rules is the registry of guards and the dispatcher that runs them.
//
// The set of guards, the tools each one inspects, and the order they run in are
// all stated here. settings.json only names the binary and the matcher that
// covers every rule (see Matcher), so adding a guard is a change to this file
// and nothing else.
package rules

import (
	"fmt"
	"slices"
	"strings"

	"claude-hooks/internal/hook"
	"claude-hooks/internal/rules/awscli"
	"claude-hooks/internal/rules/credfiles"
	"claude-hooks/internal/rules/dangerousgit"
	"claude-hooks/internal/rules/dangerousrm"
	"claude-hooks/internal/rules/rootcd"
)

// Rule is one guard. Check is pure: it reads the Request, returns a Verdict,
// and does no I/O and no exiting.
type Rule struct {
	// Name identifies the rule on the command line and in `list` output.
	Name string
	// Tools are the tool names this rule inspects. A rule is skipped for any
	// other tool, so it contributes no verdict at all.
	Tools []string
	// Nested reports whether the rule also runs over the scripts the command
	// hands to a nested shell (`bash -c '...'`; see hook.Request.Embedded).
	// It belongs to the rule rather than the dispatcher because a nested shell
	// changes what some guards mean: enforce-root opts out, since a `cd` in a
	// child shell is confined to it and is exactly the case that rule already
	// allows in `( ... )` form.
	Nested bool
	Check  func(*hook.Request) hook.Verdict
}

// all is the registered rule set, in run order. Order is not arbitrary: it
// decides how reasons are joined when several rules land on the same decision.
var all = []Rule{
	{Name: credfiles.Name, Tools: []string{"Read", "Edit", "Write", "Bash", "Grep"}, Nested: true, Check: credfiles.Check},
	{Name: awscli.Name, Tools: []string{"Bash"}, Nested: true, Check: awscli.Check},
	{Name: dangerousgit.Name, Tools: []string{"Bash"}, Nested: true, Check: dangerousgit.Check},
	{Name: dangerousrm.Name, Tools: []string{"Bash"}, Nested: true, Check: dangerousrm.Check},
	{Name: rootcd.Name, Tools: []string{"Bash"}, Check: rootcd.Check},
}

// All returns the registered rules in run order.
func All() []Rule { return slices.Clone(all) }

// ByName returns the rule with the given name.
func ByName(name string) (Rule, bool) {
	for _, r := range all {
		if r.Name == name {
			return r, true
		}
	}
	return Rule{}, false
}

// Matcher is the settings.json "matcher" value covering every registered rule:
// the union of their tools, in first-seen order. The single hook entry must be
// registered with this, or a rule silently stops running.
func Matcher() string {
	var tools []string
	for _, r := range all {
		for _, t := range r.Tools {
			if !slices.Contains(tools, t) {
				tools = append(tools, t)
			}
		}
	}
	return strings.Join(tools, "|")
}

// Applies reports whether r inspects the given tool.
func (r Rule) Applies(toolName string) bool {
	return slices.Contains(r.Tools, toolName)
}

// Apply runs every rule that applies to req and reduces the results to the one
// verdict the binary emits. Rules marked Nested are run again over each script
// the command hands to a nested shell, so a guarded command is caught whether it
// is written out or passed to `bash -c`.
func Apply(rs []Rule, req *hook.Request) hook.Verdict {
	nested := req.Embedded()

	var verdicts []hook.Verdict
	for _, r := range rs {
		if !r.Applies(req.ToolName) {
			continue
		}
		verdicts = append(verdicts, checkSafely(r, req))
		if !r.Nested {
			continue
		}
		for _, sub := range nested {
			verdicts = append(verdicts, markNested(checkSafely(r, sub)))
		}
	}
	return hook.Merge(verdicts)
}

// nestedPrefix opens the reason for a verdict a rule reached inside a nested
// shell's script. Without it the reason describes a command the user cannot
// find in the tool call: the prompt for `bash -c 'rm -rf ~'` would report a
// recursive rm against a command whose visible words contain no rm at all.
const nestedPrefix = "Inside a script passed to a nested shell: "

func markNested(v hook.Verdict) hook.Verdict {
	if v.Decision == hook.Allow {
		return v
	}
	v.Reason = nestedPrefix + v.Reason
	return v
}

// checkSafely runs one rule's Check and converts a panic into a Deny.
//
// Every rule shares this process, so an unrecovered panic would take the whole
// binary down and emit no decision at all -- and a guard that says nothing lets
// the action through unchecked. Containing the panic here means a rule that
// crashes still blocks, and the rules that did not crash still get to speak.
func checkSafely(r Rule, req *hook.Request) (v hook.Verdict) {
	defer func() {
		if p := recover(); p != nil {
			v = hook.Denied(fmt.Sprintf(
				"The %s guard failed while checking this command (%v), so the command is blocked rather "+
					"than allowed unchecked. The other guards ran normally. Re-run to retry; if it repeats, "+
					"the guard has a bug on this input.", r.Name, p,
			))
		}
	}()
	return r.Check(req)
}
