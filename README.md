# gotth-jobs

> **Distribution:** GitHub is the public clone and, only if implementation is
> admitted later, the future release endpoint.
> Forgejo remains canonical development and the issue/contribution location.
> See [the distribution contract](docs/distribution.md).


Reserved for reusable durable background-job mechanics shared by GOTTH
applications.

## Intended boundary

This project may eventually own job envelopes, atomic claiming, leases,
heartbeats, cancellation, bounded retries, idempotency, dead-letter handling,
and worker observability. Consumers retain job meaning, authorization,
transactional enqueue decisions, payload minimization, and recovery policy.

## Non-goals

- A distributed scheduler selected before workload requirements exist.
- Product notification, media, federation, or webhook policy.
- Claiming exactly-once side effects where the underlying systems cannot
  provide them.

## Status

Placeholder only. There is no implementation, API, release, tag, compatibility
promise, or dependency to pin.

## Installation, compatibility, and support

Planned placeholder only. There is no implementation, API, support promise, or
release.

There is nothing to install or import. Do not add this repository as a
dependency.

The repository has no selected license and no long-term support promise.
Versioning, release admission, security reporting, and contribution details are
in [the release policy](docs/RELEASING.md), [security policy](SECURITY.md), and
[contribution guide](CONTRIBUTING.md).
