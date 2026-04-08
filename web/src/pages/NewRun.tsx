import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, listPresets, listPackages } from "@/api/client";
import type {
  RunConfig,
  DatabaseKind,
  Provider,
  Preset,
  Package,
} from "@/api/types";
import { generateRunID } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import {
  Check,
  AlertCircle,
  Database,
  Server,
  Cpu,
  Cloud,
  Container,
  Copy,
  Rocket,
  ChevronRight,
  ChevronLeft,
  Loader2,
} from "lucide-react";

import { DB_COLORS } from "@/lib/db-colors";
import { NumericSlider, DurationSlider } from "@/components/ui/sliders";

// --- Constants ---

const DB_KINDS: DatabaseKind[] = ["postgres", "mysql", "picodata"];
const PROVIDERS: Provider[] = ["docker", "yandex"];
const WORKLOADS = ["tpcb", "tpcc"];

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["16", "17"],
  mysql: ["8.0", "8.4"],
  picodata: ["25.3"],
};

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres: { icon: Database, label: "PostgreSQL" },
  mysql:    { icon: Server,   label: "MySQL" },
  picodata: { icon: Cpu,      label: "Picodata" },
};

const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud,     label: "Yandex Cloud" },
};

const WORKLOAD_DESC: Record<string, string> = {
  tpcb: "TPC-B banking",
  tpcc: "TPC-C orders",
};

const STEPS = [
  { key: "infra", label: "Infrastructure" },
  { key: "database", label: "Database" },
  { key: "stroppy", label: "Workload" },
  { key: "review", label: "Review & Launch" },
];

// --- Main ---

export function NewRun() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const [step, setStep] = useState(0);

  const [allPresets, setAllPresets] = useState<Preset[]>([]);
  const [kind, setKind] = useState<DatabaseKind>(
    (searchParams.get("kind") as DatabaseKind) || "postgres"
  );
  const [selectedPresetId, setSelectedPresetId] = useState(searchParams.get("preset_id") || "");
  const [provider, setProvider] = useState<Provider>("docker");
  const [version, setVersion] = useState(DB_VERSIONS[kind][0]);
  const [workload, setWorkload] = useState("tpcc");
  const [duration, setDuration] = useState("5m");
  const [vusScale, setVusScale] = useState(1);
  const [poolSize, setPoolSize] = useState(100);
  const [scaleFactor, setScaleFactor] = useState(1);
  const [packageId, setPackageId] = useState("");
  const [availablePackages, setAvailablePackages] = useState<Package[]>([]);

  const [submitting, setSubmitting] = useState(false);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [dryRunResult, setDryRunResult] = useState<any>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { listPresets().then(setAllPresets).catch(() => {}); }, []);
  useEffect(() => {
    const matching = allPresets.filter((p) => p.db_kind === kind);
    if (matching.length > 0 && !matching.find((p) => p.id === selectedPresetId)) {
      setSelectedPresetId(matching[0].id);
    }
    setVersion(DB_VERSIONS[kind][0]);
  }, [kind, allPresets]);
  useEffect(() => {
    listPackages({ db_kind: kind, db_version: version }).then(setAvailablePackages).catch(() => {});
  }, [kind, version]);

  const presetsForKind = useMemo(
    () => allPresets.filter((p) => p.db_kind === kind),
    [allPresets, kind]
  );

  const selectedPreset = useMemo(
    () => presetsForKind.find((p) => p.id === selectedPresetId),
    [presetsForKind, selectedPresetId]
  );

  const runIDRef = useRef(generateRunID());

  const config = useMemo((): RunConfig => {
    const id = runIDRef.current;
    const cfg: RunConfig = {
      id, provider,
      network: { cidr: "10.0.0.0/24" },
      machines: [],
      database: { kind, version },
      monitor: {},
      stroppy: {
        version: "3.1.0",
        workload,
        duration,
        vus_scale: vusScale,
        pool_size: poolSize,
        scale_factor: scaleFactor,
      },
    };
    if (selectedPresetId) cfg.preset_id = selectedPresetId;
    if (packageId) cfg.package_id = packageId;
    return cfg;
  }, [kind, selectedPresetId, provider, version, workload, duration, vusScale, poolSize, scaleFactor, packageId]);

  const configJSON = useMemo(() => JSON.stringify(config, null, 2), [config]);

  // Auto-validate on step change
  useEffect(() => {
    if (step < 3) { setValidationResult(null); return; }
    let cancelled = false;
    setDryRunLoading(true);
    setDryRunResult(null);
    setValidationResult(null);
    setError(null);
    (async () => {
      try {
        await validateRun(config);
        if (!cancelled) setValidationResult({ ok: true, message: "Configuration is valid" });
      } catch (err) {
        if (!cancelled) setValidationResult({ ok: false, message: err instanceof Error ? err.message : "Validation failed" });
      }
      try {
        const dr = await dryRun(config);
        if (!cancelled) setDryRunResult(dr);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Dry run failed");
      }
      if (!cancelled) setDryRunLoading(false);
    })();
    return () => { cancelled = true; };
  }, [step, config]);

  const handleSubmit = useCallback(async () => {
    if (!config.stroppy.duration.trim()) { setError("Duration is required"); return; }
    setSubmitting(true); setError(null);
    try {
      const result = await startRun(config);
      navigate(`/runs/${result.run_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start run");
      setSubmitting(false);
    }
  }, [config, navigate]);

  function handleCopy() {
    navigator.clipboard.writeText(configJSON);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  const dbMeta = DB_META[kind];
  const dbColor = DB_COLORS[kind];
  const DbIcon = dbMeta.icon;
  const ProvIcon = PROVIDER_META[provider].icon;

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Step indicator */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-2.5">
        <div className="flex items-center gap-1">
          {STEPS.map((s, i) => (
            <div key={s.key} className="flex items-center gap-1">
              {i > 0 && <ChevronRight className="w-3 h-3 text-zinc-700" />}
              <button
                type="button"
                onClick={() => setStep(i)}
                className={`px-3 py-1 text-xs font-mono transition-all cursor-pointer ${
                  i === step
                    ? "text-primary border border-primary/30 bg-primary/[0.06]"
                    : i < step
                      ? "text-zinc-400 hover:text-zinc-300"
                      : "text-zinc-600"
                }`}
              >
                <span className="text-zinc-600 mr-1.5">{i + 1}.</span>
                {s.label}
              </button>
            </div>
          ))}
        </div>
      </div>

      {/* Main: step content left | sidebar right */}
      <div className="flex-1 min-h-0 flex overflow-hidden">
        {/* Left — step content */}
        <div className="flex-1 min-w-0 overflow-y-auto p-5">
          {step === 0 && (
            <StepInfra provider={provider} setProvider={setProvider} />
          )}
          {step === 1 && (
            <StepDatabase
              kind={kind} setKind={setKind}
              version={version} setVersion={setVersion}
              packageId={packageId} setPackageId={setPackageId}
              availablePackages={availablePackages}
              presetsForKind={presetsForKind}
              selectedPresetId={selectedPresetId} setSelectedPresetId={setSelectedPresetId}
              dbMeta={dbMeta} dbColor={dbColor}
            />
          )}
          {step === 2 && (
            <StepStroppy
              workload={workload} setWorkload={setWorkload}
              duration={duration} setDuration={setDuration}
              scaleFactor={scaleFactor} setScaleFactor={setScaleFactor}
              vusScale={vusScale} setVusScale={setVusScale}
              poolSize={poolSize} setPoolSize={setPoolSize}
            />
          )}
          {step === 3 && (
            <StepReview
              dryRunResult={dryRunResult}
              dryRunLoading={dryRunLoading}
              validationResult={validationResult}
              error={error}
              submitting={submitting}
              onSubmit={handleSubmit}
            />
          )}

          {/* Navigation */}
          {step < 3 && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-zinc-800/50">
              <Button variant="outline" size="sm" onClick={() => setStep(Math.max(0, step - 1))} disabled={step === 0}>
                <ChevronLeft className="h-3 w-3" /> Back
              </Button>
              <Button size="sm" onClick={() => setStep(step + 1)} className="gap-1.5">
                Next <ChevronRight className="h-3 w-3" />
              </Button>
            </div>
          )}
        </div>

        {/* Right sidebar */}
        <div className="w-80 shrink-0 flex flex-col bg-[#050505] border-l border-zinc-800/50 overflow-hidden">
          {/* Summary */}
          <div className="shrink-0 px-4 py-3 border-b border-zinc-800/50 space-y-2">
            <div className="text-[10px] font-mono text-zinc-600 uppercase tracking-wider">Setup Summary</div>
            <div className="space-y-1.5">
              <SummaryRow icon={ProvIcon} label="Provider" value={PROVIDER_META[provider].label} />
              <SummaryRow icon={DbIcon} label="Database" value={`${dbMeta.label} ${version}`} color={dbColor.text} />
              {selectedPreset && (
                <SummaryRow label="Topology" value={selectedPreset.name} />
              )}
              <SummaryRow label="Workload" value={`${workload.toUpperCase()} / ${duration}`} />
              <SummaryRow label="VUs" value={`${vusScale}x`} />
              <SummaryRow label="Pool" value={String(poolSize)} />
              {scaleFactor > 1 && <SummaryRow label="Scale" value={String(scaleFactor)} />}
            </div>
          </div>

          {/* Config JSON */}
          <div className="shrink-0 flex items-center justify-between px-4 py-2 border-b border-zinc-800/50">
            <div className="flex items-center gap-2">
              <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Config</span>
              <span className="text-[9px] font-mono text-zinc-700 tabular-nums">{runIDRef.current}</span>
            </div>
            <button type="button" onClick={handleCopy}
              className="p-1 text-zinc-600 hover:text-zinc-300 transition-colors cursor-pointer" title="Copy">
              {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
            </button>
          </div>
          <pre className="flex-1 p-3 text-[10px] font-mono leading-[1.5] text-zinc-500 overflow-auto selection:bg-primary/20">
            {configJSON}
          </pre>
        </div>
      </div>
    </div>
  );
}

// ─── Summary Row ─────────────────────────────────────────────────

function SummaryRow({ icon: Icon, label, value, color }: {
  icon?: typeof Database;
  label: string;
  value: string;
  color?: string;
}) {
  return (
    <div className="flex items-center gap-2 text-[11px] font-mono">
      {Icon && <Icon className={`w-3 h-3 shrink-0 ${color || "text-zinc-600"}`} />}
      {!Icon && <span className="w-3" />}
      <span className="text-zinc-600">{label}</span>
      <span className={`ml-auto ${color || "text-zinc-400"}`}>{value}</span>
    </div>
  );
}

// ─── Step 1: Infrastructure ──────────────────────────────────────

function StepInfra({ provider, setProvider }: {
  provider: Provider;
  setProvider: (p: Provider) => void;
}) {
  return (
    <div className="space-y-5 max-w-lg">
      <div>
        <h2 className="text-sm font-semibold mb-1">Where to run?</h2>
        <p className="text-xs text-zinc-500">Choose the infrastructure provider for provisioning machines.</p>
      </div>
      <div className="grid grid-cols-1 gap-3">
        {PROVIDERS.map((p) => {
          const pm = PROVIDER_META[p];
          const PIcon = pm.icon;
          const active = provider === p;
          return (
            <button type="button" key={p} onClick={() => setProvider(p)}
              className={`flex items-center gap-4 border p-4 transition-all cursor-pointer ${
                active
                  ? "border-primary/40 text-primary bg-primary/[0.06]"
                  : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <PIcon className={`h-6 w-6 ${active ? "text-primary" : "text-zinc-600"}`} />
              <div className="text-left">
                <div className={`text-sm font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{pm.label}</div>
                <div className="text-[10px] text-zinc-600">
                  {p === "docker" ? "Local containers — fast, no cloud credentials needed" : "Yandex Cloud VMs — real infrastructure, production-like"}
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ─── Step 2: Database ────────────────────────────────────────────

function StepDatabase({
  kind, setKind,
  version, setVersion,
  packageId, setPackageId,
  availablePackages,
  presetsForKind,
  selectedPresetId, setSelectedPresetId,
  dbMeta, dbColor,
}: {
  kind: DatabaseKind; setKind: (k: DatabaseKind) => void;
  version: string; setVersion: (v: string) => void;
  packageId: string; setPackageId: (v: string) => void;
  availablePackages: Package[];
  presetsForKind: Preset[];
  selectedPresetId: string; setSelectedPresetId: (v: string) => void;
  dbMeta: { icon: typeof Database; label: string };
  dbColor: { hex: string; text: string; accent: string };
}) {
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Database</h2>
        <p className="text-xs text-zinc-500">Choose the database engine, version, and topology preset.</p>
      </div>

      {/* DB Kind */}
      <div className="grid grid-cols-3 gap-2">
        {DB_KINDS.map((k) => {
          const meta = DB_META[k];
          const kColor = DB_COLORS[k];
          const Icon = meta.icon;
          const active = kind === k;
          return (
            <button type="button" key={k} onClick={() => setKind(k)}
              className={`flex items-center gap-2.5 border p-2.5 transition-all cursor-pointer ${
                active ? `${kColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <Icon className={`h-4 w-4 ${active ? kColor.text : "text-zinc-600"}`} />
              <span className={`text-sm font-mono font-medium ${active ? kColor.text : "text-zinc-500"}`}>{meta.label}</span>
            </button>
          );
        })}
      </div>

      {/* Version + Package */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
          <Select value={version} onValueChange={setVersion}>
            <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {DB_VERSIONS[kind].map((v) => (
                <SelectItem key={v} value={v}>{dbMeta.label} {v}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Package</Label>
          <Select value={packageId || "__default__"} onValueChange={(v) => setPackageId(v === "__default__" ? "" : v)}>
            <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__default__">Default</SelectItem>
              {availablePackages.map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.name}{p.has_deb ? " [.deb]" : ""}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <a href="/packages" className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage packages</a>
        </div>
      </div>

      {/* Topology Preset */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Topology Preset</span>
          <a href="/presets" className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300">manage presets</a>
        </div>
        <div className="grid grid-cols-3 gap-3">
          {presetsForKind.map((p) => {
            const active = selectedPresetId === p.id;
            return (
              <button type="button" key={p.id} onClick={() => setSelectedPresetId(p.id)}
                className={`border p-3 text-left transition-all cursor-pointer ${
                  active ? `${dbColor.accent}` : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                }`}
              >
                <div className="flex items-center justify-between mb-2">
                  <span className={`text-xs font-mono font-semibold uppercase tracking-wider ${active ? dbColor.text : "text-zinc-400"}`}>
                    {p.name}
                  </span>
                  <div className="flex items-center gap-1">
                    {!p.is_builtin && <span className="text-[8px] text-zinc-600 font-mono">custom</span>}
                    {active && <div className="w-1.5 h-1.5 rounded-full" style={{ backgroundColor: dbColor.hex }} />}
                  </div>
                </div>
                <TopologyDiagram kind={kind} topology={p.topology} />
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// ─── Step 3: Stroppy ─────────────────────────────────────────────

function StepStroppy({
  workload, setWorkload,
  duration, setDuration,
  scaleFactor, setScaleFactor,
  vusScale, setVusScale,
  poolSize, setPoolSize,
}: {
  workload: string; setWorkload: (v: string) => void;
  duration: string; setDuration: (v: string) => void;
  scaleFactor: number; setScaleFactor: (v: number) => void;
  vusScale: number; setVusScale: (v: number) => void;
  poolSize: number; setPoolSize: (v: number) => void;
}) {
  return (
    <div className="space-y-5 max-w-lg">
      <div>
        <h2 className="text-sm font-semibold mb-1">Workload Settings</h2>
        <p className="text-xs text-zinc-500">Configure the Stroppy test runner parameters.</p>
      </div>

      <div className="grid grid-cols-2 gap-3">
        {WORKLOADS.map((w) => {
          const active = workload === w;
          return (
            <button type="button" key={w} onClick={() => setWorkload(w)}
              className={`border p-3 text-left transition-all cursor-pointer ${
                active ? "border-primary/40 bg-primary/[0.06]" : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <div className={`text-sm font-mono font-semibold uppercase ${active ? "text-primary" : "text-zinc-400"}`}>{w}</div>
              <div className="text-[10px] text-zinc-600 mt-0.5">{WORKLOAD_DESC[w]}</div>
            </button>
          );
        })}
      </div>

      <div className="grid grid-cols-2 gap-x-6 gap-y-4">
        <DurationSlider label="Duration" value={duration} onChange={setDuration} />
        <NumericSlider label="Scale Factor" value={scaleFactor} min={1} max={100}
          onChange={setScaleFactor} hint="TPC-C warehouses" />
        <NumericSlider label="VUS Scale" value={vusScale} min={1} max={50}
          onChange={setVusScale} hint="1 = ~99 VUs for TPC-C" />
        <NumericSlider label="Pool Size" value={poolSize} min={10} max={1000} step={10}
          onChange={setPoolSize} hint="DB connections" />
      </div>
    </div>
  );
}

// ─── Step 4: Review & Launch ─────────────────────────────────────

// Phase grouping — mirrors DAG dependency structure (same as DagGraph.tsx)
const PHASE_GROUPS: { label: string; icon: typeof Database; phases: string[] }[] = [
  { label: "Infrastructure", icon: Server, phases: ["network", "machines"] },
  { label: "Database", icon: Database, phases: ["install_etcd", "configure_etcd", "install_patroni", "configure_patroni", "install_db", "configure_db", "install_pgbouncer", "configure_pgbouncer"] },
  { label: "Proxy", icon: Server, phases: ["install_proxy", "configure_proxy"] },
  { label: "Monitoring", icon: Server, phases: ["install_monitor", "configure_monitor"] },
  { label: "Benchmark", icon: Rocket, phases: ["install_stroppy", "run_stroppy"] },
  { label: "Teardown", icon: Server, phases: ["teardown"] },
];

function humanPhase(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

interface DryRunNode {
  id: string;
  type: string;
  deps?: string[];
}

function StepReview({
  dryRunResult,
  dryRunLoading,
  validationResult,
  error,
  submitting,
  onSubmit,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dryRunResult: any;
  dryRunLoading: boolean;
  validationResult: { ok: boolean; message: string } | null;
  error: string | null;
  submitting: boolean;
  onSubmit: () => void;
}) {
  const canLaunch = validationResult?.ok && !dryRunLoading && !error;

  // Parse dry-run nodes
  const dagNodes: DryRunNode[] = dryRunResult?.nodes || [];
  const nodeIds = new Set(dagNodes.map((n: DryRunNode) => n.id));

  // Build active groups from plan
  const activeGroups = PHASE_GROUPS
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeIds.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = dagNodes.length;

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Review & Launch</h2>
        <p className="text-xs text-zinc-500">
          {dryRunLoading ? "Validating configuration and building execution plan..." : "Review the execution plan and launch the run."}
        </p>
      </div>

      {/* Validation status */}
      {dryRunLoading && (
        <div className="flex items-center gap-2 text-xs p-3 border border-zinc-800/50 font-mono text-zinc-500">
          <Loader2 className="h-3 w-3 animate-spin" />
          Preparing execution plan...
        </div>
      )}
      {validationResult && !dryRunLoading && (
        <div className={`flex items-center gap-2 text-xs p-3 border font-mono ${
          validationResult.ok ? "border-emerald-500/30 text-emerald-400" : "border-red-500/30 text-red-400"
        }`}>
          {validationResult.ok ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {validationResult.message}
          {validationResult.ok && totalPhases > 0 && (
            <span className="ml-auto text-zinc-600">{totalPhases} phases</span>
          )}
        </div>
      )}
      {error && (
        <div className="flex items-center gap-2 text-xs p-3 border border-red-500/30 text-red-400 font-mono">
          <AlertCircle className="h-3 w-3" />
          {error}
        </div>
      )}

      {/* Execution plan — compact DAG */}
      {activeGroups.length > 0 && (
        <div className="select-none max-w-xl">
          {activeGroups.map((group, gi) => {
            const isLast = gi === activeGroups.length - 1;
            const GroupIcon = group.icon;

            return (
              <div key={group.label} className="relative">
                {gi > 0 && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2.5 bg-zinc-800" />
                  </div>
                )}

                <div className="border border-zinc-800/80 bg-zinc-900/30">
                  <div className="flex items-center gap-2 px-3 py-2">
                    <div className="w-6 h-6 rounded-full bg-zinc-900 border border-zinc-700 flex items-center justify-center shrink-0">
                      <GroupIcon className="w-3 h-3 text-zinc-500" />
                    </div>
                    <span className="text-[11px] font-semibold text-zinc-300 font-mono flex-1">{group.label}</span>
                    <span className="text-[10px] text-zinc-600 font-mono tabular-nums">{group.phases.length}</span>
                  </div>

                  <div className="border-t border-zinc-800/50">
                    {group.phases.map((phaseId, pi) => {
                      const node = dagNodes.find((n: DryRunNode) => n.id === phaseId);
                      const isLastStep = pi === group.phases.length - 1;
                      const deps = node?.deps?.filter((d: string) => nodeIds.has(d)) || [];

                      return (
                        <div key={phaseId}>
                          <div className="flex items-center gap-2 px-3 py-1.5 relative">
                            {!isLastStep && (
                              <div className="absolute left-[17px] top-[22px] w-px h-[calc(100%-10px)] bg-zinc-800" />
                            )}
                            <div className="w-[14px] h-[14px] rounded-full border border-zinc-700 bg-zinc-900 flex items-center justify-center shrink-0">
                              <div className="w-1 h-1 rounded-full bg-zinc-600" />
                            </div>
                            <span className="text-[11px] font-mono text-zinc-400 flex-1">{humanPhase(phaseId)}</span>
                            {deps.length > 0 && (
                              <span className="text-[9px] text-zinc-700 font-mono shrink-0">← {deps.map(humanPhase).join(", ")}</span>
                            )}
                          </div>
                          {!isLastStep && <div className="mx-3 border-b border-zinc-800/20" />}
                        </div>
                      );
                    })}
                  </div>
                </div>

                {!isLast && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2 bg-zinc-800" />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Launch */}
      <Button
        size="lg"
        onClick={onSubmit}
        disabled={!canLaunch || submitting}
        className="w-full gap-2 h-12 text-base"
      >
        <Rocket className="h-5 w-5" />
        {submitting ? "Launching..." : "Launch Run"}
      </Button>
    </div>
  );
}
