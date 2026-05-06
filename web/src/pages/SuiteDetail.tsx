import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams, Link, useNavigate } from "react-router-dom";
import {
  getSuite,
  updateSuite,
  listRunPresets,
  launchSuite,
  cancelBatch,
} from "@/api/client";
import type {
  Suite,
  SuiteItem,
  SuitePolicy,
  SuiteRunSummary,
  RunPreset,
  RunConfig,
} from "@/api/types";
import { DEFAULT_SUITE_POLICY } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Plus,
  Trash2,
  ChevronUp,
  ChevronDown,
  Play,
  Save,
  AlertCircle,
  Boxes,
  ChevronRight,
  Filter,
  ExternalLink,
  Layers,
  ListOrdered,
  History,
} from "lucide-react";
import { JsonEditor } from "@/components/ui/json-editor";
import { DB_COLORS } from "@/lib/db-colors";

// SuiteDetail — full-screen workspace for editing a suite, launching it, and
// inspecting its run history. Layout: header (identity + actions) → 3-col
// dense workspace: steps editor (left), launch params + stats (center),
// run history (right). On narrow viewports columns stack via grid breakpoints.
export function SuiteDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [suite, setSuite] = useState<Suite | null>(null);
  const [allRunPresets, setAllRunPresets] = useState<RunPreset[]>([]);
  const [loading, setLoading] = useState(true);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [items, setItems] = useState<SuiteItem[]>([]);
  const [policy, setPolicy] = useState<SuitePolicy>(DEFAULT_SUITE_POLICY);
  const [overrideDrafts, setOverrideDrafts] = useState<Record<number, string>>({});
  const [saving, setSaving] = useState(false);
  const [launching, setLaunching] = useState(false);
  const [namePrefix, setNamePrefix] = useState("");
  const [launchDescription, setLaunchDescription] = useState("");

  const load = useCallback(async () => {
    if (!id) return;
    try {
      const [s, rps] = await Promise.all([getSuite(id), listRunPresets()]);
      setSuite(s);
      setName(s.name);
      setDescription(s.description);
      setItems(s.items || []);
      setPolicy(s.policy || DEFAULT_SUITE_POLICY);
      setOverrideDrafts({});
      setAllRunPresets(rps);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed to load" });
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { load(); }, [load]);

  const presetMap = useMemo(() => {
    const m = new Map<string, RunPreset>();
    for (const p of allRunPresets) m.set(p.id, p);
    return m;
  }, [allRunPresets]);

  // Group runs by batch — newest first. Stats (total runs, batches) feed the
  // workspace stats panel.
  const batches = useMemo(() => {
    if (!suite?.runs) return [];
    const map = new Map<string, SuiteRunSummary[]>();
    for (const r of suite.runs) {
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
  }, [suite]);

  const dbKindCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const it of items) {
      const k = presetMap.get(it.run_preset_id)?.db_kind ?? "?";
      counts[k] = (counts[k] || 0) + 1;
    }
    return counts;
  }, [items, presetMap]);

  function addItem(runPresetID: string) {
    if (!runPresetID) return;
    setItems((prev) => [...prev, { run_preset_id: runPresetID }]);
  }

  function removeItem(idx: number) {
    setItems((prev) => prev.filter((_, i) => i !== idx));
    setOverrideDrafts((prev) => {
      const next: Record<number, string> = {};
      for (const [k, v] of Object.entries(prev)) {
        const ki = parseInt(k);
        if (ki < idx) next[ki] = v;
        else if (ki > idx) next[ki - 1] = v;
      }
      return next;
    });
  }

  function move(idx: number, dir: -1 | 1) {
    setItems((prev) => {
      const next = [...prev];
      const target = idx + dir;
      if (target < 0 || target >= next.length) return prev;
      [next[idx], next[target]] = [next[target], next[idx]];
      return next;
    });
  }

  function setOverrides(idx: number, value: string) {
    setOverrideDrafts((prev) => ({ ...prev, [idx]: value }));
    if (!value.trim()) {
      setItems((prev) => {
        const next = [...prev];
        next[idx] = { ...next[idx], overrides: undefined };
        return next;
      });
      return;
    }
    try {
      const parsed = JSON.parse(value) as Partial<RunConfig>;
      setItems((prev) => {
        const next = [...prev];
        next[idx] = { ...next[idx], overrides: parsed };
        return next;
      });
    } catch {
      // Keep raw text in overrideDrafts; don't touch parsed items until valid.
    }
  }

  async function handleSave() {
    if (!id) return;
    setSaving(true);
    try {
      await updateSuite(id, { name, description, items, policy });
      setMessage({ type: "success", text: "Saved" });
      load();
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Failed" });
    } finally {
      setSaving(false);
    }
  }

  async function handleLaunch() {
    if (!id || items.length === 0) return;
    setLaunching(true);
    try {
      // Persist current edits first so the batch reflects the latest plan.
      await updateSuite(id, { name, description, items, policy });
      const r = await launchSuite(id, {
        name_prefix: namePrefix || undefined,
        description: launchDescription || undefined,
      });
      setMessage({ type: "success", text: `Launched batch ${r.batch_id.slice(0, 8)} (${r.items} runs)` });
      navigate(`/runs?suite=${id}&batch=${r.batch_id}`);
    } catch (err) {
      setMessage({ type: "error", text: err instanceof Error ? err.message : "Launch failed" });
    } finally {
      setLaunching(false);
    }
  }

  if (loading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading...</div>;
  }
  if (!suite) {
    return <div className="p-6 text-sm text-destructive">Suite not found</div>;
  }

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header — identity + breadcrumb + global actions. Stays sticky-ish at
          the top while the workspace below scrolls in its own panes. */}
      <div className="shrink-0 border-b border-zinc-800 bg-[#070707] px-5 py-3 space-y-2">
        <div className="flex items-center gap-2 text-[11px] font-mono">
          <Link to="/suites" className="text-zinc-500 hover:text-zinc-300">Suites</Link>
          <ChevronRight className="h-3 w-3 text-zinc-700" />
          <span className="text-zinc-300 truncate">{suite.name}</span>
        </div>
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3 flex-1 min-w-0">
            <Boxes className="h-5 w-5 text-primary shrink-0 mt-1" />
            <div className="flex-1 min-w-0 space-y-1">
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full text-lg font-semibold bg-transparent border-0 border-b border-transparent hover:border-zinc-800 focus:border-primary/60 focus:outline-none transition-colors px-0"
              />
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={1}
                placeholder="Description — what this suite measures, who owns it"
                className="w-full text-xs font-mono text-zinc-400 bg-transparent border-0 border-b border-transparent hover:border-zinc-800 focus:border-primary/60 focus:outline-none transition-colors px-0 resize-y placeholder:text-zinc-700"
              />
            </div>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <Link
              to={`/runs?suite=${id}`}
              className="inline-flex items-center gap-1.5 border border-zinc-800 hover:border-zinc-600 px-2.5 py-1 text-[11px] font-mono text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              <Filter className="h-3 w-3" />
              All Runs
            </Link>
            <Button variant="outline" size="sm" onClick={handleSave} disabled={saving}>
              <Save className="h-3.5 w-3.5" />
              {saving ? "Saving..." : "Save"}
            </Button>
            <Button size="sm" onClick={handleLaunch} disabled={launching || items.length === 0}>
              <Play className="h-3.5 w-3.5" />
              {launching ? "Launching..." : "Launch Suite"}
            </Button>
          </div>
        </div>
      </div>

      {message && (
        <div className={`shrink-0 mx-5 mt-3 flex items-center gap-2 text-xs p-2 border font-mono ${
          message.type === "error"
            ? "border-destructive/30 text-destructive"
            : "border-emerald-500/30 text-emerald-400"
        }`}>
          <AlertCircle className="h-3.5 w-3.5" />
          {message.text}
        </div>
      )}

      {/* Workspace — three columns: steps editor / launch + stats / history. */}
      <div className="flex-1 min-h-0 grid grid-cols-1 lg:grid-cols-12 gap-0 overflow-hidden">
        {/* Steps editor (left) */}
        <div className="lg:col-span-7 border-r border-zinc-800 overflow-y-auto p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold flex items-center gap-2">
              <ListOrdered className="h-4 w-4 text-zinc-500" />
              Steps
              <span className="text-[10px] font-mono text-zinc-600">({items.length})</span>
            </h2>
            <AddStepBar runPresets={allRunPresets} onAdd={addItem} />
          </div>

          {items.length === 0 ? (
            <div className="border border-dashed border-zinc-800 p-8 text-center space-y-2">
              <Layers className="h-5 w-5 mx-auto text-zinc-700" />
              <p className="text-xs text-zinc-500">No steps. Add a run-preset above to start.</p>
              {allRunPresets.length === 0 && (
                <p className="text-[10px] text-zinc-600">
                  No run-presets exist yet. <Link to="/run-presets" className="text-primary hover:underline">Create one</Link> from the New Run wizard.
                </p>
              )}
            </div>
          ) : (
            <div className="space-y-2">
              {items.map((it, idx) => {
                const rp = presetMap.get(it.run_preset_id);
                const dbColor = rp ? DB_COLORS[rp.db_kind as keyof typeof DB_COLORS] : null;
                const overrideText = overrideDrafts[idx] ?? (it.overrides ? JSON.stringify(it.overrides, null, 2) : "");
                const overrideValid = !overrideText.trim() || (() => {
                  try { JSON.parse(overrideText); return true; } catch { return false; }
                })();
                return (
                  <div
                    key={idx}
                    className={`border bg-[#080808] ${
                      overrideValid ? "border-zinc-800/60" : "border-destructive/40"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2 px-3 py-2 border-b border-zinc-800/40">
                      <div className="flex items-center gap-3 min-w-0">
                        <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-7 text-right shrink-0">
                          #{idx + 1}
                        </span>
                        {rp ? (
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                              <span className="text-xs text-zinc-200 truncate">{rp.name}</span>
                              <Badge variant="outline" className={`text-[9px] ${dbColor?.text || ""}`}>
                                {rp.db_kind}
                              </Badge>
                            </div>
                            <div className="text-[10px] text-zinc-500 font-mono truncate">
                              {rp.config?.stroppy?.script || "—"}
                              {rp.config?.stroppy?.duration ? ` · ${rp.config.stroppy.duration}` : ""}
                              {rp.config?.stroppy?.vus ? ` · ${rp.config.stroppy.vus} VUs` : ""}
                              {rp.config?.stroppy?.pool_size ? ` · pool ${rp.config.stroppy.pool_size}` : ""}
                              {rp.config?.stroppy?.scale_factor && rp.config.stroppy.scale_factor > 1 ? ` · scale ${rp.config.stroppy.scale_factor}` : ""}
                            </div>
                          </div>
                        ) : (
                          <span className="text-xs text-destructive font-mono">
                            missing run-preset {it.run_preset_id.slice(0, 8)}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-0.5 shrink-0">
                        <button
                          onClick={() => move(idx, -1)}
                          disabled={idx === 0}
                          className="p-1 text-zinc-500 hover:text-zinc-200 disabled:opacity-30 disabled:cursor-not-allowed"
                          title="Move up"
                        >
                          <ChevronUp className="h-3.5 w-3.5" />
                        </button>
                        <button
                          onClick={() => move(idx, 1)}
                          disabled={idx === items.length - 1}
                          className="p-1 text-zinc-500 hover:text-zinc-200 disabled:opacity-30 disabled:cursor-not-allowed"
                          title="Move down"
                        >
                          <ChevronDown className="h-3.5 w-3.5" />
                        </button>
                        <button
                          onClick={() => removeItem(idx)}
                          className="p-1 text-zinc-500 hover:text-destructive"
                          title="Remove"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>

                    <details className="group">
                      <summary className="text-[10px] font-mono text-zinc-600 cursor-pointer hover:text-zinc-400 select-none px-3 py-1.5 flex items-center gap-1.5">
                        <ChevronRight className="h-3 w-3 group-open:rotate-90 transition-transform" />
                        Override (JSON patch — top-level keys merge over the preset)
                        {!overrideValid && <span className="text-destructive">— invalid JSON</span>}
                        {overrideText.trim() && overrideValid && <span className="text-emerald-500">— active</span>}
                      </summary>
                      <div className="px-3 pb-3">
                        <JsonEditor
                          value={overrideText}
                          onChange={(v) => setOverrides(idx, v)}
                          height="9rem"
                          placeholder='{"stroppy": {"vus": 200, "duration": "10m"}}'
                        />
                      </div>
                    </details>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Launch params + stats (center) */}
        <div className="lg:col-span-2 border-r border-zinc-800 overflow-y-auto p-4 space-y-4 bg-[#060606]">
          <div className="space-y-2">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              Launch
            </div>
            <div className="space-y-1">
              <label className="text-[10px] font-mono text-zinc-500">Name prefix</label>
              <Input
                value={namePrefix}
                onChange={(e) => setNamePrefix(e.target.value)}
                placeholder="(optional)"
                className="h-7 text-[11px] font-mono"
              />
            </div>
            <div className="space-y-1">
              <label className="text-[10px] font-mono text-zinc-500">Description (each run)</label>
              <textarea
                value={launchDescription}
                onChange={(e) => setLaunchDescription(e.target.value)}
                rows={3}
                placeholder="(optional)"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1 text-[11px] font-mono text-zinc-300 focus:outline-none focus:border-primary/60 resize-none"
              />
            </div>
          </div>

          {/* Execution policy — applied at the next launch. Editable while
              a batch is in flight, but the change is snapshotted into job_runs
              at launch time so the running batch keeps its prior policy. */}
          <div className="space-y-2 border-t border-zinc-800/50 pt-3">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Policy</div>
            <div className="space-y-1">
              <label className="text-[10px] font-mono text-zinc-500">Mode</label>
              <select
                value={policy.mode}
                onChange={(e) => setPolicy((p) => ({ ...p, mode: e.target.value as "sequential" | "parallel" }))}
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1 text-[11px] font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              >
                <option value="sequential">sequential</option>
                <option value="parallel">parallel</option>
              </select>
            </div>
            {policy.mode === "parallel" && (
              <div className="space-y-1">
                <label className="text-[10px] font-mono text-zinc-500">Max parallel</label>
                <input
                  type="number"
                  min={1}
                  max={64}
                  value={policy.max_parallel}
                  onChange={(e) => setPolicy((p) => ({ ...p, max_parallel: Math.max(1, parseInt(e.target.value) || 1) }))}
                  className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1 text-[11px] font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
                />
              </div>
            )}
            <div className="space-y-1">
              <label className="text-[10px] font-mono text-zinc-500">On step fail</label>
              <select
                value={policy.on_step_fail}
                onChange={(e) => setPolicy((p) => ({ ...p, on_step_fail: e.target.value as "continue" | "stop" }))}
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1 text-[11px] font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              >
                <option value="continue">continue</option>
                <option value="stop">stop</option>
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-[10px] font-mono text-zinc-500">Step timeout (min)</label>
              <input
                type="number"
                min={0}
                max={1440}
                value={policy.step_timeout_min}
                onChange={(e) => setPolicy((p) => ({ ...p, step_timeout_min: Math.max(0, parseInt(e.target.value) || 0) }))}
                placeholder="0 = no timeout"
                className="w-full bg-[#0a0a0a] border border-zinc-800 px-2 py-1 text-[11px] font-mono text-zinc-200 focus:outline-none focus:border-primary/60"
              />
            </div>
          </div>

          <div className="space-y-2 border-t border-zinc-800/50 pt-3">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Stats</div>
            <StatRow label="Steps" value={String(items.length)} />
            <StatRow label="Total batches" value={String(batches.length)} />
            <StatRow label="Total runs" value={String(suite.runs?.length ?? 0)} />
            {Object.entries(dbKindCounts).length > 0 && (
              <div className="space-y-1">
                <div className="text-[10px] font-mono text-zinc-600">By database</div>
                {Object.entries(dbKindCounts).map(([k, n]) => {
                  const c = DB_COLORS[k as keyof typeof DB_COLORS];
                  return (
                    <div key={k} className="flex justify-between text-[11px] font-mono">
                      <span className={c?.text || "text-zinc-400"}>{k}</span>
                      <span className="text-zinc-500 tabular-nums">{n}</span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div className="space-y-2 border-t border-zinc-800/50 pt-3">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">Meta</div>
            <StatRow label="Created" value={new Date(suite.created_at).toLocaleDateString()} />
            <StatRow label="Updated" value={new Date(suite.updated_at).toLocaleDateString()} />
          </div>
        </div>

        {/* Run history (right) */}
        <div className="lg:col-span-3 overflow-y-auto p-4 space-y-3">
          <div className="flex items-center justify-between">
            <div className="text-sm font-semibold flex items-center gap-2">
              <History className="h-4 w-4 text-zinc-500" />
              History
            </div>
            {batches.length > 0 && (
              <Link
                to={`/runs?suite=${id}`}
                className="text-[10px] font-mono text-primary hover:underline inline-flex items-center gap-1"
              >
                All <ExternalLink className="h-3 w-3" />
              </Link>
            )}
          </div>
          {batches.length === 0 ? (
            <div className="text-[11px] text-zinc-600 italic">No runs yet.</div>
          ) : (
            <div className="space-y-2">
              {batches.map((b, bi) => {
                const recent = bi === 0; // only let the user cancel the latest batch
                return (
                <div key={b.batchId} className="border border-zinc-800/60 bg-[#080808]">
                  <div className="flex items-center justify-between px-2 py-1.5 border-b border-zinc-800/40">
                    <span className="text-[10px] font-mono text-zinc-400">
                      {new Date(b.createdAt).toLocaleString()}
                    </span>
                    <div className="flex items-center gap-2">
                      {recent && id && (
                        <button
                          onClick={async () => {
                            if (!id) return;
                            try {
                              const r = await cancelBatch(id, b.batchId);
                              setMessage({ type: "success", text: `Cancelled ${r.cancelled} job(s)` });
                              load();
                            } catch (err) {
                              setMessage({ type: "error", text: err instanceof Error ? err.message : "Cancel failed" });
                            }
                          }}
                          className="text-[10px] font-mono text-zinc-500 hover:text-destructive"
                          title="Cancel all queued/running jobs in this batch"
                        >
                          cancel
                        </button>
                      )}
                      <Link
                        to={`/runs?suite=${id}&batch=${b.batchId}`}
                        className="text-[10px] font-mono text-primary hover:underline"
                      >
                        batch
                      </Link>
                    </div>
                  </div>
                  <div className="divide-y divide-zinc-800/30">
                    {b.runs.map((r) => (
                      <Link
                        key={r.run_id}
                        to={`/runs/${r.run_id}`}
                        className="flex items-center justify-between px-2 py-1 hover:bg-zinc-900/40 group"
                        title={r.run_id}
                      >
                        <span className="flex items-center gap-2 min-w-0">
                          <span className="text-[10px] font-mono text-zinc-600 tabular-nums w-6 shrink-0">
                            #{r.position + 1}
                          </span>
                          <span className="text-[10px] font-mono text-zinc-300 group-hover:text-primary truncate">
                            {r.run_id.length > 18 ? r.run_id.slice(-14) : r.run_id}
                          </span>
                        </span>
                        <ExternalLink className="h-3 w-3 text-zinc-700 group-hover:text-primary shrink-0" />
                      </Link>
                    ))}
                  </div>
                </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between text-[11px] font-mono">
      <span className="text-zinc-500">{label}</span>
      <span className="text-zinc-200 tabular-nums">{value}</span>
    </div>
  );
}

function AddStepBar({ runPresets, onAdd }: {
  runPresets: RunPreset[];
  onAdd: (id: string) => void;
}) {
  const [pick, setPick] = useState("");
  return (
    <div className="flex items-center gap-2">
      <Select value={pick} onValueChange={setPick}>
        <SelectTrigger className="h-7 w-56 font-mono text-[11px]">
          <SelectValue placeholder={runPresets.length === 0 ? "No run-presets yet" : "Pick a run-preset"} />
        </SelectTrigger>
        <SelectContent>
          {runPresets.map((p) => (
            <SelectItem key={p.id} value={p.id}>
              {p.name} <span className="text-zinc-600">· {p.db_kind}</span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        size="sm"
        variant="outline"
        disabled={!pick}
        onClick={() => { onAdd(pick); setPick(""); }}
      >
        <Plus className="h-3.5 w-3.5" />
        Add
      </Button>
    </div>
  );
}
