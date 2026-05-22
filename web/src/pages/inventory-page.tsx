import { Loader2, RefreshCw, RotateCcw } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth } from "@/contexts/auth-context";
import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp, providerLabel } from "@/lib/format";
import { Provider, QuotaResource, type QuotaInventory } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { ListNetworkAllocationsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/inventory_pb.ts";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

function tenantIdMessage(value: string) {
  return { value };
}

function quotaResourceLabel(resource: QuotaResource) {
  switch (resource) {
    case QuotaResource.CORES:
      return "CPU cores";
    case QuotaResource.MEMORY_GB:
      return "Memory, GB";
    case QuotaResource.SSD_GB:
      return "SSD, GB";
    case QuotaResource.HDD_GB:
      return "HDD, GB";
    case QuotaResource.INSTANCES:
      return "Instances";
    case QuotaResource.EXTERNAL_IPS:
      return "External IPs";
    case QuotaResource.NETWORKS:
      return "Networks";
    case QuotaResource.SUBNETS:
      return "Subnets";
    default:
      return "Resource";
  }
}

function formatBigint(value: bigint) {
  return new Intl.NumberFormat().format(Number(value));
}

function usagePercent(used: bigint, limit: bigint) {
  if (limit <= 0n) return 0;
  return Math.min(100, Number((used * 100n) / limit));
}

export function InventoryPage() {
  const tenantId = useTenantId();
  const { account, roleForTenant } = useAuth();
  const [provider, setProvider] = useState(String(Provider.YANDEX));
  const [leased, setLeased] = useState("all");
  const [search, setSearch] = useState("");
  const [quota, setQuota] = useState<QuotaInventory | null>(null);
  const [quotaError, setQuotaError] = useState<string | null>(null);
  const [quotaLoading, setQuotaLoading] = useState(true);
  const [reconciling, setReconciling] = useState(false);
  const [reconcileSummary, setReconcileSummary] = useState<string | null>(null);
  const debouncedSearch = useDebouncedValue(search);
  const pagination = useCursorPagination();
  const resetPagination = pagination.reset;
  const selectedProvider = Number(provider) as Provider;
  const canReconcile = account?.isAdmin || roleForTenant(tenantId) >= TenantMember_Role.OWNER;

  const loadQuotas = useCallback(
    async (live: boolean) => {
      setQuotaLoading(true);
      setQuotaError(null);
      try {
        const inventory = await api.inventory.fetchQuotas({
          tenantId: tenantIdMessage(tenantId),
          provider: selectedProvider,
          live,
        });
        setQuota(inventory);
      } catch (error) {
        setQuotaError(error instanceof Error ? error.message : "Failed to load quotas");
      } finally {
        setQuotaLoading(false);
      }
    },
    [selectedProvider, tenantId],
  );

  useEffect(() => {
    resetPagination();
  }, [provider, leased, tenantId, resetPagination]);

  useEffect(() => {
    void loadQuotas(false);
  }, [loadQuotas]);

  const allocations = useListQuery(
    () =>
      api.inventory.listNetworkAllocations({
        tenantId: tenantIdMessage(tenantId),
        provider: selectedProvider,
        search: debouncedSearch || undefined,
        leased: leased === "all" ? undefined : leased === "true",
        sortField: ListNetworkAllocationsRequest_SortField.CREATED_AT,
        order: SortOrder.DESC,
        page: pagination.page,
      }),
    [tenantId, selectedProvider, debouncedSearch, leased, pagination.page],
  );

  const quotaRows = useMemo(() => quota?.quotas ?? [], [quota]);

  async function reconcile() {
    setReconciling(true);
    setReconcileSummary(null);
    setQuotaError(null);
    try {
      const response = await api.inventory.reconcile({
        tenantId: tenantIdMessage(tenantId),
        provider: selectedProvider,
      });
      setReconcileSummary(
        `${response.quotasRefreshed} quotas refreshed, ${response.allocationsReconciled} allocations reconciled, ${response.orphansReleased} orphans released`,
      );
      await loadQuotas(false);
      pagination.reset();
    } catch (error) {
      setQuotaError(error instanceof Error ? error.message : "Failed to reconcile inventory");
    } finally {
      setReconciling(false);
    }
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <header className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Inventory</h1>
          <p className="mt-1 text-sm text-muted-foreground">Provider quotas and reserved network allocations for this tenant.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select onValueChange={setProvider} value={provider}>
            <SelectTrigger className="w-44">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={String(Provider.YANDEX)}>Yandex Cloud</SelectItem>
              <SelectItem value={String(Provider.DOCKER)}>Docker</SelectItem>
            </SelectContent>
          </Select>
          <Button onClick={() => loadQuotas(true)} type="button" variant="outline">
            {quotaLoading ? <Loader2 className="animate-spin" /> : <RefreshCw />}
            Refresh live
          </Button>
          <Button disabled={!canReconcile || reconciling} onClick={reconcile} type="button" variant="outline">
            {reconciling ? <Loader2 className="animate-spin" /> : <RotateCcw />}
            Reconcile
          </Button>
        </div>
      </header>

      {quotaError ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{quotaError}</div> : null}
      {reconcileSummary ? <div className="rounded-md border bg-muted px-3 py-2 text-sm">{reconcileSummary}</div> : null}

      <section className="rounded-md border bg-card">
        <div className="flex items-center justify-between border-b px-5 py-4">
          <div>
            <h2 className="text-sm font-semibold">Quotas</h2>
            <p className="mt-1 text-sm text-muted-foreground">
              {quota?.fetchedAt ? `Fetched ${formatTimestamp(quota.fetchedAt)}` : `Cached snapshot for ${providerLabel(selectedProvider)}`}
            </p>
          </div>
          {quotaLoading ? <Loader2 className="size-4 animate-spin text-muted-foreground" /> : null}
        </div>
        <div className="grid gap-3 p-5 md:grid-cols-2 xl:grid-cols-4">
          {quotaRows.length ? (
            quotaRows.map((row) => {
              const percent = usagePercent(row.used, row.limit);
              return (
                <div className="rounded-md border p-4" key={`${row.provider}-${row.resource}-${row.providerQuotaId}`}>
                  <div className="text-sm font-medium">{quotaResourceLabel(row.resource)}</div>
                  <div className="mt-3 flex items-end justify-between gap-3">
                    <div className="text-2xl font-semibold">{formatBigint(row.available)}</div>
                    <div className="text-xs text-muted-foreground">
                      {formatBigint(row.used)} / {formatBigint(row.limit)}
                    </div>
                  </div>
                  <div className="mt-3 h-2 overflow-hidden rounded-full bg-muted">
                    <div className="h-full bg-primary" style={{ width: `${percent}%` }} />
                  </div>
                </div>
              );
            })
          ) : (
            <div className="col-span-full rounded-md border border-dashed p-8 text-center text-sm text-muted-foreground">
              {quotaLoading ? "Loading quotas..." : "No quotas returned for this provider."}
            </div>
          )}
        </div>
      </section>

      <section className="rounded-md border bg-card">
        <div className="flex flex-col gap-3 border-b px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h2 className="text-sm font-semibold">Network allocations</h2>
            <p className="mt-1 text-sm text-muted-foreground">CIDR reservations and active leases.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Input className="w-56" onChange={(event) => setSearch(event.target.value)} placeholder="Search CIDR" value={search} />
            <Select onValueChange={setLeased} value={leased}>
              <SelectTrigger className="w-40">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All leases</SelectItem>
                <SelectItem value="true">Active</SelectItem>
                <SelectItem value="false">Free/expired</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>CIDR</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Zone</TableHead>
                <TableHead>DAG</TableHead>
                <TableHead>Lease expires</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allocations.loading ? (
                <TableRow>
                  <TableCell className="h-24 text-center text-sm text-muted-foreground" colSpan={5}>
                    Loading allocations...
                  </TableCell>
                </TableRow>
              ) : allocations.error ? (
                <TableRow>
                  <TableCell className="h-24 text-center text-sm text-destructive" colSpan={5}>
                    {allocations.error}
                  </TableCell>
                </TableRow>
              ) : allocations.data?.networkAllocations.length ? (
                allocations.data.networkAllocations.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="font-mono text-xs">{row.cidr?.value ?? "—"}</TableCell>
                    <TableCell>
                      <Badge variant="secondary">{providerLabel(row.provider)}</Badge>
                    </TableCell>
                    <TableCell>{row.zone || "—"}</TableCell>
                    <TableCell className="font-mono text-xs">{formatId(row.dagId?.value)}</TableCell>
                    <TableCell>{formatTimestamp(row.leaseExpiresAt)}</TableCell>
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell className="h-24 text-center text-sm text-muted-foreground" colSpan={5}>
                    No network allocations found.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
        <div className="flex items-center justify-between border-t px-5 py-3 text-sm text-muted-foreground">
          <span>Page {pagination.index + 1}</span>
          <div className="flex items-center gap-2">
            <Button disabled={!pagination.canPrevious || allocations.loading} onClick={pagination.previous} type="button" variant="outline">
              Previous
            </Button>
            <Button disabled={!allocations.data?.pageInfo?.nextToken || allocations.loading} onClick={() => pagination.next(allocations.data?.pageInfo)} type="button" variant="outline">
              Next
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}
