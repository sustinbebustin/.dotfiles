# Reference: RLS Performance Optimization

Row Level Security can impact query performance, especially for queries scanning many rows. Follow these recommendations.

## 1. Add Indexes

Index columns used in RLS policies:

```sql
-- Policy using user_id
create policy "Users access own data"
on posts for select to authenticated
using ( (select auth.uid()) = user_id );

-- Add index
create index posts_user_id_idx on posts(user_id);
```

Index columns used in subqueries:
```sql
-- Policy using team_id lookup
create policy "Team access"
on documents for select to authenticated
using (
  team_id in (
    select team_id from team_members
    where user_id = (select auth.uid())
  )
);

-- Add indexes
create index documents_team_id_idx on documents(team_id);
create index team_members_user_id_idx on team_members(user_id);
```

## 2. Wrap Functions with SELECT

**Always wrap `auth.uid()` and `auth.jwt()` in a select statement.**

This enables PostgreSQL to cache the result per-statement instead of calling the function for each row.

❌ **Slow** (function called per row):
```sql
create policy "Users access own data"
on posts for select to authenticated
using ( auth.uid() = user_id );
```

✅ **Fast** (function result cached):
```sql
create policy "Users access own data"
on posts for select to authenticated
using ( (select auth.uid()) = user_id );
```

**Applies to:**
- `auth.uid()`
- `auth.jwt()`
- `security definer` functions

**Caution:** Only use this when the function result doesn't change based on row data.

## 3. Minimize Joins

Avoid joining the source table (being queried) with the target table (in the policy).

❌ **Slow** (joins source to target):
```sql
create policy "Team access (slow)"
on documents for select to authenticated
using (
  (select auth.uid()) in (
    select user_id
    from team_members
    where team_members.team_id = team_id  -- References documents.team_id
  )
);
```

✅ **Fast** (no join, uses IN with subquery):
```sql
create policy "Team access (fast)"
on documents for select to authenticated
using (
  team_id in (
    select team_id
    from team_members
    where user_id = (select auth.uid())  -- No reference to documents
  )
);
```

**Pattern:** Fetch filter criteria into a set, then use `IN` or `ANY`.

## 4. Hoist Complex Checks into Security Definer Helpers

For checks that span multiple tables, encapsulate the logic in a `security definer` function. The function runs with the definer's privileges (bypassing RLS on its inputs), the planner caches the result per statement when wrapped in `(select ...)`, and the policy expression stays a single indexed call.

```sql
-- Helper function (lives in a private schema, not on the Data API)
create or replace function private.is_team_member(_team_id bigint)
returns boolean
language sql
security definer
set search_path = ''  -- required: prevents search-path injection
stable
as $$
  select exists (
    select 1
    from public.team_members
    where team_id = _team_id
      and user_id = (select auth.uid())
  );
$$;

-- Policy uses the helper (single indexed lookup, not per-row join)
create policy "Team members read documents"
on public.documents for select to authenticated
using ( (select private.is_team_member(team_id)) );
```

**Required:**
- `set search_path = ''` -- without it, an attacker who can create objects in a writable schema on the search path can override `team_members` and bypass the check.
- Fully qualify every referenced object inside the function body (`public.team_members`, not `team_members`) since the search path is empty.
- Place the function in a non-exposed schema (`private`, `internal`, etc.) so it isn't reachable through PostgREST.

## 5. Always Specify Roles

Use the `TO` clause to prevent unnecessary policy evaluation:

❌ **Evaluates for all roles:**
```sql
create policy "Users access own data"
on posts
using ( (select auth.uid()) = user_id );
```

✅ **Skips evaluation for anon:**
```sql
create policy "Users access own data"
on posts
to authenticated  -- Stops here for anon requests
using ( (select auth.uid()) = user_id );
```

## 6. Use Separate Policies

Don't combine operations or roles:

❌ **Combined (harder to optimize):**
```sql
create policy "User CRUD"
on posts for all
using ( (select auth.uid()) = user_id );
```

✅ **Separate (easier to optimize):**
```sql
create policy "Users select own posts"
on posts for select to authenticated
using ( (select auth.uid()) = user_id );

create policy "Users insert own posts"
on posts for insert to authenticated
with check ( (select auth.uid()) = user_id );

create policy "Users update own posts"
on posts for update to authenticated
using ( (select auth.uid()) = user_id )
with check ( (select auth.uid()) = user_id );

create policy "Users delete own posts"
on posts for delete to authenticated
using ( (select auth.uid()) = user_id );
```

## 7. Consider Views for Complex Access

For very complex policies, consider using views with `security_invoker = true` (Postgres 15+):

```sql
create view public.user_accessible_documents
with (security_invoker = true)
as
select d.*
from documents d
join team_members tm on tm.team_id = d.team_id
where tm.user_id = auth.uid();
```

## Performance Checklist

- [ ] Indexed all columns used in policies
- [ ] Wrapped `auth.uid()` and `auth.jwt()` in `(select ...)`
- [ ] Avoided joins between source and target tables
- [ ] Used `TO authenticated` or `TO anon` on all policies
- [ ] Separated policies by operation (SELECT, INSERT, UPDATE, DELETE)
- [ ] Tested query plans with `EXPLAIN ANALYZE`

## Testing Performance

```sql
-- Test with EXPLAIN ANALYZE
explain analyze
select * from posts where user_id = 'some-uuid';

-- Check if RLS policies are being applied efficiently
-- Look for:
-- - Seq Scan (bad for large tables)
-- - Index Scan (good)
-- - Filter vs Index Cond (Filter means post-fetch filtering)
```
