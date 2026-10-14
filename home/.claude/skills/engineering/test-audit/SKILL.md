---
name: test-audit
description: "Writing/changing/reviewing/sweeping tests: reject duplicate or implementation-coupled proof; remove test-only production seams."
metadata:
  author: openclaw
---

# Test Audit

- Authoring gates every new/changed test.
- Audits: remove low-value proof in coherent owner batches; never chase deletion counts.
- Multi-batch audits and whole-subsystem campaigns: run through `/test-audit-stack`,
  which applies these rules one batch per stacked PR.

## Authoring gate

Before adding test, answer:

1. Protected observable behavior, invariant, or independent contract?
2. Credible regression that makes it fail?
3. Why existing proof misses it? Each contract: one primary owner at strongest
   boundary. Another layer needs distinct transport/lifecycle risk. Extend
   tables/shared fixtures; consolidate repeated setup.
4. Export, flag, wrapper, injection hook with no production caller needed?
   Move proof to real boundary instead.

- Follow the repo's testing guide when `CLAUDE.md` / `AGENTS.md` names one.
- Regressions must fail pre-fix for intended reason, pass after repair; report
  unavailable baseline proof.
- One owner regression covers bug; never replay at every crossed layer.
- Behavior-preserving refactors should not break test.

## Junk patterns

- Reject self-comparisons, copied inventories, source greps,
  implementation/call-shape assertions, duplicate contracts, test-only seams,
  subject-generated expected values, mocks supplying asserted behavior.
- Fixtures must not fabricate owner results/order or claim persistence path
  never writes.
- Negative controls must fail at intended guard; names must match exercised behavior.
- Auditing candidates: use [full checklist](references/audit.md#junk-patterns).

## Retention bar

- Keep independent API, SDK, protocol, config, migration, storage, security,
  platform, default, prompt, generated-language, package, release,
  architecture, observable-ordering contracts. Static/slow is not disposable.
- Source inspection must survive identifier-only refactors and guard independent
  user-facing contract.
- Baseline failures may be bugs to repair, not tests to delete.

## Audit and validation

- Before audit edit: read [candidate evidence and workflow](references/audit.md#candidate-evidence);
  inspect full owners/callers/history; record remaining proof before deletion.
- Delete obsolete production seams with tests; never optimize for deletion counts.
- Take test, typecheck, lint, format commands from the repo's `CLAUDE.md`,
  README, or package scripts; stop any test watcher before editing; run
  owner/sibling proof + changed-path gates; then `/code-review`.
- Report causes, retained contracts, proof/cost, production versus test LOC,
  follow-ups. Branching, PRs, and landing belong to the orchestrator, not this skill.
