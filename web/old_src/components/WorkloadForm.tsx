import { useEffect, useRef, useState } from "react";
import {
  probeScript,
  getStroppyCommits,
  getStroppyVersions,
  type StroppyCommit,
} from "@/api/client";
import {
  SCRIPT_COMPAT,
  type DatabaseKind,
  type Protocol,
  type Provider,
  type ProbeResponse,
  type TenantQuotas,
  type WorkloadFile,
} from "@/api/types";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { JsonEditor } from "@/components/ui/json-editor";
import {
  AlertCircle,
  Check,
  Loader2,
  Upload,
  X,
  Cpu,
  RotateCcw,
  ChevronDown,
} from "lucide-react";
import {
  NumericSlider,
  DurationSlider,
  SliderField,
  CPU_STEPS,
  ramSteps,
  DiskTypeSelect,
  cpuStepsForPlatform,
  platformLimits,
  closestStep,
  diskStepsForType,
} from "@/components/ui/sliders";

// WorkloadForm renders the full Stroppy workload configuration surface used
// by both the new-run wizard (Step 3) and the suite item editor (workload
// axis of the matrix). It is a single source of truth — extracted from
// NewRun's StepStroppy so both sites stay in lockstep when probe metadata,
// LOAD_WORKERS linking, env-declaration handling, etc. evolve.
//
// State is owned by the caller (controlled component) so the wizard can
// fold the same fields into its dryRun/launch payload and the suite editor
// can serialise them as the items axis (StroppyConfig blob). The component
// performs the live probe (debounce 350 ms) on script/sql/version/env
// changes and reports availability via setWorkloadProbeOK so the parent
// can gate forward-navigation on a successful probe.

export type K6Mode = "duration" | "iterations";

// Presentation labels for known stroppy scripts. The source of truth for
// which scripts run on which (kind, protocol) is types.SCRIPT_COMPAT;
// SCRIPT_META just adds human-readable strings on top.
export const SCRIPT_META: Record<string, { label: string; desc: string }> = {
  "tpcc/procs":          { label: "TPC-C Procs", desc: "Stored procedures" },
  "tpcc/tx":             { label: "TPC-C Tx",    desc: "Raw transactions" },
  "tpcb/procs":          { label: "TPC-B Procs", desc: "Stored procedures" },
  "tpcb/tx":             { label: "TPC-B Tx",    desc: "Raw transactions" },
  "tpch/tx":             { label: "TPC-H Tx",    desc: "Analytical queries" },
  "tpcc/tx-ydb-pgwire":  { label: "TPC-C Tx (YDB pgwire)", desc: "Subset that fits YDB's pg-wire feature ceiling" },
  "tpcb/tx-ydb-pgwire":  { label: "TPC-B Tx (YDB pgwire)", desc: "Subset that fits YDB's pg-wire feature ceiling" },
};

export const DEFAULT_INSERT_METHODS = ["native", "plain_bulk", "plain_query"] as const;

// probeDriverType maps a (protocol, db_kind) pair to the driver_type the
// stroppy probe endpoint expects. Mirrors backend resolution; deliberately
// duplicated here because the wizard probes pre-launch.
export function probeDriverType(protocol: Protocol, dbKind: DatabaseKind): string {
  switch (protocol) {
    case "mysql": return "mysql";
    case "picodata": return "picodata";
    case "ydb-grpc":
    case "ydb-grpcs": return "ydb";
    case "pg":
    case "ydb-pgwire":
    case "cockroach": return "postgres";
    default: return dbKind;
  }
}

// Heuristic stroppy runner sizing. Suggested in the cloud-only "Stroppy
// Runner" section so users have a starting point that matches their VU /
// pool configuration without measuring a baseline run first.
export function suggestStroppyMachine(vus: number, poolSize: number): { cpus: number; memory: number; disk: number; reason: string } {
  const rawCpus = Math.max(2, Math.min(96, Math.ceil(vus / 20) * 2));
  const cpus = closestStep(rawCpus, CPU_STEPS);
  const memBase = cpus * 2048;
  const memPool = Math.ceil(poolSize / 100) * 512;
  const memory = Math.max(2048, memBase + memPool);
  const disk = 25;
  const reason = `${vus} VUs → ${cpus} vCPU, pool ${poolSize} → ${(memory/1024).toFixed(0)} GB RAM`;
  return { cpus, memory, disk, reason };
}

export function safeUploadedFileName(name: string): string {
  const base = name.split(/[\\/]/).pop() || "workload.sql";
  const safe = base.replace(/[^A-Za-z0-9._-]/g, "_");
  return safe.endsWith(".sql") ? safe : `${safe}.sql`;
}

export function setEnvOverride(
  setStroppyEnv: React.Dispatch<React.SetStateAction<Record<string, string>>>,
  name: string,
  value: string,
) {
  setStroppyEnv((prev) => {
    const next = { ...prev };
    if (value === "") delete next[name];
    else next[name] = value;
    return next;
  });
}

// StroppyVersionCombo is a free-text input + chevron-button popover.
// HTML's <datalist> filters its options against the input value, which
// makes the dropdown disappear the moment the field has any prefilled
// text — wrong UX for "pick a release or type your own". This always
// shows everything.
export function StroppyVersionCombo({
  value,
  onChange,
  versions,
  setVersions,
  loading,
  setLoading,
  versionsLoaded,
}: {
  value: string;
  onChange: (v: string) => void;
  versions: string[];
  setVersions: (v: string[]) => void;
  loading: boolean;
  setLoading: (v: boolean) => void;
  versionsLoaded: React.MutableRefObject<boolean>;
}) {
  const [open, setOpen] = useState(false);
  const ensureLoaded = () => {
    if (!versionsLoaded.current || versions.length === 0) {
      versionsLoaded.current = true;
      setLoading(true);
      getStroppyVersions()
        .then((v) => { if (v.length > 0) setVersions(v); })
        .catch(() => {})
        .finally(() => setLoading(false));
    }
  };
  return (
    <div className="flex items-stretch border border-zinc-800 rounded bg-zinc-900 focus-within:border-zinc-600 overflow-hidden">
      <input
        type="text"
        autoComplete="off"
        value={value}
        placeholder={loading ? "loading…" : "5.0.0rc3"}
        onChange={(e) => onChange(e.target.value.trim())}
        spellCheck={false}
        className="bg-transparent px-2 py-0.5 text-[11px] font-mono text-zinc-300 outline-none w-[120px]"
        title="Pick a known release or type a version tag"
      />
      <Popover open={open} onOpenChange={(o) => { setOpen(o); if (o) ensureLoaded(); }}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="px-1.5 border-l border-zinc-800 text-zinc-500 hover:text-zinc-300 flex items-center"
            aria-label="Show known releases"
          >
            <ChevronDown className="h-3 w-3" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-44 p-0 bg-zinc-950 border-zinc-800 max-h-[280px] overflow-y-auto">
          {loading && versions.length === 0 && (
            <div className="px-3 py-2 text-[11px] font-mono text-zinc-500">loading…</div>
          )}
          {versions.map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => { onChange(v); setOpen(false); }}
              className={`w-full text-left px-3 py-1.5 text-[11px] font-mono hover:bg-zinc-900 transition-colors ${v === value ? "text-primary" : "text-zinc-300"}`}
            >
              v{v}
            </button>
          ))}
        </PopoverContent>
      </Popover>
    </div>
  );
}

export interface WorkloadFormProps {
  // Workload state (fully controlled).
  script: string; setScript: (v: string) => void;
  sql: string; setSql: (v: string) => void;
  stroppyEnv: Record<string, string>; setStroppyEnv: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  workloadFiles: WorkloadFile[]; setWorkloadFiles: React.Dispatch<React.SetStateAction<WorkloadFile[]>>;
  duration: string; setDuration: (v: string) => void;
  k6Mode: K6Mode; setK6Mode: (v: K6Mode) => void;
  iterations: number; setIterations: (v: number) => void;
  quiet: boolean; setQuiet: React.Dispatch<React.SetStateAction<boolean>>;
  noThresholds: boolean; setNoThresholds: React.Dispatch<React.SetStateAction<boolean>>;
  defaultInsertMethod: string; setDefaultInsertMethod: (v: string) => void;
  scaleFactor: number; setScaleFactor: (v: number) => void;
  vus: number; setVus: (v: number) => void;
  poolSize: number; setPoolSize: (v: number) => void;

  // Database/protocol context — drives probe driver_type and the script picker.
  dbKind: DatabaseKind;
  protocol: Protocol;

  // Provider/platform context — only the Stroppy Runner machine sliders care
  // (runner sizing only matters for cloud providers).
  provider: Provider;
  platformId: string;
  quotas: TenantQuotas;

  // Probe-derived state shared with the parent so it can validate forward
  // navigation and surface step lists in other parts of the UI.
  availableSteps: string[]; setAvailableSteps: (v: string[]) => void;
  selectedSteps: string[]; setSelectedSteps: (v: string[]) => void;
  noSteps: string[]; setNoSteps: (v: string[]) => void;
  probeData: ProbeResponse | null; setProbeData: (v: ProbeResponse | null) => void;
  setWorkloadProbeOK: (v: boolean) => void;

  // Stroppy runner machine spec.
  stroppyCpus: number; setStroppyCpus: (v: number) => void;
  stroppyMemory: number; setStroppyMemory: (v: number) => void;
  stroppyDisk: number; setStroppyDisk: (v: number) => void;
  stroppyDiskType: string; setStroppyDiskType: (v: string) => void;

  // Stroppy binary version (release tag or commit:<sha>).
  stroppyVersion: string; setStroppyVersion: (v: string) => void;
  stroppyVersions: string[]; setStroppyVersions: (v: string[]) => void;
  versionsLoading: boolean; setVersionsLoading: (v: boolean) => void;
  versionsLoaded: React.MutableRefObject<boolean>;

  // Raw stroppy-config.json editor — wins over form fields when non-pristine.
  stroppyConfigDraft: string | null;
  setStroppyConfigDraft: (v: string) => void;
  stroppyConfigPristine: string | null;
  setStroppyConfigPristine: (v: string) => void;
  stroppyConfigUserEdited: boolean;
  setStroppyConfigUserEdited: (v: boolean) => void;
}

export function WorkloadForm({
  script, setScript,
  sql, setSql,
  stroppyEnv, setStroppyEnv,
  workloadFiles, setWorkloadFiles,
  duration, setDuration,
  k6Mode, setK6Mode,
  iterations, setIterations,
  quiet, setQuiet,
  noThresholds, setNoThresholds,
  defaultInsertMethod, setDefaultInsertMethod,
  scaleFactor, setScaleFactor,
  vus, setVus,
  poolSize, setPoolSize,
  dbKind,
  protocol,
  provider,
  platformId,
  quotas,
  availableSteps, setAvailableSteps,
  selectedSteps: _selectedSteps, setSelectedSteps: _setSelectedSteps,
  noSteps, setNoSteps,
  probeData, setProbeData,
  setWorkloadProbeOK,
  stroppyCpus, setStroppyCpus,
  stroppyMemory, setStroppyMemory,
  stroppyDisk, setStroppyDisk,
  stroppyDiskType, setStroppyDiskType,
  stroppyVersion, setStroppyVersion,
  stroppyVersions, setStroppyVersions,
  versionsLoading, setVersionsLoading,
  versionsLoaded,
  stroppyConfigDraft,
  setStroppyConfigDraft,
  stroppyConfigPristine,
  setStroppyConfigPristine: _setStroppyConfigPristine,
  stroppyConfigUserEdited,
  setStroppyConfigUserEdited,
}: WorkloadFormProps) {
  const [probeLoading, setProbeLoading] = useState(false);
  const [probeError, setProbeError] = useState<string | null>(null);
  const [probeExpanded, setProbeExpanded] = useState(false);
  const [loadWorkersLinked, setLoadWorkersLinked] = useState(() => {
    const current = stroppyEnv.LOAD_WORKERS;
    return current === undefined || current === "" || current === String(stroppyCpus);
  });

  // Probe on script change to get steps/env.
  useEffect(() => {
    if (!script.trim() || !stroppyVersion.trim()) {
      setWorkloadProbeOK(false);
      return;
    }
    let cancelled = false;
    setProbeLoading(true);
    setWorkloadProbeOK(false);
    const timer = window.setTimeout(() => {
      probeScript({
        version: stroppyVersion,
        script: script.trim(),
        sql: sql.trim() || undefined,
        driver_type: probeDriverType(protocol, dbKind),
        pool_size: poolSize,
        scale_factor: scaleFactor,
        env: stroppyEnv,
        files: workloadFiles,
        include_human: true,
      })
        .then((data) => {
          if (cancelled) return;
          setProbeData(data);
          setAvailableSteps(data.steps || []);
          setProbeError(null);
          setWorkloadProbeOK(true);
        })
        .catch((e) => {
          if (cancelled) return;
          setProbeData(null);
          setAvailableSteps([]);
          setProbeError(e instanceof Error ? e.message : "Probe failed");
          setWorkloadProbeOK(false);
        })
        .finally(() => { if (!cancelled) setProbeLoading(false); });
    }, 350);
    return () => { cancelled = true; window.clearTimeout(timer); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [script, sql, dbKind, protocol, poolSize, scaleFactor, stroppyVersion, stroppyEnv, workloadFiles]);

  useEffect(() => {
    if (!loadWorkersLinked) return;
    setEnvOverride(setStroppyEnv, "LOAD_WORKERS", String(stroppyCpus));
  }, [loadWorkersLinked, setStroppyEnv, stroppyCpus]);

  const toggleNoStep = (step: string) => {
    setNoSteps(noSteps.includes(step) ? noSteps.filter((s) => s !== step) : [...noSteps, step]);
  };

  // Mode is derived from current stroppyVersion value: commit:<sha> => "commit", else "release".
  const isCommit = stroppyVersion.startsWith("commit:");
  const [stroppyMode, setStroppyMode] = useState<"release" | "commit">(isCommit ? "commit" : "release");
  const [stroppyCommits, setStroppyCommits] = useState<StroppyCommit[]>([]);
  const [commitsLoading, setCommitsLoading] = useState(false);
  const [commitsError, setCommitsError] = useState<string | null>(null);
  const commitsLoaded = useRef(false);

  const loadCommits = () => {
    if (commitsLoaded.current) return;
    commitsLoaded.current = true;
    setCommitsLoading(true);
    setCommitsError(null);
    getStroppyCommits()
      .then((cs) => setStroppyCommits(cs || []))
      .catch((e) => setCommitsError(e?.message || "fetch failed"))
      .finally(() => setCommitsLoading(false));
  };

  const switchMode = (m: "release" | "commit") => {
    setStroppyMode(m);
    if (m === "release" && isCommit) {
      setStroppyVersion(stroppyVersions[0] || "");
    } else if (m === "commit" && !isCommit) {
      loadCommits();
      setStroppyVersion("");
    }
  };

  const currentCommitSha = isCommit ? stroppyVersion.slice("commit:".length) : "";
  const knownScripts = SCRIPT_COMPAT[`${dbKind}:${protocol}`] || [];
  const uploadedSQL = workloadFiles.find((f) => f.kind === "sql" || f.name.endsWith(".sql"));
  const loadWorkersValue = loadWorkersLinked ? String(stroppyCpus) : (stroppyEnv.LOAD_WORKERS ?? "");
  const stroppyErrorMode = stroppyEnv.STROPPY_ERROR_MODE ?? "";

  const handleSQLUpload = async (file: File | null) => {
    if (!file) return;
    const name = safeUploadedFileName(file.name);
    const content = await file.text();
    setWorkloadFiles([{ name, kind: "sql", content }]);
    setSql(name);
  };

  const clearUploadedSQL = () => {
    if (uploadedSQL && sql === uploadedSQL.name) setSql("");
    setWorkloadFiles((prev) => prev.filter((f) => f.name !== uploadedSQL?.name));
  };

  const updateStroppyConfigDraft = (value: string) => {
    setStroppyConfigDraft(value);
    setStroppyConfigUserEdited(stroppyConfigPristine === null ? stroppyConfigUserEdited : value !== stroppyConfigPristine);
  };

  const resetStroppyConfigDraft = () => {
    if (stroppyConfigPristine !== null) {
      setStroppyConfigDraft(stroppyConfigPristine);
      setStroppyConfigUserEdited(false);
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold mb-1">Workload Settings</h2>
          <p className="text-xs text-zinc-500">Choose the benchmark script and tune execution parameters.</p>
        </div>
        {/* Stroppy binary source: release tag OR per-commit CI artifact */}
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-zinc-600">stroppy</span>
          <div className="inline-flex border border-zinc-800 rounded overflow-hidden text-[10px] font-mono">
            <button
              type="button"
              onClick={() => switchMode("release")}
              className={`px-2 py-0.5 ${stroppyMode === "release" ? "bg-zinc-700 text-zinc-100" : "bg-zinc-900 text-zinc-500 hover:text-zinc-300"}`}
            >release</button>
            <button
              type="button"
              onClick={() => switchMode("commit")}
              className={`px-2 py-0.5 border-l border-zinc-800 ${stroppyMode === "commit" ? "bg-zinc-700 text-zinc-100" : "bg-zinc-900 text-zinc-500 hover:text-zinc-300"}`}
            >commit</button>
          </div>
          {stroppyMode === "release" ? (
            <StroppyVersionCombo
              value={isCommit ? "" : stroppyVersion}
              onChange={setStroppyVersion}
              versions={stroppyVersions}
              setVersions={setStroppyVersions}
              loading={versionsLoading}
              setLoading={setVersionsLoading}
              versionsLoaded={versionsLoaded}
            />
          ) : (
            <>
              <input
                list="stroppy-commits-list"
                value={currentCommitSha}
                placeholder={commitsLoading ? "loading…" : "short SHA (7+ hex)"}
                onChange={(e) => {
                  const raw = e.target.value.trim().toLowerCase();
                  const sha = raw.slice(0, 40).replace(/[^0-9a-f]/g, "");
                  setStroppyVersion(sha ? `commit:${sha.slice(0, 7)}` : "");
                }}
                onFocus={loadCommits}
                spellCheck={false}
                className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 text-[11px] font-mono text-zinc-300 outline-none focus:border-zinc-600 w-[280px]"
                title={commitsError || "Enter commit SHA or pick from recent builds"}
              />
              <datalist id="stroppy-commits-list">
                {stroppyCommits.map((c) => {
                  const label = c.name && c.name !== c.tag ? c.name : c.tag;
                  return (
                    <option key={c.short} value={c.short}>{label.slice(0, 60)}</option>
                  );
                })}
              </datalist>
              {commitsError && (
                <span className="text-[10px] font-mono text-red-400" title={commitsError}>err</span>
              )}
            </>
          )}
        </div>
      </div>

      <div className="space-y-3">
        <div>
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Known Scripts</span>
          <div className="grid grid-cols-2 gap-2">
            {knownScripts.map((id) => {
              const meta = SCRIPT_META[id] || { label: id, desc: "" };
              const active = script === id;
              return (
                <button type="button" key={id}
                  onClick={() => setScript(id)}
                  className={`border p-3 text-left transition-all cursor-pointer ${
                    active
                      ? "border-primary/40 bg-primary/[0.06]"
                      : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                  }`}
                >
                  <div className={`text-xs font-mono font-semibold ${active ? "text-primary" : "text-zinc-400"}`}>{meta.label}</div>
                  {meta.desc && (
                    <div className="text-[10px] text-zinc-600 mt-0.5">{meta.desc}</div>
                  )}
                </button>
              );
            })}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Script</Label>
            <input
              value={script}
              onChange={(e) => setScript(e.target.value)}
              placeholder="tpcc/tx or ./bench.ts or queries.sql"
              spellCheck={false}
              className="h-8 w-full px-2 font-mono text-xs bg-zinc-900 border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">SQL</Label>
            <div className="flex gap-2">
              <input
                value={sql}
                onChange={(e) => setSql(e.target.value)}
                placeholder="optional SQL file/name"
                spellCheck={false}
                className="h-8 min-w-0 flex-1 px-2 font-mono text-xs bg-zinc-900 border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
              />
              <label className="h-8 px-2 border border-zinc-800 bg-zinc-900 text-zinc-500 hover:text-zinc-300 flex items-center gap-1 text-[10px] font-mono cursor-pointer">
                <Upload className="h-3 w-3" />
                SQL
                <input
                  type="file"
                  accept=".sql,text/sql,text/plain"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0] || null;
                    void handleSQLUpload(f);
                    e.currentTarget.value = "";
                  }}
                />
              </label>
            </div>
            {uploadedSQL && (
              <div className="flex items-center gap-2 text-[10px] font-mono text-zinc-500">
                <span className="truncate">{uploadedSQL.name}</span>
                <span className="text-zinc-700">{uploadedSQL.content.length} chars</span>
                <button type="button" onClick={clearUploadedSQL} className="ml-auto text-zinc-600 hover:text-zinc-300">
                  <X className="h-3 w-3" />
                </button>
              </div>
            )}
          </div>
        </div>

        {(probeLoading || probeError || probeData) && (
          <div className={`border font-mono ${
            probeError ? "border-red-500/30 text-red-400" : probeLoading ? "border-zinc-800/60 text-zinc-500" : "border-emerald-500/30 text-emerald-400"
          }`}>
            <button
              type="button"
              onClick={() => setProbeExpanded((v) => !v)}
              aria-expanded={probeExpanded}
              className="flex items-center gap-2 text-xs p-2 w-full text-left"
            >
              {probeLoading ? <Loader2 className="h-3 w-3 animate-spin shrink-0" /> : probeError ? <AlertCircle className="h-3 w-3 shrink-0" /> : <Check className="h-3 w-3 shrink-0" />}
              <span className="truncate">
                {probeLoading ? "Probing workload..." : probeError ? probeError : `Probe OK: ${availableSteps.length} steps, ${probeData?.env_declarations?.length || 0} parameters`}
              </span>
            </button>
            {probeExpanded && (
              <pre className="max-h-80 overflow-auto whitespace-pre-wrap border-t border-current/20 p-2 text-[10px] leading-relaxed text-zinc-400 bg-[#070707]">
                {probeLoading ? "Waiting for probe output..." : probeError ? probeError : (probeData?.human || "No human-readable probe output returned.")}
              </pre>
            )}
          </div>
        )}
      </div>

      {/* Parameters */}
      <div className="grid grid-cols-2 gap-x-6 gap-y-4">
        <div className="space-y-2">
          <Label className="block text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Run Mode</Label>
          <div className="mt-1.5 inline-flex border border-zinc-800 bg-zinc-900 font-mono text-[11px]">
            <button
              type="button"
              onClick={() => setK6Mode("duration")}
              className={`px-3 h-8 ${k6Mode === "duration" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"}`}
            >duration</button>
            <button
              type="button"
              onClick={() => setK6Mode("iterations")}
              className={`px-3 h-8 border-l border-zinc-800 ${k6Mode === "iterations" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"}`}
            >iterations</button>
          </div>
          <div className="text-[9px] text-zinc-600">
            Use iterations for one-shot SQL workloads and repeat counts for averages.
          </div>
        </div>
        {k6Mode === "duration" ? (
          <DurationSlider label="Duration" value={duration} onChange={setDuration} hint="k6 --duration" />
        ) : (
          <NumericSlider label="Iterations" value={iterations} min={1} max={1000}
            onChange={setIterations} hint="k6 --iterations" />
        )}
        <NumericSlider label="VUs" value={vus} min={1} max={1000}
          onChange={setVus} hint="Virtual users (k6 --vus), ~VUs/warehouses per warehouse" />
        <NumericSlider label="Scale Factor" value={scaleFactor} min={1} max={1000}
          onChange={setScaleFactor} hint="TPC-C warehouses / TPC-B branches" />
        <NumericSlider label="Pool Size" value={poolSize} min={10} max={1000} step={10}
          onChange={setPoolSize} hint="DB connections" />
      </div>

      <div>
        <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Execution Options</span>
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">defaultInsertMethod</Label>
            <Select value={defaultInsertMethod} onValueChange={setDefaultInsertMethod}>
              <SelectTrigger className="h-8 bg-zinc-900 border-zinc-800 text-xs font-mono">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DEFAULT_INSERT_METHODS.map((method) => (
                  <SelectItem key={method} value={method}>{method}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="text-[9px] text-zinc-600">Driver insert path for generated data.</div>
          </div>

          <div className="space-y-1.5">
            <Label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">STROPPY_ERROR_MODE</Label>
            <input
              value={stroppyErrorMode}
              onChange={(e) => setStroppyEnv((prev) => ({ ...prev, STROPPY_ERROR_MODE: e.target.value }))}
              placeholder="default"
              spellCheck={false}
              className="h-8 w-full px-2 font-mono text-xs bg-zinc-900 border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
            />
            <div className="text-[9px] text-zinc-600">Leave empty for Stroppy default; common values are silent, log, throw, fail, abort.</div>
          </div>

          <button
            type="button"
            onClick={() => setQuiet((v) => !v)}
            className={`h-10 px-3 border text-left font-mono transition-colors ${
              quiet ? "border-primary/30 bg-primary/[0.06]" : "border-zinc-800 bg-zinc-900"
            }`}
          >
            <div className={`text-[11px] ${quiet ? "text-primary" : "text-zinc-400"}`}>-q quiet</div>
            <div className="text-[9px] text-zinc-600">Enabled by default.</div>
          </button>

          <button
            type="button"
            onClick={() => setNoThresholds((v) => !v)}
            className={`h-10 px-3 border text-left font-mono transition-colors ${
              noThresholds ? "border-primary/30 bg-primary/[0.06]" : "border-zinc-800 bg-zinc-900"
            }`}
          >
            <div className={`text-[11px] ${noThresholds ? "text-primary" : "text-zinc-400"}`}>--no-thresholds</div>
            <div className="text-[9px] text-zinc-600">Off by default.</div>
          </button>

          <div className="space-y-1.5 col-span-2">
            <Label className="text-[10px] font-mono text-zinc-500 uppercase tracking-wider">LOAD_WORKERS</Label>
            <div className="flex gap-2">
              <input
                type="number"
                min={1}
                value={loadWorkersValue}
                onChange={(e) => {
                  setLoadWorkersLinked(false);
                  setEnvOverride(setStroppyEnv, "LOAD_WORKERS", e.target.value);
                }}
                disabled={loadWorkersLinked}
                className="h-8 min-w-0 flex-1 px-2 font-mono text-xs bg-zinc-900 border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600 disabled:text-zinc-500 disabled:bg-zinc-950"
              />
              <button
                type="button"
                onClick={() => {
                  if (loadWorkersLinked) {
                    setLoadWorkersLinked(false);
                    return;
                  }
                  setLoadWorkersLinked(true);
                  setEnvOverride(setStroppyEnv, "LOAD_WORKERS", String(stroppyCpus));
                }}
                className={`h-8 px-2 border flex items-center gap-1 text-[10px] font-mono ${
                  loadWorkersLinked ? "border-primary/30 text-primary bg-primary/[0.06]" : "border-zinc-800 text-zinc-500 bg-zinc-900 hover:text-zinc-300"
                }`}
                title={loadWorkersLinked ? "Click to unlock custom LOAD_WORKERS" : "Match LOAD_WORKERS to runner CPU cores"}
              >
                <Cpu className="h-3 w-3" />
                {loadWorkersLinked ? `${stroppyCpus} cores` : "use cores"}
              </button>
            </div>
            <div className="text-[9px] text-zinc-600">Linked value follows the Stroppy runner CPU count; unlock to enter a custom loader count.</div>
          </div>
        </div>
      </div>

      {/* Steps (from probe) */}
      {availableSteps.length > 0 && (
        <div>
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">
            Steps <span className="text-zinc-700">(uncheck to skip)</span>
          </span>
          <div className="flex flex-wrap gap-2">
            {availableSteps.map((step) => {
              const skipped = noSteps.includes(step);
              return (
                <button type="button" key={step} onClick={() => toggleNoStep(step)}
                  className={`px-3 py-1.5 text-xs font-mono border transition-all cursor-pointer ${
                    skipped
                      ? "border-zinc-800/60 text-zinc-600 line-through"
                      : "border-primary/30 text-primary bg-primary/[0.06]"
                  }`}
                >
                  {step}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Stroppy runner machine — only for cloud providers. Database hardware
          comes from the topology preset and is editable on the review step. */}
      {provider === "yandex" && (
        <div>
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">Stroppy Runner</span>
          <div className="grid grid-cols-3 gap-3">
            <SliderField label="CPUs" value={stroppyCpus}
              steps={cpuStepsForPlatform(platformId).filter((s) => !quotas.max_cpus_per_node || s <= quotas.max_cpus_per_node)}
              onChange={setStroppyCpus} format={(v) => `${v} vCPU`} />
            <SliderField label="Memory" value={stroppyMemory}
              steps={ramSteps(stroppyCpus, Math.min(platformLimits(platformId).maxRamMb, (quotas.max_memory_mb_per_node || Infinity)))}
              onChange={setStroppyMemory}
              format={(v) => v >= 1024 ? `${(v/1024).toFixed(v%1024?1:0)} GB` : `${v} MB`} />
            <SliderField label="Disk" value={stroppyDisk}
              steps={diskStepsForType(stroppyDiskType).filter((s) => !quotas.max_disk_gb_per_node || s <= quotas.max_disk_gb_per_node)}
              onChange={setStroppyDisk} format={(v) => `${v} GB`} />
          </div>
          <DiskTypeSelect
            value={stroppyDiskType}
            onChange={(v) => {
              setStroppyDisk(closestStep(stroppyDisk, diskStepsForType(v)));
              setStroppyDiskType(v);
            }}
            diskSizeGb={stroppyDisk}
          />
          {(() => {
            const s = suggestStroppyMachine(vus, poolSize);
            return (
              <div className="mt-2 text-[9px] font-mono text-zinc-600">
                Suggested for current VUs/pool: {s.cpus} vCPU / {(s.memory/1024).toFixed(0)} GB RAM
              </div>
            );
          })()}
        </div>
      )}

      {/* Env parameters from probe — editable */}
      {probeData?.env_declarations && probeData.env_declarations.length > 0 && (() => {
        // Filter out env vars already covered by dedicated UI controls.
        const covered = new Set(["POOL_SIZE", "SCALE_FACTOR", "WAREHOUSES", "LOAD_WORKERS", "STROPPY_STEPS", "STROPPY_NO_STEPS", "STROPPY_ERROR_MODE"]);
        const envDecls = probeData.env_declarations.filter(
          (e) => !e.names.every((n) => covered.has(n))
        );
        if (envDecls.length === 0) return null;
        return (
          <div>
            <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider mb-2 block">
              Script Parameters
            </span>
            <div className="space-y-2">
              {envDecls.map((e, i) => {
                const name = e.names[0];
                const value = stroppyEnv[name] ?? "";
                const defaultValue = e.default || "";
                const isBool = /^(true|false)$/i.test(defaultValue);
                const isNumber = defaultValue !== "" && /^-?\d+(\.\d+)?$/.test(defaultValue);
                return (
                  <div key={`${name}-${i}`} className="grid grid-cols-[160px_minmax(0,1fr)_auto] gap-3 items-center">
                    <div className="min-w-0">
                      <label className="text-[10px] font-mono text-zinc-400 truncate block" title={e.names.join(", ")}>
                        {name}
                      </label>
                      {e.names.length > 1 && (
                        <div className="text-[9px] font-mono text-zinc-700 truncate">{e.names.slice(1).join(", ")}</div>
                      )}
                    </div>
                    {isBool ? (
                      <button
                        type="button"
                        onClick={() => setEnvOverride(setStroppyEnv, name, (value || defaultValue).toLowerCase() === "true" ? "false" : "true")}
                        className={`h-7 w-14 border text-[11px] font-mono ${
                          (value || defaultValue).toLowerCase() === "true"
                            ? "border-primary/40 bg-primary/[0.08] text-primary"
                            : "border-zinc-800 text-zinc-500 bg-zinc-900"
                        }`}
                      >
                        {(value || defaultValue).toLowerCase() === "true" ? "true" : "false"}
                      </button>
                    ) : (
                      <input
                        type={isNumber ? "number" : "text"}
                        value={value}
                        placeholder={defaultValue || "<unset>"}
                        onChange={(ev) => setEnvOverride(setStroppyEnv, name, ev.target.value)}
                        className="h-7 min-w-0 bg-zinc-900 border border-zinc-800 px-2 text-[11px] font-mono text-zinc-300 outline-none focus:border-zinc-600"
                      />
                    )}
                    <button
                      type="button"
                      onClick={() => setEnvOverride(setStroppyEnv, name, "")}
                      className="text-[10px] font-mono text-zinc-600 hover:text-zinc-300 disabled:opacity-30"
                      disabled={value === ""}
                    >
                      default
                    </button>
                    <div className="col-start-2 col-span-2 text-[9px] text-zinc-600 truncate -mt-2" title={e.description}>
                      {e.description || "No description"}{defaultValue && <span className="text-zinc-700"> · default {defaultValue}</span>}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })()}

      <div>
        <div className="flex items-center justify-between mb-2">
          <span className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">
            stroppy-config.json
          </span>
          <div className="flex items-center gap-2">
            {stroppyConfigUserEdited && <span className="text-[10px] font-mono text-amber-500">edited</span>}
            <button
              type="button"
              onClick={resetStroppyConfigDraft}
              disabled={!stroppyConfigUserEdited || stroppyConfigPristine === null}
              className="text-[10px] font-mono text-zinc-600 hover:text-zinc-300 disabled:opacity-30 flex items-center gap-1"
            >
              <RotateCcw className="h-3 w-3" />
              reset
            </button>
          </div>
        </div>
        <JsonEditor
          value={stroppyConfigDraft ?? ""}
          onChange={updateStroppyConfigDraft}
          height="20rem"
        />
      </div>
    </div>
  );
}

export default WorkloadForm;
