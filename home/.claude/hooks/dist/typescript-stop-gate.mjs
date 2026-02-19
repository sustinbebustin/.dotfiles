// src/typescript-stop-gate.ts
import { execSync } from "child_process";
import * as path from "path";
import * as fs from "fs";
function readStdin() {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("readable", () => {
      let chunk;
      while ((chunk = process.stdin.read()) !== null) {
        data += chunk;
      }
    });
    process.stdin.on("end", () => resolve(data));
  });
}
function findTsconfigRoot(startDir) {
  let current = startDir;
  while (current !== path.dirname(current)) {
    if (fs.existsSync(path.join(current, "tsconfig.json"))) {
      return current;
    }
    current = path.dirname(current);
  }
  return null;
}
async function main() {
  try {
    await readStdin();
    const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();
    const tsconfigRoot = findTsconfigRoot(projectDir);
    if (!tsconfigRoot) {
      console.log(JSON.stringify({}));
      return;
    }
    const hasTsFiles = fs.existsSync(path.join(tsconfigRoot, "tsconfig.json"));
    if (!hasTsFiles) {
      console.log(JSON.stringify({}));
      return;
    }
    try {
      execSync("npx tsc --noEmit --pretty false", {
        cwd: tsconfigRoot,
        timeout: 45e3,
        encoding: "utf8",
        stdio: ["pipe", "pipe", "pipe"]
      });
      console.log(JSON.stringify({}));
    } catch (tscError) {
      const err = tscError;
      if (!err.stdout && !err.stderr) {
        console.log(JSON.stringify({}));
        return;
      }
      const output = err.stdout || "";
      const errorLines = output.split("\n").filter((line) => line.includes("error TS")).slice(0, 15);
      if (errorLines.length === 0) {
        console.log(JSON.stringify({}));
        return;
      }
      const message = [
        `[BLOCKED] ${errorLines.length} TypeScript error(s) remain. Fix them before finishing.`,
        "",
        ...errorLines
      ].join("\n");
      console.log(JSON.stringify({
        decision: "block",
        reason: message
      }));
    }
  } catch {
    console.log(JSON.stringify({}));
  }
}
main();
