---
name: clean-copy
description: Reimplement the current branch on a new branch with a clean, narrative-quality git commit history suitable for reviewer comprehension.
disable-model-invocation: true
allowed-tools: Read, Edit, Write, Glob, Grep, Bash(git status:*), Bash(git log:*), Bash(git diff:*), Bash(git checkout:*), Bash(git switch:*), Bash(git branch:*), Bash(git add:*), Bash(git commit:*), Bash(git reset:*), Bash(git merge-base:*)
---

### Steps

1. **Validate the source branch**
   - Ensure the current branch has no merge conflicts, uncommitted changes, or other issues.
   - Confirm it is up to date with `main`.

2. **Analyze the diff**
   - Study all changes between the current branch and `main`.
   - Form a clear understanding of the final intended state.

3. **Create the clean branch**
   - Create a new branch named `{branch_name}-clean` from the current branch.

4. **Plan the commit storyline**
   - Break the implementation down into a sequence of self-contained steps.
   - Each step should reflect a logical stage of development—as if writing a tutorial.

5. **Reimplement the work**
   - Recreate the changes in the clean branch, committing step by step according to your plan.
   - Each commit must:
     - Introduce a single coherent idea.
     - Include a clear commit message and description.
     - Add comments or inline GitHub comments when needed to explain intent.

6. **Verify correctness**
   - Confirm that the final state of `{branch_name}-clean` exactly matches the final state of the original branch.
   - Use `--no-verify` only when necessary (e.g., to bypass known issues). Individual commits do not need to pass tests, but this should be rare.

There may be cases where you will need to commit with --no-verify in order to avoid known issues. It is not necessary that every commit pass tests or checks, though this should be the exception if you're doing your job correctly. It is essential that the end state of your new branch be identical to the end state of the source branch.

NOTE: Do not push the branch or create a pull request. All work is done locally.

## Authorship

- **Never** add co-author information or AI attribution
- **Never** include "Co-Authored-By" trailers
- **Never** add "Generated with Claude" or similar messages
- Write commit messages as if the user authored them directly