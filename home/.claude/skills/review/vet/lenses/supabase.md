# Lens: supabase

Postgres under Supabase: policies, migrations, functions, grants, Realtime, Edge Functions. Load: the `supabase` skill. Where RLS is the authorization authority (the app forwards the caller's JWT), a permissive policy is a real hole: grade it that way. Confirm applied state with a database MCP tool when the session has one; read only.

## Targets

- **RLS (blocking).** RLS enabled on every `public` table, in the same migration as `create table`; no `FOR ALL` policies -- one per operation per role; every policy names its role (`TO authenticated` / `TO anon`); `auth.uid()` wrapped as `(select auth.uid())`; authorization from `app_metadata`, never `user_metadata`; `security definer` functions set `search_path = ''`; views over RLS tables set `security_invoker = true`.
- **Grants.** New tables revoke all from `anon` and `authenticated`, then grant explicitly, by hand -- `supabase db diff` generates none.
- **Migration hygiene.** Under a schema-first workflow, schema files are edited and migrations generated, not hand-written; strip the spurious `drop view` + `create or replace view` pairs `db diff` emits for unchanged views, and re-apply `security_invoker` to views that genuinely changed; no inline comments in generated migrations. Indexes on foreign keys and on columns used in policy conditions. Destructive DDL (`drop column`, `drop table`) needs an explicit reason; prefer rename.
- **Data changes.** Locks: an `alter table` or non-concurrent index build on a large hot table blocks writes. Re-run safety: a data snippet or backfill without a guard double-applies. Parity: rounding, time zone, or enum handling in SQL that drifts from the application code computing the same value.
- **Realtime.** Broadcast over `postgres_changes`; `realtime.broadcast_changes` in triggers, `realtime.send` for custom messages; private channels enforced by `realtime.messages` policies; subscribers re-`setAuth()` after token refresh and unsubscribe on cleanup.
- **Edge Functions.** `Deno.serve`, `npm:pkg@version` imports, auth verified before sensitive work, CORS for browser callers, writes only under `/tmp`.

## Business risk

`possible` when a broad-looking policy may serve a product need (public share tokens, service-role paths), a policy reads a JWT claim you cannot trace to its source, or a trigger maintains a denormalised field an integration reads. `none` for RLS off, `FOR ALL`, unwrapped `auth.uid()`, missing `search_path`, missing grants.
