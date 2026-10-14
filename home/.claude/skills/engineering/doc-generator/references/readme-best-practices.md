# README Best Practices

A README is the front door: in one screen a reader should know what the project is, whether it fits their problem, and how to get it running. Everything after that is reference they scan, not prose they read.

## Pick the Archetype First

The archetype decides the sections. Read the repo, pick one, then compose from the [section patterns](#section-patterns) below. Never fill a section just because a template has it.

| Archetype | Lead with | Core sections |
|---|---|---|
| Library | A real code example right under the tagline | Install (+ runtime/version constraints), mental model, task-oriented usage, API map, docs links |
| CLI | What it is + a numbered 3-step workflow in real commands | Prerequisites, Install (each channel), Quick start, Layout, Commands table, Scripting/agents contract, Configuration, Development |
| Desktop/end-user app | Tagline + download link row + 3 benefit bullets | Install (with permissions), the core interaction, optional features, Privacy and local data, Troubleshooting, Contributing |
| Service | What it serves + who calls it | Prerequisites, env vars table, run + migrate + health check, deploy notes, docs links |
| Collection (skills, plugins, templates) | Pitch + why it exists | Install per host, setup step, problem/fix sections, grouped catalog |

Length follows where the docs live. If a docs site or `docs/` holds the depth, the README stays a front door (~2 screens) and links out. If the README *is* the docs, it may run long, but then it needs a Contents list after Install and every section must stay task-shaped.

## Anatomy

1. **Hero** -- name, one-line tagline, optional link row.
2. **What it is** -- one sentence naming the thing and its job, then how it works or why it matters.
3. **Proof** -- the shortest real example, command sequence, or screenshot that shows the value.
4. **Prerequisites + Install** -- everything needed before the first command works.
5. **Quick start** -- numbered path from nothing to first success.
6. **Reference sections** -- archetype-specific; named by what the reader is trying to do.
7. **Contributing + License** -- a pointer and one line.

## Section Patterns

### Hero
- Centered logo or banner only when the repo already has the asset. Use `<picture>` with dark/light `srcset` when both variants exist. Never invent image paths.
- Tagline under the name: what it is in under 12 words (`Lightweight Result type for TypeScript with generator-based composition.`).
- Link row for projects with external docs: `Documentation · Quickstart · API reference · Contributing`, separated by ` · `.
- Badges: 0-3, only ones that carry signal (version, CI, a registry). Skip decoration.

### What it is
- Open with `## What Is <Name>?` or no heading at all; the first sentence is a noun phrase: "A small Go CLI for managing **meta workspaces** -- ...".
- Follow with either:
  - a numbered 3-step "how it works" using real calls (`mws init`, `mws add-repo`, `mws clone`), then one `The result: ...` payoff line; or
  - 3 bold-lead benefit bullets: `- **Make failures explicit:** ...`.
- Say what it is not, or what it does not require, when readers would otherwise assume it ("ordinary dictation does not require a cloud service").

### Proof
- Library: a 15-30 line snippet that runs as-is, then one sentence on what it guarantees (`parsePort` returns `Result<number, InvalidPort>`; callers cannot use the port until they handle the failure).
- CLI: the quick start doubles as proof.
- App: a screenshot or GIF if the repo has one.

### Prerequisites
- Bullet list; each item links to the tool and says why it is needed: `[Git](https://git-scm.com/) -- every native repo is a git repository.`
- Nest alternatives under one bullet when the reader picks one (Docker / Podman / Vercel / your own).

### Install
- One `###` per channel, most common first (Homebrew, pre-built binary, `go install`, npm/pnpm/bun).
- For many hosts or platforms, one `<details><summary><strong>Host</strong></summary>` block each so the section stays scannable.
- State constraints in one line: minimum runtime version, ESM-only, zero runtime dependencies, supported OS/arch.
- End with how to verify (`mws version`) and how updates happen.
- Optional integrations (shell function, editor plugin) get their own subsection marked optional, with what changes without them.

### Quick start
- 3-5 numbered steps, each a sentence of intent then one command block.
- Show expected output or created files when that prevents confusion.
- Variants (optional flags, alternate paths) go after the main path, not inside it.

### Concepts
- `## How it works` or `## Mental model`: the 2-4 ideas a reader needs before the reference makes sense, each with a minimal snippet.
- Annotated directory tree for anything that creates or expects a layout, comments aligned in a column:
  ```
  my-project/          # this directory IS a git repo
  ├── .mws.toml        # project name, repos, env mappings
  └── main/            # first working copy (untracked)
  ```
- Link the ADR or design doc for rationale instead of arguing it inline.

### Reference
- Name sections by task, not by component: "Recover from errors", "Work with collections", "Dictate anywhere".
- CLI: a `Command | What it does` table, flags inline in backticks. Then a short scripting contract when the CLI is meant for automation or agents: non-interactive behaviour, stdout vs stderr, `--json`, exit codes.
- Library: an `Intent | APIs` table that maps goals to functions, plus a `Type | Purpose` table for public types; link the full reference for signatures.
- Configuration: a real annotated config file, then bullets on what each part does at runtime; env vars as `Variable | Required | Default | Description`. Link the full schema.
- Collections: group the catalog by category and invocation, one line each: `- **[name](./path/SKILL.md)**: what it does.`

### Trust and support
- Privacy and data (apps, anything handling user content): what stays local, what is sent where and when, retention, how to clear it.
- Troubleshooting: a `Symptom | Check` table, then where logs live and how to report a bug (what to include, what to redact).
- Limitations and beta status stated plainly, with what is and is not supported.

### Agents and AI
- When the project ships an agent skill or `AGENTS.md`, a short section with the install command and a link.

### Close
- Contributing: one paragraph pointing to `CONTRIBUTING.md`, `AGENTS.md`, `docs/`, and `SECURITY.md` as they exist.
- Development: the 2-3 commands to build and test.
- License: one line linking `LICENSE`.

## Writing Style
- Concrete over promotional: name the mechanism ("shares the harness via symlinks"), not the adjective ("seamless").
- Every command, path, flag, and env var must exist in the repo. Verify against `--help`, the manifest, or source.
- Short paragraphs; one idea each. Bold sparingly, for terms the reader must remember.
- Second person, present tense, active voice.
- Mention a caveat where it bites, in the section that triggers it, not in a trailing notes section.
- GitHub alerts (`> [!TIP]`, `> [!WARNING]`) at most once or twice per README.
- Match the repo's existing character set and dash style.

## Avoid
- Template sections with nothing real to say (empty Configuration, placeholder Roadmap, "Features" that repeat the bullets above).
- Feature lists without a single example.
- Restating the same install command in several places.
- Badge walls, emoji headings, marketing superlatives.
- Documenting internals a user never touches; that belongs in `docs/architecture.md`.

## Exemplars

Fetch one matching the archetype when unsure how a section should read.

- Library, long-form README: [dmmulroy/better-result](https://github.com/dmmulroy/better-result)
- Library with providers and CLI: [mattpocock/sandcastle](https://github.com/mattpocock/sandcastle)
- CLI: [sustinbebustin/mws](https://github.com/sustinbebustin/mws)
- Desktop app: [anomalyco/hex](https://github.com/anomalyco/hex)
- Collection: [mattpocock/skills](https://github.com/mattpocock/skills)

## Maintenance
- Update the README in the same change that alters a command, flag, env var, or install channel.
- Re-run the quick start and every snippet you touched before finishing.
