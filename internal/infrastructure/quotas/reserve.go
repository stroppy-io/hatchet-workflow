// Package quotas: this file remodels Manager.Reserve/Commit/Release (deleted
// in efd9bffc alongside the old InfrastructurePlan/QuotaRequestRef-based test
// workflow) to derive quota demand from a dslpb.CompiledPlan's machine
// groups instead — the recipe/YAML-DSL pivot's only source of "how much
// infrastructure does this run need" (see internal/workflows/runrecipe.go's
// infra stage, which calls Reserve before ProvisionActivity/Commit after
// ExecuteCompiledPlanWorkflow succeeds/Release unconditionally in its
// teardown defer).
//
// # machine_groups -> resource-kind amounts
//
// amountsForGroups sums three raw dimensions across every machine_groups
// entry (each already expanded to its final per-node resources by the
// compiler — see dslpb.MachineGroup's cpu/ram_mb/disks doc comments):
//   - cpu     = Σ group.cpu * group.count       (vCPU cores)
//   - ram_mb  = Σ group.ram_mb * group.count     (megabytes)
//   - disk_gb = Σ (Σ group.disks[].size_gb) * group.count (gigabytes)
//
// This intentionally does NOT split disk_gb by DiskSpec.Type (e.g. Yandex's
// lowered "network-ssd"/"network-hdd" — see internal/dsl/lower/lower.go) even
// though Yandex Cloud actually tracks SSD/HDD capacity as separate quotas
// (compute.ssdDisks.size vs compute.hddDisks.size, confirmed against
// https://yandex.cloud/en/docs/compute/concepts/limits): the v1 remodel
// checks/reserves every recipe's disk demand against the SSD quota
// unconditionally (see quotaDimensionsFor's PROVIDER_YANDEX case) — recipes
// provisioning HDD-backed groups on Yandex will reserve against the wrong
// quota row until a future task splits this dimension. Documented gap, not a
// silent guess: see task-2-report.md.
//
// # resource-kind -> quota_name mapping (the snapshot-keying question)
//
// quota_snapshots is keyed by (tenant, provider, resource_type, resource_id,
// quota_name) — quota_name is whatever string the live ProviderSource
// reported it under (see types.go's ProviderSource/Snapshot). That string is
// NOT a fixed "cpu"/"ram"/"disk" convention; it is provider-specific:
//
//   - DockerSource (docker.go) hardcodes "host.cpuCores" (units: cores),
//     "host.memory.size" (units: MiB) and "host.disk.size" (units: GiB) —
//     these line up with cpu/ram_mb/disk_gb exactly (same units, no
//     conversion), because DockerSource IS this codebase (we own the
//     naming).
//   - YandexSource (yandex.go) reports whatever quota_id Yandex Cloud's
//     QuotaLimitService.List returns for the "compute" service on this
//     tenant's cloud — NOT hardcoded anywhere in this repo or in the
//     yandex-cloud/go-sdk vendor tree (confirmed: no static quota_id
//     constants exist in go-sdk@v0.31.0 or go-genproto@v0.85.0's
//     quotamanager packages; the ids are discovered live, not compiled in).
//     Yandex Cloud's public docs (yandex.cloud/en/docs/compute/concepts/
//     limits) DO document stable, versioned quota_id strings though:
//     compute.instanceCores.count (vCPUs, unit: count),
//     compute.instanceMemory.size (unit: GB) and compute.ssdDisks.size
//     (unit: GB) — quotaDimensionsFor hardcodes exactly these three for
//     PROVIDER_YANDEX. This is an external assumption (verified against
//     Yandex's docs, not against a live account/quota snapshot — this
//     sandbox has no YC credentials) rather than a discoverable constant;
//     if a live account's ListQuotas ever reports these quota_ids under
//     different strings, Reserve will see ErrSnapshotMissing (fails closed,
//     not silently open) rather than reserving against nothing.
//
// So: the mapping DOES fit a "cpu/ram/disk" aggregation for both providers
// this codebase supports today (Docker, Yandex) — it just is NOT the same
// three literal strings for both, and for Yandex specifically it also
// requires a unit conversion (ram_mb is megabytes; compute.instanceMemory.
// size is gigabytes — mbToGB below divides by 1024 and rounds up, i.e.
// treats "1 GB" as 1024 MB, matching how this codebase already treats
// Docker's ram_mb/MiB as a 1:1 binary-unit correspondence).
package quotas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	derrors "github.com/stroppy-io/stroppy-cloud/internal/domain/errors"
	deploymentpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/deployment"
	dslpb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/dsl"
)

// Resource-kind keys amountsForGroups sums into — internal to this file,
// never persisted (quotaDimensionsFor translates them into the provider's
// real quota_name before anything touches the store).
const (
	quotaKindCPU  = "cpu"
	quotaKindRAM  = "ram_mb"
	quotaKindDisk = "disk_gb"
)

// quotaDimension is one resource kind's mapping, for a specific provider,
// onto the quota_snapshots row Reserve must check/consume — see this file's
// package doc for the "resource-kind -> quota_name mapping" rationale.
type quotaDimension struct {
	QuotaName string
	Units     string
	// Convert turns amountsForGroups' raw sum (always in the DSL's own
	// units — vCPU cores, megabytes, gigabytes) into the amount Reserve
	// checks/writes, expressed in Units above.
	Convert func(raw uint64) uint64
}

func identityAmount(raw uint64) uint64 { return raw }

// mbToGB converts a raw megabyte sum into gigabytes, rounding up so a
// request never under-reserves a partial GB. 1 GB is treated as 1024 MB
// (binary convention), matching how DockerSource's own "host.memory.size"
// quota already treats ram_mb 1:1 against MiB.
func mbToGB(raw uint64) uint64 {
	return uint64(math.Ceil(float64(raw) / 1024.0))
}

// quotaDimensionsFor returns provider's cpu/ram_mb/disk_gb -> quota_name
// mapping, or nil if Reserve does not (yet) support enforcing quota for
// provider — see this file's package doc.
func quotaDimensionsFor(provider deploymentpb.Provider) map[string]quotaDimension {
	switch provider {
	case deploymentpb.Provider_PROVIDER_DOCKER:
		return map[string]quotaDimension{
			quotaKindCPU:  {QuotaName: "host.cpuCores", Units: "cores", Convert: identityAmount},
			quotaKindRAM:  {QuotaName: "host.memory.size", Units: "MiB", Convert: identityAmount},
			quotaKindDisk: {QuotaName: "host.disk.size", Units: "GiB", Convert: identityAmount},
		}
	case deploymentpb.Provider_PROVIDER_YANDEX:
		return map[string]quotaDimension{
			quotaKindCPU: {QuotaName: "compute.instanceCores.count", Units: "count", Convert: identityAmount},
			quotaKindRAM: {QuotaName: "compute.instanceMemory.size", Units: "GB", Convert: mbToGB},
			// SSD-only — see package doc's disk_gb caveat.
			quotaKindDisk: {QuotaName: "compute.ssdDisks.size", Units: "GB", Convert: identityAmount},
		}
	case deploymentpb.Provider_PROVIDER_UNSPECIFIED:
		return nil
	default:
		return nil
	}
}

// providerFromDslName maps a dslpb.ProviderRef.Name ("docker"/"yandex" — the
// only two builtin providers internal/infrastructure/provider.
// NewProviderForRef resolves, see factory.go) onto the deploymentpb.Provider
// enum Scope/quota_snapshots key by. Returns PROVIDER_UNSPECIFIED for
// anything else.
func providerFromDslName(name string) deploymentpb.Provider {
	switch name {
	case "docker":
		return deploymentpb.Provider_PROVIDER_DOCKER
	case "yandex":
		return deploymentpb.Provider_PROVIDER_YANDEX
	default:
		return deploymentpb.Provider_PROVIDER_UNSPECIFIED
	}
}

// amountsForGroups sums cpu/ram_mb/disk_gb across every machine_groups
// entry — see this file's package doc for the exact per-kind formula. Pure
// (no I/O), so it and buildQuotaAmounts below are unit-tested directly
// without a database (see reserve_test.go).
func amountsForGroups(groups []*dslpb.MachineGroup) map[string]uint64 {
	var cpu, ramMB, diskGB uint64
	for _, g := range groups {
		count := uint64(g.GetCount())
		if count == 0 {
			continue
		}
		cpu += uint64(g.GetCpu()) * count
		ramMB += g.GetRamMb() * count
		var groupDiskGB uint64
		for _, d := range g.GetDisks() {
			groupDiskGB += d.GetSizeGb()
		}
		diskGB += groupDiskGB * count
	}
	return map[string]uint64{
		quotaKindCPU:  cpu,
		quotaKindRAM:  ramMB,
		quotaKindDisk: diskGB,
	}
}

// buildQuotaAmounts combines amountsForGroups with quotaDimensionsFor(
// provider) into the []QuotaAmount Store.Reserve consumes, converting each
// raw sum into the provider's native quota units and dropping zero-amount
// dimensions (a machine group with no disks contributes no disk_gb
// reservation row at all, mirroring the old aggregateRequests' skip-zero
// convention). Returns an error if provider has no quotaDimensionsFor
// mapping (Reserve enforcement is not supported for it yet).
func buildQuotaAmounts(provider deploymentpb.Provider, groups []*dslpb.MachineGroup) ([]QuotaAmount, error) {
	dims := quotaDimensionsFor(provider)
	if dims == nil {
		return nil, derrors.FailedPrecondition("QUOTA_PROVIDER_UNSUPPORTED", fmt.Sprintf("quota enforcement is not supported for provider %s", provider))
	}
	raw := amountsForGroups(groups)
	out := make([]QuotaAmount, 0, len(dims))
	for kind, dim := range dims {
		amount := dim.Convert(raw[kind])
		if amount == 0 {
			continue
		}
		out = append(out, QuotaAmount{QuotaName: dim.QuotaName, Units: dim.Units, Amount: amount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].QuotaName < out[j].QuotaName })
	return out, nil
}

// servicesForAmounts extracts the provider "service" prefix (the segment
// before the first '.') from every request's quota_name, for refreshScope's
// Services filter — e.g. ["compute"] for Yandex's compute.* quota ids, so a
// stale-snapshot refresh only lists that one service's quotas instead of
// every service Yandex exposes (see yandex.go's ListQuotas, which lists
// every known service when req.Services is empty).
func servicesForAmounts(requests []QuotaAmount) []string {
	seen := map[string]bool{}
	for _, r := range requests {
		if i := strings.IndexByte(r.QuotaName, '.'); i > 0 {
			seen[r.QuotaName[:i]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Reserve computes tenantID/runID's quota demand from plan's machine groups
// (see amountsForGroups/buildQuotaAmounts) and writes one quota_reservations
// row per resource kind, failing (no rows written) if any kind's amount
// would exceed that kind's live AvailableForRuns. Mirrors the pre-DSL-pivot
// Manager.Reserve's own snapshot-refresh-and-retry-once shape (git show
// efd9bffc^), but takes providerName + groups (dslpb.CompiledPlan's own
// shape) instead of an InfrastructurePlan + []*workflowpb.QuotaRequestRef —
// see this file's package doc for the full remodel.
//
// Called from RunRecipeWorkflow's infra stage BEFORE ProvisionActivity (see
// runrecipe.go's reserveQuotas) — a non-nil error here must fail the infra
// stage and skip provisioning entirely, never partially provision unmetered
// infrastructure.
func (m *Manager) Reserve(ctx context.Context, tenantID, runID, workflowID, providerName string, groups []*dslpb.MachineGroup) error {
	if tenantID == "" {
		return derrors.Invalid("tenant_id", "tenant_id is required")
	}
	if runID == "" {
		return derrors.Invalid("run_id", "run_id is required")
	}

	provider := providerFromDslName(providerName)
	requests, err := buildQuotaAmounts(provider, groups)
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		return nil
	}

	scope, settings, err := m.scopeForProvider(ctx, tenantID, provider, nil)
	if err != nil {
		return err
	}
	services := servicesForAmounts(requests)
	names := make([]string, 0, len(requests))
	for _, r := range requests {
		names = append(names, r.QuotaName)
	}
	sort.Strings(names)

	now := m.now()
	needsRefresh, err := m.store.ScopeNeedsRefresh(ctx, scope, names, now)
	if err != nil {
		return err
	}
	if needsRefresh {
		if err := m.refreshScope(ctx, scope, settings, services); err != nil {
			return err
		}
	}

	input := ReserveInput{
		TenantID:       tenantID,
		RunID:          runID,
		WorkflowID:     workflowID,
		Scope:          scope,
		Requests:       requests,
		ReservationTTL: m.cfg.ReservationTTL,
		Now:            m.now(),
	}
	if _, err := m.store.Reserve(ctx, input); err == nil {
		return nil
	} else if !errors.Is(err, ErrSnapshotMissing) && !errors.Is(err, ErrSnapshotStale) && !errors.Is(err, ErrInsufficient) {
		return err
	}
	// Snapshot was missing/stale (or the just-refreshed snapshot was
	// already this close to the limit) — refresh once from the live
	// provider and retry exactly once, mirroring the recovered Manager.
	// Reserve's own single-retry shape.
	if refreshErr := m.refreshScope(ctx, scope, settings, services); refreshErr != nil {
		return refreshErr
	}
	input.Now = m.now()
	if _, err := m.store.Reserve(ctx, input); err != nil {
		return mapReserveErr(err)
	}
	return nil
}

// Commit promotes tenantID/runID's RESERVED reservations to ALLOCATED —
// called once ExecuteCompiledPlanWorkflow succeeds (see runrecipe.go's
// commitQuotas). Run-id keyed, so the recovered Store.CommitRun needed no
// remodel; this wrapper just drops the deleted []*workflowpb.
// QuotaAllocationRef return value (that proto message no longer exists) in
// favor of a plain error, since no caller consumes the allocation list today.
func (m *Manager) Commit(ctx context.Context, tenantID, runID string) error {
	_, err := m.store.CommitRun(ctx, tenantID, runID, m.now())
	return err
}

// Release marks tenantID/runID's RESERVED-or-ALLOCATED reservations
// RELEASED — called unconditionally from RunRecipeWorkflow's teardown defer
// (see runrecipe.go's releaseQuotas), so a run always frees whatever it held
// regardless of how it ended. Safe to call even if Reserve was never called
// for this run (ReleaseRun affects zero rows and returns no error).
func (m *Manager) Release(ctx context.Context, tenantID, runID string) error {
	_, err := m.store.ReleaseRun(ctx, tenantID, runID, m.now())
	return err
}

// mapReserveErr turns Store.Reserve's sentinel errors into the same
// derrors.FailedPrecondition shape ListQuotas/RefreshQuotas already report
// provider errors as, so Reserve's caller-facing error is a stable,
// user-presentable message rather than a bare sentinel — recovered verbatim
// from the pre-DSL-pivot Manager (git show efd9bffc^).
func mapReserveErr(err error) error {
	if errors.Is(err, ErrSnapshotMissing) {
		return derrors.FailedPrecondition("QUOTA_SNAPSHOT_MISSING", "quota snapshot is missing").Wrap(err)
	}
	if errors.Is(err, ErrSnapshotStale) {
		return derrors.FailedPrecondition("QUOTA_SNAPSHOT_STALE", "quota snapshot is stale").Wrap(err)
	}
	if errors.Is(err, ErrInsufficient) {
		return derrors.FailedPrecondition("QUOTA_INSUFFICIENT", err.Error()).Wrap(err)
	}
	return err
}
