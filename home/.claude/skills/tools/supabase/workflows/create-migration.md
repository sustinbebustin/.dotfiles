# Workflow: Create Database Migration

You are a Postgres Expert who loves creating secure database schemas.

This project uses migrations provided by the Supabase CLI.

## Creating a Migration File

Create migration files inside `frontend/supabase/migrations/`.

### File Naming Convention

**Format:** `YYYYMMDDHHmmss_short_description.sql`

- `YYYY` - Four digits for year (e.g., `2024`)
- `MM` - Two digits for month (01-12)
- `DD` - Two digits for day (01-31)
- `HH` - Two digits for hour in 24-hour format (00-23)
- `mm` - Two digits for minute (00-59)
- `ss` - Two digits for second (00-59)
- Description should be snake_case

**Example:** `20240906123045_create_profiles.sql`

## SQL Guidelines

Write Postgres-compatible SQL that:

1. **Include header comments** with metadata:
   ```sql
   -- Migration: Create profiles table
   -- Purpose: Store user profile information
   -- Affected tables: profiles
   -- Author: [name]
   -- Date: YYYY-MM-DD
   ```

2. **Write all SQL in lowercase**

3. **Add copious comments** for destructive operations:
   ```sql
   -- WARNING: Dropping column 'old_field'
   -- This column has been deprecated since v2.0
   -- Data has been migrated to 'new_field'
   alter table public.users drop column old_field;
   ```

4. **Always enable RLS** when creating tables:
   ```sql
   create table public.profiles (
     id uuid primary key default gen_random_uuid(),
     user_id uuid references auth.users(id) on delete cascade,
     created_at timestamptz default now()
   );

   -- Enable Row Level Security
   alter table public.profiles enable row level security;
   ```

5. **Create granular RLS policies**:
   - One policy per operation (SELECT, INSERT, UPDATE, DELETE)
   - One policy per role (anon, authenticated)
   - DO NOT combine even if functionality is the same

## Migration Template

```sql
-- Migration: [description]
-- Purpose: [what this migration does]
-- Affected tables: [list tables]
-- Special considerations: [any notes]

-- ============================================
-- Table Creation
-- ============================================

create table public.example (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  content text,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

-- Add indexes for common queries
create index example_user_id_idx on public.example(user_id);

-- Enable RLS
alter table public.example enable row level security;

-- ============================================
-- Grant Hygiene (required for new tables)
-- ============================================
-- Supabase bootstrap grants ALL to anon/authenticated by default.
-- Explicitly revoke and re-grant per 55_grants.sql spec.

revoke all on table public.example from anon;
revoke all on table public.example from authenticated;
grant select on table public.example to authenticated;
grant all on table public.example to service_role;

-- ============================================
-- RLS Policies
-- ============================================

-- SELECT: Users can view their own records
create policy "Users can view own records"
on public.example for select to authenticated
using ( (select auth.uid()) = user_id );

-- INSERT: Users can create their own records
create policy "Users can create own records"
on public.example for insert to authenticated
with check ( (select auth.uid()) = user_id );

-- UPDATE: Users can update their own records
create policy "Users can update own records"
on public.example for update to authenticated
using ( (select auth.uid()) = user_id )
with check ( (select auth.uid()) = user_id );

-- DELETE: Users can delete their own records
create policy "Users can delete own records"
on public.example for delete to authenticated
using ( (select auth.uid()) = user_id );

-- ============================================
-- Triggers (if needed)
-- ============================================

-- Auto-update updated_at timestamp
create or replace function public.handle_updated_at()
returns trigger
language plpgsql
security invoker
set search_path = ''
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

create trigger on_example_updated
  before update on public.example
  for each row
  execute function public.handle_updated_at();
```

## Checklist Before Committing

- [ ] File named correctly: `YYYYMMDDHHmmss_description.sql`
- [ ] Header comment with purpose and affected tables
- [ ] All SQL in lowercase
- [ ] RLS enabled on new tables
- [ ] Separate policies for each operation and role
- [ ] Indexes added for foreign keys and common queries
- [ ] Destructive operations documented with warnings
- [ ] New tables include grant hygiene (REVOKE anon/authenticated, GRANT per 55_grants.sql)
- [ ] Migration tested locally with `supabase db reset`
