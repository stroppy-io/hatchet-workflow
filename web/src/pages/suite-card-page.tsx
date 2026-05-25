import { CalendarClock, Loader2, Play, Plus, Rows3 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { create } from "@bufbuild/protobuf";

import { FormDialog } from "@/components/form-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAuth } from "@/contexts/auth-context";
import { useAction } from "@/hooks/use-action";
import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatId, formatTimestamp, providerLabel } from "@/lib/format";
import { presetTitle } from "@/lib/preset-summary";
import { notifySuccess } from "@/lib/toast";
import { ListSuitesRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/suite_pb.ts";
import { Provider } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { SuitePresetSchema } from "@/lib/proto/cloud/v1/domain/suite_pb.ts";
import type { TestPreset } from "@/lib/proto/cloud/v1/domain/test_pb.ts";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import { Preset_Kind } from "@/lib/proto/cloud/v1/models/preset_pb.ts";
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
  onOpen,
  suite,
}: {
  canLaunch: boolean;
  launching: boolean;
  onLaunch: () => void;
  onOpen: () => void;
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
        <button type="button" onClick={onOpen} className="font-mono text-xs text-muted-foreground hover:text-foreground">
          {formatId(suite.entity?.id?.value)}
        </button>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" type="button" onClick={onOpen}>
            Open
          </Button>
          <Button disabled={!canLaunch || launching} onClick={onLaunch} size="sm" type="button">
            {launching ? <Loader2 className="animate-spin" /> : <Play />}
            Launch
          </Button>
        </div>
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
  const [createOpen, setCreateOpen] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
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
    [tenantId, debouncedSearch, hasCron, pagination.page, reloadKey],
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
          {canLaunch ? (
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus />
              New suite
            </Button>
          ) : null}
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
                      onOpen={() => navigate(tenantPath(tenantId, `/suites/${suiteId}`))}
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

      <CreateSuiteDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        tenantId={tenantId}
        onCreated={() => {
          setCreateOpen(false);
          setReloadKey((key) => key + 1);
        }}
      />
    </div>
  );
}

function CreateSuiteDialog({ open, onOpenChange, tenantId, onCreated }: { open: boolean; onOpenChange: (open: boolean) => void; tenantId: string; onCreated: () => void }) {
  const [name, setName] = useState("");
  const [provider, setProvider] = useState(String(Provider.DOCKER));
  const [mode, setMode] = useState("sequential");
  const [parallelN, setParallelN] = useState(2);
  const [selected, setSelected] = useState<string[]>([]);

  const presetsResult = useListQuery(
    () => api.preset.listPresets({ tenantId: tenantIdMessage(tenantId), kinds: [Preset_Kind.TEST], page: { size: 100 } }),
    [tenantId, open],
  );
  const presets = useMemo(() => presetsResult.data?.presets ?? [], [presetsResult.data]);

  const createSuite = useAction(() => {
    const tests = presets
      .filter((preset) => selected.includes(preset.entity?.id?.value ?? ""))
      .map((preset) => (preset.preset.case === "testPreset" ? preset.preset.value : undefined))
      .filter((test): test is TestPreset => Boolean(test));
    const scheduling =
      mode === "parallel"
        ? { mode: { case: "parallel" as const, value: { maxParallel: parallelN } } }
        : { mode: { case: "sequential" as const, value: true } };
    const preset = create(SuitePresetSchema, { provider: Number(provider) as Provider, scheduling, tests });
    return api.suite.createSuite({ tenantId: tenantIdMessage(tenantId), preset, name });
  });

  function toggle(id: string) {
    setSelected((current) => (current.includes(id) ? current.filter((x) => x !== id) : [...current, id]));
  }

  async function submit() {
    const response = await createSuite.run();
    if (!response) return;
    notifySuccess("Suite created");
    setName("");
    setSelected([]);
    onCreated();
  }

  return (
    <FormDialog open={open} onOpenChange={onOpenChange} title="New suite" submitLabel="Create" onSubmit={submit} loading={createSuite.loading} error={createSuite.error} submitDisabled={!name.trim() || selected.length === 0}>
      <div className="space-y-2">
        <Label>Name</Label>
        <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label>Provider</Label>
          <Select value={provider} onValueChange={setProvider}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={String(Provider.DOCKER)}>Docker</SelectItem>
              <SelectItem value={String(Provider.YANDEX)}>Yandex Cloud</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Scheduling</Label>
          <Select value={mode} onValueChange={setMode}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="sequential">Sequential</SelectItem>
              <SelectItem value="parallel">Parallel</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {mode === "parallel" ? (
        <div className="space-y-2">
          <Label>Max parallel</Label>
          <Input type="number" min={1} value={parallelN} onChange={(event) => setParallelN(Number(event.target.value) || 1)} />
        </div>
      ) : null}
      <div className="space-y-2">
        <Label>Test presets</Label>
        <div className="max-h-48 space-y-1 overflow-y-auto rounded-md border p-2">
          {presets.length === 0 ? (
            <p className="p-2 text-xs text-muted-foreground">{presetsResult.loading ? "Loading…" : "No test presets"}</p>
          ) : (
            presets.map((preset) => {
              const id = preset.entity?.id?.value ?? "";
              return (
                <label key={id} className="flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1 text-sm hover:bg-muted/40">
                  <Checkbox checked={selected.includes(id)} onCheckedChange={() => toggle(id)} />
                  {presetTitle(preset)}
                </label>
              );
            })
          )}
        </div>
      </div>
    </FormDialog>
  );
}
