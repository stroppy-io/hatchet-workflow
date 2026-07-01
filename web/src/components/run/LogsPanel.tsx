// Logs tab. Filtering modelled on the legacy LogStream — MultiFilter dropdowns
// (Step / Component / Machine / Unit) with cross-filtered counts + machine
// colours, inline search, wrap toggle, live tail (stream + auto-follow), and
// cursor-based infinite "load older" on scroll-up. All filters + search live in
// the URL query (?steps=&comp=&mach=&unit=&q=) so any filtered view is a
// shareable link; the pipeline "view in logs" jump writes the same params.
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowDownToLine, ArrowUpToLine, Check, ChevronDown, ChevronUp, Cpu, FileText, Link2, Radio, Search, Server, Tags, Terminal, WrapText, X, Zap } from "lucide-react";
import { DateTimePicker } from "@/components/ui/datetime-picker";
import { useSearchParams } from "@/lib/router";
import { Button } from "@/components/ui/button";
import { MultiFilter, type FilterOption } from "@/components/ui/multi-filter";
import { cn } from "@/lib/utils";
import { Source, Stream } from "@/lib/proto/cloud/v1/monitor/logs_pb";
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

const SOURCE_FILTERS = [
  { token: "command", value: Source.COMMAND, label: "Command", json: "SOURCE_COMMAND" },
  { token: "journald", value: Source.JOURNALD, label: "Journald", json: "SOURCE_JOURNALD" },
  { token: "file", value: Source.FILE, label: "File", json: "SOURCE_FILE" },
  { token: "server", value: Source.SERVER, label: "Server", json: "SOURCE_SERVER" },
] as const;

const STREAM_FILTERS = [
  { token: "stdout", value: Stream.STDOUT, label: "Stdout", json: "STREAM_STDOUT" },
  { token: "stderr", value: Stream.STDERR, label: "Stderr", json: "STREAM_STDERR" },
] as const;

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

function selectedSources(values: Set<string>): Source[] {
  return SOURCE_FILTERS.filter((s) => values.has(s.token)).map((s) => s.value);
}

function selectedStreams(values: Set<string>): Stream[] {
  return STREAM_FILTERS.filter((s) => values.has(s.token)).map((s) => s.value);
}

function sourceToken(value: string): string {
  return SOURCE_FILTERS.find((s) => s.json === value || s.token === value.toLowerCase())?.token ?? "";
}

function streamToken(value: string): string {
  return STREAM_FILTERS.find((s) => s.json === value || s.token === value.toLowerCase())?.token ?? "";
}

function sourceLabel(value: string): string {
  const token = sourceToken(value);
  return SOURCE_FILTERS.find((s) => s.token === token)?.label ?? "";
}

function streamLabel(value: string): string {
  const token = streamToken(value);
  return STREAM_FILTERS.find((s) => s.token === token)?.label ?? "";
}

// Live-tail buffer cap: at a firehose rate (500+ lines/s) an unbounded buffer
// makes appendUnique O(n^2) and renders thousands of un-virtualized rows, which
// freezes the tab. Cap the in-memory tail; scroll-up "load older" still pages
// history from the server, and pausing Live freezes the buffer for reading.
const MAX_LIVE_LINES = 5000;
// Coalesce streamed lines into one state update per FLUSH_MS instead of one per
// line — the per-line setState was the dominant cost under a firehose.
const FLUSH_MS = 150;

function lineKey(l: LogLineVM): string {
  return l.cursorKey || `${l.observedAt}|${l.lineNo}|${l.machineId}|${l.line}`;
}

function appendUnique(prev: LogLineVM[], next: LogLineVM[]): LogLineVM[] {
  if (next.length === 0) return prev;
  const seen = new Set(prev.map(lineKey));
  const out = [...prev];
  for (const line of next) {
    const key = lineKey(line);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(line);
  }
  return out;
}

function prependUnique(prev: LogLineVM[], next: LogLineVM[]): LogLineVM[] {
  if (next.length === 0) return prev;
  const seen = new Set(prev.map(lineKey));
  const out: LogLineVM[] = [];
  for (const line of next) {
    const key = lineKey(line);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(line);
  }
  return out.length ? [...out, ...prev] : prev;
}

function logClock(iso: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleTimeString(undefined, { hour12: false });
}

function logLineTooltip(l: LogLineVM, source: string, stream: string): string {
  return [
    l.observedAt && `time: ${l.observedAt}`,
    l.machineId && `machine: ${l.machineId}`,
    l.componentId && `component: ${l.componentId}`,
    source && `source: ${source}`,
    stream && `stream: ${stream}`,
    l.unit && `unit: ${l.unit}`,
    l.action && `action: ${l.action}`,
    l.stepId && `step: ${l.stepId}`,
    l.phase && `phase: ${l.phase}`,
    l.stageName && `stage: ${l.stageName}`,
    l.nodeExecutionId && `node: ${l.nodeExecutionId}`,
    l.mentions.length > 0 && `mentions: ${l.mentions.join(", ")}`,
  ]
    .filter(Boolean)
    .join("\n");
}

export function LogsPanel({ tenantSlug, runId, pipeline }: LogsPanelProps) {
  const [sp, setSp] = useSearchParams();

  // Filters derived from the URL — single source of truth (shareable).
  const stepsParam = sp.get("steps");
  const compParam = sp.get("comp");
  const machParam = sp.get("mach");
  const unitParam = sp.get("unit");
  const srcParam = sp.get("src");
  const streamParam = sp.get("stream");
  const phaseParam = sp.get("phase");
  const actionParam = sp.get("action");
  const mentionParam = sp.get("mention");
  // Time window (datetime-local strings, e.g. "2026-06-26T15:30"). A set window
  // pins the view to history, so live tail is forced off while it is active.
  const fromTs = sp.get("from_ts") ?? "";
  const toTs = sp.get("to_ts") ?? "";
  const hasRange = !!fromTs || !!toTs;
  const steps = useMemo(() => new Set(csv(stepsParam)), [stepsParam]);
  const components = useMemo(() => new Set(csv(compParam)), [compParam]);
  const machines = useMemo(() => new Set(csv(machParam)), [machParam]);
  const units = useMemo(() => new Set(csv(unitParam)), [unitParam]);
  const sources = useMemo(() => new Set(csv(srcParam)), [srcParam]);
  const streams = useMemo(() => new Set(csv(streamParam)), [streamParam]);
  const phases = useMemo(() => new Set(csv(phaseParam)), [phaseParam]);
  const actions = useMemo(() => new Set(csv(actionParam)), [actionParam]);
  const mentions = useMemo(() => new Set(csv(mentionParam)), [mentionParam]);
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
        for (const k of ["steps", "comp", "mach", "unit", "src", "stream", "phase", "action", "mention", "q", "from_ts", "to_ts"]) next.delete(k);
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
  const [loadingNewer, setLoadingNewer] = useState(false);
  // Live tail off when arriving on a line anchor (so it doesn't jump to bottom).
  const [live, setLive] = useState(!anchorKey);
  const [wrap, setWrap] = useState(true);
  const [copied, setCopied] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const olderCursor = useRef<unknown>(undefined);
  const newerCursor = useRef<unknown>(undefined);
  const [loadedFilterKey, setLoadedFilterKey] = useState("");
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
  const machineIds = useMemo(() => Array.from(machines), [machines]);
  const unitIds = useMemo(() => Array.from(units), [units]);
  const sourceIds = useMemo(() => Array.from(sources), [sources]);
  const streamIds = useMemo(() => Array.from(streams), [streams]);
  const phaseIds = useMemo(() => Array.from(phases), [phases]);
  const actionIds = useMemo(() => Array.from(actions), [actions]);
  const mentionIds = useMemo(() => Array.from(mentions), [mentions]);

  const serverFilter = useCallback(
    () => ({
      search: applied.trim() || undefined,
      nodeExecutionIds: stepIds.length ? stepIds : undefined,
      componentIds: compIds.length ? compIds : undefined,
      machineIds: machineIds.length ? machineIds : undefined,
      units: unitIds.length ? unitIds : undefined,
      sources: sourceIds.length ? selectedSources(sources) : undefined,
      streams: streamIds.length ? selectedStreams(streams) : undefined,
      phases: phaseIds.length ? phaseIds : undefined,
      actions: actionIds.length ? actionIds : undefined,
      mentions: mentionIds.length ? mentionIds : undefined,
      start: fromTs ? new Date(fromTs) : undefined,
      end: toTs ? new Date(toTs) : undefined,
    }),
    [applied, stepIds, compIds, machineIds, unitIds, sourceIds, sources, streamIds, streams, phaseIds, actionIds, mentionIds, fromTs, toTs],
  );

  const filterKey = useMemo(
    () =>
      JSON.stringify({
        q: applied.trim(),
        steps: stepIds,
        comp: compIds,
        mach: machineIds,
        unit: unitIds,
        src: sourceIds,
        stream: streamIds,
        phase: phaseIds,
        action: actionIds,
        mention: mentionIds,
        from_ts: fromTs,
        to_ts: toTs,
      }),
    [applied, stepIds, compIds, machineIds, unitIds, sourceIds, streamIds, phaseIds, actionIds, mentionIds, fromTs, toTs],
  );

  const load = useCallback(async () => {
    if (!tenantSlug || !runId) return;
    setLoading(true);
    setLoadedFilterKey("");
    setError(null);
    try {
      // First load with a #line anchor fetches the page ending at that line;
      // afterwards (and on any filter change) load the newest tail.
      const anchorCk = anchorRef.current;
      const anchor = anchorCk ? logCursorFromKey(anchorCk) : undefined;
      anchorRef.current = null;
      const page = anchor
        ? await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "older", from: anchor, limit: 300 })
        : await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "older", limit: 300 });
      let lines = page.lines;
      olderCursor.current = page.older;
      newerCursor.current = page.newer;
      // When opening a shared line link, also pull a page of newer lines so the
      // anchored line itself (and some context after it) is in the buffer — the
      // older-direction page can end just before the anchor, leaving nothing to
      // scroll to / highlight.
      if (anchor) {
        try {
          const after = await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "newer", from: anchor, limit: 150 });
          if (after.lines.length) {
            lines = appendUnique(lines, after.lines);
            newerCursor.current = after.newer;
          }
        } catch {
          /* best-effort context fetch */
        }
      }
      setLines(lines);
      setLoadedFilterKey(filterKey);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [tenantSlug, runId, serverFilter, filterKey]);

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
        setLines((prev) => prependUnique(prev, page.lines));
      }
      olderCursor.current = page.older;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingOlder(false);
    }
  }, [tenantSlug, runId, serverFilter, loadingOlder]);

  // Scroll-DOWN paging forward in time — needed when NOT live-tailing (e.g. after
  // jump-to-top, or reading a historical time range), where the live stream is
  // not appending newer lines for us.
  const loadNewer = useCallback(async () => {
    if (!tenantSlug || !runId || loadingNewer || !newerCursor.current) return;
    setLoadingNewer(true);
    try {
      const page = await queryLogs(tenantSlug, runId, {
        ...serverFilter(),
        direction: "newer",
        from: newerCursor.current,
        limit: 300,
      });
      if (page.lines.length) setLines((prev) => appendUnique(prev, page.lines));
      newerCursor.current = page.newer;
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoadingNewer(false);
    }
  }, [tenantSlug, runId, serverFilter, loadingNewer]);

  // Refetch on server-filter change.
  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenantSlug, runId, filterKey]);

  // Live tail. Streamed lines are buffered in pendingRef and flushed to state on
  // a timer (one re-render per FLUSH_MS, capped to MAX_LIVE_LINES) so a firehose
  // does not lock up the tab.
  const pendingRef = useRef<LogLineVM[]>([]);
  useEffect(() => {
    // A time window pins the view to history — no live tail while it's active.
    if (!live || hasRange || !tenantSlug || !runId || loadedFilterKey !== filterKey) return;
    const controller = new AbortController();
    tailAbort.current = controller;
    pendingRef.current = [];

    const flush = () => {
      const batch = pendingRef.current;
      if (batch.length === 0) return;
      pendingRef.current = [];
      setLines((prev) => {
        const merged = appendUnique(prev, batch);
        return merged.length > MAX_LIVE_LINES ? merged.slice(merged.length - MAX_LIVE_LINES) : merged;
      });
    };
    const timer = window.setInterval(flush, FLUSH_MS);

    void streamLogs(
      tenantSlug,
      runId,
      {
        ...serverFilter(),
        from: newerCursor.current,
      },
      controller.signal,
      (line) => {
        if (controller.signal.aborted) return;
        if (line.cursorKey) newerCursor.current = logCursorFromKey(line.cursorKey);
        pendingRef.current.push(line);
      },
    ).catch((err) => {
      if (controller.signal.aborted) return;
      setError(err instanceof Error ? err.message : String(err));
    });
    return () => {
      controller.abort();
      window.clearInterval(timer);
      pendingRef.current = [];
      if (tailAbort.current === controller) tailAbort.current = null;
    };
  }, [live, hasRange, tenantSlug, runId, loadedFilterKey, filterKey, serverFilter]);

  const rows = lines;

  // Follow to the bottom on new rows while live AND pinned to bottom. Never
  // follow while a load-older prepend is in flight (prependAnchor set) or the
  // older-page indicator is showing, otherwise prepending history yanks the
  // viewport to the bottom instead of holding the reader's position.
  useEffect(() => {
    if (!live || !atBottom.current || loadingOlder || prependAnchor.current != null) return;
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [rows, live, loadingOlder]);

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

  // After a jump-to-oldest/newest replaced the buffer, land at the requested end.
  useLayoutEffect(() => {
    if (!jumpTargetRef.current) return;
    const el = scrollRef.current;
    if (el) {
      if (jumpTargetRef.current === "top") {
        el.scrollTop = 0;
        atBottom.current = false;
      } else {
        el.scrollTop = el.scrollHeight;
        atBottom.current = true;
      }
    }
    jumpTargetRef.current = null;
  }, [rows]);

  const onScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    if (el.scrollTop < 80 && olderCursor.current && !loadingOlder) void loadOlder();
    // Page forward on scroll-down only when not live-tailing (live already
    // appends newer lines); the live stream owns the tail otherwise.
    if (!live && el.scrollHeight - el.scrollTop - el.clientHeight < 80 && newerCursor.current && !loadingNewer) {
      void loadNewer();
    }
  }, [loadingOlder, loadOlder, live, loadingNewer, loadNewer]);

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
  const { stepOpts, compOpts, machineOpts, unitOpts, sourceOpts, streamOpts, phaseOpts, actionOpts, mentionOpts } = useMemo(() => {
    const sc: Record<string, number> = {};
    const cc: Record<string, number> = {};
    const mc: Record<string, number> = {};
    const uc: Record<string, number> = {};
    const soc: Record<string, number> = {};
    const stc: Record<string, number> = {};
    const pc: Record<string, number> = {};
    const ac: Record<string, number> = {};
    const xc: Record<string, number> = {};
    for (const l of lines) {
      if (l.nodeExecutionId) sc[l.nodeExecutionId] = (sc[l.nodeExecutionId] || 0) + 1;
      if (l.componentId) cc[l.componentId] = (cc[l.componentId] || 0) + 1;
      if (l.machineId) mc[l.machineId] = (mc[l.machineId] || 0) + 1;
      if (l.unit) uc[l.unit] = (uc[l.unit] || 0) + 1;
      const src = sourceToken(l.source);
      if (src) soc[src] = (soc[src] || 0) + 1;
      const str = streamToken(l.stream);
      if (str) stc[str] = (stc[str] || 0) + 1;
      if (l.phase) pc[l.phase] = (pc[l.phase] || 0) + 1;
      if (l.action) ac[l.action] = (ac[l.action] || 0) + 1;
      for (const m of l.mentions) xc[m] = (xc[m] || 0) + 1;
    }
    const color = (id: string) => {
      if (!colorMap.current.has(id)) colorMap.current.set(id, MACHINE_COLORS[colorMap.current.size % MACHINE_COLORS.length]);
      return colorMap.current.get(id)!;
    };
    return {
      stepOpts: nodeOptions
        .filter((n) => sc[n.id] || steps.has(n.id))
        .map((n): FilterOption => ({ value: n.id, label: n.label, count: sc[n.id] || 0 })),
      compOpts: Array.from(new Set([...Object.keys(cc), ...components])).sort().map((c): FilterOption => ({ value: c, label: c, count: cc[c] || 0 })),
      machineOpts: Array.from(new Set([...Object.keys(mc), ...machines])).sort().map((m): FilterOption => ({ value: m, label: m, count: mc[m] || 0, color: color(m) })),
      unitOpts: Array.from(new Set([...Object.keys(uc), ...units])).sort().map((u): FilterOption => ({ value: u, label: u.replace(/\.service$/, ""), count: uc[u] || 0 })),
      sourceOpts: SOURCE_FILTERS.map((s): FilterOption => ({ value: s.token, label: s.label, count: soc[s.token] || 0 })),
      streamOpts: STREAM_FILTERS.map((s): FilterOption => ({ value: s.token, label: s.label, count: stc[s.token] || 0 })),
      phaseOpts: Array.from(new Set([...Object.keys(pc), ...phases])).sort().map((p): FilterOption => ({ value: p, label: humanize(p), count: pc[p] || 0 })),
      actionOpts: Array.from(new Set([...Object.keys(ac), ...actions])).sort().map((a): FilterOption => ({ value: a, label: humanize(a), count: ac[a] || 0 })),
      mentionOpts: Array.from(new Set([...Object.keys(xc), ...mentions])).sort().map((m): FilterOption => ({ value: m, label: m, count: xc[m] || 0 })),
    };
  }, [lines, nodeOptions, steps, components, machines, units, phases, actions, mentions]);

  const totalActive = steps.size + components.size + machines.size + units.size + sources.size + streams.size + phases.size + actions.size + mentions.size + (applied ? 1 : 0) + (fromTs ? 1 : 0) + (toTs ? 1 : 0);

  // Jump to the absolute oldest / newest of ALL logs (not just the loaded
  // buffer): re-query the boundary page under the current filters, replace the
  // buffer, reset the paging cursors, then scroll (via jumpTargetRef, once the
  // new rows render).
  const jumpTargetRef = useRef<null | "top" | "bottom">(null);
  const jumpTop = useCallback(async () => {
    if (!tenantSlug || !runId) return;
    setLive(false); // viewing history from the very start
    setError(null);
    try {
      const page = await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "newer", limit: 300 });
      olderCursor.current = undefined; // nothing older than the oldest page
      newerCursor.current = page.newer;
      jumpTargetRef.current = "top";
      setLines(page.lines);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [tenantSlug, runId, serverFilter]);
  const jumpBottom = useCallback(async () => {
    if (!tenantSlug || !runId) return;
    setError(null);
    try {
      const page = await queryLogs(tenantSlug, runId, { ...serverFilter(), direction: "older", limit: 300 });
      olderCursor.current = page.older;
      newerCursor.current = page.newer;
      jumpTargetRef.current = "bottom";
      setLines(page.lines);
      if (!hasRange) setLive(true); // resume following the tail
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [tenantSlug, runId, serverFilter, hasRange]);

  // Page up/down by ~500 lines. Scrolls the viewport; the edge handlers in
  // onScroll then page older/newer from the server as needed.
  const LINES_PER_PAGE = 500;
  const pageBy = useCallback((dir: -1 | 1) => {
    const el = scrollRef.current;
    if (!el) return;
    const first = el.querySelector<HTMLElement>("[data-ck]");
    const rowPx = first?.offsetHeight || 18;
    el.scrollTop += dir * LINES_PER_PAGE * rowPx;
  }, []);

  // Keyboard nav on the focused log body: Home/End = absolute oldest/newest,
  // PageUp/PageDown = ±500 lines.
  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Home") { e.preventDefault(); void jumpTop(); }
      else if (e.key === "End") { e.preventDefault(); void jumpBottom(); }
      else if (e.key === "PageUp") { e.preventDefault(); pageBy(-1); }
      else if (e.key === "PageDown") { e.preventDefault(); pageBy(1); }
    },
    [jumpTop, jumpBottom, pageBy],
  );

  return (
    <div className="flex h-full flex-col gap-2">
      {/* Row 1 — filters + time window (+ clear). */}
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
        <MultiFilter icon={<Terminal className="h-3 w-3" />} label="Source" options={sourceOpts} selected={sources} onChange={setSetParam("src")} />
        <MultiFilter icon={<Radio className="h-3 w-3" />} label="Stream" options={streamOpts} selected={streams} onChange={setSetParam("stream")} />
        {phaseOpts.length > 0 && (
          <MultiFilter icon={<Radio className="h-3 w-3" />} label="Phase" options={phaseOpts} selected={phases} onChange={setSetParam("phase")} />
        )}
        {actionOpts.length > 0 && (
          <MultiFilter icon={<FileText className="h-3 w-3" />} label="Action" options={actionOpts} selected={actions} onChange={setSetParam("action")} />
        )}
        {mentionOpts.length > 0 && (
          <MultiFilter icon={<Tags className="h-3 w-3" />} label="Mention" options={mentionOpts} selected={mentions} onChange={setSetParam("mention")} />
        )}

        {totalActive > 0 && (
          <button
            type="button"
            onClick={clearFilters}
            className="flex items-center gap-1 rounded border border-border px-2 py-0.5 font-mono text-[10px] uppercase tracking-wider text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
          >
            <X className="h-3 w-3" /> Clear
          </button>
        )}
      </div>

      {/* Row 2 — search + navigation. */}
      <div className="flex shrink-0 items-center gap-2">
        <div className="flex items-center gap-1 rounded border border-border px-2 py-0.5 font-mono text-[11px] transition-colors focus-within:border-foreground/40">
          <Search className="h-3 w-3 shrink-0 text-muted-foreground" />
          <input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && setParam("q", searchInput.trim() ? [searchInput.trim()] : [])}
            placeholder="Search logs…"
            className="w-40 bg-transparent text-foreground outline-none transition-all placeholder:text-muted-foreground focus:w-56"
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

        {/* Time window (from–to), 24-hour shadcn pickers. A set range pins the
            view to history (live tail off). */}
        <DateTimePicker
          value={fromTs ? new Date(fromTs) : undefined}
          onChange={(d) => setParam("from_ts", d ? [d.toISOString()] : [])}
          placeholder="From"
        />
        <span className="text-muted-foreground">–</span>
        <DateTimePicker
          value={toTs ? new Date(toTs) : undefined}
          onChange={(d) => setParam("to_ts", d ? [d.toISOString()] : [])}
          placeholder="To"
        />

        <div className="flex-1" />

        <span className="shrink-0 font-mono text-[10px] text-muted-foreground">
          {totalActive > 0 ? `${rows.length.toLocaleString()}/` : ""}
          {lines.length.toLocaleString()} lines
        </span>

        {/* Navigation: absolute start/end (Home/End) + ±500 lines (PgUp/PgDown). */}
        <Button variant="outline" size="sm" onClick={jumpTop} title="To start — oldest of all logs (Home)">
          <ArrowUpToLine className="h-4 w-4" />
        </Button>
        <Button variant="outline" size="sm" onClick={() => pageBy(-1)} title="Up 500 lines (PgUp)">
          <ChevronUp className="h-4 w-4" />
        </Button>
        <Button variant="outline" size="sm" onClick={() => pageBy(1)} title="Down 500 lines (PgDown)">
          <ChevronDown className="h-4 w-4" />
        </Button>
        <Button variant="outline" size="sm" onClick={jumpBottom} title="To end — newest of all logs (End)">
          <ArrowDownToLine className="h-4 w-4" />
        </Button>

        <Button
          variant={live && !hasRange ? "default" : "outline"}
          size="sm"
          disabled={hasRange}
          onClick={() => setLive((v) => !v)}
          title={hasRange ? "Live tail disabled while a time range is set" : "Stream new lines and follow to the bottom"}
        >
          <Radio className={cn("h-4 w-4", live && !hasRange && "animate-pulse")} /> Live
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
        onKeyDown={onKeyDown}
        tabIndex={0}
        className="min-h-0 flex-1 overflow-auto border border-border bg-black/40 font-mono text-[11px] leading-relaxed outline-none"
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
            const src = sourceLabel(l.source);
            const str = streamLabel(l.stream);
            const stderr = streamToken(l.stream) === "stderr";
            const tooltip = logLineTooltip(l, src, str);
            const showComponent = !!l.componentId && l.componentId !== l.machineId;
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
                    "relative w-10 shrink-0 select-none pr-1 text-right tabular-nums transition-colors",
                    isCopied ? "text-success" : "text-muted-foreground/50 hover:text-primary",
                  )}
                >
                  {isCopied ? (
                    <Check className="h-3 w-3" />
                  ) : (
                    <>
                      {/* Monotonic gutter index over the loaded buffer — the
                          server lineNo is a per-node counter and reads as
                          non-monotonic garbage when nodes are interleaved. */}
                      <span className="group-hover:invisible">{i + 1}</span>
                      <Link2 className="invisible absolute inset-0 m-auto h-3 w-3 group-hover:visible" />
                    </>
                  )}
                </button>
                {l.observedAt && (
                  <span className="shrink-0 select-none text-muted-foreground/55" title={l.observedAt}>
                    {logClock(l.observedAt)}
                  </span>
                )}
                {l.machineId && (
                  <span className={cn("max-w-[10rem] shrink-0 truncate", mColor)} title={l.machineId}>
                    [{l.machineId}]
                  </span>
                )}
                {showComponent && (
                  <span className="max-w-[12rem] shrink-0 truncate text-muted-foreground/70" title={`component: ${l.componentId}`}>
                    {l.componentId}
                  </span>
                )}
                <span className={cn("min-w-0", wrap && "[overflow-wrap:anywhere]", stderr && "text-destructive")} title={tooltip || undefined}>
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
