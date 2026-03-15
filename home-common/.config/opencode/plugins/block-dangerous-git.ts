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
  [
    /\bgh\s+api\b.*(-X\s*(PUT|POST|PATCH|DELETE)|--method\s*(PUT|POST|PATCH|DELETE))/,
    "gh api with destructive HTTP method not allowed",
  ],
]

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
  },
})
