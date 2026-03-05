import type { Plugin } from "@opencode-ai/plugin"

const ENV_FILE = /(^|\/)\.[^/]*\.env($|[^/]*$)|(^|\/)\.env($|\.[^/]*$)/
const SAFE_SUFFIX = /\.(example|sample|template)$/

const BASH_READ_ENV =
  /(cat|less|more|head|tail|bat|nano|vim|vi|code|subl|open)\s+[^|;&]*\.env/
const BASH_SOURCE_ENV = /(source|\.|export)[^|;&]*\.env/
const BASH_SEARCH_ENV = /(grep|awk|sed|xargs|find)[^|;&]*\.env/

function isBlockedEnv(filePath: string): boolean {
  return ENV_FILE.test(filePath) && !SAFE_SUFFIX.test(filePath)
}

function bashTargetsEnv(command: string): boolean {
  if (SAFE_SUFFIX.test(command)) return false
  return (
    BASH_READ_ENV.test(command) ||
    BASH_SOURCE_ENV.test(command) ||
    BASH_SEARCH_ENV.test(command)
  )
}

const BLOCK_MSG =
  "[BLOCKED] Access to .env files is blocked for security. Use .env.example as a reference."

export const BlockEnvFiles: Plugin = async () => ({
  "tool.execute.before": async (input, output) => {
    if (
      input.tool === "read" ||
      input.tool === "write" ||
      input.tool === "edit"
    ) {
      const filePath: unknown = output.args?.filePath
      if (typeof filePath === "string" && isBlockedEnv(filePath)) {
        throw new Error(BLOCK_MSG)
      }
    }

    if (input.tool === "bash") {
      const command: unknown = output.args?.command
      if (typeof command === "string" && bashTargetsEnv(command)) {
        throw new Error(BLOCK_MSG)
      }
    }
  },
})
