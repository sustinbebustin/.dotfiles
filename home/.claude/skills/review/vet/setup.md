# Setting up a repo overlay

Reached from SKILL.md when the scope block reports `Overlay: none` and the user chose to set one up. It writes `.claude/vet.md` in the format [overlay.md](overlay.md) defines, then hands back to the review. Every line of the overlay is a fact: something read in the repo or answered by the user. An overlay that guesses steers every future review wrong.

The overlay goes at `<root>/.claude/vet.md`, where `<root>` is the repo or workspace root the scope block names: the repo root for one repo, the directory holding the repos for several.

## 1. Read what the repo already says

Collect, read-only:

- **Verify.** Compare the command the scope block resolved with what CI actually runs (`.github/workflows/`, `justfile`, `Makefile`, `package.json` scripts). Write `verify` only when auto-detection missed or chose the wrong command: a correct detection already lives in the repo. In a multi-repo workspace, write `verify.<repo>` where the repos verify differently. Write `verify.fallback` only when the full command is known to run out of memory.
- **Environment failures.** The services and private packages the checks depend on (docker compose files, a local database config, private git dependencies in a lockfile or Dockerfile): an `env_failure` pattern matching how each one fails.
- **Repos.** In a multi-repo workspace, short `repos.alias.<name>` entries for long repo names.
- **Deploy.** How changes ship: deploy workflows, a changesets directory, migration tooling, whether the repos release together.
- **Project knowledge** for the body, limited to what CLAUDE.md and the repo's rules files leave out (reviewers already load those): partner or vendor integrations and domain math (business risk), user-facing surfaces and who uses each, read-only data sources an MCP config exposes, docs bundled with installed dependencies.

Done when every frontmatter key in overlay.md has either a value traced to a file or a named gap.

## 2. Ask what the code can't show

One `AskUserQuestion` call, up to four questions, covering only the gaps from step 1 that would change a review. Each question offers options drawn from what step 1 found, recommended first. Typical gaps:

- Do these parts deploy together, or can one ship without the other?
- Is rollback supported? Rollback scenarios are reviewed only when a `deploy` line enables them.
- Which integrations or calculations encode business rules a reviewer should treat as possibly intended?
- May the verifier run read-only queries against the database or API step 1 found?

With no gaps, skip the step.

## 3. Write the overlay

Write `<root>/.claude/vet.md`: frontmatter keys only where step 1 or 2 produced a value, then a short body under the headings overlay.md lists. Print it in full, so the user sees exactly what every future review will read.

## 4. Return to the review

Re-run the scope block with the same arguments, as SKILL.md describes, so the verify command and deploy facts come from the new overlay. Tell the user the overlay is a new file they choose whether to commit, then continue at the verify gate.
