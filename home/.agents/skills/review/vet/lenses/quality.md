# Lens: quality

Whether the change leaves the code simpler to read and change. Be ambitious: look for the **code judo** move -- a restructuring that keeps behaviour and makes whole branches, helpers, modes, or layers disappear -- before settling for local polish.

Every quality finding carries a concrete `proposed_fix` (the actual restructuring, extraction, or deletion, not "consider simplifying"). Prefer a few high-conviction findings over many cosmetic ones; skip naming and nits when a structural finding exists in the same code.

## Targets

- **Code judo.** A reframing of the state model or ownership boundary that deletes complexity rather than moving it. A refactor that rearranges code without reducing the concepts a reader must hold is a finding.
- **Spaghetti growth.** New ad-hoc conditionals, one-off booleans, nullable modes, or special cases bolted into an existing flow. The remedy is the abstraction the branch is missing: a typed model, a dispatcher, a dedicated module.
- **Thin layers.** Wrappers, pass-through helpers, single-use hooks, and interfaces with one non-test implementation that add indirection without clarity.
- **Canonical layer and reuse.** Logic placed in the wrong package or layer; a bespoke helper where the codebase already has one (name it, with `file:line`); copy-pasted logic that wants extraction.
- **File size.** The briefing lists files that grew past 1,000 lines. Treat crossing the line as a presumptive finding: propose the decomposition.
- **Naming.** The five-second rule: a name a reader cannot decode in five seconds fails (`handleData`, `doStuff`, bare `Handler`). Stutter (`users.UserService`).
- **Testability.** Code hard to test because concerns are mixed: name the extraction that makes it testable.
- **Test gaps.** Changed behaviour with no test covering it, judged by the repo's own test conventions (table-driven Go tests, integration over mocks where that is the norm). Name the case that is missing.
