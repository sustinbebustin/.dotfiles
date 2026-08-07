import type { Plugin } from "@opencode-ai/plugin"

const BLOCKED: Array<[RegExp, string]> = [
  [/\bgit\s+merge\b/, "git merge - branch merges not allowed"],
  [/\bgit\s+rebase\b/, "git rebase - rebasing not allowed"],
  [/\bgit\s+reset\s+--hard\b/, "git reset --hard - destructive reset not allowed"],
  [/\bgit\s+clean\b/, "git clean - file deletion not allowed"],
  [/\bgit\s+branch\s+-[dD]\b/, "git branch delete not allowed"],
  [/\bgit\s+checkout\s+--\b/, "git checkout -- discards changes, not allowed"],
  [/\bgit\s+restore\b/, "git restore - discarding changes not allowed"],
  [/\bgit\s+stash\s+(drop|clear)\b/, "git stash drop/clear - stash destruction not allowed"],
  [/\bgit\s+tag\s+-d\b/, "git tag delete not allowed"],
  [/\bgh\s+pr\s+merge\b/, "gh pr merge - PR merging not allowed"],
  [/\bgh\s+pr\s+close\b/, "gh pr close - PR closing not allowed"],
  [/\bgh\s+issue\s+(close|delete)\b/, "gh issue close/delete not allowed"],
  [/\bgh\s+release\s+(create|delete)\b/, "gh release create/delete not allowed"],
  [/\bgh\s+repo\s+(delete|rename)\b/, "gh repo delete/rename not allowed"],
]

const MUTATING_METHODS = new Set(["PUT", "POST", "PATCH", "DELETE"])

/** `-X POST`, `-XPOST`, `--method POST`, `--method=POST`. */
const EXPLICIT_METHOD = /(?:-X\s*|--method\s*=?\s*)([A-Za-z]+)/

/**
 * Payload flags, which make gh choose POST on its own: request parameters
 * (`-f k=v`, `-fk=v`, `-F k=v`, `--raw-field k=v`, `--field=k=v`) and a request
 * body (`--input file`, `--input=file`).
 */
const PAYLOAD_FLAG =
  /(?:^|\s)(?:-[fF]\S*|--raw-field(?:=\S*)?|--field(?:=\S*)?|--input(?:=\S*)?)(?=\s|$)/

/**
 * Mirrors checkGhAPI in the Claude Code hook
 * (home-common/.claude/hooks/block-dangerous-git). An explicit method flag
 * settles the verdict; otherwise a payload makes gh send POST on its own, so
 * those are writes carrying no method flag. An explicit read method keeps a
 * payload harmless, since gh then sends parameters as a query string.
 */
function checkGhApi(norm: string): string | null {
  if (!/\bgh\s+api\b/.test(norm)) return null

  const method = norm.match(EXPLICIT_METHOD)?.[1]?.toUpperCase()
  if (method !== undefined) {
    return MUTATING_METHODS.has(method) ? `gh api ${method} not allowed` : null
  }
  if (PAYLOAD_FLAG.test(norm)) {
    return "gh api sends POST when request parameters or a body are supplied - not allowed. Add `--method GET` if this is meant to be a read."
  }
  return null
}

function normalize(command: string): string {
  return command.replace(/\n/g, " ").replace(/\s+/g, " ").trimStart()
}

export const BlockDangerousGit: Plugin = async () => ({
  "tool.execute.before": async (input, output) => {
    if (input.tool !== "bash") return

    const command: unknown = output.args?.command
    if (typeof command !== "string") return

    const norm = normalize(command)

    // git push -> warn instead of block (no "ask" equivalent in plugins, so append warning)
    if (/\bgit\s+push\b/.test(norm)) {
      throw new Error(
        "[BLOCKED] git push detected - push manually to confirm intent.",
      )
    }

    for (const [pattern, reason] of BLOCKED) {
      if (pattern.test(norm)) {
        throw new Error(`[BLOCKED] ${reason}`)
      }
    }

    const apiReason = checkGhApi(norm)
    if (apiReason !== null) {
      throw new Error(`[BLOCKED] ${apiReason}`)
    }
  },
})
