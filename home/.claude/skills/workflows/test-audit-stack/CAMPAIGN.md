# Campaign mode

A campaign prunes one subsystem's whole test surface (a package, a plugin, or one core area) as one stack. It replaces step 2 (Discover) of [SKILL.md](SKILL.md) with steps C1-C4 below, then runs steps 3-5 there, with C5-C7 slotted in before the merge. Every subagent invokes the test-audit skill, whose value bar, retention bar, candidate evidence, and validation apply throughout. Each step ends on its completion criterion; start the next step only then.

## C1. Baseline and lanes

Spawn one subagent (`subagent_type: "general-purpose"`, `model: "<MODEL>"`), read-only apart from running tests, to:

- record the subsystem's test and support line counts and every test file's pass/fail state at the current `origin/<DEFAULT>` SHA. Baseline failures go in their own list: they are often real product bugs, not stale tests;
- split the surface into **lanes** along production owner boundaries, not file prefixes. A messaging integration might split into accounts, commands, context, dispatch, inbound, outbound, persistence, transport, shared, harness, and live/QA scenarios. Include the subsystem's cases at shared core boundaries and its QA and live-proof harness tests.

Done when every in-scope test file has a baseline result and belongs to exactly one lane.

## C2. Ledger per lane

Spawn one read-only subagent per lane, in parallel. Each invokes test-audit, reads every assigned test in full (parameter tables included) plus the production owners, their entry points, callers, history, and CI routing, and writes a **ledger**: every test declaration with one mark and an evidence line. A parameterized test (`it.each`, `test.each`, a table-driven loop) is one declaration unless its rows need different marks; then each row gets one.

- `R`: retain, naming the contract and the bug it catches; a retained test that only moves to a better-named file stays `R` with the move noted;
- `F`: retain the contract but repair the assertion, such as a vacuous negative that passes when only one of several items is missing;
- `C`: consolidate, naming the owner that absorbs the assertion first: a sibling table case, a stronger boundary suite, or the shared owner in another package;
- `D`: delete, naming the proof that remains, or why no contract exists.

Judge a test by its assertions, not its name: a test named for retiring a progress window can assert the window was _not_ cleared.

Done when every declaration in every lane has a mark and an evidence line.

## C3. Layer plan per lane

Send each ledger back to its lane subagent for a second read-only pass that hunts the redundant **layer**: for example, several suites replaying one shared helper through a mocked collaborator, around stronger real-stream or HTTP-fixture suites. It names the **keeper** suite for each contract, preferring the real transport boundary with a fake network over a mocked collaborator, and corrects any ledger errors it finds.

Done when each lane plan names its retired files, its keeper per contract, the assertions to carry into keepers, and the test-only production seams it unlocks.

## C4. Batch plan

Each lane plan becomes one batch; the batch carries the lane plan as its evidence. Changes to shared harnesses and support files go into one batch of their own, first in the stack, so no two lanes edit them. Each batch also removes the test-only production seams it unlocks (injection parameters, getters, reset exports, indirection layers), registers moved suites in CI routing and any test inventories, and updates size or coverage baselines it affects.

Then continue at step 3 (Approve) of SKILL.md.

## C5. Preservation review

After the last lane batch lands, spawn independent read-only reviewer subagents, one per boundary group, over `origin/<DEFAULT>..<top branch>`. They compare deleted coverage against the keepers and report contracts that lost their only proof, and new assertions that cannot fail, such as a rejection row the production code never reaches.

Gaps become one more batch at the top of the stack. Its subagent restores each gap, or rejects it with source evidence, and for each restored contract makes one deliberate **mutation** of the production owner, confirms the keeper goes red, then restores the source byte for byte.

Done when every reported gap is restored or rejected, and every restored contract has a caught mutation.

## C6. Product defects

A baseline failure that survives into a keeper is a bug report. Each one becomes its own batch, fixed at its owner and proven through the real user flow with a **control** run that reverts the fix and shows the old behavior. Unrelated product discrepancies become follow-ups in the final report, not batches.

Done when each repaired defect has a failing control and a passing candidate on the same harness.

## C7. Reconcile

Before step 5 (Merge), run `git fetch origin` and `stack sync --apply` to replay the stack onto the latest `<DEFAULT>`. Where `<DEFAULT>` modified a file the campaign deleted, keep the deletion and port the new contract into its keeper, as a fix-up batch at the top of the stack; confirm every regression `<DEFAULT>` added still has a home. A final batch adds durable test-ownership rules to the subsystem's `CLAUDE.md` (or `AGENTS.md`, whichever the repo uses), drawn from mistakes this campaign actually found. Have a subagent rerun the whole subsystem suite and repeat live proof on the top branch.

Record maintainer decisions about compatibility flags in the PR description rather than editing gates. Add to the final report:

- baseline and final test/support line counts, with production counted separately;
- lanes, retired layers, and keepers;
- preservation gaps found and their mutations;
- product defects with control and candidate proof.

Done when the top branch passes the whole subsystem suite on the latest `<DEFAULT>`.
