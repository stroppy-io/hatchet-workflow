import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { clients } from "@/api/clients";
import { useTenantId, useTenantPath } from "@/hooks/useTenantPath";
import { protoTsToISO } from "@/lib/proto-helpers";
import type { TestRunTemplate } from "@/lib/proto/cloud/v1/testing/test_run_template_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Play, Trash2, Pencil, AlertCircle, FlaskConical } from "lucide-react";
import { DB_COLORS } from "@/lib/db-colors";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Database_Kind } from "@/lib/proto/cloud/v1/catalog/database_pb";
import { Workload_Script } from "@/lib/proto/cloud/v1/catalog/workload_pb";

const ALL_DB_KINDS = ["postgres", "mysql", "mariadb", "picodata", "ydb", "ydb-managed", "cockroach"] as const;

const KIND_STRING: Partial<Record<Database_Kind, string>> = {
  [Database_Kind.DATABASE_KIND_POSTGRES]: "postgres",
  [Database_Kind.DATABASE_KIND_MYSQL]: "mysql",
  [Database_Kind.DATABASE_KIND_MARIADB]: "mariadb",
  [Database_Kind.DATABASE_KIND_YDB]: "ydb",
  [Database_Kind.DATABASE_KIND_YDB_MANAGED]: "ydb-managed",
  [Database_Kind.DATABASE_KIND_COCKROACH]: "cockroach",
  [Database_Kind.DATABASE_KIND_PICODATA]: "picodata",
};

const SCRIPT_STRING: Partial<Record<Workload_Script, string>> = {
  [Workload_Script.TPCC_PROCS]: "tpcc-procs",
  [Workload_Script.TPCC_TX]: "tpcc-tx",
  [Workload_Script.TPCB_PROCS]: "tpcb-procs",
  [Workload_Script.TPCB_TX]: "tpcb-tx",
  [Workload_Script.TPCH_TX]: "tpch-tx",
  [Workload_Script.TPCC_TX_YDB_PGWIRE]: "tpcc-tx-ydb-pgwire",
  [Workload_Script.TPCB_TX_YDB_PGWIRE]: "tpcb-tx-ydb-pgwire",
};

// Extract display info from proto template
function templateDbKind(t: TestRunTemplate): string {
  const dv = t.database?.databaseVariant;
  if (dv?.case === "database") {
    return KIND_STRING[dv.value.kind] ?? "";
  }
  return "";
}

function templateDbVersion(t: TestRunTemplate): string {
  const dv = t.database?.databaseVariant;
  if (dv?.case === "database") {
    return "";
  }
  return "";
}

function templateScript(t: TestRunTemplate): string {
  const wv = t.workload?.workloadVariant;
  if (wv?.case === "workload") {
    return SCRIPT_STRING[wv.value.shape?.script ?? 0] ?? "";
  }
  return "";
}

function templateVus(t: TestRunTemplate): number | null {
  const wv = t.workload?.workloadVariant;
  if (wv?.case === "workload") {
    return wv.value.shape?.load?.vus ?? null;
  }
  return null;
}

function templateDuration(t: TestRunTemplate): string {
  const wv = t.workload?.workloadVariant;
  if (wv?.case === "workload") {
    const mode = wv.value.shape?.load?.mode;
    if (mode?.case === "duration") return mode.value;
  }
  return "";
}

export function RunPresets() {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const tid = useTenantId();
  const tPath = useTenantPath();
  const [templates, setTemplates] = useState<TestRunTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterKind, setFilterKind] = useState<string>("");
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [editing, setEditing] = useState<TestRunTemplate | null>(null);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const resp = await clients.template.listTestRunTemplates(
        tid ? { value: tid } : {}
      );
      let list = resp.testRunTemplates ?? [];
      if (filterKind) {
        list = list.filter((t) => templateDbKind(t) === filterKind);
      }
      setTemplates(list);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, [filterKind, tid]);

  useEffect(() => { load(); }, [load]);

  function handleUse(t: TestRunTemplate) {
    // Seed the new-run wizard with template id so it can pre-fill.
    sessionStorage.setItem("rerun_config", JSON.stringify({ template_id: t.id?.value }));
    navigate(tPath("runs/new"));
  }

  async function handleDelete(t: TestRunTemplate) {
    const name = t.identity?.name ?? t.id?.value;
    if (!(await confirm({
      title: `Delete run preset "${name}"?`,
      description: "This template will no longer be available for new runs or suites that reference it.",
      danger: true,
    }))) return;
    try {
      await clients.template.deleteTestRunTemplate({ value: t.id?.value ?? "" });
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  function startEdit(t: TestRunTemplate) {
    setEditing(t);
    setEditName(t.identity?.name ?? "");
    setEditDescription(t.identity?.description ?? "");
  }

  async function saveEdit() {
    if (!editing) return;
    setSaving(true);
    try {
      await clients.template.updateTestRunTemplate({
        template: {
          id: editing.id,
          identity: { name: editName, description: editDescription },
        },
        updateMask: { paths: ["identity"] },
      });
      setEditing(null);
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold flex items-center gap-2">
            <FlaskConical className="h-4 w-4 text-primary" />
            Run Presets
          </h1>
          <p className="text-sm text-muted-foreground">
            Saved workload + infrastructure templates. Create one from the New Run wizard.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Select value={filterKind || "__all__"} onValueChange={(v) => setFilterKind(v === "__all__" ? "" : v)}>
            <SelectTrigger className="h-8 w-36 font-mono text-xs"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">All databases</SelectItem>
              {ALL_DB_KINDS.map((k) => (
                <SelectItem key={k} value={k}>{k}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" onClick={() => navigate(tPath("runs/new"))}>
            <Play className="h-3.5 w-3.5" />
            New Run
          </Button>
        </div>
      </div>

      {message && (
        <div className={`flex items-center gap-2 text-xs p-2.5 border font-mono ${
          message.type === "error"
            ? "border-destructive/30 text-destructive"
            : "border-emerald-500/30 text-emerald-400"
        }`}>
          <AlertCircle className="h-3.5 w-3.5" />
          {message.text}
        </div>
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : templates.length === 0 ? (
        <div className="border border-dashed border-zinc-800 p-10 text-center space-y-2">
          <FlaskConical className="h-6 w-6 mx-auto text-zinc-700" />
          <p className="text-sm text-zinc-500">No run presets yet.</p>
          <p className="text-xs text-zinc-600">
            Configure a run in the wizard, then click <span className="font-mono">Save as Preset</span> on the review step.
          </p>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Database</TableHead>
              <TableHead>Workload</TableHead>
              <TableHead>VUs</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="w-32" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {templates.map((t) => {
              const dbKind = templateDbKind(t);
              const dbVer = templateDbVersion(t);
              const script = templateScript(t);
              const vus = templateVus(t);
              const duration = templateDuration(t);
              const dbColor = DB_COLORS[dbKind as keyof typeof DB_COLORS];
              return (
                <TableRow key={t.id?.value}>
                  <TableCell>
                    <div className="flex flex-col leading-tight max-w-md">
                      <span className="text-xs text-zinc-200 truncate">{t.identity?.name}</span>
                      {t.identity?.description && (
                        <span className="text-[10px] text-zinc-500 truncate">{t.identity.description}</span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={dbColor?.text}>
                      {dbKind} {dbVer}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-zinc-400">
                    {script || "—"}
                    {duration && <span className="text-zinc-600"> · {duration}</span>}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-zinc-400">
                    {vus ?? "—"}
                  </TableCell>
                  <TableCell className="text-xs text-zinc-500">
                    {t.timestamps?.updatedAt ? new Date(protoTsToISO(t.timestamps.updatedAt)).toLocaleDateString() : "—"}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleUse(t)}
                        className="text-zinc-400 hover:text-primary transition-colors"
                        title="Use this preset"
                      >
                        <Play className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => startEdit(t)}
                        className="text-zinc-400 hover:text-zinc-200 transition-colors"
                        title="Rename"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => handleDelete(t)}
                        className="text-zinc-400 hover:text-destructive transition-colors"
                        title="Delete"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      <Dialog open={!!editing} onOpenChange={(v) => !v && setEditing(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Rename Run Preset</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 pt-2">
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Name</label>
              <Input value={editName} onChange={(e) => setEditName(e.target.value)} autoFocus />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs text-zinc-500 font-mono">Description</label>
              <textarea
                value={editDescription}
                onChange={(e) => setEditDescription(e.target.value)}
                rows={3}
                className="w-full bg-transparent border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
              />
            </div>
            <Button onClick={saveEdit} disabled={saving || !editName.trim()}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
