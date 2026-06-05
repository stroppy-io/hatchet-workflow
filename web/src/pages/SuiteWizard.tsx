// Suite Wizard — build a db x workload MATRIX suite through a draft-based flow.
//
// This is the SUITE analogue of NewRun.tsx (the test wizard). It is wired to the
// SuiteWizardProvider (services/suiteWizard.ts), which exercises the six
// SuiteWizardService RPCs: StartSuiteWizard, GetSuiteWizardDraft,
// ListSuiteWizardDrafts, PatchSuiteWizard, DeleteSuiteWizardDraft and
// FinishSuiteWizard. Every cell add/edit/remove + every suite-level setting
// (provider/max_parallel/schedule/rating) is a typed PatchSuiteWizardRequest; the
// server resolves each cell's source (preset_pair | test_preset | inline_test)
// into a database/workload/topology preview and recomputes per-cell + draft
// readiness. Finish persists a SuiteRecord (and optionally launches a SuiteRun).
//
// Layout: a start/resume screen (List/Start/Delete) when no draft is active,
// then a single editor view (?draft=) with the suite settings rail on the left
// and the cell matrix + live preview on the right.

import { useCallback, useEffect, useState } from "react";
import { useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import {
  getSuiteWizardProvider,
  Provider,
  type SuiteWizardDraftVM,
  type SuiteDraftSummaryVM,
  type SuiteCellVM,
  type SuitePatchInput,
} from "@/services/suiteWizard";
import {
  getPresetProvider,
  type DatabasePresetVM,
  type WorkloadPresetVM,
} from "@/services/preset";
import type { SuiteCellInput } from "@/services/suites";
import { ENGINES, type EngineKind } from "@/services/wizard";
import { dbKindProto } from "@/services/enums";
import { Database_Kind } from "@/lib/proto/cloud/v1/domain/database_pb";
import type { DbKind } from "@/services/runs";
import {
  MachinePlanEditor,
  machineSpecSummary,
} from "@/components/wizard/MachinePlanEditor";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  AlertCircle,
  Check,
  Cloud,
  Container,
  Grid3x3,
  History,
  Loader2,
  Plus,
  Rocket,
  Save,
  Trash2,
  X,
  Zap,
} from "lucide-react";

const PROVIDERS: { provider: Provider; label: string; icon: typeof Container; blurb: string }[] = [
  { provider: Provider.DOCKER, label: "Docker", icon: Container, blurb: "Local containers — fast smoke matrices." },
  { provider: Provider.YANDEX, label: "Yandex Cloud", icon: Cloud, blurb: "Provisions VMs via Terraform per cell." },
];

// The engine kinds offered for an inline cell's matrix ROW axis (Database_Kind).
const DB_KINDS: { kind: DbKind; label: string }[] = ENGINES.map((e) => ({
  kind: ENGINE_TO_DBKIND(e.kind),
  label: e.label,
})).filter((x) => x.kind !== "");

function ENGINE_TO_DBKIND(kind: EngineKind): DbKind {
  const proto = {
    postgres: Database_Kind.POSTGRES,
    mysql: Database_Kind.MYSQL,
    mariadb: Database_Kind.MARIADB,
    picodata: Database_Kind.PICODATA,
    ydb: Database_Kind.YDB,
    ydbManaged: Database_Kind.YDB_MANAGED,
    cockroach: Database_Kind.COCKROACH,
    external: Database_Kind.EXTERNAL,
  }[kind];
  // Round-trip through the enum map to get the lower-cased UI label.
  const labels: Partial<Record<Database_Kind, DbKind>> = {
    [Database_Kind.POSTGRES]: "postgres",
    [Database_Kind.MYSQL]: "mysql",
    [Database_Kind.MARIADB]: "mariadb",
    [Database_Kind.PICODATA]: "picodata",
    [Database_Kind.YDB]: "ydb",
    [Database_Kind.YDB_MANAGED]: "ydb_managed",
    [Database_Kind.COCKROACH]: "cockroach",
    [Database_Kind.EXTERNAL]: "external",
  };
  return labels[proto] ?? "";
}

export function SuiteWizard() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();

  const draftId = params.get("draft") ?? "";

  const [draft, setDraft] = useState<SuiteWizardDraftVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [patching, setPatching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [resume, setResume] = useState<SuiteDraftSummaryVM[]>([]);
  const [startName, setStartName] = useState("");

  useBreadcrumbLabel("draft", draft ? draft.name || "New Suite" : undefined);

  // Resume list (List) — shown on the start screen.
  useEffect(() => {
    if (draftId || !slug) return;
    let cancelled = false;
    getSuiteWizardProvider()
      .list(slug)
      .then((d) => !cancelled && setResume(d))
      .catch(() => !cancelled && setResume([]));
    return () => {
      cancelled = true;
    };
  }, [draftId, slug]);

  // Load the active draft (Get).
  useEffect(() => {
    if (!draftId || !slug) {
      setDraft(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    getSuiteWizardProvider()
      .get(slug, draftId)
      .then((d) => {
        if (cancelled) return;
        setDraft(d);
        setError(null);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
        setDraft(null);
      })
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [draftId, slug]);

  const setActiveDraft = useCallback(
    (id: string) => setParams({ draft: id }),
    [setParams],
  );

  const handleStart = useCallback(async () => {
    if (!slug) return;
    setError(null);
    setLoading(true);
    try {
      const d = await getSuiteWizardProvider().start(slug, startName.trim() || "Untitled suite");
      setDraft(d);
      setActiveDraft(d.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [slug, startName, setActiveDraft]);

  const patch = useCallback(
    async (input: SuitePatchInput) => {
      if (!slug || !draftId) return;
      setPatching(true);
      setError(null);
      try {
        const d = await getSuiteWizardProvider().patch(slug, draftId, input);
        setDraft(d);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setPatching(false);
      }
    },
    [slug, draftId],
  );

  const deleteDraft = useCallback(
    async (id: string) => {
      await getSuiteWizardProvider().remove(slug, id);
      setResume((prev) => prev.filter((d) => d.id !== id));
    },
    [slug],
  );

  const finish = useCallback(
    async (start: boolean) => {
      if (!slug || !draft) return;
      setPatching(true);
      setError(null);
      try {
        const res = await getSuiteWizardProvider().finish(slug, draft.id, {
          start,
          suiteName: draft.name,
          inTenantRating: draft.defaultInTenantRating,
          inGlobalRating: draft.defaultInGlobalRating,
        });
        if (res.suiteId) navigate(`/suites/${res.suiteId}`);
        else navigate("/suites");
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setPatching(false);
      }
    },
    [slug, draft, navigate],
  );

  if (!draftId) {
    return (
      <SuiteStartScreen
        startName={startName}
        setStartName={setStartName}
        onStart={handleStart}
        loading={loading}
        error={error}
        resume={resume}
        onResume={setActiveDraft}
        onDelete={deleteDraft}
      />
    );
  }

  if (loading && !draft) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading draft…
      </div>
    );
  }

  if (!draft) {
    return (
      <div className="p-8">
        <div className="flex items-center gap-2 text-sm text-red-400">
          <AlertCircle className="h-4 w-4" /> {error ?? "Draft not found."}
        </div>
        <Button variant="outline" size="sm" className="mt-4" onClick={() => setParams({})}>
          Back to start
        </Button>
      </div>
    );
  }

  return (
    <SuiteEditor
      slug={slug}
      draft={draft}
      patching={patching}
      error={error}
      onPatch={patch}
      onFinish={finish}
    />
  );
}

// ─── Start / Resume screen ─────────────────────────────────────────────────────

function SuiteStartScreen({
  startName,
  setStartName,
  onStart,
  loading,
  error,
  resume,
  onResume,
  onDelete,
}: {
  startName: string;
  setStartName: (s: string) => void;
  onStart: () => void;
  loading: boolean;
  error: string | null;
  resume: SuiteDraftSummaryVM[];
  onResume: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <div className="mx-auto max-w-2xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Suite Wizard
      </div>
      <h1 className="text-2xl font-semibold tracking-tight text-foreground">New suite</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Build a benchmark suite — a matrix of cells (each a database × workload preset pair, a test
        preset, or an inline engine), one deployment provider, and an optional cron schedule. The
        server validates each cell after every edit; finish to persist (and optionally launch) it.
      </p>

      {error && (
        <div className="mt-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-sm text-red-400">
          <AlertCircle className="h-4 w-4" /> {error}
        </div>
      )}

      <div className="mt-6 border border-zinc-800 bg-[#0a0a0a] p-5">
        <Label>Suite name</Label>
        <div className="mt-1 flex gap-2">
          <Input
            placeholder="e.g. nightly oltp matrix"
            value={startName}
            onChange={(e) => setStartName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") onStart();
            }}
          />
          <Button onClick={onStart} disabled={loading} className="gap-1.5">
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Zap className="h-4 w-4" />}
            Start
          </Button>
        </div>
        <p className="mt-2 text-[11px] text-zinc-600">
          Creates a draft (StartSuiteWizard). You can rename it later.
        </p>
      </div>

      {resume.length > 0 && (
        <div className="mt-8">
          <div className="mb-2 flex items-center gap-2 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
            <History className="h-3 w-3" /> Continue a draft
          </div>
          <div className="divide-y divide-zinc-800/60 border border-zinc-800">
            {resume.map((d) => (
              <div key={d.id} className="flex items-center gap-3 px-4 py-3">
                <div className="min-w-0 flex-1">
                  <div className="truncate text-sm text-foreground">{d.name || "Untitled suite"}</div>
                  <div className="mt-0.5 flex items-center gap-2 text-[11px] text-zinc-600">
                    <span className="font-mono">{d.cellCount} cell{d.cellCount === 1 ? "" : "s"}</span>
                    <span>·</span>
                    <span className="font-mono">{d.provider || "no provider"}</span>
                    <span>·</span>
                    <span>{d.updatedAt ? new Date(d.updatedAt).toLocaleString() : ""}</span>
                    {d.ready && (
                      <span className="inline-flex items-center gap-0.5 text-emerald-400">
                        <Check className="h-3 w-3" /> ready
                      </span>
                    )}
                  </div>
                </div>
                <Button size="sm" variant="outline" onClick={() => onResume(d.id)}>
                  Resume
                </Button>
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 text-zinc-500 hover:text-red-400"
                  onClick={() => onDelete(d.id)}
                  title="Delete draft"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Editor ─────────────────────────────────────────────────────────────────────

function SuiteEditor({
  slug,
  draft,
  patching,
  error,
  onPatch,
  onFinish,
}: {
  slug: string;
  draft: SuiteWizardDraftVM;
  patching: boolean;
  error: string | null;
  onPatch: (input: SuitePatchInput) => Promise<void>;
  onFinish: (start: boolean) => void;
}) {
  const errCount = draft.errors.filter((e) => e.severity === "error").length;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Header */}
      <div className="flex shrink-0 items-center gap-3 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <Grid3x3 className="h-4 w-4 text-primary" />
        <div className="min-w-0">
          <div className="truncate font-mono text-sm text-zinc-200">{draft.name || "Untitled suite"}</div>
          <div className="font-mono text-[10px] text-zinc-700">draft {draft.id}</div>
        </div>
        <div className="ml-auto flex items-center gap-2">
          {patching && <Loader2 className="h-3.5 w-3.5 animate-spin text-zinc-500" />}
          {errCount > 0 && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-red-400">
              <AlertCircle className="h-3 w-3" /> {errCount} error{errCount > 1 ? "s" : ""}
            </span>
          )}
          <span
            className={`inline-flex items-center gap-1 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
              draft.ready ? "bg-emerald-500/10 text-emerald-400" : "bg-zinc-800/60 text-zinc-500"
            }`}
          >
            {draft.ready ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
            {draft.ready ? "Ready" : "Incomplete"}
          </span>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6 lg:px-8">
        {error && (
          <div className="mb-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-sm text-red-400">
            <AlertCircle className="h-4 w-4" /> {error}
          </div>
        )}

        <div className="grid gap-6 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
          <SuiteSettingsRail draft={draft} onPatch={onPatch} />
          <CellMatrix slug={slug} draft={draft} onPatch={onPatch} />
        </div>

        {draft.errors.length > 0 && (
          <div className="mt-6 space-y-1">
            {draft.errors.map((e, i) => (
              <div
                key={`${e.field}-${i}`}
                className={`flex items-start gap-2 font-mono text-[11px] ${
                  e.severity === "error" ? "text-red-400" : "text-amber-400"
                }`}
              >
                <AlertCircle className="mt-0.5 h-3 w-3 shrink-0" />
                <span className="text-zinc-500">{e.field}</span>
                <span>{e.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Finish footer */}
      <div className="flex shrink-0 items-center justify-end gap-2 border-t border-zinc-800 bg-[#070707] px-6 py-3 lg:px-8">
        <Button variant="outline" size="sm" disabled={!draft.ready || patching} onClick={() => onFinish(false)}>
          <Save className="h-3.5 w-3.5" /> Save suite
        </Button>
        <Button size="sm" className="gap-1.5" disabled={!draft.ready || patching} onClick={() => onFinish(true)}>
          <Rocket className="h-3.5 w-3.5" /> Save &amp; run
        </Button>
      </div>
    </div>
  );
}

// ─── Suite settings rail (provider + schedule + max_parallel + ratings) ──────────

function SuiteSettingsRail({
  draft,
  onPatch,
}: {
  draft: SuiteWizardDraftVM;
  onPatch: (input: SuitePatchInput) => Promise<void>;
}) {
  const [cron, setCron] = useState(draft.cron);
  const [timezone, setTimezone] = useState(draft.timezone);
  useEffect(() => setCron(draft.cron), [draft.cron]);
  useEffect(() => setTimezone(draft.timezone), [draft.timezone]);

  return (
    <div className="space-y-5">
      {/* Provider */}
      <div>
        <div className="mb-2 text-[10px] font-mono uppercase tracking-wider text-zinc-600">Provider</div>
        <div className="grid gap-2">
          {PROVIDERS.map((p) => {
            const active = draft.provider === p.provider;
            const Icon = p.icon;
            return (
              <button
                key={p.provider}
                type="button"
                onClick={() => active || void onPatch({ provider: p.provider })}
                className={`flex items-center gap-3 border p-3 text-left transition-all ${
                  active ? "border-primary/40 bg-primary/[0.06] text-primary" : "border-zinc-800/60 hover:border-zinc-700 hover:bg-zinc-900/50"
                }`}
              >
                <Icon className={`h-4 w-4 shrink-0 ${active ? "text-primary" : "text-zinc-600"}`} />
                <div>
                  <div className={`text-xs font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{p.label}</div>
                  <div className="text-[10px] text-zinc-600">{p.blurb}</div>
                </div>
                {active && <Check className="ml-auto h-4 w-4 text-primary" />}
              </button>
            );
          })}
        </div>
      </div>

      {/* Concurrency */}
      <div>
        <Label className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Max parallel (0 = unlimited)</Label>
        <Input
          type="number"
          min={0}
          className="mt-1 h-8 font-mono text-xs"
          value={draft.maxParallel}
          onChange={(e) => {
            const v = Math.max(0, Number.parseInt(e.target.value, 10) || 0);
            void onPatch({ maxParallel: v });
          }}
        />
      </div>

      {/* Schedule */}
      <div className="space-y-2 border border-zinc-800/60 bg-[#070707] p-3">
        <div className="flex items-center justify-between">
          <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Schedule</span>
          <button
            type="button"
            onClick={() =>
              void onPatch({ schedule: { enabled: !draft.scheduleEnabled, cron, timezone } })
            }
            className={`px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
              draft.scheduleEnabled ? "bg-emerald-500/10 text-emerald-400" : "bg-zinc-800/60 text-zinc-500"
            }`}
          >
            {draft.scheduleEnabled ? "Enabled" : "Disabled"}
          </button>
        </div>
        <div>
          <Label className="text-[9px] font-mono text-zinc-600">cron</Label>
          <Input
            className="mt-1 h-7 font-mono text-[11px]"
            value={cron}
            placeholder="0 2 * * *"
            onChange={(e) => setCron(e.target.value)}
            onBlur={() => void onPatch({ schedule: { enabled: draft.scheduleEnabled, cron, timezone } })}
          />
        </div>
        <div>
          <Label className="text-[9px] font-mono text-zinc-600">timezone</Label>
          <Input
            className="mt-1 h-7 font-mono text-[11px]"
            value={timezone}
            placeholder="UTC"
            onChange={(e) => setTimezone(e.target.value)}
            onBlur={() => void onPatch({ schedule: { enabled: draft.scheduleEnabled, cron, timezone } })}
          />
        </div>
      </div>

      {/* Rating defaults */}
      <div className="space-y-1.5">
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Rating defaults</div>
        <RatingToggle
          label="In tenant rating"
          value={draft.defaultInTenantRating}
          onChange={(v) => void onPatch({ defaultInTenantRating: v })}
        />
        <RatingToggle
          label="In global rating"
          value={draft.defaultInGlobalRating}
          onChange={(v) => void onPatch({ defaultInGlobalRating: v })}
        />
      </div>
    </div>
  );
}

function RatingToggle({
  label,
  value,
  onChange,
}: {
  label: string;
  value: boolean | undefined;
  onChange: (v: boolean) => void;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!value)}
      className="flex w-full items-center gap-2 text-[11px] font-mono text-zinc-400"
    >
      <span
        className={`flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border ${
          value ? "border-primary bg-primary" : "border-zinc-700"
        }`}
      >
        {value && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
      </span>
      {label}
    </button>
  );
}

// ─── Cell matrix ─────────────────────────────────────────────────────────────────

function CellMatrix({
  slug,
  draft,
  onPatch,
}: {
  slug: string;
  draft: SuiteWizardDraftVM;
  onPatch: (input: SuitePatchInput) => Promise<void>;
}) {
  const [adding, setAdding] = useState(false);

  const addCell = async (cell: SuiteCellInput) => {
    setAdding(false);
    await onPatch({ cells: [{ cellId: "", cell }] });
  };
  const confirmAllMachinePlans = async () => {
    const cells = draft.cells
      .filter((c) => c.infrastructurePlan.machines.length > 0)
      .map((c) => ({
        cellId: c.id,
        machineOverrides: c.infrastructurePlan.machines,
      }));
    if (cells.length === 0) return;
    await onPatch({ cells });
  };
  const setCellMachine = (cell: SuiteCellVM, nodeId: string, spec: SuiteCellVM["infrastructurePlan"]["machines"][number]["spec"]) => {
    const machineOverrides = cell.infrastructurePlan.machines.map((m) =>
      m.nodeId === nodeId ? { ...m, spec } : m,
    );
    void onPatch({ cells: [{ cellId: cell.id, machineOverrides }] });
  };
  const hasMachinePlans = draft.cells.some((c) => c.infrastructurePlan.machines.length > 0);

  return (
    <div className="flex min-h-0 flex-col">
      <div className="mb-2 flex items-center justify-between">
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
          Matrix — {draft.cells.length} cell{draft.cells.length === 1 ? "" : "s"}
        </div>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            className="h-7 gap-1.5"
            disabled={!hasMachinePlans}
            onClick={confirmAllMachinePlans}
          >
            <Check className="h-3.5 w-3.5" /> Confirm all machines
          </Button>
          <Button size="sm" variant="outline" className="h-7 gap-1.5" onClick={() => setAdding((v) => !v)}>
            <Plus className="h-3.5 w-3.5" /> Add cell
          </Button>
        </div>
      </div>

      {adding && <CellComposer slug={slug} onAdd={addCell} onCancel={() => setAdding(false)} />}

      {draft.cells.length === 0 && !adding ? (
        <div className="border border-zinc-800 bg-[#0a0a0a] p-6 text-center text-[12px] text-zinc-600">
          No cells yet. Add a database × workload preset pair, a test preset, or an inline engine.
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {draft.cells.map((c, i) => (
            <CellCard
              key={c.id || `local-${i}`}
              cell={c}
              onToggle={(enabled) => void onPatch({ cells: [{ cellId: c.id, enabled }] })}
              onRemove={() => void onPatch({ cells: [{ cellId: c.id, remove: true }] })}
              onConfirmMachines={() => void onPatch({ cells: [{ cellId: c.id, machineOverrides: c.infrastructurePlan.machines }] })}
              onMachineChange={(nodeId, spec) => setCellMachine(c, nodeId, spec)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function CellCard({
  cell,
  onToggle,
  onRemove,
  onConfirmMachines,
  onMachineChange,
}: {
  cell: SuiteCellVM;
  onToggle: (enabled: boolean) => void;
  onRemove: () => void;
  onConfirmMachines: () => void;
  onMachineChange: (nodeId: string, spec: SuiteCellVM["infrastructurePlan"]["machines"][number]["spec"]) => void;
}) {
  const sourceLabel =
    cell.source === "presetPair"
      ? `${cell.dbPresetId || "?"} / ${cell.workloadPresetId || "?"}`
      : cell.source === "testPreset"
        ? cell.testPresetId || "?"
        : "inline";
  return (
    <div
      className={`border bg-[#0a0a0a] p-3 ${cell.enabled ? "border-zinc-800" : "border-zinc-800/40 opacity-60"}`}
    >
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => onToggle(!cell.enabled)}
          className={`flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-sm border ${
            cell.enabled ? "border-primary bg-primary" : "border-zinc-700"
          }`}
          title={cell.enabled ? "Disable cell" : "Enable cell"}
        >
          {cell.enabled && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
        </button>
        <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-zinc-300">
          {cell.name || cell.id || sourceLabel}
        </span>
        {cell.ready ? (
          <Check className="h-3.5 w-3.5 shrink-0 text-emerald-400" />
        ) : (
          <AlertCircle className="h-3.5 w-3.5 shrink-0 text-zinc-600" />
        )}
        <button
          type="button"
          onClick={onRemove}
          className="shrink-0 text-zinc-600 hover:text-red-400"
          title="Remove cell"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      </div>
      <div className="mt-2 grid grid-cols-2 gap-1 font-mono text-[10px] text-zinc-500">
        <span>db: {cell.dbKind || "—"}</span>
        <span>wl: {cell.workload || "—"}</span>
        <span>{cell.source}</span>
        <span>
          {cell.nodeCount} node{cell.nodeCount === 1 ? "" : "s"} · {cell.machineOverrideCount} confirmed
        </span>
      </div>
      <div className="mt-1 truncate font-mono text-[10px] text-zinc-700" title={sourceLabel}>
        {sourceLabel}
      </div>
      {cell.infrastructurePlan.machines.length > 0 && (
        <div className="mt-3 border-t border-zinc-800/70 pt-3">
          <div className="mb-2 flex items-center justify-between gap-2">
            <div className="min-w-0">
              <div className="font-mono text-[10px] uppercase tracking-wider text-zinc-600">Machines</div>
              <div className="truncate font-mono text-[10px] text-zinc-700">
                {machineSpecSummary(cell.infrastructurePlan.machines[0]?.spec)}
              </div>
            </div>
            <Button size="sm" variant="outline" className="h-7 shrink-0 gap-1.5" onClick={onConfirmMachines}>
              <Check className="h-3.5 w-3.5" /> Confirm
            </Button>
          </div>
          <MachinePlanEditor
            machines={cell.infrastructurePlan.machines}
            settings={cell.infrastructurePlan.settings}
            onMachineChange={onMachineChange}
          />
        </div>
      )}
      {cell.errors.filter((e) => e.severity === "error").length > 0 && (
        <div className="mt-1 font-mono text-[10px] text-red-400">
          {cell.errors.filter((e) => e.severity === "error")[0].message}
        </div>
      )}
    </div>
  );
}

// ─── Cell composer (add a new cell) ──────────────────────────────────────────────

type CellMode = "presetPair" | "inline";

function CellComposer({
  slug,
  onAdd,
  onCancel,
}: {
  slug: string;
  onAdd: (cell: SuiteCellInput) => void;
  onCancel: () => void;
}) {
  const [mode, setMode] = useState<CellMode>("presetPair");
  const [engine, setEngine] = useState<EngineKind>("postgres");
  const [dbPresets, setDbPresets] = useState<DatabasePresetVM[]>([]);
  const [wlPresets, setWlPresets] = useState<WorkloadPresetVM[]>([]);
  const [dbPresetId, setDbPresetId] = useState("");
  const [workloadPresetId, setWorkloadPresetId] = useState("");
  const [name, setName] = useState("");
  const [inlineDbKind, setInlineDbKind] = useState<DbKind>("postgres");

  // Load the db presets for the chosen engine + the (engine-agnostic) workloads.
  useEffect(() => {
    let cancelled = false;
    getPresetProvider()
      .listDatabasePresets(slug, engine)
      .then((p) => {
        if (cancelled) return;
        setDbPresets(p);
        setDbPresetId("");
      })
      .catch(() => !cancelled && setDbPresets([]));
    return () => {
      cancelled = true;
    };
  }, [slug, engine]);

  useEffect(() => {
    let cancelled = false;
    getPresetProvider()
      .listWorkloadPresets(slug)
      .then((p) => {
        if (cancelled) return;
        setWlPresets(p);
        setWorkloadPresetId("");
      })
      .catch(() => !cancelled && setWlPresets([]));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const canAdd =
    mode === "presetPair"
      ? !!dbPresetId && !!workloadPresetId
      : dbKindProto(inlineDbKind) !== Database_Kind.UNSPECIFIED;

  const submit = () => {
    if (mode === "presetPair") {
      onAdd({ name, enabled: true, source: "presetPair", dbPresetId, workloadPresetId });
    } else {
      onAdd({ name, enabled: true, source: "inline", inlineDbKind });
    }
  };

  return (
    <div className="mb-3 border border-primary/30 bg-primary/[0.03] p-3">
      <div className="mb-3 inline-flex border border-zinc-800 text-[11px] font-mono">
        {(["presetPair", "inline"] as CellMode[]).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMode(m)}
            className={`px-3 py-1 transition-colors ${
              mode === m ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {m === "presetPair" ? "Preset pair" : "Inline engine"}
          </button>
        ))}
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <Label className="text-[9px] font-mono text-zinc-600">Cell name (optional)</Label>
          <Input
            className="mt-1 h-7 font-mono text-[11px]"
            value={name}
            placeholder="server derives if empty"
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        <div>
          <Label className="text-[9px] font-mono text-zinc-600">Engine</Label>
          <Select value={engine} onValueChange={(v) => setEngine(v as EngineKind)}>
            <SelectTrigger className="mt-1 h-7 text-[11px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ENGINES.map((e) => (
                <SelectItem key={e.kind} value={e.kind}>
                  {e.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {mode === "presetPair" ? (
          <>
            <div>
              <Label className="text-[9px] font-mono text-zinc-600">Database preset</Label>
              <Select value={dbPresetId} onValueChange={setDbPresetId}>
                <SelectTrigger className="mt-1 h-7 text-[11px]">
                  <SelectValue placeholder="select…" />
                </SelectTrigger>
                <SelectContent>
                  {dbPresets.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-[9px] font-mono text-zinc-600">Workload preset</Label>
              <Select value={workloadPresetId} onValueChange={setWorkloadPresetId}>
                <SelectTrigger className="mt-1 h-7 text-[11px]">
                  <SelectValue placeholder="select…" />
                </SelectTrigger>
                <SelectContent>
                  {wlPresets.map((p) => (
                    <SelectItem key={p.id} value={p.id}>
                      {p.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </>
        ) : (
          <div>
            <Label className="text-[9px] font-mono text-zinc-600">Inline DB kind</Label>
            <Select value={inlineDbKind || "postgres"} onValueChange={(v) => setInlineDbKind(v as DbKind)}>
              <SelectTrigger className="mt-1 h-7 text-[11px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DB_KINDS.map((k) => (
                  <SelectItem key={k.kind} value={k.kind}>
                    {k.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      <div className="mt-3 flex items-center justify-end gap-2">
        <Button size="sm" variant="ghost" className="h-7" onClick={onCancel}>
          Cancel
        </Button>
        <Button size="sm" className="h-7 gap-1.5" disabled={!canAdd} onClick={submit}>
          <Plus className="h-3.5 w-3.5" /> Add
        </Button>
      </div>
    </div>
  );
}
