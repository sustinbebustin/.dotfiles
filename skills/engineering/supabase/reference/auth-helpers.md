# Reference: Supabase Auth Helpers

Helper functions for authentication in RLS policies and database functions.

## `auth.uid()`

Returns the UUID of the user making the request. Returns `null` for unauthenticated requests.

```sql
-- Basic usage (always wrap in select for performance)
(select auth.uid())

-- In a policy
create policy "Users access own data"
on posts for select to authenticated
using ( (select auth.uid()) = user_id );
```

## `auth.jwt()`

Returns the full JWT claims as JSONB. Access metadata and custom claims.

```sql
-- Get the full JWT
(select auth.jwt())

-- Access specific claims
(select auth.jwt()->>'email')
(select auth.jwt()->'app_metadata')
(select auth.jwt()->'user_metadata')
```

### JWT Structure

```json
{
  "aud": "authenticated",
  "exp": 1234567890,
  "sub": "user-uuid-here",
  "email": "user@example.com",
  "phone": "",
  "app_metadata": {
    "provider": "email",
    "providers": ["email"],
    "custom_claim": "value"
  },
  "user_metadata": {
    "name": "John Doe"
  },
  "role": "authenticated",
  "aal": "aal1",
  "session_id": "session-uuid"
}
```

### Metadata Types

| Field | User Can Modify | Use For |
|-------|-----------------|---------|
| `user_metadata` | ✅ Yes (via `auth.update()`) | Display name, preferences |
| `app_metadata` | ❌ No | Roles, permissions, team IDs |

**Security:** Never use `user_metadata` for authorization decisions!

## Common Patterns

### Check User Role (app_metadata)

```sql
-- Check if user has admin role
create policy "Admins can manage all"
on posts for all to authenticated
using (
  (select auth.jwt()->'app_metadata'->>'role') = 'admin'
);
```

### Team-Based Access

```sql
-- User belongs to teams stored in app_metadata.teams array
create policy "Team members can access"
on documents for select to authenticated
using (
  team_id::text in (
    select jsonb_array_elements_text(
      (select auth.jwt()->'app_metadata'->'teams')
    )
  )
);
```

### MFA Requirement (AAL)

Authentication Assurance Level (AAL):
- `aal1` - Single factor (password, magic link, OAuth)
- `aal2` - Multi-factor authentication (TOTP, etc.)

```sql
-- Require MFA for sensitive operations
create policy "MFA required for updates"
on sensitive_data
as restrictive  -- Combines with other policies using AND
for update to authenticated
using (
  (select auth.jwt()->>'aal') = 'aal2'
);
```

### Email Verification

```sql
-- Only verified users can post
create policy "Verified users can post"
on posts for insert to authenticated
with check (
  (select auth.jwt()->'app_metadata'->>'email_verified')::boolean = true
);
```

### Provider Check

```sql
-- Different access based on auth provider
create policy "Google users get premium"
on premium_features for select to authenticated
using (
  'google' = any(
    array(select jsonb_array_elements_text(
      (select auth.jwt()->'app_metadata'->'providers')
    ))
  )
);
```

## Setting app_metadata (Server-Side)

Use the admin client to set custom claims:

```typescript
// In Edge Function or server-side code
import { createClient } from '@supabase/supabase-js';

const supabase = createClient(
  process.env.SUPABASE_URL,
  process.env.SUPABASE_SERVICE_ROLE_KEY
);

// Set custom app_metadata
await supabase.auth.admin.updateUserById(userId, {
  app_metadata: {
    role: 'admin',
    teams: ['team-1', 'team-2'],
  }
});
```

## Supabase Roles

| Role | Description | When Used |
|------|-------------|-----------|
| `anon` | Unauthenticated | No token or expired session |
| `authenticated` | Authenticated | Valid JWT provided |
| `service_role` | Admin | Server-side with service key |

```sql
-- Target specific roles
create policy "Anyone can read"
on posts for select
to anon, authenticated
using (true);

create policy "Only authenticated can write"
on posts for insert
to authenticated
with check (true);
```

## Anonymous Users

Anonymous users (via `signInAnonymously()`) have `authenticated` role but can be distinguished:

```sql
-- Exclude anonymous users
create policy "Registered users only"
on premium_features for select to authenticated
using (
  (select auth.jwt()->'is_anonymous')::boolean is not true
);
```

## Common Mistakes

❌ **Using user_metadata for auth:**
```sql
-- WRONG: User can modify this!
using ( (select auth.jwt()->'user_metadata'->>'role') = 'admin' )
```

✅ **Use app_metadata for auth:**
```sql
-- CORRECT: User cannot modify
using ( (select auth.jwt()->'app_metadata'->>'role') = 'admin' )
```

❌ **Forgetting to wrap in select:**
```sql
-- WRONG: Called per row
using ( auth.uid() = user_id )
```

✅ **Always wrap in select:**
```sql
-- CORRECT: Cached per statement
using ( (select auth.uid()) = user_id )
```

❌ **Not specifying role:**
```sql
-- WRONG: Evaluated for all requests
create policy "User data" on posts
using ( (select auth.uid()) = user_id );
```

✅ **Specify the role:**
```sql
-- CORRECT: Skipped for anon
create policy "User data" on posts
to authenticated
using ( (select auth.uid()) = user_id );
```
