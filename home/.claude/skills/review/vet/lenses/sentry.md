# Lens: sentry

Sentry wiring in a JavaScript app, attached to another reviewer when the diff touches Sentry config or an `@sentry/*` import. SDK options move between versions: check claims against the SDK docs (context7 `getsentry/sentry-javascript`) rather than memory.

## Targets

- **Init per runtime.** One init each for the browser, Node server, and edge runtimes the app uses; missing one is blocking. `instrumentation.ts` dispatches on `NEXT_RUNTIME` and exports `onRequestError = Sentry.captureRequestError`; the client init exports `onRouterTransitionStart`.
- **DSN and rates.** The public DSN variable client-side, the server one on server and edge; no hard-coded DSN; sample rates explicit (1.0 in production is costly, 0.0 silently disables tracing); `enableLogs` wherever `Sentry.logger` is used.
- **Capture paths.** A root `global-error.tsx` that captures and renders the fallback; a `catch` that logs and returns a shaped error without `captureException` swallows the error from Sentry.
- **Build.** `withSentryConfig` names the right org and project per app and gets its auth token on the production build path; webpack-only options are no-ops under Turbopack; source maps upload before any bundler step that rewrites the output; the build-plugin env file is gitignored.
- **Tunnel.** A `tunnelRoute` excluded from the middleware matcher -- auth-gated tunnels silently drop browser events. Always check it.
- **Tracing.** `tracePropagationTargets` includes the API origin, or traces never link across services.
- **Privacy.** `sendDefaultPii` is a compliance decision: flag a change, do not set it. Relaxed Replay masking, or a new capture path that bypasses `beforeSend` scrubbing, is a finding.

## Business risk

`possible` for sample-rate and Replay choices that trade cost or privacy, either direction of `sendDefaultPii`, and an aggressive scrub. `none` for a missing tunnel exclusion, hard-coded DSN, public DSN in a server-only init, missing global error capture, missing production auth token, or swallowed server-action errors.
