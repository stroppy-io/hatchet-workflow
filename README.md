# Stroppy Cloud

Database benchmarking as a service. The product server over
[Graphene CI](https://github.com/graphene-ci): tenants, benchmark model and
results UI live here; provisioning, agents, runs and observability are
Graphene's. Identity is [gopherex/iam](https://github.com/gopherex/iam).

This branch is a ground-up rewrite. The previous implementation is preserved on
`main-v0`.

## Layout

- `cmd/stroppy-server/` — the server binary (API + embedded SPA).
- `internal/` — server code.
- `pipelines/` — Graphene pipelines (`stroppy-run`, `stroppy-suite`), a separate
  Go module; `pipelines/spec` is the run specification shared with the server.
- `api/` — OpenAPI spec (source of truth for the Go server and the TS client).
- `web/` — SPA (React, Vite, Tailwind).
- `docs/` — documentation and design notes.

## Development

```bash
make configure
make build
make test
make lint
make web-dev
```
