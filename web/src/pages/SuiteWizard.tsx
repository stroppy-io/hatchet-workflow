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

import { useCallback, useEffect, useMemo, useState } from "react";
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
  type TestPresetRow,
  type WorkloadPresetVM,
} from "@/services/preset";
import type { SuiteCellInput } from "@/services/suites";
import { ENGINES, type EngineKind, type MachineSpecVM } from "@/services/wizard";
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
  ChevronLeft,
  ChevronRight,
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

type SuiteStepKey = "cells" | "machines" | "settings";

const SUITE_STEPS: { key: SuiteStepKey; label: string; hint: string }[] = [
  { key: "cells", label: "Cells", hint: "what to run" },
  { key: "machines", label: "Machines", hint: "where it runs" },
  { key: "settings", label: "Options", hint: "schedule and save" },
];

const PROVIDERS: { provider: Provider; label: string; icon: typeof Container; blurb: string }[] = [
  { provider: Provider.DOCKER, label: "Docker", icon: Container, blurb: "Local containers — fast smoke matrices." },
  { provider: Provider.YANDEX, label: "Yandex Cloud", icon: Cloud, blurb: "Provisions VMs via Terraform per cell." },
];

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
        Build a benchmark suite from database and workload presets, choose one deployment provider,
        size the provider machines for each cell, and optionally add a cron schedule.
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
          The suite stays editable until you save or launch it.
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

// ─── Wizard state helpers ─────────────────────────────────────────────────────

function cellTitle(cell: SuiteCellVM): string {
  if (cell.name.trim()) return cell.name;
  if (cell.source === "presetPair") {
    return `${cell.dbKind || "database"} / ${cell.workload || "workload"}`;
  }
  if (cell.source === "testPreset") return cell.testPresetId || "Test preset";
  return cell.id || "Suite cell";
}

function sourceLabel(cell: SuiteCellVM): string {
  if (cell.source === "presetPair") return "Database + workload presets";
  if (cell.source === "testPreset") return "Test preset";
  return "Custom test";
}

function cellSourceValue(cell: SuiteCellVM): string {
  if (cell.source === "presetPair") return `${cell.dbPresetId || "?"} / ${cell.workloadPresetId || "?"}`;
  if (cell.source === "testPreset") return cell.testPresetId || "?";
  return "custom";
}

function cellsWithMachines(draft: SuiteWizardDraftVM): SuiteCellVM[] {
  return draft.cells.filter((c) => c.enabled && c.infrastructurePlan.machines.length > 0);
}

function cellsNeedingMachineConfirmation(draft: SuiteWizardDraftVM): SuiteCellVM[] {
  return cellsWithMachines(draft).filter(
    (c) => c.machineOverrideCount < c.infrastructurePlan.machines.length,
  );
}

function friendlySuiteError(message: string): string {
  if (message.toLowerCase().includes("confirm machine settings")) {
    return "Review and confirm provider machine settings.";
  }
  return message;
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
  const [step, setStep] = useState<SuiteStepKey>("cells");
  const errCount = draft.errors.filter((e) => e.severity === "error").length;
  const machineCells = useMemo(() => cellsWithMachines(draft), [draft]);
  const unconfirmedMachineCells = useMemo(() => cellsNeedingMachineConfirmation(draft), [draft]);
  const hasMachinePlans = machineCells.length > 0;
  const stepIndex = Math.max(0, SUITE_STEPS.findIndex((s) => s.key === step));
  const prevStep = stepIndex > 0 ? SUITE_STEPS[stepIndex - 1].key : null;
  const nextStep = stepIndex < SUITE_STEPS.length - 1 ? SUITE_STEPS[stepIndex + 1].key : null;
  const confirmAllMachinePlans = useCallback(async () => {
    const cells = machineCells.map((c) => ({
      cellId: c.id,
      machineOverrides: c.infrastructurePlan.machines,
    }));
    if (cells.length === 0) return;
    await onPatch({ cells });
  }, [machineCells, onPatch]);
  const blocker =
    draft.cells.length === 0
      ? "Add at least one suite cell."
      : unconfirmedMachineCells.length > 0
        ? "Confirm provider machine settings."
        : errCount > 0
          ? "Resolve validation errors."
          : "";

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Header */}
      <div className="flex shrink-0 items-center gap-3 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <Grid3x3 className="h-4 w-4 text-primary" />
        <div className="min-w-0">
          <div className="truncate font-mono text-sm text-zinc-200">{draft.name || "Untitled suite"}</div>
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

      <SuiteStepNav
        active={step}
        draft={draft}
        unconfirmedMachineCells={unconfirmedMachineCells.length}
        onStep={setStep}
      />

      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6 lg:px-8">
        {error && (
          <div className="mb-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-sm text-red-400">
            <AlertCircle className="h-4 w-4" /> {error}
          </div>
        )}

        <SuiteFlow
          slug={slug}
          draft={draft}
          step={step}
          onStep={setStep}
          onPatch={onPatch}
          onConfirmAllMachines={confirmAllMachinePlans}
        />
      </div>

      {/* Finish footer */}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-zinc-800 bg-[#070707] px-6 py-3 lg:px-8">
        <div className="min-w-0 text-[11px] text-zinc-500">
          {draft.ready ? (
            <span className="inline-flex items-center gap-1.5 text-emerald-400">
              <Check className="h-3.5 w-3.5" /> Suite is ready to save or run.
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5">
              <AlertCircle className="h-3.5 w-3.5 text-amber-400" /> {blocker}
            </span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            disabled={!prevStep || patching}
            onClick={() => prevStep && setStep(prevStep)}
          >
            <ChevronLeft className="h-3.5 w-3.5" /> Back
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={!nextStep || patching}
            onClick={() => nextStep && setStep(nextStep)}
          >
            Next <ChevronRight className="h-3.5 w-3.5" />
          </Button>
          {hasMachinePlans && unconfirmedMachineCells.length > 0 && (
            <Button variant="outline" size="sm" disabled={patching} onClick={() => void confirmAllMachinePlans()}>
              <Check className="h-3.5 w-3.5" /> Confirm machine settings
            </Button>
          )}
          <Button variant="outline" size="sm" disabled={!draft.ready || patching} onClick={() => onFinish(false)}>
            <Save className="h-3.5 w-3.5" /> Save suite
          </Button>
          <Button size="sm" className="gap-1.5" disabled={!draft.ready || patching} onClick={() => onFinish(true)}>
            <Rocket className="h-3.5 w-3.5" /> Save &amp; run
          </Button>
        </div>
      </div>
    </div>
  );
}

function SuiteStepNav({
  active,
  draft,
  unconfirmedMachineCells,
  onStep,
}: {
  active: SuiteStepKey;
  draft: SuiteWizardDraftVM;
  unconfirmedMachineCells: number;
  onStep: (step: SuiteStepKey) => void;
}) {
  const statusFor = (key: SuiteStepKey): "done" | "attention" | "idle" => {
    if (key === "cells") return draft.cells.length > 0 ? "done" : "attention";
    if (key === "machines") {
      if (draft.cells.length === 0) return "idle";
      return unconfirmedMachineCells > 0 ? "attention" : "done";
    }
    return draft.ready ? "done" : "idle";
  };

  return (
    <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2">
      <div className="mx-auto grid max-w-5xl grid-cols-3 gap-2">
        {SUITE_STEPS.map((item, index) => {
          const isActive = active === item.key;
          const status = statusFor(item.key);
          return (
            <button
              key={item.key}
              type="button"
              onClick={() => onStep(item.key)}
              className={`min-w-0 border px-3 py-2 text-left transition-colors ${
                isActive
                  ? "border-primary/50 bg-primary/[0.07]"
                  : "border-zinc-800/70 hover:border-zinc-700 hover:bg-zinc-900/40"
              }`}
            >
              <div className="flex items-center gap-2">
                <span
                  className={`flex h-5 w-5 shrink-0 items-center justify-center border font-mono text-[10px] ${
                    isActive
                      ? "border-primary/40 text-primary"
                      : status === "done"
                        ? "border-emerald-500/40 text-emerald-400"
                        : status === "attention"
                          ? "border-amber-500/40 text-amber-300"
                          : "border-zinc-700 text-zinc-500"
                  }`}
                >
                  {status === "done" ? <Check className="h-3 w-3" /> : index + 1}
                </span>
                <div className="min-w-0">
                  <div className={`truncate text-xs font-medium ${isActive ? "text-primary" : "text-zinc-300"}`}>
                    {item.label}
                  </div>
                  <div className="truncate font-mono text-[9px] uppercase tracking-wider text-zinc-600">
                    {item.hint}
                  </div>
                </div>
              </div>
            </button>
          );
        })}
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

// ─── Main suite flow ─────────────────────────────────────────────────────────────

function SuiteFlow({
  slug,
  draft,
  step,
  onStep,
  onPatch,
  onConfirmAllMachines,
}: {
  slug: string;
  draft: SuiteWizardDraftVM;
  step: SuiteStepKey;
  onStep: (step: SuiteStepKey) => void;
  onPatch: (input: SuitePatchInput) => Promise<void>;
  onConfirmAllMachines: () => Promise<void>;
}) {
  const [adding, setAdding] = useState(false);

  const addCell = async (cell: SuiteCellInput) => {
    setAdding(false);
    await onPatch({ cells: [{ cellId: "", cell }] });
    onStep("machines");
  };
  const setCellMachine = (cell: SuiteCellVM, nodeId: string, spec: MachineSpecVM) => {
    const machineOverrides = cell.infrastructurePlan.machines.map((m) =>
      m.nodeId === nodeId ? { ...m, spec } : m,
    );
    void onPatch({ cells: [{ cellId: cell.id, machineOverrides }] });
  };
  const setCellMachines = (cell: SuiteCellVM, updates: { nodeId: string; spec: MachineSpecVM }[]) => {
    if (updates.length === 0) return;
    const byNode = new Map(updates.map((u) => [u.nodeId, u.spec]));
    const machineOverrides = cell.infrastructurePlan.machines.map((m) => {
      const spec = byNode.get(m.nodeId);
      return spec ? { ...m, spec } : m;
    });
    void onPatch({ cells: [{ cellId: cell.id, machineOverrides }] });
  };

  return (
    <div className="mx-auto max-w-[120rem] space-y-6">
      <SuiteReadinessPanel draft={draft} onConfirmAllMachines={onConfirmAllMachines} />
      {step === "cells" && (
        <CellsSection
          slug={slug}
          draft={draft}
          adding={adding}
          onAddClick={() => setAdding((v) => !v)}
          onAdd={addCell}
          onCancelAdd={() => setAdding(false)}
          onPatch={onPatch}
        />
      )}
      {step === "machines" && (
        <MachineSettingsSection
          draft={draft}
          onConfirmAllMachines={onConfirmAllMachines}
          onConfirmCell={(cell) =>
            void onPatch({ cells: [{ cellId: cell.id, machineOverrides: cell.infrastructurePlan.machines }] })
          }
          onMachineChange={setCellMachine}
          onMachinesChange={setCellMachines}
        />
      )}
      {step === "settings" && (
        <section className="border border-zinc-800/70 bg-[#070707] p-4">
          <div className="mb-4">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">3. Suite options</div>
            <h2 className="mt-1 text-base font-semibold text-foreground">Set provider, schedule and run policy</h2>
            <p className="mt-1 max-w-3xl text-[12px] leading-snug text-zinc-500">
              These settings apply to the whole suite. Per-node provider machine resources stay in the Machines step.
            </p>
          </div>
          <div className="max-w-3xl">
            <SuiteSettingsRail draft={draft} onPatch={onPatch} />
          </div>
        </section>
      )}
    </div>
  );
}

function SuiteReadinessPanel({
  draft,
  onConfirmAllMachines,
}: {
  draft: SuiteWizardDraftVM;
  onConfirmAllMachines: () => Promise<void>;
}) {
  const machineCells = cellsWithMachines(draft);
  const unconfirmed = cellsNeedingMachineConfirmation(draft);
  const hasCells = draft.cells.length > 0;
  const blockingErrors = draft.errors.filter((e) => e.severity === "error");

  if (!hasCells) {
    return (
      <div className="border border-zinc-800 bg-[#0a0a0a] p-4">
        <div className="flex items-start gap-3">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center border border-primary/30 bg-primary/[0.08] font-mono text-[11px] text-primary">1</span>
          <div className="min-w-0">
            <div className="text-sm font-medium text-foreground">Add suite cells</div>
            <p className="mt-1 text-[12px] leading-snug text-zinc-500">
              A cell is one test in the suite. Add either a database + workload preset pair or an existing test preset.
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (unconfirmed.length > 0) {
    return (
      <div className="border border-amber-900/60 bg-amber-950/15 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-sm font-medium text-amber-300">
              <AlertCircle className="h-4 w-4" /> Provider machines need confirmation
            </div>
            <p className="mt-1 max-w-3xl text-[12px] leading-snug text-amber-200/70">
              Presets define what to test. The generated nodes below define where it runs. Review CPU, RAM and disk for each provider machine, then confirm the visible settings.
            </p>
          </div>
          <Button size="sm" className="shrink-0 gap-1.5" onClick={() => void onConfirmAllMachines()}>
            <Check className="h-3.5 w-3.5" /> Confirm visible machine settings
          </Button>
        </div>
        <div className="mt-3 grid gap-2 md:grid-cols-2">
          {unconfirmed.map((cell) => (
            <div key={cell.id} className="min-w-0 border border-amber-900/40 bg-black/20 px-3 py-2">
              <div className="truncate text-[12px] font-medium text-amber-100">{cellTitle(cell)}</div>
              <div className="mt-1 flex flex-wrap gap-1.5">
                {cell.infrastructurePlan.machines.map((m) => (
                  <span key={m.nodeId} className="border border-amber-900/40 px-1.5 py-0.5 font-mono text-[10px] text-amber-200/75">
                    {m.nodeId}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (blockingErrors.length > 0) {
    return (
      <div className="border border-red-900/50 bg-red-950/20 p-4">
        <div className="mb-2 flex items-center gap-2 text-sm font-medium text-red-400">
          <AlertCircle className="h-4 w-4" /> Resolve suite issues
        </div>
        <div className="space-y-1">
          {blockingErrors.map((e, i) => (
            <div key={`${e.field}-${i}`} className="text-[12px] text-red-200/80">
              {friendlySuiteError(e.message)}
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="border border-emerald-900/50 bg-emerald-950/15 p-4">
      <div className="flex items-center gap-2 text-sm font-medium text-emerald-300">
        <Check className="h-4 w-4" /> Suite is ready
      </div>
      <p className="mt-1 text-[12px] text-emerald-200/70">
        {machineCells.length} cell{machineCells.length === 1 ? "" : "s"} configured with confirmed provider machine settings.
      </p>
    </div>
  );
}

function CellsSection({
  slug,
  draft,
  adding,
  onAddClick,
  onAdd,
  onCancelAdd,
  onPatch,
}: {
  slug: string;
  draft: SuiteWizardDraftVM;
  adding: boolean;
  onAddClick: () => void;
  onAdd: (cell: SuiteCellInput) => void;
  onCancelAdd: () => void;
  onPatch: (input: SuitePatchInput) => Promise<void>;
}) {
  return (
    <section className="border border-zinc-800/70 bg-[#070707] p-4">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">1. Suite cells</div>
          <h2 className="mt-1 text-base font-semibold text-foreground">Choose what the suite runs</h2>
          <p className="mt-1 max-w-3xl text-[12px] leading-snug text-zinc-500">
            Each cell becomes one test run. Use preset pairs for matrix coverage, or reuse a complete test preset.
          </p>
        </div>
        <Button size="sm" variant="outline" className="h-8 shrink-0 gap-1.5" onClick={onAddClick}>
          <Plus className="h-3.5 w-3.5" /> {adding ? "Close add form" : "Add cell"}
        </Button>
      </div>

      {adding && <CellComposer slug={slug} onAdd={onAdd} onCancel={onCancelAdd} />}

      {draft.cells.length === 0 && !adding ? (
        <div className="border border-zinc-800 bg-[#0a0a0a] p-6 text-center text-[12px] text-zinc-600">
          No cells yet. Add a database + workload preset pair or reuse an existing test preset.
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          {draft.cells.map((c, i) => (
            <CellSummaryCard
              key={c.id || `local-${i}`}
              cell={c}
              onToggle={(enabled) => void onPatch({ cells: [{ cellId: c.id, enabled }] })}
              onRemove={() => void onPatch({ cells: [{ cellId: c.id, remove: true }] })}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function MachineSettingsSection({
  draft,
  onConfirmAllMachines,
  onConfirmCell,
  onMachineChange,
  onMachinesChange,
}: {
  draft: SuiteWizardDraftVM;
  onConfirmAllMachines: () => Promise<void>;
  onConfirmCell: (cell: SuiteCellVM) => void;
  onMachineChange: (cell: SuiteCellVM, nodeId: string, spec: MachineSpecVM) => void;
  onMachinesChange: (cell: SuiteCellVM, updates: { nodeId: string; spec: MachineSpecVM }[]) => void;
}) {
  const machineCells = cellsWithMachines(draft);
  const unconfirmed = cellsNeedingMachineConfirmation(draft);
  const [activeCellId, setActiveCellId] = useState("");
  const activeCell = machineCells.find((cell) => cell.id === activeCellId) ?? machineCells[0];

  useEffect(() => {
    if (machineCells.length === 0) {
      if (activeCellId) setActiveCellId("");
      return;
    }
    if (!machineCells.some((cell) => cell.id === activeCellId)) {
      setActiveCellId(machineCells[0].id);
    }
  }, [activeCellId, machineCells]);

  return (
    <section className="border border-zinc-800/70 bg-[#070707] p-4">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">2. Provider machines</div>
          <h2 className="mt-1 text-base font-semibold text-foreground">Review resources for every generated node</h2>
          <p className="mt-1 max-w-3xl text-[12px] leading-snug text-zinc-500">
            These are real provider machine settings produced from the selected presets. The suite cannot be saved until every node has been confirmed.
          </p>
        </div>
        <Button
          size="sm"
          variant={unconfirmed.length > 0 ? "default" : "outline"}
          className="h-8 shrink-0 gap-1.5"
          disabled={machineCells.length === 0}
          onClick={() => void onConfirmAllMachines()}
        >
          <Check className="h-3.5 w-3.5" /> Confirm all visible machines
        </Button>
      </div>

      {machineCells.length === 0 ? (
        <div className="border border-zinc-800 bg-[#0a0a0a] p-6 text-center text-[12px] text-zinc-600">
          Add cells first. Machine settings appear here after presets resolve into topology.
        </div>
      ) : (
        <div className="grid min-h-[34rem] gap-4 xl:grid-cols-[minmax(16rem,22rem)_minmax(0,1fr)]">
          <div className="min-w-0 border border-zinc-800 bg-[#0a0a0a]">
            <div className="border-b border-zinc-800 px-3 py-2">
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Suite cells</div>
              <div className="mt-0.5 text-[12px] text-zinc-500">
                {machineCells.length} cell{machineCells.length === 1 ? "" : "s"} with generated machines
              </div>
            </div>
            <div className="max-h-[38rem] space-y-1 overflow-y-auto p-2">
              {machineCells.map((cell) => {
                const total = cell.infrastructurePlan.machines.length;
                const confirmed = cell.machineOverrideCount >= total;
                const active = activeCell?.id === cell.id;
                return (
                  <button
                    key={cell.id}
                    type="button"
                    onClick={() => setActiveCellId(cell.id)}
                    className={`w-full min-w-0 border p-2.5 text-left transition-colors ${
                      active
                        ? "border-primary/50 bg-primary/[0.07]"
                        : confirmed
                          ? "border-zinc-800 hover:border-zinc-700"
                          : "border-amber-900/45 bg-amber-950/10 hover:border-amber-700/60"
                    }`}
                  >
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-zinc-200">
                        {cellTitle(cell)}
                      </span>
                      <span className={`shrink-0 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide ${confirmed ? "bg-emerald-500/10 text-emerald-400" : "bg-amber-500/10 text-amber-300"}`}>
                        {confirmed ? "ok" : "edit"}
                      </span>
                    </div>
                    <div className="mt-1 grid grid-cols-1 gap-0.5 font-mono text-[10px] text-zinc-600">
                      <span className="truncate">db: {cell.dbKind || "—"}</span>
                      <span className="truncate">workload: {cell.workload || "—"}</span>
                      <span>
                        {cell.machineOverrideCount}/{total} confirmed · {total} node{total === 1 ? "" : "s"}
                      </span>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>

          {activeCell && (
            <div className={`min-w-0 border p-3 ${activeCell.machineOverrideCount >= activeCell.infrastructurePlan.machines.length ? "border-zinc-800 bg-[#0a0a0a]" : "border-amber-900/50 bg-amber-950/10"}`}>
              <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Selected cell machines</div>
                  <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
                    <h3 className="min-w-0 truncate text-sm font-medium text-foreground">{cellTitle(activeCell)}</h3>
                    <span className={`px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide ${activeCell.machineOverrideCount >= activeCell.infrastructurePlan.machines.length ? "bg-emerald-500/10 text-emerald-400" : "bg-amber-500/10 text-amber-300"}`}>
                      {activeCell.machineOverrideCount >= activeCell.infrastructurePlan.machines.length ? "confirmed" : "needs confirmation"}
                    </span>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 font-mono text-[10px] text-zinc-600">
                    <span>db: {activeCell.dbKind || "—"}</span>
                    <span>workload: {activeCell.workload || "—"}</span>
                    <span>
                      {activeCell.machineOverrideCount}/{activeCell.infrastructurePlan.machines.length} confirmed
                    </span>
                  </div>
                </div>
                <Button size="sm" variant="outline" className="h-7 shrink-0 gap-1.5" onClick={() => onConfirmCell(activeCell)}>
                  <Check className="h-3.5 w-3.5" /> Confirm this cell
                </Button>
              </div>
              <MachinePlanEditor
                machines={activeCell.infrastructurePlan.machines}
                settings={activeCell.infrastructurePlan.settings}
                onMachineChange={(nodeId, spec) => onMachineChange(activeCell, nodeId, spec)}
                onMachinesChange={(updates) => onMachinesChange(activeCell, updates)}
              />
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function CellSummaryCard({
  cell,
  onToggle,
  onRemove,
}: {
  cell: SuiteCellVM;
  onToggle: (enabled: boolean) => void;
  onRemove: () => void;
}) {
  const sourceValue = cellSourceValue(cell);
  const totalMachines = cell.infrastructurePlan.machines.length;
  const confirmed = totalMachines > 0 && cell.machineOverrideCount >= totalMachines;
  return (
    <div
      className={`min-w-0 border bg-[#0a0a0a] p-3 ${cell.enabled ? "border-zinc-800" : "border-zinc-800/40 opacity-60"}`}
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
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-zinc-200">
          {cellTitle(cell)}
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
      <div className="mt-2 grid grid-cols-1 gap-1 font-mono text-[10px] text-zinc-500 sm:grid-cols-2">
        <span>db: {cell.dbKind || "—"}</span>
        <span>wl: {cell.workload || "—"}</span>
        <span>{sourceLabel(cell)}</span>
        <span>
          {cell.nodeCount} node{cell.nodeCount === 1 ? "" : "s"} · {totalMachines === 0 ? "waiting for topology" : confirmed ? "machines confirmed" : "machines need confirmation"}
        </span>
      </div>
      <div className="mt-1 truncate font-mono text-[10px] text-zinc-700" title={sourceValue}>
        {sourceValue}
      </div>
      {totalMachines > 0 && (
        <div className="mt-2 border-t border-zinc-800/60 pt-2 text-[10px] text-zinc-600">
          First machine: <span className="font-mono text-zinc-500">{machineSpecSummary(cell.infrastructurePlan.machines[0]?.spec)}</span>
        </div>
      )}
      {cell.errors.filter((e) => e.severity === "error").length > 0 && (
        <div className="mt-2 text-[11px] text-red-400">
          {friendlySuiteError(cell.errors.filter((e) => e.severity === "error")[0].message)}
        </div>
      )}
    </div>
  );
}

// ─── Cell composer (add a new cell) ──────────────────────────────────────────────

type CellMode = "presetPair" | "testPreset";

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
  const [testPresets, setTestPresets] = useState<TestPresetRow[]>([]);
  const [dbPresetId, setDbPresetId] = useState("");
  const [workloadPresetId, setWorkloadPresetId] = useState("");
  const [testPresetId, setTestPresetId] = useState("");
  const [name, setName] = useState("");

  // Load the db presets for the chosen engine + the reusable workload/test catalogs.
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

  useEffect(() => {
    let cancelled = false;
    getPresetProvider()
      .listTestPresetRows(slug, { pageSize: 200 })
      .then((p) => {
        if (cancelled) return;
        setTestPresets(p.rows);
        setTestPresetId("");
      })
      .catch(() => !cancelled && setTestPresets([]));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const canAdd =
    mode === "presetPair"
      ? !!dbPresetId && !!workloadPresetId
      : !!testPresetId;

  const submit = () => {
    if (mode === "presetPair") {
      onAdd({ name, enabled: true, source: "presetPair", dbPresetId, workloadPresetId });
    } else {
      onAdd({ name, enabled: true, source: "testPreset", testPresetId });
    }
  };

  return (
    <div className="mb-3 border border-primary/30 bg-primary/[0.03] p-3">
      <div className="mb-3 inline-flex border border-zinc-800 text-[11px] font-mono">
        {(["presetPair", "testPreset"] as CellMode[]).map((m) => (
          <button
            key={m}
            type="button"
            onClick={() => setMode(m)}
            className={`px-3 py-1 transition-colors ${
              mode === m ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
            }`}
          >
            {m === "presetPair" ? "Preset pair" : "Test preset"}
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

        {mode === "presetPair" && (
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
        )}

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
          <div className="sm:col-span-2">
            <Label className="text-[9px] font-mono text-zinc-600">Test preset</Label>
            <Select value={testPresetId} onValueChange={setTestPresetId}>
              <SelectTrigger className="mt-1 h-7 text-[11px]">
                <SelectValue placeholder="select…" />
              </SelectTrigger>
              <SelectContent>
                {testPresets.map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {p.name || p.id}
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
