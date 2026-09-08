# OpenAPI — the HTTP contract

`openapi.yaml` is **generated** from `parts/*.yaml` by `make openapi`
(`scripts/openapi_merge.py`). Edit the parts; never the merged file. Parts are
plain OpenAPI fragments (`paths` and/or `components`) merged one level deep; a
path or component defined twice is a merge error.

| Part | Contents |
| --- | --- |
| `00-head.yaml` | info, servers, tags, tag groups, default security |
| `10-components-common.yaml` | security scheme, shared parameters, `Problem`, schemapb envelopes, `Entity`, sizes, fit |
| `20-public-me-tenants.yaml`, `21-components-account.yaml` | public, `/me`, tenants, members, invites, tokens, audit |
| `30-tenant-settings.yaml` | settings, limits, provider profiles, quotas, webhooks |
| `40-catalog.yaml` | databases, providers/sizes, stroppy, examples, schemas, metrics |
| `50-library.yaml` | databases, workloads, tests |
| `60-runs.yaml`, `61-components-runs.yaml` | runs, compare |
| `70-suites-schedules.yaml` | suites, suite runs, schedules |
| `80-results.yaml` | favorites, shares, rating, dashboard |
| `90-admin.yaml` | platform administration |

Pipeline: `parts/*.yaml` → `openapi.yaml` (3.1, source of truth) →
`.build/openapi.3.0.yaml` (`scripts/openapi_to_30.py`, ogen consumes 3.0) →
`internal/oas` (Go server + client, `make generate-go`) and the TS client
(`make generate-ts`).

## Conventions

- Prefix `/api/v1`. Tenant scope in the path: `/api/v1/t/{slug}/...`.
- `Authorization: Bearer <token>` — IAM access token or `stc_...` API token.
  Public endpoints declare `security: []`.
- Errors: RFC 9457 `application/problem+json` (`Problem`) with a stable `code`;
  domain validation adds `validation` (schemapb `ValidationResult`).
- Lists: `{data[], meta{next_cursor, has_more}}`; filters as query params;
  `sort`/`order`; facet endpoints `...:facets`.
- Actions are `POST <resource>:<verb>`; `Idempotency-Key` on launches.
- Forms: a property marked `x-schema: <id>` is a schemapb value. Schemas are
  served by `/api/v1/catalog/schemas/{id}` and validated by
  `/api/v1/catalog/schemas/{id}:validate`. Ids: `db.<kind>.params`,
  `cfg.<software>@<major>`, `workload.segment`, `provider.<kind>.settings|credentials`,
  `spec.run`, `system.stroppy_catalog`.

## WebSocket `/api/v1/ws`

Not expressible in OpenAPI; the contract lives here. One connection per
client, JSON text frames, `Authorization` via the first message.

```jsonc
// client → server
{ "type": "auth",        "token": "..." }
{ "type": "subscribe",   "sub_id": "s1", "topic": "run.overview/<run_id>", "cursor": null }
{ "type": "subscribe",   "sub_id": "s2", "topic": "run.logs/<run_id>", "filter": { "role": ["master"], "q": "ERROR" } }
{ "type": "unsubscribe", "sub_id": "s2" }
{ "type": "input",       "sub_id": "s3", "data": "ls\n" }              // agent.pty
{ "type": "resize",      "sub_id": "s3", "cols": 120, "rows": 40 }     // agent.pty
{ "type": "ping" }

// server → client
{ "type": "ready",  "sub_id": "s1" }
{ "type": "event",  "sub_id": "s1", "cursor": "…", "payload": { ...RunOverview } }
{ "type": "event",  "sub_id": "s2", "cursor": "…", "payload": { "lines": [ ...LogLine ] } }
{ "type": "event",  "sub_id": "s3", "payload": { "data": "…" } }        // pty output
{ "type": "dropped","sub_id": "s2", "count": 120 }                     // slow consumer
{ "type": "closed", "sub_id": "s1", "reason": "terminal" }
{ "type": "error",  "sub_id": "s2", "problem": { ...Problem } }
{ "type": "pong" }
```

Topics: `run.overview/{id}` (full `RunOverview` per tick, closes when the run
is terminal), `run.events/{id}`, `run.logs/{id}` (tail; `filter` = the typed
log query params; resume by `cursor`), `run.metrics/{id}` (live samples for the
catalog keys), `suite_run/{id}` (`SuiteRun`), `tenant.runs/{slug}` (list
deltas: `{run_id, status, phase, summary}`), `agent.pty/{run_id}/{machine}`
(half-duplex pty; `input`/`resize` from the client). Every topic is
tenant-scoped by the token; a subscription outside the caller's tenants is
refused with `error`.
