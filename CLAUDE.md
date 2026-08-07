# terraform-provider-nullplatform — Project Instructions

## Language & Tooling

- **[Learned] This is a Go repository: repo tooling is written in Go, never Python or
  shell-with-logic.** The coverage gate lives in `tools/covergate` (pure stdlib, run via
  `go run ./tools/covergate`) precisely because a Python version was rejected. New tools
  follow the same pattern: a `package main` under `tools/`, stdlib-only unless there is a
  strong reason, no new entries in the main `go.mod`.
- Docs under `docs/` are **generated** (`make update-docs` / tfplugindocs, kept in sync by
  `.github/workflows/docs.yml`). Never hand-edit them; change the schema `Description`s and
  `examples/` instead. Custom templates in `templates/` exist only where generation cannot
  express the page.

## Coverage — two rules, enforced by CI

`make coverage-new` (and the `test` workflow on every PR) runs `tools/covergate`:

1. **Every line a branch adds under `nullplatform/` must be executed by some test.**
   The bar applies to the change, not the repo's historical total.
2. **Total statement coverage never falls below `scripts/coverage_floor.txt`.** The floor
   is a ratchet: raise it when a PR lifts the total; lower it ONLY when deleting covered
   code, in the same PR, saying so in the commit message. It is deliberately not "every
   commit must improve the total" — deleting covered dead code and pure refactors are
   legitimate and flat.

- `scripts/coverage_accepted.txt` lists lines accepted as uncovered, each with its reason.
  **An entry is a claim of impossibility, not of inconvenience** (e.g. `d.Set` error arms
  that only fire on a schema type mismatch). Review the file like code; a growing file is
  a smell. Coverage output (`coverage.out`, `coverage.html`) is build product — gitignored,
  never committed.

## Testing

- **Watch every regression test fail first** against the unfixed code, for the stated
  reason, then go green. A test that never failed proves nothing (the approval-race test in
  `resource_service_archive_test.go` documents a real bug caught exactly this way).
- **The suite must be deterministic.** The `TestGenerateParameterValueID` flake was a real
  production bug (id hashed in map-iteration order); it is fixed with sorted keys and a
  100-round determinism test. Treat any new intermittent failure as a bug to root-cause,
  never as noise to retry past.
- Unit tests drive resources through the **Context functions Terraform actually invokes**
  (`ServiceCreateContext`, `LinkDeleteContext`, …) against `httptest` fakes via
  `newTestClient`. Defensive arms unreachable through the HTTP client (nil-instance
  checks) are still contract: drive them through the `NullOps` interface with an embedded
  stub (`nullOpsWithNilService` pattern).
- Waiter/polling tests call `shortenPolling(t)`; intervals are vars for exactly this.

## API contract — the provider is a client of main-service-api

- **`main-service-api`'s `docs/CASES-MATRIX.md` is the behavioral reference** (§13a
  Z1–Z21 for archive). Provider changes that touch lifecycle, status, actions or
  specifications are validated against it case by case — PR #147's body carries the
  disposition table as the template for how to do this.
- **Error bodies must reach the operator.** Every refusal arrives as a 400 whose message
  is the only thing naming the guard ("archive its links first", the archived-twin
  "unarchive it, or request its deletion"). Client methods include the response body in
  returned errors — never `io.Copy(os.Stdout, ...)` (two legacy delete/get paths still do;
  fix them if touched, and never add new ones).
- Tri-state booleans (`use_default_actions`, `use_managed_actions`, …) follow the
  `configuredBool` idiom from `utils.go`: `Optional+Computed`, `*bool` + `omitempty` on
  the wire, presence detected via raw config. No schema `Default` for values the API
  defaults — the provider must not guess the platform's defaults.
- **Match main's idiom over introducing a parallel helper.** PR #148 was rewritten when
  main landed `configuredBool` independently: one mechanism, main's name, ours deleted.

## Conventions

- Conventional commits; never any Claude attribution.
- `git add -A` is how `coverage.out` once got committed — stage deliberately.
