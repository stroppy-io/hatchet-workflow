import { useCallback, useEffect, useMemo, useState, Fragment } from "react";
import { Link, useNavigate, useParams } from "@/lib/router";
import {
  cancelBatch,
  cloneSuite,
  compareRuns,
  crossBatch,
  deleteSuiteItem,
  getSuite,
  launchSuite,
  listPresets,
  reorderSuiteItems,
  updateSuite,
  updateSuiteItem,
} from "@/services/client";
import { KIND_PROTOCOLS, SCRIPT_COMPAT } from "@/services/types";
import type {
  ComparisonResponse,
  Preset,
  Suite,
  SuiteBatchSummary,
  SuiteCrossGroup,
  SuiteCrossMember,
  SuiteItem,
} from "@/services/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { DB_COLORS } from "@/lib/db-colors";
import {
  AlertCircle,
  ArrowLeft,
  Boxes,
  CalendarClock,
  Check,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  Cpu,
  Database,
  GitCompareArrows,
  GripVertical,
  Layers,
  Network,
  Pause,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Tag,
  Trash2,
  Wrench,
  X,
  XCircle,
  Zap,
} from "lucide-react";

// SuiteDetail — visual analogue of RunDetail. Header (identity + status +
// actions), tabbed content area (Overview, Workloads, Matrix, Batches),
// each tab wrapped in a `Card` so the page reads as a sibling of the runs
// section.

function formatTimestamp(ts?: string): string {
  if (!ts) return "—";
  const d = new Date(ts);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "—";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function isCompatible(preset: Preset, item: SuiteItem): boolean {
  const protocols = KIND_PROTOCOLS[preset.db_kind as keyof typeof KIND_PROTOCOLS] ?? [];
  const proto = item.workload?.protocol || protocols[0];
  if (!proto) return false;
  const supported = SCRIPT_COMPAT[`${preset.db_kind}:${proto}`] ?? [];
  return supported.includes(item.workload?.script ?? "");
}

function countCompatibleCells(presets: Preset[], items: SuiteItem[]): number {
  let n = 0;
  for (const p of presets) for (const it of items) if (isCompatible(p, it)) n++;
  return n;
}

export function SuiteDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const confirm = useConfirm();

  const [suite, setSuite] = useState<Suite | null>(null);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [, setLoading] = useState(true);

  const [activeTab, setActiveTab] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get("tab") || "overview";
  });

  const presetMap = useMemo(() => {
    const m: Record<string, Preset> = {};
    for (const p of presets) m[p.id] = p;
    return m;
  }, [presets]);

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [s, ps] = await Promise.all([getSuite(id), listPresets()]);
      setSuite(s);
      setPresets(ps);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  async function toggleSuiteEnabled(enabled: boolean) {
    if (!suite) return;
    setBusy(true);
    try {
      await updateSuite(suite.id, { enabled });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function toggleItem(it: SuiteItem, enabled: boolean) {
    setBusy(true);
    try {
      await updateSuiteItem(it.suite_id, it.id, { enabled });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function removeItem(it: SuiteItem) {
    const ok = await confirm({
      title: `Delete workload "${it.name}"?`,
      description: "Removed from this suite immediately. Past batches keep their snapshot.",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await deleteSuiteItem(it.suite_id, it.id);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed");
    } finally {
      setBusy(false);
    }
  }

  async function commitOrder(items: SuiteItem[]) {
    if (!id) return;
    const order: Record<string, number> = {};
    items.forEach((it, idx) => { order[it.id] = idx; });
    setBusy(true);
    try {
      await reorderSuiteItems(id, order);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Reorder failed");
    } finally {
      setBusy(false);
    }
  }

  async function launch() {
    if (!suite) return;
    setBusy(true);
    try {
      const r = await launchSuite(suite.id);
      navigate(`/runs?suite=${suite.id}&batch=${r.batch_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Launch failed");
    } finally {
      setBusy(false);
    }
  }

  async function clone() {
    if (!suite) return;
    setBusy(true);
    try {
      const r = await cloneSuite(suite.id);
      navigate(`/suites/${r.id}/edit`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Clone failed");
    } finally {
      setBusy(false);
    }
  }

  async function cancelB(batchID: string) {
    if (!suite) return;
    const ok = await confirm({
      title: `Cancel batch "${batchID.slice(0, 8)}"?`,
      description: "Cancels queued + running runs. Already-finished runs stay as they are.",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await cancelBatch(suite.id, batchID);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Cancel failed");
    } finally {
      setBusy(false);
    }
  }

  if (!suite) {
    return (
      <div className="p-6">
        {error ? (
          <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        ) : (
          <div className="text-sm text-muted-foreground">Loading suite…</div>
        )}
      </div>
    );
  }

  const items = (suite.items ?? []).slice().sort((a, b) => a.position - b.position);
  const enabledItems = items.filter((i) => i.enabled);
  const dbPresets = (suite.db_preset_ids ?? []).map((pid) => presetMap[pid]).filter(Boolean) as Preset[];
  const cellsTotal = dbPresets.length * enabledItems.length;
  const cellsCompatible = countCompatibleCells(dbPresets, enabledItems);
  const batches = suite.batches ?? [];

  // Status badge mirrors RunDetail's inline status indicator next to the title.
  const statusNode =
    suite.cron_expr && suite.enabled ? (
      <Badge className="bg-emerald-500/15 text-emerald-400 border-emerald-500/30">Scheduled</Badge>
    ) : suite.cron_expr ? (
      <Badge className="bg-zinc-500/20 text-zinc-400 border-zinc-500/30">Paused</Badge>
    ) : (
      <Badge className="bg-zinc-700/40 text-zinc-300 border-zinc-600/40">Manual</Badge>
    );

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 min-w-0">
          <Button variant="ghost" size="sm" onClick={() => navigate("/suites")} className="text-zinc-500 hover:text-zinc-200 -ml-2">
            <ArrowLeft className="h-3.5 w-3.5" />
          </Button>
          <div className="min-w-0">
            <h1 className="text-lg font-semibold leading-tight flex items-center gap-2">
              <Boxes className="w-4 h-4 text-primary shrink-0" />
              <span className="truncate">{suite.name}</span>
            </h1>
            <p className="text-[11px] font-mono text-zinc-500 leading-tight truncate">{suite.id}</p>
            {suite.description && (
              <p className="text-xs text-zinc-400 mt-1 max-w-2xl whitespace-pre-wrap">
                {suite.description}
              </p>
            )}
          </div>
          {statusNode}
        </div>
        <div className="flex items-center gap-3">
          <Button variant="outline" size="sm" onClick={load} disabled={busy}>
            <RefreshCw className={`h-3.5 w-3.5 ${busy ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          {suite.cron_expr && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => toggleSuiteEnabled(!suite.enabled)}
              disabled={busy}
              className={suite.enabled ? "border-zinc-700" : "border-emerald-700 text-emerald-400 hover:bg-emerald-500/10"}
            >
              {suite.enabled ? <Pause className="h-3.5 w-3.5" /> : <Play className="h-3.5 w-3.5" />}
              {suite.enabled ? "Pause" : "Resume"}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={() => navigate(`/suites/${suite.id}/edit`)}>
            <Pencil className="h-3.5 w-3.5" />
            Edit
          </Button>
          <Button variant="outline" size="sm" onClick={clone} disabled={busy}>
            <Copy className="h-3.5 w-3.5" />
            Clone
          </Button>
          <Button
            size="sm"
            onClick={launch}
            disabled={busy || cellsCompatible === 0}
            className="border-primary/40 bg-primary/10 text-primary hover:bg-primary/20"
            variant="outline"
          >
            <Play className="h-3.5 w-3.5" />
            Launch matrix
            <span className="text-[10px] font-mono opacity-70 tabular-nums">
              ({cellsCompatible}/{cellsTotal})
            </span>
          </Button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="workloads">
            Workloads
            <span className="ml-1.5 text-[10px] text-zinc-600 tabular-nums">{items.length}</span>
          </TabsTrigger>
          <TabsTrigger value="matrix">
            Matrix
            <span className="ml-1.5 text-[10px] text-zinc-600 tabular-nums">{cellsCompatible}/{cellsTotal}</span>
          </TabsTrigger>
          <TabsTrigger value="batches">
            Batches
            <span className="ml-1.5 text-[10px] text-zinc-600 tabular-nums">{batches.length}</span>
          </TabsTrigger>
          <TabsTrigger value="compare">Compare</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <Card className="min-h-[calc(100vh-13rem)]">
            <CardContent className="p-0 h-full">
              <OverviewPanel suite={suite} dbPresets={dbPresets} cellsCompatible={cellsCompatible} cellsTotal={cellsTotal} />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="workloads">
          <Card className="min-h-[calc(100vh-13rem)]">
            <CardContent className="p-0">
              <div className="flex items-center justify-between px-4 py-2.5 border-b border-zinc-800/60">
                <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">Workloads</div>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => navigate(`/suites/${suite.id}/items/new`)}
                  className="border-primary/40 text-primary hover:bg-primary/10"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add workload
                </Button>
              </div>
              {items.length === 0 ? (
                <div className="px-4 py-12 text-center text-xs font-mono text-zinc-600">
                  No workloads yet — add one to fan out across every DB target.
                </div>
              ) : (
                <ItemsList
                  items={items}
                  busy={busy}
                  onReorder={commitOrder}
                  onToggle={toggleItem}
                  onRemove={removeItem}
                  onEdit={(it) => navigate(`/suites/${suite.id}/items/${it.id}/edit`)}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="matrix">
          <Card className="min-h-[calc(100vh-13rem)]">
            <CardContent className="p-0">
              <div className="flex items-center justify-between px-4 py-2.5 border-b border-zinc-800/60">
                <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">
                  Matrix · {cellsCompatible} compatible / {cellsTotal} cells
                </div>
              </div>
              {dbPresets.length === 0 || items.length === 0 ? (
                <div className="px-4 py-12 text-center text-xs font-mono text-zinc-600">
                  Need at least one DB target and one workload to build the matrix.
                </div>
              ) : (
                <MatrixView dbPresets={dbPresets} items={items} />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="batches">
          <Card className="min-h-[calc(100vh-13rem)]">
            <CardContent className="p-0">
              <div className="flex items-center justify-between px-4 py-2.5 border-b border-zinc-800/60">
                <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">
                  Batches
                </div>
              </div>
              {batches.length === 0 ? (
                <div className="px-4 py-12 text-center text-xs font-mono text-zinc-600">
                  No batches yet — launch the matrix to produce one.
                </div>
              ) : (
                <BatchesList suiteID={suite.id} batches={batches} presetMap={presetMap} onCancel={cancelB} />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="compare">
          <Card className="min-h-[calc(100vh-13rem)]">
            <CardContent className="p-0">
              <CompareTab suiteID={suite.id} batches={batches} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

// ─── Overview panel — RunOverview-style sections ───────────────────

function OverviewPanel({
  suite,
  dbPresets,
  cellsCompatible,
  cellsTotal,
}: {
  suite: Suite;
  dbPresets: Preset[];
  cellsCompatible: number;
  cellsTotal: number;
}) {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] divide-zinc-800/50 lg:divide-x">
      {/* Left column — config readout */}
      <div className="flex flex-col">
        {/* Identity */}
        <Section label="Identity">
          <ConfigLine label="id" value={suite.id} icon={Tag} />
          <ConfigLine label="name" value={suite.name} icon={Boxes} />
          {suite.description && <ConfigLine label="desc" value={suite.description} />}
        </Section>

        {/* Schedule */}
        <Section label="Schedule">
          {suite.cron_expr ? (
            <>
              <ConfigLine label="cron" value={suite.cron_expr} icon={Clock} />
              <ConfigLine label="tz" value={suite.timezone || "UTC"} />
              <ConfigLine label="enabled" value={suite.enabled ? "yes" : "paused"} />
              <ConfigLine label="next" value={suite.next_fire_at ? formatTimestamp(suite.next_fire_at) : "—"} icon={CalendarClock} />
              <ConfigLine label="last" value={formatTimestamp(suite.last_fire_at)} icon={Clock} />
              <ConfigLine label="concurrent" value={suite.concurrent_policy} />
              <ConfigLine label="catchup" value={suite.catchup_mode} />
            </>
          ) : (
            <div className="text-[11px] font-mono text-zinc-600">Manual launches only.</div>
          )}
        </Section>

        {/* Policy + Comparison + Retention */}
        <Section label="Execution">
          <ConfigLine label="mode" value={suite.policy?.mode} icon={Wrench} />
          {suite.policy?.mode === "parallel" && (
            <ConfigLine label="parallel" value={String(suite.policy.max_parallel || "unlimited")} />
          )}
          <ConfigLine label="on fail" value={suite.policy?.on_step_fail} />
          {(suite.policy?.step_timeout_min ?? 0) > 0 && (
            <ConfigLine label="timeout" value={`${suite.policy.step_timeout_min} min`} />
          )}
          <ConfigLine
            label="baseline"
            value={suite.comparison?.strategy || "none"}
            icon={GitCompareArrows}
          />
          <ConfigLine label="diff %" value={String(suite.comparison?.threshold_pct ?? 5)} />
          <ConfigLine label="retain" value={`last ${suite.retention_runs}`} />
        </Section>

        {/* Infrastructure */}
        <Section label="Infrastructure">
          <ConfigLine label="provider" value={suite.provider} icon={Cpu} />
          {suite.platform_id && <ConfigLine label="platform" value={suite.platform_id} />}
          <ConfigLine label="cidr" value={suite.network?.cidr} icon={Network} />
          {suite.network?.zone && <ConfigLine label="zone" value={suite.network.zone} />}
        </Section>
      </div>

      {/* Right column — matrix summary + targets */}
      <div className="flex flex-col">
        <Section label="Matrix summary">
          <div className="grid grid-cols-3 gap-3">
            <Stat label="DB targets" value={dbPresets.length} icon={Layers} />
            <Stat label="Workloads" value={(suite.items ?? []).length} icon={Zap} />
            <Stat
              label="Matrix"
              value={`${cellsCompatible}/${cellsTotal}`}
              valueClass={
                cellsCompatible === 0
                  ? "text-zinc-500"
                  : cellsCompatible < cellsTotal
                    ? "text-amber-400"
                    : "text-emerald-400"
              }
            />
          </div>
        </Section>

        <Section label="DB Targets">
          {dbPresets.length === 0 ? (
            <div className="text-[11px] font-mono text-amber-500/80">
              No targets — open the builder to pick at least one.
            </div>
          ) : (
            <div className="space-y-2">
              {dbPresets.map((p) => {
                const color = DB_COLORS[p.db_kind as keyof typeof DB_COLORS];
                return (
                  <div
                    key={p.id}
                    className="border border-zinc-800/60 bg-[#070707] px-2.5 py-2 flex gap-3"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span
                          className="w-2 h-2 rounded-full shrink-0"
                          style={{ backgroundColor: color?.hex ?? "#666" }}
                        />
                        <span className={`text-[10px] font-mono uppercase tracking-wider ${color?.text ?? "text-zinc-400"}`}>
                          {p.db_kind}
                        </span>
                        <span className="text-xs font-mono text-zinc-300 truncate">{p.name}</span>
                      </div>
                      <TopologyDiagram kind={p.db_kind} topology={p.topology} />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Section>
      </div>
    </div>
  );
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="px-4 py-3 border-b border-zinc-800/50 last:border-b-0">
      <div className="text-[11px] text-zinc-500 uppercase tracking-wider mb-2">{label}</div>
      <div className="space-y-0">{children}</div>
    </div>
  );
}

function ConfigLine({
  label,
  value,
  icon: Icon,
}: {
  label: string;
  value?: React.ReactNode;
  icon?: typeof Cpu;
}) {
  if (value === undefined || value === null || value === "") return null;
  return (
    <div className="flex items-start gap-2 py-[3px]">
      {Icon ? <Icon className="w-3 h-3 text-zinc-600 shrink-0 mt-[3px]" /> : <span className="w-3 shrink-0" />}
      <span className="text-[11px] text-zinc-500 shrink-0 w-20 mt-[1px]">{label}</span>
      <span className="text-xs text-zinc-300 font-mono break-all flex-1 min-w-0">{value}</span>
    </div>
  );
}

function Stat({
  label,
  value,
  icon: Icon,
  valueClass,
}: {
  label: string;
  value: React.ReactNode;
  icon?: typeof Cpu;
  valueClass?: string;
}) {
  return (
    <div className="border border-zinc-800/60 bg-[#070707] px-2.5 py-2">
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600 inline-flex items-center gap-1">
        {Icon && <Icon className="w-3 h-3" />}
        {label}
      </div>
      <div className={`text-base font-mono font-semibold tabular-nums mt-0.5 ${valueClass ?? "text-zinc-100"}`}>
        {value}
      </div>
    </div>
  );
}

// ─── Workloads list (drag-drop) ─────────────────────────────────────

function ItemsList({
  items, busy, onReorder, onToggle, onRemove, onEdit,
}: {
  items: SuiteItem[];
  busy: boolean;
  onReorder: (next: SuiteItem[]) => void;
  onToggle: (it: SuiteItem, enabled: boolean) => void;
  onRemove: (it: SuiteItem) => void;
  onEdit: (it: SuiteItem) => void;
}) {
  const [draft, setDraft] = useState<SuiteItem[] | null>(null);
  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const view = draft ?? items;

  function onDragStart(idx: number) {
    setDragIdx(idx);
    setDraft(items.slice());
  }
  function onDragOver(idx: number, e: React.DragEvent) {
    e.preventDefault();
    if (dragIdx === null || draft === null || idx === dragIdx) return;
    const next = draft.slice();
    const [moved] = next.splice(dragIdx, 1);
    next.splice(idx, 0, moved);
    setDragIdx(idx);
    setDraft(next);
  }
  function commit() {
    if (draft && dragIdx !== null) onReorder(draft);
    setDraft(null);
    setDragIdx(null);
  }

  return (
    <div>
      {view.map((it, idx) => {
        const w = it.workload;
        const summary = [w?.script, w?.duration, w?.vus ? `${w.vus} VU` : null, w?.scale_factor ? `sf ${w.scale_factor}` : null]
          .filter(Boolean).join(" · ");
        return (
          <div
            key={it.id}
            draggable
            onDragStart={() => onDragStart(idx)}
            onDragOver={(e) => onDragOver(idx, e)}
            onDrop={commit}
            onDragEnd={commit}
            className={`flex items-center gap-3 px-4 py-2.5 border-b border-zinc-800/50 last:border-b-0 hover:bg-zinc-900/50 transition-colors ${
              dragIdx === idx ? "opacity-50 bg-primary/[0.04]" : ""
            }`}
          >
            <GripVertical className="w-3.5 h-3.5 text-zinc-700 cursor-grab shrink-0" />
            <span className="text-[10px] text-zinc-700 font-mono w-6 text-right tabular-nums">
              {String(idx + 1).padStart(2, "0")}
            </span>
            <div className="flex-1 min-w-0">
              <div className={`text-xs font-mono truncate ${it.enabled ? "text-zinc-200" : "text-zinc-500"}`}>
                {it.name || "(unnamed)"}
              </div>
              <div className="text-[10px] font-mono text-zinc-600 truncate">{summary || "—"}</div>
            </div>
            <span data-row-stop className="inline-flex items-center" onClick={(e) => e.stopPropagation()}>
              <Switch
                checked={it.enabled}
                disabled={busy}
                onCheckedChange={(v: boolean) => onToggle(it, v)}
              />
            </span>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-500 hover:text-primary"
              onClick={() => onEdit(it)}
              title="Edit workload"
            >
              <Pencil className="w-3.5 h-3.5" />
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0 text-zinc-600 hover:text-destructive"
              onClick={() => onRemove(it)}
              disabled={busy}
              title="Delete workload"
            >
              <Trash2 className="w-3.5 h-3.5" />
            </Button>
          </div>
        );
      })}
    </div>
  );
}

// ─── Matrix table ──────────────────────────────────────────────────

function MatrixView({ dbPresets, items }: { dbPresets: Preset[]; items: SuiteItem[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="text-xs w-full font-mono">
        <thead>
          <tr className="border-b border-zinc-800/80">
            <th className="text-left px-3 py-2 text-[11px] font-mono uppercase tracking-wider text-zinc-500 bg-zinc-900/50 sticky left-0 z-10">
              db preset \ workload
            </th>
            {items.map((it) => (
              <th
                key={it.id}
                className="text-left px-3 py-2 text-[11px] font-mono uppercase tracking-wider text-zinc-500 bg-zinc-900/50 border-l border-zinc-800/50 min-w-[10rem]"
              >
                <div className="text-zinc-300 normal-case font-medium truncate max-w-48 tracking-normal">
                  {it.name}
                </div>
                <div className="text-[10px] font-mono text-zinc-600 normal-case truncate tracking-normal">
                  {it.workload?.script}
                </div>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {dbPresets.map((p) => {
            const color = DB_COLORS[p.db_kind as keyof typeof DB_COLORS];
            return (
              <tr key={p.id} className="border-b border-zinc-800/50 last:border-b-0 hover:bg-zinc-900/30">
                <td className="px-3 py-2 sticky left-0 z-10 bg-[#080808]">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span
                      className="w-1.5 h-1.5 rounded-full shrink-0"
                      style={{ backgroundColor: color?.hex ?? "#666" }}
                    />
                    <span className={`text-[10px] uppercase tracking-wider ${color?.text ?? "text-zinc-400"}`}>
                      {p.db_kind}
                    </span>
                  </div>
                  <div className="text-xs text-zinc-300 truncate">{p.name}</div>
                </td>
                {items.map((it) => {
                  const compat = isCompatible(p, it);
                  return (
                    <td
                      key={it.id}
                      className={`px-3 py-2 border-l border-zinc-800/50 text-center ${
                        compat ? "bg-emerald-500/[0.04]" : "bg-zinc-900/30"
                      }`}
                    >
                      {compat ? (
                        <span className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-emerald-500/20 text-emerald-400" title="Will run on launch">
                          <Check className="w-3 h-3" strokeWidth={3} />
                        </span>
                      ) : (
                        <span
                          className="inline-flex items-center justify-center w-5 h-5 rounded-full bg-zinc-800/60 text-zinc-600"
                          title={`Script "${it.workload?.script}" not supported on ${p.db_kind}`}
                        >
                          <X className="w-3 h-3" strokeWidth={3} />
                        </span>
                      )}
                    </td>
                  );
                })}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ─── Batches list ──────────────────────────────────────────────────

function BatchesList({
  suiteID, batches, presetMap, onCancel,
}: {
  suiteID: string;
  batches: SuiteBatchSummary[];
  presetMap: Record<string, Preset>;
  onCancel: (batchID: string) => void;
}) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  function toggle(bid: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(bid)) next.delete(bid); else next.add(bid);
      return next;
    });
  }

  return (
    <Table>
      <TableHeader>
        <TableRow className="border-zinc-800/80 hover:bg-transparent">
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50 w-8" />
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Batch</TableHead>
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">When</TableHead>
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Trigger</TableHead>
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50">Progress</TableHead>
          <TableHead className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 h-9 bg-zinc-900/50 text-right" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {batches.map((b) => {
          const isOpen = expanded.has(b.batch_id);
          const live = b.running > 0 || b.queued > 0;
          return (
            <Fragment key={b.batch_id}>
              <TableRow
                className="border-zinc-800/50 hover:bg-zinc-900/60 cursor-pointer transition-colors"
                onClick={() => toggle(b.batch_id)}
              >
                <TableCell className="py-2.5">
                  {isOpen ? <ChevronDown className="w-4 h-4 text-zinc-500" /> : <ChevronRight className="w-4 h-4 text-zinc-500" />}
                </TableCell>
                <TableCell className="py-2.5 font-mono text-xs text-primary">{b.batch_id.slice(0, 8)}</TableCell>
                <TableCell className="py-2.5 font-mono text-xs text-zinc-400">{formatTimestamp(b.created_at)}</TableCell>
                <TableCell className="py-2.5 font-mono text-xs text-zinc-500 capitalize">{b.trigger || "—"}</TableCell>
                <TableCell className="py-2.5">
                  <BatchProgress b={b} />
                </TableCell>
                <TableCell className="py-2.5 text-right" onClick={(e) => e.stopPropagation()}>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => onCancel(b.batch_id)}
                    disabled={!live}
                    className="h-7 w-7 p-0 text-amber-600 hover:text-amber-400 cursor-pointer disabled:opacity-30"
                    title={live ? "Cancel queued/running runs" : "Nothing to cancel"}
                  >
                    <XCircle className="h-3.5 w-3.5" />
                  </Button>
                </TableCell>
              </TableRow>
              {isOpen && (
                <TableRow className="border-zinc-800/50 hover:bg-transparent">
                  <TableCell colSpan={6} className="bg-[#050505] p-4">
                    <BatchDetail batch={b} presetMap={presetMap} />
                  </TableCell>
                </TableRow>
              )}
            </Fragment>
          );
        })}
      </TableBody>
    </Table>
  );
}

function BatchProgress({ b }: { b: SuiteBatchSummary }) {
  const total = b.total || 1;
  const donePct = (b.finished / total) * 100;
  const failPct = (b.failed / total) * 100;
  const cxlPct = (b.cancelled / total) * 100;
  const runPct = (b.running / total) * 100;
  return (
    <div className="flex items-center gap-2 min-w-[180px]">
      <div className="flex-1 bg-zinc-900 h-1.5 overflow-hidden flex">
        <div className="bg-emerald-500 h-full transition-all" style={{ width: `${donePct}%` }} />
        <div className="bg-destructive h-full transition-all" style={{ width: `${failPct}%` }} />
        <div className="bg-zinc-500 h-full transition-all" style={{ width: `${cxlPct}%` }} />
        <div className="bg-amber-500 h-full transition-all animate-pulse" style={{ width: `${runPct}%` }} />
      </div>
      <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-16 text-right">
        {b.finished + b.failed + b.cancelled}/{b.total}
      </span>
    </div>
  );
}

function BatchDetail({
  batch,
  presetMap,
}: {
  batch: SuiteBatchSummary;
  presetMap: Record<string, Preset>;
}) {
  return (
    <div>
      <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 mb-2">
        Cells <span className="text-zinc-700">({batch.runs.length})</span>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-1">
        {batch.runs.map((r) => {
          const pname = r.db_preset_id ? presetMap[r.db_preset_id]?.name ?? r.db_preset_id.slice(0, 6) : "—";
          const color = r.db_preset_id ? DB_COLORS[presetMap[r.db_preset_id]?.db_kind as keyof typeof DB_COLORS] : null;
          const stateColor =
            r.state === "finished" ? "text-emerald-400" :
            r.state === "failed" ? "text-red-400" :
            r.state === "cancelled" ? "text-zinc-500" :
            r.state === "running" ? "text-amber-400" :
            "text-zinc-500";
          return (
            <Link
              key={r.run_id}
              to={`/runs/${r.run_id}`}
              className="flex items-center gap-2 px-2 py-1 border border-zinc-800/60 bg-[#070707] hover:border-zinc-700 hover:bg-zinc-900/60 transition-colors text-[11px] font-mono"
              title={r.run_id}
            >
              {color ? (
                <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: color.hex }} />
              ) : (
                <span className="w-1.5 h-1.5 shrink-0" />
              )}
              <span className="text-zinc-500 truncate flex-1">[{pname}] {r.run_id.slice(-12)}</span>
              <span className={`shrink-0 ${stateColor}`}>{r.state}</span>
            </Link>
          );
        })}
      </div>
    </div>
  );
}


// ─── Compare tab — N-way cross-DB / cross-workload table ──────────

function CompareTab({ suiteID, batches }: { suiteID: string; batches: SuiteBatchSummary[] }) {
  const [pivot, setPivot] = useState<"item" | "preset">("item");
  const [batchID, setBatchID] = useState<string>(() => batches[0]?.batch_id || "");
  const [groups, setGroups] = useState<SuiteCrossGroup[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!batchID) return;
    setLoading(true);
    setErr(null);
    crossBatch(suiteID, batchID, pivot)
      .then((r) => setGroups(r.groups))
      .catch((e) => setErr(e instanceof Error ? e.message : "Failed"))
      .finally(() => setLoading(false));
  }, [suiteID, batchID, pivot]);

  if (batches.length === 0) {
    return (
      <div className="px-4 py-12 text-center text-xs font-mono text-zinc-600">
        No batches yet — launch the matrix to compare runs.
      </div>
    );
  }

  return (
    <div>
      {/* Toolbar */}
      <div className="flex items-center gap-3 px-4 py-2.5 border-b border-zinc-800/60 flex-wrap">
        <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">Compare</span>

        <div className="inline-flex border border-zinc-800 overflow-hidden text-[11px] font-mono">
          <button
            type="button"
            onClick={() => setPivot("item")}
            className={`px-2.5 py-1 ${pivot === "item" ? "bg-primary/10 text-primary" : "text-zinc-500 hover:text-zinc-300"}`}
          >by workload</button>
          <button
            type="button"
            onClick={() => setPivot("preset")}
            className={`px-2.5 py-1 border-l border-zinc-800 ${pivot === "preset" ? "bg-primary/10 text-primary" : "text-zinc-500 hover:text-zinc-300"}`}
          >by DB</button>
        </div>

        <div className="flex items-center gap-1.5">
          <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Batch</span>
          <select
            value={batchID}
            onChange={(e) => setBatchID(e.target.value)}
            className="h-7 bg-zinc-900 border border-zinc-800 px-2 text-[11px] font-mono text-zinc-300 outline-none focus:border-zinc-600"
          >
            {batches.map((b) => (
              <option key={b.batch_id} value={b.batch_id}>
                {b.batch_id.slice(0, 8)} · {new Date(b.created_at).toLocaleString()}
              </option>
            ))}
          </select>
        </div>

        {loading && <Loader className="text-[10px] font-mono text-zinc-500">loading…</Loader>}
      </div>

      {err && (
        <div className="px-4 py-3 text-[11px] font-mono text-destructive border-b border-zinc-800/60">
          {err}
        </div>
      )}

      {groups && groups.length === 0 && (
        <div className="px-4 py-12 text-center text-xs font-mono text-zinc-600">
          No groups in this batch.
        </div>
      )}

      {groups && groups.length > 0 && (
        <div className="space-y-4 p-4">
          {groups.map((g) => (
            <NWayTable key={g.group_key} group={g} pivot={pivot} />
          ))}
        </div>
      )}
    </div>
  );
}

function Loader({ children, className }: { children: React.ReactNode; className?: string }) {
  return <span className={className}>{children}</span>;
}

// NWayTable fans out compareRuns(anchor, member) for every non-anchor
// member of one group, then merges results into a single table by metric
// key. Anchor's avg comes from the first compare's avg_a; each other
// member contributes its avg_b + diff_avg_pct + verdict.
function NWayTable({ group, pivot }: { group: SuiteCrossGroup; pivot: "item" | "preset" }) {
  const finishedMembers = useMemo(
    () => group.members.filter((m) => m.state === "finished"),
    [group],
  );
  const anchor = finishedMembers[0];
  const others = finishedMembers.slice(1);
  const failed = group.members.filter((m) => m.state !== "finished");

  // For each (anchor, other) pair fetch the comparison.
  const [pairs, setPairs] = useState<Record<string, ComparisonResponse | null>>({});
  const [pairErrs, setPairErrs] = useState<Record<string, string>>({});
  const [pairLoading, setPairLoading] = useState(false);

  useEffect(() => {
    if (!anchor || others.length === 0) {
      setPairs({});
      return;
    }
    setPairLoading(true);
    setPairErrs({});
    Promise.all(
      others.map((m) =>
        compareRuns(anchor.run_id, m.run_id)
          .then((r) => [m.run_id, r] as const)
          .catch((e) => {
            setPairErrs((p) => ({ ...p, [m.run_id]: e instanceof Error ? e.message : "fail" }));
            return [m.run_id, null] as const;
          }),
      ),
    )
      .then((results) => {
        const map: Record<string, ComparisonResponse | null> = {};
        for (const [k, v] of results) map[k] = v;
        setPairs(map);
      })
      .finally(() => setPairLoading(false));
  }, [anchor?.run_id, others.map((o) => o.run_id).join(",")]);

  // Build merged metric rows. Key = metric key. Each row gets:
  //   anchor: avg_a from any pair (all should match)
  //   per-other: avg_b + diff_avg_pct + verdict
  const merged = useMemo(() => {
    const out: Record<string, {
      key: string; name: string; unit: string;
      anchorAvg: number;
      perOther: Record<string, { avg: number; diff: number; verdict: string } | null>;
    }> = {};
    for (const [otherID, resp] of Object.entries(pairs)) {
      if (!resp) continue;
      for (const r of resp.metrics) {
        const row = out[r.key] || {
          key: r.key, name: r.name, unit: r.unit,
          anchorAvg: r.avg_a, perOther: {},
        };
        row.anchorAvg = r.avg_a;
        row.perOther[otherID] = { avg: r.avg_b, diff: r.diff_avg_pct, verdict: r.verdict };
        out[r.key] = row;
      }
    }
    return Object.values(out).sort((a, b) => a.name.localeCompare(b.name));
  }, [pairs]);

  if (!anchor) {
    return (
      <div className="border border-zinc-800/60 bg-[#070707] p-3">
        <div className="text-[11px] font-mono text-zinc-300 mb-1.5">
          {group.group_label}
          <span className="text-zinc-700 ml-2">({group.members.length} runs)</span>
        </div>
        <div className="text-[11px] font-mono text-amber-500/80">
          No finished runs — nothing to compare.
        </div>
      </div>
    );
  }

  const sub = (m: SuiteCrossMember) =>
    pivot === "item"
      ? m.preset_name || m.db_preset_id?.slice(0, 6) || "—"
      : m.item_name || "—";

  return (
    <div className="border border-zinc-800/60 bg-[#070707]">
      <div className="px-3 py-2 border-b border-zinc-800/50 flex items-center gap-2">
        <span className="text-xs font-mono font-semibold text-zinc-200">{group.group_label}</span>
        <span className="text-[10px] font-mono text-zinc-700">{finishedMembers.length}/{group.members.length} finished</span>
        {failed.length > 0 && (
          <span className="text-[10px] font-mono text-red-400/70">
            ({failed.map((f) => `${sub(f)}: ${f.state}`).join(", ")})
          </span>
        )}
        {pairLoading && <span className="text-[10px] font-mono text-zinc-500 ml-auto">collecting metrics…</span>}
      </div>

      {others.length === 0 ? (
        <div className="px-3 py-4 text-[11px] font-mono text-zinc-500">
          Only one finished run — nothing to diff against.
          <Link to={`/runs/${anchor.run_id}/metrics`} className="text-primary hover:underline ml-2">
            view metrics →
          </Link>
        </div>
      ) : merged.length === 0 ? (
        <div className="px-3 py-4 text-[11px] font-mono text-zinc-500">
          {pairLoading ? "collecting metrics…" : "No comparison data available yet."}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-xs font-mono">
            <thead>
              <tr className="border-b border-zinc-800/60 text-left text-[10px] uppercase tracking-wider text-zinc-500 bg-zinc-900/30">
                <th className="py-2 px-3">Metric</th>
                <th className="py-2 px-3 text-right">
                  <span className="text-cyan-400">{sub(anchor)}</span>
                  <span className="text-zinc-700 ml-1">(anchor)</span>
                </th>
                {others.map((m) => (
                  <th key={m.run_id} className="py-2 px-3 text-right">
                    <span className="text-amber-400">{sub(m)}</span>
                    {pairErrs[m.run_id] && (
                      <span className="text-red-400 ml-1" title={pairErrs[m.run_id]}>err</span>
                    )}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {merged.map((row) => (
                <tr key={row.key} className="border-b border-zinc-800/40 hover:bg-zinc-900/30">
                  <td className="py-1.5 px-3">
                    <div className="text-zinc-300">{row.name}</div>
                    <div className="text-[9px] text-zinc-700">{row.key}</div>
                  </td>
                  <td className="py-1.5 px-3 text-right text-zinc-300 tabular-nums">
                    {formatMetric(row.anchorAvg, row.unit)}
                  </td>
                  {others.map((m) => {
                    const d = row.perOther[m.run_id];
                    if (!d) return <td key={m.run_id} className="py-1.5 px-3 text-right text-zinc-700">—</td>;
                    const cls =
                      d.verdict === "better" ? "text-emerald-400" :
                      d.verdict === "worse" ? "text-red-400" :
                      "text-zinc-500";
                    return (
                      <td key={m.run_id} className="py-1.5 px-3 text-right tabular-nums">
                        <div className="text-zinc-300">{formatMetric(d.avg, row.unit)}</div>
                        <div className={`text-[10px] ${cls}`}>
                          {d.diff > 0 ? "+" : ""}{d.diff.toFixed(1)}%
                        </div>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function formatMetric(v: number, unit: string): string {
  if (unit === "%" || unit === "percent") return `${v.toFixed(1)}%`;
  if (unit === "ms") return v >= 1000 ? `${(v / 1000).toFixed(2)}s` : `${v.toFixed(1)}ms`;
  if (unit === "bytes/s" || unit === "B/s") {
    if (v >= 1e9) return `${(v / 1e9).toFixed(1)}GB/s`;
    if (v >= 1e6) return `${(v / 1e6).toFixed(1)}MB/s`;
    if (v >= 1e3) return `${(v / 1e3).toFixed(1)}KB/s`;
    return `${v.toFixed(0)}B/s`;
  }
  if (Math.abs(v) >= 1e6) return `${(v / 1e6).toFixed(1)}M`;
  if (Math.abs(v) >= 1e3) return `${(v / 1e3).toFixed(1)}K`;
  if (Number.isInteger(v)) return String(v);
  return v.toFixed(2);
}

export default SuiteDetail;

// Pause icon used inline in the header — re-exported just so tree-shake
// keeps it linked when the icon-only path runs first.
export { Database };
