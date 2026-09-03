# Distribution Contract

## Endpoints

- Canonical development and change tracking:
  <https://git.dannyhunn.com/agents/gotth-jobs>
- Public clone and, after implementation and consumer admission, releases:
  <https://github.com/gotthboard/gotth-jobs>

Forgejo pushes one way to GitHub. GitHub does not feed commits or tags back to
Forgejo. A ref is distributed only when the exact object ID is visible at both
endpoints.

## Maturity and compatibility

Current status: unreleased Go library with an unstable pre-1.0 API.

## Installation

The future import path is `github.com/gotthboard/gotth-jobs/pkg/jobs`. No tag or
compatibility promise exists yet; consumers must not pin it until admission.

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
