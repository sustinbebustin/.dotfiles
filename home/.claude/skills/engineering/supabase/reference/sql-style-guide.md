# Reference: Postgres SQL Style Guide

Consistent SQL style for Supabase projects.

## General Rules

1. **Always use lowercase** except for acronyms or when readability requires it
2. **Include comments** for complex logic:
   - `/* ... */` for block comments
   - `--` for line comments

## Naming Conventions

### Tables
- Use **plural** nouns: `users`, `orders`, `products`
- Use **snake_case**: `order_items`, `user_profiles`

### Columns
- Use **snake_case**: `first_name`, `created_at`
- Foreign keys: singular table name + `_id`
  - `user_id` references `users`
  - `order_id` references `orders`
- Timestamps: `created_at`, `updated_at`, `deleted_at`
- Booleans: prefix with `is_`, `has_`, `can_`
  - `is_active`, `has_verified`, `can_edit`

### Constraints & Indexes
- Primary key: `tablename_pkey`
- Foreign key: `tablename_columnname_fkey`
- Unique: `tablename_columnname_key`
- Index: `tablename_columnname_idx`
- Check: `tablename_columnname_check`

## Formatting

### Short Queries
Keep on few lines:
```sql
select id, name from users where is_active = true;
```

### Longer Queries
Add newlines for readability:
```sql
select
  u.id,
  u.name,
  u.email,
  p.avatar_url
from users u
left join profiles p on p.user_id = u.id
where u.is_active = true
  and u.created_at > '2024-01-01'
order by u.created_at desc
limit 100;
```

### Keywords
- Use lowercase: `select`, `from`, `where`, `join`
- Align major clauses:
```sql
select
  id,
  name
from users
where is_active = true
order by name;
```

## Data Types

### Preferred Types
| Use | Instead of |
|-----|------------|
| `text` | `varchar` (unless length constraint needed) |
| `timestamptz` | `timestamp` (always store with timezone) |
| `uuid` | `serial` for IDs |
| `boolean` | `int` for true/false |
| `jsonb` | `json` (better performance) |

### UUIDs
```sql
create table users (
  id uuid primary key default gen_random_uuid(),
  ...
);
```

### Timestamps
```sql
create table posts (
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);
```

## Common Patterns

### Table with Standard Fields
```sql
create table public.items (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  name text not null,
  description text,
  is_active boolean default true,
  metadata jsonb default '{}',
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

create index items_user_id_idx on public.items(user_id);
```

### Soft Delete
```sql
create table public.posts (
  id uuid primary key default gen_random_uuid(),
  content text,
  deleted_at timestamptz,  -- null = not deleted
  created_at timestamptz default now()
);

-- Only show non-deleted posts
create policy "View active posts"
on public.posts for select
using (deleted_at is null);
```

### Enum Alternative (Use Check)
```sql
create table public.orders (
  id uuid primary key default gen_random_uuid(),
  status text not null default 'pending'
    check (status in ('pending', 'processing', 'completed', 'cancelled'))
);
```

### Junction Table (Many-to-Many)
```sql
create table public.user_teams (
  user_id uuid references auth.users(id) on delete cascade,
  team_id uuid references public.teams(id) on delete cascade,
  role text not null default 'member',
  joined_at timestamptz default now(),
  primary key (user_id, team_id)
);

create index user_teams_team_id_idx on public.user_teams(team_id);
```

## Anti-Patterns to Avoid

❌ **Don't use reserved words** as identifiers:
```sql
-- Bad
create table user (...);
create table order (...);

-- Good
create table users (...);
create table orders (...);
```

❌ **Don't mix naming conventions**:
```sql
-- Bad
create table UserProfiles (userId int, FirstName text);

-- Good
create table user_profiles (user_id uuid, first_name text);
```

❌ **Don't skip NOT NULL** where appropriate:
```sql
-- Bad (allows null user_id)
create table posts (
  user_id uuid references auth.users(id)
);

-- Good
create table posts (
  user_id uuid not null references auth.users(id)
);
```

❌ **Don't use timestamp without timezone**:
```sql
-- Bad
created_at timestamp default now()

-- Good
created_at timestamptz default now()
```
