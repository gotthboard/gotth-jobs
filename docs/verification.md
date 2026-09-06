# Verification status

Current repair verification:

- Repair source `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`
  passed focused local package tests and vet under Go 1.26.6 with
  `GOMAXPROCS=2` and `-p=1`.
- An exact clean clone on `development` passed format, vet, unit, build, race,
  50 repeated race runs, coverage, and two five-second fuzz admissions.
- Clean-clone statement coverage is 96.3%; PostgreSQL integration coverage is
  96.5%. `validateEnqueue` is 100% covered. `validateStoredJob` is 95.5%
  covered; its residual is the preexisting terminal-timestamp rejection, which
  one older malformed fixture now reaches the new state/attempt rejection
  before. Every legal and illegal state/attempt boundary has direct coverage.
- PostgreSQL 17.10 integration passes against the pinned image, including exact
  availability endpoint round trips and all 16 state/attempt boundary pairs,
  in addition to the existing transaction, concurrency, fencing, lifecycle,
  cancellation, dead-letter, and worker cases.
- Fuzz admissions executed 66,722 envelope inputs and 342,657 diagnostic-text
  inputs without a product failure.
- Performance, separate external-consumer compilation, and Graphify integrity
  gates pass. The measured matrix and limitations are in
  `docs/performance.md`.
- The prior clean review files are historical and do not admit this repair.
  Two attributable fresh independent reviews of the final candidate remain
  required and are orchestrator-owned.

Exact commands, artifact hashes, environment differences, and the remaining
review gate are recorded under
`workflow/features/reusable-v0-admission/evidence/`.
