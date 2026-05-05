import { useEffect, useState, useCallback } from "react";
import { useNavigate, Link } from "react-router-dom";
import {
  listSuites,
  createSuite,
  deleteSuite,
  getSuite,
  listRunPresets,
} from "@/api/client";
import type { Suite, SuiteRunSummary, RunPreset } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Plus,
  Trash2,
  AlertCircle,
  Boxes,
  ChevronRight,
  ChevronDown,
  Filter,
  Play,
  ExternalLink,
} from "lucide-react";
import { useConfirm } from "@/components/ui/confirm-dialog";

interface SuiteExtra {
  runs: SuiteRunSummary[];
  loaded: boolean;
}

export function Suites() {
  const navigate = useNavigate();
  const confirm = useConfirm();
  const [suites, setSuites] = useState<Suite[]>([]);
  const [runPresets, setRunPresets] = useState<RunPreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  // Per-suite expansion state. Suite-level run history is fetched lazily
  // when first opened — listSuites doesn't include runs to keep the index
  // cheap for tenants with hundreds of suites.
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [extras, setExtras] = useState<Record<string, SuiteExtra>>({});

  const [createOpen, setCreateOpen] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDescription, setNewDescription] = useState("");
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    try {
      const [s, rps] = await Promise.all([listSuites(), listRunPresets()]);
      setSuites(s);
      setRunPresets(rps);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function toggle(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
    if (!extras[id]?.loaded) {
      try {
        const detail = await getSuite(id);
        setExtras((prev) => ({
          ...prev,
          [id]: { loaded: true, runs: detail.runs ?? [] },
        }));
      } catch {
        setExtras((prev) => ({ ...prev, [id]: { loaded: true, runs: [] } }));
      }
    }
  }

  async function handleCreate() {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const r = await createSuite({ name: newName.trim(), description: newDescription, items: [] });
      setNewName(""); setNewDescription("");
      setCreateOpen(false);
      navigate(`/suites/${r.id}`);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(s: Suite) {
    if (!(await confirm({
      title: `Delete suite "${s.name}"?`,
      description: "Run history attached to this suite will remain but lose its grouping.",
      danger: true,
    }))) return;
    try {
      await deleteSuite(s.id);
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    }
  }

  // Group runs by batch_id for display, newest batch first.
  function groupBatches(runs: SuiteRunSummary[]): { batchId: string; runs: SuiteRunSummary[]; createdAt: string }[] {
    const map = new Map<string, SuiteRunSummary[]>();
    for (const r of runs) {
      const arr = map.get(r.batch_id) ?? [];
      arr.push(r);
      map.set(r.batch_id, arr);
    }
    const out: { batchId: string; runs: SuiteRunSummary[]; createdAt: string }[] = [];
    for (const [batchId, list] of map) {
      list.sort((a, b) => a.position - b.position);
      out.push({ batchId, runs: list, createdAt: list[0]?.created_at ?? "" });
    }
    out.sort((a, b) => (a.createdAt < b.createdAt ? 1 : -1));
    return out;
  }

  const presetMap = new Map(runPresets.map((p) => [p.id, p]));

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold flex items-center gap-2">
            <Boxes className="h-4 w-4 text-primary" />
            Suites
          </h1>
          <p className="text-sm text-muted-foreground">
            Sequenced groups of run-presets. Re-runnable as a batch with parameter overrides.
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              New Suite
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create Suite</DialogTitle>
            </DialogHeader>
            <div className="space-y-3 pt-2">
              <div className="space-y-1.5">
                <label className="text-xs text-zinc-500 font-mono">Name</label>
                <Input value={newName} onChange={(e) => setNewName(e.target.value)} autoFocus
                  placeholder="e.g. tpcc-pg-sweep-100-200-400-vus" />
              </div>
              <div className="space-y-1.5">
                <label className="text-xs text-zinc-500 font-mono">Description</label>
                <textarea
                  value={newDescription}
                  onChange={(e) => setNewDescription(e.target.value)}
                  rows={3}
                  placeholder="What this suite measures, when to run it, who owns it"
                  className="w-full bg-transparent border border-zinc-800 px-2 py-1.5 text-xs font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
                />
              </div>
              <Button onClick={handleCreate} disabled={creating || !newName.trim()}>
                {creating ? "Creating..." : "Create & Edit"}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
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
      ) : suites.length === 0 ? (
        <div className="border border-dashed border-zinc-800 p-10 text-center space-y-2">
          <Boxes className="h-6 w-6 mx-auto text-zinc-700" />
          <p className="text-sm text-zinc-500">No suites yet.</p>
          <p className="text-xs text-zinc-600">
            Create one and add run-presets to compose a reproducible batch.
          </p>
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8" />
              <TableHead>Name</TableHead>
              <TableHead>Steps</TableHead>
              <TableHead>Last batch</TableHead>
              <TableHead>Updated</TableHead>
              <TableHead className="w-32" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {suites.map((s) => {
              const isOpen = expanded.has(s.id);
              const extra = extras[s.id];
              const batches = extra ? groupBatches(extra.runs) : [];
              const lastBatch = batches[0];
              return (
                <>
                  <TableRow
                    key={s.id}
                    className="cursor-pointer hover:bg-zinc-900/40"
                    onClick={() => toggle(s.id)}
                  >
                    <TableCell>
                      {isOpen ? <ChevronDown className="h-3.5 w-3.5 text-zinc-500" /> : <ChevronRight className="h-3.5 w-3.5 text-zinc-500" />}
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-col leading-tight max-w-md">
                        <span className="text-xs text-zinc-200 truncate">{s.name}</span>
                        {s.description && (
                          <span className="text-[10px] text-zinc-500 truncate">{s.description}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs text-zinc-400">
                      {s.items?.length ?? 0}
                    </TableCell>
                    <TableCell className="font-mono text-[10px] text-zinc-500">
                      {lastBatch ? (
                        <>
                          <span className="text-zinc-400">{new Date(lastBatch.createdAt).toLocaleString()}</span>
                          <span className="text-zinc-600 ml-1.5">· {lastBatch.runs.length} run{lastBatch.runs.length === 1 ? "" : "s"}</span>
                        </>
                      ) : isOpen ? "—" : ""}
                    </TableCell>
                    <TableCell className="text-xs text-zinc-500">
                      {new Date(s.updated_at).toLocaleDateString()}
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <div className="flex items-center gap-2">
                        <Link
                          to={`/runs?suite=${s.id}`}
                          className="text-zinc-400 hover:text-primary transition-colors"
                          title="View all runs from this suite"
                        >
                          <Filter className="h-3.5 w-3.5" />
                        </Link>
                        <Link
                          to={`/suites/${s.id}`}
                          className="text-zinc-400 hover:text-zinc-200 transition-colors"
                          title="Open suite"
                        >
                          <ExternalLink className="h-3.5 w-3.5" />
                        </Link>
                        <button
                          onClick={() => handleDelete(s)}
                          className="text-zinc-400 hover:text-destructive transition-colors"
                          title="Delete"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                  {isOpen && (
                    <TableRow key={`${s.id}-detail`} className="bg-zinc-950/40 hover:bg-zinc-950/40">
                      <TableCell colSpan={6} className="py-3 px-6">
                        {!extra?.loaded ? (
                          <div className="text-[10px] text-zinc-600 font-mono">Loading runs...</div>
                        ) : (
                          <div className="space-y-3">
                            {/* Steps */}
                            {s.items && s.items.length > 0 && (
                              <div className="space-y-1">
                                <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                                  Steps ({s.items.length})
                                </div>
                                <div className="flex flex-wrap gap-1.5">
                                  {s.items.map((it, i) => {
                                    const rp = presetMap.get(it.run_preset_id);
                                    return (
                                      <span
                                        key={i}
                                        className="inline-flex items-center gap-1.5 border border-zinc-800 px-1.5 py-0.5 text-[10px] font-mono text-zinc-400"
                                      >
                                        <span className="text-zinc-600">#{i + 1}</span>
                                        {rp ? rp.name : <span className="text-destructive">missing</span>}
                                      </span>
                                    );
                                  })}
                                </div>
                              </div>
                            )}

                            {/* Batches */}
                            {batches.length === 0 ? (
                              <div className="flex items-center gap-2 text-[11px] text-zinc-500">
                                <span>No runs yet.</span>
                                <Link
                                  to={`/suites/${s.id}`}
                                  className="text-primary hover:underline inline-flex items-center gap-1"
                                >
                                  <Play className="h-3 w-3" /> Launch
                                </Link>
                              </div>
                            ) : (
                              <div className="space-y-2">
                                <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
                                  Recent batches
                                </div>
                                {batches.slice(0, 3).map((b) => (
                                  <div
                                    key={b.batchId}
                                    className="border border-zinc-800/60 bg-[#070707] p-2 space-y-1"
                                  >
                                    <div className="flex items-center justify-between">
                                      <span className="text-[10px] font-mono text-zinc-500">
                                        batch {b.batchId.slice(0, 8)}
                                        <span className="text-zinc-700 ml-2">
                                          {new Date(b.createdAt).toLocaleString()}
                                        </span>
                                      </span>
                                      <Link
                                        to={`/runs?suite=${s.id}&batch=${b.batchId}`}
                                        className="text-[10px] font-mono text-primary hover:underline"
                                      >
                                        View all →
                                      </Link>
                                    </div>
                                    <div className="flex flex-wrap gap-1">
                                      {b.runs.map((r) => (
                                        <Link
                                          key={r.run_id}
                                          to={`/runs/${r.run_id}`}
                                          className="inline-flex items-center gap-1 border border-zinc-800/60 hover:border-primary/40 hover:bg-primary/5 px-1.5 py-0.5 text-[10px] font-mono text-zinc-300 hover:text-primary transition-colors"
                                          title={r.run_id}
                                        >
                                          <span className="text-zinc-600">#{r.position + 1}</span>
                                          <span>{r.run_id.length > 16 ? r.run_id.slice(-12) : r.run_id}</span>
                                        </Link>
                                      ))}
                                    </div>
                                  </div>
                                ))}
                                {batches.length > 3 && (
                                  <Link
                                    to={`/runs?suite=${s.id}`}
                                    className="text-[10px] font-mono text-zinc-500 hover:text-primary"
                                  >
                                    +{batches.length - 3} more batches in Test Runs →
                                  </Link>
                                )}
                              </div>
                            )}
                          </div>
                        )}
                      </TableCell>
                    </TableRow>
                  )}
                </>
              );
            })}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
