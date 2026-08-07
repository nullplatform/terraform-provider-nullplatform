# terraform-provider-nullplatform

Terraform provider (SDKv2) for nullplatform, published to the public registry.
It is a client of ALL nullplatform APIs (services, scopes, parameters,
packages, approvals, notification channels, …). Where an API documents its
behavior, that document is the reference — `main-service-api`'s
`docs/CASES-MATRIX.md` covers services/links/actions/specifications; consult
the owning API repo for the rest, and validate provider changes against the
API's actual responses, never against assumptions.

## Commands

| Task | Command |
| --- | --- |
| Build | `make build` · install locally: `make install` |
| Unit tests | `go test ./nullplatform/ -count=1` |
| Coverage + gate | `make coverage-new` (runs `tools/covergate`, same gate as CI) |
| Acceptance tests | `make testacc` (needs `TF_ACC=1` + real credentials) |
| Regenerate docs | `make update-docs` (tfplugindocs; CI re-runs it on every PR) |

CI: `.github/workflows/test.yml` (vet, unit suite with `-race`, coverage gate),
`docs.yml` (docs sync), `release.yml` (goreleaser).

## Provider practices

- **Every schema attribute carries a `Description`. This is public registry
  documentation** — say what the attribute does, its default, and its
  interaction with other attributes (see `archive_on_destroy` for the bar).
- Booleans the API defaults: `Optional+Computed`, **no schema `Default`**,
  `*bool` + `omitempty` on the wire, presence via `configuredBool` (utils.go).
  The provider never guesses a platform default — a wrong guess silently
  rewrites resources on the next apply.
- **Errors carry the API's response body** — the 400 message is the only thing
  naming the guard that refused ("archive its links first"). Never
  `io.Copy(os.Stdout, ...)`; two legacy delete/get paths still do — fix if
  touched, never add more.
- Async transitions poll through the shared waiter
  (`waitForInstanceStatusTerminal`) with `Timeouts` declared on the resource;
  ordinary operations must never enter a waiter.
- Use the `Context` CRUD variants; return `diag.Diagnostics`; support
  `Importer` on every resource.

## Documentation

`docs/` is **generated** — never hand-edit. Change schema `Description`s and
`examples/`, then `make update-docs`. Custom `templates/` only where generation
cannot express the page.

## Testing — regression is non-negotiable

- **Never change existing behavior as a side effect.** Old configurations must
  produce byte-identical requests and identical plans. Any deliberate behavior
  change is named in the commit and the PR, with its migration story.
- **Red first**: watch every regression test fail against the unfixed code for
  the stated reason, then go green.
- Unit tests drive the **Context functions Terraform actually invokes** against
  `httptest` fakes (`newTestClient`). Defensive arms unreachable through the
  HTTP client are still contract — drive them through the `NullOps` interface
  (`nullOpsWithNilService` pattern). Polling tests call `shortenPolling(t)`.
- **The suite is deterministic.** An intermittent failure is a bug to
  root-cause (the map-order id flake was a production bug), never noise to
  retry past.
- Coverage gate (CI-enforced): every added line under `nullplatform/` is
  executed by some test, and the total never falls below
  `scripts/coverage_floor.txt` (a ratchet — lower it only when deleting covered
  code, in the same PR, saying so). `scripts/coverage_accepted.txt` entries are
  claims of **impossibility**, not inconvenience.

## Tooling

Repo tooling is **Go only** (`tools/`, stdlib, `go run ./tools/<name>`) — no
Python, no logic in shell. Coverage output (`coverage.out`, `coverage.html`) is
build product: gitignored, never committed. Stage deliberately — no `git add -A`.
