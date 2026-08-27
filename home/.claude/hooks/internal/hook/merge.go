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
// A reason already joined is not repeated. One rule now yields several verdicts
// on the same command -- one per nested shell script it inspects -- and the same
// finding in two of them says nothing the first said, while reading as though
// two different things were wrong.
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
	seen := make(map[string]bool, len(verdicts))
	for _, v := range verdicts {
		if v.Decision != winner || v.Reason == "" || seen[v.Reason] {
			continue
		}
		seen[v.Reason] = true
		reasons = append(reasons, v.Reason)
	}
	return Verdict{Decision: winner, Reason: strings.Join(reasons, " ")}
}
