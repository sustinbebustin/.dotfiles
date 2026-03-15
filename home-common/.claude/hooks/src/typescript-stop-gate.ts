/**
 * Stop Hook: TypeScript Stop Gate
 *
 * Blocks the agent from finishing if there are unresolved TypeScript errors.
 * Runs tsc --noEmit on the project; if errors exist, blocks with the error list
 * so the agent must fix them before completing.
 */

import { execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';

function readStdin(): Promise<string> {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('readable', () => {
      let chunk;
      while ((chunk = process.stdin.read()) !== null) {
        data += chunk;
      }
    });
    process.stdin.on('end', () => resolve(data));
  });
}

function findTsconfigRoot(startDir: string): string | null {
  let current = startDir;
  while (current !== path.dirname(current)) {
    if (fs.existsSync(path.join(current, 'tsconfig.json'))) {
      return current;
    }
    current = path.dirname(current);
  }
  return null;
}

async function main() {
  try {
    // Consume stdin (required by hook protocol)
    await readStdin();

    const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();

    // Find nearest tsconfig.json from project root
    const tsconfigRoot = findTsconfigRoot(projectDir);
    if (!tsconfigRoot) {
      // No TypeScript project -- nothing to check
      console.log(JSON.stringify({}));
      return;
    }

    // Check if any .ts/.tsx files were touched in this project
    // (Skip check entirely for non-TS projects)
    const hasTsFiles = fs.existsSync(path.join(tsconfigRoot, 'tsconfig.json'));
    if (!hasTsFiles) {
      console.log(JSON.stringify({}));
      return;
    }

    try {
      execSync('npx tsc --noEmit --pretty false', {
        cwd: tsconfigRoot,
        timeout: 45000,
        encoding: 'utf8',
        stdio: ['pipe', 'pipe', 'pipe'],
      });

      // Exit code 0 -- no errors
      console.log(JSON.stringify({}));
    } catch (tscError: unknown) {
      const err = tscError as { stdout?: string; stderr?: string; status?: number };

      if (!err.stdout && !err.stderr) {
        // Something unexpected, don't block
        console.log(JSON.stringify({}));
        return;
      }

      // Parse tsc output for error lines
      const output = err.stdout || '';
      const errorLines = output
        .split('\n')
        .filter((line: string) => line.includes('error TS'))
        .slice(0, 15);

      if (errorLines.length === 0) {
        // No actual TS errors (maybe a different failure)
        console.log(JSON.stringify({}));
        return;
      }

      const message = [
        `[BLOCKED] ${errorLines.length} TypeScript error(s) remain. Fix them before finishing.`,
        '',
        ...errorLines,
      ].join('\n');

      console.log(JSON.stringify({
        decision: 'block',
        reason: message,
      }));
    }
  } catch {
    // Don't block on hook infrastructure errors
    console.log(JSON.stringify({}));
  }
}

main();
