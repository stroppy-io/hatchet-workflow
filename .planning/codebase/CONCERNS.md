# Codebase Concerns

**Analysis Date:** 2026-02-27

## Tech Debt

**Multi-cloud dispatch hardcoded to Yandex Cloud:**
- Issue: `BuildPlacement` and `getCloudInitForEdgeWorker` always read from `settings.GetYandexCloud()` regardless of the deployment target. Four separate `// TODO: dispatch by cloud` comments mark these call sites.
- Files: `internal/domain/provision/provision.go` lines 299, 314, 344, 366
- Impact: Adding a second cloud provider (e.g. AWS, GCP) requires rewriting the entire placement logic. Docker target works accidentally because it falls through to defaults.
- Fix approach: Introduce a per-target `HardwareSettings` and `VmUser` resolver; switch on `target` to pick the correct settings block before constructing `VmTemplate`.

**Terraform actor always initialized for non-Docker targets:**
- Issue: A `sync.Once`-guarded `NewActor()` call runs even when the specific run will use Docker. The TODO comment acknowledges this.
- Files: `internal/domain/deployment/deployment.go` line 51
- Impact: Terraform binary must be present at startup for any non-Docker registry build, even if the run never uses Yandex Cloud. Wastes startup time and requires Terraform installed in the master-worker container.
- Fix approach: Lazy-initialize the Terraform actor on first actual Yandex Cloud deployment request rather than at registry construction.

**Commented-out code left in tree:**
- Issue: Large blocks of commented-out code exist in `internal/domain/deployment/scripting/cloud_init.go` (a full `GenerateCloudInit` function, ~60 lines) and `internal/core/ids/ids.go` (a `HatchetRunId` type, ~8 lines).
- Files: `internal/domain/deployment/scripting/cloud_init.go` lines 54-112, `internal/core/ids/ids.go` lines 56-64
- Impact: Dead code confuses readers about which patterns are canonical.
- Fix approach: Delete commented blocks; recover via git history if needed.

**Hardcoded Grafana URL placeholder:**
- Issue: `RunStroppyTask` returns a result containing a literal `"http://some-grafana-url?runId=%s"` URL.
- Files: `internal/domain/workflows/edge/stroppy.go` line 202
- Impact: The `GrafanaUrl` field in `stroppy.TestResult` is always wrong in production; consumers cannot navigate to real dashboards.
- Fix approach: Add a `GrafanaBaseUrl` setting to `Settings` proto, read it in `RunStroppyTask`.

**Stroppy binary installed via unauthenticated `http.Get`:**
- Issue: `InstallStroppy` uses `http.Get` (not the Hatchet task context) and downloads from a GitHub release URL without checksum verification or TLS pinning.
- Files: `internal/domain/workflows/edge/stroppy.go` lines 41-75
- Impact: Download is not cancellable via context, no retry logic, and no integrity check. A compromised GitHub release would be executed.
- Fix approach: Use `http.NewRequestWithContext` to honour cancellation; verify SHA256 checksum against a known value; add exponential retry.

**`installStroppy` task has no execution timeout:**
- Issue: The `InstallStroppyTaskName` task has a commented-out timeout (`// TODO: add if default timeout is set too low`).
- Files: `internal/domain/workflows/test/test-run.go` lines 297-299
- Impact: If the binary download or extraction hangs, the task runs indefinitely.
- Fix approach: Set an explicit `hatchetLib.WithExecutionTimeout` (e.g. 10 minutes) and resolve the underlying "default too low" concern separately.

**No health-check endpoint on edge worker:**
- Issue: A `// TODO: Add health check endpoint to validate container health` comment sits at the top of `cmd/edge-worker/edge.go`.
- Files: `cmd/edge-worker/edge.go` line 22
- Impact: Edge workers cannot be health-checked by container orchestrators or load balancers. Unhealthy workers silently consume tasks.
- Fix approach: Expose a simple `/healthz` HTTP endpoint that returns 200 when the Hatchet worker is running.

**Worker-wait loop uses `time.Sleep` inside a hot loop:**
- Issue: `waitMultipleWorkersUp` polls Hatchet's worker list in a tight loop with a fixed `time.Sleep(time.Second)`.
- Files: `internal/domain/workflows/test/utils.go` lines 24-42
- Impact: Each iteration makes a full API call; no exponential back-off or jitter. Under high concurrency this hammers the Hatchet API.
- Fix approach: Replace with exponential backoff (already used elsewhere via `cenkalti/backoff`).

## Known Bugs

**Race condition in network reservation (missing lock acquisition):**
- Symptoms: Concurrent test runs can allocate the same subnet. `ReserveNetwork` reads the existing set, computes a new subnet, then writes—but the Valkey locker is created and set up in `NewNetworkManager` but never *acquired* before the read-compute-write sequence in `ReserveNetwork`.
- Files: `internal/domain/managers/network.go` lines 45-108
- Trigger: Two `AcquireNetwork` calls arriving simultaneously for the same `networkStorageKey`.
- Workaround: None. The `locker` field is stored but the `locker.TryLock` / `locker.Unlock` calls are absent in the `ReserveNetwork` body.

**`results` slice in `RunStroppyTestTask` has a data race:**
- Symptoms: Multiple goroutines in the `pool.New()` worker pool each append to the shared `results` slice without synchronisation.
- Files: `internal/domain/workflows/test/test-run.go` lines 312-348
- Trigger: Any test run with more than one `stroppyItem`.
- Workaround: None currently. Fix: use a mutex-protected append or collect results through a channel.

**`Cleanup` in `containers` package accesses `client` before nil-check:**
- Symptoms: If `getDockerClient()` was never called (no containers were deployed), `Cleanup` still calls `client.Close()` on a nil client.
- Files: `internal/domain/edge/containers/docker.go` lines 80-97
- Trigger: Edge worker shutdown when no containers were started.
- Workaround: None; produces a nil-pointer panic on clean shutdown paths.

**Docker deployment uses `network.NetworkHost` but sets internal IP to container ID:**
- Symptoms: `CreateDeployment` for the Docker target creates containers with host networking but stores the Docker container ID (not an IP) in `AssignedInternalIp.Value`. Downstream code that treats this value as an IP address will fail.
- Files: `internal/domain/deployment/docker/docker.go` lines 113-118
- Trigger: Any Docker deployment followed by IP-based routing logic.
- Workaround: The Docker path is partially functional because `DestroyDeployment` re-uses the same `Value` field as a container ID for stop/remove.

## Security Considerations

**Hatchet token passed through Cloud-Init environment variables:**
- Risk: The Hatchet client token (`HATCHET_CLIENT_TOKEN`) is injected into VM Cloud-Init user-data as a plain-text environment variable written to `/etc/hatchet/edge-worker.env`. Cloud-Init data is typically readable from instance metadata APIs without authentication.
- Files: `internal/domain/provision/provision.go` lines 246-267, `internal/domain/deployment/scripting/install-edge-worker.sh` lines 120-157
- Current mitigation: None beyond OS-level access controls on the file.
- Recommendations: Use a secret management service (Yandex Lockbox / Vault) to inject credentials at runtime rather than baking them into cloud-init.

**PostgreSQL default credentials hardcoded in placement builder:**
- Risk: All database containers are created with `postgres`/`postgres` as superuser credentials. These constants are baked into container environment variables.
- Files: `internal/domain/provision/postgres-placement-builder.go` lines 33-35
- Current mitigation: None.
- Recommendations: Generate per-run random credentials; store them in Valkey for retrieval by the stroppy runner.

**Patroni `pg_hba.conf` allows all hosts with MD5:**
- Risk: The embedded docker-compose template configures `host all all 0.0.0.0/0 md5`, accepting connections from any IP.
- Files: `internal/domain/edge/postgres/embed/postgres-docker-compose.yaml` line 41
- Current mitigation: Network isolation via the internal Docker network partially limits exposure.
- Recommendations: Restrict `pg_hba.conf` to the known subnet CIDR; require `scram-sha-256` instead of `md5`.

**Docker socket mounted read-only inside edge worker containers:**
- Risk: Mounting the Docker socket (`/var/run/docker.sock`) into a container—even read-only—grants the container the ability to enumerate all running containers and images on the host.
- Files: `internal/domain/deployment/docker/docker.go` lines 60-67
- Current mitigation: Mount is read-only, so no container creation from inside.
- Recommendations: Evaluate whether the socket mount is actually used by the edge worker; remove if not.

**Terraform TRACE logging enabled by default:**
- Risk: `actor.go` sets Terraform log level to `TRACE`, which outputs all API calls including credentials to stdout/stderr.
- Files: `internal/infrastructure/terraform/actor.go` lines 236-247
- Current mitigation: Log output goes to container stdout; access requires access to container logs.
- Recommendations: Set log level from configuration; default to `WARN` or `ERROR` in production.

**S3 client construction panics on failure:**
- Risk: `NewS3Client` uses `panic(err)` if AWS SDK config loading fails. A misconfigured environment variable silently crashes the process.
- Files: `internal/infrastructure/s3/s3.go` line 30
- Current mitigation: The caller never receives an error to handle.
- Recommendations: Return `(*s3.Client, error)` from `NewS3Client`; let callers decide how to handle config failures.

## Performance Bottlenecks

**Valkey locker `ExtendInterval` set to 1 minute but `KeyValidity` is 5 seconds:**
- Problem: `NewValkeyLocker` sets `KeyValidity: 5*time.Second` and `ExtendInterval: time.Minute`. The lock will expire 12x before the first extension fires, causing spurious lock loss under any operation taking more than 5 seconds.
- Files: `internal/infrastructure/valkey/locker.go` lines 11-26
- Cause: Likely a copy-paste from a default configuration; values were not tuned.
- Improvement path: Either increase `KeyValidity` to match expected operation duration (≥1 minute) or decrease `ExtendInterval` to a fraction of `KeyValidity` (e.g. 2 seconds).

**S3 `CreateS3BucketIfNotExists` does a full `ListBuckets` call on every invocation:**
- Problem: `ListBuckets` returns all buckets in the account; the function then searches the list locally. At scale this is expensive both in latency and IAM permission scope.
- Files: `internal/infrastructure/s3/s3.go` lines 45-57
- Cause: Simple linear scan approach; no caching.
- Improvement path: Use `HeadBucket` to probe for existence directly; fall back to create only on 404.

**`s3.go` uses `context.TODO()` for all calls:**
- Problem: All S3 operations (`LoadDefaultConfig`, `ListBuckets`, `CreateBucket`) use `context.TODO()`, so they cannot be cancelled by callers and will block indefinitely on network failure.
- Files: `internal/infrastructure/s3/s3.go` lines 17, 46, 53
- Cause: Context threading was not completed.
- Improvement path: Accept `context.Context` parameters in `NewS3Client` and `CreateS3BucketIfNotExists`.

**Suite workflow polls run status in a tight backoff loop:**
- Problem: `waitForRunCompletion` polls `c.Runs().Get()` using exponential backoff with no maximum elapsed time (`MaxElapsedTime: 0`). For very long test runs (up to 24 h), this generates thousands of API calls.
- Files: `internal/domain/workflows/test/test-suit.go` lines 86-116
- Cause: No event-driven notification mechanism from Hatchet; polling is the only option.
- Improvement path: Cap maximum polling interval at 30 seconds; document the polling cost.

## Fragile Areas

**`postgres-placement-builder.go` (588 lines) builds all container specs imperatively:**
- Files: `internal/domain/provision/postgres-placement-builder.go`
- Why fragile: A single large builder function accumulates all database topology permutations. Adding a new sidecar or changing port assignments requires editing deep into the builder.
- Safe modification: Changes must update both the builder and the corresponding test in `internal/domain/provision/postgres_placement_builder_test.go` (871 lines). Run the full test suite before merging.
- Test coverage: High; 871-line test file covers major topology combinations.

**`docker.go` (containers package, 681 lines) uses process-global mutable state:**
- Files: `internal/domain/edge/containers/docker.go`
- Why fragile: `globalMapping` and `client` are package-level variables initialised via `sync.Once`. Tests that run in the same process share this state; a failed test can corrupt state for subsequent tests.
- Safe modification: Avoid running integration tests in parallel (`-parallel 1`); reset `globalMapping` between tests if possible.
- Test coverage: One integration test file (`docker_integration_test.go`); no unit tests for individual functions.

**Terraform working directory uses `/tmp/stroppy-terraform`:**
- Files: `internal/infrastructure/terraform/actor.go` line 23
- Why fragile: On process restart the working directory is recreated but in-memory `workdirs` map is lost. `DestroyTerraform` will fail for any deployment created before the restart because it looks up `WdId` in the in-memory map.
- Safe modification: Always restart cleanly; do not rely on destroy after master-worker restart.
- Test coverage: One test (`TestInstallTerraform`) that only verifies binary installation; no destroy path is tested.

**`onceTerraformActor` is a package-level `sync.Once`:**
- Files: `internal/domain/deployment/deployment.go` lines 36-38
- Why fragile: If `terraform.NewActor()` fails, the `err` from the `once.Do` closure is captured by the outer function's `err` variable. But `sync.Once` guarantees the closure runs only once—a transient failure permanently prevents the Terraform actor from being created for the lifetime of the process.
- Safe modification: Do not retry; fix the underlying error (missing binary, missing `/tmp` permissions) before starting the process.

**Web UI `handleExecute` is a visual mock only:**
- Files: `web/src/App.tsx` lines 86-98
- Why fragile: Clicking "Run" triggers a `setInterval` animation loop with no actual API call. The "Users" section is also a static placeholder.
- Safe modification: Do not treat UI state as workflow state; any real integration must be added as a separate HTTP/gRPC call.

## Scaling Limits

**Valkey network key uses target+name as discriminator:**
- Current capacity: All reservations for a given `(target, networkName)` pair are stored in a single Redis Set. Works correctly for low concurrency.
- Limit: With thousands of concurrent runs under the same network name, the Set grows unboundedly. Subnet scan in `ips.NextSubnetWithIPs` becomes O(n) against a large set.
- Scaling path: Shard by run ID or use a dedicated subnet allocation service.

**Terraform `workdirs` map is in-process:**
- Current capacity: Bounded by memory; each entry holds file paths.
- Limit: Across a restart all active deployments become un-destroyable.
- Scaling path: Persist working directory metadata to Valkey or a database keyed by deployment ID.

## Dependencies at Risk

**`github.com/mitchellh/mapstructure` (v1.5.0):**
- Risk: Unmaintained since 2021; used in `test-suit.go` to decode workflow run output. The newer `go-viper/mapstructure/v2` (already in the indirect dependency tree) is its successor.
- Impact: Struct tag semantics may diverge from protobuf JSON output; silent field mismatches.
- Migration plan: Replace `mapstructure.Decode(run.Run.Output, &result)` with direct protobuf JSON unmarshalling.

**`edoburu/pgbouncer:latest` and other `:latest` image tags:**
- Risk: Placement builder hardcodes `:latest` for PgBouncer, node-exporter, postgres-exporter images. These tags are mutable and can change between test runs.
- Files: `internal/domain/provision/postgres-placement-builder.go` lines 18-22
- Impact: Non-reproducible test environments; a registry update could break running tests.
- Migration plan: Pin all image tags to specific digest or semantic version.

## Missing Critical Features

**No cleanup on workflow failure before `deployPlanTask` completes:**
- Problem: The `OnFailure` handler in `TestRunWorkflow` reads the parent output of `deployPlanTask` to destroy provisioned infrastructure. If the failure occurs in `acquireNetworkTask`, `planPlacementIntentTask`, or `buildPlacementTask`—before deployment—`ctx.ParentOutput(deployPlanTask, &placement)` will fail, leaving the `OnFailure` handler itself erroring and not cleaning up the allocated network.
- Files: `internal/domain/workflows/test/test-run.go` lines 384-415
- Blocks: Reliable infrastructure cleanup on early failure.

**No observability into Stroppy test results beyond Grafana URL:**
- Problem: `TestResult` carries only a `GrafanaUrl` (which is a placeholder) and the raw test/run-id reference. There is no structured result parsing, threshold comparison, or test-failure detection.
- Files: `internal/domain/workflows/edge/stroppy.go` lines 198-208, `internal/proto/stroppy/test.pb.go`
- Blocks: Automated pass/fail determination; CI integration.

## Test Coverage Gaps

**`provision.go` and `BuildPlacement` are untested:**
- What's not tested: The full `BuildPlacement` function that assembles VM templates, the `AcquireNetwork` / `FreeNetwork` round-trip, and `DeployPlan` / `DestroyPlan` lifecycle.
- Files: `internal/domain/provision/provision.go`
- Risk: Cloud dispatch TODOs and IP allocation logic can break silently.
- Priority: High

**`deployment/docker/docker.go` `CreateDeployment` has no tests:**
- What's not tested: Container creation, the Docker socket mount logic, and the `DestroyDeployment` error aggregation.
- Files: `internal/domain/deployment/docker/docker.go`
- Risk: Docker-target runs are untested beyond manual smoke tests.
- Priority: High

**`valkey/locker.go` lock validity mismatch is not exercised:**
- What's not tested: Concurrent calls to `ReserveNetwork` that would expose the missing lock acquisition and the misconfigured `KeyValidity`/`ExtendInterval` ratio.
- Files: `internal/infrastructure/valkey/locker.go`, `internal/domain/managers/network.go`
- Risk: Race condition in network allocation discovered only under production load.
- Priority: High

**Web UI has no tests of any kind:**
- What's not tested: Component rendering, YAML generation from proto state, topology canvas interactions, settings propagation.
- Files: `web/src/`
- Risk: Regressions in proto schema changes silently break YAML output.
- Priority: Medium

---

*Concerns audit: 2026-02-27*
