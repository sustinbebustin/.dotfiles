---
agent: synthesis
description: "Combine triage and findings into a final pre-PR review summary."
---

Generates a synthesis report from triage output and supplemental findings.

**Request:** $ARGUMENTS

**Process:**
1. Load triage report and findings artifacts.
2. Summarize critical issues and recommendations.
3. Produce the final synthesis review summary.

**Syntax:**
```bash
/synthesize-review --triage "<triage file>" --findings "<findings file>"
```

**Parameters:**
- `--triage`: Path to the triage report file.
- `--findings`: Path to supplemental findings or notes.

**Options:**
- `--triage`: Required. Must be a readable file path.
- `--findings`: Required. Must be a readable file path.

**Examples:**
```bash
# Example 1: Standard synthesis
/synthesize-review --triage ".opencode/reviews/triage-review_456.md" --findings ".opencode/reviews/findings_456.md"

# Example 2: Alternative findings source
/synthesize-review --triage ".opencode/reviews/triage-review_789.md" --findings "notes/pre-pr-findings.md"

# Example 3: Multiple artifacts
/synthesize-review --triage "reviews/triage-review_latest.md" --findings "reviews/manual-findings.md"
```

**Output:**
```yaml
pre_pr_review:
  summary: "Large refactor with missing tests and a security risk in token handling."
  risks:
    - severity: "high"
      area: "security"
      detail: "Token stored in client-readable storage."
      recommendation: "Move token to HttpOnly cookie."
  overall_recommendation: "approve-with-changes"
```

**Notes:**
- Ensure triage and findings files use consistent terminology.
- Highlight blocking issues clearly for final approval.
