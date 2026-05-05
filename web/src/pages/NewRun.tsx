import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { startRun, validateRun, dryRun, listPresets, listPackages, probeScript, getStroppyVersions, getStroppyCommits, getSettings, previewStroppyConfig, type StroppyCommit } from "@/api/client";
import {
  ALL_DB_KINDS,
  KIND_PROTOCOLS,
  SCRIPT_COMPAT,
  type RunConfig,
  type DatabaseKind,
  type Protocol,
  type Provider,
  type Preset,
  type Package,
  type ProbeResponse,
  type TenantQuotas,
  type WorkloadFile,
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
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
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
  ChevronDown,
  Loader2,
  Pencil,
  Upload,
  X,
  RotateCcw,
} from "lucide-react";

import { DB_COLORS } from "@/lib/db-colors";
import { NumericSlider, DurationSlider, SliderField, CPU_STEPS, ramSteps, DiskTypeSelect, PlatformSelect, cpuStepsForPlatform, platformLimits, closestStep, diskStepsForType } from "@/components/ui/sliders";

// --- Constants ---

const DB_KINDS = ALL_DB_KINDS;
const PROVIDERS: Provider[] = ["docker", "yandex"];
type K6Mode = "duration" | "iterations";
const DEFAULT_INSERT_METHODS = ["native", "plain_bulk", "plain_query"] as const;
// SCRIPT_META is a label/description lookup for known stroppy scripts.
// SCRIPT_COMPAT (in api/types.ts) is the source of truth for which scripts
// run on which (kind, protocol); this map only adds presentation. Anything
// not listed here renders with its raw ID and an empty description — useful
// for engine-specific variants we add later without touching this file.
const SCRIPT_META: Record<string, { label: string; desc: string }> = {
  "tpcc/procs":          { label: "TPC-C Procs", desc: "Stored procedures" },
  "tpcc/tx":             { label: "TPC-C Tx",    desc: "Raw transactions" },
  "tpcb/procs":          { label: "TPC-B Procs", desc: "Stored procedures" },
  "tpcb/tx":             { label: "TPC-B Tx",    desc: "Raw transactions" },
  "tpcc/tx-ydb-pgwire":  { label: "TPC-C Tx (YDB pgwire)", desc: "Subset that fits YDB's pg-wire feature ceiling" },
  "tpcb/tx-ydb-pgwire":  { label: "TPC-B Tx (YDB pgwire)", desc: "Subset that fits YDB's pg-wire feature ceiling" },
};

const DB_VERSIONS: Record<DatabaseKind, string[]> = {
  postgres: ["17", "16", "15"],
  mysql: ["8.4", "8.0"],
  mariadb: ["11.4", "10.11"], // both LTS releases — 11.4 is current, 10.11 supported until 2028-02
  picodata: ["25.3"],
  ydb: ["25.2", "25.1", "24.4", "24.3"],
  // Managed YDB version is fixed by Yandex Cloud; surface "managed" so the
  // wizard's required version field has something coherent to render.
  "ydb-managed": ["managed"],
  cockroach: ["24.2", "24.1", "23.2"],
};

const DB_META: Record<DatabaseKind, { icon: typeof Database; label: string }> = {
  postgres:  { icon: Database, label: "PostgreSQL" },
  mysql:     { icon: Server,   label: "MySQL" },
  mariadb:   { icon: Server,   label: "MariaDB" },
  picodata:  { icon: Cpu,      label: "Picodata" },
  ydb:       { icon: Database, label: "YDB" },
  "ydb-managed": { icon: Cloud, label: "YDB Managed" },
  cockroach: { icon: Database, label: "CockroachDB" },
};

const PROVIDER_META: Record<Provider, { icon: typeof Cloud; label: string }> = {
  docker: { icon: Container, label: "Docker" },
  yandex: { icon: Cloud,     label: "Yandex Cloud" },
};

function redactRunConfigForDisplay(cfg: RunConfig): RunConfig {
  if (!cfg.stroppy.files?.length) return cfg;
  return {
    ...cfg,
    stroppy: {
      ...cfg.stroppy,
      files: cfg.stroppy.files.map((f) => ({
        ...f,
        content: `<${f.content.length} chars redacted>`,
      })),
    },
  };
}

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

  // Check for rerun config from sessionStorage (set by RunDetail's Rerun button).
  const rerunConfig = useMemo<RunConfig | null>(() => {
    try {
      const raw = sessionStorage.getItem("rerun_config");
      if (raw) {
        sessionStorage.removeItem("rerun_config");
        return JSON.parse(raw) as RunConfig;
      }
    } catch { /* ignore */ }
    return null;
  }, []);

  const rc = rerunConfig; // shorthand
  const rcS = rc?.stroppy;
  const rcSM = rc?.stroppy?.machine;

  const [step, setStep] = useState(0);

  const [allPresets, setAllPresets] = useState<Preset[]>([]);
  const [kind, setKind] = useState<DatabaseKind>(
    rc?.database?.kind as DatabaseKind || (searchParams.get("kind") as DatabaseKind) || "postgres"
  );
  const [selectedPresetId, setSelectedPresetId] = useState(rc?.preset_id || searchParams.get("preset_id") || "");
  const [provider, setProvider] = useState<Provider>(rc?.provider || "docker");
  const [platformId, setPlatformId] = useState(rc?.platform_id || "standard-v3");
  const [version, setVersion] = useState(rc?.database?.version || DB_VERSIONS[kind][0]);
  // Protocol axis: how stroppy talks to the picked engine. Defaults to the
  // first protocol KIND_PROTOCOLS lists for the kind (preserves behaviour
  // for runs that never set this field). Toggling resets the script if the
  // current one isn't supported by the new (kind, protocol) combo.
  const [protocol, setProtocol] = useState<Protocol>(
    (rcS?.protocol as Protocol) || KIND_PROTOCOLS[(rc?.database?.kind as DatabaseKind) || "postgres"][0],
  );
  const [script, setScript] = useState(rcS?.script || rcS?.workload || "tpcc/procs");
  const [sql, setSql] = useState(rcS?.sql || "");
  const [stroppyEnv, setStroppyEnv] = useState<Record<string, string>>({ STROPPY_ERROR_MODE: "", ...(rcS?.env || {}) });
  const [workloadFiles, setWorkloadFiles] = useState<WorkloadFile[]>(rcS?.files || []);
  const [duration, setDuration] = useState(rcS?.duration || "5m");
  const [k6Mode, setK6Mode] = useState<K6Mode>((rcS?.k6_mode as K6Mode) || (rcS?.iterations ? "iterations" : "duration"));
  const [iterations, setIterations] = useState(rcS?.iterations || 1);
  const [quiet, setQuiet] = useState(rcS?.quiet ?? true);
  const [noThresholds, setNoThresholds] = useState(rcS?.no_thresholds || false);
  const [defaultInsertMethod, setDefaultInsertMethod] = useState(rcS?.default_insert_method || "native");
  // TPC-C-tuned defaults: scale 500 warehouses, 300 VUs, 200-conn pool. The
  // workload is OLTP-heavy enough that the smaller previous defaults
  // (10/100/1) produced runs that finished before any meaningful state was
  // built up. Overridden by rcS.* on rerun.
  const [vus, setVus] = useState(rcS?.vus || rcS?.vus_scale || 300);
  const [poolSize, setPoolSize] = useState(rcS?.pool_size || 200);
  const [scaleFactor, setScaleFactor] = useState(rcS?.scale_factor || 500);
  const [stroppyVersion, setStroppyVersion] = useState(rcS?.version || "");
  const [stroppyVersions, setStroppyVersions] = useState<string[]>(rcS?.version ? [rcS.version] : []);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const versionsLoaded = useRef(false);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [workloadProbeOK, setWorkloadProbeOK] = useState(false);
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>(rcS?.steps || []);
  const [noSteps, setNoSteps] = useState<string[]>(rcS?.no_steps || []);
  // Stroppy runner sizing — defaults to 32 vCPU / 64 GB / 100 GB, sized for
  // the new TPC-C defaults (300 VUs, pool 200). Overridden by rcSM.* on
  // rerun. The suggestStroppyMachine() helper is still used as a live hint
  // in StepStroppy when the user adjusts VUs/pool.
  const [stroppyCpus, setStroppyCpus] = useState(rcSM?.cpus || 32);
  const [stroppyMemory, setStroppyMemory] = useState(rcSM?.memory_mb || 65536);
  const [stroppyDisk, setStroppyDisk] = useState(rcSM?.disk_gb || 100);
  const [stroppyDiskType, setStroppyDiskType] = useState(rcSM?.disk_type || "network-ssd");
  const [packageId, setPackageId] = useState(rc?.package_id || "");
  const [availablePackages, setAvailablePackages] = useState<Package[]>([]);
  const [quotas, setQuotas] = useState<TenantQuotas>({});

  const allowedKinds = useMemo(() =>
    quotas.allowed_db_kinds?.length ? DB_KINDS.filter((k) => quotas.allowed_db_kinds!.includes(k)) : DB_KINDS,
    [quotas]);
  const allowedProviders = useMemo(() =>
    quotas.allowed_providers?.length ? PROVIDERS.filter((p) => quotas.allowed_providers!.includes(p)) : PROVIDERS,
    [quotas]);

  const [submitting, setSubmitting] = useState(false);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const [dryRunResult, setDryRunResult] = useState<any>(null);
  const [dryRunLoading, setDryRunLoading] = useState(false);
  const [resolvedConfig, setResolvedConfig] = useState<RunConfig | null>(null);
  const [dbConfigDraft, setDbConfigDraft] = useState<string | null>(null);
  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(rcS?.config_override_json || null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(null);
  const [stroppyConfigUserEdited, setStroppyConfigUserEdited] = useState(!!rcS?.config_override_json);
  const [renderedConfigsPristine, setRenderedConfigsPristine] = useState<Record<string, string>>({});
  const [renderedConfigDrafts, setRenderedConfigDrafts] = useState<Record<string, string>>({});
  const [validationResult, setValidationResult] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => { listPresets().then(setAllPresets).catch(() => {}); }, []);
  useEffect(() => { getSettings().then((s) => setQuotas(s.quotas || {})).catch(() => {}); }, []);
  useEffect(() => {
    versionsLoaded.current = true;
    setVersionsLoading(true);
    getStroppyVersions()
      .then((versions) => {
        if (versions.length === 0) return;
        setStroppyVersions((prev) => {
          const pinned = rcS?.version && !versions.includes(rcS.version) ? [rcS.version] : [];
          return [...pinned, ...versions];
        });
        if (!rcS?.version) setStroppyVersion(versions[0]);
      })
      .catch(() => {})
      .finally(() => setVersionsLoading(false));
    // rcS is an initial rerun snapshot; this effect should run once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  useEffect(() => {
    // Reconcile dependent state when kind / allPresets change. Each branch
    // is conditional — unconditional setters here run again every time
    // allPresets resolves async (~100 ms after mount) and would clobber
    // values the user just edited (or restored from a rerun config). That's
    // the bug behind "I edit version and the old value pops back".
    const matching = allPresets.filter((p) => p.db_kind === kind);
    if (matching.length > 0 && !matching.find((p) => p.id === selectedPresetId)) {
      setSelectedPresetId(matching[0].id);
    }
    if (!DB_VERSIONS[kind].includes(version)) {
      setVersion(DB_VERSIONS[kind][0]);
    }
    // Protocol must be one the new kind speaks — switching kinds resets
    // anything stale to the kind's default (first entry).
    if (!KIND_PROTOCOLS[kind].includes(protocol)) {
      setProtocol(KIND_PROTOCOLS[kind][0]);
    }
  }, [kind, allPresets]);
  // Script compatibility now keys on (kind, protocol). Reset to the first
  // valid entry whenever the current pick falls off the matrix — both kind
  // changes and protocol toggles can break compatibility.
  useEffect(() => {
    const compat = SCRIPT_COMPAT[`${kind}:${protocol}`] || [];
    const known = Object.values(SCRIPT_COMPAT).some((scripts) => scripts.includes(script));
    if (known && compat.length > 0 && !compat.includes(script)) {
      setScript(compat[0]);
    }
  }, [kind, protocol]);
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
        version: stroppyVersion,
        protocol,
        script,
        ...(sql.trim() ? { sql: sql.trim() } : {}),
        duration,
        k6_mode: k6Mode,
        ...(k6Mode === "iterations" ? { iterations } : {}),
        quiet,
        no_thresholds: noThresholds,
        vus,
        pool_size: poolSize,
        scale_factor: scaleFactor,
        default_insert_method: defaultInsertMethod,
        ...(Object.keys(stroppyEnv).length > 0 ? { env: stroppyEnv } : {}),
        ...(workloadFiles.length > 0 ? { files: workloadFiles } : {}),
        ...(selectedSteps.length > 0 ? { steps: selectedSteps } : {}),
        ...(noSteps.length > 0 ? { no_steps: noSteps } : {}),
        ...(provider === "yandex" ? { machine: { role: "stroppy" as const, count: 1, cpus: stroppyCpus, memory_mb: stroppyMemory, disk_gb: stroppyDisk, disk_type: stroppyDiskType } } : {}),
      },
    };
    if (selectedPresetId) cfg.preset_id = selectedPresetId;
    if (packageId) cfg.package_id = packageId;
    if (provider === "yandex") {
      cfg.platform_id = platformId;
    }
    return cfg;
  }, [kind, protocol, selectedPresetId, provider, platformId, version, script, sql, duration, k6Mode, iterations, quiet, noThresholds, vus, poolSize, scaleFactor, defaultInsertMethod, packageId, stroppyEnv, workloadFiles, selectedSteps, noSteps, stroppyCpus, stroppyMemory, stroppyDisk, stroppyDiskType, stroppyVersion]);

  const configJSON = useMemo(() => JSON.stringify(redactRunConfigForDisplay(config), null, 2), [config]);

  useEffect(() => {
    if (step !== 2 || stroppyConfigUserEdited) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      previewStroppyConfig(config)
        .then((resp) => {
          if (cancelled) return;
          setStroppyConfigDraft(resp.stroppy_config);
          setStroppyConfigPristine(resp.stroppy_config);
        })
        .catch(() => {
          if (!cancelled && stroppyConfigDraft === null) {
            setStroppyConfigDraft("");
            setStroppyConfigPristine("");
          }
        });
    }, 250);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [step, config, stroppyConfigUserEdited]);

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
        if (!cancelled) {
          setDryRunResult(dr);
          const drObj = dr as Record<string, unknown>;
          if (drObj.resolved_config) {
            const rc = drObj.resolved_config as RunConfig;
            setResolvedConfig(rc);
            setDbConfigDraft(JSON.stringify(rc.database, null, 2));
          }
          if (typeof drObj.stroppy_config === "string" && !stroppyConfigUserEdited) {
            setStroppyConfigDraft(drObj.stroppy_config);
            setStroppyConfigPristine(drObj.stroppy_config);
          }
          // Seed rendered config files. Pristine map keeps a baseline so we
          // only ship overrides for entries the user actually edited.
          if (drObj.rendered_configs && typeof drObj.rendered_configs === "object") {
            const next = drObj.rendered_configs as Record<string, string>;
            setRenderedConfigsPristine(next);
            setRenderedConfigDrafts({ ...next });
          }
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : "Dry run failed");
      }
      if (!cancelled) setDryRunLoading(false);
    })();
    return () => { cancelled = true; };
  }, [step, config, stroppyConfigUserEdited]);

  const handleSubmit = useCallback(async () => {
    if (config.stroppy.k6_mode !== "iterations" && !config.stroppy.duration.trim()) { setError("Duration is required"); return; }
    if (config.stroppy.k6_mode === "iterations" && (config.stroppy.iterations || 0) < 1) { setError("Iterations must be at least 1"); return; }
    setSubmitting(true); setError(null);
    try {
      // The review-step textarea is the source of truth for the database config.
      const launchConfig = { ...config };
      if (dbConfigDraft && resolvedConfig) {
        try {
          launchConfig.database = JSON.parse(dbConfigDraft);
        } catch (e) {
          setError(`Invalid JSON in database config: ${e instanceof Error ? e.message : String(e)}`);
          setSubmitting(false);
          return;
        }
      }
      // Ship rendered_config_overrides only for files the user actually edited
      // away from the dry-run baseline. Avoids round-tripping pristine bodies
      // (which would still match server output but inflate the request).
      const renderedOverrides: Record<string, string> = {};
      for (const [k, v] of Object.entries(renderedConfigDrafts)) {
        if (renderedConfigsPristine[k] !== v) renderedOverrides[k] = v;
      }
      if (Object.keys(renderedOverrides).length > 0) {
        launchConfig.database = {
          ...launchConfig.database,
          rendered_config_overrides: { ...(launchConfig.database.rendered_config_overrides || {}), ...renderedOverrides },
        };
      }
      // Only send override when the user actually edited the preview — the server
      // builds the preview with placeholder db host/port (resolved at run time), so
      // sending it back verbatim would ship those placeholders to the stroppy binary.
      if (stroppyConfigUserEdited) {
        try {
          JSON.parse(stroppyConfigDraft || ""); // validate
          launchConfig.stroppy = { ...launchConfig.stroppy, config_override_json: stroppyConfigDraft || "" };
        } catch (e) {
          setError(`Invalid JSON in stroppy config: ${e instanceof Error ? e.message : String(e)}`);
          setSubmitting(false);
          return;
        }
      }
      const result = await startRun(launchConfig);
      navigate(`/runs/${result.run_id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start run");
      setSubmitting(false);
    }
  }, [config, navigate, dbConfigDraft, resolvedConfig, stroppyConfigDraft, stroppyConfigUserEdited, renderedConfigDrafts, renderedConfigsPristine]);

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
            <StepInfra provider={provider} setProvider={setProvider} platformId={platformId} setPlatformId={setPlatformId} providers={allowedProviders} />
          )}
          {step === 1 && (
            <StepDatabase
              kind={kind} setKind={setKind}
              protocol={protocol} setProtocol={setProtocol}
              version={version} setVersion={setVersion}
              packageId={packageId} setPackageId={setPackageId}
              availablePackages={availablePackages}
              presetsForKind={presetsForKind}
              selectedPresetId={selectedPresetId} setSelectedPresetId={setSelectedPresetId}
              dbMeta={dbMeta} dbColor={dbColor}
              allowedKinds={allowedKinds}
            />
          )}
          {step === 2 && (
            <StepStroppy
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
              vus={vus} setVus={setVus}
              poolSize={poolSize} setPoolSize={setPoolSize}
              dbKind={kind}
              protocol={protocol}
              provider={provider}
              platformId={platformId}
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
          )}
          {step === 3 && (
            <StepReview
              dryRunResult={dryRunResult}
              dryRunLoading={dryRunLoading}
              validationResult={validationResult}
              error={error}
              submitting={submitting}
              onSubmit={handleSubmit}
              onEdit={(group, key, value) => {
                const n = parseInt(value);
                if (group === "benchmark") {
                  if (key === "VUs" && !isNaN(n)) setVus(n);
                  else if (key === "duration") setDuration(value);
                  else if (key === "pool" && !isNaN(n)) setPoolSize(n);
                  else if (key === "scale" && !isNaN(n)) setScaleFactor(n);
                } else if (group === "infrastructure") {
                  if (key === "platform") setPlatformId(value);
                }
              }}
              dbConfigDraft={dbConfigDraft}
              setDbConfigDraft={setDbConfigDraft}
              stroppyConfigDraft={stroppyConfigDraft}
              setStroppyConfigDraft={(v) => { setStroppyConfigDraft(v); setStroppyConfigUserEdited(v !== stroppyConfigPristine); }}
              script={script}
              scaleFactor={scaleFactor}
              stroppyVersion={stroppyVersion}
              setStroppyVersion={setStroppyVersion}
              renderedConfigDrafts={renderedConfigDrafts}
              setRenderedConfigDrafts={setRenderedConfigDrafts}
              renderedConfigsPristine={renderedConfigsPristine}
            />
          )}

          {/* Navigation */}
          {step < 3 && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-zinc-800/50">
              <Button variant="outline" size="sm" onClick={() => setStep(Math.max(0, step - 1))} disabled={step === 0}>
                <ChevronLeft className="h-3 w-3" /> Back
              </Button>
              <Button size="sm" onClick={() => setStep(step + 1)} className="gap-1.5" disabled={step === 2 && !workloadProbeOK}>
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
              {provider === "yandex" && <SummaryRow label="Platform" value={platformLimits(platformId).label} />}
              <SummaryRow icon={DbIcon} label="Database" value={`${dbMeta.label} ${version}`} color={dbColor.text} />
              {selectedPreset && (
                <SummaryRow label="Topology" value={selectedPreset.name} />
              )}
              <SummaryRow label="Script" value={script} />
              {sql && <SummaryRow label="SQL" value={sql} />}
              <SummaryRow label={k6Mode === "iterations" ? "Iterations" : "Duration"} value={k6Mode === "iterations" ? String(iterations) : duration} />
              <SummaryRow label="VUs" value={String(vus)} />
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

function StepInfra({ provider, setProvider, platformId, setPlatformId, providers }: {
  provider: Provider;
  setProvider: (p: Provider) => void;
  platformId: string;
  setPlatformId: (p: string) => void;
  providers: Provider[];
}) {
  return (
    <div className="space-y-5 max-w-lg">
      <div>
        <h2 className="text-sm font-semibold mb-1">Where to run?</h2>
        <p className="text-xs text-zinc-500">Choose the infrastructure provider for provisioning machines.</p>
      </div>
      <div className="grid grid-cols-2 gap-3">
        {providers.map((p) => {
          const pm = PROVIDER_META[p];
          const PIcon = pm.icon;
          const active = provider === p;
          return (
            <button type="button" key={p} onClick={() => setProvider(p)}
              className={`flex items-center gap-3 border p-4 transition-all cursor-pointer ${
                active
                  ? "border-primary/40 text-primary bg-primary/[0.06]"
                  : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
              }`}
            >
              <PIcon className={`h-5 w-5 shrink-0 ${active ? "text-primary" : "text-zinc-600"}`} />
              <div className="text-left">
                <div className={`text-xs font-mono font-medium ${active ? "text-primary" : "text-zinc-400"}`}>{pm.label}</div>
                <div className="text-[10px] text-zinc-600">
                  {p === "docker" ? "Local containers" : "Yandex Cloud VMs"}
                </div>
              </div>
            </button>
          );
        })}
      </div>
      {provider === "yandex" && (
        <PlatformSelect value={platformId} onChange={setPlatformId} />
      )}
    </div>
  );
}

// ─── Step 2: Database ────────────────────────────────────────────

function StepDatabase({
  kind, setKind,
  protocol, setProtocol,
  version, setVersion,
  packageId, setPackageId,
  availablePackages,
  presetsForKind,
  selectedPresetId, setSelectedPresetId,
  dbMeta, dbColor,
  allowedKinds,
}: {
  kind: DatabaseKind; setKind: (k: DatabaseKind) => void;
  protocol: Protocol; setProtocol: (p: Protocol) => void;
  allowedKinds: DatabaseKind[];
  version: string; setVersion: (v: string) => void;
  packageId: string; setPackageId: (v: string) => void;
  availablePackages: Package[];
  presetsForKind: Preset[];
  selectedPresetId: string; setSelectedPresetId: (v: string) => void;
  dbMeta: { icon: typeof Database; label: string };
  dbColor: { hex: string; text: string; accent: string };
}) {
  const protocolsForKind = KIND_PROTOCOLS[kind];
  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-sm font-semibold mb-1">Database</h2>
        <p className="text-xs text-zinc-500">Choose the database engine, version, and topology preset.</p>
      </div>

      {/* DB Kind */}
      <div className="grid grid-cols-3 gap-2">
        {allowedKinds.map((k) => {
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

      {/* Protocol toggle — hidden for engines that speak only one protocol.
          Selecting a different protocol re-filters the script picker on
          step 2, since SCRIPT_COMPAT keys on (kind, protocol). */}
      {protocolsForKind.length > 1 && (
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Protocol</Label>
          <div className="inline-flex border border-zinc-800 overflow-hidden text-[11px] font-mono">
            {protocolsForKind.map((p) => {
              const active = protocol === p;
              return (
                <button
                  key={p}
                  type="button"
                  onClick={() => setProtocol(p)}
                  className={`px-3 py-1 transition-colors cursor-pointer ${
                    active ? `${dbColor.text} ${dbColor.accent}` : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-900"
                  }`}
                >
                  {p}
                </button>
              );
            })}
          </div>
          <p className="text-[10px] font-mono text-zinc-600">
            Wire format stroppy uses to talk to {dbMeta.label}. Different protocols expose different SQL feature ceilings — the script list on step 2 narrows accordingly.
          </p>
        </div>
      )}

      {/* Version + Package */}
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-[11px] font-mono text-zinc-500 uppercase tracking-wider">Version</Label>
          <Select
            value={DB_VERSIONS[kind].includes(version) ? version : "__custom__"}
            onValueChange={(v) => { if (v !== "__custom__") setVersion(v); }}
          >
            <SelectTrigger className="h-8 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              {DB_VERSIONS[kind].map((v) => (
                <SelectItem key={v} value={v}>{v}</SelectItem>
              ))}
              <SelectItem value="__custom__">Custom...</SelectItem>
            </SelectContent>
          </Select>
          {!DB_VERSIONS[kind].includes(version) && (
            <input
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="Enter version"
              className="h-7 w-full px-2 font-mono text-xs bg-transparent border border-zinc-800 text-zinc-300 outline-none focus:border-zinc-600"
            />
          )}
        </div>
        {kind !== "ydb" && (
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
        )}
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

// Suggest minimum DB-node disk (GB) for a script × scale combo.
// TPC-C: ~100 MB per warehouse (data + indexes). TPC-B: ~15 MB per scale unit. 2x headroom for WAL/bloat.
function suggestDiskGb(script: string, scaleFactor: number): { diskGb: number; reason: string } {
  const isTpcc = script.startsWith("tpcc");
  const dataPerUnit = isTpcc ? 100 : 15; // MB
  const rawMb = dataPerUnit * Math.max(1, scaleFactor);
  const withHeadroom = rawMb * 2;
  const diskGb = Math.max(25, Math.ceil(withHeadroom / 1024));
  const reason = `${isTpcc ? "TPC-C" : "TPC-B"} × ${scaleFactor} ≈ ${Math.ceil(rawMb / 1024)} GB data → ${diskGb} GB with headroom`;
  return { diskGb, reason };
}

// Extract the DB-node data-volume disk size from a DatabaseConfig JSON string.
// Returns null if the JSON is invalid or the topology shape doesn't match the kind.
function extractDbDiskGb(dbCfgJSON: string): number | null {
  try {
    const c = JSON.parse(dbCfgJSON) as { kind?: string; postgres?: { master?: { disk_gb?: number } }; mysql?: { primary?: { disk_gb?: number } }; picodata?: { instances?: Array<{ disk_gb?: number }> }; ydb?: { storage?: { disk_gb?: number } } };
    switch (c.kind) {
      case "postgres": return c.postgres?.master?.disk_gb ?? null;
      case "mysql": return c.mysql?.primary?.disk_gb ?? null;
      case "picodata": {
        const insts = c.picodata?.instances;
        if (!insts || insts.length === 0) return null;
        const sizes = insts.map((i) => i.disk_gb ?? 0).filter((n) => n > 0);
        return sizes.length ? Math.min(...sizes) : null;
      }
      case "ydb": return c.ydb?.storage?.disk_gb ?? null;
      default: return null;
    }
  } catch {
    return null;
  }
}

// Suggest optimal stroppy machine based on VUs and pool size.
// StroppyVersionCombo is a free-text input + chevron-button popover.
// HTML's <datalist> filters its options against the input value, which makes
// the dropdown disappear the moment the field has any prefilled text — wrong
// UX for "pick a release or type your own". This always shows everything.
function StroppyVersionCombo({
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

function suggestStroppyMachine(vus: number, poolSize: number): { cpus: number; memory: number; disk: number; reason: string } {
  // Rule of thumb: 1 vCPU per ~20 VUs, min 2. Snap to valid platform steps.
  const rawCpus = Math.max(2, Math.min(96, Math.ceil(vus / 20) * 2));
  const cpus = closestStep(rawCpus, CPU_STEPS);
  const memBase = cpus * 2048;
  const memPool = Math.ceil(poolSize / 100) * 512;
  const memory = Math.max(2048, memBase + memPool);
  const disk = 25;
  const reason = `${vus} VUs → ${cpus} vCPU, pool ${poolSize} → ${(memory/1024).toFixed(0)} GB RAM`;
  return { cpus, memory, disk, reason };
}

function probeDriverType(protocol: Protocol, dbKind: DatabaseKind): string {
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

function safeUploadedFileName(name: string): string {
  const base = name.split(/[\\/]/).pop() || "workload.sql";
  const safe = base.replace(/[^A-Za-z0-9._-]/g, "_");
  return safe.endsWith(".sql") ? safe : `${safe}.sql`;
}

function setEnvOverride(
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

function StepStroppy({
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
  selectedSteps, setSelectedSteps,
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
  setStroppyConfigPristine,
  stroppyConfigUserEdited,
  setStroppyConfigUserEdited,
}: {
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
  dbKind: DatabaseKind;
  protocol: Protocol;
  provider: Provider;
  platformId: string;
  quotas: TenantQuotas;
  availableSteps: string[]; setAvailableSteps: (v: string[]) => void;
  selectedSteps: string[]; setSelectedSteps: (v: string[]) => void;
  noSteps: string[]; setNoSteps: (v: string[]) => void;
  probeData: ProbeResponse | null; setProbeData: (v: ProbeResponse | null) => void;
  setWorkloadProbeOK: (v: boolean) => void;
  stroppyCpus: number; setStroppyCpus: (v: number) => void;
  stroppyMemory: number; setStroppyMemory: (v: number) => void;
  stroppyDisk: number; setStroppyDisk: (v: number) => void;
  stroppyDiskType: string; setStroppyDiskType: (v: string) => void;
  stroppyVersion: string; setStroppyVersion: (v: string) => void;
  stroppyVersions: string[]; setStroppyVersions: (v: string[]) => void;
  versionsLoading: boolean; setVersionsLoading: (v: boolean) => void;
  versionsLoaded: React.MutableRefObject<boolean>;
  stroppyConfigDraft: string | null;
  setStroppyConfigDraft: (v: string) => void;
  stroppyConfigPristine: string | null;
  setStroppyConfigPristine: (v: string) => void;
  stroppyConfigUserEdited: boolean;
  setStroppyConfigUserEdited: (v: boolean) => void;
}) {
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
    setStroppyConfigUserEdited(value !== (stroppyConfigPristine || ""));
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
            // Free-text input + chevron button that opens a popover with the
            // full version list. Built from scratch instead of a <datalist>
            // because <datalist> filters its options against the current
            // input value — a prefilled release would hide every non-matching release
            // that doesn't match. The popover always shows everything.
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
                  // Accept full or short SHA; trim to 7 chars (matches `nightly-<short>` tag).
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
              <ChevronDown className={`h-3 w-3 ml-auto shrink-0 transition-transform ${probeExpanded ? "rotate-180" : ""}`} />
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
            >
              duration
            </button>
            <button
              type="button"
              onClick={() => setK6Mode("iterations")}
              className={`px-3 h-8 border-l border-zinc-800 ${k6Mode === "iterations" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"}`}
            >
              iterations
            </button>
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

      {/* Stroppy runner machine — only for cloud providers.
          Database hardware comes from the topology preset and is editable on
          the Review step textarea, not here. */}
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
          <SliderField label="Disk" value={stroppyDisk} steps={diskStepsForType(stroppyDiskType).filter((s) => !quotas.max_disk_gb_per_node || s <= quotas.max_disk_gb_per_node)}
            onChange={setStroppyDisk} format={(v) => `${v} GB`} />
        </div>
        <DiskTypeSelect
          value={stroppyDiskType}
          onChange={setStroppyDiskType}
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
        <textarea
          value={stroppyConfigDraft ?? ""}
          onChange={(e) => updateStroppyConfigDraft(e.target.value)}
          spellCheck={false}
          className="w-full h-80 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
        />
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

// Keys that can be edited in the review step — mapped back to state via onEdit.
// Database rows are display-only here: the textarea above them is the source of truth.
const EDITABLE_KEYS: Record<string, Set<string>> = {
  benchmark: new Set(["VUs", "duration", "pool", "scale"]),
  infrastructure: new Set(["platform"]),
};

function EditableCfgRow({ k, v, groupKey, onEdit }: {
  k: string; v: string; groupKey: string;
  onEdit?: (group: string, key: string, value: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(v);
  const editable = onEdit && EDITABLE_KEYS[groupKey]?.has(k);

  const commit = () => {
    setEditing(false);
    if (draft !== v && onEdit) onEdit(groupKey, k, draft);
  };

  if (editing) {
    return (
      <div className="flex gap-2 text-[10px] font-mono leading-relaxed">
        <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
        <input
          autoFocus
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => { if (e.key === "Enter") commit(); if (e.key === "Escape") { setDraft(v); setEditing(false); } }}
          className="flex-1 bg-zinc-800 text-zinc-200 px-1 py-0 border border-zinc-600 outline-none text-[10px] font-mono"
        />
      </div>
    );
  }

  return (
    <div className="flex gap-2 text-[10px] font-mono leading-relaxed group/row">
      <span className="text-zinc-600 shrink-0 w-20 text-right">{k}</span>
      <span className="text-zinc-400 truncate flex-1" title={v}>{v}</span>
      {editable && (
        <button type="button" onClick={() => { setDraft(v); setEditing(true); }}
          className="opacity-0 group-hover/row:opacity-100 text-zinc-600 hover:text-zinc-400 transition-opacity shrink-0">
          <Pencil className="w-2.5 h-2.5" />
        </button>
      )}
    </div>
  );
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
  onEdit,
  dbConfigDraft,
  setDbConfigDraft,
  stroppyConfigDraft,
  setStroppyConfigDraft,
  script,
  scaleFactor,
  stroppyVersion,
  setStroppyVersion,
  renderedConfigDrafts,
  setRenderedConfigDrafts,
  renderedConfigsPristine,
}: {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dryRunResult: any;
  dryRunLoading: boolean;
  validationResult: { ok: boolean; message: string } | null;
  error: string | null;
  submitting: boolean;
  onSubmit: () => void;
  onEdit?: (group: string, key: string, value: string) => void;
  dbConfigDraft: string | null;
  setDbConfigDraft: (v: string) => void;
  stroppyConfigDraft: string | null;
  setStroppyConfigDraft: (v: string) => void;
  script: string;
  scaleFactor: number;
  stroppyVersion: string;
  setStroppyVersion: (v: string) => void;
  renderedConfigDrafts: Record<string, string>;
  setRenderedConfigDrafts: React.Dispatch<React.SetStateAction<Record<string, string>>>;
  renderedConfigsPristine: Record<string, string>;
}) {
  const canLaunch = validationResult?.ok && !dryRunLoading && !error;
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggle = (label: string) => setExpanded((prev) => {
    const next = new Set(prev);
    next.has(label) ? next.delete(label) : next.add(label);
    return next;
  });

  // Parse dry-run nodes
  const dagNodes: DryRunNode[] = dryRunResult?.nodes || [];
  const nodeIds = new Set(dagNodes.map((n: DryRunNode) => n.id));

  // Build active groups from plan
  const activeGroups = PHASE_GROUPS
    .map((g) => ({ ...g, phases: g.phases.filter((p) => nodeIds.has(p)) }))
    .filter((g) => g.phases.length > 0);

  const totalPhases = dagNodes.length;
  const reviewConfigs: Record<string, Record<string, string>> = dryRunResult?.effective_config || {};

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

      {/* Execution plan — accordion groups */}
      {activeGroups.length > 0 && (
        <div className="select-none max-w-xl">
          {activeGroups.map((group, gi) => {
            const isLast = gi === activeGroups.length - 1;
            const GroupIcon = group.icon;
            const isExpanded = expanded.has(group.label);
            const groupKey = group.label.toLowerCase();
            const cfgEntries = reviewConfigs[groupKey];

            return (
              <div key={group.label} className="relative">
                {gi > 0 && (
                  <div className="flex justify-start pl-[11px]">
                    <div className="w-px h-2.5 bg-zinc-800" />
                  </div>
                )}

                <div className="border border-zinc-800/80 bg-zinc-900/30">
                  <button
                    type="button"
                    onClick={() => toggle(group.label)}
                    className="flex items-center gap-2 px-3 py-2 w-full text-left cursor-pointer hover:bg-white/[0.02] transition-colors"
                  >
                    <div className="w-6 h-6 rounded-full bg-zinc-900 border border-zinc-700 flex items-center justify-center shrink-0">
                      <GroupIcon className="w-3 h-3 text-zinc-500" />
                    </div>
                    <span className="text-[11px] font-semibold text-zinc-300 font-mono flex-1">{group.label}</span>
                    <span className="text-[10px] text-zinc-600 font-mono tabular-nums">{group.phases.length}</span>
                    <ChevronDown className={`w-3 h-3 text-zinc-600 transition-transform duration-200 ${isExpanded ? "rotate-180" : ""}`} />
                  </button>

                  {isExpanded && (
                    <>
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

                      {groupKey === "database" && dbConfigDraft !== null ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50">
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Database Config (editable JSON — overrides field-level settings)</span>
                          </div>
                          <textarea
                            value={dbConfigDraft}
                            onChange={(e) => setDbConfigDraft(e.target.value)}
                            spellCheck={false}
                            className="w-full h-72 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
                          />
                          {(() => {
                            const ds = suggestDiskGb(script, scaleFactor);
                            const actual = extractDbDiskGb(dbConfigDraft);
                            if (actual === null) return null;
                            const undersized = actual < ds.diskGb;
                            return (
                              <div className={`mt-2 text-[10px] font-mono ${undersized ? "text-amber-500" : "text-zinc-600"}`}>
                                {undersized
                                  ? `Preset DB disk ${actual} GB may be too small: ${ds.reason}. Bump disk_gb in the JSON above.`
                                  : `DB disk ${actual} GB >= recommended ${ds.diskGb} GB (${ds.reason})`}
                              </div>
                            );
                          })()}
                          {Object.keys(renderedConfigDrafts).length > 0 && (
                            <div className="mt-3 space-y-3">
                              {Object.entries(renderedConfigDrafts).map(([key, body]) => {
                                const dirty = renderedConfigsPristine[key] !== body;
                                return (
                                  <div key={key}>
                                    <div className="flex items-center gap-2 mb-1">
                                      <span className="text-[9px] font-mono text-zinc-600 uppercase">{key}</span>
                                      {dirty && <span className="text-[9px] font-mono text-amber-500">edited</span>}
                                      {dirty && (
                                        <button
                                          type="button"
                                          onClick={() => setRenderedConfigDrafts((prev) => ({ ...prev, [key]: renderedConfigsPristine[key] }))}
                                          className="text-[9px] font-mono text-zinc-500 hover:text-zinc-300 ml-auto"
                                        >
                                          revert
                                        </button>
                                      )}
                                    </div>
                                    <textarea
                                      value={body}
                                      onChange={(e) => setRenderedConfigDrafts((prev) => ({ ...prev, [key]: e.target.value }))}
                                      spellCheck={false}
                                      className="w-full h-56 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
                                    />
                                  </div>
                                );
                              })}
                            </div>
                          )}
                        </div>
                      ) : groupKey === "benchmark" && stroppyConfigDraft !== null ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50">
                          <div className="flex items-center gap-2 mb-2">
                            <span className="text-[10px] font-mono text-zinc-500 shrink-0">Stroppy Version</span>
                            <input
                              type="text"
                              value={stroppyVersion}
                              onChange={(e) => setStroppyVersion(e.target.value)}
                              spellCheck={false}
                              placeholder="5.0.0rc3"
                              className="flex-1 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 px-2 py-1 outline-none focus:border-zinc-600"
                            />
                          </div>
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-[9px] font-mono text-zinc-600 uppercase">Stroppy Config (editable protojson — overrides field-level settings)</span>
                          </div>
                          <textarea
                            value={stroppyConfigDraft}
                            onChange={(e) => setStroppyConfigDraft(e.target.value)}
                            spellCheck={false}
                            className="w-full h-72 bg-[#0a0a0a] text-[11px] font-mono text-zinc-300 border border-zinc-800 p-2 outline-none focus:border-zinc-600 resize-y"
                          />
                          {cfgEntries && Object.keys(cfgEntries).length > 0 && (
                            <div className="mt-2 space-y-0.5">
                              <span className="text-[9px] font-mono text-zinc-600 uppercase">Summary</span>
                              {Object.entries(cfgEntries).map(([k, v]) => (
                                <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                              ))}
                            </div>
                          )}
                        </div>
                      ) : cfgEntries && Object.keys(cfgEntries).length > 0 ? (
                        <div className="border-t border-zinc-800/20 px-3 py-1.5 bg-zinc-900/50 space-y-0.5">
                          {Object.entries(cfgEntries).map(([k, v]) => (
                            <EditableCfgRow key={k} k={k} v={v} groupKey={groupKey} onEdit={onEdit} />
                          ))}
                        </div>
                      ) : null}
                    </>
                  )}
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
