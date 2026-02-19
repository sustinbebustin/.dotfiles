---
name: interview-spec
description: Interview user about a plan file to enrich it with detailed specifications.
allowed-tools: Read, Write, Edit, AskUserQuestion
argument-hint: [.claude/plans/*.md]
disable-model-invocation: true
---

# Interview Spec

Argument: `$ARGUMENTS`

## Plan File

**Newest plan:** !`ls -t .claude/plans/*.md 2>/dev/null | head -1`

If `$ARGUMENTS` was provided, use it as the plan file path. Otherwise use the newest plan path detected above.

If no plan file was resolved, ask the user to provide a path.

Read the resolved plan file with the Read tool before starting the interview.

## Interview Process

1. Interview me in detail using the AskUserQuestion tool about literally anything: technical implementation, UI & UX, concerns, tradeoffs, edge cases, error handling, data modeling, performance, security, accessibility, etc.
2. Questions should NOT be obvious -- go deep. Ask about things I probably haven't considered yet.
3. Continue interviewing me iteratively until all areas are thoroughly covered
4. After each round of answers, identify follow-up areas and ask more questions

## After the Interview

Once the interview is complete, update the original plan file at the path shown above:
- Preserve the existing structure and content
- Enrich sections with the details gathered during the interview
- Add new sections as needed for topics not originally covered
- Mark any unresolved decisions or open questions
