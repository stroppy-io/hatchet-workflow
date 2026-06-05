import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  Activity,
  ChevronDown,
  ChevronUp,
  ChevronsUpDown,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { useTenantSlug } from "@/lib/router";
import { getQuotaProvider } from "@/services/quotas";
import type { QuotaReservationView, QuotaView } from "@/lib/proto/cloud/v1/api/quota_pb";
import { Provider as ProviderEnum } from "@/lib/proto/cloud/v1/deployment/provider_pb";
import { Quota_ReservationStatus } from "@/lib/proto/cloud/v1/deployment/quota_pb";

type SortDirection = "asc" | "desc";
type QuotaSortField = "scope" | "quota" | "usage" | "observed" | "stale";
type RunSortField = "node" | "quota" | "status" | "amount" | "created" | "expires";
type FreshnessFilter = "all" | "fresh" | "stale";

interface SortState<T extends string> {
  field: T;
  direction: SortDirection;
}

export function Quotas() {
  const tenantSlug = useTenantSlug() ?? "";
  const [rows, setRows] = useState<QuotaView[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [quotaSearch, setQuotaSearch] = useState("");
  const [quotaProviderFilter, setQuotaProviderFilter] = useState("all");
  const [freshnessFilter, setFreshnessFilter] = useState<FreshnessFilter>("all");
  const [quotaSort, setQuotaSort] = useState<SortState<QuotaSortField>>({
    field: "scope",
    direction: "asc",
  });

  const [runId, setRunId] = useState("");
  const [runRows, setRunRows] = useState<QuotaReservationView[]>([]);
  const [runLoading, setRunLoading] = useState(false);
  const [runError, setRunError] = useState<string | null>(null);
  const [runSearch, setRunSearch] = useState("");
  const [runStatusFilter, setRunStatusFilter] = useState("all");
  const [runSort, setRunSort] = useState<SortState<RunSortField>>({
    field: "created",
    direction: "desc",
  });

  const load = useCallback(
    async (force = false) => {
      if (!tenantSlug) return;
      setLoading(true);
      setError(null);
      try {
        const data = await getQuotaProvider().listQuotas(tenantSlug, {
          refresh: force ? "force" : "refreshIfStale",
        });
        setRows(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [tenantSlug],
  );

  useEffect(() => {
    void load(false);
  }, [load]);

  const providerOptions = useMemo(() => {
    return Array.from(
      new Set(rows.map((row) => providerLabel(row.info?.provider)).filter((value) => value !== "—")),
    ).sort((a, b) => a.localeCompare(b));
  }, [rows]);

  const stats = useMemo(() => {
    const providers = new Set<string>();
    let stale = 0;
    let reserved = 0;
    for (const row of rows) {
      const provider = providerLabel(row.info?.provider);
      if (provider !== "—") providers.add(provider);
      if (row.stale) stale += 1;
      if (finite(row.reserved) > 0) reserved += 1;
    }
    return { rows: rows.length, providers: providers.size, stale, reserved };
  }, [rows]);

  const visibleRows = useMemo(() => {
    const query = normalizeSearch(quotaSearch);
    return rows
      .filter((row) => {
        const provider = providerLabel(row.info?.provider);
        if (quotaProviderFilter !== "all" && provider !== quotaProviderFilter) return false;
        if (freshnessFilter === "fresh" && row.stale) return false;
        if (freshnessFilter === "stale" && !row.stale) return false;
        if (!query) return true;
        return includesQuery(
          [
            provider,
            row.service,
            row.info?.name,
            row.info?.units,
            row.resourceType,
            row.resourceId,
            visibleService(row.service, provider),
          ],
          query,
        );
      })
      .sort((left, right) => sortQuotaRows(left, right, quotaSort));
  }, [freshnessFilter, quotaProviderFilter, quotaSearch, quotaSort, rows]);

  const quotaFiltersActive =
    quotaSearch.trim().length > 0 || quotaProviderFilter !== "all" || freshnessFilter !== "all";

  const loadRunUsage = useCallback(async () => {
    if (!tenantSlug || !runId.trim()) return;
    setRunLoading(true);
    setRunError(null);
    try {
      const data = await getQuotaProvider().getRunQuotaUsage(tenantSlug, runId.trim());
      setRunRows(data);
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err));
    } finally {
      setRunLoading(false);
    }
  }, [runId, tenantSlug]);

  const runStatusOptions = useMemo(() => {
    return Array.from(new Set(runRows.map((row) => reservationStatus(row.status)))).sort((a, b) =>
      a.localeCompare(b),
    );
  }, [runRows]);

  const visibleRunRows = useMemo(() => {
    const query = normalizeSearch(runSearch);
    return runRows
      .filter((row) => {
        const status = reservationStatus(row.status);
        if (runStatusFilter !== "all" && status !== runStatusFilter) return false;
        if (!query) return true;
        return includesQuery(
          [
            row.nodeId,
            row.info?.name,
            row.info?.units,
            row.resourceType,
            row.resourceId,
            row.service,
            status,
          ],
          query,
        );
      })
      .sort((left, right) => sortRunRows(left, right, runSort));
  }, [runRows, runSearch, runSort, runStatusFilter]);

  const runFiltersActive = runSearch.trim().length > 0 || runStatusFilter !== "all";

  return (
    <div className="min-h-full bg-background text-foreground">
      <div className="mx-auto flex max-w-[1600px] flex-col gap-4 p-4 md:p-6">
        <div className="flex flex-col gap-3 border-b border-border pb-4 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Activity className="h-5 w-5 text-primary" />
              <h1>Quotas</h1>
            </div>
            <div className="mt-1 text-xs text-muted-foreground">
              {visibleRows.length} of {rows.length} rows · {stats.stale} stale
            </div>
          </div>
          <Button variant="outline" size="sm" onClick={() => void load(true)} disabled={loading}>
            <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
            Refresh
          </Button>
        </div>

        <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
          <Stat label="Rows" value={formatInteger(stats.rows)} />
          <Stat label="Providers" value={formatInteger(stats.providers)} />
          <Stat label="Stale" value={formatInteger(stats.stale)} />
          <Stat label="Reserved rows" value={formatInteger(stats.reserved)} />
        </div>

        {error && (
          <div className="border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="overflow-hidden border border-border bg-background">
          <TableToolbar>
            <SearchInput
              value={quotaSearch}
              onChange={setQuotaSearch}
              placeholder="Filter provider, quota, resource..."
            />
            <div className="flex flex-wrap items-center gap-2">
              <FilterSelect
                label="Provider"
                value={quotaProviderFilter}
                onChange={setQuotaProviderFilter}
                options={[
                  { value: "all", label: "All providers" },
                  ...providerOptions.map((provider) => ({ value: provider, label: provider })),
                ]}
              />
              <FilterSelect
                label="State"
                value={freshnessFilter}
                onChange={(value) => setFreshnessFilter(value as FreshnessFilter)}
                options={[
                  { value: "all", label: "All states" },
                  { value: "fresh", label: "Fresh" },
                  { value: "stale", label: "Stale" },
                ]}
              />
              {quotaFiltersActive && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setQuotaSearch("");
                    setQuotaProviderFilter("all");
                    setFreshnessFilter("all");
                  }}
                >
                  <X className="h-4 w-4" />
                  Clear
                </Button>
              )}
            </div>
          </TableToolbar>

          <div className="h-[460px] overflow-auto">
            <table className="w-full min-w-[1060px] border-collapse text-sm">
              <thead className="sticky top-0 z-10 bg-muted/95 text-xs uppercase text-muted-foreground backdrop-blur">
                <tr className="border-b border-border">
                  <SortTh
                    field="scope"
                    sort={quotaSort}
                    onSort={(field) => setQuotaSort((current) => nextSort(current, field))}
                  >
                    Scope
                  </SortTh>
                  <SortTh
                    field="quota"
                    sort={quotaSort}
                    onSort={(field) => setQuotaSort((current) => nextSort(current, field))}
                  >
                    Quota
                  </SortTh>
                  <SortTh
                    field="usage"
                    sort={quotaSort}
                    onSort={(field) => setQuotaSort((current) => nextSort(current, field))}
                  >
                    Usage
                  </SortTh>
                  <SortTh
                    field="observed"
                    sort={quotaSort}
                    onSort={(field) => setQuotaSort((current) => nextSort(current, field))}
                  >
                    Observed
                  </SortTh>
                  <SortTh
                    field="stale"
                    sort={quotaSort}
                    onSort={(field) => setQuotaSort((current) => nextSort(current, field))}
                  >
                    Stale after
                  </SortTh>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/70">
                {visibleRows.map((row) => (
                  <tr
                    key={`${row.info?.provider ?? 0}:${row.resourceId}:${row.info?.name ?? ""}`}
                    className={cn("transition-colors hover:bg-muted/30", row.stale && "bg-warning/5")}
                  >
                    <Td>
                      <ScopeCell row={row} />
                    </Td>
                    <Td>
                      <QuotaName row={row} />
                    </Td>
                    <Td className="min-w-[320px]">
                      <QuotaUsage row={row} />
                    </Td>
                    <Td>{formatTimestamp(row.observedAt)}</Td>
                    <Td>
                      <span className={cn(row.stale && "font-medium text-warning")}>
                        {formatTimestamp(row.staleAfter)}
                      </span>
                    </Td>
                  </tr>
                ))}
                {!loading && visibleRows.length === 0 && (
                  <EmptyRow cols={5} text={rows.length === 0 ? "No quota snapshots." : "No quotas matching filters."} />
                )}
                {loading && rows.length === 0 && <EmptyRow cols={5} text="Loading quotas..." />}
              </tbody>
            </table>
          </div>
        </div>

        <div className="mt-2 border-t border-border pt-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
            <div>
              <h2 className="text-base font-semibold">Run quota usage</h2>
              <div className="mt-1 text-xs text-muted-foreground">
                {visibleRunRows.length} of {runRows.length} ledger rows
              </div>
            </div>
            <div className="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
              <Input
                value={runId}
                onChange={(event) => setRunId(event.target.value)}
                placeholder="Run ID"
                className="sm:w-[320px]"
              />
              <Button
                size="sm"
                variant="outline"
                onClick={() => void loadRunUsage()}
                disabled={runLoading || !runId.trim()}
              >
                <RefreshCw className={cn("h-4 w-4", runLoading && "animate-spin")} />
                Load
              </Button>
            </div>
          </div>

          {runError && (
            <div className="mt-3 border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {runError}
            </div>
          )}

          <div className="mt-3 overflow-hidden border border-border bg-background">
            <TableToolbar>
              <SearchInput
                value={runSearch}
                onChange={setRunSearch}
                placeholder="Filter node, quota, resource..."
              />
              <div className="flex flex-wrap items-center gap-2">
                <FilterSelect
                  label="Status"
                  value={runStatusFilter}
                  onChange={setRunStatusFilter}
                  options={[
                    { value: "all", label: "All statuses" },
                    ...runStatusOptions.map((status) => ({ value: status, label: status })),
                  ]}
                />
                {runFiltersActive && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setRunSearch("");
                      setRunStatusFilter("all");
                    }}
                  >
                    <X className="h-4 w-4" />
                    Clear
                  </Button>
                )}
              </div>
            </TableToolbar>

            <div className="h-[300px] overflow-auto">
              <table className="w-full min-w-[920px] border-collapse text-sm">
                <thead className="sticky top-0 z-10 bg-muted/95 text-xs uppercase text-muted-foreground backdrop-blur">
                  <tr className="border-b border-border">
                    <SortTh
                      field="node"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                    >
                      Node
                    </SortTh>
                    <SortTh
                      field="quota"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                    >
                      Quota
                    </SortTh>
                    <SortTh
                      field="status"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                    >
                      Status
                    </SortTh>
                    <SortTh
                      field="amount"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                      align="right"
                    >
                      Amount
                    </SortTh>
                    <SortTh
                      field="created"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                    >
                      Created
                    </SortTh>
                    <SortTh
                      field="expires"
                      sort={runSort}
                      onSort={(field) => setRunSort((current) => nextSort(current, field))}
                    >
                      Expires
                    </SortTh>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/70">
                  {visibleRunRows.map((row) => (
                    <tr key={row.id} className="transition-colors hover:bg-muted/30">
                      <Td>
                        <span className="font-mono text-xs text-foreground">{row.nodeId || "—"}</span>
                      </Td>
                      <Td>
                        <QuotaReservationName row={row} />
                      </Td>
                      <Td>
                        <Badge variant={reservationStatusVariant(row.status)}>
                          {reservationStatus(row.status)}
                        </Badge>
                      </Td>
                      <Td className="text-right tabular-nums">
                        {formatQuantity(Number(row.amount), row.info?.units)}
                      </Td>
                      <Td>{formatTimestamp(row.createdAt)}</Td>
                      <Td>{formatTimestamp(row.expiresAt)}</Td>
                    </tr>
                  ))}
                  {!runLoading && visibleRunRows.length === 0 && (
                    <EmptyRow
                      cols={6}
                      text={runRows.length === 0 ? "No run quota rows." : "No run quotas matching filters."}
                    />
                  )}
                  {runLoading && runRows.length === 0 && <EmptyRow cols={6} text="Loading run quotas..." />}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="border border-border bg-muted/20 px-3 py-2">
      <div className="text-[11px] uppercase text-muted-foreground">{label}</div>
      <div className="mt-1 text-lg font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function TableToolbar({ children }: { children: ReactNode }) {
  return (
    <div
      className="flex flex-col gap-2 border-b border-border bg-muted/10 p-3 lg:flex-row lg:items-center lg:justify-between"
    >
      {children}
    </div>
  );
}

function SearchInput({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <div className="relative w-full lg:max-w-[420px]">
      <Search
        className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
        className="pl-8"
      />
    </div>
  );
}

function FilterSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex items-center gap-2 text-xs text-muted-foreground">
      <span className="uppercase">{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 min-w-[150px] border border-border bg-background px-2 text-sm text-foreground outline-none transition-colors hover:border-primary/60 focus:border-primary"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function SortTh<T extends string>({
  children,
  field,
  sort,
  onSort,
  align = "left",
}: {
  children: ReactNode;
  field: T;
  sort: SortState<T>;
  onSort: (field: T) => void;
  align?: "left" | "right";
}) {
  const active = sort.field === field;
  const Icon = !active ? ChevronsUpDown : sort.direction === "desc" ? ChevronDown : ChevronUp;
  return (
    <th className="border-r border-border/60 px-3 py-2 text-left font-medium last:border-r-0">
      <button
        type="button"
        onClick={() => onSort(field)}
        className={cn(
          "flex w-full items-center gap-1.5 whitespace-nowrap",
          align === "right" && "justify-end",
          active ? "text-primary" : "text-muted-foreground hover:text-foreground",
        )}
      >
        <span>{children}</span>
        <Icon className="h-3.5 w-3.5" />
      </button>
    </th>
  );
}

function Td({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <td className={cn("border-r border-border/40 px-3 py-2 align-middle last:border-r-0", className)}>
      {children}
    </td>
  );
}

function EmptyRow({ cols, text }: { cols: number; text: string }) {
  return (
    <tr>
      <td colSpan={cols} className="px-4 py-10 text-center text-sm text-muted-foreground">
        {text}
      </td>
    </tr>
  );
}

function ScopeCell({ row }: { row: QuotaView }) {
  const provider = providerLabel(row.info?.provider);
  const service = visibleService(row.service, provider);
  return (
    <div className="flex min-w-[180px] flex-col gap-1">
      <div className="flex flex-wrap items-center gap-1.5">
        <ProviderBadge provider={provider} />
        {row.stale && (
          <Badge variant="warning" className="uppercase">
            stale
          </Badge>
        )}
      </div>
      {service && <div className="text-xs text-foreground">{service}</div>}
      <div className="text-[11px] text-muted-foreground">
        {row.resourceType || "—"} · {row.resourceId || "—"}
      </div>
    </div>
  );
}

function ProviderBadge({ provider }: { provider: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center border px-2 py-0.5 text-xs font-medium",
        provider === "Yandex" && "border-sky-500/30 bg-sky-500/15 text-sky-300",
        provider === "Docker" && "border-cyan-500/30 bg-cyan-500/15 text-cyan-300",
        provider === "—" && "border-border bg-muted/30 text-muted-foreground",
      )}
    >
      {provider}
    </span>
  );
}

function QuotaName({ row }: { row: QuotaView }) {
  const unit = unitLabel(row.info?.units);
  return (
    <div className="min-w-[210px]">
      <div className="font-mono text-xs text-foreground">{row.info?.name || "—"}</div>
      {unit && <div className="mt-1 text-[11px] text-muted-foreground">{unit}</div>}
    </div>
  );
}

function QuotaReservationName({ row }: { row: QuotaReservationView }) {
  const provider = providerLabel(row.info?.provider);
  const service = visibleService(row.service, provider);
  return (
    <div className="min-w-[260px]">
      <div className="font-mono text-xs text-foreground">{row.info?.name || "—"}</div>
      <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
        <ProviderBadge provider={provider} />
        {service && <span>{service}</span>}
        <span>
          {row.resourceType || "—"} · {row.resourceId || "—"}
        </span>
      </div>
    </div>
  );
}

function QuotaUsage({ row }: { row: QuotaView }) {
  const ratio = usageRatio(row);
  const tone = usageTone(ratio);
  return (
    <div className="space-y-2">
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-xs text-muted-foreground">Used / limit</div>
          <div className="font-mono text-sm tabular-nums">
            {formatQuantity(row.providerUsed, row.info?.units)} / {formatQuantity(row.limit, row.info?.units)}
          </div>
        </div>
        <div className={cn("font-mono text-xs tabular-nums", tone.text)}>
          {ratio === null ? "—" : `${formatNumber(ratio * 100)}%`}
        </div>
      </div>
      <div className="h-2 overflow-hidden bg-muted">
        <div className={cn("h-full", tone.bar)} style={{ width: `${Math.round((ratio ?? 0) * 100)}%` }} />
      </div>
      <div className="grid grid-cols-2 gap-2 text-[11px] md:grid-cols-3">
        <MiniMetric label="Reserved" value={formatQuantity(row.reserved, row.info?.units)} tone="warning" />
        <MiniMetric label="Run free" value={formatQuantity(row.availableForRuns, row.info?.units)} tone="success" />
        <MiniMetric label="Provider free" value={formatQuantity(row.providerAvailable, row.info?.units)} tone="muted" />
      </div>
    </div>
  );
}

function MiniMetric({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "success" | "warning" | "muted";
}) {
  return (
    <div className="border border-border/50 bg-muted/10 px-2 py-1">
      <div className="text-muted-foreground">{label}</div>
      <div
        className={cn(
          "mt-0.5 font-mono tabular-nums",
          tone === "success" && "text-success",
          tone === "warning" && "text-warning",
          tone === "muted" && "text-foreground",
        )}
      >
        {value}
      </div>
    </div>
  );
}

function nextSort<T extends string>(current: SortState<T>, field: T): SortState<T> {
  if (current.field !== field) return { field, direction: "asc" };
  return { field, direction: current.direction === "asc" ? "desc" : "asc" };
}

function sortQuotaRows(left: QuotaView, right: QuotaView, sort: SortState<QuotaSortField>): number {
  const multiplier = sort.direction === "asc" ? 1 : -1;
  let result = 0;
  switch (sort.field) {
    case "scope":
      result =
        compareText(providerLabel(left.info?.provider), providerLabel(right.info?.provider)) ||
        compareText(
          visibleService(left.service, providerLabel(left.info?.provider)),
          visibleService(right.service, providerLabel(right.info?.provider)),
        ) ||
        compareText(left.resourceId, right.resourceId);
      break;
    case "quota":
      result = compareText(left.info?.name, right.info?.name) || compareText(left.resourceType, right.resourceType);
      break;
    case "usage":
      result = compareNumber(usageRatio(left) ?? -1, usageRatio(right) ?? -1);
      break;
    case "observed":
      result = compareNumber(timestampMillis(left.observedAt), timestampMillis(right.observedAt));
      break;
    case "stale":
      result = compareNumber(timestampMillis(left.staleAfter), timestampMillis(right.staleAfter));
      break;
  }
  return result * multiplier;
}

function sortRunRows(
  left: QuotaReservationView,
  right: QuotaReservationView,
  sort: SortState<RunSortField>,
): number {
  const multiplier = sort.direction === "asc" ? 1 : -1;
  let result = 0;
  switch (sort.field) {
    case "node":
      result = compareText(left.nodeId, right.nodeId);
      break;
    case "quota":
      result = compareText(left.info?.name, right.info?.name) || compareText(left.resourceId, right.resourceId);
      break;
    case "status":
      result = compareText(reservationStatus(left.status), reservationStatus(right.status));
      break;
    case "amount":
      result = compareNumber(Number(left.amount), Number(right.amount));
      break;
    case "created":
      result = compareNumber(timestampMillis(left.createdAt), timestampMillis(right.createdAt));
      break;
    case "expires":
      result = compareNumber(timestampMillis(left.expiresAt), timestampMillis(right.expiresAt));
      break;
  }
  return result * multiplier;
}

function compareText(left?: string, right?: string): number {
  return (left || "—").localeCompare(right || "—", undefined, { numeric: true, sensitivity: "base" });
}

function compareNumber(left: number, right: number): number {
  return left === right ? 0 : left < right ? -1 : 1;
}

function normalizeSearch(value: string): string {
  return value.trim().toLowerCase();
}

function includesQuery(values: Array<string | undefined>, query: string): boolean {
  return values.some((value) => (value || "").toLowerCase().includes(query));
}

function visibleService(service: string | undefined, provider: string): string {
  const value = (service || "").trim();
  if (!value) return "";
  if (value.toLowerCase() === provider.toLowerCase()) return "";
  return value;
}

function finite(value: number): number {
  return Number.isFinite(value) ? value : 0;
}

function usageRatio(row: QuotaView): number | null {
  const limit = finite(row.limit);
  if (limit <= 0) return null;
  return clamp(finite(row.providerUsed) / limit, 0, 1);
}

function usageTone(ratio: number | null): { text: string; bar: string } {
  if (ratio === null) return { text: "text-muted-foreground", bar: "bg-muted-foreground/40" };
  if (ratio >= 0.85) return { text: "text-destructive", bar: "bg-destructive" };
  if (ratio >= 0.65) return { text: "text-warning", bar: "bg-warning" };
  return { text: "text-success", bar: "bg-success" };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function formatInteger(value: number): string {
  return new Intl.NumberFormat().format(value);
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(finite(value));
}

function formatQuantity(value: number, units?: string): string {
  const unit = unitLabel(units);
  return unit ? `${formatNumber(value)} ${unit}` : formatNumber(value);
}

function unitLabel(units?: string): string {
  const unit = (units || "").trim();
  if (!unit || unit.toLowerCase() === "count") return "";
  return unit;
}

function formatTimestamp(ts?: Timestamp): string {
  const millis = timestampMillis(ts);
  if (millis <= 0) return "—";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(millis));
}

function timestampMillis(ts?: Timestamp): number {
  if (!ts) return 0;
  const millis = Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000);
  return Number.isFinite(millis) ? millis : 0;
}

function providerLabel(provider?: ProviderEnum): string {
  switch (provider) {
    case ProviderEnum.DOCKER:
      return "Docker";
    case ProviderEnum.YANDEX:
      return "Yandex";
    default:
      return "—";
  }
}

function reservationStatusVariant(
  status: Quota_ReservationStatus,
): "default" | "success" | "destructive" | "warning" | "pending" {
  switch (status) {
    case Quota_ReservationStatus.ALLOCATED:
      return "success";
    case Quota_ReservationStatus.RESERVED:
      return "default";
    case Quota_ReservationStatus.FAILED:
      return "destructive";
    case Quota_ReservationStatus.EXPIRED:
      return "warning";
    case Quota_ReservationStatus.RELEASED:
      return "pending";
    default:
      return "pending";
  }
}

function reservationStatus(status: Quota_ReservationStatus): string {
  switch (status) {
    case Quota_ReservationStatus.RESERVED:
      return "Reserved";
    case Quota_ReservationStatus.ALLOCATED:
      return "Allocated";
    case Quota_ReservationStatus.RELEASED:
      return "Released";
    case Quota_ReservationStatus.EXPIRED:
      return "Expired";
    case Quota_ReservationStatus.FAILED:
      return "Failed";
    default:
      return "Unknown";
  }
}
