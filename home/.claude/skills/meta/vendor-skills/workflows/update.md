# Update

Input: skill names, or nothing for every skill in the catalog with `status.state` `tracked`.

1. **Targets.** Run `vendor.ts catalog list` and `vendor.ts root`.
   - A named skill missing from the catalog, or marked `removed-upstream`, is reported and skipped.
   - If `root.skills[<name>]` differs from the entry's `localPath`, the skill moved. Use the new path and pass it as `--local` when recording.
2. **Per skill, in order.** `<url>` is `https://github.com/<source.repo>/tree/<source.ref>/<source.path>`, and `<local>` is `<root>/<localPath>`.
   1. **Fetch upstream.** Run `vendor.ts fetch <url>` and call its output `new`.
      - The repo answers but the path is gone (`not-found`): run `vendor.ts catalog mark-removed <name>` and keep the local copy. Later updates skip it.
      - The repo is unreachable (git error): report it, then ask whether to skip it this time or mark it removed.
   2. **Unchanged?** If `new.tree` equals `sync.tree`, the skill is up to date. Clean up and move on.
   3. **Fetch the base.** Run `vendor.ts fetch <url> --commit <sync.commit>` and call its output `base`. If the base commit is gone (rewritten history), reconcile with the Adopt flow in [add.md](add.md) against `new` instead.
   4. **Gate** the incoming change per [security-review.md](../references/security-review.md):
      - `vendor.ts scan <new.skillDir>`;
      - the review, scoped to `git diff --no-index <base.skillDir> <new.skillDir>`.
   5. **Merge.** Run `vendor.ts merge <base.skillDir> <local> <new.skillDir> --out <tmp> --author <author>`. `<tmp>` is a fresh dir in the cache.
   6. **Reconcile** every `localChanges` note against the upstream diff from step 4. Classify each note:
      - **still needed:** upstream didn't touch that area or didn't address it. Keep it.
      - **obsolete:** upstream now does the same thing, or removed the section. Drop our change and the note.
      - **needs rework:** upstream restructured around it. Carry the intent into the new structure and reword the note.
      
      Decide yourself when the diff makes the answer plain, and state the reasoning in the report. Otherwise use AskUserQuestion with concrete options (keep ours / take upstream / combined), showing the texts in `preview`.
   7. **Resolve conflicts.** Every `conflict` entry gets the same treatment, hunk by hunk.
      - Remove every marker. The step is done when `rg -n '^(<<<<<<<|=======|>>>>>>>)( |$)' <tmp>` prints nothing.
      - A `conflict` whose detail says one side deleted the file needs an explicit keep-or-drop decision.
   8. **Adapt.** Rewrite any npm, npx or yarn commands still in `<tmp>` per [package-manager.md](../references/package-manager.md). Add the note from that file if the entry doesn't already have it.
   9. **Apply.** Run `vendor.ts install <tmp> <local> --replace --author <author>`.
   10. **Record.** Run `vendor.ts catalog upsert <name> --fetch <new.out> --local <localPath>` with one `--note` per surviving change, or `--clear-notes` when none survive.
   11. **Clean up** `new.out`, `base.out` and `<tmp>`.
3. **Report** a table with columns skill | result | decisions. Results: up to date, updated, updated with decisions, removed upstream, skipped, failed. The user reviews the file changes with `git diff` in the skills repo.
