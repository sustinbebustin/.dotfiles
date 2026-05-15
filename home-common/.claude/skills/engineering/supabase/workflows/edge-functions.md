# Workflow: Writing Supabase Edge Functions

You're an expert in TypeScript and the Deno runtime. Generate high-quality Supabase Edge Functions following these best practices.

## Core Principles

1. **Prefer Web APIs and Deno core** over external dependencies
   - Use `fetch` instead of Axios
   - Use WebSocket API instead of node-ws
   - Use Web Crypto API instead of crypto libraries

2. **Shared utilities** go in `supabase/functions/_shared/`
   - Import using relative paths
   - Do NOT have cross-dependencies between Edge Functions

3. **No bare specifiers** for imports
   ```typescript
   // ❌ Wrong
   import express from "express";
   
   // ✅ Correct
   import express from "npm:express@4.18.2";
   ```

4. **Use `Deno.serve`** - not the old std library serve
   ```typescript
   // ❌ Wrong
   import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
   
   // ✅ Correct
   Deno.serve(async (req) => { ... });
   ```

## Environment Variables

**Pre-populated** (no setup needed):
- `SUPABASE_URL`
- `SUPABASE_ANON_KEY`
- `SUPABASE_SERVICE_ROLE_KEY`
- `SUPABASE_DB_URL`

**Custom secrets:**
```bash
supabase secrets set --env-file path/to/env-file
```

## File Operations

Only `/tmp` directory is writable:
```typescript
await Deno.writeTextFile("/tmp/output.txt", data);
```

## Background Tasks

Use `EdgeRuntime.waitUntil()` for long-running tasks:
```typescript
Deno.serve(async (req) => {
  // Start background task without blocking response
  EdgeRuntime.waitUntil(sendAnalytics(req));
  
  return new Response("OK");
});
```

## Multi-Route Functions

Use Express or Hono for multiple routes. Prefix with function name:

```typescript
import express from "npm:express@4.18.2";

const app = express();

// Routes must be prefixed with /function-name
app.get("/my-function/users", (req, res) => {
  res.json({ users: [] });
});

app.post("/my-function/users", (req, res) => {
  res.json({ created: true });
});

app.listen(8000);
```

## Templates

### Simple Hello World

```typescript
interface ReqPayload {
  name: string;
}

console.info('server started');

Deno.serve(async (req: Request) => {
  const { name }: ReqPayload = await req.json();
  
  const data = {
    message: `Hello ${name}!`,
  };

  return new Response(
    JSON.stringify(data),
    { 
      headers: { 
        'Content-Type': 'application/json',
        'Connection': 'keep-alive'
      }
    }
  );
});
```

### Using Node Built-in APIs

```typescript
import { randomBytes } from "node:crypto";
import process from "node:process";

Deno.serve(async (req: Request) => {
  const randomString = randomBytes(16).toString('hex');
  
  return new Response(
    JSON.stringify({ random: randomString }),
    { headers: { 'Content-Type': 'application/json' }}
  );
});
```

### Using npm Packages

```typescript
import express from "npm:express@4.18.2";

const app = express();

app.use(express.json());

app.get("/api/health", (req, res) => {
  res.json({ status: "ok" });
});

app.listen(8000);
```

### With Supabase Client

```typescript
import { createClient } from "npm:@supabase/supabase-js@2";

Deno.serve(async (req: Request) => {
  const supabase = createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
  );

  const { data, error } = await supabase
    .from('users')
    .select('*')
    .limit(10);

  if (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      { status: 500, headers: { 'Content-Type': 'application/json' }}
    );
  }

  return new Response(
    JSON.stringify({ data }),
    { headers: { 'Content-Type': 'application/json' }}
  );
});
```

### With Auth Verification

```typescript
import { createClient } from "npm:@supabase/supabase-js@2";

Deno.serve(async (req: Request) => {
  const authHeader = req.headers.get('Authorization');
  
  if (!authHeader) {
    return new Response(
      JSON.stringify({ error: 'Missing authorization header' }),
      { status: 401, headers: { 'Content-Type': 'application/json' }}
    );
  }

  const supabase = createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_ANON_KEY')!,
    { global: { headers: { Authorization: authHeader }}}
  );

  const { data: { user }, error } = await supabase.auth.getUser();

  if (error || !user) {
    return new Response(
      JSON.stringify({ error: 'Invalid token' }),
      { status: 401, headers: { 'Content-Type': 'application/json' }}
    );
  }

  return new Response(
    JSON.stringify({ user_id: user.id }),
    { headers: { 'Content-Type': 'application/json' }}
  );
});
```

### CORS Handling

```typescript
const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};

Deno.serve(async (req: Request) => {
  // Handle CORS preflight
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders });
  }

  // Your logic here
  const data = { message: 'Hello' };

  return new Response(
    JSON.stringify(data),
    { headers: { ...corsHeaders, 'Content-Type': 'application/json' }}
  );
});
```

## Project Structure

```
frontend/supabase/
├── functions/
│   ├── my-function/
│   │   └── index.ts
│   ├── another-function/
│   │   └── index.ts
│   └── _shared/           # Shared utilities
│       ├── cors.ts
│       ├── supabase.ts
│       └── utils.ts
└── config.toml
```

## Shared Utilities Example

`_shared/cors.ts`:
```typescript
export const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
};
```

`_shared/supabase.ts`:
```typescript
import { createClient } from "npm:@supabase/supabase-js@2";

export const getSupabaseClient = (authHeader?: string) => {
  return createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_ANON_KEY')!,
    authHeader ? { global: { headers: { Authorization: authHeader }}} : undefined
  );
};

export const getSupabaseAdmin = () => {
  return createClient(
    Deno.env.get('SUPABASE_URL')!,
    Deno.env.get('SUPABASE_SERVICE_ROLE_KEY')!
  );
};
```

Using shared utilities:
```typescript
import { corsHeaders } from "../_shared/cors.ts";
import { getSupabaseClient } from "../_shared/supabase.ts";

Deno.serve(async (req) => {
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders });
  }
  
  const supabase = getSupabaseClient(req.headers.get('Authorization')!);
  // ...
});
```

## Checklist

- [ ] Using `Deno.serve` (not std library)
- [ ] No bare specifiers (use `npm:package@version`)
- [ ] Shared code in `_shared/` directory
- [ ] File writes only to `/tmp`
- [ ] CORS headers for browser requests
- [ ] Background tasks use `EdgeRuntime.waitUntil()`
- [ ] Multi-route functions prefixed with function name
