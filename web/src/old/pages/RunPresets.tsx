import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "@/lib/router";
import {
  listRunPresets,
  deleteRunPreset,
  updateRunPreset,
} from "@/services/client";
import { ALL_DB_KINDS, type RunPreset } from "@/services/types";
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

export function RunPresets() {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [presets, setPresets] = useState<RunPreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [filterKind, setFilterKind] = useState<string>("");
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // Edit dialog — only name/description; the underlying RunConfig is edited
  // from NewRun via "Save as preset" instead, since editing it inline would
  // re-implement the whole wizard.
  const [editing, setEditing] = useState<RunPreset | null>(null);
  const [editName, setEditName] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const params = filterKind ? { db_kind: filterKind } : undefined;
      setPresets(await listRunPresets(params));
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, [filterKind]);

  useEffect(() => { load(); }, [load]);

  function handleUse(p: RunPreset) {
    // Hydrate NewRun by piggy-backing on the existing rerun flow — same path
    // already used for re-running an existing run. The wizard reads
    // sessionStorage on mount and seeds every state field from it.
    const cfg = { ...p.config, run_preset_id: p.id };
    sessionStorage.setItem("rerun_config", JSON.stringify(cfg));
    navigate("/runs/new");
  }

  async function handleDelete(p: RunPreset) {
    if (!(await confirm({
      title: `Delete run preset "${p.name}"?`,
      description: "This template will no longer be available for new runs or suites that reference it.",
      danger: true,
    }))) return;
    try {
      await deleteRunPreset(p.id);
      setMessage({ type: "success", text: "Deleted" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  function startEdit(p: RunPreset) {
    setEditing(p);
    setEditName(p.name);
    setEditDescription(p.description);
  }

  async function saveEdit() {
    if (!editing) return;
    setSaving(true);
    try {
      await updateRunPreset(editing.id, { name: editName, description: editDescription });
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
          <Button size="sm" onClick={() => navigate("/runs/new")}>
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
      ) : presets.length === 0 ? (
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
            {presets.map((p) => {
              const c = p.config || {};
              const dbColor = DB_COLORS[c.database?.kind as keyof typeof DB_COLORS];
              return (
                <TableRow key={p.id}>
                  <TableCell>
                    <div className="flex flex-col leading-tight max-w-md">
                      <span className="text-xs text-zinc-200 truncate">{p.name}</span>
                      {p.description && (
                        <span className="text-[10px] text-zinc-500 truncate">{p.description}</span>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={dbColor?.text}>
                      {c.database?.kind} {c.database?.version}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-zinc-400">
                    {c.stroppy?.script || "—"}
                    {c.stroppy?.duration && (
                      <span className="text-zinc-600"> · {c.stroppy.duration}</span>
                    )}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-zinc-400">
                    {c.stroppy?.vus ?? "—"}
                  </TableCell>
                  <TableCell className="text-xs text-zinc-500">
                    {new Date(p.updated_at).toLocaleDateString()}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleUse(p)}
                        className="text-zinc-400 hover:text-primary transition-colors"
                        title="Use this preset"
                      >
                        <Play className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => startEdit(p)}
                        className="text-zinc-400 hover:text-zinc-200 transition-colors"
                        title="Rename"
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </button>
                      <button
                        onClick={() => handleDelete(p)}
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
