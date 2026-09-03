# Feature plan

| Slice | Requirements | Production units | Verification |
| --- | --- | --- | --- |
| Envelope | JOB-001/003/007 | values, validation, IDs, retry policy | boundary unit tests and fuzz |
| Durable enqueue | JOB-002/003/011 | migrations, transaction helper, enqueue | unit plus PostgreSQL rollback/idempotency |
| Lease lifecycle | JOB-004/005/006/007 | claim, heartbeat, complete, fail | concurrency, expiry, fencing, race |
| Operations | JOB-008/009 | cancel, get, counts, dead listing, redrive | terminal/idempotent/boundary tests |
| Worker | JOB-010/012 | serial loop, heartbeat, panic and error classification | deterministic fake-store tests and race |
| Admission | JOB-011/013 | public consumer, clean clone, performance, reviews | complete verification matrix |

Only one slice may be unfinished. The feature remains active until all
required evidence exists and two fresh cold Judge passes are clean.
