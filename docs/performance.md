# Performance admission

## Decision

No optimization was introduced and no speedup is claimed. The direct
PostgreSQL mechanism remains a provisional candidate because its measured
behavior is bounded enough for the first consumer and its cost remains visible:
one short transaction per mutation, one returned payload copy, and an indexed
`FOR UPDATE SKIP LOCKED` scan whose work grows with eligible and locked rows.

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
profiled because no optimization is proposed. The returned payload copy and
database round trips are the expected visible costs. Re-profile when a real
consumer supplies representative payloads, concurrency, retention, and
service-level objectives.
