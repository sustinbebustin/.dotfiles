# Package Manager

pnpm is the only Node package manager on this machine. It provides `node` too. `npm`, `npx`, `yarn` and `corepack` are not installed, so a vendored skill that runs one of them fails with "command not found". Adapt those commands before the skill is recorded.

## Find

```
rg -n '\b(npx|npm|yarn|corepack)\b' <dir>
```

## Rewrite

Rewrite a command the agent or user would run:

| Upstream | Rewrite |
|----------|---------|
| `npx <pkg>[@ver] ...`, `yarn dlx`, `npm exec` for a one-off tool | `pnpm dlx <pkg>[@ver] ...` |
| `npx <bin>` where `<bin>` is a dependency of the project, e.g. `npx eslint src/` | `pnpm exec <bin> ...` |
| `npx -p <pkg> <bin>` | `pnpm --package=<pkg> dlx <bin>` |
| `npm install -g <pkg>` | `pnpm add -g <pkg>` |
| `npm install [-D] <pkg>`, `yarn add [-D] <pkg>` | `pnpm add [-D] <pkg>` |
| `npm install`, `npm ci` | `pnpm install` |
| `npm run <script>`, `npm init -y`, `npm pkg set k=v` | `pnpm run <script>`, `pnpm init`, `pnpm pkg set k=v` |
| `allowed-tools: Bash(npx <pkg> *)` | `Bash(pnpm dlx <pkg> *)` |
| MCP config `command: npx`, `args: ["-y", "<pkg>"]` | `command: pnpm`, `args: ["dlx", "<pkg>"]` |

Keep upstream's arguments and version pins as they are.

When a skill tells the agent to follow the project's lockfile, keep that rule. Rewrite its npm or npx example to pnpm, and list pnpm first where it names several runners. An npm project has no npx here either, so `pnpm dlx` stays the right runner for one-off tools.

## Leave alone

These don't run npm:
- npm as a noun: "an npm package", the registry, `npm:` import specifiers, CDN URLs such as `cdn.jsdelivr.net/npm/...`;
- lists of package managers, lockfile names, and `packageManager` values;
- reference docs that compare package managers side by side, such as turborepo's;
- commands that run on someone else's machine: CI workflow steps, Vercel's Ignored Build Step;
- `node` and `bunx` commands, which work as written.

## Record

Add one `localChanges` note per adapted skill: "Rewrote npx/npm commands to pnpm (dlx/exec/add): npm and npx are not installed here."

On update, the merge carries earlier rewrites forward. Run Find on the merged copy to catch any new upstream commands, and keep the note while any rewrite survives.
