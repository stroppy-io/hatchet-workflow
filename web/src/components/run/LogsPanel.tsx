// Logs tab. Filtering modelled on the legacy LogStream — MultiFilter dropdowns
// (Step / Component / Machine / Unit) with cross-filtered counts + machine
// colours, inline search, wrap toggle, live tail (stream + auto-follow), and
// cursor-based infinite "load older" on scroll-up. All filters + search live in
// the URL query (?steps=&comp=&mach=&unit=&q=) so any filtered view is a
// shareable link; the pipeline "view in logs" jump writes the same params.
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Check, Cpu, FileText, Link2, Radio, Search, Server, WrapText, X, Zap } from "lucide-react";
import { useSearchParams } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { MultiFilter, type FilterOption } from "@/components/ui/multi-filter";
import { cn } from "@/lib/utils";
import {
  logCursorFromKey,
  queryLogs,
  streamLogs,
  type LogLineVM,
  type PipelineNodeVM,
} from "@/services/run_overview";

interface LogsPanelProps {
  tenantSlug: string;
  runId: string;
  pipeline: PipelineNodeVM[];
}

const MACHINE_COLORS = [
  "text-cyan-400", "text-amber-400", "text-emerald-400", "text-pink-400",
  "text-orange-400", "text-violet-400", "text-blue-400", "text-rose-400",
];

function humanize(name: string): string {
  return name.replace(/[_-]+/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

interface NodeOption {
  id: string;
  label: string;
}
function flattenNodes(nodes: PipelineNodeVM[], out: NodeOption[] = []): NodeOption[] {
  for (const n of nodes) {
    if (n.nodeExecutionId) out.push({ id: n.nodeExecutionId, label: humanize(n.name) || n.nodeExecutionId.slice(0, 8) });
    if (n.children.length) flattenNodes(n.children, out);
  }
  return out;
}

function Highlight({ text, q }: { text: string; q: string }) {
  if (!q) return <>{text}</>;
  const idx = text.toLowerCase().indexOf(q.toLowerCase());
  if (idx < 0) return <>{text}</>;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="rounded-sm bg-warning/30 px-px text-foreground">{text.slice(idx, idx + q.length)}</mark>
      {text.slice(idx + q.length)}
    </>
  );
}

const csv = (v: string | null): string[] => (v ? v.split(",").filter(Boolean) : []);

export function LogsPanel({ tenantSlug, runId, pipeline }: LogsPanelProps) {
  const [sp, setSp] = useSearchParams();

  // Filters derived from the URL — single source of truth (shareable).
  const steps = useMemo(() => new Set(csv(sp.get("steps"))), [sp]);
  const components = useMemo(() => new Set(csv(sp.get("comp"))), [sp]);
  const machines = useMemo(() => new Set(csv(sp.get("mach"))), [sp]);
  const units = useMemo(() => new Set(csv(sp.get("unit"))), [sp]);
  const applied = sp.get("q") ?? "";

  const setParam = useCallback(
    (key: string, values: string[]) =>
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (values.length) next.set(key, values.join(","));
          else next.delete(key);
          return next;
        },
        { replace: true },
      ),
    [setSp],
  );
  const setSetParam = (key: string) => (s: Set<string>) => setParam(key, Array.from(s));

  const clearFilters = useCallback(() => {
    setSp(
      (prev) => {
        const next = new URLSearchParams(prev);
        for (const k of ["steps", "comp", "mach", "unit", "q"]) next.delete(k);
        return next;
      },
      { replace: true },
    );
  }, [setSp]);

  // Stable line anchor from a shared deep-link (?line=<cursorKey>): highlights
  // and re-fetches the page around that exact server line.
  const anchorKey = sp.get("line");
  const anchorRef = useRef<string | null>(anchorKey); // consumed on first load

  const [lines, setLines] = useState<LogLineVM[]>([]);
  const [searchInput, setSearchInput] = useState(applied);
  const [loading, setLoading] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  // Live tail off when arriving on a line anchor (so it doesn't jump to bottom).
  const [live, setLive] = useState(!anchorKey);
  const [wrap, setWrap] = useState(true);
  const [copied, setCopied] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const olderCursor = useRef<unknown>(undefined);
  const tailAbort = useRef<AbortController | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const colorMap = useRef(new Map<string, string>());
  const atBottom = useRef(true);
  const prependAnchor = useRef<number | null>(null);
  const didAnchorScroll = useRef(false);

  // Copy a stable, cross-client deep-link to a specific line (by its cursor).
  // Does NOT mutate the current URL — copying shouldn't pin/highlight the line
  // here; the anchor only applies when someone opens the shared link.
  const copyLineLink = useCallback(
    (cursorKey: string) => {
      if (!cursorKey) return;
      const next = new URLSearchParams(sp);
      next.set("line", cursorKey);
      const url = `${window.location.origin}${window.location.pathname}?${next.toString()}`;
      void navigator.clipboard.writeText(url).then(() => {
        setCopied(cursorKey);
        window.setTimeout(() => setCopied((c) => (c === cursorKey ? null : c)), 1500);
      });
    },
    [sp],
  );

  // Keep the search box in sync if the URL changes underneath us (e.g. jump).
  useEffect(() => setSearchInput(applied), [applied]);

  const nodeOptions = useMemo(() => flattenNodes(pipeline), [pipeline]);
  const stepIds = useMemo(() => Array.from(steps), [steps]);
  const compIds = useMemo(() => Array.from(components), [components]);

  const serverFilter = useCallback(
    () => ({
      search: applied.trim() || undefined,
      nodeExecutionIds: stepIds.length ? stepIds : undefined,
      componentIds: compIds.length ? compIds : undefined,
    }),
    [applied, stepIds, compIds],
  );

  const load = useCallback(async () => {
    if (!tenantSlug || !runId) return;
    setLoading(true);
    setError(null);
    try {
      // First load with a #line anchor fetches the page ending at that line;
      // afterwards (and on any filter change) load the newest tail.
      const anchor = anchorRef.current ? logCursorFromKey(anchorRef.current) : undefined;
      anchorRef.current = null;
      const page = anchor
        ? await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "older", from: anchor, limit: 300 })
        : await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "newer", limit: 300 });
      setLines(page.lines);
      olderCursor.current = page.older;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [tenantSlug, runId, serverFilter]);

  const loadOlder = useCallback(async () => {
    if (!tenantSlug || !runId || loadingOlder || !olderCursor.current) return;
    setLoadingOlder(true);
    try {
      const page = await queryLogs(tenantSlug, runId, {
        ...serverFilter(),
        direction: "older",
        from: olderCursor.current,
        limit: 300,
      });
      if (page.lines.length) {
        prependAnchor.current = scrollRef.current?.scrollHeight ?? null;
        setLines((prev) => [...page.lines, ...prev]);
      }
      olderCursor.current = page.older;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingOlder(false);
    }
  }, [tenantSlug, runId, serverFilter, loadingOlder]);

  // Refetch on server-filter change.
  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug, runId, applied, stepIds.join(","), compIds.join(",")]);

  // Live tail.
  useEffect(() => {
    if (!live || !tenantSlug || !runId) return;
    const controller = new AbortController();
    tailAbort.current = controller;
    void streamLogs(
      tenantSlug,
      runId,
      {
        search: applied.trim() || undefined,
        nodeExecutionIds: stepIds.length ? stepIds : undefined,
        componentIds: compIds.length ? compIds : undefined,
      },
      controller.signal,
      (line) => {
        if (controller.signal.aborted) return;
        setLines((prev) => [...prev, line]);
      },
    ).catch((err) => {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      controller.abort();
      if (tailAbort.current === controller) tailAbort.current = null;
    };
  }, [live, tenantSlug, runId, applied, stepIds, compIds]);

  // Client-side machine + unit filter over the loaded buffer.
  const rows = useMemo(() => {
    const hasM = machines.size > 0;
    const hasU = units.size > 0;
    if (!hasM && !hasU) return lines;
    return lines.filter((l) => {
      if (hasM && !machines.has(l.machineId)) return false;
      if (hasU && !units.has(l.unit)) return false;
      return true;
    });
  }, [lines, machines, units]);

  // Follow to the bottom on new rows while live AND pinned to bottom.
  useEffect(() => {
    if (!live || !atBottom.current) return;
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [rows, live]);

  useEffect(() => {
    if (!live) return;
    const el = scrollRef.current;
    if (el) {
      el.scrollTop = el.scrollHeight;
      atBottom.current = true;
    }
  }, [live]);

  // After a prepend (load-older), keep the viewport on the same line.
  useLayoutEffect(() => {
    if (prependAnchor.current == null) return;
    const el = scrollRef.current;
    if (el) el.scrollTop += el.scrollHeight - prependAnchor.current;
    prependAnchor.current = null;
  }, [rows]);

  const onScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    if (el.scrollTop < 80 && olderCursor.current && !loadingOlder) void loadOlder();
  }, [loadingOlder, loadOlder]);

  // Scroll to + keep highlight on the deep-linked line once it's in the buffer.
  useEffect(() => {
    if (didAnchorScroll.current || !anchorKey || rows.length === 0) return;
    const el = scrollRef.current?.querySelector<HTMLElement>(`[data-ck="${CSS.escape(anchorKey)}"]`);
    if (el) {
      didAnchorScroll.current = true;
      el.scrollIntoView({ block: "center" });
    }
  }, [anchorKey, rows]);

  // Cross-filtered option counts over the loaded buffer.
  const { stepOpts, compOpts, machineOpts, unitOpts } = useMemo(() => {
    const sc: Record<string, number> = {};
    const cc: Record<string, number> = {};
    const mc: Record<string, number> = {};
    const uc: Record<string, number> = {};
    const hasM = machines.size > 0;
    const hasU = units.size > 0;
    for (const l of lines) {
      if (l.nodeExecutionId) sc[l.nodeExecutionId] = (sc[l.nodeExecutionId] || 0) + 1;
      if (l.componentId) cc[l.componentId] = (cc[l.componentId] || 0) + 1;
      if (l.machineId && (!hasU || units.has(l.unit))) mc[l.machineId] = (mc[l.machineId] || 0) + 1;
      if (l.unit && (!hasM || machines.has(l.machineId))) uc[l.unit] = (uc[l.unit] || 0) + 1;
    }
    const color = (id: string) => {
      if (!colorMap.current.has(id)) colorMap.current.set(id, MACHINE_COLORS[colorMap.current.size % MACHINE_COLORS.length]);
      return colorMap.current.get(id)!;
    };
    return {
      stepOpts: nodeOptions
        .filter((n) => sc[n.id] || steps.has(n.id))
        .map((n): FilterOption => ({ value: n.id, label: n.label, count: sc[n.id] || 0 })),
      compOpts: Object.keys(cc).sort().map((c): FilterOption => ({ value: c, label: c, count: cc[c] })),
      machineOpts: Object.keys(mc).sort().map((m): FilterOption => ({ value: m, label: m, count: mc[m], color: color(m) })),
      unitOpts: Object.keys(uc).sort().map((u): FilterOption => ({ value: u, label: u.replace(/\.service$/, ""), count: uc[u] })),
    };
  }, [lines, nodeOptions, steps, machines, units]);

  const totalActive = steps.size + components.size + machines.size + units.size + (applied ? 1 : 0);

  return (
    <div className="flex h-full flex-col gap-2">
      {/* Toolbar */}
      <div className="flex shrink-0 flex-wrap items-center gap-2">
        {stepOpts.length > 0 && (
          <MultiFilter icon={<Zap className="h-3 w-3" />} label="Step" options={stepOpts} selected={steps} onChange={setSetParam("steps")} />
        )}
        {compOpts.length > 0 && (
          <MultiFilter icon={<FileText className="h-3 w-3" />} label="Component" options={compOpts} selected={components} onChange={setSetParam("comp")} />
        )}
        {machineOpts.length > 0 && (
          <MultiFilter icon={<Server className="h-3 w-3" />} label="Machine" options={machineOpts} selected={machines} onChange={setSetParam("mach")} />
        )}
        {unitOpts.length > 0 && (
          <MultiFilter icon={<Cpu className="h-3 w-3" />} label="Unit" options={unitOpts} selected={units} onChange={setSetParam("unit")} />
        )}

        <div className="flex items-center gap-1 rounded border border-border px-2 py-0.5 font-mono text-[11px] transition-colors focus-within:border-foreground/40">
          <Search className="h-3 w-3 shrink-0 text-muted-foreground" />
          <input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && setParam("q", searchInput.trim() ? [searchInput.trim()] : [])}
            placeholder="Search logs…"
            className="w-28 bg-transparent text-foreground outline-none transition-all placeholder:text-muted-foreground focus:w-40"
          />
          {searchInput && (
            <X
              className="h-3 w-3 shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
              onClick={() => {
                setSearchInput("");
                setParam("q", []);
              }}
            />
          )}
        </div>

        {totalActive > 0 && (
          <button
            type="button"
            onClick={clearFilters}
            className="flex items-center gap-1 rounded border border-border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
          >
            <X className="h-3 w-3" /> Clear
          </button>
        )}

        <div className="flex-1" />

        <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
          {totalActive > 0 ? `${rows.length.toLocaleString()}/` : ""}
          {lines.length.toLocaleString()} lines
        </span>

        <Button
          variant={live ? "default" : "outline"}
          size="sm"
          onClick={() => setLive((v) => !v)}
          title="Stream new lines and follow to the bottom"
        >
          <Radio className={cn("h-4 w-4", live && "animate-pulse")} /> Live
        </Button>
        <Button variant={wrap ? "default" : "outline"} size="sm" onClick={() => setWrap((v) => !v)} title="Wrap lines">
          <WrapText className="h-4 w-4" /> Wrap
        </Button>
      </div>

      {error && (
        <div className="shrink-0 border border-destructive/40 bg-destructive/10 px-3 py-1.5 text-xs text-destructive">{error}</div>
      )}

      {/* Log body — fills remaining height; scroll-up auto-loads older lines. */}
      <div
        ref={scrollRef}
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-auto border border-border bg-black/40 font-mono text-[11px] leading-relaxed"
      >
        {loadingOlder && (
          <div className="border-b border-border/50 py-1 text-center text-[10px] text-muted-foreground">Loading older…</div>
        )}
        {rows.length === 0 ? (
          <div className="p-3 text-muted-foreground">{loading ? "Loading logs…" : "No log lines."}</div>
        ) : (
          rows.map((l, i) => {
            const mColor = colorMap.current.get(l.machineId) ?? "text-muted-foreground";
            const isAnchor = !!anchorKey && l.cursorKey === anchorKey;
            const isCopied = copied === l.cursorKey;
            return (
              <div
                key={l.cursorKey || `${l.lineNo}-${i}`}
                data-ck={l.cursorKey}
                className={cn(
                  "group flex gap-2 px-1 hover:bg-foreground/[0.03]",
                  wrap ? "whitespace-pre-wrap" : "whitespace-pre",
                  isAnchor && "bg-warning/10 ring-1 ring-inset ring-warning/40",
                )}
              >
                {/* Line number; on row hover it becomes a copy-link button
                    (stable link via server cursor, not the row index). */}
                <button
                  type="button"
                  onClick={() => copyLineLink(l.cursorKey)}
                  disabled={!l.cursorKey}
                  title="Copy link to this line"
                  className={cn(
                    "relative shrink-0 select-none tabular-nums transition-colors",
                    isCopied ? "text-success" : "text-muted-foreground/50 hover:text-primary",
                  )}
                >
                  {isCopied ? (
                    <Check className="h-3 w-3" />
                  ) : (
                    <>
                      <span className="group-hover:invisible">{l.lineNo}</span>
                      <Link2 className="invisible absolute inset-0 m-auto h-3 w-3 group-hover:visible" />
                    </>
                  )}
                </button>
                {l.machineId && (
                  <span className={cn("max-w-[10rem] shrink-0 truncate", mColor)} title={l.machineId}>
                    [{l.machineId}]
                  </span>
                )}
                <span className={cn("min-w-0", l.stream === "STREAM_STDERR" && "text-destructive")}>
                  <Highlight text={l.line} q={applied.trim()} />
                </span>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
