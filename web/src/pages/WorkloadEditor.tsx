import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "@/lib/router";
import {
  createSuiteItem,
  getSettings,
  getSuite,
  listPresets,
  listSuiteItems,
  updateSuiteItem,
} from "@/api/client";
import {
  KIND_PROTOCOLS,
  type DatabaseKind,
  type Preset,
  type Protocol,
  type ProbeResponse,
  type StroppyConfig,
  type Suite,
  type SuiteItem,
  type TenantQuotas,
  type WorkloadFile,
} from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { WorkloadForm, type K6Mode } from "@/components/WorkloadForm";
import { ArrowLeft, AlertCircle, Save, Boxes, Database } from "lucide-react";

// WorkloadEditor edits one suite item — i.e. one workload definition along
// the items axis of the suite matrix. The DB axis lives on the suite
// itself (db_preset_ids[]); this page picks one of those presets as the
// "probe target" so the embedded WorkloadForm can run a live driver_type-
// aware probe. Other matrix cells get validated at launch.
//
// Routes:
//   /suites/:id/items/new                — create
//   /suites/:id/items/:itemId/edit       — edit
export function WorkloadEditor() {
  const navigate = useNavigate();
  const { id, itemId } = useParams<{ id: string; itemId?: string }>();
  const editing = Boolean(itemId);

  const [suite, setSuite] = useState<Suite | null>(null);
  const [presets, setPresets] = useState<Preset[]>([]);
  const [item, setItem] = useState<SuiteItem | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // --- item identity ---
  const [name, setName] = useState("");

  // --- workload state (mirrors NewRun's StepStroppy fields) ---
  const [script, setScript] = useState("tpcc/procs");
  const [sql, setSql] = useState("");
  const [stroppyEnv, setStroppyEnv] = useState<Record<string, string>>({});
  const [workloadFiles, setWorkloadFiles] = useState<WorkloadFile[]>([]);
  const [duration, setDuration] = useState("5m");
  const [k6Mode, setK6Mode] = useState<K6Mode>("duration");
  const [iterations, setIterations] = useState(1);
  const [quiet, setQuiet] = useState(true);
  const [noThresholds, setNoThresholds] = useState(false);
  const [defaultInsertMethod, setDefaultInsertMethod] = useState("native");
  const [scaleFactor, setScaleFactor] = useState(500);
  const [vus, setVUs] = useState(100);
  const [poolSize, setPoolSize] = useState(150);

  // Probe state.
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>([]);
  const [noSteps, setNoSteps] = useState<string[]>([]);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [, setWorkloadProbeOK] = useState(false);

  // Stroppy runner machine — only relevant when suite.provider === yandex,
  // but we wire it through anyway so the override survives a save.
  const [stroppyCpus, setStroppyCpus] = useState(32);
  const [stroppyMemory, setStroppyMemory] = useState(65536);
  const [stroppyDisk, setStroppyDisk] = useState(100);
  const [stroppyDiskType, setStroppyDiskType] = useState("network-ssd");

  // Stroppy version.
  const [stroppyVersion, setStroppyVersion] = useState("");
  const [stroppyVersions, setStroppyVersions] = useState<string[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const versionsLoaded = useRef(false);

  // Raw stroppy-config.json editor.
  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(null);
  const [stroppyConfigUserEdited, setStroppyConfigUserEdited] = useState(false);

  // Probe target picker — defaults to the first selected db_preset.
  const [probeTargetID, setProbeTargetID] = useState("");

  // Tenant quotas — passed to WorkloadForm so the runner sliders cap at
  // the tenant's per-node limits.
  const [quotas, setQuotas] = useState<TenantQuotas>({});

  // Bootstrap.
  useEffect(() => {
    if (!id) return;
    Promise.all([
      getSuite(id),
      listPresets(),
      editing ? listSuiteItems(id) : Promise.resolve<SuiteItem[]>([]),
      getSettings().then((s) => s.quotas || {}).catch(() => ({} as TenantQuotas)),
    ])
      .then(([s, ps, items, q]) => {
        setSuite(s);
        setPresets(ps);
        setQuotas(q);
        if (s.db_preset_ids.length > 0) {
          setProbeTargetID(s.db_preset_ids[0]);
        }
        if (editing) {
          const it = items.find((x) => x.id === itemId) ?? null;
          if (it) hydrateFromItem(it);
          else setMessage({ type: "error", text: "item not found" });
        } else {
          setName("New workload");
        }
      })
      .catch((err) => setMessage({ type: "error", text: err instanceof Error ? err.message : "load failed" }))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, itemId]);

  function hydrateFromItem(it: SuiteItem) {
    setItem(it);
    setName(it.name);
    const w = it.workload || ({} as StroppyConfig);
    setScript(w.script || "tpcc/procs");
    setSql(w.sql || "");
    setStroppyEnv(w.env || {});
    setWorkloadFiles(w.files || []);
    setDuration(w.duration || "5m");
    setK6Mode((w.k6_mode as K6Mode) || "duration");
    setIterations(w.iterations || 1);
    setQuiet(w.quiet ?? true);
    setNoThresholds(Boolean(w.no_thresholds));
    setDefaultInsertMethod(w.default_insert_method || "native");
    setScaleFactor(w.scale_factor ?? 500);
    setVUs(w.vus ?? 100);
    setPoolSize(w.pool_size ?? 150);
    setNoSteps(w.no_steps || []);
    setSelectedSteps(w.steps || []);
    setStroppyVersion(w.version || "");
    if (w.machine) {
      if (w.machine.cpus) setStroppyCpus(w.machine.cpus);
      if (w.machine.memory_mb) setStroppyMemory(w.machine.memory_mb);
      if (w.machine.disk_gb) setStroppyDisk(w.machine.disk_gb);
      if (w.machine.disk_type) setStroppyDiskType(w.machine.disk_type);
    }
    if (w.config_override_json) {
      setStroppyConfigDraft(w.config_override_json);
      setStroppyConfigPristine(w.config_override_json);
    }
  }

  // Resolve the probe-target preset → (kind, protocol) drives WorkloadForm.
  const probePreset = useMemo(
    () => presets.find((p) => p.id === probeTargetID),
    [presets, probeTargetID],
  );
  const probeKind = (probePreset?.db_kind as DatabaseKind) || "postgres";
  const probeProtocol: Protocol = useMemo(() => {
    const protos = KIND_PROTOCOLS[probeKind] ?? [];
    return protos[0] ?? ("pg" as Protocol);
  }, [probeKind]);

  // Provider/platform inherited from the suite — used by WorkloadForm to
  // decide whether to render the runner machine sliders.
  const provider = suite?.provider ?? "docker";
  const platformId = suite?.platform_id ?? "";

  function buildWorkload(): StroppyConfig {
    const w: StroppyConfig = {
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
      w.machine = {
        role: "stroppy",
        count: 1,
        cpus: stroppyCpus,
        memory_mb: stroppyMemory,
        disk_gb: stroppyDisk,
        disk_type: stroppyDiskType,
      };
    }
    return w;
  }

  async function save() {
    if (!id) return;
    setSaving(true);
    setMessage(null);
    try {
      const workload = buildWorkload();
      if (editing && itemId) {
        await updateSuiteItem(id, itemId, { name, workload });
      } else {
        await createSuiteItem(id, { name, workload });
      }
      navigate(`/suites/${id}`);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Save failed" });
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <div className="container mx-auto px-4 py-6 text-sm text-muted-foreground">Loading…</div>;
  }

  if (suite && suite.db_preset_ids.length === 0) {
    return (
      <div className="container mx-auto px-4 py-6 max-w-3xl">
        <Button variant="ghost" size="sm" onClick={() => navigate(`/suites/${id}`)}>
          <ArrowLeft className="w-4 h-4 mr-1" /> Back
        </Button>
        <div className="mt-6 border rounded p-6 text-sm text-center text-muted-foreground">
          Pick at least one database preset on the suite first — workloads
          probe against a target preset's protocol.
        </div>
      </div>
    );
  }

  return (
    <div className="container mx-auto px-4 py-6 max-w-4xl">
      <div className="flex items-center gap-2 mb-6">
        <Button variant="ghost" size="sm" onClick={() => navigate(`/suites/${id}`)}>
          <ArrowLeft className="w-4 h-4 mr-1" /> Back
        </Button>
        <h1 className="text-xl font-semibold flex items-center gap-2">
          <Boxes className="w-5 h-5" />
          {editing ? `Edit workload — ${item?.name ?? ""}` : "New workload"}
          {suite && (
            <span className="text-sm text-muted-foreground font-normal">in “{suite.name}”</span>
          )}
        </h1>
      </div>

      {message && (
        <div
          className={`mb-4 px-3 py-2 rounded text-sm flex items-center gap-2 ${
            message.type === "success"
              ? "bg-green-50 text-green-900 dark:bg-green-950 dark:text-green-100"
              : "bg-red-50 text-red-900 dark:bg-red-950 dark:text-red-100"
          }`}
        >
          <AlertCircle className="w-4 h-4" /> {message.text}
        </div>
      )}

      <div className="space-y-6">
        <Section title="Identity">
          <Field label="Workload name" required>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. TPC-C 5m heavy" />
          </Field>
        </Section>

        <Section
          title="Probe target"
          icon={<Database className="w-4 h-4" />}
          description="Driver-type for live workload probe. Matrix-launch validates every other suite preset against this same workload at fire time."
        >
          <Field label="Probe against">
            <Select value={probeTargetID} onValueChange={setProbeTargetID}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {(suite?.db_preset_ids || []).map((pid) => {
                  const p = presets.find((x) => x.id === pid);
                  return (
                    <SelectItem key={pid} value={pid}>
                      {p ? `${p.db_kind} · ${p.name}` : pid.slice(0, 8)}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Probe driver: <span className="font-mono">{probeKind}/{probeProtocol}</span>
            </p>
          </Field>
        </Section>

        <div className="border rounded p-4">
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
        </div>

        <div className="flex items-center justify-end gap-2 pt-4 border-t">
          <Button variant="ghost" onClick={() => navigate(`/suites/${id}`)}>Cancel</Button>
          <Button disabled={saving || !name.trim()} onClick={save}>
            <Save className="w-4 h-4 mr-1" />
            {editing ? "Save changes" : "Add workload"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function Section({
  title, description, icon, children,
}: {
  title: string; description?: string; icon?: React.ReactNode; children: React.ReactNode;
}) {
  return (
    <div className="border rounded p-4 space-y-4">
      <div>
        <h2 className="font-semibold flex items-center gap-2">{icon}{title}</h2>
        {description && (
          <p className="text-xs text-muted-foreground mt-1">{description}</p>
        )}
      </div>
      {children}
    </div>
  );
}

function Field({
  label, hint, required, children,
}: {
  label: string; hint?: string; required?: boolean; children: React.ReactNode;
}) {
  return (
    <div className="space-y-1">
      <Label>
        {label}
        {required && <span className="text-destructive ml-0.5">*</span>}
      </Label>
      {children}
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}

export default WorkloadEditor;
