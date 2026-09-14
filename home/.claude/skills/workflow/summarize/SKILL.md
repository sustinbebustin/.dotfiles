---
name: summarize
description: Convert a URL or document (PDF, DOCX, PPTX, HTML) to Markdown with markitdown, and optionally summarize it with a Haiku subagent. Use when reading a web page or binary document, or condensing a long one before deeper work.
argument-hint: <url-or-path> [focus]
allowed-tools: Bash(node ${CLAUDE_SKILL_DIR}/to-markdown.mjs *)
---

# Summarize

## 1. Convert

```bash
node ${CLAUDE_SKILL_DIR}/to-markdown.mjs <url-or-path> --tmp
```

Prints the path of the converted `.md` file. Use `--out <file>` instead when the user wants the Markdown kept somewhere specific. Done when a path is printed; on failure, relay the script's error and stop.

## 2. Use the Markdown

- **User wants the content itself** (quote, inspect, process): Read the file, in ranges if large.
- **User wants a summary**: go to step 3.

## 3. Summarize

Establish the **focus** first: what to extract, and for whom. Take it from the arguments or the conversation; ask the user only when neither supplies one. A summary without a focus is generic and rarely useful.

Dispatch an Agent with `model: haiku`, passing the file path and focus:

> Read the Markdown document at `<path>` in full (in ranges if large). Focus: `<focus>`.
> Produce: a one-paragraph executive summary; 8-15 bullets of key facts, decisions, and requirements; an "Open questions / missing info" section. Preserve exact numbers, names, and constraints.

Return the summary to the user with the Markdown path, so they can open the full document.
