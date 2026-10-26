# Lens: react-next

React correctness and the Next.js App Router around it. Load: the `react-best-practices` skill.

Docs: caching, server-action, and routing semantics move between Next versions, so read the docs for every Next API the diff touches before judging it, never your memory. First choice is the docs bundled with the installed version (Next 16.2+): the nearest `node_modules/next/dist/docs/` above the changed file, which in a monorepo sits under the app (`apps/<app>/node_modules/next/dist/docs/`); its layout mirrors nextjs.org/docs. Without it, use context7 (`vercel/next.js`, `facebook/react`). An overlay that pins a source wins.

## Targets

- **Type holes.** `any`, `!`, and unchecked `as` that hide a reachable runtime failure; boolean-flag state that should be a discriminated union; API or user input used without parsing at the boundary.
- **Purity.** Side effects during render (`ref.current = x` inline, setState in render, fetching in the render body); mutating props, state, hook values, or a value after it was used in JSX, including through a spread that shares a nested object.
- **Hooks.** Conditional hooks, hooks in loops or after early returns; dependency arrays missing a dependency or churning every render.
- **Compiler bail-outs.** `try/finally`, `try` without `catch`, and value-block expressions (`?.`, ternaries, `&&`, `||`, `??`) inside a `try/catch` body: hoist the value out, then call inside. `useMemo`/`useCallback` on trivial work, which the compiler already memoises.
- **Effects and state.** `useEffect` as a last resort -- derived values, event handlers, `key` resets, and `useSyncExternalStore` replace most effects. State at the lowest common ancestor; no state that duplicates props or derived values; composition over a prop threaded past two levels; `ref` as a prop on React 19.
- **Server/client boundary.** `use client` only for hooks, handlers, browser APIs, or client-only libraries, pushed to the leaves; props crossing the boundary serialisable.
- **Data.** Reads in server components, not `useEffect` + fetch; mutations through server actions (or the project's API layer), not route handlers, which serve streaming, webhooks, and external consumers; no N+1 fetches in a server tree; `cache()` for a read repeated in one request.
- **Caching.** A mutating server action without `revalidatePath`/`revalidateTag` for every affected route or tag is blocking. User-specific data must not be cached static. Tag invalidation matches tag creation.
- **Routes.** `loading.tsx`/`error.tsx` beside the segment they cover; metadata through `metadata`/`generateMetadata`; segment config (`dynamic`, `revalidate`, runtime) set deliberately; layouts without navigation state.
- **Middleware.** The matcher excludes static assets and any monitoring tunnel route; auth gating matches the app's existing pattern; no Node-only API on the edge runtime.

## Business risk

`possible` when a component encodes domain math (pricing, financing, measurements), sits in a partner-integration flow, caches aggressively by apparent design, or serves a public route that may use token access instead of a session. `none` for purity and hook violations, `any`, missing revalidation, fetch-in-effect, and `use client` with no client features.
