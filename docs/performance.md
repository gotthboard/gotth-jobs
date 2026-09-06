# Performance admission

## Decision

No optimization was introduced and no speedup is claimed. The direct
PostgreSQL mechanism remains a provisional candidate because its measured
behavior is bounded enough for the first consumer and its cost remains visible:
one short transaction per mutation, one returned payload copy, and an indexed
`FOR UPDATE SKIP LOCKED` scan whose work grows with eligible and locked rows.
Heartbeat is the exception to payload return: its response is one boolean, so
renewal network traffic and allocation remain independent of payload size.
Job-returning statements explicitly use pgx DescribeExec and therefore pay two
protocol round trips. The one-copy claim refers to the bounded scanner's one
library ownership copy from pgx's borrowed binary bytea source; it does not
claim pgx or the network stack holds no other buffers.

Because there is no baseline/candidate optimization comparison, hotspot share
`P`, hotspot speedup `S_hotspot`, and the Amdahl prediction
`1 / ((1-P) + P/S_hotspot)` are N/A. Inventing numeric substitutions without a
candidate would be dishonest. A future optimization must supply matched
baseline and candidate results before making any overall speedup claim.

## Environment

- Host: designated `development` validation host, Linux amd64.
- PostgreSQL: 17.10 from pinned image digest
  `sha256:a426e44bac0b759c95894d68e1a0ac03ecc20b619f498a91aae373bf06d8508d`.
- Docker Engine: 29.7.1.
- Remote launcher: Go 1.26.5-X:nodwarf5; the module selected Go 1.26.6 for the
  exact clean-revision gates. The canonical local focused checks used Go
  1.26.6-X:nodwarf5.
- Connection: loopback published disposable container port, no network backend
  and no live database.

## Results

Claim latency measures only `Claim`. Throughput uses the full measured loop,
including `Complete` for nonempty workloads. Fixtures and enqueue setup are
outside the measurement.

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 837.634 us | 872.034 us | 975.047 us | 1246.95 ops/s |
| 100-job small backlog, empty payload | 50 | 3.734972 ms | 7.999032 ms | 8.214656 ms | 118.23 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 4.329901 ms | 4.811190 ms | 5.379659 ms | 127.10 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 6.209012 ms | 9.404505 ms | 9.707590 ms | 76.69 ops/s |
| 100-row locked prefix | 1 | 23.060410 ms | N/A | N/A | N/A |

The pathological sample proves that `SKIP LOCKED` can walk past a locked
prefix; it is not a distribution and supports no percentile claim. These
latencies come from the uninstrumented repair-source run at `72c6223`; the
separate race-instrumented integration run is a correctness gate, not a timing
source.
CPU, I/O, allocation, and execution-plan attribution were not separately
profiled because no optimization is proposed. Returned jobs make one bounded
owning payload copy after checking source length; rejected oversized rows make
no payload-sized allocation. Heartbeat returns no job payload. Database round
trips and accepted result ownership are the expected visible costs. Re-profile
when a real consumer supplies representative payloads, concurrency, retention,
and service-level objectives.

The later cursor-range repair at `9f6acc7` adds only validation before an
invalid `ListDead` query and does not alter the successful query path. The
performance matrix was therefore not rerun; these measurements remain ancestor
evidence, not exact-source timing for that repair.

The later `6655331` repair changes Claim only after a commit error and changes
worker coordination only after handler return. It does not alter successful
Claim SQL or the measured claim/complete workload, so the performance matrix
was not rerun and remains ancestor evidence.

The `62d565a` repair moves valid Enqueue preparation before `BeginTx`, changes
the idempotent-conflict read, and adds a public Worker error type. Because the
enqueue allocation order changed, the complete matrix was rerun against exact
clean source on the same designated host and pinned PostgreSQL image. The
workloads do not use idempotency keys, so they exercise the moved valid enqueue
path but not the conflict-only retaining read.

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 794.121 us | 947.374 us | 1.128226 ms | 1245.20 ops/s |
| 100-job small backlog, empty payload | 50 | 3.670492 ms | 8.327549 ms | 8.425270 ms | 114.11 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 4.171220 ms | 4.501594 ms | 5.340076 ms | 130.53 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 4.944501 ms | 7.527697 ms | 7.935184 ms | 92.14 ops/s |
| 100-row locked prefix | 1 | 22.865428 ms | N/A | N/A | N/A |

This is confirmation evidence, not an optimization claim. No formal latency
threshold or representative consumer workload exists yet; the original
provisional admission and re-profile trigger remain unchanged.

The `53cf140` repair removes payload return from Heartbeat and removes the
second payload copy from all job-row scans. Exact-source PostgreSQL integration
benchmarks Heartbeat through pgx with empty and 1 MiB jobs and requires the
large-payload result to remain within 64 KiB/op of the empty-payload allocation;
three repeats passed. The SQL-shape unit test separately requires exactly one
returned boolean and rejects any payload column, covering network response
shape without relying on allocator behavior.

The complete matrix was rerun because shared row scanning changed. It remains
admission evidence rather than a speedup claim:

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 767.891 us | 1.024705 ms | 1.239738 ms | 1254.83 ops/s |
| 100-job small backlog, empty payload | 50 | 3.825155 ms | 4.030597 ms | 5.746831 ms | 136.03 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 4.277140 ms | 9.873139 ms | 10.001151 ms | 115.86 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 7.055610 ms | 12.140182 ms | 12.525347 ms | 63.77 ops/s |
| 100-row locked prefix | 1 | 23.375121 ms | N/A | N/A | N/A |

The Judge 6 repair forces DescribeExec and binary job-column formats on every
job-returning query so the bounded-scanner allocation contract holds even when
a connection defaults to Exec or SimpleProtocol. This intentionally adds a
describe round trip compared with cached extended modes. Exact-source
cross-mode allocation and complete performance results are recorded in the
verification evidence for that repair; no speedup is claimed.

The exact-source matrix at `1fc2a7b` measured the disclosed round-trip cost:

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 1.267654 ms | 1.335244 ms | 1.550079 ms | 769.75 ops/s |
| 100-job small backlog, empty payload | 50 | 5.499281 ms | 7.755423 ms | 10.715397 ms | 90.74 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 6.090113 ms | 7.456307 ms | 7.545329 ms | 94.79 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 7.159882 ms | 10.696947 ms | 13.177113 ms | 64.03 ops/s |
| 100-row locked prefix | 1 | 23.612485 ms | N/A | N/A | N/A |

These results are slower than the prior cached-mode matrix, as expected from
describing every job-returning statement. They remain provisional admission
evidence, not a latency guarantee or optimization claim.

The Judge 7 repair extends the borrowed-source length check from payload to
every stored text field and the request fingerprint. Under all five supported
connection defaults, three exact-source runs rejected a 1 MiB text source with
about 4-19 KiB/op and a 1 MiB fingerprint source with about 9-46 KiB/op. These
figures include pgx/network and transaction storage but remain far below one
source-sized destination ownership copy. Accepted values still make one
bounded library ownership conversion.

Because the shared row scan changed, the complete exact-source matrix at
`9711e2b` was rerun:

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 1.111502 ms | 1.330294 ms | 1.412434 ms | 858.65 ops/s |
| 100-job small backlog, empty payload | 50 | 5.309285 ms | 5.782740 ms | 12.895895 ms | 104.59 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 6.119254 ms | 7.812782 ms | 10.988735 ms | 89.74 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 6.933212 ms | 8.361697 ms | 9.502809 ms | 76.12 ops/s |
| 100-row locked prefix | 1 | 16.594103 ms | N/A | N/A | N/A |

This is confirmation evidence, not an optimization or latency guarantee.

The Judge 8 repair extends borrowed-source validation to the scalar state read
used after a rejected lease mutation. Three exact-source runs across all five
supported pgx connection defaults rejected a 1 MiB state source at 5,963 to
39,351 bytes/op without a library-owned source-sized string. Ten
race-instrumented repeats measured 408,174 to 747,438 bytes/op, below the 1 MiB
source; those figures include race, pgx, network, and transaction storage and
are correctness evidence rather than allocator targets.

The complete exact-source matrix at `4ec1970` was rerun because a production
query and the count query changed:

| Workload | Samples | p50 | p95 | p99 | Loop throughput |
| --- | ---: | ---: | ---: | ---: | ---: |
| empty queue | 100 | 1.310979 ms | 2.369704 ms | 2.742109 ms | 598.16 ops/s |
| 100-job small backlog, empty payload | 50 | 5.309876 ms | 5.572750 ms | 12.339297 ms | 106.08 ops/s |
| 500-job typical backlog, 1 KiB payload | 200 | 6.008526 ms | 7.469857 ms | 8.555403 ms | 97.05 ops/s |
| 20-job, 1 MiB payload boundary | 20 | 7.139892 ms | 8.117877 ms | 8.569803 ms | 72.14 ops/s |
| 100-row locked prefix | 1 | 27.105679 ms | N/A | N/A | N/A |

No optimization, latency guarantee, or speedup is claimed.
