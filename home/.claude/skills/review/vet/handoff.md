# Handing off a mid-flight fix run

Reached from fix mode's context check when the user chooses a new session. The synthesis log and report exist; nothing has been applied, and nothing may be in this session.

1. Load the handoff skill: `Skill({ skill: "handoff" })`.
2. Write the handoff per that skill. It must carry:
   - That `/vet fix` stopped after the report: auto-applies and asks not yet run.
   - The synthesis log path -- the source of truth for findings, verdicts, and routing.
   - The report as printed, so the next session sees the same picture without re-running reviewers.
   - The exact scope: the argument string, every `--repo` and `--base`, and, if the run auto-selected `branch` or `commit`, that choice written out explicitly with the per-repo base SHAs.
   - The verify command the gate resolved.
   - Verifier caveats the next session must keep when it phrases questions (PLAUSIBLE findings, gates cited).
3. Leave the code untouched: no edits, no `AskUserQuestion` for pending findings. Stop after writing the handoff.
4. Reply with only the prompt the user pastes into the new session:

   ```
   /vet fix <scope args> -- resume from handoff @<handoff path>. The synthesis log at <synthesis path> holds the verified findings and routing. Skip to fix mode: snapshot, apply the auto set, ask the ask set, verify once with `<verify command>`, then the delta review.
   ```

   Keep every scope argument: dropping one silently widens or narrows the resumed run. The `--` before `resume` is required, or the scope script rejects `resume` as an unknown argument.
