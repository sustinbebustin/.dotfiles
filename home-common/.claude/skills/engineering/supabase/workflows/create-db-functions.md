# Workflow: Create Database Functions

You're a Supabase Postgres expert in writing database functions. Generate high-quality PostgreSQL functions following these best practices.

## Security Guidelines

### 1. Default to `SECURITY INVOKER`

Functions run with permissions of the invoking user (safer).

```sql
create or replace function public.my_function()
returns text
language plpgsql
security invoker  -- Recommended default
set search_path = ''
as $$
begin
  return 'hello';
end;
$$;
```

Use `SECURITY DEFINER` only when explicitly required (e.g., accessing data the user can't directly access). Always explain the rationale.

### 2. Always Set `search_path = ''`

Prevents security risks from untrusted schemas.

```sql
security invoker
set search_path = ''  -- Always include this
```

### 3. Use Fully Qualified Names

Always prefix with schema name:

```sql
-- ✅ Correct
select * from public.users;
insert into public.orders (...);

-- ❌ Wrong
select * from users;
```

## Best Practices

### 1. Minimize Side Effects
Prefer functions that return results over those that modify data (unless triggers).

### 2. Use Explicit Typing
Clearly specify input and output types:

```sql
create function calculate_total(
  order_id bigint,      -- explicit input type
  include_tax boolean default true
)
returns numeric         -- explicit return type
```

### 3. Function Volatility

| Category | Use When | Optimization |
|----------|----------|--------------|
| `IMMUTABLE` | Same inputs always return same output | Best - can be pre-evaluated |
| `STABLE` | Returns same result within single query | Good - cached per statement |
| `VOLATILE` | Result can change (default) | None - called every time |

```sql
-- IMMUTABLE: Pure computation
create function full_name(first text, last text)
returns text
language sql
immutable
as $$
  select first || ' ' || last;
$$;

-- STABLE: Reads data but doesn't modify
create function get_user_email(uid uuid)
returns text
language sql
stable
as $$
  select email from auth.users where id = uid;
$$;

-- VOLATILE: Modifies data or has side effects
create function log_action(action text)
returns void
language plpgsql
volatile
as $$
begin
  insert into public.audit_log (action, created_at)
  values (action, now());
end;
$$;
```

## Function Templates

### Simple Function

```sql
create or replace function public.hello_world()
returns text
language plpgsql
security invoker
set search_path = ''
as $$
begin
  return 'hello world';
end;
$$;
```

### Function with Parameters

```sql
create or replace function public.calculate_total_price(order_id bigint)
returns numeric
language plpgsql
security invoker
set search_path = ''
as $$
declare
  total numeric;
begin
  select sum(price * quantity)
  into total
  from public.order_items
  where order_items.order_id = calculate_total_price.order_id;

  return coalesce(total, 0);
end;
$$;
```

### Trigger Function

```sql
create or replace function public.update_updated_at()
returns trigger
language plpgsql
security invoker
set search_path = ''
as $$
begin
  new.updated_at := now();
  return new;
end;
$$;

-- Attach the trigger
create trigger update_updated_at_trigger
  before update on public.my_table
  for each row
  execute function public.update_updated_at();
```

### Function with Error Handling

```sql
create or replace function public.safe_divide(
  numerator numeric,
  denominator numeric
)
returns numeric
language plpgsql
security invoker
set search_path = ''
as $$
begin
  if denominator = 0 then
    raise exception 'Division by zero is not allowed';
  end if;

  return numerator / denominator;
end;
$$;
```

### Immutable Pure Function (SQL)

```sql
create or replace function public.full_name(
  first_name text,
  last_name text
)
returns text
language sql
security invoker
set search_path = ''
immutable
as $$
  select first_name || ' ' || last_name;
$$;
```

### Function Returning Table

```sql
create or replace function public.get_user_posts(uid uuid)
returns table (
  id uuid,
  title text,
  created_at timestamptz
)
language plpgsql
security invoker
set search_path = ''
stable
as $$
begin
  return query
  select p.id, p.title, p.created_at
  from public.posts p
  where p.user_id = uid
  order by p.created_at desc;
end;
$$;
```

## Checklist

- [ ] Using `SECURITY INVOKER` (or justified `DEFINER`)
- [ ] Set `search_path = ''`
- [ ] All object names fully qualified (schema.table)
- [ ] Explicit input/output types
- [ ] Appropriate volatility (IMMUTABLE/STABLE/VOLATILE)
- [ ] Error handling for edge cases
- [ ] Trigger includes `CREATE TRIGGER` statement
