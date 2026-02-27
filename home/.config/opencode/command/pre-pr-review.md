---
agent: pre-pr-review-orchestrator
description: "Coordinate a full pre-PR review across triage and synthesis."
---

Runs an orchestrated pre-PR review by collecting file context, triggering triage, and assembling a synthesis summary. If files or diff are omitted, it derives them from `git diff --name-only main...HEAD` and `git diff --stat main...HEAD`.

**Request:** $ARGUMENTS

**Process:**
1. Validate the summary, file list, and diff context.
2. Route triage work to the triage subagent.
3. Collect specialist findings and generate the synthesis report.

**Syntax:**
```bash
/pre-pr-review "<summary>" --files "<file1>,<file2>" --diff "<diff summary>"
```

**Parameters:**
- `<summary>`: Short description of the change set and intent.
- `--files`: Comma-separated list of files or globs to review.
- `--diff`: High-level summary of the diff or primary changes.

**Options:**
- `--files`: Optional. If omitted, derived from `git diff --name-only main...HEAD`.
- `--diff`: Optional. If omitted, derived from `git diff --stat main...HEAD`.

**Examples:**
```bash
# Example 1: Small change set
/pre-pr-review "Add PDF export for proposal" --files "frontend/src/app/(internal)/proposal/export.tsx" --diff "Adds export action and button UI"

# Example 2: Multiple files
/pre-pr-review "Refactor pricing calculation" --files "frontend/src/hooks/use-proposal-calculations.ts,frontend/src/components/proposals/price-summary.tsx" --diff "Simplifies calculation flow and updates UI totals"

# Example 3: Backend updates
/pre-pr-review "Add webhook validation" --files "backend/src/app/webhooks/handlers.go,backend/src/app/webhooks/service.go" --diff "Adds signature validation and error handling"
```

**Output:**
```yaml
output:
  - block: "review_plan"
    content: { ... }
  - block: "findings"
    content:
      - { ... }
  - block: "final_summary"
    content: { ... }
```

**Notes:**
- Provide a clear summary so the orchestrator can scope the review.
- Keep file lists focused to reduce noise.
