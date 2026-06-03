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
//        (MachinePlan → Docker.Container | Yandex.Vm) → .infrastructure_plan.
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

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import {
  getWizardProvider,
  ENGINES,
  Provider,
  Workload_Protocol,
  RenderArtifact_Kind,
  RenderArtifact_Origin,
  RenderArtifact_Mutability,
  Yandex_Settings_PlatformId,
  defaultWorkload,
  driverTypeFor,
  type WizardDraftVM,
  type DraftSummaryVM,
  type DatabaseVM,
  type WorkloadVM,
  type EngineKind,
  type ProbeMetaVM,
  type DraftErrorVM,
  type InfrastructurePlanVM,
  type MachineVM,
  type MachineSpecVM,
  type ProviderSettingsVM,
  type RenderArtifactVM,
  type FileOverrideVM,
} from "@/services/wizard";
import {
  getPresetProvider,
  type DatabasePresetVM,
  type WorkloadPresetVM,
} from "@/services/preset";
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
import { WorkloadParamsForm } from "@/components/workload/WorkloadParamsForm";
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
import { ConfigEditor } from "@/components/ui/config-editor";
import {
  NumericSlider,
  DiskTypeSelect,
  cpuStepsForPlatform,
  ramSteps,
  diskStepsForType,
  platformLimits,
  YC_PLATFORMS,
  SliderField,
} from "@/components/ui/sliders";
import {
  Check,
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
} from "lucide-react";

// ─── Step model ──────────────────────────────────────────────────────────────

type StepKey = "infra" | "database" | "workload" | "review";

const STEPS: { key: StepKey; label: string }[] = [
  { key: "database", label: "Database" },
  { key: "workload", label: "Workload" },
  { key: "infra", label: "Infrastructure" },
  { key: "review", label: "Review" },
];

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
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Step</div>
      <h2 className="text-lg font-semibold tracking-tight text-foreground">{children}</h2>
      {hint && <p className="mt-1 text-sm text-muted-foreground">{hint}</p>}
    </div>
  );
}

// ─── Main ────────────────────────────────────────────────────────────────────

export function NewRun() {
  const slug = useTenantSlug() ?? "";
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();

  const draftId = params.get("draft") ?? "";
  const stepKey = (params.get("step") as StepKey) || "database";
  const stepIndex = Math.max(0, STEPS.findIndex((s) => s.key === stepKey));

  const [draft, setDraft] = useState<WizardDraftVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [patching, setPatching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Resume picker (List/Get/Delete) shown when no draft is active.
  const [resume, setResume] = useState<DraftSummaryVM[]>([]);
  const [startName, setStartName] = useState("");

  useBreadcrumbLabel("draft", draft ? draft.name || "New Run" : undefined);

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

  const goStep = useCallback(
    (key: StepKey) => {
      const next = new URLSearchParams(params);
      next.set("step", key);
      setParams(next);
    },
    [params, setParams],
  );

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
      let d = await getWizardProvider().start(slug, startName.trim() || "Untitled run");
      // Default a provider up front so the Database step's patch can compute an
      // initial topology + machine plan. The user revisits/changes the provider
      // in the Infrastructure step (which re-patches).
      if (d.provider === Provider.UNSPECIFIED) {
        d = await getWizardProvider().patch(slug, d.id, { provider: Provider.DOCKER });
      }
      setDraft(d);
      setActiveDraft(d.id, "database");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [slug, startName, setActiveDraft]);

  const patch = useCallback(
    async (input: Parameters<ReturnType<typeof getWizardProvider>["patch"]>[2]) => {
      if (!slug || !draftId) return;
      setPatching(true);
      setError(null);
      try {
        const d = await getWizardProvider().patch(slug, draftId, input);
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
      await getWizardProvider().remove(slug, id);
      setResume((prev) => prev.filter((d) => d.id !== id));
    },
    [slug],
  );

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
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
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
          {stepKey === "infra" && <StepInfra draft={draft} patch={patch} />}
          {stepKey === "database" && <StepDatabase draft={draft} patch={patch} />}
          {stepKey === "workload" && <StepWorkload draft={draft} patch={patch} slug={slug} />}
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
      <div className="flex shrink-0 items-center justify-between border-t border-zinc-800 bg-[#070707] px-6 py-3 lg:px-8">
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

      <div className="mt-6 border border-zinc-800 bg-[#0a0a0a] p-5">
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
          Creates a draft (StartTestWizard). You can rename it later.
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

/** Map the proto Yandex platform enum (from the server-resolved settings) to the
 * slider's platform key string — read-only, used to bound the VM sliders. */
const PLATFORM_ENUM_TO_KEY: Record<number, keyof typeof YC_PLATFORMS> = {
  [Yandex_Settings_PlatformId.STANDARD_V2]: "standard-v2",
  [Yandex_Settings_PlatformId.STANDARD_V3]: "standard-v3",
  [Yandex_Settings_PlatformId.HIGHFREQ_V3]: "highfreq-v3",
};
function platformKey(s: ProviderSettingsVM): keyof typeof YC_PLATFORMS {
  if (s.case !== "yandex") return "standard-v3";
  return PLATFORM_ENUM_TO_KEY[s.yandex.platformId] ?? "standard-v3";
}

function StepInfra({
  draft,
  patch,
}: {
  draft: WizardDraftVM;
  patch: (input: { provider?: Provider; infrastructurePlan?: InfrastructurePlanVM }) => Promise<void>;
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
    void patch({ infrastructurePlan: next });
  };

  const pickProvider = (p: Provider) => {
    if (p === plan.provider) return;
    // Provider switch: let the server recompute settings + machine variants.
    void patch({ provider: p });
  };

  // Platform key for the VM sliders comes from the server-resolved settings
  // (tenant default) — read-only here, the wizard does not edit it.
  const pkey = platformKey(plan.settings);
  const provErrs = errorsFor(draft.errors, "provider").concat(errorsFor(draft.errors, "infrastructure_plan"));

  return (
    <div className="flex h-full flex-col">
      <SectionTitle hint="Where to run, and the per-machine specs for the now-complete node set (Database + Workload produced it). Provider-level settings (cloud/folder/zone/network/image/…) are tenant defaults resolved server-side — only the provider choice and per-node sizing are edited here. Editing re-Patches PatchTestWizard.infrastructure_plan.machines.">
        Infrastructure
      </SectionTitle>

      <div className="grid min-h-0 flex-1 gap-5 lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)] lg:items-stretch">
        <div className="space-y-5 overflow-y-auto">
          {/* Identity */}
          <div className="border border-zinc-800/60 bg-[#070707] px-3 py-2.5">
            <div className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">Run</div>
            <div className="mt-1 font-mono text-sm text-zinc-200">{draft.name || "Untitled run"}</div>
            <div className="mt-0.5 text-[10px] font-mono text-zinc-700">draft {draft.id}</div>
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
                      active ? "border-primary/40 bg-primary/[0.06] text-primary" : "border-zinc-800/60 hover:border-zinc-700 hover:bg-zinc-900/50"
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

        {/* Per-machine specs (infrastructure_plan.machines) — fills the height,
            scrolls internally, and uses the full width with more grid columns. */}
        <div className="flex min-h-0 flex-col">
          {plan.machines.length > 0 ? (
            <>
              <div className="mb-2 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                Machine plan — {plan.machines.length} node{plan.machines.length > 1 ? "s" : ""}
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto pr-1">
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                  {plan.machines.map((m) => (
                    <MachineRow key={m.nodeId} m={m} platformKey={pkey} onChange={(spec) => setMachine(m.nodeId, spec)} />
                  ))}
                </div>
                <FieldErrors errs={provErrs} />
              </div>
            </>
          ) : (
            <div className="border border-zinc-800 bg-[#0a0a0a] p-5 text-center text-[12px] text-zinc-600">
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

/** One MachinePlan row: a Docker.Container or Yandex.Vm spec, edited with the
 * same sliders the old NewRun used for per-role machine sizing. */
function MachineRow({
  m,
  platformKey,
  onChange,
}: {
  m: MachineVM;
  platformKey: keyof typeof YC_PLATFORMS;
  onChange: (spec: MachineSpecVM) => void;
}) {
  const Icon = ROLE_ICON[m.role] ?? Box;
  const limits = platformLimits(platformKey);
  const spec = m.spec;

  return (
    <div className="border border-zinc-800 bg-[#0a0a0a] p-2.5">
      <div className="mb-2 flex items-center gap-1.5">
        <Icon className="h-3.5 w-3.5 shrink-0 text-zinc-400" />
        <span className="min-w-0 truncate font-mono text-[11px] text-zinc-300">{m.nodeId}</span>
        <span className="shrink-0 bg-zinc-800/60 px-1 py-0.5 font-mono text-[9px] uppercase tracking-wide text-zinc-500">{m.role}</span>
        <span className="ml-auto shrink-0 font-mono text-[9px] text-zinc-600">{spec.case === "yandex" ? "Yandex.Vm" : "Docker.Container"}</span>
      </div>

      {spec.case === "yandex" && (
        <div className="space-y-2">
          <div className="grid grid-cols-2 gap-2">
            <NumericSlider
              label="vCPU (cores)"
              value={spec.yandex.cores}
              min={2}
              max={limits.maxCores}
              onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, cores: v } })}
            />
            <NumericSlider
              label="RAM (GB)"
              value={spec.yandex.memoryGb}
              min={1}
              max={Math.round(limits.maxRamMb / 1024)}
              onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, memoryGb: v } })}
            />
          </div>
          <SliderField
            label="boot disk (GB)"
            value={spec.yandex.bootDiskGb}
            steps={diskStepsForType(spec.yandex.bootDiskType)}
            onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, bootDiskGb: v } })}
            format={(v) => `${v}`}
          />
          <DiskTypeSelect
            value={spec.yandex.bootDiskType}
            diskSizeGb={spec.yandex.bootDiskGb}
            onChange={(v) => onChange({ case: "yandex", yandex: { ...spec.yandex, bootDiskType: v } })}
          />
        </div>
      )}

      {spec.case === "docker" && (
        <div className="space-y-2">
          <div>
            <Label className="text-[9px] font-mono text-zinc-600">image</Label>
            <Input
              className="mt-1 h-6 font-mono text-[10px]"
              value={spec.docker.image}
              onChange={(e) => onChange({ case: "docker", docker: { ...spec.docker, image: e.target.value } })}
              placeholder="postgres:16"
            />
          </div>
          <div className="grid grid-cols-2 gap-2">
            <SliderField
              label="cpu cores"
              value={spec.docker.cpuCores}
              steps={[1, 2, 4, 8, 12, 16, 24, 32]}
              onChange={(v) => onChange({ case: "docker", docker: { ...spec.docker, cpuCores: v } })}
              format={(v) => `${v}`}
            />
            <SliderField
              label="memory (MB)"
              value={spec.docker.memoryMb}
              steps={[512, 1024, 2048, 4096, 8192, 16384, 32768, 65536]}
              onChange={(v) => onChange({ case: "docker", docker: { ...spec.docker, memoryMb: v } })}
              format={(v) => `${v}`}
            />
          </div>
        </div>
      )}
    </div>
  );
}

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
}: {
  draft: WizardDraftVM;
  patch: (input: { database?: DatabaseVM }) => Promise<void>;
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

  const apply = useCallback(
    (next: DatabaseVM) => {
      setDb(next);
      void patch({ database: next });
    },
    [patch],
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

  // Pane 2 — pick a preset (or Custom/blank). Seeds + Patches the settings pane.
  const pickPreset = (preset: DatabasePresetVM) => {
    setPresetId(preset.id);
    apply(preset.database);
  };

  const errs = errorsFor(draft.errors, "database");

  return (
    <div className="flex h-full flex-col">
      <SectionTitle hint="The provider-agnostic database under test (PatchTestWizard.database → domain.Database). Pick an engine, then a preset to seed the settings, then fine-tune. Node roles and counts come from these typed params and drive the topology + machine plan.">
        Database under test
      </SectionTitle>

      {/* Three progressive panes, horizontal on lg+, stacked on narrow viewports.
          Each revealed pane grows to fill the available width; the row fills the
          remaining height and each pane scrolls internally. */}
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
          />
        )}

        {/* Pane 3 — Settings (slides in once a preset/custom is chosen) */}
        {engine && presetId && db && (
          <SettingsPane key={`${engine}-settings`} draft={draft} db={db} apply={apply} errs={errs} />
        )}
      </div>
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
                  : "border-zinc-800 bg-[#0a0a0a] hover:border-zinc-700 hover:bg-zinc-900/50"
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
}: {
  engine: EngineKind;
  selectedId: string | null;
  onPick: (p: DatabasePresetVM) => void;
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
  }, [slug, engine]);

  const meta = ENGINES.find((m) => m.kind === engine);

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
      {presets && (
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {presets.map((p) => {
            const sel = selectedId === p.id;
            const custom = !p.isSystem;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => onPick(p)}
                className={`flex w-full items-start gap-2.5 border p-3 text-left transition-all ${
                  sel
                    ? "border-primary/50 bg-primary/[0.07]"
                    : "border-zinc-800 bg-[#0a0a0a] hover:border-zinc-700 hover:bg-zinc-900/50"
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
}: {
  draft: WizardDraftVM;
  db: DatabaseVM;
  apply: (d: DatabaseVM) => void;
  errs: DraftErrorVM[];
}) {
  return (
    <div className="pane-reveal flex min-h-0 min-w-0 flex-1 flex-col">
      <PaneHeader index={3} title="Settings" subtitle="Typed domain.Database — pre-filled, editable" />
      <div className="grid min-h-0 flex-1 gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(0,22rem)] xl:items-stretch">
        <div className="min-h-0 space-y-6 overflow-y-auto pr-1">
          <EngineVersionSelect db={db} apply={apply} />

          <EngineParamsForm db={db} apply={apply} />
          <FieldErrors errs={errs} />
        </div>

        {/* Derived topology diagram — fills the pane height, scrolls internally. */}
        <TopologyDiagram draft={draft} />
      </div>
    </div>
  );
}

/** Compact numbered header shared by the three Database panes. */
function PaneHeader({ index, title, subtitle }: { index: number; title: string; subtitle: string }) {
  return (
    <div className="mb-3 flex items-center gap-2">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-primary/30 bg-primary/[0.08] font-mono text-[10px] text-primary">
        {index}
      </span>
      <div className="min-w-0">
        <div className="text-sm font-semibold leading-none text-foreground">{title}</div>
        <div className="mt-1 truncate text-[10px] font-mono uppercase tracking-wider text-zinc-600">{subtitle}</div>
      </div>
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
    <div className="flex min-h-0 flex-col border border-zinc-800/60 bg-[#070707] p-3 xl:h-full">
      <div className="mb-2 shrink-0 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Derived topology — {draft.topologyNodes.length} node{draft.topologyNodes.length > 1 ? "s" : ""}
      </div>
      <div className="grid min-h-0 flex-1 auto-rows-min grid-cols-2 gap-2 overflow-y-auto pr-1 sm:grid-cols-3 xl:grid-cols-2">
        {draft.topologyComponents.map((c) => {
          const Icon = ROLE_ICON[c.role] ?? COMP_ICON[c.kind] ?? Box;
          const meta = ENGINES.find((e) => e.kind === c.engine);
          const color = meta?.hex ?? "#71717a";
          return (
            <div key={c.id} className="flex flex-col gap-1 border border-zinc-800 bg-[#0a0a0a] p-3">
              <div className="flex items-center gap-2">
                <Icon className="h-4 w-4" style={{ color }} />
                <span className="font-mono text-[11px] text-zinc-300">{c.role}</span>
              </div>
              <div className="font-mono text-[10px] text-zinc-600">{c.engine}</div>
              <div className="mt-0.5 font-mono text-[10px] text-zinc-500">{specSummary(specByNode.get(c.nodeId))}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function specSummary(spec: MachineSpecVM | undefined): string {
  if (!spec || spec.case === undefined) return "—";
  if (spec.case === "yandex") {
    const y = spec.yandex;
    const t = y.bootDiskType.replace("network-ssd-io-m3", "io-m3").replace("network-ssd", "ssd");
    return `${y.cores} vCPU / ${y.memoryGb} GB / ${y.bootDiskGb} GB ${t}`;
  }
  const d = spec.docker;
  return `${d.cpuCores} cpu / ${(d.memoryMb / 1024).toFixed(d.memoryMb % 1024 ? 1 : 0)} GB`;
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

  const apply = useCallback(
    (next: WorkloadVM) => {
      setW(next);
      void patch({ workload: next });
    },
    [patch],
  );

  const setVersion = useCallback(
    (version: string) => {
      setVersionChosen(true);
      apply({ ...w, stroppyVersion: version });
    },
    [apply, w],
  );

  // Pane 2 — pick a workload preset (or Custom/blank). Seeds + Patches the
  // parameters pane, but PRESERVES the version the user already chose in pane 1
  // (presets carry stroppyVersion "").
  const pickPreset = useCallback(
    (preset: WorkloadPresetVM) => {
      setPresetId(preset.id);
      apply({ ...preset.workload, stroppyVersion: w.stroppyVersion });
    },
    [apply, w.stroppyVersion],
  );

  // --- Probe: fires the MOMENT there's enough to probe (script + version),
  // debounced, and re-fires whenever the script/sql/version/driver change. The
  // probe pane reveals as soon as a RESULT is ready — concurrently with the
  // user still editing pane 2 — exactly like the old WorkloadForm's auto-probe.
  const [probe, setProbe] = useState<ProbeMetaVM | null>(null);
  const [probing, setProbing] = useState(false);
  const [probeErr, setProbeErr] = useState<string | null>(null);

  const canProbe = versionChosen && !!w.script.trim() && !!w.stroppyVersion.trim();
  // Debounce key — only the inputs the probe actually depends on.
  const probeKey = `${w.stroppyVersion}|${w.script}|${w.sql}|${engine}|${w.parameters.poolSize}|${w.parameters.scaleFactor}`;
  useEffect(() => {
    if (!canProbe) {
      setProbe(null);
      setProbeErr(null);
      return;
    }
    let cancelled = false;
    setProbing(true);
    setProbeErr(null);
    const timer = window.setTimeout(() => {
      getWizardProvider()
        .probe(slug, {
          version: w.stroppyVersion,
          script: w.script.trim(),
          sql: w.sql,
          driverType: driverTypeFor(engine),
          poolSize: w.parameters.poolSize,
          scaleFactor: w.parameters.scaleFactor,
          includeHuman: true,
        })
        .then((meta) => {
          if (cancelled) return;
          setProbe(meta);
          setProbeErr(null);
        })
        .catch((e) => {
          if (cancelled) return;
          setProbe(null);
          setProbeErr(e instanceof Error ? e.message : String(e));
        })
        .finally(() => {
          if (!cancelled) setProbing(false);
        });
    }, 350);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
    // probeKey captures every dependency; slug/engine are stable per step.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [probeKey, canProbe, slug]);

  // Pane 3 reveals as soon as there's a result, an error, or a probe in flight.
  const probeStarted = canProbe && (probing || probe !== null || probeErr !== null);

  const errs = errorsFor(draft.errors, "workload");

  return (
    <div className="flex h-full flex-col">
      <SectionTitle hint="The stroppy load (PatchTestWizard.workload → domain.Workload). Pick a stroppy version, tune the workload parameters, then probe the script for its phases &amp; env. The probe pane opens the moment the probe returns.">
        Workload
      </SectionTitle>

      {/* Four progressive panes, horizontal on lg+, stacked on narrow
          viewports — mirrors the Database step. Version + Preset are narrow
          columns (version is small, preset is a short list) so Parameters +
          Probe keep room. The row fills the remaining height and each pane
          scrolls internally. */}
      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:items-stretch">
        {/* Pane 1 — Version (compact column on the left) */}
        <WorkloadVersionPane
          slug={slug}
          value={versionChosen ? w.stroppyVersion : ""}
          onPick={setVersion}
        />

        {/* Pane 2 — Preset (slides in once a version is chosen) */}
        {versionChosen && (
          <WorkloadPresetPane slug={slug} selectedId={presetId} onPick={pickPreset} />
        )}

        {/* Pane 3 — Parameters (slides in once a preset/Custom is chosen) */}
        {versionChosen && presetId && (
          <WorkloadParametersPane w={w} apply={apply} errs={errs} />
        )}

        {/* Pane 4 — Probe (reveals the moment the probe result is ready) */}
        {probeStarted && (
          <WorkloadProbePane
            probe={probe}
            probing={probing}
            probeErr={probeErr}
            workload={w}
            apply={apply}
          />
        )}
      </div>
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
                        : "border-zinc-800 bg-[#0a0a0a] hover:border-zinc-700 hover:bg-zinc-900/50"
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
            Builds stroppy from a specific CI commit — sent as{" "}
            <span className="font-mono text-zinc-500">commit:&lt;sha&gt;</span>.
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
  selectedId,
  onPick,
}: {
  slug: string;
  selectedId: string | null;
  onPick: (p: WorkloadPresetVM) => void;
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
  }, [slug]);

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
      {presets && (
        <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {presets.map((p) => {
            const sel = selectedId === p.id;
            const custom = !p.isSystem;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => onPick(p)}
                className={`flex w-full items-start gap-2.5 border p-3 text-left transition-all ${
                  sel
                    ? "border-primary/50 bg-primary/[0.07]"
                    : "border-zinc-800 bg-[#0a0a0a] hover:border-zinc-700 hover:bg-zinc-900/50"
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
}: {
  w: WorkloadVM;
  apply: (w: WorkloadVM) => void;
  errs: DraftErrorVM[];
}) {
  return (
    <div className="pane-reveal flex min-h-0 min-w-0 flex-1 flex-col lg:max-w-2xl">
      <PaneHeader index={3} title="Parameters" subtitle="Typed domain.Workload — editable" />
      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        <WorkloadParamsForm w={w} apply={apply} />
        <FieldErrors errs={errs} />
      </div>
    </div>
  );
}

// ─── Workload pane 4: ProbeScript results (reveals when probe is ready) ────────

function WorkloadProbePane({
  probe,
  probing,
  probeErr,
  workload,
  apply,
}: {
  probe: ProbeMetaVM | null;
  probing: boolean;
  probeErr: string | null;
  workload: WorkloadVM;
  apply: (w: WorkloadVM) => void;
}) {
  const toggleStep = (step: string) => {
    const has = workload.parameters.steps.includes(step);
    const steps = has ? workload.parameters.steps.filter((s) => s !== step) : [...workload.parameters.steps, step];
    apply({ ...workload, parameters: { ...workload.parameters, steps, noSteps: [] } });
  };
  const setEnv = (name: string, val: string) => {
    apply({ ...workload, parameters: { ...workload.parameters, env: { ...workload.parameters.env, [name]: val } } });
  };

  return (
    <div className="pane-reveal flex min-h-0 min-w-0 flex-1 flex-col lg:max-w-md">
      <PaneHeader index={4} title="Probe" subtitle="stroppy probe — phases &amp; env" />
      <div className="flex min-h-0 flex-1 flex-col border border-zinc-800 bg-[#070707] p-4">
        <div className="flex shrink-0 items-center gap-2 text-[11px]">
          {probing ? (
            <span className="flex items-center gap-2 text-zinc-500">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Probing workload…
            </span>
          ) : probeErr ? (
            <span className="flex items-center gap-2 text-red-400">
              <AlertCircle className="h-3.5 w-3.5" /> {probeErr}
            </span>
          ) : probe ? (
            <span className="flex items-center gap-2 text-emerald-400">
              <Check className="h-3.5 w-3.5" /> Probe OK — {probe.steps.length} phase{probe.steps.length === 1 ? "" : "s"}, {probe.env.length} env
            </span>
          ) : null}
        </div>

        {probe && (
          <div className="mt-3 min-h-0 flex-1 space-y-4 overflow-y-auto pr-1">
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-[11px] text-zinc-500">
              <span>driver: <span className="font-mono text-zinc-300">{probe.driverType}</span></span>
              <span>pool: <span className="font-mono text-zinc-300">{probe.poolSize}</span></span>
              {probe.sqlSections.length > 0 && (
                <span>sql: <span className="font-mono text-zinc-300">{probe.sqlSections.join(", ")}</span></span>
              )}
            </div>

            {probe.steps.length > 0 && (
              <div>
                <Label>Phases (steps allowlist — empty runs all)</Label>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {probe.steps.map((s) => {
                    const on = workload.parameters.steps.includes(s);
                    return (
                      <button
                        key={s}
                        type="button"
                        onClick={() => toggleStep(s)}
                        className={`px-2.5 py-1 font-mono text-[11px] transition-all ${
                          on ? "border border-primary/40 bg-primary/[0.08] text-primary" : "border border-zinc-800 text-zinc-500 hover:text-zinc-300"
                        }`}
                      >
                        {s}
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {probe.env.length > 0 && (
              <div>
                <Label>Environment</Label>
                <div className="mt-1.5 space-y-2">
                  {probe.env.map((d) => (
                    <div key={d.name} className="flex items-center gap-2">
                      <div className="w-40 shrink-0">
                        <div className="font-mono text-[11px] text-zinc-300">
                          {d.name}
                          {d.required && <span className="ml-1 text-red-400">*</span>}
                        </div>
                        <div className="truncate text-[10px] text-zinc-600">{d.description}</div>
                      </div>
                      <Input
                        className="h-8 font-mono text-xs"
                        placeholder={d.default}
                        value={workload.parameters.env[d.name] ?? ""}
                        onChange={(e) => setEnv(d.name, e.target.value)}
                      />
                    </div>
                  ))}
                </div>
              </div>
            )}

            {probe.human && (
              <div>
                <Label>probe -o human</Label>
                <pre className="mt-1.5 max-h-56 overflow-auto border border-zinc-800/60 bg-black/40 p-2 font-mono text-[10px] leading-relaxed text-zinc-400">
                  {probe.human}
                </pre>
              </div>
            )}
          </div>
        )}

        {!probe && !probing && probeErr && (
          <p className="mt-3 text-[11px] text-zinc-600">
            Adjust the script or version to re-run <span className="font-mono">stroppy probe</span>.
          </p>
        )}
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

  // Group artifacts by render component (RenderPreview.components order).
  const byComponent = useMemo(() => {
    const map = new Map<string, RenderArtifactVM[]>();
    for (const a of draft.artifacts) {
      const list = map.get(a.componentId) ?? [];
      list.push(a);
      map.set(a.componentId, list);
    }
    return map;
  }, [draft.artifacts]);

  const order = draft.renderComponents.length
    ? draft.renderComponents.map((c) => c.componentId)
    : [...byComponent.keys()];
  const overrideCount = draft.artifacts.filter((a) => a.origin === RenderArtifact_Origin.USER_OVERRIDE).length;

  return (
    <div className="flex h-full flex-col">
      <SectionTitle hint="The server-rendered deployment.RenderPreview — per-component artifacts. EDITABLE configs are editable inline (→ render_overrides / FileOverride); READ_ONLY ones are locked. Then bake into a TestRun (FinishTestWizard).">
        Review &amp; render
      </SectionTitle>

      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
      {/* Summary cards */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
        <ReviewCard title="Provider" value={Provider[draft.provider].replace("UNSPECIFIED", "—").toLowerCase()} />
        <ReviewCard title="Database" value={draft.database ? `${ENGINES.find((e) => e.kind === draft.database!.kind)?.label} ${draft.database.version}` : "—"} />
        <ReviewCard title="Workload" value={draft.workload?.script ?? "—"} />
        <ReviewCard
          title="Execution"
          value={
            draft.workload
              ? draft.workload.execution.limit.case === "duration"
                ? `${draft.workload.execution.vus} VU · ${draft.workload.execution.limit.duration}`
                : `${draft.workload.execution.vus} VU · ${draft.workload.execution.limit.iterations} iters`
              : "—"
          }
        />
        <ReviewCard title="Nodes" value={String(draft.topologyNodes.length)} />
        <ReviewCard title="Overrides" value={overrideCount ? `${overrideCount} edited` : "none"} />
      </div>

      {/* Render artifacts, grouped by component */}
      {draft.artifacts.length > 0 && (
        <div className="mt-6">
          <div className="mb-2 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
            Render preview — generated configs &amp; commands
          </div>
          <div className="grid gap-4 xl:grid-cols-2 xl:items-start 2xl:grid-cols-3">
            {order.map((compId) => {
              const list = byComponent.get(compId);
              if (!list || list.length === 0) return null;
              return (
                <div key={compId}>
                  <div className="mb-1.5 flex items-center gap-2">
                    {(() => {
                      const comp = draft.topologyComponents.find((c) => c.id === compId);
                      const Icon = comp ? COMP_ICON[comp.kind] ?? Box : Box;
                      return <Icon className="h-3.5 w-3.5 text-zinc-500" />;
                    })()}
                    <span className="font-mono text-[12px] text-zinc-400">{compId}</span>
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
        <ToggleRow label="Launch now" hint="persist + start the run immediately (start=true)" checked={start} onChange={setStart} />
        {start && (
          <div className="ml-1 grid grid-cols-2 gap-2 border-l border-zinc-800 pl-4">
            <ToggleRow label="Tenant rating" hint="in_tenant_rating" checked={inTenantRating} onChange={setInTenantRating} />
            <ToggleRow label="Global rating" hint="in_global_rating" checked={inGlobalRating} onChange={setInGlobalRating} />
          </div>
        )}
        <ToggleRow label="Save as preset" hint="save db+workload as a reusable test preset" checked={saveAsPreset} onChange={setSaveAsPreset} />
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
    <div className="border border-zinc-800 bg-[#0a0a0a]">
      <button
        type="button"
        onClick={() => (editable || hasContent) && setOpen((o) => !o)}
        className="flex w-full items-center gap-3 px-4 py-2.5 text-left"
      >
        <Icon className="h-4 w-4 text-zinc-400" />
        <span className="min-w-0 flex-1 truncate font-mono text-[12px] text-zinc-300">{a.title}</span>
        {overridden && (
          <span className="inline-flex items-center gap-1 bg-primary/10 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide text-primary">
            <Pencil className="h-2.5 w-2.5" /> override
          </span>
        )}
        <span
          className={`inline-flex items-center gap-1 px-1.5 py-0.5 font-mono text-[9px] uppercase tracking-wide ${
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

function ReviewCard({ title, value }: { title: string; value: string }) {
  return (
    <div className="border border-zinc-800 bg-[#0a0a0a] p-3">
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">{title}</div>
      <div className="mt-1 truncate text-sm text-foreground">{value}</div>
    </div>
  );
}

