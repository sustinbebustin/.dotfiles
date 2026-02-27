---
agent: triage
description: "Perform focused triage analysis on listed files."
---

Analyzes the provided file set and returns a routing plan with risk scoring.

**Request:** $ARGUMENTS

**Process:**
1. Review the files against the provided summary.
2. Identify risk drivers and scope size.
3. Produce a triage routing plan for specialists.

**Syntax:**
```bash
/triage-review "<summary>" --files "<file1>,<file2>"
```

**Parameters:**
- `<summary>`: Short description of the change set and intent.
- `--files`: Comma-separated list of files or globs to review.

**Options:**
- `--files`: Required. Use quotes if the list contains spaces.

**Examples:**
```bash
# Example 1: Single file
/triage-review "Tighten auth checks" --files "backend/src/app/middleware.go"

# Example 2: UI updates
/triage-review "Refresh proposal header layout" --files "frontend/src/components/proposals/proposal-header.tsx,frontend/src/components/proposals/proposal-layout.tsx"

# Example 3: Mixed scope
/triage-review "Update financing copy" --files "frontend/src/app/(public)/proposal/page.tsx,frontend/src/components/proposals/financing-copy.tsx"
```

**Output:**
```yaml
triage_result:
  scope: "large"
  risk_level: "high"
  flags:
    architecture: true
    implementation: true
    security: false
    testing: true
  rationale:
    - "Introduces new module and refactors core workflow."
    - "Tests modified without new coverage for edge cases."
```

**Notes:**
- Focus on classification and routing, not deep critique.
