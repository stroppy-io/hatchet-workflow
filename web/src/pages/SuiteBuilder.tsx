import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "@/lib/router";
import {
  createSuite,
  createSuiteItem,
  deleteSuiteItem,
  getSettings,
  getStroppyVersions,
  getSuite,
  listPresets,
  listSuiteItems,
  previewStroppyConfig,
  updateSuite,
  updateSuiteItem,
} from "@/api/client";
import {
  ALL_DB_KINDS,
  DEFAULT_SUITE_POLICY,
  KIND_PROTOCOLS,
} from "@/api/types";
import type {
  ComparisonConfig,
  DatabaseKind,
  MachineSpec,
  NetworkConfig,
  Preset,
  Protocol,
  ProbeResponse,
  Provider,
  StroppyConfig,
  Suite,
  SuiteItem,
  SuitePolicy,
  TenantQuotas,
  WorkloadFile,
} from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  InfrastructureForm,
  PROVIDER_META,
} from "@/components/InfrastructureForm";
import { WorkloadForm, type K6Mode } from "@/components/WorkloadForm";
import { DB_COLORS } from "@/lib/db-colors";
import {
  AlertCircle,
  AlertTriangle,
  ArrowLeft,
  Boxes,
  CalendarClock,
  CheckSquare,
  ChevronLeft,
  ChevronRight,
  Clock,
  Cpu,
  Database,
  GitCompareArrows,
  Loader2,
  Pencil,
  Plus,
  Save,
  Settings2,
  Sparkles,
  Square,
  Trash2,
  Wrench,
  X,
  Zap,
} from "lucide-react";

// SuiteBuilder — three-step wizard:
//   1. Setup     — identity + infrastructure + schedule + policy
//   2. Matrix    — db targets (left) + workloads (right, multi-edit)
//   3. Review    — summary + save
//
// Workloads (suite items) are buffered in local state during edit. On save
// we diff: drafts with a serverID → updateSuiteItem, drafts without →
// createSuiteItem, originally-loaded IDs missing from drafts → deleteSuiteItem.

const STEPS = [
  { key: "setup",  label: "Setup",   icon: Settings2 },
  { key: "matrix", label: "Matrix",  icon: Boxes },
  { key: "review", label: "Review",  icon: CheckSquare },
] as const;

type WorkloadDraft = {
  clientID: string;
  serverID?: string;
  name: string;
  workload: StroppyConfig;
};

function newDraftWorkload(): WorkloadDraft {
  return {
    clientID: cryptoRandomID(),
    name: "New workload",
    workload: {
      version: "",
      script: "tpcc/procs",
      duration: "5m",
      vus: 100,
      pool_size: 150,
      scale_factor: 500,
      k6_mode: "duration",
      quiet: true,
    },
  };
}

function cryptoRandomID(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2);
}

export function SuiteBuilder() {
  const navigate = useNavigate();
  const { id } = useParams<{ id?: string }>();
  const editing = Boolean(id);

  const [step, setStep] = useState(0);
  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [presets, setPresets] = useState<Preset[]>([]);
  const [loaded, setLoaded] = useState<Suite | null>(null);
  const [originalItemIDs, setOriginalItemIDs] = useState<string[]>([]);
  const [quotas, setQuotas] = useState<TenantQuotas>({});

  // --- Setup state (step 0) ---
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const [provider, setProvider] = useState<Provider>("docker");
  const [platformID, setPlatformID] = useState("");
  const [advancedInfraOpen, setAdvancedInfraOpen] = useState(false);
  const [network, setNetwork] = useState<NetworkConfig>({ cidr: "10.10.0.0/24" });
  const [stroppyMachine, setStroppyMachine] = useState<Partial<MachineSpec>>({});

  const [scheduleEnabled, setScheduleEnabled] = useState(false);
  const [cronExpr, setCronExpr] = useState("");
  const [timezone, setTimezone] = useState("UTC");
  const [enabled, setEnabled] = useState(true);
  const [concurrentPolicy, setConcurrentPolicy] = useState<"forbid" | "allow">("forbid");
  const [catchupMode, setCatchupMode] = useState<"skip" | "once">("skip");

  const [policy, setPolicy] = useState<SuitePolicy>(DEFAULT_SUITE_POLICY);
  const [comparison, setComparison] = useState<ComparisonConfig>({
    strategy: "previous",
    threshold_pct: 5,
  });
  const [retentionRuns, setRetentionRuns] = useState(30);

  // --- Matrix state (step 1) ---
  const [dbPresetIDs, setDbPresetIDs] = useState<string[]>([]);
  const [drafts, setDrafts] = useState<WorkloadDraft[]>([]);
  const [editingDraftID, setEditingDraftID] = useState<string | null>(null);

  // Stroppy releases — loaded once at suite level. Without this every
  // mounted DraftEditor would fire its own GitHub-proxied request and the
  // upstream rate-limits with 502.
  const [stroppyVersions, setStroppyVersions] = useState<string[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const versionsLoaded = useRef(false);

  useEffect(() => {
    if (versionsLoaded.current) return;
    versionsLoaded.current = true;
    setVersionsLoading(true);
    getStroppyVersions()
      .then((vers) => { if (vers.length > 0) setStroppyVersions(vers); })
      .catch(() => {})
      .finally(() => setVersionsLoading(false));
  }, []);

  // Bootstrap.
  useEffect(() => {
    listPresets().then(setPresets).catch(() => {});
    getSettings().then((s) => setQuotas(s.quotas || {})).catch(() => {});
  }, []);

  useEffect(() => {
    if (!editing || !id) return;
    Promise.all([getSuite(id), listSuiteItems(id)])
      .then(([s, items]) => {
        setLoaded(s);
        setName(s.name);
        setDescription(s.description || "");
        setPolicy(s.policy ?? DEFAULT_SUITE_POLICY);
        setDbPresetIDs(s.db_preset_ids ?? []);
        setProvider(s.provider || "docker");
        setPlatformID(s.platform_id || "");
        setNetwork(s.network || { cidr: "10.10.0.0/24" });
        setStroppyMachine(s.stroppy_machine || {});
        const hasCron = Boolean(s.cron_expr);
        setScheduleEnabled(hasCron);
        setCronExpr(s.cron_expr || "");
        setTimezone(s.timezone || "UTC");
        setEnabled(s.enabled);
        setConcurrentPolicy(s.concurrent_policy || "forbid");
        setCatchupMode(s.catchup_mode || "skip");
        setRetentionRuns(s.retention_runs || 30);
        setComparison(s.comparison || { strategy: "previous", threshold_pct: 5 });
        setOriginalItemIDs(items.map((it) => it.id));
        setDrafts(
          items.sort((a, b) => a.position - b.position).map<WorkloadDraft>((it) => ({
            clientID: cryptoRandomID(),
            serverID: it.id,
            name: it.name,
            workload: it.workload || ({} as StroppyConfig),
          })),
        );
      })
      .catch((err) => setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" }))
      .finally(() => setLoading(false));
  }, [editing, id]);

  // --- derived ---

  const presetsByKind = useMemo(() => {
    const m: Record<string, Preset[]> = {};
    for (const p of presets) (m[p.db_kind] ||= []).push(p);
    return m;
  }, [presets]);

  const selectedPresets = useMemo(
    () => dbPresetIDs.map((pid) => presets.find((p) => p.id === pid)).filter(Boolean) as Preset[],
    [dbPresetIDs, presets],
  );

  const stepValid: Record<number, boolean> = {
    0: name.trim().length > 0 && (!scheduleEnabled || cronShapeOK(cronExpr)),
    1: dbPresetIDs.length > 0 && drafts.length > 0,
    2: name.trim().length > 0 && dbPresetIDs.length > 0 && drafts.length > 0,
  };

  function togglePreset(pid: string) {
    setDbPresetIDs((prev) =>
      prev.includes(pid) ? prev.filter((x) => x !== pid) : [...prev, pid],
    );
  }

  function addDraft() {
    const d = newDraftWorkload();
    setDrafts((prev) => [...prev, d]);
    setEditingDraftID(d.clientID);
  }

  function commitDraft(clientID: string, patch: Partial<WorkloadDraft>) {
    setDrafts((prev) => prev.map((d) => (d.clientID === clientID ? { ...d, ...patch } : d)));
  }

  function removeDraft(clientID: string) {
    setDrafts((prev) => prev.filter((d) => d.clientID !== clientID));
    if (editingDraftID === clientID) setEditingDraftID(null);
  }

  async function save() {
    setSaving(true);
    setMessage(null);
    try {
      const payload = {
        name,
        description,
        policy,
        comparison,
        db_preset_ids: dbPresetIDs,
        provider,
        platform_id: platformID,
        network,
        stroppy_machine: stroppyMachine,
        cron_expr: scheduleEnabled ? cronExpr : "",
        timezone,
        enabled: scheduleEnabled ? enabled : false,
        concurrent_policy: concurrentPolicy,
        catchup_mode: catchupMode,
        retention_runs: retentionRuns,
      };

      let suiteID: string;
      if (editing && id) {
        await updateSuite(id, payload);
        suiteID = id;
      } else {
        const r = await createSuite({ ...payload, name });
        suiteID = r.id;
      }

      // Sync items: deletions, updates, creations.
      const draftServerIDs = new Set(drafts.filter((d) => d.serverID).map((d) => d.serverID!));
      const toDelete = originalItemIDs.filter((oid) => !draftServerIDs.has(oid));
      for (const oid of toDelete) {
        await deleteSuiteItem(suiteID, oid);
      }
      for (let i = 0; i < drafts.length; i++) {
        const d = drafts[i];
        if (d.serverID) {
          await updateSuiteItem(suiteID, d.serverID, {
            name: d.name,
            workload: d.workload,
            position: i,
          });
        } else {
          await createSuiteItem(suiteID, {
            name: d.name,
            workload: d.workload,
            position: i,
          });
        }
      }

      navigate(`/suites/${suiteID}`);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Save failed" });
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500">
        <Loader2 className="w-4 h-4 animate-spin mr-2" /> loading suite…
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full overflow-hidden bg-[#050505]">
      {/* Top bar */}
      <div className="shrink-0 px-5 py-3 border-b border-zinc-800 bg-[#070707] flex items-center gap-3">
        <Button variant="ghost" size="sm" onClick={() => navigate("/suites")} className="text-zinc-500 hover:text-zinc-200">
          <ArrowLeft className="w-3.5 h-3.5 mr-1.5" /> Suites
        </Button>
        <div className="flex items-center gap-2">
          <Boxes className="w-4 h-4 text-primary" />
          <h1 className="text-sm font-mono font-semibold text-zinc-200">
            {editing ? `Edit suite — ${loaded?.name ?? ""}` : "New suite"}
          </h1>
        </div>
      </div>

      {/* Step rail */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <div className="flex items-center gap-1">
          {STEPS.map((s, i) => {
            const SIcon = s.icon;
            const reachable = i <= step || stepValid[i - 1] !== false;
            return (
              <div key={s.key} className="flex items-center gap-1">
                {i > 0 && <ChevronRight className="w-3 h-3 text-zinc-700" />}
                <button
                  type="button"
                  onClick={() => reachable && setStep(i)}
                  disabled={!reachable}
                  className={`flex items-center gap-1.5 px-3 py-1 text-xs font-mono transition-all cursor-pointer disabled:cursor-not-allowed ${
                    i === step
                      ? "text-primary border border-primary/30 bg-primary/[0.06]"
                      : i < step
                        ? "text-zinc-400 hover:text-zinc-300"
                        : "text-zinc-600"
                  }`}
                >
                  <SIcon className="w-3 h-3" />
                  <span className="text-zinc-600 mr-0.5">{i + 1}.</span>
                  {s.label}
                </button>
              </div>
            );
          })}
        </div>
      </div>

      {message && (
        <div
          className={`shrink-0 px-5 py-2 border-b flex items-center gap-2 text-xs font-mono ${
            message.type === "success"
              ? "border-emerald-500/30 bg-emerald-500/[0.06] text-emerald-400"
              : "border-red-500/30 bg-red-500/[0.06] text-red-400"
          }`}
        >
          <AlertCircle className="w-3.5 h-3.5" />
          {message.text}
        </div>
      )}

      <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
        <div className="flex-1 min-h-0 overflow-y-auto">
          {step === 0 && (
            <SetupStep
              name={name} setName={setName}
              description={description} setDescription={setDescription}
              provider={provider} setProvider={setProvider}
              platformID={platformID} setPlatformID={setPlatformID}
              network={network} setNetwork={setNetwork}
              stroppyMachine={stroppyMachine} setStroppyMachine={setStroppyMachine}
              advancedInfraOpen={advancedInfraOpen} setAdvancedInfraOpen={setAdvancedInfraOpen}
              scheduleEnabled={scheduleEnabled} setScheduleEnabled={setScheduleEnabled}
              cronExpr={cronExpr} setCronExpr={setCronExpr}
              timezone={timezone} setTimezone={setTimezone}
              enabled={enabled} setEnabled={setEnabled}
              concurrentPolicy={concurrentPolicy} setConcurrentPolicy={setConcurrentPolicy}
              catchupMode={catchupMode} setCatchupMode={setCatchupMode}
              policy={policy} setPolicy={setPolicy}
              comparison={comparison} setComparison={setComparison}
              retentionRuns={retentionRuns} setRetentionRuns={setRetentionRuns}
              loaded={loaded}
            />
          )}
          {step === 1 && (
            <MatrixStep
              presetsByKind={presetsByKind}
              selectedPresetIDs={dbPresetIDs}
              onTogglePreset={togglePreset}
              selectedPresets={selectedPresets}
              drafts={drafts}
              editingDraftID={editingDraftID}
              setEditingDraftID={setEditingDraftID}
              addDraft={addDraft}
              commitDraft={commitDraft}
              removeDraft={removeDraft}
              provider={provider}
              platformID={platformID}
              network={network}
              quotas={quotas}
              stroppyVersions={stroppyVersions}
              versionsLoading={versionsLoading}
              versionsLoaded={versionsLoaded}
              setStroppyVersions={setStroppyVersions}
              setVersionsLoading={setVersionsLoading}
            />
          )}
          {step === 2 && (
            <ReviewStep
              editing={editing}
              name={name}
              description={description}
              presets={selectedPresets}
              drafts={drafts}
              provider={provider}
              platformID={platformID}
              network={network}
              stroppyMachine={stroppyMachine}
              scheduleEnabled={scheduleEnabled}
              cronExpr={cronExpr}
              timezone={timezone}
              policy={policy}
              comparison={comparison}
              retentionRuns={retentionRuns}
            />
          )}
        </div>

        {/* Sticky nav */}
        <div className="shrink-0 px-5 py-3 border-t border-zinc-800 bg-[#070707] flex items-center justify-between">
          <Button
            variant="outline" size="sm"
            onClick={() => setStep(Math.max(0, step - 1))}
            disabled={step === 0}
            className="gap-1.5"
          >
            <ChevronLeft className="h-3 w-3" /> Back
          </Button>
          <div className="text-[10px] font-mono text-zinc-600">
            {step + 1} / {STEPS.length}
          </div>
          {step < STEPS.length - 1 ? (
            <Button
              size="sm"
              onClick={() => setStep(step + 1)}
              disabled={!stepValid[step]}
              className="gap-1.5"
            >
              Next <ChevronRight className="h-3 w-3" />
            </Button>
          ) : (
            <Button
              size="sm"
              onClick={save}
              disabled={saving || !stepValid[2]}
              className="gap-1.5"
            >
              {saving ? <Loader2 className="h-3 w-3 animate-spin" /> : <Save className="h-3 w-3" />}
              {editing ? "Save changes" : "Create suite"}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Step 0: Setup ──────────────────────────────────────────────

function SetupStep(props: {
  name: string; setName: (v: string) => void;
  description: string; setDescription: (v: string) => void;
  provider: Provider; setProvider: (p: Provider) => void;
  platformID: string; setPlatformID: (p: string) => void;
  network: NetworkConfig; setNetwork: (n: NetworkConfig) => void;
  stroppyMachine: Partial<MachineSpec>; setStroppyMachine: (m: Partial<MachineSpec>) => void;
  advancedInfraOpen: boolean; setAdvancedInfraOpen: (v: boolean) => void;
  scheduleEnabled: boolean; setScheduleEnabled: (v: boolean) => void;
  cronExpr: string; setCronExpr: (v: string) => void;
  timezone: string; setTimezone: (v: string) => void;
  enabled: boolean; setEnabled: (v: boolean) => void;
  concurrentPolicy: "forbid" | "allow"; setConcurrentPolicy: (v: "forbid" | "allow") => void;
  catchupMode: "skip" | "once"; setCatchupMode: (v: "skip" | "once") => void;
  policy: SuitePolicy; setPolicy: (p: SuitePolicy) => void;
  comparison: ComparisonConfig; setComparison: (c: ComparisonConfig) => void;
  retentionRuns: number; setRetentionRuns: (v: number) => void;
  loaded: Suite | null;
}) {
  return (
    <div className="p-5 grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
      {/* LEFT column — Identity + Infrastructure */}
      <div className="space-y-5">
      <SectionCard icon={Sparkles} title="Identity">
        <div className="space-y-3">
          <FieldStack
            label="Name"
            required
            children={
              <input
                type="text"
                value={props.name}
                onChange={(e) => props.setName(e.target.value)}
                placeholder="e.g. nightly-tpcc-cross-engine"
                className="w-full bg-transparent border-0 border-b border-zinc-800 px-0 py-1.5 text-sm font-mono text-zinc-200 placeholder:text-zinc-700 focus:outline-none focus:border-primary/60 transition-colors"
                maxLength={128}
                autoFocus
              />
            }
          />
          <FieldStack
            label="Description"
            children={
              <textarea
                value={props.description}
                onChange={(e) => props.setDescription(e.target.value)}
                placeholder="What does this suite test? Why does it exist?"
                className="w-full bg-transparent border-0 border-b border-zinc-800 px-0 py-1.5 text-xs font-mono text-zinc-300 placeholder:text-zinc-700 focus:outline-none focus:border-primary/60 transition-colors resize-none"
                rows={2}
                maxLength={1024}
              />
            }
          />
        </div>
      </SectionCard>

      {/* Infrastructure */}
      <SectionCard icon={Cpu} title="Infrastructure">
        <InfrastructureForm
          provider={props.provider} setProvider={props.setProvider}
          platformId={props.platformID} setPlatformId={props.setPlatformID}
        />
        <div className="border-t border-zinc-800/50 mt-4 pt-3">
          <button
            type="button"
            onClick={() => props.setAdvancedInfraOpen(!props.advancedInfraOpen)}
            className="flex items-center gap-2 text-[11px] font-mono uppercase tracking-wider text-zinc-500 hover:text-zinc-300 cursor-pointer"
          >
            <ChevronRight className={`w-3 h-3 transition-transform ${props.advancedInfraOpen ? "rotate-90" : ""}`} />
            Advanced — network + stroppy runner
          </button>
          {props.advancedInfraOpen && (
            <div className="mt-3 space-y-4 max-w-lg">
              <div className="grid grid-cols-2 gap-3">
                <FieldRow label="CIDR">
                  <Input
                    value={props.network.cidr}
                    onChange={(e) => props.setNetwork({ ...props.network, cidr: e.target.value })}
                    className="font-mono"
                  />
                </FieldRow>
                <FieldRow label="Zone (optional)">
                  <Input
                    value={props.network.zone || ""}
                    onChange={(e) => props.setNetwork({ ...props.network, zone: e.target.value })}
                  />
                </FieldRow>
              </div>
              <div>
                <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Stroppy runner</Label>
                <p className="text-[10px] font-mono text-zinc-600 mt-0.5">Empty = server defaults (32 vCPU / 65 GB / 100 GB).</p>
                <div className="grid grid-cols-3 gap-2 mt-2">
                  <Input
                    type="number" placeholder="vCPU"
                    value={props.stroppyMachine.cpus ?? ""}
                    onChange={(e) => props.setStroppyMachine({ ...props.stroppyMachine, cpus: e.target.value ? Number(e.target.value) : undefined })}
                  />
                  <Input
                    type="number" placeholder="memory MB"
                    value={props.stroppyMachine.memory_mb ?? ""}
                    onChange={(e) => props.setStroppyMachine({ ...props.stroppyMachine, memory_mb: e.target.value ? Number(e.target.value) : undefined })}
                  />
                  <Input
                    type="number" placeholder="disk GB"
                    value={props.stroppyMachine.disk_gb ?? ""}
                    onChange={(e) => props.setStroppyMachine({ ...props.stroppyMachine, disk_gb: e.target.value ? Number(e.target.value) : undefined })}
                  />
                </div>
              </div>
            </div>
          )}
        </div>
      </SectionCard>

      </div>

      {/* RIGHT column — Schedule + Policy */}
      <div className="space-y-5">
      <SectionCard icon={CalendarClock} title="Schedule" subtitle="Optional. Off = manual-only suite.">
        <div
          role="button"
          tabIndex={0}
          onClick={() => props.setScheduleEnabled(!props.scheduleEnabled)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              props.setScheduleEnabled(!props.scheduleEnabled);
            }
          }}
          className={`w-full flex items-center gap-3 border p-3 text-left transition-all cursor-pointer ${
            props.scheduleEnabled
              ? "border-primary/40 bg-primary/[0.06]"
              : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
          }`}
        >
          <div className={`w-8 h-8 border flex items-center justify-center shrink-0 ${
            props.scheduleEnabled ? "border-primary/40 bg-primary/[0.08]" : "border-zinc-800"
          }`}>
            <CalendarClock className={`w-4 h-4 ${props.scheduleEnabled ? "text-primary" : "text-zinc-600"}`} />
          </div>
          <div className="flex-1 min-w-0">
            <div className={`text-xs font-mono font-medium ${props.scheduleEnabled ? "text-primary" : "text-zinc-300"}`}>
              Schedule with cron
            </div>
            <div className="text-[10px] font-mono text-zinc-600">
              {props.scheduleEnabled ? "Auto-fire on cron expression" : "Manual launches only"}
            </div>
          </div>
          <Switch
            checked={props.scheduleEnabled}
            onCheckedChange={props.setScheduleEnabled}
            onClick={(e) => e.stopPropagation()}
          />
        </div>

        {props.scheduleEnabled && (
          <div className="mt-3 space-y-3">
            <div>
              <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Cron expression</Label>
              <input
                value={props.cronExpr}
                onChange={(e) => props.setCronExpr(e.target.value)}
                placeholder="e.g. 0 2 * * *  —  or @daily / @hourly"
                className="w-full mt-1.5 h-9 bg-zinc-900 border border-zinc-800 px-2 font-mono text-xs text-zinc-200 outline-none focus:border-primary/60"
                spellCheck={false}
              />
              <CronPreview expr={props.cronExpr} tz={props.timezone} loaded={props.loaded} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <FieldRow label="Timezone">
                <Input value={props.timezone} onChange={(e) => props.setTimezone(e.target.value)} className="font-mono" />
              </FieldRow>
              <FieldRow label="Cron paused?">
                <div className="flex items-center gap-2 h-9 px-2 border border-zinc-800 bg-zinc-900">
                  <Switch checked={!props.enabled} onCheckedChange={(v: boolean) => props.setEnabled(!v)} />
                  <span className="text-[10px] font-mono text-zinc-500">
                    {props.enabled ? "active" : "paused"}
                  </span>
                </div>
              </FieldRow>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <FieldRow label="Concurrent policy" hint="if previous batch still running">
                <Select value={props.concurrentPolicy} onValueChange={(v) => props.setConcurrentPolicy(v as "forbid" | "allow")}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="forbid">forbid</SelectItem>
                    <SelectItem value="allow">allow</SelectItem>
                  </SelectContent>
                </Select>
              </FieldRow>
              <FieldRow label="Missed window catchup">
                <Select value={props.catchupMode} onValueChange={(v) => props.setCatchupMode(v as "skip" | "once")}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="skip">skip</SelectItem>
                    <SelectItem value="once">once</SelectItem>
                  </SelectContent>
                </Select>
              </FieldRow>
            </div>
          </div>
        )}
      </SectionCard>

      {/* Policy */}
      <SectionCard icon={Wrench} title="Execution policy &amp; comparison">
        <div className="grid grid-cols-2 gap-3">
          <FieldRow label="Mode">
            <Select
              value={props.policy.mode}
              onValueChange={(v) => props.setPolicy({ ...props.policy, mode: v as "sequential" | "parallel" })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="sequential">sequential</SelectItem>
                <SelectItem value="parallel">parallel</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          <FieldRow label="Max parallel" hint="parallel only · 0 = unlimited">
            <Input
              type="number" min={0}
              value={props.policy.max_parallel}
              onChange={(e) => props.setPolicy({ ...props.policy, max_parallel: Number(e.target.value) || 0 })}
            />
          </FieldRow>
          <FieldRow label="On step failure">
            <Select
              value={props.policy.on_step_fail}
              onValueChange={(v) => props.setPolicy({ ...props.policy, on_step_fail: v as "continue" | "stop" })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="continue">continue</SelectItem>
                <SelectItem value="stop">stop</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          <FieldRow label="Step timeout (min)" hint="0 = no timeout">
            <Input
              type="number" min={0}
              value={props.policy.step_timeout_min}
              onChange={(e) => props.setPolicy({ ...props.policy, step_timeout_min: Number(e.target.value) || 0 })}
            />
          </FieldRow>
        </div>
        <div className="border-t border-zinc-800/50 mt-3 pt-3 grid grid-cols-2 gap-3">
          <FieldRow label="Baseline strategy">
            <Select
              value={props.comparison.strategy || "none"}
              onValueChange={(v) => props.setComparison({ ...props.comparison, strategy: (v === "none" ? "" : v) as ComparisonConfig["strategy"] })}
            >
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="none">none</SelectItem>
                <SelectItem value="previous">previous batch</SelectItem>
                <SelectItem value="first">first batch</SelectItem>
                <SelectItem value="fixed">fixed batch id</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          <FieldRow label="Diff threshold %">
            <Input
              type="number" min={0} step={0.5}
              value={props.comparison.threshold_pct ?? 5}
              onChange={(e) => props.setComparison({ ...props.comparison, threshold_pct: Number(e.target.value) || 0 })}
            />
          </FieldRow>
          {props.comparison.strategy === "fixed" && (
            <FieldRow label="Baseline batch ID">
              <Input
                value={props.comparison.baseline_batch_id || ""}
                onChange={(e) => props.setComparison({ ...props.comparison, baseline_batch_id: e.target.value })}
                placeholder="batch UUID"
                className="font-mono"
              />
            </FieldRow>
          )}
          <FieldRow label="Retention" hint="keep last N batches">
            <Input
              type="number" min={1}
              value={props.retentionRuns}
              onChange={(e) => props.setRetentionRuns(Number(e.target.value) || 30)}
            />
          </FieldRow>
        </div>
      </SectionCard>
      </div>
    </div>
  );
}

// ─── Step 1: Matrix (DB targets + Workloads, split-view) ───────

function MatrixStep({
  presetsByKind,
  selectedPresetIDs,
  onTogglePreset,
  selectedPresets,
  drafts,
  editingDraftID,
  setEditingDraftID,
  addDraft,
  commitDraft,
  removeDraft,
  provider,
  platformID,
  network,
  quotas,
  stroppyVersions,
  versionsLoading,
  versionsLoaded,
  setStroppyVersions,
  setVersionsLoading,
}: {
  presetsByKind: Record<string, Preset[]>;
  selectedPresetIDs: string[];
  onTogglePreset: (pid: string) => void;
  selectedPresets: Preset[];
  drafts: WorkloadDraft[];
  editingDraftID: string | null;
  setEditingDraftID: (id: string | null) => void;
  addDraft: () => void;
  commitDraft: (id: string, patch: Partial<WorkloadDraft>) => void;
  removeDraft: (id: string) => void;
  provider: Provider;
  platformID: string;
  network: NetworkConfig;
  quotas: TenantQuotas;
  stroppyVersions: string[];
  versionsLoading: boolean;
  versionsLoaded: React.MutableRefObject<boolean>;
  setStroppyVersions: (v: string[]) => void;
  setVersionsLoading: (v: boolean) => void;
}) {
  const editingDraft = drafts.find((d) => d.clientID === editingDraftID) ?? null;
  // Probe target for active workload editor — first compatible db_preset.
  const probePreset = selectedPresets[0] ?? null;

  return (
    <div className="flex h-full">
      {/* LEFT — DB targets */}
      <div className="w-[400px] shrink-0 border-r border-zinc-800/50 overflow-y-auto p-5">
        <div className="mb-4">
          <h2 className="text-sm font-semibold text-zinc-200 inline-flex items-center gap-2">
            <Database className="w-4 h-4 text-primary" />
            DB Targets
          </h2>
          <p className="text-xs text-zinc-500 mt-1">
            {selectedPresetIDs.length} selected — each one becomes a row of the launch matrix.
          </p>
        </div>

        <div className="space-y-4">
          {ALL_DB_KINDS.filter((k) => presetsByKind[k]?.length).map((k) => {
            const color = DB_COLORS[k as keyof typeof DB_COLORS];
            return (
              <div key={k} className="space-y-1.5">
                <div className="flex items-center gap-2">
                  <span className="w-2 h-2 rounded-full" style={{ backgroundColor: color?.hex ?? "#666" }} />
                  <span className={`text-[11px] font-mono uppercase tracking-wider ${color?.text ?? "text-zinc-400"}`}>
                    {k}
                  </span>
                </div>
                <div className="space-y-1.5">
                  {presetsByKind[k].map((p) => {
                    const active = selectedPresetIDs.includes(p.id);
                    const Icon = active ? CheckSquare : Square;
                    return (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => onTogglePreset(p.id)}
                        className={`w-full border p-2 text-left transition-all cursor-pointer flex items-start gap-2 ${
                          active
                            ? color?.accent ?? "border-primary/40 bg-primary/[0.06]"
                            : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                        }`}
                      >
                        <Icon className={`w-3.5 h-3.5 shrink-0 mt-0.5 ${
                          active ? color?.text ?? "text-primary" : "text-zinc-700"
                        }`} />
                        <div className="flex-1 min-w-0">
                          <div className={`text-xs font-mono font-medium truncate ${
                            active ? color?.text ?? "text-primary" : "text-zinc-300"
                          }`}>
                            {p.name}
                          </div>
                          <div className="mt-1.5">
                            <TopologyDiagram kind={p.db_kind} topology={p.topology} />
                          </div>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>

        {selectedPresetIDs.length === 0 && (
          <div className="mt-4 border border-amber-500/30 bg-amber-500/[0.06] text-amber-400 text-[11px] font-mono p-2 inline-flex items-center gap-2">
            <AlertTriangle className="w-3 h-3" /> at least one required
          </div>
        )}
      </div>

      {/* RIGHT — workloads */}
      <div className="flex-1 min-w-0 overflow-y-auto">
        <div className="p-5">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-sm font-semibold text-zinc-200 inline-flex items-center gap-2">
                <Zap className="w-4 h-4 text-primary" />
                Workloads
              </h2>
              <p className="text-xs text-zinc-500 mt-1">
                {drafts.length} workload{drafts.length === 1 ? "" : "s"} — each cross-tabulates against every selected DB target.
              </p>
            </div>
            <Button size="sm" onClick={addDraft} className="gap-1.5">
              <Plus className="w-3 h-3" /> Add workload
            </Button>
          </div>

          {drafts.length === 0 ? (
            <div className="border border-dashed border-zinc-800 p-8 text-center text-xs font-mono text-zinc-500">
              No workloads yet — click "Add workload" to define one.
            </div>
          ) : (
            <div className="space-y-2">
              {drafts.map((d) => {
                const active = editingDraftID === d.clientID;
                const w = d.workload;
                const summary = [w.script, w.duration, w.vus ? `${w.vus} VU` : null, w.scale_factor ? `sf ${w.scale_factor}` : null]
                  .filter(Boolean).join(" · ");
                return (
                  <div key={d.clientID}>
                    <div className={`border ${
                      active ? "border-primary/40 bg-primary/[0.06]" : "border-zinc-800/60 hover:border-zinc-700"
                    } transition-colors`}>
                      <div className="flex items-center gap-3 px-3 py-2">
                        <div className="flex-1 min-w-0">
                          <div className={`text-xs font-mono font-medium truncate ${
                            active ? "text-primary" : "text-zinc-200"
                          }`}>
                            {d.name || "(unnamed)"}
                          </div>
                          <div className="text-[10px] font-mono text-zinc-500 truncate">{summary || "—"}</div>
                        </div>
                        {d.serverID && (
                          <span className="text-[9px] font-mono text-zinc-700 px-1.5 py-0.5 border border-zinc-800">saved</span>
                        )}
                        <Button
                          size="sm" variant="ghost"
                          onClick={() => setEditingDraftID(active ? null : d.clientID)}
                          className="text-zinc-500 hover:text-zinc-200"
                        >
                          {active ? <X className="w-3.5 h-3.5" /> : <Pencil className="w-3.5 h-3.5" />}
                        </Button>
                        <Button
                          size="sm" variant="ghost"
                          onClick={() => removeDraft(d.clientID)}
                          className="text-zinc-500 hover:text-destructive"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </Button>
                      </div>

                      {active && (
                        <div className="border-t border-zinc-800/50 p-4 bg-[#070707]">
                          <DraftEditor
                            draft={d}
                            commit={(patch) => commitDraft(d.clientID, patch)}
                            probePreset={probePreset}
                            provider={provider}
                            platformID={platformID}
                            network={network}
                            quotas={quotas}
                            stroppyVersions={stroppyVersions}
                            versionsLoading={versionsLoading}
                            versionsLoaded={versionsLoaded}
                            setStroppyVersions={setStroppyVersions}
                            setVersionsLoading={setVersionsLoading}
                          />
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── DraftEditor — wraps WorkloadForm with local state per draft ─

function DraftEditor({
  draft,
  commit,
  probePreset,
  provider,
  platformID,
  network,
  quotas,
  stroppyVersions,
  versionsLoading,
  versionsLoaded,
  setStroppyVersions,
  setVersionsLoading,
}: {
  draft: WorkloadDraft;
  commit: (patch: Partial<WorkloadDraft>) => void;
  probePreset: Preset | null;
  provider: Provider;
  platformID: string;
  network: NetworkConfig;
  quotas: TenantQuotas;
  stroppyVersions: string[];
  versionsLoading: boolean;
  versionsLoaded: React.MutableRefObject<boolean>;
  setStroppyVersions: (v: string[]) => void;
  setVersionsLoading: (v: boolean) => void;
}) {
  const w = draft.workload;
  const [name, setName] = useState(draft.name);

  const [script, setScript] = useState(w.script || "tpcc/procs");
  const [sql, setSql] = useState(w.sql || "");
  const [stroppyEnv, setStroppyEnv] = useState<Record<string, string>>(w.env || {});
  const [workloadFiles, setWorkloadFiles] = useState<WorkloadFile[]>(w.files || []);
  const [duration, setDuration] = useState(w.duration || "5m");
  const [k6Mode, setK6Mode] = useState<K6Mode>((w.k6_mode as K6Mode) || "duration");
  const [iterations, setIterations] = useState(w.iterations || 1);
  const [quiet, setQuiet] = useState(w.quiet ?? true);
  const [noThresholds, setNoThresholds] = useState(Boolean(w.no_thresholds));
  const [defaultInsertMethod, setDefaultInsertMethod] = useState(w.default_insert_method || "native");
  const [scaleFactor, setScaleFactor] = useState(w.scale_factor ?? 500);
  const [vus, setVUs] = useState(w.vus ?? 100);
  const [poolSize, setPoolSize] = useState(w.pool_size ?? 150);
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>(w.steps || []);
  const [noSteps, setNoSteps] = useState<string[]>(w.no_steps || []);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [, setWorkloadProbeOK] = useState(false);
  const [stroppyCpus, setStroppyCpus] = useState(w.machine?.cpus ?? 32);
  const [stroppyMemory, setStroppyMemory] = useState(w.machine?.memory_mb ?? 65536);
  const [stroppyDisk, setStroppyDisk] = useState(w.machine?.disk_gb ?? 100);
  const [stroppyDiskType, setStroppyDiskType] = useState(w.machine?.disk_type || "network-ssd");
  const [stroppyVersion, setStroppyVersion] = useState(w.version || "");

  // Lifted: parent owns the version list (one fetch for the whole suite).
  // When the list arrives and this draft has no pinned version, fall back
  // to versions[0] — the probe / binary-download path 404s on "latest".
  useEffect(() => {
    if (!stroppyVersion && stroppyVersions.length > 0) {
      setStroppyVersion(stroppyVersions[0]);
    }
  }, [stroppyVersions, stroppyVersion]);
  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(w.config_override_json || null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(w.config_override_json || null);
  const [stroppyConfigUserEdited, setStroppyConfigUserEdited] = useState(false);

  // Probe driver context — defaults to first selected suite preset.
  const probeKind: DatabaseKind = (probePreset?.db_kind as DatabaseKind) || "postgres";
  const probeProtocol: Protocol = useMemo(() => {
    const ps = KIND_PROTOCOLS[probeKind] ?? [];
    return (ps[0] ?? "pg") as Protocol;
  }, [probeKind]);

  // stroppy-config.json preview — mirror NewRun's useEffect (NewRun:373).
  // Builds a minimal RunConfig stitched from suite-level infra + this
  // draft's workload + the probe target preset, then asks the backend to
  // render the pristine stroppy-config.json. Skipped while the user has
  // hand-edited the editor so we don't blow away their changes.
  useEffect(() => {
    if (!probePreset || !stroppyVersion || stroppyConfigUserEdited) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      const stroppyCfg: StroppyConfig = {
        version: stroppyVersion,
        protocol: probeProtocol,
        script,
        sql: sql || undefined,
        duration,
        k6_mode: k6Mode,
        iterations: k6Mode === "iterations" ? iterations : undefined,
        quiet,
        no_thresholds: noThresholds || undefined,
        vus,
        pool_size: poolSize,
        scale_factor: scaleFactor,
        default_insert_method: defaultInsertMethod,
        env: Object.keys(stroppyEnv).length > 0 ? stroppyEnv : undefined,
        files: workloadFiles.length > 0 ? workloadFiles : undefined,
        no_steps: noSteps.length > 0 ? noSteps : undefined,
        steps: selectedSteps.length > 0 ? selectedSteps : undefined,
      };
      const minimalRun = {
        id: `preview-${draft.clientID}`,
        provider,
        network,
        machines: [],
        database: { kind: probeKind, version: defaultDBVersionTS(probeKind) },
        monitor: {},
        stroppy: stroppyCfg,
        preset_id: probePreset.id,
        platform_id: platformID || undefined,
      };
      previewStroppyConfig(minimalRun)
        .then((resp) => {
          if (cancelled) return;
          setStroppyConfigPristine(resp.stroppy_config);
          setStroppyConfigDraft(resp.stroppy_config);
          setStroppyConfigUserEdited(false);
        })
        .catch(() => {
          if (cancelled || stroppyConfigDraft !== null) return;
          setStroppyConfigPristine("");
          setStroppyConfigDraft("");
          setStroppyConfigUserEdited(false);
        });
    }, 300);
    return () => { cancelled = true; window.clearTimeout(timer); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    probePreset?.id, probeKind, probeProtocol, stroppyVersion,
    script, sql, duration, k6Mode, iterations, quiet, noThresholds,
    vus, poolSize, scaleFactor, defaultInsertMethod,
    stroppyEnv, workloadFiles, noSteps, selectedSteps,
    provider, platformID, network, stroppyConfigUserEdited,
  ]);

  // Auto-commit on every change so the parent draft stays in sync without
  // an explicit save. The list summary updates live.
  useEffect(() => {
    const workload: StroppyConfig = {
      version: stroppyVersion || "latest",
      script,
      duration,
      vus,
      pool_size: poolSize,
      scale_factor: scaleFactor,
      k6_mode: k6Mode,
      quiet,
      no_thresholds: noThresholds || undefined,
      iterations: k6Mode === "iterations" ? iterations : undefined,
      sql: sql || undefined,
      default_insert_method: defaultInsertMethod,
      env: Object.keys(stroppyEnv).length > 0 ? stroppyEnv : undefined,
      files: workloadFiles.length > 0 ? workloadFiles : undefined,
      no_steps: noSteps.length > 0 ? noSteps : undefined,
      steps: selectedSteps.length > 0 ? selectedSteps : undefined,
      config_override_json: stroppyConfigUserEdited && stroppyConfigDraft ? stroppyConfigDraft : undefined,
    };
    if (provider === "yandex") {
      workload.machine = {
        role: "stroppy",
        count: 1,
        cpus: stroppyCpus,
        memory_mb: stroppyMemory,
        disk_gb: stroppyDisk,
        disk_type: stroppyDiskType,
      };
    }
    commit({ name, workload });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    name, script, sql, stroppyEnv, workloadFiles, duration, k6Mode, iterations,
    quiet, noThresholds, defaultInsertMethod, scaleFactor, vus, poolSize,
    selectedSteps, noSteps, stroppyVersion, stroppyCpus, stroppyMemory,
    stroppyDisk, stroppyDiskType, stroppyConfigDraft, stroppyConfigUserEdited, provider,
  ]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider w-24 shrink-0">Name</Label>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="flex-1 h-8 bg-zinc-900 border border-zinc-800 px-2 font-mono text-xs text-zinc-200 outline-none focus:border-primary/60"
          placeholder="e.g. TPC-C 5m heavy"
        />
      </div>
      <div className="text-[10px] font-mono text-zinc-600 inline-flex items-center gap-2">
        Probe target: {probePreset ? (
          <span className="text-zinc-300">{probePreset.db_kind}/{probeProtocol} · {probePreset.name}</span>
        ) : (
          <span className="text-amber-500/80">no DB target selected — probe disabled</span>
        )}
      </div>

      <div className="border border-zinc-800/60 bg-[#050505] p-4">
        <WorkloadForm
          script={script} setScript={setScript}
          sql={sql} setSql={setSql}
          stroppyEnv={stroppyEnv} setStroppyEnv={setStroppyEnv}
          workloadFiles={workloadFiles} setWorkloadFiles={setWorkloadFiles}
          duration={duration} setDuration={setDuration}
          k6Mode={k6Mode} setK6Mode={setK6Mode}
          iterations={iterations} setIterations={setIterations}
          quiet={quiet} setQuiet={setQuiet}
          noThresholds={noThresholds} setNoThresholds={setNoThresholds}
          defaultInsertMethod={defaultInsertMethod} setDefaultInsertMethod={setDefaultInsertMethod}
          scaleFactor={scaleFactor} setScaleFactor={setScaleFactor}
          vus={vus} setVus={setVUs}
          poolSize={poolSize} setPoolSize={setPoolSize}
          dbKind={probeKind}
          protocol={probeProtocol}
          provider={provider}
          platformId={platformID}
          quotas={quotas}
          availableSteps={availableSteps} setAvailableSteps={setAvailableSteps}
          selectedSteps={selectedSteps} setSelectedSteps={setSelectedSteps}
          noSteps={noSteps} setNoSteps={setNoSteps}
          probeData={probeData} setProbeData={setProbeData}
          setWorkloadProbeOK={setWorkloadProbeOK}
          stroppyCpus={stroppyCpus} setStroppyCpus={setStroppyCpus}
          stroppyMemory={stroppyMemory} setStroppyMemory={setStroppyMemory}
          stroppyDisk={stroppyDisk} setStroppyDisk={setStroppyDisk}
          stroppyDiskType={stroppyDiskType} setStroppyDiskType={setStroppyDiskType}
          stroppyVersion={stroppyVersion} setStroppyVersion={setStroppyVersion}
          stroppyVersions={stroppyVersions} setStroppyVersions={setStroppyVersions}
          versionsLoading={versionsLoading} setVersionsLoading={setVersionsLoading}
          versionsLoaded={versionsLoaded}
          stroppyConfigDraft={stroppyConfigDraft}
          setStroppyConfigDraft={setStroppyConfigDraft}
          stroppyConfigPristine={stroppyConfigPristine}
          setStroppyConfigPristine={setStroppyConfigPristine}
          stroppyConfigUserEdited={stroppyConfigUserEdited}
          setStroppyConfigUserEdited={setStroppyConfigUserEdited}
        />
      </div>
    </div>
  );
}

// ─── Step 2: Review ────────────────────────────────────────────

function ReviewStep({
  editing, name, description, presets, drafts, provider, platformID,
  network, stroppyMachine, scheduleEnabled, cronExpr, timezone,
  policy, comparison, retentionRuns,
}: {
  editing: boolean;
  name: string; description: string;
  presets: Preset[]; drafts: WorkloadDraft[];
  provider: Provider; platformID: string;
  network: NetworkConfig; stroppyMachine: Partial<MachineSpec>;
  scheduleEnabled: boolean; cronExpr: string; timezone: string;
  policy: SuitePolicy; comparison: ComparisonConfig; retentionRuns: number;
}) {
  const cells = presets.length * drafts.length;
  return (
    <div className="p-5 space-y-5">
      <div>
        <h2 className="text-sm font-semibold text-zinc-200">{editing ? "Review changes" : "Review &amp; create"}</h2>
        <p className="text-xs text-zinc-500 mt-1">
          Final pass over the suite payload. The launch matrix has{" "}
          <span className="text-primary font-mono">{cells}</span> cell{cells === 1 ? "" : "s"} ({presets.length} preset
          {presets.length === 1 ? "" : "s"} × {drafts.length} workload{drafts.length === 1 ? "" : "s"}).
        </p>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <ReviewBlock title="Identity">
          <ReviewRow label="Name" value={name || <em className="text-zinc-700">unnamed</em>} />
          {description && <ReviewRow label="Description" value={<span className="text-zinc-400">{description}</span>} />}
        </ReviewBlock>

        <ReviewBlock title="Infrastructure">
          <ReviewRow label="Provider" value={PROVIDER_META[provider].label} />
          {provider === "yandex" && platformID && <ReviewRow label="Platform" value={platformID} />}
          <ReviewRow label="Network" value={<span className="font-mono">{network.cidr}{network.zone ? ` / ${network.zone}` : ""}</span>} />
          {(stroppyMachine.cpus || stroppyMachine.memory_mb || stroppyMachine.disk_gb) && (
            <ReviewRow
              label="Stroppy"
              value={<span className="font-mono">{stroppyMachine.cpus ?? "—"} vCPU · {stroppyMachine.memory_mb ?? "—"} MB · {stroppyMachine.disk_gb ?? "—"} GB</span>}
            />
          )}
        </ReviewBlock>

        <ReviewBlock title="Schedule">
          {scheduleEnabled ? (
            <>
              <ReviewRow label="Cron" value={<span className="font-mono">{cronExpr}</span>} />
              <ReviewRow label="Timezone" value={timezone} />
            </>
          ) : (
            <ReviewRow label="Mode" value={<span className="text-zinc-500">manual-only</span>} />
          )}
        </ReviewBlock>

        <ReviewBlock title="Policy">
          <ReviewRow label="Mode" value={<span className="capitalize">{policy.mode}</span>} />
          <ReviewRow label="On step fail" value={<span className="capitalize">{policy.on_step_fail}</span>} />
          {policy.mode === "parallel" && (
            <ReviewRow label="Max parallel" value={String(policy.max_parallel || "unlimited")} />
          )}
          <ReviewRow label="Baseline" value={comparison.strategy || "none"} />
          <ReviewRow label="Retention" value={`last ${retentionRuns} batches`} />
        </ReviewBlock>
      </div>

      <ReviewBlock title={`DB Targets (${presets.length})`}>
        {presets.length === 0 ? (
          <div className="text-[11px] font-mono text-amber-500/80">no presets — go back to Matrix step</div>
        ) : (
          <div className="grid grid-cols-2 gap-1">
            {presets.map((p) => {
              const color = DB_COLORS[p.db_kind as keyof typeof DB_COLORS];
              return (
                <div key={p.id} className="flex items-center gap-2 text-[11px] font-mono">
                  <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: color?.hex ?? "#666" }} />
                  <span className={color?.text ?? "text-zinc-300"}>{p.db_kind}</span>
                  <span className="text-zinc-600">·</span>
                  <span className="text-zinc-400 truncate">{p.name}</span>
                </div>
              );
            })}
          </div>
        )}
      </ReviewBlock>

      <ReviewBlock title={`Workloads (${drafts.length})`}>
        {drafts.length === 0 ? (
          <div className="text-[11px] font-mono text-amber-500/80">no workloads — go back to Matrix step</div>
        ) : (
          <div className="space-y-1">
            {drafts.map((d) => {
              const w = d.workload;
              const summary = [w.script, w.duration, w.vus ? `${w.vus} VU` : null].filter(Boolean).join(" · ");
              return (
                <div key={d.clientID} className="flex items-center gap-2 text-[11px] font-mono">
                  <Zap className="w-3 h-3 text-primary shrink-0" />
                  <span className="text-zinc-300">{d.name}</span>
                  <span className="text-zinc-600">·</span>
                  <span className="text-zinc-500 truncate">{summary || "—"}</span>
                </div>
              );
            })}
          </div>
        )}
      </ReviewBlock>
    </div>
  );
}

// ─── Helpers ───────────────────────────────────────────────────

function SectionCard({
  icon: Icon, title, subtitle, children, className,
}: {
  icon: typeof Database;
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`border border-zinc-800/60 bg-[#070707] ${className ?? ""}`}>
      <div className="px-4 py-2.5 border-b border-zinc-800/50 flex items-baseline justify-between">
        <h3 className="text-xs font-mono uppercase tracking-wider text-zinc-300 inline-flex items-center gap-2">
          <Icon className="w-3.5 h-3.5 text-primary" />
          {title}
        </h3>
        {subtitle && <span className="text-[10px] font-mono text-zinc-600">{subtitle}</span>}
      </div>
      <div className="p-4">{children}</div>
    </div>
  );
}

function FieldStack({
  label, required, children,
}: {
  label: string; required?: boolean; children: React.ReactNode;
}) {
  return (
    <div>
      <div className="flex items-baseline justify-between">
        <Label className="text-[11px] font-mono uppercase tracking-wider text-zinc-500">{label}</Label>
        {required && <span className="text-[10px] font-mono text-zinc-700">required</span>}
      </div>
      <div className="mt-1">{children}</div>
    </div>
  );
}

function FieldRow({
  label, hint, children,
}: {
  label: string; hint?: string; children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">{label}</Label>
      {children}
      {hint && <p className="text-[10px] font-mono text-zinc-600">{hint}</p>}
    </div>
  );
}

function ReviewBlock({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="border border-zinc-800/60 bg-[#070707] px-3 py-3 space-y-1.5">
      <h3 className="text-[11px] font-mono uppercase tracking-wider text-zinc-500 mb-2">{title}</h3>
      {children}
    </div>
  );
}

function ReviewRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-baseline gap-2 text-[11px] font-mono">
      <span className="text-zinc-600 w-24 shrink-0">{label}</span>
      <span className="text-zinc-300 truncate">{value}</span>
    </div>
  );
}

function CronPreview({
  expr, tz, loaded,
}: {
  expr: string; tz: string; loaded: Suite | null;
}) {
  if (!expr.trim()) {
    return <p className="text-[10px] font-mono text-zinc-600 mt-1">Pick a cron expression to schedule this suite.</p>;
  }
  if (!cronShapeOK(expr)) {
    return (
      <p className="text-[10px] font-mono text-red-400 mt-1 inline-flex items-center gap-1">
        <AlertCircle className="w-3 h-3" /> Doesn't look like a 5-field cron or @descriptor.
      </p>
    );
  }
  if (loaded && loaded.cron_expr === expr && loaded.next_fire_at) {
    return (
      <p className="text-[10px] font-mono text-emerald-500/80 mt-1 inline-flex items-center gap-1">
        <Clock className="w-3 h-3" /> Next fire ({tz}): {new Date(loaded.next_fire_at).toLocaleString()}
      </p>
    );
  }
  return <p className="text-[10px] font-mono text-zinc-600 mt-1">Save to compute the next firing time.</p>;
}

// defaultDBVersionTS mirrors backend's defaultDBVersion(). Keeps the
// preview RunConfig honest about the database version the launch path
// will eventually use, so the rendered stroppy config matches what gets
// run for the probe-target cell.
function defaultDBVersionTS(kind: DatabaseKind): string {
  switch (kind) {
    case "postgres": return "17";
    case "mysql": return "8.4";
    case "mariadb": return "11.4";
    case "picodata": return "25.3";
    case "ydb": return "25.3";
    case "ydb-managed": return "managed";
    case "cockroach": return "24.2";
    default: return "latest";
  }
}

function cronShapeOK(expr: string): boolean {
  const t = expr.trim();
  if (!t) return false;
  if (/^@(yearly|annually|monthly|weekly|daily|hourly)$/.test(t)) return true;
  return t.split(/\s+/).length === 5;
}

export default SuiteBuilder;
