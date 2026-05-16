import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import type { TestSuite } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
import type { DatabasePreset } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Workload_Script, Workload_Protocol, WorkloadSchema, WorkloadOrPresetSchema } from "@/lib/proto/cloud/v1/catalog/workload_pb";
import { create } from "@bufbuild/protobuf";
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
import { WorkloadForm, type K6Mode, type WorkloadFile, type ProbeResponse, type TenantQuotas } from "@/components/WorkloadForm";
import { KIND_PROTOCOLS } from "@/components/WorkloadForm";
import type { DatabaseKind, Protocol } from "@/components/WorkloadForm";
import { ArrowLeft, AlertCircle, Save, Boxes, Database } from "lucide-react";

// KIND_PROTOCOLS is defined in WorkloadForm — but we need it here too.
// Import it or duplicate. Imported above.

// Map Database_Kind enum → string
const KIND_TO_STR: Partial<Record<Database_Kind, DatabaseKind>> = {
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

// Map Workload_Script enum → legacy script string (for WorkloadForm)
const SCRIPT_TO_STR: Partial<Record<Workload_Script, string>> = {
  [Workload_Script.TPCC_PROCS]: "tpcc/procs",
  [Workload_Script.TPCC_TX]: "tpcc/tx",
  [Workload_Script.TPCB_PROCS]: "tpcb/procs",
  [Workload_Script.TPCB_TX]: "tpcb/tx",
  [Workload_Script.TPCH_TX]: "tpch/tx",
  [Workload_Script.TPCC_TX_YDB_PGWIRE]: "tpcc/tx-ydb-pgwire",
  [Workload_Script.TPCB_TX_YDB_PGWIRE]: "tpcb/tx-ydb-pgwire",
};

const STR_TO_SCRIPT: Record<string, Workload_Script> = {
  "tpcc/procs": Workload_Script.TPCC_PROCS,
  "tpcc/tx": Workload_Script.TPCC_TX,
  "tpcb/procs": Workload_Script.TPCB_PROCS,
  "tpcb/tx": Workload_Script.TPCB_TX,
  "tpch/tx": Workload_Script.TPCH_TX,
  "tpcc/tx-ydb-pgwire": Workload_Script.TPCC_TX_YDB_PGWIRE,
  "tpcb/tx-ydb-pgwire": Workload_Script.TPCB_TX_YDB_PGWIRE,
};

const STR_TO_PROTOCOL: Record<string, Workload_Protocol> = {
  "pg": Workload_Protocol.PG,
  "mysql": Workload_Protocol.MYSQL,
  "picodata": Workload_Protocol.PICODATA,
  "ydb-grpc": Workload_Protocol.YDB_GRPCS,
  "ydb-grpcs": Workload_Protocol.YDB_GRPCS,
  "ydb-pgwire": Workload_Protocol.YDB_PGWIRE,
  "cockroach": Workload_Protocol.PG,
};

// WorkloadEditor: create or edit a workload entry in suite.matrix.workloads[].
// Routes:
//   /suites/:id/items/new              — add new workload to suite matrix
//   /suites/:id/items/:itemId/edit     — edit workload at index (itemId = numeric index)
export function WorkloadEditor() {
  const navigate = useNavigate();
  const { id, itemId } = useParams<{ id: string; itemId?: string }>();
  const editing = Boolean(itemId);
  const workloadIndex = itemId ? parseInt(itemId, 10) : NaN;

  const [suite, setSuite] = useState<TestSuite | null>(null);
  const [dbPresets, setDbPresets] = useState<DatabasePreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // item identity
  const [name, setName] = useState("");

  // workload state
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

  // Probe state
  const [availableSteps, setAvailableSteps] = useState<string[]>([]);
  const [selectedSteps, setSelectedSteps] = useState<string[]>([]);
  const [noSteps, setNoSteps] = useState<string[]>([]);
  const [probeData, setProbeData] = useState<ProbeResponse | null>(null);
  const [, setWorkloadProbeOK] = useState(false);

  // Stroppy runner machine
  const [stroppyCpus, setStroppyCpus] = useState(32);
  const [stroppyMemory, setStroppyMemory] = useState(65536);
  const [stroppyDisk, setStroppyDisk] = useState(100);
  const [stroppyDiskType, setStroppyDiskType] = useState("network-ssd");

  const [stroppyVersion, setStroppyVersion] = useState("");
  const [stroppyVersions, setStroppyVersions] = useState<string[]>([]);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const versionsLoaded = useRef(false);

  const [stroppyConfigDraft, setStroppyConfigDraft] = useState<string | null>(null);
  const [stroppyConfigPristine, setStroppyConfigPristine] = useState<string | null>(null);
  const [stroppyConfigUserEdited, setStroppyConfigUserEdited] = useState(false);

  // Probe target picker
  const [probeTargetID, setProbeTargetID] = useState("");

  const [quotas] = useState<TenantQuotas>({});

  useEffect(() => {
    if (!id) return;
    const tid = getTenantId();
    Promise.all([
      clients.suite.getTestSuite({ value: id }),
      clients.databasePreset.listDatabasePresets(tid ? { value: tid } : {}),
    ])
      .then(([s, presetsResp]) => {
        setSuite(s);
        const allPresets = presetsResp.databasePresets ?? [];
        setDbPresets(allPresets);

        // Set default probe target from suite matrix databases
        const dbs = s.matrix?.databases ?? [];
        if (dbs.length > 0) {
          const first = dbs[0].databaseVariant;
          if (first?.case === "databasePresetId") {
            setProbeTargetID(first.value.value ?? "");
          }
        }

        if (editing && !isNaN(workloadIndex)) {
          const wl = s.matrix?.workloads?.[workloadIndex];
          if (wl?.workloadVariant?.case === "workload") {
            const w = wl.workloadVariant.value;
            const shape = w.shape;
            const tuning = w.tuning;
            if (shape) {
              setScript(SCRIPT_TO_STR[shape.script] ?? "tpcc/procs");
              setSql(shape.sql ?? "");
              if (shape.load) {
                setVUs(shape.load.vus ?? 100);
                setQuiet(shape.load.quiet ?? true);
                setNoThresholds(shape.load.noThresholds ?? false);
                if (shape.load.mode.case === "duration") {
                  setK6Mode("duration");
                  setDuration(shape.load.mode.value);
                } else if (shape.load.mode.case === "iterations") {
                  setK6Mode("iterations");
                  setIterations(shape.load.mode.value);
                }
              }
            }
            if (tuning) {
              setPoolSize(tuning.poolSize ?? 150);
              setScaleFactor(tuning.scaleFactor ?? 500);
              setDefaultInsertMethod(tuning.defaultInsertMethod ?? "native");
              const env: Record<string, string> = {};
              for (const [k, v] of Object.entries(tuning.env ?? {})) {
                env[k] = v;
              }
              setStroppyEnv(env);
              setSelectedSteps(tuning.steps ?? []);
              setNoSteps(tuning.noSteps ?? []);
              const files: WorkloadFile[] = (tuning.files ?? []).map((f) => ({
                name: f.path ?? "",
                kind: "sql",
                content: f.content?.case === "inline" ? f.content.value : "",
              }));
              setWorkloadFiles(files);
            }
          }
          // name from workload preset if available
          setName(`Workload ${workloadIndex + 1}`);
        } else {
          setName("New workload");
        }
      })
      .catch((err) => setMessage({ type: "error", text: err instanceof Error ? err.message : "Load failed" }))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, itemId]);

  // Resolve probe-target preset → (kind, protocol)
  const probePreset = useMemo(
    () => dbPresets.find((p) => p.id?.value === probeTargetID),
    [dbPresets, probeTargetID],
  );
  const probeKind: DatabaseKind = (KIND_TO_STR[probePreset?.database?.kind ?? 0] as DatabaseKind) ?? "postgres";
  const probeProtocol: Protocol = useMemo(() => {
    const protos = KIND_PROTOCOLS[probeKind] ?? [];
    return protos[0] ?? ("pg" as Protocol);
  }, [probeKind]);

  const provider = "docker"; // proto TestSuite has no provider field

  async function save() {
    if (!id || !suite) return;
    setSaving(true);
    setMessage(null);
    try {
      const scriptEnum = STR_TO_SCRIPT[script] ?? Workload_Script.TPCC_TX;
      const protocolEnum = STR_TO_PROTOCOL[probeProtocol] ?? Workload_Protocol.PG;

      const workload = create(WorkloadSchema, {
        shape: {
          script: scriptEnum,
          protocol: protocolEnum,
          sql,
          load: {
            vus,
            quiet,
            noThresholds,
            mode: k6Mode === "duration"
              ? { case: "duration" as const, value: duration }
              : { case: "iterations" as const, value: iterations },
          },
        },
        tuning: {
          poolSize,
          scaleFactor,
          defaultInsertMethod,
          env: stroppyEnv,
          steps: selectedSteps,
          noSteps,
          files: workloadFiles.map((f) => ({
            path: f.name,
            content: { case: "inline" as const, value: f.content },
          })),
          configOverrideJson: stroppyConfigUserEdited && stroppyConfigDraft ? stroppyConfigDraft : "",
        },
      });

      const newWorkloads = [...(suite.matrix?.workloads ?? [])];
      if (editing && !isNaN(workloadIndex)) {
        newWorkloads[workloadIndex] = create(WorkloadOrPresetSchema, { workloadVariant: { case: "workload", value: workload } });
      } else {
        newWorkloads.push(create(WorkloadOrPresetSchema, { workloadVariant: { case: "workload", value: workload } }));
      }

      await clients.suite.updateTestSuite({
        suite: {
          id: suite.id,
          matrix: {
            databases: suite.matrix?.databases ?? [],
            workloads: newWorkloads,
          },
        },
        updateMask: { paths: ["matrix"] },
      });
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

  const suiteDbPresets = (suite?.matrix?.databases ?? [])
    .map((d) => {
      if (d.databaseVariant?.case === "databasePresetId") {
        return d.databaseVariant.value.value ?? "";
      }
      return "";
    })
    .filter(Boolean);

  if (suiteDbPresets.length === 0) {
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
          {editing ? `Edit workload #${workloadIndex + 1}` : "New workload"}
          {suite && (
            <span className="text-sm text-muted-foreground font-normal">in "{suite.identity?.name}"</span>
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
          description="Driver-type for live workload probe."
        >
          <Field label="Probe against">
            <Select value={probeTargetID} onValueChange={setProbeTargetID}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {suiteDbPresets.map((pid) => {
                  const p = dbPresets.find((x) => x.id?.value === pid);
                  const kind = KIND_TO_STR[p?.database?.kind ?? 0] ?? "db";
                  return (
                    <SelectItem key={pid} value={pid}>
                      {p ? `${kind} · ${p.identity?.name}` : pid.slice(0, 8)}
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
            platformId=""
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
