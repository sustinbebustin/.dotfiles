# Audit workflow

- Use after [authoring gate](../SKILL.md#authoring-gate).
- Multi-batch audits and whole-subsystem campaigns: run through `/test-audit-stack`.

## Junk patterns

Reject new matches unless [retention bar](#retention-bar) identifies independent
contract. Audit existing matches:

- Assertion-free probes, self-comparisons, identity copiers, subject-generated
  expected values.
- Copied inventories/manifests/export lists; exact source/import/string greps.
- Private predicate/call-shape tests duplicating boundary proof; repeated
  invocations/per-implementation replays of shared helpers.
- Test-only exports/globals/wrappers; production paths called only by tests.
- Mocks implementing asserted behavior or pretending distinct APIs match.
- Fixtures supplying results, admission, callback ordering owner should produce;
  persistence asserted against store path never writes.
- Capability flags asserted instead of promised delivery/acknowledgement.
- Negative controls passing on unrelated guard or unreachable rejection.
- Names promising behavior inputs/assertions never exercise.

## Retention bar

- Keep independent public API, SDK, protocol, config, migration, storage,
  security, platform, default, prompt, generated cross-language, package,
  release, architecture contracts. Keep observable ordering + credible regressions.
- Source inspection valid when cheapest independent guard of user-facing key,
  byte, path, and survives identifier-only refactoring.
- Static/slow is not a deletion reason. Baseline failure may be product bug:
  reproduce + repair owner; never delete test for it.

## Candidate evidence

- Before editing: read full test, production owner, entry point, callers,
  callees, siblings, overlapping proof, CI routing, history. Dependency-backed
  claims: inspect dependency source/types. Read `CLAUDE.md` / `AGENTS.md`
  scoped to touched directories.
- Record each candidate's:
  - Exact name/location + detectable failure.
  - Non-test callers of covered production/support seam.
  - Stronger remaining owner proof, or why none needed.
  - History/reason it exists + deletion it unlocks.
  - Risk + focused validation command.
- Missing evidence: no deletion. Discovery read-only; report before edits.
  Prefer few high-confidence candidates.

## Edit shape

- One coherent owner-boundary batch.
- Delete obsolete seams/dead paths; move retained regressions to owner;
  consolidate generic package/dependency contracts.
- Prefer net-negative production LOC. Never add replacement implementation tests
  or turn uncertainty into cleanup.

## Validation

- Take test, typecheck, lint, format commands from the repo's `CLAUDE.md`,
  README, or package scripts. Stop any test watcher before editing source or tests.
- Run owner/sibling tests with the repo's runner, filtered to changed paths.
- Removed source greps/plan assertions: exercise owning executable/dry-run.
- Run targeted format, `git diff --check`, then the typecheck, lint, and test
  gates the repo requires for changed paths.
- Inspect `git diff --numstat`; run `/code-review` after final edits.

## Handoff

Report causes, removed categories/seams, retained false positives, proof
actually run, production versus test/support LOC, follow-ups.
