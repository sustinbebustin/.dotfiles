# Lens: typescript

TypeScript and JavaScript outside React: CLIs, servers, workers, scripts, infrastructure code. Load: the `typescript-best-practices` skill.

## Targets

- **Type holes.** `any`, `!`, and unchecked `as` that hide a reachable runtime failure; optionality or loose object shapes that blur the real invariant; boolean flags that should be a discriminated union; input (argv, env, files, HTTP, queue messages) used without parsing at the boundary.
- **Async.** Floating promises (an un-awaited call whose rejection nobody handles), `await` in a loop over independent work, unhandled rejections that kill or silently wedge a process, `Promise.all` where one failure should not cancel the rest.
- **Process behaviour.** Exit codes that report success on failure, errors written to stdout, sync file or child-process calls on a hot path, environment read lazily deep in the call graph instead of parsed once at startup.
- **Modules.** ESM/CJS interop that only works under one loader, side effects at import time, a default export where the codebase uses named exports.
- **Infrastructure code.** A renamed resource or logical id that forces replacement (data loss on a table or bucket), a removal or retention policy dropped, a permission widened to `*`, a secret moved into plain config.
