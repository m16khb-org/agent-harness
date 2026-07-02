# Codd Deep Database Review Reference

Use this file only after the first-turn Codd contract confirms a relational database, SQL-like query engine, or row-count driven schema question is in scope.

## Survey Commands

PostgreSQL reference queries:

```sql
SELECT schemaname, relname, n_live_tup
FROM pg_stat_user_tables
ORDER BY n_live_tup DESC;

SELECT relname, pg_size_pretty(pg_total_relation_size(relid)) AS total_size
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC;

SELECT indexrelname, idx_scan, idx_tup_read, idx_tup_fetch,
       pg_size_pretty(pg_relation_size(indexrelid)) AS size
FROM pg_stat_user_indexes
ORDER BY pg_relation_size(indexrelid) DESC;

SELECT queryid, calls, mean_exec_time, total_exec_time, LEFT(query, 120) AS query_preview
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 20;
```

Engine equivalents:

- MySQL: `performance_schema`, `information_schema`, `SHOW ENGINE INNODB STATUS`, and `EXPLAIN ANALYZE`.
- SQLite: `sqlite_master`, `sqlite_stat*`, `.eqp on`, and `.stats on`.
- SQL Server: `sys.dm_exec_*`, `sys.dm_tran_locks`, and actual execution plans.

## Row-Count Classes

- Tiny: fewer than 10k rows. Prefer simple indexes and avoid partitioning.
- Small: 10k to 1m rows. Index exact access patterns; validate sort and pagination plans.
- Medium: 1m to 100m rows. Treat missing composite indexes, unbounded scans, and N+1 as production risks.
- Large: more than 100m rows. Require partitioning/archival analysis, migration safety, and realistic load verification.

## Normalization Audit

- 1NF: every column is atomic; no repeating groups or embedded lists that the query model needs to search independently.
- 2NF: non-key attributes depend on the whole key, not part of a composite key.
- 3NF: non-key attributes depend only on the key, not other non-key attributes.
- BCNF: every determinant is a candidate key.

For each violation, name the anomaly:

- Insert anomaly: data cannot be recorded without unrelated data.
- Update anomaly: the same fact can diverge across rows.
- Delete anomaly: deleting one fact accidentally deletes another.

## Denormalization Gate

Denormalize only when:

- A measured read path is too slow after reasonable indexing and query-shape fixes.
- The duplicated or derived data has a single owner and deterministic refresh path.
- The write amplification and consistency repair story are explicit.
- The accepted anomaly and recovery path are documented.

Reject denormalization when the problem is missing indexes, N+1 application loops, stale planner stats, or a query that can be rewritten without duplicating facts.

## Index Selection

Choose the index shape from the query:

- Equality predicates first.
- Range predicate next.
- Sort columns only when the preceding predicates preserve useful order.
- Include columns only when an index-only scan materially reduces heap reads.
- Partial indexes only when the predicate is stable and appears in the query.
- Unique indexes when they enforce a real invariant, not just performance.

Always state:

- Read path served.
- Rejected index alternatives.
- Write penalty for insert/update/delete.
- Storage cost and maintenance risk.
- Queries that may regress due to planner choice changes.

## EXPLAIN Evidence

For PostgreSQL, prefer:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT ...
```

Record at least two of:

- Planner cost.
- Actual execution time.
- Shared/local/temp buffer reads.
- Row estimates vs actual rows.
- Join strategy.
- Sort/hash memory or disk spill.

Evidence template:

```markdown
### Before
- Plan: <seq scan / nested loop / sort / etc>
- Cost/time/buffers: <numbers>
- Cause: <missing index, bad join, N+1, stale stats, etc>

### After
- Change: <index/query/schema/pool change>
- Plan: <new plan>
- Cost/time/buffers: <numbers>
- Write or operational penalty: <numbers/risk>
- Verdict: <accept/reject>
```

## Concurrency And Pooling

Check:

- Long-running transactions and idle-in-transaction sessions.
- Blocking lock graph.
- Lock ordering across code paths.
- Deadlock reports and retry policy.
- Isolation level needed for correctness.
- `lock_timeout`, `statement_timeout`, and `idle_in_transaction_session_timeout`.

Pool sizing requires:

- Database `max_connections`.
- Reserved admin connections.
- Service instance count.
- CPU/core count.
- Expected concurrency and query latency.

Rule of thumb only after those inputs exist:

```text
pool_per_instance = floor((max_connections - reserved) / service_instances)
```

Do not present a formula as proof; validate with queue time, active/idle counts, and timeout/error metrics.
