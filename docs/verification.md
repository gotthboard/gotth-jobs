# Verification status

Current Judge 10 repair verification:

- Independent Judge 10 rejected candidate
  `a6900a151c2fecf8eae21d845f82880643f81d28` because legacy `panicnil=1`
  behavior let `panic(nil)` reach Complete and because a custom Store's large
  unknown state was copied into validation errors.
- Exact implementation repair source
  `389915b6c4f27b1a2d5912de369a80b918c394fb`, tree
  `81511f58ac262b3adcfb5ce3a77d381f84288116`, uses an explicit normal-return
  flag and constant classified unknown-state text.
- Expected red proved `panic(nil)` called Complete once and Fail zero times.
  The preallocated 1 MiB state caused 5,284,888 library bytes allocated and a
  1,048,665-byte error.
- Focused local repeats, the full package, vet, formatting, integration-tag
  compilation, and `git diff --check` passed with `GOMAXPROCS=2` and
  `go test -p=1`.
- Exact detached source on `development` passed format, vet, unit, build, full
  race, 50 affected race repeats, 20 verbose allocation runs, 100 focused
  repeats, and coverage.
- Unit statement coverage is 97.5%; `callHandler` and `validateStoredJob` are
  100% covered. Retained allocation samples used 648-1,088 bytes to reject the
  preallocated 1 MiB unknown state and returned an 86-byte error.
- No relevant existing fuzz target reaches handler unwinding or custom stored
  jobs, so fuzz was not rerun. PostgreSQL, external-consumer, graph, and
  PostgreSQL performance gates were not invalidated and remain ancestor
  evidence only.
- Literal commands, cwd, toolchain, `GOMAXPROCS`, package patterns, regexes,
  options, exact HEAD/tree, bundle hash, and clean pre/post status are retained
  in the hashed runner and transcript. An initial runner failed before cloning
  because bundle verification lacked a repository context; it is retained and
  superseded by the successful runner.
- Workflow remains active. Two fresh attributable independent reviews of the
  final candidate remain orchestrator-owned; this worker claims no admission.

Complete hashes and scope are recorded in
`workflow/features/reusable-v0-admission/evidence/verification.md`.
