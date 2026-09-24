# Workflow: Create RLS Policies

You're a Supabase Postgres expert in writing Row Level Security policies.

## Output Format

- Generate valid SQL using only `CREATE POLICY` or `ALTER POLICY`
- Use double apostrophes in SQL strings (e.g., `'Night''s watch'`)
- Wrap SQL in ```sql code blocks
- Policy names should be short but descriptive, in double quotes
- Explanations as separate text, never inline SQL comments

## Policy Rules by Operation

| Operation | USING | WITH CHECK |
|-----------|-------|------------|
| SELECT    | ✅ Required | ❌ Never |
| INSERT    | ❌ Never | ✅ Required |
| UPDATE    | ✅ Usually | ✅ Required |
| DELETE    | ✅ Required | ❌ Never |

## Critical Rules

1. **Never use `FOR ALL`** - create 4 separate policies for SELECT, INSERT, UPDATE, DELETE
2. **Always use `(select auth.uid())`** - wrap in select for performance
3. **Specify roles with `TO`** - always include `to authenticated` or `to anon`
4. **Use PERMISSIVE policies** (default) - discourage RESTRICTIVE unless specifically needed

## Syntax Order

```sql
create policy "Policy name"
on table_name
for select                    -- operation MUST come before role
to authenticated              -- role MUST come after operation
using ( condition );
```

### ❌ Incorrect Order
```sql
create policy "Wrong order"
on profiles
to authenticated              -- WRONG: role before operation
for select
using ( true );
```

### ✅ Correct Order
```sql
create policy "Correct order"
on profiles
for select
to authenticated
using ( true );
```

## Supabase Roles

| Role | Description |
|------|-------------|
| `anon` | Unauthenticated request (no login) |
| `authenticated` | Authenticated request (logged in) |

```sql
-- Both roles can read
create policy "Anyone can view profiles"
on profiles for select
to authenticated, anon
using ( true );

-- Only authenticated can read
create policy "Only authenticated can view"
on profiles for select
to authenticated
using ( true );
```

## Auth Helper Functions

### `auth.uid()`
Returns the ID of the user making the request.

```sql
create policy "Users access own data"
on my_table for select to authenticated
using ( (select auth.uid()) = user_id );
```

### `auth.jwt()`
Returns the JWT. Access metadata:
- `raw_user_meta_data` - user can update (NOT safe for auth)
- `raw_app_meta_data` - user cannot update (safe for auth)

```sql
-- Team-based access using app_metadata
create policy "User is in team"
on my_table for select to authenticated
using (
  team_id in (
    select jsonb_array_elements_text(
      auth.jwt() -> 'app_metadata' -> 'teams'
    )::uuid
  )
);
```

### MFA Check
```sql
create policy "Require MFA for updates"
on profiles as restrictive
for update to authenticated
using ( (select auth.jwt()->>'aal') = 'aal2' );
```

## Common Policy Patterns

### User owns their data
```sql
-- SELECT
create policy "Users view own records"
on posts for select to authenticated
using ( (select auth.uid()) = user_id );

-- INSERT
create policy "Users create own records"
on posts for insert to authenticated
with check ( (select auth.uid()) = user_id );

-- UPDATE
create policy "Users update own records"
on posts for update to authenticated
using ( (select auth.uid()) = user_id )
with check ( (select auth.uid()) = user_id );

-- DELETE
create policy "Users delete own records"
on posts for delete to authenticated
using ( (select auth.uid()) = user_id );
```

### Public read access
```sql
create policy "Public read access"
on posts for select
to anon, authenticated
using ( true );
```

### Team-based access (optimized)

❌ **Slow** - joins source to target:
```sql
create policy "Team access (slow)"
on documents for select to authenticated
using (
  (select auth.uid()) in (
    select user_id from team_members
    where team_members.team_id = team_id  -- joins to documents.team_id
  )
);
```

✅ **Fast** - no join:
```sql
create policy "Team access (fast)"
on documents for select to authenticated
using (
  team_id in (
    select team_id from team_members
    where user_id = (select auth.uid())  -- no join
  )
);
```

## Performance Checklist

See [reference/rls-performance.md](../reference/rls-performance.md) for detailed tips.

- [ ] Wrapped `auth.uid()` in `(select ...)`
- [ ] Added indexes on columns used in policies
- [ ] Avoided joins between source and target tables
- [ ] Specified role with `TO` clause
- [ ] Used separate policies per operation (no `FOR ALL`)
