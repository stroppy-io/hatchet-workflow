import { CalendarClock, Loader2, Play, Rows3 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAuth } from "@/contexts/auth-context";
import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp, providerLabel } from "@/lib/format";
import { ListSuitesRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import type { Suite } from "@/lib/proto/cloud/v1/models/testing_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";
import { tenantPath } from "@/lib/routes";

function tenantIdMessage(value: string) {
  return { value };
}

function suiteTitle(suite: Suite) {
  return suite.name || formatId(suite.entity?.id?.value);
}

function schedulingLabel(suite: Suite) {
  const mode = suite.preset?.scheduling?.mode;
  if (mode?.case === "parallel") return `Parallel x${mode.value.maxParallel}`;
  if (mode?.case === "sequential") return "Sequential";
  return "Default scheduling";
}

function SuiteCard({
  canLaunch,
  launching,
  onLaunch,
  suite,
}: {
  canLaunch: boolean;
  launching: boolean;
  onLaunch: () => void;
  suite: Suite;
}) {
  const scheduled = Boolean(suite.cron?.expr);
  const tests = suite.preset?.tests ?? [];

  return (
    <article className="rounded-md border bg-card p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold">{suiteTitle(suite)}</h3>
          <p className="mt-1 line-clamp-2 min-h-10 text-sm text-muted-foreground">{suite.description || "No description."}</p>
        </div>
        <Badge variant={scheduled ? "default" : "secondary"}>{scheduled ? "Scheduled" : "Manual"}</Badge>
      </div>

      <div className="mt-4 grid gap-3 text-sm md:grid-cols-2">
        <div>
          <div className="text-xs text-muted-foreground">Provider</div>
          <div className="mt-1">{providerLabel(suite.preset?.provider)}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Tests</div>
          <div className="mt-1">{tests.length}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Scheduling</div>
          <div className="mt-1">{schedulingLabel(suite)}</div>
        </div>
        <div>
          <div className="text-xs text-muted-foreground">Next fire</div>
          <div className="mt-1">{formatTimestamp(suite.nextFireAt)}</div>
        </div>
      </div>

      {scheduled ? (
        <div className="mt-4 rounded-md border bg-muted/30 px-3 py-2 text-sm">
          <div className="flex items-center gap-2">
            <CalendarClock className="size-4 text-muted-foreground" />
            <span className="font-mono text-xs">{suite.cron?.expr}</span>
          </div>
          <div className="mt-1 text-xs text-muted-foreground">{suite.cron?.timezone || "UTC"}</div>
        </div>
      ) : null}

      <div className="mt-5 flex items-center justify-between gap-2">
        <span className="font-mono text-xs text-muted-foreground">{formatId(suite.entity?.id?.value)}</span>
        <Button disabled={!canLaunch || launching} onClick={onLaunch} size="sm" type="button">
          {launching ? <Loader2 className="animate-spin" /> : <Play />}
          Launch
        </Button>
      </div>
    </article>
  );
}

export function SuitesPage() {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const { account, roleForTenant } = useAuth();
  const [scheduleFilter, setScheduleFilter] = useState("all");
  const [search, setSearch] = useState("");
  const [launchingId, setLaunchingId] = useState<string | null>(null);
  const debouncedSearch = useDebouncedValue(search);
  const pagination = useCursorPagination(24);
  const resetPagination = pagination.reset;
  const canLaunch = account?.isAdmin || roleForTenant(tenantId) >= TenantMember_Role.ADMIN;
  const hasCron = scheduleFilter === "all" ? undefined : scheduleFilter === "scheduled";

  useEffect(() => {
    resetPagination();
  }, [debouncedSearch, scheduleFilter, tenantId, resetPagination]);

  const result = useListQuery(
    () =>
      api.suite.listSuites({
        tenantId: tenantIdMessage(tenantId),
        search: debouncedSearch || undefined,
        hasCron,
        sortField: ListSuitesRequest_SortField.CREATED_AT,
        order: SortOrder.DESC,
        page: pagination.page,
      }),
    [tenantId, debouncedSearch, hasCron, pagination.page],
  );

  const groups = useMemo(() => {
    const suites = result.data?.suites ?? [];
    return [
      { title: "Scheduled", icon: CalendarClock, items: suites.filter((suite) => suite.cron?.expr) },
      { title: "Manual", icon: Rows3, items: suites.filter((suite) => !suite.cron?.expr) },
    ];
  }, [result.data?.suites]);

  async function launch(suite: Suite) {
    const suiteId = suite.entity?.id?.value;
    if (!suiteId) return;

    setLaunchingId(suiteId);
    try {
      const run = await api.suite.launchSuiteRun({
        tenantId: tenantIdMessage(tenantId),
        suiteId: { value: suiteId },
      });
      navigate(tenantPath(tenantId, `/suite-runs/${run.entity?.id?.value ?? ""}`));
    } finally {
      setLaunchingId(null);
    }
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <header className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">Suites</h1>
          <p className="mt-1 text-sm text-muted-foreground">Reusable launch presets for one or more test presets.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Input className="w-64" onChange={(event) => setSearch(event.target.value)} placeholder="Search suites" value={search} />
          <Select onValueChange={setScheduleFilter} value={scheduleFilter}>
            <SelectTrigger className="w-40">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All suites</SelectItem>
              <SelectItem value="scheduled">Scheduled</SelectItem>
              <SelectItem value="manual">Manual</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </header>

      {result.error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{result.error}</div> : null}

      {result.loading ? (
        <div className="rounded-md border bg-card p-12 text-center text-sm text-muted-foreground">Loading suites...</div>
      ) : groups.every((group) => group.items.length === 0) ? (
        <div className="rounded-md border bg-card p-12 text-center text-sm text-muted-foreground">No suites found.</div>
      ) : (
        groups.map((group) => {
          const Icon = group.icon;
          return group.items.length ? (
            <section className="space-y-3" key={group.title}>
              <div className="flex items-center gap-2">
                <Icon className="size-4 text-muted-foreground" />
                <h2 className="text-sm font-semibold">{group.title}</h2>
                <Badge variant="secondary">{group.items.length}</Badge>
              </div>
              <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
                {group.items.map((suite) => {
                  const suiteId = suite.entity?.id?.value ?? "";
                  return (
                    <SuiteCard
                      canLaunch={Boolean(canLaunch)}
                      key={suiteId || suite.name}
                      launching={launchingId === suiteId}
                      onLaunch={() => launch(suite)}
                      suite={suite}
                    />
                  );
                })}
              </div>
            </section>
          ) : null;
        })
      )}

      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>Page {pagination.index + 1}</span>
        <div className="flex items-center gap-2">
          <Button disabled={!pagination.canPrevious || result.loading} onClick={pagination.previous} type="button" variant="outline">
            Previous
          </Button>
          <Button disabled={!result.data?.pageInfo?.nextToken || result.loading} onClick={() => pagination.next(result.data?.pageInfo)} type="button" variant="outline">
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}
