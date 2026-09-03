# Distribution Contract

## Endpoints

- Canonical development and change tracking:
  <https://git.dannyhunn.com/agents/gotth-jobs>
- Public clone and, only after implementation admission, future releases:
  <https://github.com/gotthboard/gotth-jobs>

Forgejo pushes one way to GitHub. GitHub does not feed commits or tags back to
Forgejo. A ref is distributed only when the exact object ID is visible at both
endpoints.

## Maturity and compatibility

Current status: planned placeholder; no implementation or API.

## Installation

There is nothing to install or import. This repository is a planned namespace,
not a library release.

The repository pins Go 1.26.6 where a Go module exists. Supported protocol,
runtime, database, and tool versions remain the ones stated in the README and
project verification documents; this distribution change does not widen those
contracts.

## Licensing gate

No license file is present. No license has been inferred or selected. New
release publication remains blocked until the maintainer makes that decision.

## Migration traceability

| Requirement | Repository implementation | Verification |
| --- | --- | --- |
| DIST-001 | Existing history, tags, worktrees, and mirror direction remain unchanged | pinned ref and worktree inventory |
| DIST-005 | Placeholder remains documentation-only and claims no release | tracked-tree and README audit |
| DIST-003/004 | README, contribution, security, changelog, and release contracts describe public use and support | documentation audit |
| DIST-006 | Missing license is stated as a decision gate | license inventory |
| DIST-008 | Forgejo remains source and GitHub remains the one-way mirror target | push-mirror configuration and exact ref comparison |
