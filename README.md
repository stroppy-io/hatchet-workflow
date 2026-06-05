# Stroppy Cloud

Database benchmarking orchestrator. One binary that deploys databases, configures HA topologies, runs [Stroppy](https://github.com/stroppy-io/stroppy) benchmarks, and collects metrics. Supports local Docker runs and Yandex Cloud VMs.

## Supported databases

| Database   | Topologies                          | Versions    |
|------------|-------------------------------------|-------------|
| PostgreSQL | `single`, `ha`, `scale`             | 16, 17      |
| MySQL      | `single`, `replica`, `group`        | 8.0, 8.4    |
| Picodata   | `single`, `cluster`, `scale`        | 25.3        |

## Quick start

```bash
# Build (compiles Go binary with embedded SPA)
make build

# Start the full stack (server + VictoriaMetrics + VictoriaLogs + Grafana)
docker compose -f docker-compose.yaml up -d --build

# Open the web UI
open http://localhost:8080
```

Default login: `admin` / `admin`.

### CLI usage

```bash
# Start a run from JSON config (no server needed)
./bin/stroppy-cloud run -c examples/run-postgres-single.json

# Validate a config
./bin/stroppy-cloud validate -c examples/run-postgres-ha.json

# Print execution DAG
./bin/stroppy-cloud dry-run -c examples/run-mysql-group.json

# Start server with all integrations
./bin/stroppy-cloud serve \
  --addr :8080 \
  --victoria-url http://localhost:8428 \
  --victoria-logs-url http://localhost:9428 \
  --api-key my-secret-key
```

### API usage

The browser and external clients use Connect RPC over the same HTTP origin as
the SPA. RPC paths are generated as `/cloud.v1.api.<Service>/<Method>`; the
frontend clients are in `web/src/services/client.ts`.

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/cloud.v1.api.IamService/Login \
  -H 'Content-Type: application/json' \
  -d '{"login":"admin","password":"admin"}' | jq -r .tokens.accessToken)
```

Package upload is a three-step flow: `PackageService/CreatePackageUpload` mints
a signed PUT URL, the client PUTs the blob to that URL, then
`PackageService/CompleteUpload` verifies size and SHA-256 and marks the package
ready.

## Architecture

```
Server (Go binary)          Agent (same binary, agent mode)
  |                            |
  |- Connect RPC API           |- Receives commands from workflows
  |- Embedded SPA              |- Executes shell scripts
  |- Package PUT gateway       |- Ships logs/metrics back
  |- Workflow worker           |- Manages background daemons
  |- Postgres state            |
  |                            |
  +-- VictoriaMetrics (metrics storage)
  +-- VictoriaLogs (log persistence)
  +-- Grafana (dashboards)
```

The server deploys agents on target machines (Docker containers or Yandex Cloud VMs) via cloud-init. Each agent receives commands from the server, executes them, and reports back. The server orchestrates the entire run as a DAG with automatic retries and backoff.

## CI Integration

See [docs/docs/ci-integration.md](docs/docs/ci-integration.md) for a complete guide on:
- Building custom database packages (.deb/.rpm)
- Uploading packages to the server
- Starting test runs via API
- Comparing results between runs
- Extracting metrics for CI reporting

## Configuration

Example run configs in `examples/`:

- `run-postgres-single.json` -- single-node PostgreSQL
- `run-postgres-ha.json` -- PostgreSQL HA with Patroni + etcd
- `run-mysql-group.json` -- MySQL Group Replication

## Development

```bash
make configure        # Check dependencies
make build            # Build binary (with embedded SPA)
make test             # Unit tests
make test-integration # Integration tests (requires Docker)
make lint             # Run linters
make web-dev          # Start SPA dev server
make docs-dev         # Start docs dev server
```

## Links

- [Stroppy](https://github.com/stroppy-io/stroppy) -- the K6-based benchmark tool
- [Documentation](docs/) -- full API reference, configuration guide, architecture

## License

See the [LICENSE](LICENSE) file for details.
