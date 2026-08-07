# terraform-provider-nullplatform

Terraform provider (SDKv2) for nullplatform, published to the public registry.
It is a client of ALL nullplatform APIs (services, scopes, parameters,
packages, approvals, notification channels, …). The platform API's documented
behavior is the reference: validate provider changes against the API's actual
responses, case by case, never against assumptions.

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

Industry standards (HashiCorp provider canon), all load-bearing here:

- **Read reconciles drift**: it reports what the API holds, and a deleted
  resource removes itself from state (`d.SetId("")`) instead of erroring, so
  the next plan offers re-creation.
- **Plans are stable** — a clean apply followed by a plan shows no diff, ever.
  Perpetual diffs are bugs; semantically-equal values get a
  `DiffSuppressFunc` (`suppressEquivalentJSON`).
- **`SetId` the moment the resource exists**, before any follow-up call, so a
  failed enrichment never orphans a live resource; best-effort post-create
  reads warn and continue, never fail the apply.
- `ForceNew` only on attributes the API truly cannot mutate. Secrets are
  `Sensitive: true`.
- **Schema changes are versioned**: anything that breaks existing state needs
  `SchemaVersion` + `StateUpgraders`; releases are semver via goreleaser, and
  a major schema break is a major version.

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
- **Three layers**, all on `terraform-plugin-testing` idioms (never the
  deprecated SDKv2 `helper/resource` — the two cannot share a test binary):
  1. **Unit** — the Context functions against `httptest` fakes
     (`newTestClient`). Defensive arms unreachable through the HTTP client are
     still contract — drive them through the `NullOps` interface
     (`nullOpsWithNilService` pattern). Polling tests call `shortenPolling(t)`.
  2. **Functional** (`functional_test.go`) — `resource.UnitTest` runs REAL
     `terraform plan/apply/import/destroy` against the in-process provider
     backed by `fakePlatform`, a stateful executable copy of the platform
     API's archive contract (guards, refusal messages, managed/unmanaged/
     approval resolution, link rules); only `ConfigureContextFunc` is swapped.
     The framework re-plans after every apply and fails on any diff — the
     perpetual-diff class is checked for free. Runs on every `go test`, no
     credentials. Day one it found the unset-`messages` diff, the `selectors`
     perpetual diff, a third `Selectors` nil-panic and the link delete
     treating the API's 204 as failure. The fake is our belief about the API:
     when the API's behavior changes, change the fake in the same breath, and
     use `make testacc` as the on-demand check that the real API still agrees.
  3. **Acceptance** (`make testacc`, `TestAcc*`) — real API, real credentials,
     gated on `TF_ACC=1`.
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
