// Test Wizard — create a test run through a multi-step wizard.
//
// Rebuilt against the typed cloud.v1.api.TestWizardService proto + the
// WizardProvider abstraction (services/wizard.ts) AND the old NewRun design
// language (provider tiles + machine sizing sliders + topology diagram +
// ConfigEditor). The page never touches the wire format directly: each step
// edits one flat VM sub-section and calls provider.patch(); the server (mock)
// recomputes the draft — topology, the deployment.InfrastructurePlan (provider
// settings + per-node machine specs the user EDITS), the render-preview
// artifacts (EDITABLE configs the user overrides, READ_ONLY ones shown locked),
// the per-field validation errors and the `ready` flag.
//
// Steps (and the PatchTestWizard sub-message each edits):
//   1. Database       — domain.Database (typed per engine) + topology diagram.
//        The DB choice drives node roles/counts, so it comes first: the server
//        recomputes the topology + machine plan FROM the database params.
//   2. Workload       — domain.Workload + ProbeScript. The workload can ADD
//        infrastructure components (the stroppy-agent runner node, monitoring),
//        so it runs BEFORE Infrastructure — the machine list isn't final until
//        the workload is set.
//   3. Infrastructure — provider (deployment.Provider) + per-machine specs
//        (MachinePlan → Docker.Container | Yandex.Vm) → .machine_overrides.
//        Comes LAST of the editing steps so you size the now-complete machine
//        set. Provider-level settings (cloud/folder/zone/network/image/…) are
//        TENANT provider defaults resolved server-side and are NOT edited here —
//        only the provider SELECTION and the per-node machine sizing are.
//   4. Review/Render  — render artifacts (RenderPreview); EDITABLE configs via
//        ConfigEditor → .render_overrides (FileOverride + base_hash); then
//        FinishTestWizard (Launch start=true and/or Save-as-preset + ratings)
//
// Steps are URL-backed via ?step= and ?draft= so the browser Back button walks
// the wizard and a draft is resumable.

import { create } from "@bufbuild/protobuf";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import {
  getWizardProvider,
  ENGINES,
  ENGINE_TO_KIND,
  Provider,
  Workload_Protocol,
  RenderArtifact_Kind,
  RenderArtifact_Origin,
  RenderArtifact_Mutability,
  blankEngineParams,
  defaultWorkload,
  type WizardDraftVM,
  type DraftSummaryVM,
  type DatabaseVM,
  type DatabasePackageVM,
  type WorkloadVM,
  type EngineKind,
  type DraftErrorVM,
  type InfrastructurePlanVM,
  type MachineSpecVM,
  type RenderArtifactVM,
  type FileOverrideVM,
} from "@/services/wizard";
import {
  getPresetProvider,
  type DatabasePresetVM,
  type WorkloadPresetVM,
} from "@/services/preset";
import {
  getPackagesProvider,
  type PackageRow,
} from "@/services/packages";
import type { DbKind } from "@/components/library-table/labels";
import {
  getStroppyProvider,
  commitVersion,
  isCommitVersion,
  commitSha,
} from "@/services/stroppy";
import {
  NumField,
  ToggleRow,
  FieldErrors,
  errorsFor,
  EngineParamsForm,
  EngineVersionSelect,
} from "@/components/database/DatabaseParamsForm";
import { WorkloadParamsForm, type SegmentProbeContext } from "@/components/workload/WorkloadParamsForm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { ConfigEditor } from "@/components/ui/config-editor";
import {
  MachinePlanEditor,
  machineSpecSummary,
} from "@/components/wizard/MachinePlanEditor";
import { PackageSchema } from "@/lib/proto/cloud/v1/domain/database_pb";
import {
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  AlertCircle,
  AlertTriangle,
  Info,
  Database,
  Server,
  Cpu,
  Cloud,
  Container,
  Network,
  FileText,
  Terminal,
  Rocket,
  Loader2,
  Save,
  History,
  Trash2,
  Zap,
  Box,
  Globe,
  Layers,
  Lock,
  RotateCcw,
  Pencil,
  Tag,
  GitCommit,
  Package as PackageIcon,
} from "lucide-react";

// ─── Step model ──────────────────────────────────────────────────────────────

type StepKey = "infra" | "database" | "workload" | "review";

const STEPS: { key: StepKey; label: string }[] = [
  { key: "database", label: "Database" },
  { key: "workload", label: "Workload" },
  { key: "infra", label: "Infrastructure" },
  { key: "review", label: "Review" },
];

const CUSTOM_DATABASE_PRESET_ID = "__custom_database__";
const CUSTOM_WORKLOAD_PRESET_ID = "__custom_workload__";

const ENGINE_ICON: Record<EngineKind, typeof Database> = {
  postgres: Database,
  mysql: Server,
  mariadb: Server,
  picodata: Cpu,
  ydb: Database,
  ydbManaged: Cloud,
  cockroach: Database,
  external: Network,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** Which top-level draft sections a step is responsible for (for the dot). */
const STEP_FIELDS: Record<StepKey, string[]> = {
  infra: ["provider", "infrastructure_plan", "name"],
  database: ["database"],
  workload: ["workload"],
  review: [],
};

function SectionTitle({ children, hint }: { children: React.ReactNode; hint?: string }) {
  return (
    <div className="mb-4">
      <h2 className="text-lg font-semibold tracking-tight text-foreground">{children}</h2>
      {hint && <p className="mt-1 max-w-3xl text-sm text-muted-foreground">{hint}</p>}
    </div>
  );
}

/** A flex-1 right rail rendered in the PROGRESSIVE pane rows whenever the final
 * editing pane isn't revealed yet. It guarantees the row always fills the width
 * (no giant dead space on the right before the user has advanced) and shows a
 * calm empty-state telling them what to do next. Hidden on narrow viewports
 * where the panes stack — an empty-state block would just be noise there. */
function PreviewAside({
  icon: Icon,
  title,
  hint,
}: {
  icon: typeof Database;
  title: string;
  hint?: string;
}) {
  return (
    <div className="hidden min-h-0 min-w-0 flex-1 flex-col lg:flex">
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center border border-dashed border-zinc-800/70 bg-surface-chrome/40 p-8 text-center">
        <Icon className="mb-3 h-8 w-8 text-zinc-700" />
        <div className="text-sm font-medium text-zinc-400">{title}</div>
        {hint && <p className="mt-1.5 max-w-xs text-xs leading-relaxed text-zinc-600">{hint}</p>}
      </div>
    </div>
  );
}

// ─── Main ────────────────────────────────────────────────────────────────────

export function NewRun() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();

  const draftId = params.get("draft") ?? "";
  // "New from run": ?from=<runId> auto-starts a draft seeded from that run's spec.
  const fromRunId = params.get("from") ?? "";
  const stepKey = (params.get("step") as StepKey) || "database";
  const stepIndex = Math.max(0, STEPS.findIndex((s) => s.key === stepKey));

  const [draft, setDraft] = useState<WizardDraftVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [patching, setPatching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const patchQueueRef = useRef<Promise<void>>(Promise.resolve());
  const pendingPatchCountRef = useRef(0);
  const activeDraftIdRef = useRef(draftId);

  // Resume picker (List/Get/Delete) shown when no draft is active.
  const [resume, setResume] = useState<DraftSummaryVM[]>([]);
  const [startName, setStartName] = useState("");

  useBreadcrumbLabel("draft", draft ? draft.name || "New Run" : undefined);

  useEffect(() => {
    activeDraftIdRef.current = draftId;
  }, [draftId]);

  useEffect(() => {
    if (draftId || !slug) return;
    let cancelled = false;
    getWizardProvider()
      .list(slug)
      .then((d) => !cancelled && setResume(d))
      .catch(() => !cancelled && setResume([]));
    return () => {
      cancelled = true;
    };
  }, [draftId, slug]);

  useEffect(() => {
    if (!draftId || !slug) {
      setDraft(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    getWizardProvider()
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
    (id: string, step: StepKey) => {
      setParams({ draft: id, step });
    },
    [setParams],
  );

  const handleStart = useCallback(async () => {
    if (!slug) return;
    setError(null);
    setLoading(true);
    try {
      const d = await getWizardProvider().start(slug, startName.trim() || "Untitled run");
      setDraft(d);
      setActiveDraft(d.id, "database");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [slug, startName, setActiveDraft]);

  // "New from run": when arriving with ?from=<runId> and no active draft, start a
  // draft seeded from that run's spec and replace the URL with the new draft (so a
  // reload/back doesn't re-clone). The ref guards against StrictMode double-fire.
  const cloneStartedRef = useRef(false);
  useEffect(() => {
    if (!slug || !fromRunId || draftId || cloneStartedRef.current) return;
    cloneStartedRef.current = true;
    setError(null);
    setLoading(true);
    getWizardProvider()
      .start(slug, "Untitled run", { sourceRunId: fromRunId })
      .then((d) => {
        setDraft(d);
        setParams({ draft: d.id, step: "database" }, { replace: true });
      })
      .catch((e) => {
        cloneStartedRef.current = false;
        setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => setLoading(false));
  }, [slug, fromRunId, draftId, setParams]);

  const patch = useCallback(
    (input: Parameters<ReturnType<typeof getWizardProvider>["patch"]>[2]) => {
      if (!slug || !draftId) return Promise.resolve();
      const requestSlug = slug;
      const requestDraftId = draftId;
      pendingPatchCountRef.current += 1;
      setPatching(true);
      setError(null);

      const run = async () => {
        try {
          const d = await getWizardProvider().patch(requestSlug, requestDraftId, input);
          if (activeDraftIdRef.current === requestDraftId) setDraft(d);
        } catch (e) {
          if (activeDraftIdRef.current === requestDraftId) {
            setError(e instanceof Error ? e.message : String(e));
          }
        } finally {
          pendingPatchCountRef.current = Math.max(0, pendingPatchCountRef.current - 1);
          if (pendingPatchCountRef.current === 0) setPatching(false);
        }
      };

      const queued = patchQueueRef.current.then(run, run);
      patchQueueRef.current = queued.catch(() => undefined);
      return queued;
    },
    [slug, draftId],
  );

  // Debounced patch — the form steps fire a patch on every keystroke, and each
  // server round-trip recomputes draft.errors. Validating per-keystroke made
  // the inline error rows fl/blink and the layout jump ("epilepsy"). We coalesce
  // rapid edits (latest-wins) into one patch ~350 ms after typing settles, and
  // flush any pending edit synchronously before navigating so the next step
  // always sees the validated draft. Discrete picks still go through the
  // immediate `patch` (and flush covers the in-flight case anyway).
  const patchTimerRef = useRef<number | undefined>(undefined);
  const pendingPatchRef = useRef<Parameters<typeof patch>[0] | null>(null);

  const flushPatch = useCallback(() => {
    if (patchTimerRef.current !== undefined) {
      window.clearTimeout(patchTimerRef.current);
      patchTimerRef.current = undefined;
    }
    const input = pendingPatchRef.current;
    pendingPatchRef.current = null;
    if (input) void patch(input);
  }, [patch]);

  const patchDebounced = useCallback(
    (input: Parameters<typeof patch>[0]): Promise<void> => {
      pendingPatchRef.current = input;
      setPatching(true); // surface activity immediately while the edit settles
      if (patchTimerRef.current !== undefined) window.clearTimeout(patchTimerRef.current);
      patchTimerRef.current = window.setTimeout(() => {
        patchTimerRef.current = undefined;
        const queued = pendingPatchRef.current;
        pendingPatchRef.current = null;
        if (queued) void patch(queued);
      }, 350);
      return Promise.resolve();
    },
    [patch],
  );

  // Navigate between steps. Flush any pending debounced edit first so the step
  // we land on validates against the latest values.
  const goStep = useCallback(
    (key: StepKey) => {
      flushPatch();
      const next = new URLSearchParams(params);
      next.set("step", key);
      setParams(next);
    },
    [params, setParams, flushPatch],
  );

  const deleteDraft = useCallback(
    async (id: string) => {
      await getWizardProvider().remove(slug, id);
      setResume((prev) => prev.filter((d) => d.id !== id));
    },
    [slug],
  );

  // Cloning from a run (?from=…): show a spinner instead of the start/resume
  // picker until the seeded draft is created and the URL flips to ?draft=….
  if (!draftId && fromRunId && !error) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Cloning run…
      </div>
    );
  }

  if (!draftId) {
    return (
      <StartScreen
        startName={startName}
        setStartName={setStartName}
        onStart={handleStart}
        loading={loading}
        error={error}
        resume={resume}
        onResume={(id) => setActiveDraft(id, "database")}
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

  const canPrev = stepIndex > 0;
  const canNext = stepIndex < STEPS.length - 1;

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Step bar */}
      <div className="shrink-0 border-b border-zinc-800 bg-surface-chrome px-6 py-2.5 lg:px-8">
        <div className="flex items-center gap-1">
          {STEPS.map((s, i) => {
            const fields = STEP_FIELDS[s.key];
            const sectionErrs = draft.errors.filter(
              (e) => e.severity === "error" && fields.some((f) => e.field === f || e.field.startsWith(f + ".")),
            );
            return (
              <div key={s.key} className="flex items-center gap-1">
                {i > 0 && <ChevronRight className="h-3 w-3 text-zinc-700" />}
                <button
                  type="button"
                  onClick={() => goStep(s.key)}
                  className={`flex items-center gap-1.5 px-3 py-1 font-mono text-xs transition-all ${
                    i === stepIndex
                      ? "border border-primary/30 bg-primary/[0.06] text-primary"
                      : i < stepIndex
                        ? "text-zinc-400 hover:text-zinc-300"
                        : "text-zinc-600 hover:text-zinc-500"
                  }`}
                >
                  <span className="mr-0.5 text-zinc-600">{i + 1}.</span>
                  {s.label}
                  {sectionErrs.length > 0 && i !== stepIndex && (
                    <span className="h-1.5 w-1.5 rounded-full bg-red-500" />
                  )}
                </button>
              </div>
            );
          })}
          <div className="ml-auto flex items-center gap-2">
            {patching && <Loader2 className="h-3.5 w-3.5 animate-spin text-zinc-500" />}
            {(() => {
              const errCount = draft.errors.filter((e) => e.severity === "error").length;
              const warnCount = draft.errors.filter((e) => e.severity === "warning").length;
              return (
                <>
                  {errCount > 0 && (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-red-400">
                      <AlertCircle className="h-3 w-3" /> {errCount} error{errCount > 1 ? "s" : ""}
                    </span>
                  )}
                  {warnCount > 0 && (
                    <span className="inline-flex items-center gap-1 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-amber-400">
                      <AlertTriangle className="h-3 w-3" /> {warnCount}
                    </span>
                  )}
                  <span
                    className={`inline-flex items-center gap-1 px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider ${
                      draft.ready
                        ? "bg-emerald-500/10 text-emerald-400"
                        : "bg-zinc-800/60 text-zinc-500"
                    }`}
                  >
                    {draft.ready ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
                    {draft.ready ? "Ready" : "Incomplete"}
                  </span>
                </>
              );
            })()}
          </div>
        </div>
      </div>

      {/* Body: full-bleed, full-height step content. The summary rail was
          removed (readiness lives in the header badge, validation is surfaced
          inline per step, and the final "ready to finish" gate lives in the
          Review step). The step region is a min-h-0 flex child that fills all
          remaining vertical space; each step is itself a full-height flex
          column whose tall inner elements grow to fill it. The Back/Next footer
          is pinned at the bottom (shrink-0) instead of floating with dead space
          beneath it. Long content scrolls WITHIN the step region. */}
      <div className="min-h-0 flex-1 overflow-y-auto px-6 py-6 lg:px-8">
        {error && (
          <div className="mb-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-sm text-red-400">
            <AlertCircle className="h-4 w-4" /> {error}
          </div>
        )}

        <div className="flex h-full flex-col">
          {stepKey === "infra" && <StepInfra draft={draft} patch={patchDebounced} />}
          {stepKey === "database" && <StepDatabase draft={draft} patch={patchDebounced} slug={slug} />}
          {stepKey === "workload" && <StepWorkload draft={draft} patch={patchDebounced} slug={slug} />}
          {stepKey === "review" && (
            <StepReview
              draft={draft}
              slug={slug}
              patch={patch}
              onDone={(runId) => {
                if (runId) navigate(`/runs/${runId}`);
                else navigate("/runs");
              }}
            />
          )}
        </div>
      </div>

      {/* Nav footer — pinned at the bottom of the content viewport. */}
      <div className="flex shrink-0 items-center justify-between border-t border-zinc-800 bg-surface-chrome px-6 py-3 lg:px-8">
        <Button
          variant="outline"
          size="sm"
          disabled={!canPrev}
          onClick={() => goStep(STEPS[stepIndex - 1].key)}
        >
          <ChevronLeft className="h-3 w-3" /> Back
        </Button>
        {canNext && (
          <Button size="sm" className="gap-1.5" onClick={() => goStep(STEPS[stepIndex + 1].key)}>
            Next <ChevronRight className="h-3 w-3" />
          </Button>
        )}
      </div>
    </div>
  );
}

// ─── Start / Resume screen ────────────────────────────────────────────────────

function StartScreen({
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
  resume: DraftSummaryVM[];
  onResume: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  return (
    <div className="mx-auto max-w-2xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Test Wizard
      </div>
      <h1 className="text-2xl font-semibold tracking-tight text-foreground">New test run</h1>
      <p className="mt-1 text-sm text-muted-foreground">
        Build a benchmark run step by step — a database engine, then the workload (which may add
        runner/monitoring nodes), then pick a provider &amp; size the now-complete machine set, and
        finally review the server-rendered configs before launching.
      </p>

      {error && (
        <div className="mt-4 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-sm text-red-400">
          <AlertCircle className="h-4 w-4" /> {error}
        </div>
      )}

      <div className="mt-6 border border-zinc-800 bg-surface-tile p-5">
        <Label>Run name</Label>
        <div className="mt-1 flex gap-2">
          <Input
            placeholder="e.g. pg16 tpcc baseline"
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
          The run stays editable until you launch it.
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
                  <div className="truncate text-sm text-foreground">{d.name || "Untitled run"}</div>
                  <div className="mt-0.5 flex items-center gap-2 text-[11px] text-zinc-600">
                    <span className="font-mono">{d.engine || "no engine"}</span>
                    <span>·</span>
                    <span>{new Date(d.updatedAt).toLocaleString()}</span>
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

// ─── Step 1: Infrastructure (provider + settings + per-machine specs) ──────────

const PROVIDERS: { provider: Provider; label: string; icon: typeof Container; blurb: string }[] = [
  { provider: Provider.DOCKER, label: "Docker", icon: Container, blurb: "Local containers — fast, single-host, great for smoke tests." },
  { provider: Provider.YANDEX, label: "Yandex Cloud", icon: Cloud, blurb: "Provisions VMs via Terraform — real multi-node infrastructure." },
];

function StepInfra({
  draft,
  patch,
}: {
  draft: WizardDraftVM;
  patch: (input: {
    provider?: Provider;
    machineOverrides?: InfrastructurePlanVM["machines"];
    machineOverrideSettings?: InfrastructurePlanVM["settings"];
  }) => Promise<void>;
}) {
  // Local editable copy of the server-returned infrastructure plan.
  const [plan, setPlan] = useState<InfrastructurePlanVM>(draft.infrastructurePlan);
  // Re-seed from the server whenever it recomputes the machine list (provider or
  // engine change adds/removes nodes). Keyed by a signature so in-flight edits
  // aren't clobbered by an unrelated re-render.
  const sig = useMemo(
    () => `${draft.infrastructurePlan.provider}|${draft.infrastructurePlan.machines.map((m) => m.nodeId).join(",")}`,
    [draft.infrastructurePlan],
  );
  const lastSig = useRef(sig);
  useEffect(() => {
    if (sig !== lastSig.current) {
      setPlan(draft.infrastructurePlan);
      lastSig.current = sig;
    }
  }, [sig, draft.infrastructurePlan]);

  const setMachine = (nodeId: string, spec: MachineSpecVM) => {
    // Send ONLY the edited machines. Provider settings are tenant defaults
    // resolved server-side — we never send edited settings, we just echo back
    // whatever the server returned in draft.infrastructurePlan.settings.
    const next: InfrastructurePlanVM = {
      ...plan,
      machines: plan.machines.map((m) => (m.nodeId === nodeId ? { ...m, spec } : m)),
    };
    setPlan(next);
    void patch({ machineOverrides: next.machines, machineOverrideSettings: next.settings });
  };

  const setMachines = (updates: { nodeId: string; spec: MachineSpecVM }[]) => {
    if (updates.length === 0) return;
    const byNode = new Map(updates.map((u) => [u.nodeId, u.spec]));
    const next: InfrastructurePlanVM = {
      ...plan,
      machines: plan.machines.map((m) => {
        const spec = byNode.get(m.nodeId);
        return spec ? { ...m, spec } : m;
      }),
    };
    setPlan(next);
    void patch({ machineOverrides: next.machines, machineOverrideSettings: next.settings });
  };

  const pickProvider = (p: Provider) => {
    if (p === plan.provider) return;
    // Provider switch: let the server recompute settings + machine variants.
    void patch({ provider: p });
  };

  const confirmPlan = () => {
    if (plan.machines.length === 0) return;
    void patch({ machineOverrides: plan.machines, machineOverrideSettings: plan.settings });
  };

  const provErrs = errorsFor(draft.errors, "provider")
    .concat(errorsFor(draft.errors, "infrastructure_plan"))
    .concat(errorsFor(draft.errors, "machine_overrides"));

  return (
    <div className="mx-auto flex h-full w-full max-w-[120rem] flex-col">
      <SectionTitle hint="Choose where to run and size the machines produced by the selected database and workload. Tenant provider defaults are resolved server-side; this step edits only provider choice and per-node resources.">
        Infrastructure
      </SectionTitle>

      <div className="grid min-h-0 flex-1 gap-5 xl:grid-cols-[minmax(16rem,20rem)_minmax(0,1fr)] xl:items-stretch">
        <div className="space-y-5 overflow-y-auto">
          {/* Identity */}
          <div className="border border-zinc-800/60 bg-surface-chrome px-3 py-2.5">
            <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">Run</div>
            <div className="mt-1 font-mono text-sm text-zinc-200">{draft.name || "Untitled run"}</div>
          </div>

          {/* Provider tiles */}
          <div>
            <div className="mb-1 text-sm font-semibold text-foreground">Where to run?</div>
            <p className="mb-3 text-xs text-zinc-500">
              Choose the deployment backend. Provider-level credentials &amp; defaults are resolved
              from tenant settings server-side.
            </p>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              {PROVIDERS.map((p) => {
                const active = plan.provider === p.provider;
                const Icon = p.icon;
                return (
                  <button
                    key={p.provider}
                    type="button"
                    onClick={() => pickProvider(p.provider)}
                    className={`flex items-center gap-3 border p-4 text-left transition-all ${
                      active ? "border-primary/50 bg-primary/[0.07] text-primary" : "border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900/50"
                    }`}
                  >
                    <Icon className={`h-5 w-5 shrink-0 ${active ? "text-primary" : "text-zinc-600"}`} />
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
        </div>

        {/* Per-machine specs (machine_overrides from infrastructure_plan preview) — fills the height,
            scrolls internally, and uses the full width with more grid columns. */}
        <div className="flex min-h-0 flex-col">
          {plan.machines.length > 0 ? (
            <>
              <div className="mb-2 flex shrink-0 items-center justify-between gap-3">
                <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                  Machine plan — {plan.machines.length} node{plan.machines.length > 1 ? "s" : ""}
                </div>
                <Button size="sm" variant="outline" className="h-7 gap-1.5" onClick={confirmPlan}>
                  <Check className="h-3.5 w-3.5" /> Confirm machine settings
                </Button>
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto pr-1">
                <MachinePlanEditor
                  machines={plan.machines}
                  settings={plan.settings}
                  onMachineChange={setMachine}
                  onMachinesChange={setMachines}
                />
                <FieldErrors errs={provErrs} />
              </div>
            </>
          ) : (
            <div className="border border-zinc-800 bg-surface-tile p-5 text-center text-[12px] text-zinc-600">
              Pick a database engine (Database step) to populate the machine plan.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const ROLE_ICON: Record<string, typeof Database> = {
  master: Database,
  primary: Server,
  replica: Database,
  instance: Cpu,
  storage: Database,
  node: Database,
  haproxy: Globe,
  proxysql: Globe,
  etcd: Layers,
  agent: Zap,
  metrics: Server,
};

// ─── Step 2: Database (+ topology diagram) ─────────────────────────────────────

// The Database step is a three-pane PROGRESSIVE flow laid out horizontally,
// each pane sliding in from the right once the previous is chosen:
//
//   Pane 1 — Engine   : a compact column of engine tiles (left).
//   Pane 2 — Preset   : the database presets for that engine + a Custom/blank
//                       option; picking one pre-fills the settings pane.
//   Pane 3 — Settings : the typed domain.Database form (per-engine param editors)
//                       seeded from the chosen preset and fully editable. This is
//                       the source of truth that Patch(database) ships.
//
// The chosen DatabaseVM still flows through the unchanged Patch(database) →
// recompute-topology path; the preset only SEEDS the form.

type DbPhase = "engine" | "preset" | "settings";

function StepDatabase({
  draft,
  patch,
  slug,
}: {
  draft: WizardDraftVM;
  patch: (input: { database?: DatabaseVM }) => Promise<void>;
  slug: string;
}) {
  // A resumed draft already has a database → jump straight to the settings pane
  // (engine + preset are implicitly "chosen"). A fresh draft starts at engine.
  const initial = draft.database;
  const [engine, setEngine] = useState<EngineKind | null>(initial?.kind ?? null);
  const [presetId, setPresetId] = useState<string | null>(initial ? "__resumed__" : null);
  const [db, setDb] = useState<DatabaseVM | null>(initial ?? null);

  const phase: DbPhase = !engine ? "engine" : !presetId ? "preset" : "settings";

  // Resume: if the server hands back a database (e.g. after navigating away and
  // back) and we hadn't seeded one yet, adopt it.
  const adopted = useRef(!!initial);
  useEffect(() => {
    if (draft.database && !adopted.current) {
      setEngine(draft.database.kind);
      setPresetId("__resumed__");
      setDb(draft.database);
      adopted.current = true;
    }
  }, [draft.database]);

  const drafts = usePresetDraft<DatabaseVM>(normDatabase);

  // setDb + server patch — no preset-cache write (used by pick / reset).
  const commit = useCallback(
    (next: DatabaseVM) => {
      setDb(next);
      void patch({ database: next });
    },
    [patch],
  );

  // The form's edit path: commit AND remember the working copy for this preset
  // so switching presets away and back restores the user's edits.
  const apply = useCallback(
    (next: DatabaseVM) => {
      commit(next);
      drafts.edit(presetId, next);
    },
    [commit, drafts, presetId],
  );

  // Pane 1 — pick an engine. Resets the downstream panes so 2 (and then 3)
  // re-reveal for the new engine.
  const pickEngine = (kind: EngineKind) => {
    if (kind === engine) return;
    setEngine(kind);
    setPresetId(null);
    setDb(null);
    adopted.current = true; // user-driven; don't let the effect clobber the reset
  };

  // Pane 2 — pick a preset (or Custom/blank). Restores prior edits for that
  // preset and records its pristine baseline.
  const pickPreset = (preset: DatabasePresetVM) => {
    const loaded = drafts.pick(preset.id, preset.database);
    setPresetId(preset.id);
    commit(loaded);
  };

  // Dirty / reset / save-as-preset for the picked database preset.
  const dirty = db ? drafts.isDirty(presetId, db) : false;
  const resetToPreset = useCallback(() => {
    const base = drafts.baseline(presetId);
    if (base) apply(base);
  }, [drafts, presetId, apply]);

  const [saveOpen, setSaveOpen] = useState(false);
  const [presetReload, setPresetReload] = useState(0);
  const saveAsPreset = useCallback(
    async (name: string, description: string) => {
      if (!db) return;
      const id = await getPresetProvider().createDatabasePreset(slug, {
        name,
        description,
        tags: {},
        database: db,
      });
      drafts.markSaved(id, db);
      setPresetId(id);
      setPresetReload((n) => n + 1); // refetch the preset list so it shows up
    },
    [slug, db, drafts],
  );

  const errs = errorsFor(draft.errors, "database");

  return (
    <div className="mx-auto flex h-full w-full max-w-[120rem] flex-col">
      <SectionTitle hint="Pick a database engine, seed it from a preset, then fine-tune the topology and engine settings. Node roles and counts from this step drive the machine plan.">
        Database under test
      </SectionTitle>

      {/* Three progressive panes, horizontal on lg+, stacked on narrow viewports.
          Engine + Preset are fixed-width columns; the third slot ALWAYS grows
          (the settings pane once revealed, otherwise a fill-the-width empty
          state) so the row never leaves dead space on the right. The row fills
          the remaining height and each pane scrolls internally. */}
      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:items-stretch">
        {/* Pane 1 — Engine (compact column on the left) */}
        <EnginePane active={engine} onPick={pickEngine} />

        {/* Pane 2 — Preset list (slides in once an engine is chosen) */}
        {engine && (
          <PresetPane
            key={engine}
            engine={engine}
            selectedId={presetId}
            onPick={pickPreset}
            reloadKey={presetReload}
          />
        )}

        {/* Pane 3 — Settings, or a fill-width hint until it's reached */}
        {engine && presetId && db ? (
          <SettingsPane
            key={`${engine}-settings`}
            draft={draft}
            db={db}
            apply={apply}
            errs={errs}
            dirty={dirty}
            canReset={isRealPresetId(presetId)}
            onReset={resetToPreset}
            onSave={() => setSaveOpen(true)}
          />
        ) : (
          <PreviewAside
            icon={engine ? Layers : Database}
            title={engine ? "Pick a preset" : "Pick an engine"}
            hint={
              engine
                ? "Choose a preset (or Custom) to configure the engine settings and see the derived topology."
                : "Select a database engine on the left to begin. Its node roles drive the machine plan."
            }
          />
        )}
      </div>

      <SavePresetDialog
        open={saveOpen}
        onOpenChange={setSaveOpen}
        kind="database"
        defaultName={draft.name ? `${draft.name} database` : ""}
        onSave={saveAsPreset}
      />
    </div>
  );
}

// ─── Database pane 1: engine picker (left column) ──────────────────────────────

function EnginePane({ active, onPick }: { active: EngineKind | null; onPick: (k: EngineKind) => void }) {
  return (
    <div className="flex min-h-0 shrink-0 flex-col lg:w-56">
      <PaneHeader index={1} title="Engine" subtitle="Database under test" />
      <div className="grid min-h-0 flex-1 grid-cols-2 gap-2 overflow-y-auto pr-1 lg:auto-rows-min lg:grid-cols-1 lg:content-start">
        {ENGINES.map((meta) => {
          const Icon = ENGINE_ICON[meta.kind];
          const sel = active === meta.kind;
          return (
            <button
              key={meta.kind}
              type="button"
              onClick={() => onPick(meta.kind)}
              className={`flex items-center gap-2.5 border p-3 text-left transition-all ${
                sel
                  ? "border-primary/50 bg-primary/[0.07]"
                  : "border-zinc-800 bg-surface-tile hover:border-zinc-700 hover:bg-zinc-900/50"
              }`}
            >
              <Icon className="h-4 w-4 shrink-0" style={{ color: meta.hex }} />
              <span className={`text-xs font-medium ${sel ? "text-primary" : "text-foreground"}`}>{meta.label}</span>
              {sel && <Check className="ml-auto h-3.5 w-3.5 shrink-0 text-primary" />}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ─── Database pane 2: preset picker (slides in) ────────────────────────────────

function PresetPane({
  engine,
  selectedId,
  onPick,
  reloadKey = 0,
}: {
  engine: EngineKind;
  selectedId: string | null;
  onPick: (p: DatabasePresetVM) => void;
  /** Bumped after a save-as-preset so the freshly created preset appears here. */
  reloadKey?: number;
}) {
  const slug = useTenantSlug() ?? "";
  const [presets, setPresets] = useState<DatabasePresetVM[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPresets(null);
    setErr(null);
    getPresetProvider()
      .listDatabasePresets(slug, engine)
      .then((p) => !cancelled && setPresets(p))
      .catch((e) => !cancelled && setErr(e instanceof Error ? e.message : String(e)));
    return () => {
      cancelled = true;
    };
  }, [slug, engine, reloadKey]);

  const meta = ENGINES.find((m) => m.kind === engine);
  const items = useMemo<DatabasePresetVM[] | null>(() => {
    if (!presets) return null;
    const pkg = builtinPackageForEngine(engine, "");
    const custom: DatabasePresetVM = {
      id: CUSTOM_DATABASE_PRESET_ID,
      name: "Custom",
      description: "Configure topology and engine parameters manually.",
      isSystem: false,
      database: {
        kind: engine,
        version: "",
        packageId: pkg?.id,
        installPackage: pkg,
        params: blankEngineParams(engine),
      },
    };
    return [custom, ...presets.filter((preset) => preset.id !== CUSTOM_DATABASE_PRESET_ID)];
  }, [engine, presets]);

  return (
    <div className="pane-reveal flex min-h-0 shrink-0 flex-col lg:w-72">
      <PaneHeader index={2} title="Preset" subtitle={`${meta?.label ?? engine} configurations`} />
      {err && (
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-3.5 w-3.5" /> {err}
        </div>
      )}
      {!presets && !err && (
        <div className="flex items-center gap-2 px-1 py-3 text-xs text-zinc-600">
          <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading presets…
        </div>
      )}
      {items && (
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {items.map((p) => {
            const sel = selectedId === p.id;
            const custom = p.id === CUSTOM_DATABASE_PRESET_ID;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => onPick(p)}
                className={`flex w-full items-start gap-2.5 border p-3 text-left transition-all ${
                  sel
                    ? "border-primary/50 bg-primary/[0.07]"
                    : "border-zinc-800 bg-surface-tile hover:border-zinc-700 hover:bg-zinc-900/50"
                }`}
              >
                {custom ? (
                  <Pencil className={`mt-0.5 h-4 w-4 shrink-0 ${sel ? "text-primary" : "text-zinc-500"}`} />
                ) : (
                  <Layers className={`mt-0.5 h-4 w-4 shrink-0 ${sel ? "text-primary" : "text-zinc-500"}`} />
                )}
                <div className="min-w-0">
                  <div className={`flex items-center gap-1.5 text-xs font-medium ${sel ? "text-primary" : "text-foreground"}`}>
                    {p.name}
                    {p.isSystem && (
                      <span className="shrink-0 bg-zinc-800/70 px-1 py-px font-mono text-[8px] uppercase tracking-wide text-zinc-500">
                        builtin
                      </span>
                    )}
                  </div>
                  <div className="mt-0.5 text-[11px] leading-snug text-zinc-600">{p.description}</div>
                </div>
                {sel && <Check className="ml-auto mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ─── Database pane 3: typed settings form + derived topology (slides in) ───────

function SettingsPane({
  draft,
  db,
  apply,
  errs,
  dirty,
  canReset,
  onReset,
  onSave,
}: {
  draft: WizardDraftVM;
  db: DatabaseVM;
  apply: (d: DatabaseVM) => void;
  errs: DraftErrorVM[];
  dirty: boolean;
  canReset: boolean;
  onReset: () => void;
  onSave: () => void;
}) {
  // The topology only gets its own SIDE column on very wide screens (2xl); below
  // that, Engine + Preset already eat ~32rem of the row, so a third settings
  // sub-column would crush the form (the old bug: a 22rem track was reserved for
  // the topology even when it was empty, squeezing the form to a sliver). Here
  // the topology stacks UNDER the form below 2xl, and is omitted entirely when
  // there's nothing to show — the form always keeps the full pane width.
  const hasTopo = draft.topologyComponents.length > 0;
  return (
    <div className="pane-reveal flex min-h-0 min-w-0 flex-1 flex-col">
      <PaneHeader
        index={3}
        title="Settings"
        subtitle="Preset values, editable"
        right={<PresetEditControls dirty={dirty} canReset={canReset} onReset={onReset} onSave={onSave} />}
      />
      <div
        className={`grid min-h-0 flex-1 gap-6 2xl:items-stretch ${
          hasTopo ? "2xl:grid-cols-[minmax(24rem,1fr)_minmax(0,22rem)]" : ""
        }`}
      >
        <div className="@container min-h-0 space-y-6 overflow-y-auto pr-1">
          <EngineVersionSelect db={db} apply={apply} />

          <DatabasePackageSelector db={db} apply={apply} />

          <EngineParamsForm db={db} apply={apply} />
          <FieldErrors errs={errs} />
          {/* Below 2xl the topology stacks under the form (the grid is a single
              column there); on 2xl it moves to the side column below instead. */}
          {hasTopo && (
            <div className="2xl:hidden">
              <TopologyDiagram draft={draft} />
            </div>
          )}
        </div>

        {/* Derived topology diagram — side column on 2xl only. */}
        {hasTopo && (
          <div className="hidden min-h-0 2xl:block">
            <TopologyDiagram draft={draft} />
          </div>
        )}
      </div>
    </div>
  );
}

function DatabasePackageSelector({
  db,
  apply,
}: {
  db: DatabaseVM;
  apply: (d: DatabaseVM) => void;
}) {
  const slug = useTenantSlug() ?? "";
  const [rows, setRows] = useState<PackageRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const builtin = useMemo(() => builtinPackageForDatabase(db), [db.kind, db.version]);
  const dbKind = packageDbKind(db.kind);

  useEffect(() => {
    if (!builtin) return;
    if (db.installPackage?.packageRecordId) return;
    if (db.installPackage?.isBuiltin && db.installPackage.id !== builtin.id) {
      apply({ ...db, packageId: builtin.id, installPackage: builtin });
    }
  }, [apply, builtin, db]);

  useEffect(() => {
    if (!slug || !dbKind) {
      setRows([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setErr(null);
    getPackagesProvider()
      .listPackages(slug, { dbKinds: [dbKind], pageSize: 100 })
      .then((page) => {
        if (cancelled) return;
        setRows(page.rows.filter((row) => row.status === "ready" && !row.isBuiltin));
      })
      .catch((e) => {
        if (!cancelled) setErr(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [dbKind, slug]);

  if (!builtin && rows.length === 0 && !loading && !err) return null;

  const selectedId = db.installPackage?.packageRecordId || db.packageId || db.installPackage?.id || builtin?.id || "";
  const selectBuiltin = () => {
    if (!builtin) return;
    apply({ ...db, packageId: builtin.id, installPackage: builtin });
  };
  const selectUploaded = (row: PackageRow) => {
    apply({
      ...db,
      packageId: row.id,
      installPackage: packageFromRecord(row, db.kind, db.version),
    });
  };

  return (
    <div className="border border-zinc-800 bg-surface-tile p-3">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <Label>Package</Label>
          <div className="mt-0.5 text-[11px] text-zinc-600">Selected install recipe for this database.</div>
        </div>
        <Link to="/packages" className="shrink-0 font-mono text-[10px] text-zinc-500 hover:text-zinc-300">
          manage
        </Link>
      </div>

      {err && (
        <div className="mb-2 flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-3.5 w-3.5" /> {err}
        </div>
      )}

      <div className="grid gap-2 md:grid-cols-2">
        {builtin && (
          <PackageOption
            active={selectedId === builtin.id}
            title={builtin.name || builtin.id}
            subtitle={packageSummary(builtin)}
            badge="builtin"
            onClick={selectBuiltin}
          />
        )}
        {rows.map((row) => (
          <PackageOption
            key={row.id}
            active={selectedId === row.id}
            title={row.name || row.id}
            subtitle={[row.version || "custom", row.format || "package", row.os, row.arch].filter(Boolean).join(" · ")}
            badge="uploaded"
            onClick={() => selectUploaded(row)}
          />
        ))}
        {loading && (
          <div className="flex items-center gap-2 border border-zinc-800 bg-black/10 px-3 py-2 text-xs text-zinc-600">
            <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading packages...
          </div>
        )}
      </div>
    </div>
  );
}

function PackageOption({
  active,
  title,
  subtitle,
  badge,
  onClick,
}: {
  active: boolean;
  title: string;
  subtitle: string;
  badge: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex min-w-0 items-start gap-2.5 border p-3 text-left transition-all ${
        active
          ? "border-primary/50 bg-primary/[0.07]"
          : "border-zinc-800 bg-black/10 hover:border-zinc-700 hover:bg-zinc-900/50"
      }`}
    >
      <PackageIcon className={`mt-0.5 h-4 w-4 shrink-0 ${active ? "text-primary" : "text-zinc-500"}`} />
      <div className="min-w-0 flex-1">
        <div className={`flex items-center gap-1.5 text-xs font-medium ${active ? "text-primary" : "text-foreground"}`}>
          <span className="min-w-0 truncate">{title}</span>
          <span className="shrink-0 bg-zinc-800/70 px-1 py-px font-mono text-[8px] uppercase tracking-wide text-zinc-500">
            {badge}
          </span>
        </div>
        <div className="mt-0.5 truncate text-[11px] text-zinc-600">{subtitle}</div>
      </div>
      {active && <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />}
    </button>
  );
}

function builtinPackageForDatabase(db: DatabaseVM): DatabasePackageVM | undefined {
  return builtinPackageForEngine(db.kind, db.version);
}

function builtinPackageForEngine(kind: EngineKind, inputVersion: string): DatabasePackageVM | undefined {
  if (kind === "external" || kind === "ydbManaged") return undefined;
  let version = inputVersion.trim() || "default";
  const dbKind = ENGINE_TO_KIND[kind];
  const base = {
    dbKind,
    dbVersion: version,
    isBuiltin: true,
  };
  switch (kind) {
    case "postgres":
      return create(PackageSchema, {
        ...base,
        id: `builtin/postgres/${version}`,
        name: `PostgreSQL ${version}`,
      });
    case "mysql":
      return create(PackageSchema, {
        ...base,
        id: `builtin/mysql/${version}`,
        name: `MySQL ${version}`,
      });
    case "mariadb":
      return create(PackageSchema, {
        ...base,
        id: `builtin/mariadb/${version}`,
        name: `MariaDB ${version}`,
      });
    case "picodata":
      return create(PackageSchema, {
        ...base,
        id: `builtin/picodata/${version}`,
        name: `Picodata ${version}`,
      });
    case "ydb":
      return create(PackageSchema, {
        ...base,
        id: `builtin/ydb/${version}`,
        name: `YDB ${version}`,
      });
    case "cockroach":
      return create(PackageSchema, {
        ...base,
        id: `builtin/cockroach/${version}`,
        name: `CockroachDB ${version}`,
      });
  }
}

function packageFromRecord(row: PackageRow, engine: EngineKind, currentVersion: string): DatabasePackageVM {
  const version = row.version || currentVersion.trim() || "custom";
  const blobURL = row.storageUri
    ? `\${STROPPY_SERVER_ADDR%/}/api/packages/blob/${encodeURIComponent(row.storageUri)}`
    : "";
  return create(PackageSchema, {
    id: row.id,
    name: row.name || row.id,
    dbKind: ENGINE_TO_KIND[engine],
    dbVersion: version,
    isBuiltin: false,
    debFilename: blobURL,
    packageRecordId: row.id,
  });
}

function packageDbKind(kind: EngineKind): Exclude<DbKind, ""> | undefined {
  switch (kind) {
    case "postgres":
      return "postgres";
    case "mysql":
      return "mysql";
    case "mariadb":
      return "mariadb";
    case "picodata":
      return "picodata";
    case "ydb":
      return "ydb";
    case "ydbManaged":
      return undefined;
    case "cockroach":
      return "cockroach";
    case "external":
      return undefined;
  }
}

function packageSummary(pkg: DatabasePackageVM): string {
  if (pkg.aptPackages.length > 0) return pkg.aptPackages.join(", ");
  if (pkg.debFilename) return pkg.debFilename;
  if (pkg.packageRecordId) return `uploaded ${pkg.packageRecordId}`;
  return pkg.dbVersion || "package";
}

/** Compact numbered header shared by the three Database panes. */
function PaneHeader({
  index,
  title,
  subtitle,
  right,
}: {
  index: number;
  title: string;
  subtitle: string;
  right?: React.ReactNode;
}) {
  return (
    <div className="mb-3 flex items-center gap-2">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-primary/30 bg-primary/[0.08] font-mono text-[10px] text-primary">
        {index}
      </span>
      <div className="min-w-0">
        <div className="text-sm font-semibold leading-none text-foreground">{title}</div>
        <div className="mt-1 truncate text-[10px] font-mono uppercase tracking-wider text-zinc-600">{subtitle}</div>
      </div>
      {right}
    </div>
  );
}

/** A compact role-card topology diagram (mirrors old TopologyDiagram), derived
 * from the server's topology_spec + the machine plan specs. */
function TopologyDiagram({ draft }: { draft: WizardDraftVM }) {
  const specByNode = useMemo(() => {
    const m = new Map<string, MachineSpecVM>();
    for (const x of draft.infrastructurePlan.machines) m.set(x.nodeId, x.spec);
    return m;
  }, [draft.infrastructurePlan.machines]);

  if (draft.topologyComponents.length === 0) return null;

  return (
    <div className="flex min-h-0 flex-col border border-zinc-800/60 bg-surface-chrome p-3 xl:h-full">
      <div className="mb-2 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Derived topology — {draft.topologyNodes.length} node{draft.topologyNodes.length > 1 ? "s" : ""}
      </div>
      <div className="grid min-h-0 flex-1 auto-rows-min grid-cols-2 gap-2 overflow-y-auto pr-1 sm:grid-cols-3 xl:grid-cols-2">
        {draft.topologyComponents.map((c) => {
          const Icon = ROLE_ICON[c.role] ?? COMP_ICON[c.kind] ?? Box;
          const meta = ENGINES.find((e) => e.kind === c.engine);
          const color = meta?.hex ?? "#71717a";
          return (
            <div key={c.id} className="flex flex-col gap-1 border border-zinc-800 bg-surface-tile p-3">
              <div className="flex items-center gap-2">
                <Icon className="h-4 w-4" style={{ color }} />
                <span className="font-mono text-[11px] text-zinc-300">{c.role}</span>
              </div>
              <div className="font-mono text-[10px] text-zinc-600">{c.engine}</div>
              <div className="mt-0.5 font-mono text-[10px] text-zinc-500">{machineSpecSummary(specByNode.get(c.nodeId))}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ─── Step 3: Workload + ProbeScript ───────────────────────────────────────────

// The Workload step mirrors the Database step: a PROGRESSIVE flow laid out
// horizontally, each pane sliding in (pane-reveal) once enough of the previous
// is set. It fills the width (version | preset | parameters | probe) and the
// height (each pane scrolls internally). Version + Preset are narrow columns so
// Parameters + Probe keep room.
//
//   Pane 1 — Version    : choose the stroppy binary — a SUGGESTED release tag
//                         (StroppyProvider.listStroppyVersions) OR a COMMIT hash
//                         (free input, always available). Sets stroppy_version
//                         (tag verbatim, commit as "commit:<sha7>").
//   Pane 2 — Preset     : a workload preset that SEEDS the parameters (TPC-C,
//                         YCSB-A/B/C, pgbench, TPC-H, …) + a "Custom · configure
//                         from scratch" blank option. Slides in once a version
//                         is chosen. Picking one seeds + Patches workload (while
//                         PRESERVING the picked version); Custom resets to blank.
//   Pane 3 — Parameters : the typed domain.Workload editing (script, sql,
//                         protocol, k6 execution, data parameters), seeded from
//                         the chosen preset and fully editable. Slides in once a
//                         preset/Custom is chosen.
//   Pane 4 — Probe      : the ProbeScript results (phases allowlist → selected
//                         steps, env declarations → Parameters.env, -o human
//                         render). It OPENS THE MOMENT THE PROBE RESULT IS READY
//                         — it does NOT wait for the user to finish pane 3. The
//                         probe fires (debounced) as soon as there's a script +
//                         a version and re-fires when either changes.
//
// The composed WorkloadVM still flows through the unchanged patch({ workload }).

// ─── Preset draft memory (edit persistence + dirty + reset) ────────────────────
// Picking a preset used to overwrite the form outright: switch preset → back and
// your edits were gone. This keeps, per preset id, both the pristine baseline
// (to detect "dirty" and to reset) and the working copy (so switching away and
// back restores your edits). `normalize` strips fields the preset doesn't own
// (e.g. the workload's stroppy_version, chosen in a separate pane) from the
// comparison/storage so they don't read as edits.

interface PresetDraft<T> {
  /** Record the pristine baseline (first time only) and return the VM to load. */
  pick: (id: string, pristine: T) => T;
  /** Remember the working copy for the active preset. */
  edit: (id: string | null, vm: T) => void;
  /** Adopt a freshly-saved preset: its current VM becomes the new baseline. */
  markSaved: (id: string, vm: T) => void;
  /** Has the VM drifted from the picked preset's baseline? */
  isDirty: (id: string | null, vm: T) => boolean;
  /** The pristine baseline VM for a preset, if known. */
  baseline: (id: string | null) => T | null;
}

function usePresetDraft<T>(normalize: (v: T) => T): PresetDraft<T> {
  const baseline = useRef(new Map<string, T>());
  const working = useRef(new Map<string, T>());
  return useMemo(() => {
    const key = (v: T) => JSON.stringify(normalize(v));
    return {
      pick(id, pristine) {
        const norm = normalize(pristine);
        if (!baseline.current.has(id)) baseline.current.set(id, norm);
        return working.current.get(id) ?? norm;
      },
      edit(id, vm) {
        if (id) working.current.set(id, normalize(vm));
      },
      markSaved(id, vm) {
        const norm = normalize(vm);
        baseline.current.set(id, norm);
        working.current.set(id, norm);
      },
      isDirty(id, vm) {
        const b = id ? baseline.current.get(id) : undefined;
        return b !== undefined && key(b) !== key(vm);
      },
      baseline(id) {
        return id ? baseline.current.get(id) ?? null : null;
      },
    };
  }, [normalize]);
}

// stroppy_version is owned by the Version pane, not the preset, so exclude it
// from the workload preset's dirty/baseline comparison.
const normWorkload = (w: WorkloadVM): WorkloadVM => ({ ...w, stroppyVersion: "" });
const normDatabase = (d: DatabaseVM): DatabaseVM => d;

// A preset id that is real and editable (not the synthetic Custom tile or a
// resumed draft placeholder) — i.e. one we can offer "reset to preset" against.
function isRealPresetId(id: string | null): boolean {
  return (
    !!id &&
    id !== "__resumed__" &&
    id !== CUSTOM_DATABASE_PRESET_ID &&
    id !== CUSTOM_WORKLOAD_PRESET_ID
  );
}

// Dialog that names + saves the current edited VM as a brand-new preset.
function SavePresetDialog({
  open,
  onOpenChange,
  kind,
  defaultName,
  onSave,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  kind: "workload" | "database";
  defaultName: string;
  onSave: (name: string, description: string) => Promise<void>;
}) {
  const [name, setName] = useState(defaultName);
  const [description, setDescription] = useState("");
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  // Reseed the name each time the dialog opens.
  useEffect(() => {
    if (open) {
      setName(defaultName);
      setDescription("");
      setErr(null);
    }
  }, [open, defaultName]);

  const submit = async () => {
    if (!name.trim()) {
      setErr("A preset needs a name.");
      return;
    }
    setSaving(true);
    setErr(null);
    try {
      await onSave(name.trim(), description.trim());
      onOpenChange(false);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Save as {kind} preset</DialogTitle>
          <DialogDescription>
            Save the current {kind} configuration as a reusable preset in your library.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <Label>Name</Label>
            <Input
              className="mt-1"
              value={name}
              autoFocus
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") void submit();
              }}
              placeholder={`my-${kind}-preset`}
            />
          </div>
          <div>
            <Label>Description (optional)</Label>
            <Input
              className="mt-1"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What is this preset for?"
            />
          </div>
          {err && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" /> {err}
            </div>
          )}
          <div className="flex justify-end gap-2 pt-1">
            <Button variant="outline" size="sm" onClick={() => onOpenChange(false)} disabled={saving}>
              Cancel
            </Button>
            <Button size="sm" className="gap-1.5" onClick={() => void submit()} disabled={saving}>
              {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
              Save preset
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

// Compact "edited / reset / save" cluster shown in a pane header when a preset
// is active. Reset reverts to the preset's pristine baseline; Save persists the
// current edits as a new preset.
function PresetEditControls({
  dirty,
  canReset,
  onReset,
  onSave,
}: {
  dirty: boolean;
  canReset: boolean;
  onReset: () => void;
  onSave: () => void;
}) {
  return (
    <div className="ml-auto flex shrink-0 items-center gap-2">
      {dirty && <span className="font-mono text-[10px] uppercase tracking-wider text-amber-500">edited</span>}
      {canReset && (
        <button
          type="button"
          onClick={onReset}
          disabled={!dirty}
          title="Reset to the preset's values"
          className="flex items-center gap-1 text-[10px] font-mono text-zinc-500 transition-colors hover:text-zinc-300 disabled:opacity-30"
        >
          <RotateCcw className="h-3 w-3" /> reset
        </button>
      )}
      <button
        type="button"
        onClick={onSave}
        title="Save the current settings as a new preset"
        className="flex items-center gap-1 text-[10px] font-mono text-zinc-500 transition-colors hover:text-zinc-300"
      >
        <Save className="h-3 w-3" /> save as preset
      </button>
    </div>
  );
}

function StepWorkload({
  draft,
  patch,
  slug,
}: {
  draft: WizardDraftVM;
  patch: (input: { workload?: WorkloadVM }) => Promise<void>;
  slug: string;
}) {
  const engine = draft.database?.kind ?? "postgres";
  const [w, setW] = useState<WorkloadVM>(() => draft.workload ?? defaultWorkload(engine));
  // A resumed draft (or one with a non-empty stroppy_version) already has a
  // version → reveal panes 2/3 (and 4) immediately, with the preset implicitly
  // "chosen". A fresh draft starts at the version pane.
  const seeded = useRef(!!draft.workload);
  const [versionChosen, setVersionChosen] = useState<boolean>(
    !!(draft.workload && draft.workload.stroppyVersion.trim()),
  );
  // The preset pane gates the parameters pane (mirrors the DB step's presetId).
  // A resumed workload is treated as already past the preset pane.
  const [presetId, setPresetId] = useState<string | null>(draft.workload ? "__resumed__" : null);
  useEffect(() => {
    if (draft.workload && !seeded.current) {
      setW(draft.workload);
      if (draft.workload.stroppyVersion.trim()) setVersionChosen(true);
      setPresetId("__resumed__");
      seeded.current = true;
    }
  }, [draft.workload]);

  const drafts = usePresetDraft<WorkloadVM>(normWorkload);

  // setW + server patch — no preset-cache write (used by pick / reset).
  const commit = useCallback(
    (next: WorkloadVM) => {
      setW(next);
      void patch({ workload: next });
    },
    [patch],
  );

  // The form's edit path: commit AND remember the working copy for this preset
  // so switching presets away and back restores the user's edits.
  const apply = useCallback(
    (next: WorkloadVM) => {
      commit(next);
      drafts.edit(presetId, next);
    },
    [commit, drafts, presetId],
  );

  const setVersion = useCallback(
    (version: string) => {
      setVersionChosen(true);
      apply({ ...w, stroppyVersion: version });
    },
    [apply, w],
  );

  // Pane 2 — pick a workload preset (or Custom/blank). Restores prior edits for
  // that preset, records its pristine baseline, and PRESERVES the chosen stroppy
  // version (presets carry stroppyVersion "").
  const pickPreset = useCallback(
    (preset: WorkloadPresetVM) => {
      const loaded = drafts.pick(preset.id, { ...preset.workload, stroppyVersion: "" });
      setPresetId(preset.id);
      commit({ ...loaded, stroppyVersion: w.stroppyVersion });
    },
    [drafts, commit, w.stroppyVersion],
  );

  // Dirty / reset / save-as-preset for the picked workload preset.
  const dirty = drafts.isDirty(presetId, w);
  const resetToPreset = useCallback(() => {
    const base = drafts.baseline(presetId);
    if (base) apply({ ...base, stroppyVersion: w.stroppyVersion });
  }, [drafts, presetId, apply, w.stroppyVersion]);

  const [saveOpen, setSaveOpen] = useState(false);
  const [presetReload, setPresetReload] = useState(0);
  const saveAsPreset = useCallback(
    async (name: string, description: string) => {
      const id = await getPresetProvider().createWorkloadPreset(slug, {
        name,
        description,
        tags: {},
        workload: { ...w, stroppyVersion: "" },
      });
      drafts.markSaved(id, w);
      setPresetId(id);
      setPresetReload((n) => n + 1); // refetch the preset list so it shows up
    },
    [slug, w, drafts],
  );

  // Each segment editor runs its own debounced `stroppy probe` for its script;
  // this context supplies the shared inputs (slug, engine, chosen version).
  const probeContext = useMemo<SegmentProbeContext>(
    () => ({ slug, engine, version: w.stroppyVersion }),
    [slug, engine, w.stroppyVersion],
  );

  const errs = errorsFor(draft.errors, "workload");

  return (
    <div className="mx-auto flex h-full w-full max-w-[120rem] flex-col">
      <SectionTitle hint="Pick a stroppy build, choose a workload preset, tune its execution and data parameters, then probe the script for phases and environment variables.">
        Workload
      </SectionTitle>

      {/* Progressive panes — mirrors the Database step. Version + Preset are
          fixed-width columns; the third slot ALWAYS grows: a flex-1 main region
          holding Parameters (+ Probe once ready), or a fill-width empty state
          until a preset is chosen. Keeping it to a single growing region (rather
          than two flex siblings) means there are never more than two fixed
          columns, so the row neither leaves dead space nor overflows when the
          window is narrow. */}
      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:items-stretch">
        {/* Pane 1 — Version (compact column on the left) */}
        <WorkloadVersionPane
          slug={slug}
          value={versionChosen ? w.stroppyVersion : ""}
          onPick={setVersion}
        />

        {/* Pane 2 — Preset (slides in once a version is chosen) */}
        {versionChosen && (
          <WorkloadPresetPane
            slug={slug}
            engine={engine}
            selectedId={presetId}
            onPick={pickPreset}
            reloadKey={presetReload}
          />
        )}

        {/* Pane 3 — Parameters. Probe-declared phases/env are folded into the
            form; the probe itself is just a collapsed status/output disclosure
            at the foot of the pane (no longer a competing column). */}
        {versionChosen && presetId ? (
          <WorkloadParametersPane
            w={w}
            apply={apply}
            errs={errs}
            probeContext={probeContext}
            dirty={dirty}
            canReset={isRealPresetId(presetId)}
            onReset={resetToPreset}
            onSave={() => setSaveOpen(true)}
          />
        ) : (
          <PreviewAside
            icon={versionChosen ? Layers : Tag}
            title={versionChosen ? "Pick a workload preset" : "Pick a stroppy version"}
            hint={
              versionChosen
                ? "Choose a preset (or Custom) to edit the execution and data parameters; the probe runs automatically."
                : "Select a release tag or commit on the left to choose the stroppy build."
            }
          />
        )}
      </div>

      <SavePresetDialog
        open={saveOpen}
        onOpenChange={setSaveOpen}
        kind="workload"
        defaultName={draft.name ? `${draft.name} workload` : ""}
        onSave={saveAsPreset}
      />
    </div>
  );
}

// ─── Workload pane 1: stroppy version picker (left column) ─────────────────────

function WorkloadVersionPane({
  slug,
  value,
  onPick,
}: {
  slug: string;
  value: string;
  onPick: (version: string) => void;
}) {
  const [versions, setVersions] = useState<string[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const commit = isCommitVersion(value);
  const [mode, setMode] = useState<"release" | "commit">(commit ? "commit" : "release");
  const [sha, setSha] = useState<string>(commitSha(value));

  useEffect(() => {
    let cancelled = false;
    setVersions(null);
    setErr(null);
    getStroppyProvider()
      .listStroppyVersions(slug)
      .then((v) => !cancelled && setVersions(v))
      .catch((e) => !cancelled && setErr(e instanceof Error ? e.message : String(e)));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const releaseSelected = !commit ? value : "";

  return (
    <div className="flex min-h-0 shrink-0 flex-col lg:w-64">
      <PaneHeader index={1} title="Version" subtitle="stroppy binary — release or commit" />

      {/* release | commit toggle */}
      <div className="mb-3 inline-flex shrink-0 border border-zinc-800 text-[11px] font-mono">
        <button
          type="button"
          onClick={() => setMode("release")}
          className={`flex items-center gap-1.5 px-3 py-1 transition-colors ${
            mode === "release" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
          }`}
        >
          <Tag className="h-3 w-3" /> release
        </button>
        <button
          type="button"
          onClick={() => setMode("commit")}
          className={`flex items-center gap-1.5 border-l border-zinc-800 px-3 py-1 transition-colors ${
            mode === "commit" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
          }`}
        >
          <GitCommit className="h-3 w-3" /> commit
        </button>
      </div>

      {mode === "release" ? (
        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          {err && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-3.5 w-3.5" /> {err}
            </div>
          )}
          {!versions && !err && (
            <div className="flex items-center gap-2 px-1 py-3 text-xs text-zinc-600">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading versions…
            </div>
          )}
          {versions && (
            <div className="space-y-2">
              {versions.map((v) => {
                const sel = releaseSelected === v;
                return (
                  <button
                    key={v}
                    type="button"
                    onClick={() => onPick(v)}
                    className={`flex w-full items-center gap-2.5 border p-3 text-left transition-all ${
                      sel
                        ? "border-primary/50 bg-primary/[0.07]"
                        : "border-zinc-800 bg-surface-tile hover:border-zinc-700 hover:bg-zinc-900/50"
                    }`}
                  >
                    <Tag className={`h-4 w-4 shrink-0 ${sel ? "text-primary" : "text-zinc-500"}`} />
                    <span className={`font-mono text-xs font-medium ${sel ? "text-primary" : "text-foreground"}`}>{v}</span>
                    {sel && <Check className="ml-auto h-3.5 w-3.5 shrink-0 text-primary" />}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <Label>Commit hash</Label>
          <Input
            className="mt-1 font-mono text-xs"
            value={sha}
            spellCheck={false}
            placeholder="short SHA (7+ hex)"
            onChange={(e) => {
              const raw = e.target.value.trim().toLowerCase().slice(0, 40).replace(/[^0-9a-f]/g, "");
              setSha(raw);
              onPick(commitVersion(raw));
            }}
          />
          <p className="mt-2 text-[11px] text-zinc-600">
            Builds stroppy from a specific CI commit.
          </p>
          {value && commit && (
            <div className="mt-3 flex items-center gap-2 border border-primary/30 bg-primary/[0.06] px-3 py-2">
              <GitCommit className="h-3.5 w-3.5 text-primary" />
              <span className="font-mono text-[11px] text-primary">{value}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ─── Workload pane 2: workload-preset picker (slides in) ───────────────────────
//
// Mirrors the Database step's PresetPane: lists the workload presets (builtin +
// the "Custom · configure from scratch" blank) and, on pick, SEEDS the typed
// WorkloadVM the parameters pane edits. Workloads are engine-agnostic, so unlike
// the DB preset pane this is not scoped to the chosen engine.

function WorkloadPresetPane({
  slug,
  engine,
  selectedId,
  onPick,
  reloadKey = 0,
}: {
  slug: string;
  engine: EngineKind;
  selectedId: string | null;
  onPick: (p: WorkloadPresetVM) => void;
  /** Bumped after a save-as-preset so the freshly created preset appears here. */
  reloadKey?: number;
}) {
  const [presets, setPresets] = useState<WorkloadPresetVM[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPresets(null);
    setErr(null);
    getPresetProvider()
      .listWorkloadPresets(slug)
      .then((p) => !cancelled && setPresets(p))
      .catch((e) => !cancelled && setErr(e instanceof Error ? e.message : String(e)));
    return () => {
      cancelled = true;
    };
  }, [slug, reloadKey]);

  const items = useMemo<WorkloadPresetVM[] | null>(() => {
    if (!presets) return null;
    const custom: WorkloadPresetVM = {
      id: CUSTOM_WORKLOAD_PRESET_ID,
      name: "Custom",
      description: "Configure workload settings manually.",
      isSystem: false,
      workload: defaultWorkload(engine),
    };
    return [custom, ...presets.filter((preset) => preset.id !== CUSTOM_WORKLOAD_PRESET_ID)];
  }, [engine, presets]);

  return (
    <div className="pane-reveal flex min-h-0 shrink-0 flex-col lg:w-72">
      <PaneHeader index={2} title="Preset" subtitle="Workload — seeds the parameters" />
      {err && (
        <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
          <AlertCircle className="h-3.5 w-3.5" /> {err}
        </div>
      )}
      {!presets && !err && (
        <div className="flex items-center gap-2 px-1 py-3 text-xs text-zinc-600">
          <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading presets…
        </div>
      )}
      {items && (
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {items.map((p) => {
            const sel = selectedId === p.id;
            const custom = p.id === CUSTOM_WORKLOAD_PRESET_ID;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => onPick(p)}
                className={`flex w-full items-start gap-2.5 border p-3 text-left transition-all ${
                  sel
                    ? "border-primary/50 bg-primary/[0.07]"
                    : "border-zinc-800 bg-surface-tile hover:border-zinc-700 hover:bg-zinc-900/50"
                }`}
              >
                {custom ? (
                  <Pencil className={`mt-0.5 h-4 w-4 shrink-0 ${sel ? "text-primary" : "text-zinc-500"}`} />
                ) : (
                  <Layers className={`mt-0.5 h-4 w-4 shrink-0 ${sel ? "text-primary" : "text-zinc-500"}`} />
                )}
                <div className="min-w-0">
                  <div className={`flex items-center gap-1.5 text-xs font-medium ${sel ? "text-primary" : "text-foreground"}`}>
                    {p.name}
                    {p.isSystem && (
                      <span className="shrink-0 bg-zinc-800/70 px-1 py-px font-mono text-[8px] uppercase tracking-wide text-zinc-500">
                        builtin
                      </span>
                    )}
                  </div>
                  <div className="mt-0.5 text-[11px] leading-snug text-zinc-600">{p.description}</div>
                </div>
                {sel && <Check className="ml-auto mt-0.5 h-3.5 w-3.5 shrink-0 text-primary" />}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ─── Workload pane 3: typed parameters form (slides in) ────────────────────────

function WorkloadParametersPane({
  w,
  apply,
  errs,
  probeContext,
  dirty,
  canReset,
  onReset,
  onSave,
}: {
  w: WorkloadVM;
  apply: (w: WorkloadVM) => void;
  errs: DraftErrorVM[];
  probeContext: SegmentProbeContext;
  dirty: boolean;
  canReset: boolean;
  onReset: () => void;
  onSave: () => void;
}) {
  return (
    <div className="pane-reveal flex min-h-0 min-w-0 flex-1 flex-col">
      <PaneHeader
        index={3}
        title="Parameters"
        subtitle="Segments, execution and data settings"
        right={<PresetEditControls dirty={dirty} canReset={canReset} onReset={onReset} onSave={onSave} />}
      />
      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        <WorkloadParamsForm w={w} apply={apply} probeContext={probeContext} />
        <FieldErrors errs={errs} />
      </div>
    </div>
  );
}

// ─── Step 4: Review / Render (editable artifacts + Finish) ─────────────────────

const COMP_ICON: Record<string, typeof Database> = {
  database: Database,
  replica: Database,
  proxy: Network,
  coordinator: Cpu,
  workload: Zap,
  monitor: Server,
  external: Cloud,
};

const KIND_ICON: Record<RenderArtifact_Kind, typeof FileText> = {
  [RenderArtifact_Kind.UNSPECIFIED]: FileText,
  [RenderArtifact_Kind.FILE]: FileText,
  [RenderArtifact_Kind.COMMAND]: Terminal,
  [RenderArtifact_Kind.DIRECTORY]: Box,
  [RenderArtifact_Kind.RUNTIME_VALUE]: Cloud,
};

type ArtifactCategoryKey = "editable" | "commands" | "generated" | "directories" | "runtime" | "other";

const ARTIFACT_CATEGORIES: {
  key: ArtifactCategoryKey;
  title: string;
  hint: string;
  icon: typeof FileText;
}[] = [
  { key: "editable", title: "Editable configs", hint: "Files that can be overridden before launch.", icon: Pencil },
  { key: "commands", title: "Commands", hint: "Commands the launcher renders for execution.", icon: Terminal },
  { key: "generated", title: "Generated files", hint: "Rendered read-only files used by the deployment.", icon: FileText },
  { key: "directories", title: "Directories", hint: "Directories the deployment expects to create or use.", icon: Box },
  { key: "runtime", title: "Runtime values", hint: "Values resolved only when the run starts.", icon: Cloud },
  { key: "other", title: "Other output", hint: "Unclassified generated artifacts.", icon: Tag },
];

function artifactCategory(a: RenderArtifactVM): ArtifactCategoryKey {
  if (a.kind === RenderArtifact_Kind.COMMAND) return "commands";
  if (a.kind === RenderArtifact_Kind.DIRECTORY) return "directories";
  if (a.kind === RenderArtifact_Kind.RUNTIME_VALUE || a.mutability === RenderArtifact_Mutability.RUNTIME_ONLY) {
    return "runtime";
  }
  if (a.kind === RenderArtifact_Kind.FILE && a.mutability === RenderArtifact_Mutability.EDITABLE) return "editable";
  if (a.kind === RenderArtifact_Kind.FILE) return "generated";
  return "other";
}

function StepReview({
  draft,
  slug,
  patch,
  onDone,
}: {
  draft: WizardDraftVM;
  slug: string;
  patch: (input: { renderOverrides?: FileOverrideVM[] }) => Promise<void>;
  onDone: (runId: string) => void;
}) {
  const [start, setStart] = useState(true);
  const [saveAsPreset, setSaveAsPreset] = useState(false);
  const [presetName, setPresetName] = useState(draft.name);
  const [inTenantRating, setInTenantRating] = useState(true);
  const [inGlobalRating, setInGlobalRating] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const blocking = draft.errors.filter((e) => e.severity === "error");

  // The active override set lives on the draft (artifacts with origin =
  // USER_OVERRIDE). Build the next set, then re-Patch render_overrides.
  const currentOverrides = useCallback((): FileOverrideVM[] => {
    return draft.artifacts
      .filter((a) => a.origin === RenderArtifact_Origin.USER_OVERRIDE)
      .map((a) => ({
        artifactId: a.id,
        componentId: a.componentId,
        baseHash: a.baseHash,
        path: a.title,
        content: a.content,
      }));
  }, [draft.artifacts]);

  const saveOverride = useCallback(
    (a: RenderArtifactVM, content: string) => {
      const next = currentOverrides().filter((o) => o.artifactId !== a.id);
      next.push({ artifactId: a.id, componentId: a.componentId, baseHash: a.baseHash, path: a.title, content });
      void patch({ renderOverrides: next });
    },
    [currentOverrides, patch],
  );

  const resetOverride = useCallback(
    (a: RenderArtifactVM) => {
      const next = currentOverrides().filter((o) => o.artifactId !== a.id);
      void patch({ renderOverrides: next });
    },
    [currentOverrides, patch],
  );

  const finish = useCallback(async () => {
    setSubmitting(true);
    setErr(null);
    try {
      const res = await getWizardProvider().finish(slug, draft.id, {
        start,
        saveAsPreset,
        presetName: presetName.trim(),
        inTenantRating: start ? inTenantRating : undefined,
        inGlobalRating: start ? inGlobalRating : undefined,
      });
      onDone(res.runId);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSubmitting(false);
    }
  }, [slug, draft.id, start, saveAsPreset, presetName, inTenantRating, inGlobalRating, onDone]);

  const componentOrder = useMemo(() => {
    const ids = new Set<string>();
    for (const c of draft.renderComponents) ids.add(c.componentId);
    for (const a of draft.artifacts) ids.add(a.componentId);
    return [...ids];
  }, [draft.artifacts, draft.renderComponents]);

  const artifactCategories = useMemo(() => {
    const buckets = new Map<ArtifactCategoryKey, Map<string, RenderArtifactVM[]>>();
    for (const a of draft.artifacts) {
      const key = artifactCategory(a);
      const byComponent = buckets.get(key) ?? new Map<string, RenderArtifactVM[]>();
      const list = byComponent.get(a.componentId) ?? [];
      list.push(a);
      byComponent.set(a.componentId, list);
      buckets.set(key, byComponent);
    }
    return ARTIFACT_CATEGORIES.map((cat) => {
      const byComponent = buckets.get(cat.key);
      const components = componentOrder
        .map((componentId) => ({ componentId, list: byComponent?.get(componentId) ?? [] }))
        .filter((group) => group.list.length > 0);
      const count = components.reduce((acc, group) => acc + group.list.length, 0);
      return { ...cat, count, components };
    }).filter((cat) => cat.count > 0);
  }, [componentOrder, draft.artifacts]);

  const overrideCount = draft.artifacts.filter((a) => a.origin === RenderArtifact_Origin.USER_OVERRIDE).length;
  const [showArtifacts, setShowArtifacts] = useState(overrideCount > 0);
  useEffect(() => {
    if (overrideCount > 0) setShowArtifacts(true);
  }, [overrideCount]);

  const machineGroups = useMemo(() => groupReviewMachines(draft.infrastructurePlan.machines), [draft.infrastructurePlan.machines]);
  const workloadLimit = workloadExecutionLabel(draft.workload);
  const packageLabel = draft.database?.installPackage?.name || draft.database?.packageId || "—";

  return (
    <div className="mx-auto flex h-full w-full max-w-[120rem] flex-col">
      <SectionTitle hint="Review the selected provider, database, workload and launch options. Generated output is grouped below for troubleshooting or manual overrides.">
        Review
      </SectionTitle>

      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
      {/* Summary cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <ReviewCard title="Provider" value={providerDisplay(draft.provider)} />
        <ReviewCard title="Database" value={draft.database ? `${ENGINES.find((e) => e.kind === draft.database!.kind)?.label} ${draft.database.version}` : "—"} />
        <ReviewCard title="Workload" value={workloadScriptLabel(draft.workload)} />
        <ReviewCard title="Execution" value={workloadExecutionLabel(draft.workload)} />
        <ReviewCard title="Nodes" value={String(draft.topologyNodes.length)} />
        <ReviewCard title="Overrides" value={overrideCount ? `${overrideCount} edited` : "none"} />
      </div>

      <div className="mt-4 grid gap-3 xl:grid-cols-[minmax(0,1fr)_minmax(24rem,0.7fr)]">
        <div className="border border-zinc-800 bg-surface-tile p-3">
          <div className="mb-3 flex items-center gap-2">
            <Rocket className="h-4 w-4 text-zinc-400" />
            <div>
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Will execute</div>
              <div className="mt-0.5 text-[11px] text-zinc-500">{draft.ready ? "Ready to launch" : "Waiting for valid test input"}</div>
            </div>
          </div>
          <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
            <PlanFact label="Provider" value={providerDisplay(draft.provider)} />
            <PlanFact label="Topology" value={`${draft.topologyNodes.length} node${draft.topologyNodes.length === 1 ? "" : "s"}`} />
            <PlanFact label="Workload run" value={workloadLimit} />
            <PlanFact label="Script" value={workloadScriptLabel(draft.workload)} />
            <PlanFact label="Protocol" value={protocolDisplay(draft.workload)} />
            <PlanFact label="Stroppy" value={draft.workload?.stroppyVersion || "—"} />
          </div>
          <div className="mt-3 grid gap-2 lg:grid-cols-2">
            {machineGroups.length > 0 ? (
              machineGroups.map((group) => (
                <div key={group.key} className="border border-zinc-800/80 bg-black/10 px-3 py-2">
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0 truncate text-sm text-zinc-300">{group.label}</div>
                    <span className="shrink-0 font-mono text-[10px] text-zinc-600">x{group.count}</span>
                  </div>
                  <div className="mt-1 truncate font-mono text-[11px] text-zinc-500">{group.spec}</div>
                </div>
              ))
            ) : (
              <div className="border border-dashed border-zinc-800 px-3 py-4 text-center text-xs text-zinc-600 lg:col-span-2">
                No machine plan yet.
              </div>
            )}
          </div>
        </div>

        <div className="border border-zinc-800 bg-surface-tile p-3">
          <div className="mb-3 flex items-center gap-2">
            <GitCommit className="h-4 w-4 text-zinc-400" />
            <div>
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Test definition</div>
              <div className="mt-0.5 text-[11px] text-zinc-500">Database, package and workload selected for this run.</div>
            </div>
          </div>
          <div className="grid gap-2">
            <PlanFact label="Database" value={databaseDisplay(draft.database)} />
            <PlanFact label="Package" value={packageLabel} />
            <PlanFact label="Scale factor" value={firstSegment(draft.workload) ? String(firstSegment(draft.workload)!.parameters.scaleFactor) : "—"} />
            <PlanFact label="Pool size" value={firstSegment(draft.workload) ? String(firstSegment(draft.workload)!.parameters.poolSize) : "—"} />
            <PlanFact label="Steps" value={workloadStepsLabel(draft.workload)} />
            <PlanFact label="Files" value={draft.workload ? String(draft.workload.segments.reduce((n, s) => n + s.files.length, 0)) : "—"} />
          </div>
        </div>
      </div>

      {/* Render artifacts, categorized by type and grouped by component inside each category. */}
      {draft.artifacts.length > 0 && (
        <div className="mt-6">
          <button
            type="button"
            onClick={() => setShowArtifacts((v) => !v)}
            className="flex w-full items-center justify-between gap-3 border border-zinc-800 bg-surface-tile px-3 py-2 text-left transition-colors hover:bg-zinc-900/40"
          >
            <div className="min-w-0">
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Advanced generated output</div>
              <div className="mt-0.5 text-[11px] text-zinc-500">
                {draft.artifacts.length} artifact{draft.artifacts.length === 1 ? "" : "s"} in {artifactCategories.length} categor{artifactCategories.length === 1 ? "y" : "ies"}.
              </div>
            </div>
            <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-zinc-600 transition-transform ${showArtifacts ? "rotate-90" : ""}`} />
          </button>
          {showArtifacts && (
            <div className="mt-3 space-y-4">
              {artifactCategories.map((category) => {
                const CategoryIcon = category.icon;
                return (
                  <div key={category.key} className="border border-zinc-800/80 bg-black/10 p-3">
                    <div className="mb-3 flex items-start justify-between gap-3">
                      <div className="flex min-w-0 items-start gap-2">
                        <CategoryIcon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-zinc-500" />
                        <div className="min-w-0">
                          <div className="font-mono text-[12px] uppercase tracking-wider text-zinc-400">{category.title}</div>
                          <div className="mt-0.5 text-[11px] text-zinc-600">{category.hint}</div>
                        </div>
                      </div>
                      <span className="shrink-0 bg-zinc-900 px-1.5 py-0.5 font-mono text-[10px] text-zinc-500">{category.count}</span>
                    </div>
                    <div className="grid gap-4 xl:grid-cols-2 xl:items-start">
                      {category.components.map(({ componentId, list }) => {
                        const comp = draft.topologyComponents.find((c) => c.id === componentId);
                        const Icon = comp ? COMP_ICON[comp.kind] ?? Box : Box;
                        return (
                          <div key={`${category.key}:${componentId}`} className="min-w-0">
                            <div className="mb-1.5 flex items-center gap-2">
                              <Icon className="h-3.5 w-3.5 shrink-0 text-zinc-500" />
                              <span className="min-w-0 truncate font-mono text-[12px] text-zinc-400">{componentDisplay(comp, componentId)}</span>
                            </div>
                            <div className="space-y-2">
                              {list.map((a) => (
                                <ArtifactRow key={a.id} a={a} onSave={(c) => saveOverride(a, c)} onReset={() => resetOverride(a)} />
                              ))}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {blocking.length > 0 && (
        <div className="mt-5 border border-red-900/50 bg-red-950/20 p-3">
          <div className="mb-1 flex items-center gap-1.5 text-xs font-medium text-red-400">
            <AlertCircle className="h-3.5 w-3.5" /> Resolve {blocking.length} error{blocking.length > 1 ? "s" : ""} before finishing
          </div>
          <FieldErrors errs={blocking} />
        </div>
      )}

      {/* Finish options */}
      <div className="mt-6 space-y-3">
        <ToggleRow label="Launch now" hint="Persist and start the run immediately." checked={start} onChange={setStart} />
        {start && (
          <div className="ml-1 grid grid-cols-1 gap-2 border-l border-zinc-800 pl-4 sm:grid-cols-2">
            <ToggleRow label="Tenant rating" hint="Include in tenant-level reports." checked={inTenantRating} onChange={setInTenantRating} />
            <ToggleRow label="Global rating" hint="Include in global reports." checked={inGlobalRating} onChange={setInGlobalRating} />
          </div>
        )}
        <ToggleRow label="Save as preset" hint="Keep this database and workload combination reusable." checked={saveAsPreset} onChange={setSaveAsPreset} />
        {saveAsPreset && (
          <div className="ml-1 border-l border-zinc-800 pl-4">
            <Label>Preset name</Label>
            <Input className="mt-1" value={presetName} onChange={(e) => setPresetName(e.target.value)} placeholder="empty → derived" />
          </div>
        )}
      </div>

      {err && (
        <div className="mt-4 flex items-center gap-2 text-sm text-red-400">
          <AlertCircle className="h-4 w-4" /> {err}
        </div>
      )}

      <div className="mt-6 flex items-center gap-2">
        <Button onClick={finish} disabled={submitting || !draft.ready || (!start && !saveAsPreset)} className="gap-1.5">
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : start ? <Rocket className="h-4 w-4" /> : <Save className="h-4 w-4" />}
          {start ? "Launch run" : "Save preset"}
        </Button>
        {!draft.ready && <span className="text-[11px] text-zinc-600">Finish is enabled once the draft is ready.</span>}
      </div>
      </div>
    </div>
  );
}

/** One render artifact. EDITABLE FILE artifacts open the ConfigEditor and feed
 * a FileOverride on save; everything else is shown locked / read-only. */
function ArtifactRow({
  a,
  onSave,
  onReset,
}: {
  a: RenderArtifactVM;
  onSave: (content: string) => void;
  onReset: () => void;
}) {
  const Icon = KIND_ICON[a.kind] ?? FileText;
  const editable = a.mutability === RenderArtifact_Mutability.EDITABLE && a.kind === RenderArtifact_Kind.FILE;
  const overridden = a.origin === RenderArtifact_Origin.USER_OVERRIDE;
  const hasContent = a.content.trim().length > 0;

  const [open, setOpen] = useState(false);
  const [text, setText] = useState(a.content);
  const lastContent = useRef(a.content);
  // Re-seed editor text whenever the server re-renders this artifact.
  useEffect(() => {
    if (a.content !== lastContent.current) {
      setText(a.content);
      lastContent.current = a.content;
    }
  }, [a.content]);
  const dirty = text !== a.content;

  return (
    <div className="border border-zinc-800 bg-surface-tile">
      <button
        type="button"
        onClick={() => (editable || hasContent) && setOpen((o) => !o)}
        className="flex w-full flex-wrap items-center gap-2 px-4 py-2.5 text-left"
      >
        <Icon className="h-4 w-4 shrink-0 text-zinc-400" />
        <span className="min-w-[8rem] flex-1 truncate font-mono text-[12px] text-zinc-300">{a.title}</span>
        {overridden && (
          <span className="inline-flex shrink-0 items-center gap-1 bg-primary/10 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide text-primary">
            <Pencil className="h-2.5 w-2.5" /> override
          </span>
        )}
        <span
          className={`inline-flex shrink-0 items-center gap-1 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide ${
            editable ? "bg-emerald-500/10 text-emerald-400" : "bg-zinc-800/60 text-zinc-500"
          }`}
        >
          {editable ? <Pencil className="h-2.5 w-2.5" /> : <Lock className="h-2.5 w-2.5" />}
          {MUTABILITY_LABEL[a.mutability]}
        </span>
        {(editable || hasContent) && (
          <ChevronRight className={`h-3.5 w-3.5 text-zinc-600 transition-transform ${open ? "rotate-90" : ""}`} />
        )}
      </button>

      {open && (
        <div className="border-t border-zinc-800/60">
          {a.lockReason && !editable && (
            <div className="flex items-center gap-1.5 px-4 py-2 text-[10px] text-zinc-500">
              <Lock className="h-3 w-3" /> {a.lockReason}
            </div>
          )}
          {editable ? (
            <div className="p-3">
              <ConfigEditor filename={a.title} value={text} height="14rem" onChange={setText} />
              <div className="mt-2 flex items-center gap-2">
                <Button size="sm" disabled={!dirty} onClick={() => onSave(text)} className="gap-1.5">
                  <Save className="h-3.5 w-3.5" /> Save override
                </Button>
                {overridden && (
                  <Button size="sm" variant="outline" onClick={onReset} className="gap-1.5">
                    <RotateCcw className="h-3.5 w-3.5" /> Reset to default
                  </Button>
                )}
                <span className="ml-auto font-mono text-[10px] text-zinc-700">base {a.baseHash || "—"}</span>
              </div>
            </div>
          ) : hasContent ? (
            <pre className="max-h-56 overflow-auto bg-black/40 p-3 font-mono text-[10px] leading-relaxed text-zinc-400">{a.content}</pre>
          ) : null}
        </div>
      )}
    </div>
  );
}

const MUTABILITY_LABEL: Record<RenderArtifact_Mutability, string> = {
  [RenderArtifact_Mutability.UNSPECIFIED]: "—",
  [RenderArtifact_Mutability.READ_ONLY]: "read-only",
  [RenderArtifact_Mutability.EDITABLE]: "editable",
  [RenderArtifact_Mutability.RUNTIME_ONLY]: "runtime",
};

function PlanFact({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 border border-zinc-800/80 bg-black/10 px-3 py-2">
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">{label}</div>
      <div className="mt-1 min-w-0 truncate text-sm text-zinc-300" title={value}>{value}</div>
    </div>
  );
}

function providerDisplay(provider: Provider): string {
  return (Provider[provider] ?? "UNSPECIFIED").replace("UNSPECIFIED", "—").replace("PROVIDER_", "").toLowerCase();
}

function databaseDisplay(db?: DatabaseVM): string {
  if (!db) return "—";
  const label = ENGINES.find((e) => e.kind === db.kind)?.label ?? db.kind;
  return [label, db.version].filter(Boolean).join(" ");
}

function protocolDisplay(workload?: WorkloadVM): string {
  if (!workload) return "—";
  return (Workload_Protocol[workload.protocol] ?? "PROTOCOL_UNSPECIFIED").replace("PROTOCOL_", "").toLowerCase();
}

/** The first (primary) segment — drives the single-line review summaries. */
function firstSegment(workload?: WorkloadVM) {
  return workload?.segments[0];
}

/** Script label for the run summary: the first segment's script + a "+N" hint. */
function workloadScriptLabel(workload?: WorkloadVM): string {
  const seg = firstSegment(workload);
  if (!workload || !seg) return "—";
  const head = seg.script || "—";
  return workload.segments.length > 1 ? `${head} +${workload.segments.length - 1}` : head;
}

function workloadExecutionLabel(workload?: WorkloadVM): string {
  const seg = firstSegment(workload);
  if (!seg) return "—";
  const limit =
    seg.execution.limit.case === "duration"
      ? seg.execution.limit.duration
      : `${seg.execution.limit.iterations} iterations`;
  const suffix = workload && workload.segments.length > 1 ? ` · ${workload.segments.length} segments` : "";
  return `${seg.execution.vus} VU · ${limit}${suffix}`;
}

function workloadStepsLabel(workload?: WorkloadVM): string {
  const seg = firstSegment(workload);
  if (!seg) return "—";
  if (seg.parameters.steps.length > 0) return seg.parameters.steps.join(", ");
  if (seg.parameters.noSteps.length > 0) return `all except ${seg.parameters.noSteps.join(", ")}`;
  return "all";
}

function componentDisplay(
  comp: WizardDraftVM["topologyComponents"][number] | undefined,
  fallback: string,
): string {
  if (!comp) return fallback;
  const label = [comp.engine, comp.role || comp.kind].filter(Boolean).join(" ");
  return label ? `${label} · ${fallback}` : fallback;
}

function groupReviewMachines(machines: InfrastructurePlanVM["machines"]) {
  const groups = new Map<string, { key: string; label: string; spec: string; count: number }>();
  for (const machine of machines) {
    const spec = machineSpecSummary(machine.spec);
    const label = [machine.engine, machine.role].filter(Boolean).join(" ") || machine.nodeId || "machine";
    const key = `${label}:${spec}`;
    const current = groups.get(key);
    if (current) {
      current.count += 1;
    } else {
      groups.set(key, { key, label, spec, count: 1 });
    }
  }
  return [...groups.values()].sort((a, b) => a.label.localeCompare(b.label));
}

function ReviewCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="border border-zinc-800 bg-surface-tile p-3">
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">{title}</div>
      <div className="mt-1 truncate text-sm text-foreground">{value}</div>
    </div>
  );
}
