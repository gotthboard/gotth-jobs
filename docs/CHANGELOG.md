# Changelog

This repository records user-visible and compatibility-relevant changes here.
Released sections use Semantic Versioning; unreleased work remains under
`Unreleased` and does not imply a tag.

## Unreleased

### 2026-09-06 00:54 CDT — Route vulnerability reports through GitHub

Commit: current commit; hash assigned by Git after commit

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/CHANGELOG.md`

Explanation:

Keep Forgejo as the canonical development source while routing public bugs to
GitHub Issues and confidential vulnerability reports to GitHub private
security advisories. Remove the obsolete private Forgejo reporting path and
the retired `agents/` namespace.

Verification:

- documentation whitespace and stale-policy scans
- direct GitHub API confirmation that private vulnerability reporting is enabled

Risks / non-goals:

- no source code, public API, tag, release, deployment, or compatibility promise changed

### 2026-09-03 00:42 CDT — Establish GitHub public distribution

Commit: current commit; hash assigned by Git after commit

Affected files:

- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/distribution.md`
- `docs/RELEASING.md`

Explanation:

Declare GitHub as the public distribution endpoint while retaining Forgejo as
canonical development, define maturity and support honestly, and document the
independent release process. The reserved namespace remains a documentation-only placeholder and makes no API or release claim.

Verification:

- exact old-import search
- documentation contract audit

Risks / non-goals:

- No license is selected.
- No existing tag is changed and no new release is created.
- Mirror direction, repository ownership, and account type are unchanged.
