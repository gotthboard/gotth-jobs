# Distribution Contract

## Endpoints

- Canonical development source:
  <https://git.dannyhunn.com/gotthboard/gotth-jobs>
- Public clone and, after consumer and release admission, releases:
  <https://github.com/gotthboard/gotth-jobs>
- Public bug tracker:
  <https://github.com/gotthboard/gotth-jobs/issues>
- Private vulnerability reports:
  <https://github.com/gotthboard/gotth-jobs/security/advisories/new>

Forgejo pushes one way to GitHub. GitHub does not feed commits or tags back to
Forgejo. A ref is distributed only when the exact object ID is visible at both
endpoints.

## Maturity and compatibility

Current status: unreleased Go library with an unstable pre-1.0 API.

## Installation

The import path is `github.com/gotthboard/gotth-jobs/pkg/jobs`. The standalone
implementation is technically admitted, but no tag or compatibility promise
exists yet. Product consumers may use an exact candidate pin for integration
proof; they must not treat it as a released dependency until release admission.

The repository pins Go 1.26.6 where a Go module exists. Supported protocol,
runtime, database, and tool versions remain the ones stated in the README and
project verification documents; this distribution change does not widen those
contracts.

## License

The maintainer selected the MIT license for all canonical `gotth-*` projects.
This repository carries the standard MIT text in `LICENSE`.

## Migration traceability

| Requirement | Repository implementation | Verification |
| --- | --- | --- |
| DIST-001 | Existing history, tags, worktrees, and mirror direction remain unchanged | pinned ref and worktree inventory |
| DIST-005 | Unreleased implementation claims no compatibility release | tracked-tree and README audit |
| DIST-003/004 | README, contribution, security, changelog, and release contracts describe public use and support | documentation audit |
| DIST-006 | MIT license is present and documented | license inventory |
| DIST-008 | Forgejo remains source and GitHub remains the one-way mirror target | push-mirror configuration and exact ref comparison |
