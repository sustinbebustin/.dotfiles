#!/usr/bin/env node
/**
 * Convert a URL or local file to Markdown using `uvx markitdown`.
 *
 * markitdown fetches URLs on its own; this wrapper adds PDF support by default
 * and writes the result to a file so large documents stay out of stdout.
 *
 * Usage:
 *   node to-markdown.mjs <url-or-path> [--out <file> | --tmp]
 *
 * Without --out or --tmp the Markdown is printed to stdout.
 * With --out or --tmp only the written file's path is printed.
 */

import { existsSync, mkdirSync, writeFileSync } from 'fs';
import { basename, join } from 'path';
import { tmpdir } from 'os';
import { spawnSync } from 'child_process';

const USAGE = 'Usage: node to-markdown.mjs <url-or-path> [--out <file> | --tmp]';

function fail(message) {
  console.error(message);
  process.exit(1);
}

function isUrl(s) {
  return /^https?:\/\//i.test(s);
}

function makeTmpMdPath(input) {
  const dir = join(tmpdir(), 'claude-summarize');
  mkdirSync(dir, { recursive: true });
  const raw = isUrl(input) ? basename(new URL(input).pathname) : basename(input);
  const base = (raw || 'document').replace(/[^a-z0-9._-]+/gi, '_');
  const stamp = Date.now().toString(36);
  const rand = Math.random().toString(16).slice(2, 8);
  return join(dir, `${base}-${stamp}-${rand}.md`);
}

function parseArgs(argv) {
  let input = null;
  let destination = { kind: 'stdout' };

  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--out') {
      const path = argv[i + 1];
      if (!path || path.startsWith('--')) fail(`Expected a file path after --out.\n${USAGE}`);
      destination = { kind: 'file', path };
      i++;
    } else if (a === '--tmp') {
      destination = { kind: 'tmp' };
    } else if (a.startsWith('--')) {
      fail(`Unknown flag: ${a}\n${USAGE}`);
    } else if (input === null) {
      input = a;
    } else {
      fail(`Unexpected argument: ${a}. Quote paths containing spaces.\n${USAGE}`);
    }
  }

  if (input === null) fail(USAGE);
  return { input, destination };
}

function runMarkitdown(input) {
  // Always include the PDF extra: many document URLs (e.g. arXiv) are only
  // detected as PDFs after fetching, so extension-based switching is unreliable.
  const result = spawnSync('uvx', ['--from', 'markitdown[pdf]', 'markitdown', input], {
    encoding: 'utf-8',
    maxBuffer: 50 * 1024 * 1024
  });

  if (result.error) {
    fail(`Could not run uvx (${result.error.message}). Check that uv is installed and on PATH.`);
  }
  if (result.status !== 0) {
    const stderr = (result.stderr || '').trim();
    fail(`markitdown exited ${result.status} converting ${input}. Nothing was written.${stderr ? `\n${stderr}` : ''}`);
  }
  return result.stdout;
}

const { input, destination } = parseArgs(process.argv.slice(2));

if (!isUrl(input) && !existsSync(input)) {
  fail(`File not found: ${input}. Pass an http(s) URL or an existing local path.`);
}

const md = runMarkitdown(input);

switch (destination.kind) {
  case 'stdout':
    process.stdout.write(md);
    break;
  case 'file':
    writeFileSync(destination.path, md, 'utf-8');
    console.log(destination.path);
    break;
  case 'tmp': {
    const path = makeTmpMdPath(input);
    writeFileSync(path, md, 'utf-8');
    console.log(path);
    break;
  }
}
