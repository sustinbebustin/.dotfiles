#!/usr/bin/env node
/**
 * Helper for the vendor-skills skill: fetches one skill folder from GitHub into
 * quarantine, scans it, compares and 3-way merges it against the local copy,
 * stamps `metadata.author`, and owns every write to the vendored-skills catalog.
 *
 * Every command prints JSON on stdout. Failures print one line on stderr and
 * exit 1 (2 for usage errors). Run with `node vendor.ts help`.
 */
import { spawnSync } from "node:child_process";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { parseArgs } from "node:util";

// --- Result ---------------------------------------------------------------

export type VendorError = {
  readonly kind: "usage" | "source" | "git" | "not-found" | "frontmatter" | "catalog" | "io";
  readonly message: string;
};

export type Result<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly error: VendorError };

const ok = <T>(value: T): Result<T> => ({ ok: true, value });
const fail = (kind: VendorError["kind"], message: string): Result<never> => ({
  ok: false,
  error: { kind, message },
});

// --- Locator: where a skill lives on GitHub -------------------------------

/**
 * A parsed skill source. `tree/` and `blob/` URLs put the ref and the path in
 * one run of segments; where the ref ends is unknown until the remote's refs
 * are listed, because branch names may contain slashes.
 */
export type Locator =
  | { readonly kind: "default-branch"; readonly owner: string; readonly repo: string; readonly path: string }
  | {
      readonly kind: "ref-and-path";
      readonly owner: string;
      readonly repo: string;
      readonly segments: readonly string[];
    };

export type ResolvedSource = {
  readonly owner: string;
  readonly repo: string;
  readonly ref: string;
  readonly path: string;
};

const OWNER_RE = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$/;
const REPO_RE = /^[A-Za-z0-9._-]+$/;
const SHA_RE = /^[0-9a-f]{40}$/;

const unsafeSegment = (segment: string): boolean => segment === "." || segment === "..";

export const Locator = {
  /** Accepts `https://github.com/o/r/tree/<ref>/<path>`, `.../blob/...`, and `o/r/<path>`. */
  parse(input: string): Result<Locator> {
    const trimmed = input.trim().replace(/[?#].*$/, "").replace(/\/+$/, "");
    const url = /^(?:https?:\/\/)?(?:www\.)?github\.com\/(.+)$/.exec(trimmed);
    let rest: string;
    if (url?.[1] !== undefined) {
      rest = url[1];
    } else if (!trimmed.includes(":") && /^[^/\s]+\/[^/\s]+/.test(trimmed)) {
      rest = trimmed;
    } else {
      return fail("source", `"${input}" is not a GitHub URL or owner/repo/path shorthand; only github.com sources are supported`);
    }
    const segments = rest.split("/").filter((s) => s !== "");
    const [owner, rawRepo, marker, ...tail] = segments;
    if (owner === undefined || rawRepo === undefined) {
      return fail("source", `"${input}" needs at least owner/repo`);
    }
    const repo = rawRepo.replace(/\.git$/, "");
    if (!OWNER_RE.test(owner) || !REPO_RE.test(repo)) {
      return fail("source", `"${owner}/${repo}" is not a valid GitHub owner/repo`);
    }
    const pathTail = marker === undefined ? [] : [marker, ...tail];
    const hasRef = marker === "tree" || marker === "blob";
    const rawSegments = hasRef ? tail : pathTail;
    const withoutSkillFile = rawSegments.at(-1) === "SKILL.md" ? rawSegments.slice(0, -1) : rawSegments;
    if (withoutSkillFile.some(unsafeSegment)) {
      return fail("source", `"${input}" contains a "." or ".." path segment`);
    }
    if (!hasRef) {
      return ok({ kind: "default-branch", owner, repo, path: withoutSkillFile.join("/") });
    }
    if (withoutSkillFile.length === 0) {
      return fail("source", `"${input}" has /${marker}/ but no ref after it`);
    }
    return ok({ kind: "ref-and-path", owner, repo, segments: withoutSkillFile });
  },

  /** Splits `ref-and-path` segments at the longest prefix naming a remote ref (or a full SHA). */
  splitRef(segments: readonly string[], refs: readonly string[]): Result<{ ref: string; path: string }> {
    const [first] = segments;
    if (first !== undefined && SHA_RE.test(first)) {
      return ok({ ref: first, path: segments.slice(1).join("/") });
    }
    const known = new Set(refs);
    for (let i = segments.length; i > 0; i--) {
      const candidate = segments.slice(0, i).join("/");
      if (known.has(candidate)) return ok({ ref: candidate, path: segments.slice(i).join("/") });
    }
    return fail("not-found", `no branch or tag on the remote matches the start of "${segments.join("/")}"`);
  },
} as const;

// --- git ------------------------------------------------------------------

const GIT_ENV = { ...process.env, GIT_TERMINAL_PROMPT: "0", GIT_LFS_SKIP_SMUDGE: "1" };
const GIT_SAFETY = ["-c", "core.hooksPath=/dev/null", "-c", "protocol.file.allow=never"];

function git(args: readonly string[], cwd?: string): Result<string> {
  const r = spawnSync("git", [...GIT_SAFETY, ...args], {
    cwd,
    env: GIT_ENV,
    encoding: "utf8",
    maxBuffer: 64 * 1024 * 1024,
  });
  if (r.error !== undefined) return fail("git", `git ${args[0] ?? ""} could not start: ${r.error.message}`);
  if (r.status !== 0) return fail("git", `git ${args.join(" ")} exited ${r.status ?? "by signal"}: ${r.stderr.trim()}`);
  return ok(r.stdout);
}

const remoteUrl = (owner: string, repo: string): string => `https://github.com/${owner}/${repo}.git`;

function listRefs(url: string): Result<string[]> {
  const out = git(["ls-remote", "--heads", "--tags", url]);
  if (!out.ok) return out;
  return ok(
    out.value
      .split("\n")
      .map((line) => line.split("\t")[1] ?? "")
      .filter((ref) => ref !== "")
      .map((ref) => ref.replace(/^refs\/(heads|tags)\//, "").replace(/\^\{\}$/, "")),
  );
}

function defaultBranch(url: string): Result<string> {
  const out = git(["ls-remote", "--symref", url, "HEAD"]);
  if (!out.ok) return out;
  const match = /^ref: refs\/heads\/(\S+)\tHEAD$/m.exec(out.value);
  if (match?.[1] === undefined) return fail("git", `could not read the default branch of ${url}`);
  return ok(match[1]);
}

export function resolveSource(locator: Locator): Result<ResolvedSource> {
  const url = remoteUrl(locator.owner, locator.repo);
  if (locator.kind === "default-branch") {
    const ref = defaultBranch(url);
    if (!ref.ok) return ref;
    return ok({ owner: locator.owner, repo: locator.repo, ref: ref.value, path: locator.path });
  }
  const refs = listRefs(url);
  if (!refs.ok) return refs;
  const split = Locator.splitRef(locator.segments, refs.value);
  if (!split.ok) return split;
  return ok({ owner: locator.owner, repo: locator.repo, ...split.value });
}

// --- fetch: one folder into quarantine ------------------------------------

export type FetchResult = ResolvedSource & {
  readonly commit: string;
  readonly tree: string;
  /** The fetched skill folder, inside `<out>/repo/`. */
  readonly skillDir: string;
  readonly out: string;
};

const cacheRoot = (): string =>
  path.join(process.env["XDG_CACHE_HOME"] ?? path.join(os.homedir(), ".cache"), "vendor-skills");

/** Candidate SKILL.md folders in the fetched commit, for a helpful not-found error. */
function skillCandidates(repoDir: string, under: string): string[] {
  const listing = git(["ls-tree", "-r", "--name-only", "FETCH_HEAD", ...(under === "" ? [] : [under])], repoDir);
  if (!listing.ok) return [];
  return listing.value
    .split("\n")
    .filter((f) => f === "SKILL.md" || f.endsWith("/SKILL.md"))
    .map((f) => path.posix.dirname(f))
    .slice(0, 20);
}

/**
 * Partial, sparse, shallow fetch: trees for one commit plus only the blobs
 * under `source.path`. No checkout hooks, LFS, or submodules run.
 */
export function fetchSkill(source: ResolvedSource, commit: string | null, out: string): Result<FetchResult> {
  if (fs.existsSync(out) && fs.readdirSync(out).length > 0) {
    return fail("io", `${out} already exists and is not empty; pass a fresh --out`);
  }
  const repoDir = path.join(out, "repo");
  fs.mkdirSync(repoDir, { recursive: true });
  const target = commit ?? source.ref;
  const sparse = source.path === "" ? "/*" : `/${source.path}/`;
  const steps: readonly (readonly string[])[] = [
    ["init", "-q", repoDir],
    ["-C", repoDir, "remote", "add", "origin", remoteUrl(source.owner, source.repo)],
    ["-C", repoDir, "sparse-checkout", "set", "--no-cone", sparse],
    ["-C", repoDir, "fetch", "-q", "--depth", "1", "--filter=blob:none", "--no-tags", "origin", target],
  ];
  for (const step of steps) {
    const r = git(step);
    if (!r.ok) return r;
  }
  const head = git(["rev-parse", "FETCH_HEAD^{commit}"], repoDir);
  if (!head.ok) return head;
  const treeSpec = source.path === "" ? "FETCH_HEAD^{tree}" : `FETCH_HEAD:${source.path}`;
  const tree = git(["rev-parse", "--verify", "-q", treeSpec], repoDir);
  const skillFile = source.path === "" ? "SKILL.md" : `${source.path}/SKILL.md`;
  if (!tree.ok || !git(["cat-file", "-e", `FETCH_HEAD:${skillFile}`], repoDir).ok) {
    const where = tree.ok ? source.path : path.posix.dirname(source.path);
    const candidates = skillCandidates(repoDir, where === "." ? "" : where);
    const hint = candidates.length > 0 ? ` Skill folders nearby: ${candidates.join(", ")}` : "";
    return fail(
      "not-found",
      `${source.owner}/${source.repo}@${target} has no ${skillFile}.${hint}`,
    );
  }
  const checkout = git(["-C", repoDir, "checkout", "-q", "FETCH_HEAD"]);
  if (!checkout.ok) return checkout;
  const result: FetchResult = {
    ...source,
    commit: head.value.trim(),
    tree: tree.value.trim(),
    skillDir: path.join(repoDir, source.path),
    out,
  };
  fs.writeFileSync(path.join(out, "fetch.json"), `${JSON.stringify(result, null, 2)}\n`);
  return ok(result);
}

/** Changed-line count between two folders; binary changes count as one line each. */
function lineDistance(a: string, b: string): Result<number> {
  const r = spawnSync("git", ["diff", "--no-index", "--numstat", "--", a, b], { encoding: "utf8", maxBuffer: 64 * 1024 * 1024 });
  if (r.error !== undefined) return fail("git", `git diff could not start: ${r.error.message}`);
  if (r.status !== 0 && r.status !== 1) return fail("git", `git diff --no-index failed: ${r.stderr.trim()}`);
  let total = 0;
  for (const line of r.stdout.split("\n")) {
    const [added, deleted] = line.split("\t");
    if (added === undefined || deleted === undefined) continue;
    total += added === "-" ? 1 : Number(added) + Number(deleted);
  }
  return ok(total);
}

export type ClosestResult = {
  readonly fetch: FetchResult;
  readonly distance: number;
  readonly candidates: readonly { readonly commit: string; readonly distance: number }[];
};

/**
 * Finds the upstream commit whose copy of the skill folder is nearest to
 * `localDir`: the likely point the local copy was taken from. Checks the
 * newest `max` commits touching the path (listed via `gh api`), leaves that
 * commit checked out in `out`, and writes `<out>/fetch.json` for it, so the
 * diff from it to `localDir` isolates the local edits.
 */
export function closest(source: ResolvedSource, localDir: string, out: string, max: number): Result<ClosestResult> {
  const listing = spawnSync(
    "gh",
    ["api", "-X", "GET", `repos/${source.owner}/${source.repo}/commits`, "-f", `path=${source.path}`, "-f", `sha=${source.ref}`, "-f", `per_page=${max}`, "--jq", ".[].sha"],
    { encoding: "utf8" },
  );
  if (listing.error !== undefined) return fail("git", `gh could not start: ${listing.error.message}; closest needs an authenticated gh`);
  if (listing.status !== 0) return fail("git", `gh api commits for ${source.owner}/${source.repo}:${source.path} failed: ${listing.stderr.trim()}`);
  const commits = listing.stdout.split("\n").filter((s) => SHA_RE.test(s));
  const [newest] = commits;
  if (newest === undefined) return fail("not-found", `no commits on ${source.ref} touch ${source.path}`);

  const fetched = fetchSkill(source, newest, out);
  if (!fetched.ok) return fetched;
  const repoDir = path.join(out, "repo");
  const older = commits.slice(1);
  if (older.length > 0) {
    const more = git(["-C", repoDir, "fetch", "-q", "--depth", "1", "--filter=blob:none", "--no-tags", "origin", ...older]);
    if (!more.ok) return more;
  }

  const candidates: { commit: string; distance: number }[] = [];
  for (const commit of commits) {
    const checkout = git(["-C", repoDir, "checkout", "-q", "--detach", commit]);
    if (!checkout.ok) return checkout;
    if (!fs.existsSync(path.join(fetched.value.skillDir, "SKILL.md"))) continue;
    const distance = lineDistance(fetched.value.skillDir, localDir);
    if (!distance.ok) return distance;
    candidates.push({ commit, distance: distance.value });
  }
  // Newest wins ties: `commits` is newest-first and the reduce keeps the first minimum.
  const best = candidates.reduce<{ commit: string; distance: number } | null>(
    (acc, c) => (acc === null || c.distance < acc.distance ? c : acc),
    null,
  );
  if (best === null) return fail("not-found", `none of the ${commits.length} commits touching ${source.path} has a SKILL.md there`);
  const checkout = git(["-C", repoDir, "checkout", "-q", "--detach", best.commit]);
  if (!checkout.ok) return checkout;
  const tree = git(["rev-parse", source.path === "" ? `${best.commit}^{tree}` : `${best.commit}:${source.path}`], repoDir);
  if (!tree.ok) return tree;
  const result: FetchResult = { ...fetched.value, commit: best.commit, tree: tree.value.trim() };
  fs.writeFileSync(path.join(out, "fetch.json"), `${JSON.stringify(result, null, 2)}\n`);
  return ok({ fetch: result, distance: best.distance, candidates });
}

function readFetch(out: string): Result<FetchResult> {
  const file = path.join(out, "fetch.json");
  if (!fs.existsSync(file)) return fail("not-found", `${file} does not exist; run fetch with --out ${out} first`);
  const parsed: unknown = JSON.parse(fs.readFileSync(file, "utf8"));
  if (
    !isRecord(parsed) ||
    typeof parsed["owner"] !== "string" ||
    typeof parsed["repo"] !== "string" ||
    typeof parsed["ref"] !== "string" ||
    typeof parsed["path"] !== "string" ||
    typeof parsed["commit"] !== "string" ||
    typeof parsed["tree"] !== "string" ||
    typeof parsed["skillDir"] !== "string"
  ) {
    return fail("io", `${file} is not a fetch result written by this script`);
  }
  return ok({
    owner: parsed["owner"],
    repo: parsed["repo"],
    ref: parsed["ref"],
    path: parsed["path"],
    commit: parsed["commit"],
    tree: parsed["tree"],
    skillDir: parsed["skillDir"],
    out,
  });
}

// --- Files ----------------------------------------------------------------

type Entry =
  | { readonly kind: "file"; readonly rel: string; readonly abs: string }
  | { readonly kind: "symlink"; readonly rel: string; readonly abs: string }
  | { readonly kind: "other"; readonly rel: string; readonly abs: string };

/** VCS and runtime byproducts: never part of a skill, never copied or compared. */
const IGNORED_NAMES: ReadonlySet<string> = new Set([".git", "__pycache__", "node_modules", ".DS_Store"]);

/** Every non-directory under `root`, skipping `IGNORED_NAMES`. Symlinks are not followed. */
function walk(root: string): Entry[] {
  const entries: Entry[] = [];
  const visit = (dir: string): void => {
    for (const dirent of fs.readdirSync(dir, { withFileTypes: true })) {
      if (IGNORED_NAMES.has(dirent.name)) continue;
      const abs = path.join(dir, dirent.name);
      const rel = path.relative(root, abs).split(path.sep).join("/");
      if (dirent.isDirectory()) visit(abs);
      else if (dirent.isSymbolicLink()) entries.push({ kind: "symlink", rel, abs });
      else if (dirent.isFile()) entries.push({ kind: "file", rel, abs });
      else entries.push({ kind: "other", rel, abs });
    }
  };
  visit(root);
  return entries.sort((a, b) => a.rel.localeCompare(b.rel));
}

const isBinary = (data: Buffer): boolean => data.subarray(0, 8192).includes(0);

/** Where a symlink points, and whether that escapes `root`. */
function symlinkEscapes(root: string, abs: string): boolean {
  const target = path.resolve(path.dirname(abs), fs.readlinkSync(abs));
  const base = path.resolve(root);
  return target !== base && !target.startsWith(base + path.sep);
}

// --- Frontmatter: metadata.author -----------------------------------------

const LOGIN_RE = OWNER_RE;

type Split = { readonly fm: string[]; readonly rest: string[] };

function splitFrontmatter(content: string): Result<Split> {
  if (content.includes("\r")) return fail("frontmatter", "file uses CRLF line endings; convert to LF first");
  const lines = content.split("\n");
  if (lines[0] !== "---") return fail("frontmatter", "file does not start with a --- frontmatter line");
  const close = lines.findIndex((line, i) => i > 0 && line.trimEnd() === "---");
  if (close === -1) return fail("frontmatter", "frontmatter has no closing --- line");
  return ok({ fm: lines.slice(1, close), rest: lines.slice(close) });
}

/** The `metadata:` block: its header index and the indented lines under it. */
function metadataBlock(fm: readonly string[]): Result<{ header: number; end: number; indent: string } | null> {
  const header = fm.findIndex((line) => /^metadata:/.test(line));
  if (header === -1) return ok(null);
  const headerLine = fm[header] ?? "";
  if (!/^metadata:\s*(#.*)?$/.test(headerLine)) {
    return fail("frontmatter", `metadata has an inline value ("${headerLine}"); rewrite it as an indented block`);
  }
  let end = header + 1;
  while (end < fm.length && (fm[end] === "" || /^\s/.test(fm[end] ?? ""))) end++;
  const firstChild = fm.slice(header + 1, end).find((line) => line.trim() !== "");
  const indent = firstChild === undefined ? "  " : (/^\s+/.exec(firstChild)?.[0] ?? "  ");
  return ok({ header, end, indent });
}

export const Frontmatter = {
  getAuthor(content: string): Result<string | null> {
    const split = splitFrontmatter(content);
    if (!split.ok) return split;
    const block = metadataBlock(split.value.fm);
    if (!block.ok) return block;
    if (block.value === null) return ok(null);
    const { header, end, indent } = block.value;
    for (const line of split.value.fm.slice(header + 1, end)) {
      if (line.startsWith(`${indent}author:`)) return ok(line.slice(`${indent}author:`.length).trim());
    }
    return ok(null);
  },

  /** Sets `metadata.author`, keeping every other key and line as-is. Idempotent. */
  setAuthor(content: string, login: string): Result<string> {
    if (!LOGIN_RE.test(login)) return fail("usage", `"${login}" is not a valid GitHub login`);
    const split = splitFrontmatter(content);
    if (!split.ok) return split;
    const fm = [...split.value.fm];
    const block = metadataBlock(fm);
    if (!block.ok) return block;
    if (block.value === null) {
      fm.push("metadata:", `  author: ${login}`);
    } else {
      const { header, end, indent } = block.value;
      const line = `${indent}author: ${login}`;
      const existing = fm.findIndex((l, i) => i > header && i < end && l.startsWith(`${indent}author:`));
      if (existing === -1) fm.splice(header + 1, 0, line);
      else fm[existing] = line;
    }
    return ok(["---", ...fm, ...split.value.rest].join("\n"));
  },
} as const;

function stampAuthor(skillDir: string, login: string): Result<string> {
  const file = path.join(skillDir, "SKILL.md");
  if (!fs.existsSync(file)) return fail("not-found", `${file} does not exist`);
  const updated = Frontmatter.setAuthor(fs.readFileSync(file, "utf8"), login);
  if (!updated.ok) return fail(updated.error.kind, `${file}: ${updated.error.message}`);
  fs.writeFileSync(file, updated.value);
  return ok(file);
}

// --- scan -----------------------------------------------------------------

export type Severity = "block" | "warn";

export type Finding = {
  readonly severity: Severity;
  readonly rule: string;
  readonly file: string;
  readonly line: number | null;
  readonly detail: string;
};

type LineRule = {
  readonly rule: string;
  readonly severity: Severity;
  readonly pattern: RegExp;
  readonly markdownOnly: boolean;
};

// Zero-width, bidi-override, and Unicode tag characters: invisible text a
// reviewer reading the file will not see but the model will.
const HIDDEN_CHARS = /[​-‏‪-‮⁠-⁤⁦-⁩﻿\u{E0000}-\u{E007F}]/u;
const HIDDEN_CHARS_G = new RegExp(HIDDEN_CHARS.source, "gu");

const LINE_RULES: readonly LineRule[] = [
  { rule: "hidden-unicode", severity: "block", pattern: HIDDEN_CHARS, markdownOnly: false },
  // Claude Code runs `!` + backtick-wrapped commands when the skill loads,
  // before anyone reviews what the command did.
  { rule: "load-time-command", severity: "block", pattern: /!`[^`\n]+`/, markdownOnly: true },
  { rule: "html-comment", severity: "warn", pattern: /<!--/, markdownOnly: true },
  { rule: "pipe-to-shell", severity: "warn", pattern: /\b(curl|wget)\b[^|\n]*\|\s*(ba|z)?sh\b/, markdownOnly: false },
  {
    rule: "decode-payload",
    severity: "warn",
    pattern: /base64\s+(-d|--decode)\b|\batob\s*\(|Buffer\.from\([^)]*['"]base64['"]/,
    markdownOnly: false,
  },
  {
    rule: "credential-access",
    severity: "warn",
    pattern:
      /~\/\.ssh\b|\.aws\/credentials|\.netrc\b|\bid_(rsa|ed25519)\b|\b(GITHUB_TOKEN|GH_TOKEN|NPM_TOKEN|ANTHROPIC_API_KEY|OPENAI_API_KEY|AWS_SECRET_ACCESS_KEY)\b/,
    markdownOnly: false,
  },
  { rule: "eval", severity: "warn", pattern: /\beval\s*[("$]/, markdownOnly: false },
];

const MAX_FILE_BYTES = 512 * 1024;

const excerpt = (line: string): string =>
  line
    .trim()
    .replace(HIDDEN_CHARS_G, (ch) => `\\u{${(ch.codePointAt(0) ?? 0).toString(16).toUpperCase()}}`)
    .slice(0, 160);

/** Top-level frontmatter keys that grant the skill power at load time. */
function frontmatterFindings(file: string, content: string): Finding[] {
  const split = splitFrontmatter(content);
  if (!split.ok) return [{ severity: "warn", rule: "frontmatter", file, line: 1, detail: split.error.message }];
  const findings: Finding[] = [];
  const fm = split.value.fm;
  fm.forEach((line, i) => {
    if (/^hooks:/.test(line)) {
      findings.push({ severity: "block", rule: "frontmatter-hooks", file, line: i + 2, detail: "registers hooks that keep running for the rest of the session" });
    }
    if (/^allowed-tools:/.test(line)) {
      let end = i + 1;
      while (end < fm.length && /^\s/.test(fm[end] ?? "")) end++;
      const value = fm.slice(i, end).join(" ");
      if (/(^|[\s,[:-])Bash\s*(,|\]|$|\(\s*\*\s*\))/.test(value) || /\bBash\(\s*(sh|bash|node|python3?|curl)\b/.test(value)) {
        findings.push({ severity: "warn", rule: "broad-allowed-tools", file, line: i + 2, detail: excerpt(value) });
      }
    }
  });
  return findings;
}

export function scan(root: string): { readonly files: number; readonly blocked: boolean; readonly findings: Finding[] } {
  const findings: Finding[] = [];
  const entries = walk(root);
  for (const entry of entries) {
    if (entry.kind === "symlink") {
      const escapes = symlinkEscapes(root, entry.abs);
      findings.push({
        severity: escapes ? "block" : "warn",
        rule: escapes ? "symlink-escape" : "symlink",
        file: entry.rel,
        line: null,
        detail: `-> ${fs.readlinkSync(entry.abs)}`,
      });
      continue;
    }
    if (entry.kind === "other") {
      findings.push({ severity: "block", rule: "special-file", file: entry.rel, line: null, detail: "not a regular file, directory, or symlink" });
      continue;
    }
    const stat = fs.statSync(entry.abs);
    if ((stat.mode & 0o111) !== 0) {
      findings.push({ severity: "warn", rule: "executable", file: entry.rel, line: null, detail: "has the executable bit" });
    }
    if (stat.size > MAX_FILE_BYTES) {
      findings.push({ severity: "warn", rule: "oversized", file: entry.rel, line: null, detail: `${stat.size} bytes` });
    }
    const data = fs.readFileSync(entry.abs);
    if (isBinary(data)) {
      findings.push({ severity: "warn", rule: "binary", file: entry.rel, line: null, detail: "binary content; not scanned" });
      continue;
    }
    const text = data.toString("utf8").replace(/^﻿/, "");
    const markdown = /\.mdx?$/i.test(entry.rel);
    text.split("\n").forEach((line, i) => {
      for (const rule of LINE_RULES) {
        if ((!rule.markdownOnly || markdown) && rule.pattern.test(line)) {
          findings.push({ severity: rule.severity, rule: rule.rule, file: entry.rel, line: i + 1, detail: excerpt(line) });
        }
      }
    });
    if (entry.rel === "SKILL.md") findings.push(...frontmatterFindings(entry.rel, text));
  }
  return { files: entries.length, blocked: findings.some((f) => f.severity === "block"), findings };
}

// --- compare and merge ----------------------------------------------------

type Snapshot = ReadonlyMap<string, { readonly data: Buffer; readonly mode: number }>;

/** Regular files under `root`, with SKILL.md's author normalised when `author` is given. */
function snapshot(root: string, author: string | null): Result<Snapshot> {
  const files = new Map<string, { data: Buffer; mode: number }>();
  for (const entry of walk(root)) {
    if (entry.kind !== "file") {
      return fail("io", `${path.join(root, entry.rel)} is a ${entry.kind}; resolve it before comparing`);
    }
    let data = fs.readFileSync(entry.abs);
    if (author !== null && entry.rel === "SKILL.md") {
      const stamped = Frontmatter.setAuthor(data.toString("utf8"), author);
      if (!stamped.ok) return fail(stamped.error.kind, `${entry.abs}: ${stamped.error.message}`);
      data = Buffer.from(stamped.value);
    }
    files.set(entry.rel, { data, mode: fs.statSync(entry.abs).mode & 0o777 });
  }
  return ok(files);
}

const sameBytes = (a: Buffer | undefined, b: Buffer | undefined): boolean =>
  a === undefined ? b === undefined : b !== undefined && a.equals(b);

export type CompareStatus = "equal" | "differs" | "only-local" | "only-upstream";

export function compare(local: string, upstream: string, author: string | null): Result<{ file: string; status: CompareStatus }[]> {
  const a = snapshot(local, author);
  if (!a.ok) return a;
  const b = snapshot(upstream, author);
  if (!b.ok) return b;
  const files = [...new Set([...a.value.keys(), ...b.value.keys()])].sort();
  return ok(
    files.map((file) => {
      const x = a.value.get(file)?.data;
      const y = b.value.get(file)?.data;
      const status: CompareStatus =
        x === undefined ? "only-upstream" : y === undefined ? "only-local" : x.equals(y) ? "equal" : "differs";
      return { file, status };
    }),
  );
}

export type MergeStatus =
  | "unchanged"
  | "ours-only"
  | "theirs-only"
  | "clean"
  | "conflict"
  | "added-upstream"
  | "deleted-upstream"
  | "deleted-locally";

export type MergeEntry = { readonly file: string; readonly status: MergeStatus; readonly detail: string | null };

function mergeText(ours: Buffer, base: Buffer, theirs: Buffer): Result<{ data: Buffer; conflicts: number }> {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "vendor-merge-"));
  try {
    const o = path.join(tmp, "local");
    const b = path.join(tmp, "base");
    const t = path.join(tmp, "upstream");
    fs.writeFileSync(o, ours);
    fs.writeFileSync(b, base);
    fs.writeFileSync(t, theirs);
    const r = spawnSync("git", ["merge-file", "-p", "-L", "local", "-L", "base", "-L", "upstream", o, b, t], {
      maxBuffer: 64 * 1024 * 1024,
    });
    if (r.error !== undefined) return fail("git", `git merge-file could not start: ${r.error.message}`);
    if (r.status === null || r.status < 0 || r.status > 127) {
      return fail("git", `git merge-file failed (${r.status ?? "signal"}): ${r.stderr.toString().trim()}`);
    }
    return ok({ data: r.stdout, conflicts: r.status });
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

/**
 * 3-way merge of skill folders: base is upstream at the last sync, ours the
 * local copy, theirs upstream now. Writes the result to `out`; conflicted text
 * files carry git conflict markers, and conflicted binary or delete/modify
 * files keep the local side.
 */
export function merge(baseDir: string, oursDir: string, theirsDir: string, out: string, author: string | null): Result<MergeEntry[]> {
  if (fs.existsSync(out)) return fail("io", `${out} already exists; pass a fresh --out`);
  const snaps = [snapshot(baseDir, author), snapshot(oursDir, author), snapshot(theirsDir, author)] as const;
  const [base, ours, theirs] = snaps;
  if (!base.ok) return base;
  if (!ours.ok) return ours;
  if (!theirs.ok) return theirs;
  const files = [...new Set([...base.value.keys(), ...ours.value.keys(), ...theirs.value.keys()])].sort();
  const entries: MergeEntry[] = [];
  const write = (file: string, data: Buffer, mode: number): void => {
    const dest = path.join(out, file);
    fs.mkdirSync(path.dirname(dest), { recursive: true });
    fs.writeFileSync(dest, data, { mode });
  };
  fs.mkdirSync(out, { recursive: true });
  for (const file of files) {
    const b = base.value.get(file);
    const o = ours.value.get(file);
    const t = theirs.value.get(file);
    if (sameBytes(o?.data, t?.data)) {
      if (o !== undefined) write(file, o.data, o.mode);
      entries.push({ file, status: "unchanged", detail: null });
    } else if (sameBytes(b?.data, o?.data)) {
      if (t === undefined) entries.push({ file, status: "deleted-upstream", detail: null });
      else {
        write(file, t.data, t.mode);
        entries.push({ file, status: b === undefined ? "added-upstream" : "theirs-only", detail: null });
      }
    } else if (sameBytes(b?.data, t?.data)) {
      if (o === undefined) entries.push({ file, status: "deleted-locally", detail: null });
      else {
        write(file, o.data, o.mode);
        entries.push({ file, status: "ours-only", detail: null });
      }
    } else if (o === undefined || t === undefined) {
      if (o !== undefined) write(file, o.data, o.mode);
      const detail = o === undefined ? "deleted locally, changed upstream; kept deleted" : "changed locally, deleted upstream; kept local";
      entries.push({ file, status: "conflict", detail });
    } else if (isBinary(o.data) || isBinary(t.data) || (b !== undefined && isBinary(b.data))) {
      write(file, o.data, o.mode);
      entries.push({ file, status: "conflict", detail: "binary changed on both sides; kept local" });
    } else {
      const merged = mergeText(o.data, b?.data ?? Buffer.alloc(0), t.data);
      if (!merged.ok) return merged;
      write(file, merged.value.data, o.mode);
      entries.push(
        merged.value.conflicts === 0
          ? { file, status: "clean", detail: null }
          : { file, status: "conflict", detail: `${merged.value.conflicts} conflict hunk(s) marked in the file` },
      );
    }
  }
  return ok(entries);
}

// --- install --------------------------------------------------------------

/**
 * Copies a skill folder as real files. Symlinks inside the folder are
 * dereferenced; any that escape it, or any special file, refuse the install
 * outright so a reviewer override cannot copy files from elsewhere on disk.
 */
export function install(src: string, dest: string, replace: boolean, author: string | null): Result<{ dest: string; files: number }> {
  if (!fs.existsSync(path.join(src, "SKILL.md"))) return fail("not-found", `${src} has no SKILL.md`);
  const entries = walk(src);
  const unsafe = entries.filter((e) => e.kind === "other" || (e.kind === "symlink" && symlinkEscapes(src, e.abs)));
  if (unsafe.length > 0) {
    return fail("io", `refusing to install: ${unsafe.map((e) => e.rel).join(", ")} escape the skill folder or are special files`);
  }
  if (fs.existsSync(dest)) {
    if (!replace) return fail("io", `${dest} already exists; pass --replace to overwrite it`);
    fs.rmSync(dest, { recursive: true });
  }
  fs.cpSync(src, dest, { recursive: true, dereference: true, filter: (p) => path.basename(p) !== ".git" });
  if (author !== null) {
    const stamped = stampAuthor(dest, author);
    if (!stamped.ok) return stamped;
  }
  return ok({ dest, files: entries.length });
}

// --- root: where global skills live ---------------------------------------

export type SkillsRoot = {
  /** Where the skill sources live (a repo dir when ~/.claude/skills is a symlink farm). */
  readonly root: string;
  /** Where Claude Code reads skills from. */
  readonly published: string;
  /** True when a new skill under `root` is not live until the repo's publish step links it. */
  readonly needsPublish: boolean;
  readonly categories: string[];
  /** Skill name -> path relative to `root`. */
  readonly skills: Record<string, string>;
};

function commonDir(dirs: readonly string[]): string {
  const split = dirs.map((d) => d.split(path.sep));
  const first = split[0] ?? [];
  let n = first.length;
  for (const parts of split) {
    let i = 0;
    while (i < n && parts[i] === first[i]) i++;
    n = i;
  }
  return first.slice(0, n).join(path.sep) || path.sep;
}

export function resolveRoot(published: string): Result<SkillsRoot> {
  if (!fs.existsSync(published)) return fail("not-found", `${published} does not exist`);
  const targets = fs
    .readdirSync(published, { withFileTypes: true })
    .filter((d) => d.isSymbolicLink())
    .map((d) => fs.realpathSync(path.join(published, d.name)))
    .filter((t) => fs.existsSync(path.join(t, "SKILL.md")));
  const root = targets.length === 0 ? fs.realpathSync(published) : commonDir(targets.map((t) => path.dirname(t)));
  const categories: string[] = [];
  const skills: Record<string, string> = {};
  for (const dirent of fs.readdirSync(root, { withFileTypes: true })) {
    if (!dirent.isDirectory() || dirent.name.startsWith(".")) continue;
    const dir = path.join(root, dirent.name);
    if (fs.existsSync(path.join(dir, "SKILL.md"))) {
      skills[dirent.name] = dirent.name;
      continue;
    }
    const children = fs
      .readdirSync(dir, { withFileTypes: true })
      .filter((c) => c.isDirectory() && fs.existsSync(path.join(dir, c.name, "SKILL.md")));
    if (children.length === 0) continue;
    categories.push(dirent.name);
    for (const child of children) skills[child.name] = `${dirent.name}/${child.name}`;
  }
  return ok({ root, published, needsPublish: root !== fs.realpathSync(published), categories: categories.sort(), skills });
}

const claudeDir = (): string => process.env["CLAUDE_CONFIG_DIR"] ?? path.join(os.homedir(), ".claude");

// --- Catalog --------------------------------------------------------------

export type SkillStatus = { readonly state: "tracked" } | { readonly state: "removed-upstream"; readonly detectedAt: string };

export type CatalogEntry = {
  readonly author: string;
  readonly source: { readonly repo: string; readonly ref: string; readonly path: string };
  /** Path relative to the skills root. */
  readonly localPath: string;
  /** Upstream state the local copy was last reconciled with; the base of the next 3-way merge. */
  readonly sync: { readonly commit: string; readonly tree: string; readonly at: string };
  readonly status: SkillStatus;
  /** Why the local copy deliberately differs from upstream, one note per change. */
  readonly localChanges: readonly string[];
};

export type Catalog = {
  readonly version: 1;
  readonly trustedAuthors: readonly string[];
  readonly skills: Readonly<Record<string, CatalogEntry>>;
};

export const CATALOG_FILE = "vendored-skills.json";

const EMPTY_CATALOG: Catalog = { version: 1, trustedAuthors: [], skills: {} };

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === "object" && v !== null && !Array.isArray(v);

function field(obj: Record<string, unknown>, key: string, where: string): Result<string> {
  const v = obj[key];
  return typeof v === "string" && v !== "" ? ok(v) : fail("catalog", `${where}.${key}: expected a non-empty string`);
}

function parseEntry(v: unknown, where: string): Result<CatalogEntry> {
  if (!isRecord(v)) return fail("catalog", `${where}: expected an object`);
  const { source, sync, status, localChanges } = v;
  if (!isRecord(source)) return fail("catalog", `${where}.source: expected an object`);
  if (!isRecord(sync)) return fail("catalog", `${where}.sync: expected an object`);
  if (!isRecord(status)) return fail("catalog", `${where}.status: expected an object`);
  if (!Array.isArray(localChanges) || !localChanges.every((n): n is string => typeof n === "string")) {
    return fail("catalog", `${where}.localChanges: expected an array of strings`);
  }
  const author = field(v, "author", where);
  if (!author.ok) return author;
  const repo = field(source, "repo", `${where}.source`);
  if (!repo.ok) return repo;
  const ref = field(source, "ref", `${where}.source`);
  if (!ref.ok) return ref;
  const sourcePath = source["path"];
  if (typeof sourcePath !== "string") return fail("catalog", `${where}.source.path: expected a string`);
  const localPath = field(v, "localPath", where);
  if (!localPath.ok) return localPath;
  const commit = field(sync, "commit", `${where}.sync`);
  if (!commit.ok) return commit;
  const tree = field(sync, "tree", `${where}.sync`);
  if (!tree.ok) return tree;
  const at = field(sync, "at", `${where}.sync`);
  if (!at.ok) return at;
  if (!SHA_RE.test(commit.value) || !SHA_RE.test(tree.value)) {
    return fail("catalog", `${where}.sync: commit and tree must be 40-hex SHAs`);
  }
  let parsedStatus: SkillStatus;
  if (status["state"] === "tracked") parsedStatus = { state: "tracked" };
  else if (status["state"] === "removed-upstream" && typeof status["detectedAt"] === "string") {
    parsedStatus = { state: "removed-upstream", detectedAt: status["detectedAt"] };
  } else return fail("catalog", `${where}.status: expected {state:"tracked"} or {state:"removed-upstream",detectedAt}`);
  return ok({
    author: author.value,
    source: { repo: repo.value, ref: ref.value, path: sourcePath },
    localPath: localPath.value,
    sync: { commit: commit.value, tree: tree.value, at: at.value },
    status: parsedStatus,
    localChanges,
  });
}

export const Catalog = {
  parse(text: string): Result<Catalog> {
    let raw: unknown;
    try {
      raw = JSON.parse(text);
    } catch (e) {
      return fail("catalog", `not valid JSON: ${e instanceof Error ? e.message : String(e)}`);
    }
    if (!isRecord(raw) || raw["version"] !== 1) return fail("catalog", "expected an object with version: 1");
    const { trustedAuthors, skills } = raw;
    if (!Array.isArray(trustedAuthors) || !trustedAuthors.every((a): a is string => typeof a === "string")) {
      return fail("catalog", "trustedAuthors: expected an array of strings");
    }
    if (!isRecord(skills)) return fail("catalog", "skills: expected an object");
    const parsed: Record<string, CatalogEntry> = {};
    for (const [name, entry] of Object.entries(skills)) {
      const e = parseEntry(entry, `skills.${name}`);
      if (!e.ok) return e;
      parsed[name] = e.value;
    }
    return ok({ version: 1, trustedAuthors, skills: parsed });
  },

  /** Stable output: sorted skill names and authors, so diffs stay small. */
  serialize(catalog: Catalog): string {
    const skills = Object.fromEntries(Object.entries(catalog.skills).sort(([a], [b]) => a.localeCompare(b)));
    const trustedAuthors = [...new Set(catalog.trustedAuthors)].sort();
    return `${JSON.stringify({ version: 1, trustedAuthors, skills }, null, 2)}\n`;
  },

  load(file: string): Result<Catalog> {
    if (!fs.existsSync(file)) return ok(EMPTY_CATALOG);
    const parsed = Catalog.parse(fs.readFileSync(file, "utf8"));
    return parsed.ok ? parsed : fail("catalog", `${file}: ${parsed.error.message}`);
  },
} as const;

const today = (): string => new Date().toISOString().slice(0, 10);

// --- CLI ------------------------------------------------------------------

const USAGE = `usage: node vendor.ts <command> [args]

  root                                      resolve the global skills root, its categories and skills
  fetch <source> [--commit sha] [--out dir] fetch one skill folder into quarantine (writes <out>/fetch.json)
  closest <source> <local-dir> [--max n] [--out dir]
                                            find the upstream commit nearest the local copy (needs gh);
                                            leaves it checked out with <out>/fetch.json
  scan <dir>                                deterministic security findings for a skill folder
  compare <local> <upstream> [--author x]   per-file equal/differs, ignoring metadata.author
  merge <base> <ours> <theirs> --out <dir> [--author x]
                                            3-way merge of skill folders into <dir>
  install <src> <dest> [--replace] [--author x]
                                            copy a skill folder as real files, then stamp the author
  author <skill-dir> <login>                set metadata.author in SKILL.md
  catalog list | show <name> | remove <name> | mark-removed <name> | trust <author> | untrust <author>
  catalog upsert <name> --fetch <out> --local <path> [--note text]... [--clear-notes]
                                            (catalog commands take [--catalog file]; default <root>/${CATALOG_FILE})`;

function print(value: unknown): void {
  process.stdout.write(`${JSON.stringify(value, null, 2)}\n`);
}

function catalogCommand(args: readonly string[]): Result<unknown> {
  const { values, positionals } = parseArgs({
    args: [...args],
    allowPositionals: true,
    options: {
      catalog: { type: "string" },
      fetch: { type: "string" },
      local: { type: "string" },
      note: { type: "string", multiple: true },
      "clear-notes": { type: "boolean" },
    },
  });
  const [action, target] = positionals;
  let file = values.catalog;
  if (file === undefined) {
    const root = resolveRoot(path.join(claudeDir(), "skills"));
    if (!root.ok) return root;
    file = path.join(root.value.root, CATALOG_FILE);
  }
  const loaded = Catalog.load(file);
  if (!loaded.ok) return loaded;
  const catalog = loaded.value;
  const save = (next: Catalog): Result<unknown> => {
    fs.writeFileSync(file, Catalog.serialize(next));
    return ok({ catalog: file, ...(target === undefined ? {} : { changed: target }) });
  };
  const requireEntry = (name: string | undefined): Result<[string, CatalogEntry]> => {
    if (name === undefined) return fail("usage", `catalog ${action ?? ""} needs a skill name`);
    const entry = catalog.skills[name];
    return entry === undefined ? fail("not-found", `${name} is not in ${file}`) : ok([name, entry]);
  };
  const withoutSkill = (name: string): Record<string, CatalogEntry> =>
    Object.fromEntries(Object.entries(catalog.skills).filter(([n]) => n !== name));

  switch (action) {
    case "list":
      return ok({ catalog: file, ...catalog });
    case "show": {
      const entry = requireEntry(target);
      return entry.ok ? ok(entry.value[1]) : entry;
    }
    case "remove": {
      const entry = requireEntry(target);
      if (!entry.ok) return entry;
      return save({ ...catalog, skills: withoutSkill(entry.value[0]) });
    }
    case "mark-removed": {
      const entry = requireEntry(target);
      if (!entry.ok) return entry;
      const [name, e] = entry.value;
      return save({ ...catalog, skills: { ...catalog.skills, [name]: { ...e, status: { state: "removed-upstream", detectedAt: today() } } } });
    }
    case "trust":
    case "untrust": {
      if (target === undefined || !LOGIN_RE.test(target)) return fail("usage", `catalog ${action} needs a GitHub login`);
      const trustedAuthors =
        action === "trust" ? [...catalog.trustedAuthors, target] : catalog.trustedAuthors.filter((a) => a !== target);
      return save({ ...catalog, trustedAuthors });
    }
    case "upsert": {
      if (target === undefined || values.fetch === undefined || values.local === undefined) {
        return fail("usage", "catalog upsert <name> --fetch <out> --local <path> [--note text]... [--clear-notes]");
      }
      const fetched = readFetch(values.fetch);
      if (!fetched.ok) return fetched;
      const f = fetched.value;
      const previous = catalog.skills[target];
      const localChanges = values["clear-notes"] === true ? [] : (values.note ?? previous?.localChanges ?? []);
      const entry: CatalogEntry = {
        author: f.owner,
        source: { repo: `${f.owner}/${f.repo}`, ref: f.ref, path: f.path },
        localPath: values.local,
        sync: { commit: f.commit, tree: f.tree, at: today() },
        status: { state: "tracked" },
        localChanges,
      };
      return save({ ...catalog, skills: { ...catalog.skills, [target]: entry } });
    }
    default:
      return fail("usage", `unknown catalog action "${action ?? ""}"\n${USAGE}`);
  }
}

function run(argv: readonly string[]): Result<unknown> {
  const [command, ...rest] = argv;
  if (command === "catalog") return catalogCommand(rest);
  const { values, positionals } = parseArgs({
    args: [...rest],
    allowPositionals: true,
    options: {
      commit: { type: "string" },
      out: { type: "string" },
      author: { type: "string" },
      replace: { type: "boolean" },
      max: { type: "string" },
    },
  });
  const author = values.author ?? null;
  const need = (n: number): Result<string[]> =>
    positionals.length === n ? ok(positionals) : fail("usage", `${command ?? ""} takes ${n} argument(s)\n${USAGE}`);

  switch (command) {
    case "root":
      return resolveRoot(path.join(claudeDir(), "skills"));
    case "fetch": {
      const args = need(1);
      if (!args.ok) return args;
      const locator = Locator.parse(args.value[0] ?? "");
      if (!locator.ok) return locator;
      const source = resolveSource(locator.value);
      if (!source.ok) return source;
      if (values.commit !== undefined && !SHA_RE.test(values.commit)) return fail("usage", "--commit must be a 40-hex SHA");
      const leaf = source.value.path === "" ? source.value.repo : path.posix.basename(source.value.path);
      const out = values.out ?? path.join(cacheRoot(), `${Date.now()}-${leaf}`);
      return fetchSkill(source.value, values.commit ?? null, path.resolve(out));
    }
    case "closest": {
      const args = need(2);
      if (!args.ok) return args;
      const [src, local] = args.value;
      const locator = Locator.parse(src ?? "");
      if (!locator.ok) return locator;
      const source = resolveSource(locator.value);
      if (!source.ok) return source;
      const max = Number(values.max ?? "40");
      if (!Number.isInteger(max) || max < 1 || max > 100) return fail("usage", "--max must be an integer from 1 to 100");
      const leaf = source.value.path === "" ? source.value.repo : path.posix.basename(source.value.path);
      const out = values.out ?? path.join(cacheRoot(), `${Date.now()}-${leaf}-closest`);
      return closest(source.value, path.resolve(local ?? ""), path.resolve(out), max);
    }
    case "scan": {
      const args = need(1);
      return args.ok ? ok(scan(path.resolve(args.value[0] ?? ""))) : args;
    }
    case "compare": {
      const args = need(2);
      if (!args.ok) return args;
      const [local, upstream] = args.value.map((p) => path.resolve(p));
      return compare(local ?? "", upstream ?? "", author);
    }
    case "merge": {
      const args = need(3);
      if (!args.ok) return args;
      if (values.out === undefined) return fail("usage", "merge needs --out <dir>");
      const [base, ours, theirs] = args.value.map((p) => path.resolve(p));
      return merge(base ?? "", ours ?? "", theirs ?? "", path.resolve(values.out), author);
    }
    case "install": {
      const args = need(2);
      if (!args.ok) return args;
      const [src, dest] = args.value.map((p) => path.resolve(p));
      return install(src ?? "", dest ?? "", values.replace === true, author);
    }
    case "author": {
      const args = need(2);
      if (!args.ok) return args;
      const [dir, login] = args.value;
      return stampAuthor(path.resolve(dir ?? ""), login ?? "");
    }
    case undefined:
    case "help":
    case "--help":
      process.stdout.write(`${USAGE}\n`);
      return ok(undefined);
    default:
      return fail("usage", `unknown command "${command}"\n${USAGE}`);
  }
}

if (import.meta.main) {
  let result: Result<unknown>;
  try {
    result = run(process.argv.slice(2));
  } catch (e) {
    result = fail("io", e instanceof Error ? e.message : String(e));
  }
  if (result.ok) {
    if (result.value !== undefined) print(result.value);
  } else {
    process.stderr.write(`vendor.ts: ${result.error.kind}: ${result.error.message}\n`);
    process.exit(result.error.kind === "usage" ? 2 : 1);
  }
}
