import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { clients } from "@/api/clients";
import { useTenantId, useTenantPath } from "@/hooks/useTenantPath";
import { protoTsToISO } from "@/lib/proto-helpers";
import type { TestSuite } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
import type { DatabasePreset } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Workload_Script } from "@/lib/proto/cloud/v1/catalog/workload_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { DB_COLORS } from "@/lib/db-colors";
import {
  AlertCircle,
  ArrowLeft,
  Boxes,
  Check,
  Database,
  Play,
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
} from "lucide-react";

// ─── Helpers ─────────────────────────────────────────────────────

const KIND_TO_STR: Partial<Record<Database_Kind, string>> = {
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

const SCRIPT_TO_STR: Partial<Record<Workload_Script, string>> = {
  [Workload_Script.TPCC_PROCS]: "tpcc/procs",
  [Workload_Script.TPCC_TX]: "tpcc/tx",
  [Workload_Script.TPCB_PROCS]: "tpcb/procs",
  [Workload_Script.TPCB_TX]: "tpcb/tx",
  [Workload_Script.TPCH_TX]: "tpch/tx",
  [Workload_Script.TPCC_TX_YDB_PGWIRE]: "tpcc/tx-ydb-pgwire",
  [Workload_Script.TPCB_TX_YDB_PGWIRE]: "tpcb/tx-ydb-pgwire",
};

function formatTimestamp(ts?: string): string {
  if (!ts) return "—";
  const d = new Date(ts);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return "—";
  return d.toLocaleString("en-GB", { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

export function SuiteDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const tid = useTenantId();
  const tPath = useTenantPath();

  const [suite, setSuite] = useState<TestSuite | null>(null);
  const [dbPresets, setDbPresets] = useState<DatabasePreset[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  const [loading, setLoading] = useState(true);

  const [activeTab, setActiveTab] = useState(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get("tab") || "overview";
  });

  const load = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [s, presetsResp] = await Promise.all([
        clients.suite.getTestSuite({ value: id }),
        clients.databasePreset.listDatabasePresets(tid ? { value: tid } : {}),
      ]);
      setSuite(s);
      setDbPresets(presetsResp.databasePresets ?? []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  async function launch() {
    if (!suite?.id) return;
    setBusy(true);
    setMessage(null);
    try {
      await clients.suiteRun.launchTestSuite(suite.id);
      setMessage({ type: "success", text: "Suite launched" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Launch failed" });
    } finally {
      setBusy(false);
    }
  }

  async function removeWorkload(idx: number) {
    if (!suite) return;
    const ok = await confirm({
      title: `Remove workload #${idx + 1}?`,
      description: "Removed from this suite's matrix immediately.",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      const newWorkloads = (suite.matrix?.workloads ?? []).filter((_, i) => i !== idx);
      await clients.suite.updateTestSuite({
        suite: { id: suite.id, matrix: { databases: suite.matrix?.databases ?? [], workloads: newWorkloads } },
        updateMask: { paths: ["matrix"] },
      });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    } finally {
      setBusy(false);
    }
  }

  if (loading && !suite) {
    return (
      <div className="p-6 text-sm text-muted-foreground">Loading suite…</div>
    );
  }

  if (!suite) {
    return (
      <div className="p-6">
        {error && (
          <div className="flex items-center gap-2 text-sm p-3 border border-destructive/30 text-destructive">
            <AlertCircle className="h-4 w-4" />
            {error}
          </div>
        )}
      </div>
    );
  }

  const workloads = suite.matrix?.workloads ?? [];
  const databases = suite.matrix?.databases ?? [];

  // Map database preset IDs from matrix
  const matrixPresetIds = databases.map((d) =>
    d.databaseVariant?.case === "databasePresetId" ? d.databaseVariant.value.value : ""
  ).filter(Boolean);
  const matrixPresets = matrixPresetIds.map((pid) => dbPresets.find((p) => p.id?.value === pid)).filter(Boolean) as DatabasePreset[];

  const policyMode = suite.policy?.mode === 2 ? "parallel" : "sequential";
  const maxParallel = suite.policy?.maxParallel ?? 1;

  const createdAt = suite.timestamps?.createdAt ? formatTimestamp(protoTsToISO(suite.timestamps.createdAt)) : "—";
  const updatedAt = suite.timestamps?.updatedAt ? formatTimestamp(protoTsToISO(suite.timestamps.updatedAt)) : "—";

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3 min-w-0">
          <Button variant="ghost" size="sm" onClick={() => navigate(tPath("suites"))} className="text-zinc-500 hover:text-zinc-200 -ml-2">
            <ArrowLeft className="h-3.5 w-3.5" />
          </Button>
          <div className="min-w-0">
            <h1 className="text-lg font-semibold leading-tight flex items-center gap-2">
              <Boxes className="w-4 h-4 text-primary shrink-0" />
              <span className="truncate">{suite.identity?.name}</span>
            </h1>
            <p className="text-[11px] font-mono text-zinc-500 leading-tight truncate">{suite.id?.value}</p>
            {suite.identity?.description && (
              <p className="text-xs text-zinc-400 mt-1 max-w-2xl">{suite.identity.description}</p>
            )}
          </div>
          <Badge variant="outline">{policyMode}</Badge>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={load} disabled={busy}>
            <RefreshCw className={`h-3.5 w-3.5 ${busy ? "animate-spin" : ""}`} />
            Refresh
          </Button>
          <Link to={tPath(`suites/${id}/edit`)}>
            <Button variant="outline" size="sm">
              <Pencil className="h-3.5 w-3.5" />
              Edit
            </Button>
          </Link>
          <Button size="sm" onClick={launch} disabled={busy || workloads.length === 0 || databases.length === 0}>
            <Play className="h-3.5 w-3.5" />
            Launch
          </Button>
        </div>
      </div>

      {/* Messages */}
      {(error || message) && (
        <div className={`flex items-center gap-2 text-xs p-2 border font-mono ${
          (message?.type === "error" || error) ? "border-destructive/30 text-destructive" : "border-emerald-500/30 text-emerald-400"
        }`}>
          {message?.type === "success" ? <Check className="h-3 w-3" /> : <AlertCircle className="h-3 w-3" />}
          {error ?? message?.text}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="workloads">
            Workloads <Badge variant="secondary" className="ml-1 text-[9px]">{workloads.length}</Badge>
          </TabsTrigger>
          <TabsTrigger value="databases">
            Databases <Badge variant="secondary" className="ml-1 text-[9px]">{databases.length}</Badge>
          </TabsTrigger>
        </TabsList>

        {/* Overview tab */}
        <TabsContent value="overview">
          <Card>
            <CardContent className="pt-4 space-y-4">
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <div className="text-xs text-zinc-500 font-mono mb-0.5">Mode</div>
                  <div className="font-mono">{policyMode}{policyMode === "parallel" ? ` (max ${maxParallel})` : ""}</div>
                </div>
                <div>
                  <div className="text-xs text-zinc-500 font-mono mb-0.5">Matrix cells</div>
                  <div className="font-mono">{databases.length} db × {workloads.length} workloads = {databases.length * workloads.length}</div>
                </div>
                <div>
                  <div className="text-xs text-zinc-500 font-mono mb-0.5">Created</div>
                  <div className="font-mono text-xs text-zinc-400">{createdAt}</div>
                </div>
                <div>
                  <div className="text-xs text-zinc-500 font-mono mb-0.5">Updated</div>
                  <div className="font-mono text-xs text-zinc-400">{updatedAt}</div>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Workloads tab */}
        <TabsContent value="workloads">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs text-zinc-500">Workload configurations in this suite's matrix.</p>
              <Link to={tPath(`suites/${id}/items/new`)}>
                <Button size="sm" variant="outline">
                  <Plus className="h-3.5 w-3.5" />
                  Add Workload
                </Button>
              </Link>
            </div>
            {workloads.length === 0 ? (
              <div className="border border-dashed border-zinc-800 p-8 text-center text-xs text-zinc-500">
                No workloads yet. Add a workload to start building the matrix.
              </div>
            ) : (
              <div className="space-y-2">
                {workloads.map((w, idx) => {
                  const script = w.workloadVariant?.case === "workload"
                    ? SCRIPT_TO_STR[w.workloadVariant.value.shape?.script ?? 0] ?? "?"
                    : w.workloadVariant?.case === "workloadPresetId"
                    ? `preset:${w.workloadVariant.value.value?.slice(0, 8)}`
                    : "?";
                  const vus = w.workloadVariant?.case === "workload"
                    ? w.workloadVariant.value.shape?.load?.vus
                    : null;
                  const duration = w.workloadVariant?.case === "workload"
                    ? (w.workloadVariant.value.shape?.load?.mode?.case === "duration" ? w.workloadVariant.value.shape.load.mode.value : null)
                    : null;
                  return (
                    <Card key={idx}>
                      <CardContent className="py-3 px-4 flex items-center justify-between">
                        <div>
                          <div className="font-mono text-sm text-zinc-200">{script}</div>
                          <div className="text-[11px] font-mono text-zinc-500 mt-0.5">
                            {vus ? `${vus} VUs` : ""}
                            {duration ? ` · ${duration}` : ""}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          <Link to={tPath(`suites/${id}/items/${idx}/edit`)}>
                            <button className="p-1.5 text-zinc-600 hover:text-zinc-300" title="Edit">
                              <Pencil className="w-3 h-3" />
                            </button>
                          </Link>
                          <button onClick={() => removeWorkload(idx)} className="p-1.5 text-zinc-600 hover:text-red-400" title="Remove">
                            <Trash2 className="w-3 h-3" />
                          </button>
                        </div>
                      </CardContent>
                    </Card>
                  );
                })}
              </div>
            )}
          </div>
        </TabsContent>

        {/* Databases tab */}
        <TabsContent value="databases">
          <div className="space-y-3">
            <p className="text-xs text-zinc-500">Database presets included in this suite's matrix.</p>
            {matrixPresets.length === 0 ? (
              <div className="border border-dashed border-zinc-800 p-8 text-center text-xs text-zinc-500">
                No database presets selected. Edit the suite to add databases.
              </div>
            ) : (
              <div className="grid grid-cols-3 gap-4">
                {matrixPresets.map((p) => {
                  const kind = KIND_TO_STR[p.database?.kind ?? 0] ?? "postgres";
                  const dbColor = DB_COLORS[kind as keyof typeof DB_COLORS] ?? {};
                  return (
                    <Card key={p.id?.value}>
                      <CardContent className="py-3 px-4">
                        <div className="flex items-center gap-2 mb-2">
                          <Database className="w-3.5 h-3.5 shrink-0" style={{ color: (dbColor as Record<string, string>).hex }} />
                          <span className="font-mono text-sm text-zinc-200">{p.identity?.name}</span>
                        </div>
                        <Badge variant="outline" className={(dbColor as Record<string, string>).text}>{kind}</Badge>
                        <div className="mt-2">
                          <TopologyDiagram kind={kind as never} topology={undefined} />
                        </div>
                      </CardContent>
                    </Card>
                  );
                })}
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default SuiteDetail;
