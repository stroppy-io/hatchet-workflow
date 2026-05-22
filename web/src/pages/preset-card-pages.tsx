import { ArrowRight, Database, FlaskConical, Search, Workflow } from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatTimestamp } from "@/lib/format";
import {
  databasePresetSummary,
  presetDescription,
  presetTitle,
  testPresetSummary,
  workloadPresetSummary,
} from "@/lib/preset-summary";
import { Preset_Kind, type Preset } from "@/lib/proto/cloud/v1/models/preset_pb.ts";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import { ListPresetRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/preset_pb.ts";
import { tenantPath } from "@/lib/routes";

type PresetCardPageProps = {
  icon: ReactNode;
  kind: Preset_Kind;
  searchPlaceholder: string;
  title: string;
  variant: "database" | "workload" | "test";
};

export function DatabasePresetsPage() {
  return (
    <PresetCardPage
      icon={<Database className="size-5" />}
      kind={Preset_Kind.DATABASE}
      searchPlaceholder="Search database presets"
      title="Database Presets"
      variant="database"
    />
  );
}

export function WorkloadPresetsPage() {
  return (
    <PresetCardPage
      icon={<FlaskConical className="size-5" />}
      kind={Preset_Kind.WORKLOAD}
      searchPlaceholder="Search workload presets"
      title="Workload Presets"
      variant="workload"
    />
  );
}

export function TestPresetsPage() {
  return (
    <PresetCardPage
      icon={<Workflow className="size-5" />}
      kind={Preset_Kind.TEST}
      searchPlaceholder="Search test presets"
      title="Test Presets"
      variant="test"
    />
  );
}

function PresetCardPage({ icon, kind, searchPlaceholder, title, variant }: PresetCardPageProps) {
  const tenantId = useTenantId();
  const pagination = useCursorPagination(24);
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search);
  const result = useListQuery(
    () =>
      api.preset.listPresets({
        tenantId: { value: tenantId },
        kinds: [kind],
        search: debouncedSearch || undefined,
        sortField: ListPresetRequest_SortField.NAME,
        order: SortOrder.ASC,
        page: pagination.page,
      }),
    [tenantId, kind, debouncedSearch, pagination.page],
  );

  const groups = useMemo(() => groupPresets(result.data?.presets ?? [], variant), [result.data?.presets, variant]);

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-md border bg-muted text-primary">{icon}</div>
          <div>
            <h1 className="text-xl font-semibold tracking-normal">{title}</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              {result.loading ? "Loading" : `${result.data?.presets.length ?? 0} presets`}
              {result.data?.pageInfo?.total !== undefined ? ` of ${Number(result.data.pageInfo.total)}` : ""}
            </p>
          </div>
        </div>
        <div className="relative min-w-72">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="pl-9"
            onChange={(event) => {
              setSearch(event.target.value);
              pagination.reset();
            }}
            placeholder={searchPlaceholder}
            value={search}
          />
        </div>
      </div>

      {result.error ? <div className="border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">{result.error}</div> : null}

      {result.loading && !result.data ? (
        <div className="border py-16 text-center text-sm text-muted-foreground">Loading presets...</div>
      ) : groups.length ? (
        <div className="space-y-7">
          {groups.map((group) => (
            <div key={group.title} className="space-y-3">
              <div className="flex items-center gap-3">
                <h2 className="text-sm font-semibold uppercase tracking-normal text-muted-foreground">{group.title}</h2>
                <div className="h-px flex-1 bg-border" />
                <span className="text-xs text-muted-foreground">{group.presets.length}</span>
              </div>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {group.presets.map((preset) => (
                  <PresetCard key={preset.entity?.id?.value ?? presetTitle(preset)} preset={preset} variant={variant} />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="border py-16 text-center text-sm text-muted-foreground">No presets</div>
      )}

      <div className="flex items-center justify-between gap-3">
        <div className="text-sm text-muted-foreground">Page {pagination.index + 1}</div>
        <div className="flex items-center gap-2">
          <Button disabled={!pagination.canPrevious || result.loading} onClick={pagination.previous} size="sm" variant="outline">
            Previous
          </Button>
          <Button disabled={!result.data?.pageInfo?.hasMore || result.loading} onClick={() => pagination.next(result.data?.pageInfo)} size="sm" variant="outline">
            Next
          </Button>
        </div>
      </div>
    </section>
  );
}

function PresetCard({ preset, variant }: { preset: Preset; variant: PresetCardPageProps["variant"] }) {
  const tenantId = useTenantId();
  const navigate = useNavigate();
  const description = presetDescription(preset);
  const summary = buildSummary(preset, variant);

  return (
    <article className="flex min-h-56 flex-col rounded-md border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold">{presetTitle(preset)}</h3>
          <p className="mt-1 line-clamp-2 min-h-10 text-sm text-muted-foreground">{description || summary.headline}</p>
        </div>
        <Badge variant="outline">{summary.badge}</Badge>
      </div>

      <div className="mt-4 grid gap-2 text-sm">
        {summary.rows.map((row) => (
          <div key={row.label} className="grid grid-cols-[6rem_minmax(0,1fr)] gap-3">
            <span className="text-muted-foreground">{row.label}</span>
            <span className="min-w-0 truncate">{row.value}</span>
          </div>
        ))}
      </div>

      {summary.chips.length ? (
        <div className="mt-4 flex flex-wrap gap-1.5">
          {summary.chips.map((chip) => (
            <Badge key={chip} variant="secondary">
              {chip}
            </Badge>
          ))}
        </div>
      ) : null}

      <div className="mt-auto flex items-center justify-between pt-4">
        <span className="text-xs text-muted-foreground">{formatTimestamp(preset.entity?.timestamps?.updatedAt || preset.entity?.timestamps?.createdAt)}</span>
        <Button
          onClick={() => navigate(tenantPath(tenantId, `/presets/${routePart(variant)}/${preset.entity?.id?.value ?? ""}`))}
          size="sm"
          variant="ghost"
        >
          Open
          <ArrowRight />
        </Button>
      </div>
    </article>
  );
}

function groupPresets(presets: Preset[], variant: PresetCardPageProps["variant"]) {
  const map = new Map<string, Preset[]>();
  for (const preset of presets) {
    const group = buildSummary(preset, variant).group;
    map.set(group, [...(map.get(group) ?? []), preset]);
  }
  return Array.from(map.entries())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([groupTitle, groupPresets]) => ({
      title: groupTitle,
      presets: groupPresets.sort((left, right) => presetTitle(left).localeCompare(presetTitle(right))),
    }));
}

function buildSummary(preset: Preset, variant: PresetCardPageProps["variant"]) {
  if (variant === "database") {
    const summary = databasePresetSummary(preset.preset.case === "databasePreset" ? preset.preset.value : undefined);
    return {
      badge: summary.primary,
      chips: summary.components,
      group: summary.group,
      headline: summary.topology,
      rows: [
        { label: "Target", value: summary.target },
        { label: "Topology", value: summary.topology },
      ],
    };
  }

  if (variant === "workload") {
    const summary = workloadPresetSummary(preset.preset.case === "workloadPreset" ? preset.preset.value : undefined);
    return {
      badge: summary.protocol,
      chips: [],
      group: summary.group,
      headline: summary.primary,
      rows: [
        { label: "Script", value: summary.primary },
        { label: "Execution", value: summary.execution },
        { label: "Topology", value: summary.topology },
      ],
    };
  }

  const summary = testPresetSummary(preset.preset.case === "testPreset" ? preset.preset.value : undefined);
  return {
    badge: summary.database,
    chips: [summary.protocol],
    group: summary.group,
    headline: `${summary.database} + ${summary.workload}`,
    rows: [
      { label: "Database", value: summary.database },
      { label: "Workload", value: summary.workload },
      { label: "Topology", value: summary.topology },
      { label: "Deploy", value: summary.deployment },
    ],
  };
}

function routePart(variant: PresetCardPageProps["variant"]) {
  switch (variant) {
    case "database":
      return "database";
    case "workload":
      return "workload";
    case "test":
      return "test";
  }
}
