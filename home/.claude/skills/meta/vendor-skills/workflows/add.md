# Add

Input:
- a GitHub `tree/` or `blob/` URL, or `owner/repo/path`;
- optional `--project`;
- optional `--adopt <local-skill-dir>`.

1. **Fetch.** Run `vendor.ts fetch <source>` and keep its `out`, `skillDir` and `owner`.
   - A not-found error lists the skill folders nearby. Retry with the one that matches the user's intent.
   - If several fit, ask which one.
2. **Destination.** The skill's name is the fetched folder's name.
   - `--project`: `<git toplevel of the cwd>/.claude/skills/<name>`. Not catalogued.
   - `--adopt`: skip to [Adopt](#adopt).
   - Global: run `vendor.ts root`.
     - If `<name>` is already in `skills`, the name is taken. When `vendor.ts catalog show <name>` finds it, send the user to `update`. Otherwise ask: adopt it, install under a new folder name, or abort. Skill names must be unique across all categories.
     - If `categories` is non-empty, pick one with AskUserQuestion. Put the best fit first, judged from the skill's description and the skills already in each category. The destination is `<root>/<category>/<name>`.
3. **Gate.** Run the full review in [security-review.md](../references/security-review.md) on `skillDir`. Continue only when the gate passes.
4. **Install.** Run `vendor.ts install <skillDir> <dest> --author <owner>`.
5. **Record.** Global only: run `vendor.ts catalog upsert <name> --fetch <out> --local <dest relative to root>`.
6. **Publish.** When `needsPublish` is true, run the publish step from SKILL.md. The skill is done when `~/.claude/skills/<name>/SKILL.md` resolves.
7. **Clean up.** Remove `<out>`.
8. **Report** the destination, the short commit, the scan summary, and the gate outcome.

## Adopt

Adopt catalogues a skill that is already in the tree but has no catalog entry. It fetches the source from step 1 and uses the existing local folder as `<local>`.

1. Run `vendor.ts compare <local> <skillDir> --author <owner>`.
   - If every file is `equal`, run `vendor.ts author <local> <owner>`, then `vendor.ts catalog upsert <name> --fetch <out> --local <local relative to root>`. Done.
2. Find the **base**, the upstream commit the local copy was taken from: `vendor.ts closest <source> <local> --out <base-out>`. Diffing against HEAD would show every upstream edit since the copy as a local change. Diffing against the base isolates the real local edits.
3. Run `vendor.ts compare <local> <base-skillDir> --author <owner>`. For each file that is not `equal`, read `git diff --no-index <base-skillDir>/<file> <local>/<file>`. Split the differences into distinct changes: one intent each, not one per hunk.
4. For each change, ask whether it is ours or stale. Batch up to 4 changes per AskUserQuestion. Each question:
   - names the file;
   - describes the change in one sentence;
   - gives the options "Ours: keep and note it" and "Stale: take upstream".

   The base is only the nearest commit, so a leftover can still look like a local edit. The user is the only one who knows which it is.
5. Apply the answers.
   - When everything is stale: run `vendor.ts install <skillDir> <local> --replace --author <owner>`, then `vendor.ts catalog upsert <name> --fetch <out> --local <local relative to root>`. Skip to step 7.
   - Otherwise edit the stale changes in `<local>` to match the base, then run `vendor.ts author <local> <owner>`.
   - Any upstream content taken into `<local>` goes through the scanner first (`vendor.ts scan <skillDir>`), with the same gate rules.
6. Run `vendor.ts catalog upsert <name> --fetch <base-out> --local <local relative to root>` with one `--note` per kept change. Each note says what differs and why. The entry now syncs at the base. If the base is older than HEAD, run [update](update.md) for `<name>` to merge the newer upstream edits in.
7. Remove `<out>` and `<base-out>`.
