# Verification status

Current repair verification:

- Repair source `9f6acc74f8901a58a3a9929d10ad7a3779241f4d`
  passed focused local package tests and vet under Go 1.26.6 with
  `GOMAXPROCS=2` and `-p=1`.
- An exact clean clone on `development` passed format, vet, unit, build, race,
  and coverage. Clean-source statement coverage is 96.3%; the shared
  PostgreSQL timestamp predicate and `validateEnqueue` are 100% covered, and
  `ListDead` is 90% covered. Every endpoint, adjacent out-of-range value, and
  the exact wrap-to-Y2K fixture has direct coverage.
- PostgreSQL 17.10 race and coverage integration passed against the pinned
  image. Integration coverage is 96.5%. The new integration regression proves
  the invalid cursor returns `ErrInvalid` instead of executing the wrapped
  query and returning a modern dead row.
- A standalone external consumer passed test and build against the exact clean
  source. The source clone was clean before and after all gates.
- The prior 50-repeat race, fuzz, performance, and Graphify results belong to
  ancestor `72c62231fa4a0012ceef0a9c5ff61ff05feaf859`; they were not rerun for
  this bounded validation-only repair and are not current-source evidence.
- The first independent review rejected candidate
  `671a1eac9ddc6d273136d46de6d906730c7182e5`. Its cursor defect is repaired,
  and exact implementation gates remain bound to repair source `9f6acc74`.
- The second independent review rejected documentation candidate
  `68e2f24b2f20b0c3905d46e9a228d28f044ca9a4` because the runtime contract
  understated Claim's lock and cleanup cardinality. Documentation repair
  `b54cd4f3c385cbe0df1158c2a866efe7fbc216d1` now records up to 100 exhausted
  cleanup rows plus at most one disjoint eligible candidate, with locks held
  to transaction end and at most one returned job.
- The second repair changes no Go or SQL. Focused contract inspection,
  repository-wide false-claim search, whitespace checks, and workflow-format
  validation are proportional; prior race, coverage, PostgreSQL, external,
  repeat, fuzz, performance, and graph evidence was not rerun or rebound.
- Two attributable fresh independent reviews of the final candidate remain
  required and are orchestrator-owned.

Exact commands, artifact hashes, environment differences, and the remaining
review gate are recorded under
`workflow/features/reusable-v0-admission/evidence/`.
