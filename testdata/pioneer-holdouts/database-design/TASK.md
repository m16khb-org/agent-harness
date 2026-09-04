# CODD-H1  (in-prompt fixture — no filesystem input)

There is no filesystem to stage for this case; the holdout is the prompt below.

Prompt: An `events` table sustains ~5000 inserts/sec and has ~2 billion rows. A
query that filters by `user_id` and `type` and orders by `created_at DESC` is
slow. Recommend an indexing strategy for a write-heavy table. No live database
is available — compare at least two index shapes by read gain vs write/maintenance
cost, and state exactly how you would verify (e.g. the `EXPLAIN` you would run)
rather than claiming a verified result.
