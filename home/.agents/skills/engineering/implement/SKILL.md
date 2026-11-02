---
name: implement
description: "Implement a piece of work based on a spec or set of tickets."
metadata:
  author: mattpocock
---

Implement the work described by the user in the spec or tickets.

If the user passes a ticket reference, fetch it from the issue tracker and state its title before starting. If the reference is ambiguous, ask.

Call the Skill tool with "tdd" where possible, at pre-agreed seams.

Optional, where their descriptions fit the work: /go-best-practices, /react-best-practices, /typescript-best-practices, /supabase, /ddd.

Run typechecking regularly, single test files regularly, and the full test suite once at the end.

Once done, call the Skill tool with "code-review" to review the work.

Then invoke /commit to commit the work to the current branch.
