# Postgres Performance Best Practices

Atomic Postgres optimization rules drawn from Supabase's published guidance, organized by impact. Each file contains a focused rule with an incorrect example, a correct example, and references.

## Categories by Priority

| Priority | Category               | Impact       | Prefix      |
|----------|------------------------|--------------|-------------|
| 1        | Query Performance      | CRITICAL     | `query-`    |
| 2        | Connection Management  | CRITICAL     | `conn-`     |
| 3        | Security & Privileges  | CRITICAL     | `security-` |
| 4        | Schema Design          | HIGH         | `schema-`   |
| 5        | Concurrency & Locking  | MEDIUM-HIGH  | `lock-`     |
| 6        | Data Access Patterns   | MEDIUM       | `data-`     |
| 7        | Monitoring & Diagnostics | LOW-MEDIUM | `monitor-`  |
| 8        | Advanced Features      | LOW          | `advanced-` |

For RLS basics and RLS performance, see [`../../workflows/create-rls-policies.md`](../../workflows/create-rls-policies.md) and [`../rls-performance.md`](../rls-performance.md) -- those are the canonical references.

## Query Performance (`query-`)

- [query-composite-indexes.md](query-composite-indexes.md)
- [query-covering-indexes.md](query-covering-indexes.md)
- [query-index-types.md](query-index-types.md)
- [query-missing-indexes.md](query-missing-indexes.md)
- [query-partial-indexes.md](query-partial-indexes.md)

## Connection Management (`conn-`)

- [conn-idle-timeout.md](conn-idle-timeout.md)
- [conn-limits.md](conn-limits.md)
- [conn-pooling.md](conn-pooling.md)
- [conn-prepared-statements.md](conn-prepared-statements.md)

## Security & Privileges (`security-`)

- [security-privileges.md](security-privileges.md) -- GRANT/REVOKE patterns for custom Postgres roles

## Schema Design (`schema-`)

- [schema-constraints.md](schema-constraints.md)
- [schema-data-types.md](schema-data-types.md)
- [schema-foreign-key-indexes.md](schema-foreign-key-indexes.md)
- [schema-lowercase-identifiers.md](schema-lowercase-identifiers.md)
- [schema-partitioning.md](schema-partitioning.md)
- [schema-primary-keys.md](schema-primary-keys.md)

## Concurrency & Locking (`lock-`)

- [lock-advisory.md](lock-advisory.md)
- [lock-deadlock-prevention.md](lock-deadlock-prevention.md)
- [lock-short-transactions.md](lock-short-transactions.md)
- [lock-skip-locked.md](lock-skip-locked.md)

## Data Access Patterns (`data-`)

- [data-batch-inserts.md](data-batch-inserts.md)
- [data-n-plus-one.md](data-n-plus-one.md)
- [data-pagination.md](data-pagination.md)
- [data-upsert.md](data-upsert.md)

## Monitoring & Diagnostics (`monitor-`)

- [monitor-explain-analyze.md](monitor-explain-analyze.md)
- [monitor-pg-stat-statements.md](monitor-pg-stat-statements.md)
- [monitor-vacuum-analyze.md](monitor-vacuum-analyze.md)

## Advanced Features (`advanced-`)

- [advanced-full-text-search.md](advanced-full-text-search.md)
- [advanced-jsonb-indexing.md](advanced-jsonb-indexing.md)

## Sources

- https://www.postgresql.org/docs/current/
- https://supabase.com/docs/guides/database/overview
- https://wiki.postgresql.org/wiki/Performance_Optimization
