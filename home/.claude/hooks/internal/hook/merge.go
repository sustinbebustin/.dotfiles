package hook

import "strings"

// Merge reduces the rule verdicts to the single verdict the binary emits:
// worst wins. Any deny blocks the call, otherwise any ask prompts, otherwise
// the call is allowed. A guard is only useful if it cannot be outvoted by the
// guards with nothing to say.
//
// When several rules reach the winning decision their reasons are joined, in
// the order the rules are registered, so nothing a rule said is dropped. Every
// reason is a self-contained sentence, so a space is enough to separate them.
//
// An empty slice means no rule applied to this tool, which is an Allow.
func Merge(verdicts []Verdict) Verdict {
	winner := Allow
	for _, v := range verdicts {
		if v.Decision > winner {
			winner = v.Decision
		}
	}
	if winner == Allow {
		return Allowed()
	}

	var reasons []string
	for _, v := range verdicts {
		if v.Decision == winner && v.Reason != "" {
			reasons = append(reasons, v.Reason)
		}
	}
	return Verdict{Decision: winner, Reason: strings.Join(reasons, " ")}
}
