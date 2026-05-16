import { useEffect, useState, useCallback } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import type { TestSuite } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
import { TestSuite_Policy_Mode } from "@/lib/proto/cloud/v1/testing/test_suite_pb";
import type { DatabasePreset } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
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
import { TopologyDiagram } from "@/components/TopologyDiagram";
import { DB_COLORS } from "@/lib/db-colors";
import {
  AlertCircle,
  ArrowLeft,
  Boxes,
  Check,
  Database,
  Loader2,
  Save,
  X,
} from "lucide-react";

const KIND_TO_STR: Partial<Record<Database_Kind, string>> = {
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

export function SuiteBuilder() {
  const navigate = useNavigate();
  const { id } = useParams<{ id?: string }>();
  const editing = Boolean(id);

  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [existingSuite, setExistingSuite] = useState<TestSuite | null>(null);
  const [allPresets, setAllPresets] = useState<DatabasePreset[]>([]);

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedPresetIDs, setSelectedPresetIDs] = useState<string[]>([]);
  const [policyMode, setPolicyMode] = useState<"sequential" | "parallel">("sequential");
  const [maxParallel, setMaxParallel] = useState(1);

  const load = useCallback(async () => {
    const tid = getTenantId();
    try {
      const [presetsResp, suiteResp] = await Promise.all([
        clients.databasePreset.listDatabasePresets(tid ? { value: tid } : {}),
        editing && id ? clients.suite.getTestSuite({ value: id }) : Promise.resolve(null),
      ]);
      setAllPresets(presetsResp.databasePresets ?? []);
      if (suiteResp) {
        setExistingSuite(suiteResp);
        setName(suiteResp.identity?.name ?? "");
        setDescription(suiteResp.identity?.description ?? "");
        // Extract preset IDs from matrix.databases
        const pids = (suiteResp.matrix?.databases ?? [])
          .map((d) => d.databaseVariant?.case === "databasePresetId" ? d.databaseVariant.value.value ?? "" : "")
          .filter(Boolean);
        setSelectedPresetIDs(pids);
        const mode = suiteResp.policy?.mode;
        setPolicyMode(mode === TestSuite_Policy_Mode.PARALLEL ? "parallel" : "sequential");
        setMaxParallel(suiteResp.policy?.maxParallel ?? 1);
      }
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Load failed" });
    } finally {
      setLoading(false);
    }
  }, [id, editing]);

  useEffect(() => { load(); }, [load]);

  function togglePreset(pid: string) {
    setSelectedPresetIDs((prev) =>
      prev.includes(pid) ? prev.filter((x) => x !== pid) : [...prev, pid]
    );
  }

  async function save() {
    if (!name.trim()) {
      setMessage({ type: "error", text: "Name is required" });
      return;
    }
    setSaving(true);
    setMessage(null);
    try {
      const databases = selectedPresetIDs.map((pid) => ({
        databaseVariant: { case: "databasePresetId" as const, value: { value: pid } },
      }));

      const policy = {
        mode: policyMode === "parallel" ? TestSuite_Policy_Mode.PARALLEL : TestSuite_Policy_Mode.SEQUENTIAL,
        maxParallel,
      };

      if (editing && id && existingSuite) {
        await clients.suite.updateTestSuite({
          suite: {
            id: existingSuite.id,
            identity: { name: name.trim(), description: description.trim() },
            matrix: {
              databases,
              workloads: existingSuite.matrix?.workloads ?? [],
            },
            policy,
          },
          updateMask: { paths: ["identity", "matrix", "policy"] },
        });
        navigate(`/suites/${id}`);
      } else {
        const result = await clients.suite.createTestSuite({
          suite: {
            identity: { name: name.trim(), description: description.trim() },
            matrix: { databases, workloads: [] },
            policy,
          },
        });
        navigate(`/suites/${result.id?.value}`);
      }
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Save failed" });
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-zinc-500 gap-2">
        <Loader2 className="w-4 h-4 animate-spin" /> Loading…
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
            {editing ? `Edit suite — ${existingSuite?.identity?.name ?? ""}` : "New suite"}
          </h1>
        </div>
      </div>

      {message && (
        <div className={`shrink-0 px-5 py-2 border-b flex items-center gap-2 text-xs font-mono ${
          message.type === "error"
            ? "border-destructive/30 text-destructive bg-destructive/5"
            : "border-emerald-500/30 text-emerald-400 bg-emerald-500/5"
        }`}>
          {message.type === "error" ? <AlertCircle className="h-3.5 w-3.5 shrink-0" /> : <Check className="h-3.5 w-3.5 shrink-0" />}
          {message.text}
        </div>
      )}

      {/* Body */}
      <div className="flex-1 overflow-auto p-6">
        <div className="max-w-3xl mx-auto space-y-8">

          {/* Identity */}
          <section className="space-y-4">
            <h2 className="text-xs font-mono text-zinc-500 uppercase tracking-wider">1. Identity</h2>
            <div className="space-y-3">
              <div className="space-y-1.5">
                <Label className="text-xs font-mono text-zinc-500">Name <span className="text-destructive">*</span></Label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. PG vs MySQL baseline"
                  className="font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs font-mono text-zinc-500">Description</Label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  rows={2}
                  placeholder="Optional suite description"
                  className="w-full bg-transparent border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
                />
              </div>
            </div>
          </section>

          {/* Policy */}
          <section className="space-y-4">
            <h2 className="text-xs font-mono text-zinc-500 uppercase tracking-wider">2. Policy</h2>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label className="text-xs font-mono text-zinc-500">Run mode</Label>
                <Select value={policyMode} onValueChange={(v) => setPolicyMode(v as "sequential" | "parallel")}>
                  <SelectTrigger className="font-mono text-xs"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="sequential">Sequential</SelectItem>
                    <SelectItem value="parallel">Parallel</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {policyMode === "parallel" && (
                <div className="space-y-1.5">
                  <Label className="text-xs font-mono text-zinc-500">Max parallel</Label>
                  <Input
                    type="number"
                    min={1}
                    max={64}
                    value={maxParallel}
                    onChange={(e) => setMaxParallel(Math.max(1, parseInt(e.target.value) || 1))}
                    className="font-mono"
                  />
                </div>
              )}
            </div>
          </section>

          {/* Database presets */}
          <section className="space-y-4">
            <h2 className="text-xs font-mono text-zinc-500 uppercase tracking-wider">3. Database presets</h2>
            <p className="text-xs text-zinc-600">Select one or more database presets. Each selected preset will be crossed with every workload at launch.</p>
            {allPresets.length === 0 ? (
              <div className="border border-dashed border-zinc-800 p-6 text-center text-xs text-zinc-500">
                No database presets available. Create one in the Presets section first.
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-3">
                {allPresets.map((p) => {
                  const pid = p.id?.value ?? "";
                  const selected = selectedPresetIDs.includes(pid);
                  const kind = KIND_TO_STR[p.database?.kind ?? 0] ?? "postgres";
                  const dbColor = DB_COLORS[kind as keyof typeof DB_COLORS] ?? {};
                  return (
                    <button
                      key={pid}
                      type="button"
                      onClick={() => togglePreset(pid)}
                      className={`border p-3 text-left transition-all cursor-pointer space-y-2 ${
                        selected
                          ? "border-primary/40 bg-primary/[0.06]"
                          : "border-zinc-800/60 hover:bg-zinc-900/50 hover:border-zinc-700"
                      }`}
                    >
                      <div className="flex items-center gap-2 justify-between">
                        <div className="flex items-center gap-2">
                          <Database className="w-3 h-3 shrink-0" style={{ color: (dbColor as Record<string, string>).hex }} />
                          <span className="text-xs font-mono text-zinc-200">{p.identity?.name}</span>
                        </div>
                        {selected && <Check className="w-3 h-3 text-primary shrink-0" />}
                      </div>
                      <TopologyDiagram kind={kind as never} topology={undefined} />
                    </button>
                  );
                })}
              </div>
            )}
            {selectedPresetIDs.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {selectedPresetIDs.map((pid) => {
                  const p = allPresets.find((x) => x.id?.value === pid);
                  return (
                    <span key={pid} className="inline-flex items-center gap-1 px-2 py-0.5 border border-primary/30 text-[10px] font-mono text-primary/80">
                      {p?.identity?.name ?? pid.slice(0, 8)}
                      <button type="button" onClick={() => togglePreset(pid)} className="hover:text-destructive">
                        <X className="w-2.5 h-2.5" />
                      </button>
                    </span>
                  );
                })}
              </div>
            )}
          </section>
        </div>
      </div>

      {/* Footer */}
      <div className="shrink-0 px-5 py-3 border-t border-zinc-800 bg-[#070707] flex items-center justify-end gap-2">
        <Button variant="ghost" onClick={() => navigate("/suites")} disabled={saving}>
          Cancel
        </Button>
        <Button onClick={save} disabled={saving || !name.trim()}>
          {saving ? <Loader2 className="w-3.5 h-3.5 animate-spin mr-1" /> : <Save className="w-3.5 h-3.5 mr-1" />}
          {saving ? "Saving…" : editing ? "Save changes" : "Create suite"}
        </Button>
      </div>
    </div>
  );
}

export default SuiteBuilder;
