import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowData,
} from "@tanstack/react-table";

// Per-column metadata: optional className applied to both the <th> and <td> so
// a column (e.g. the thin trigger-icon column) can carry its own width/padding.
declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className?: string;
    // `group` marks which parent header block a leaf column belongs to so the
    // body cells can carry the subtle banding that visually ties each block
    // together. Every leaf lives in one of the table-specific groups below.
    group?:
      | "controls"
      | "run"
      | "target"
      | "execution"
      | "definition"
      | "configuration"
      | "activity";
    // `groupEdge` paints the left / right divider that brackets a group: set on
    // the FIRST leaf of a block ("left"), the LAST ("right"), or "both" if a
    // block has a single member.
    groupEdge?: "left" | "right" | "both";
    // Whole-cell dead zone for utility columns: clicks in checkbox/actions
    // padding should not fall through to the row-level run navigation.
    disableRowNavigation?: boolean;
  }
}
import {
  AlertCircle,
  Ban,
  Bookmark,
  CalendarClock,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  Circle,
  CopyPlus,
  Eye,
  Filter,
  GitCompare,
  Link2,
  Loader2,
  MoreHorizontal,
  MousePointerClick,
  Plus,
  RefreshCw,
  RotateCcw,
  Star,
  Boxes,
  Trash2,
  Webhook,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Link, useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
import {
  fallbackAuthorDisplay,
  resolveAuthorDisplay,
  type AuthorDisplay,
} from "@/lib/author-display";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/utils";
import {
  DB_KINDS,
  PROTOCOLS,
  RUN_STATUSES,
  TRIGGERS,
  actionsForStatus,
  getRunsProvider,
  type DbKind,
  type Protocol,
  type RunAction,
  type RunFacets,
  type RunStatus,
  type RunTrigger,
  type RunsQuery,
  type RunVM,
  type SortField,
} from "@/services/runs";
import { createShare } from "@/services/shares";

// Test Runs — the runs list/table. Built around the slice of
// cloud.v1.api.ListTestRunsRequest the provider wires (search, statuses[],
// db_kinds[], stroppy_versions[], protocols[], triggers[], started/finished
// windows, sort{entity|kind,desc}, page{size,token}). EVERY filterable column
// carries its own header-dropdown filter affordance; all filter/sort/page state
// lives in the URL query string so browser Back works and views are shareable
// (per the back-button convention in src/lib/router.tsx). Data comes through the
// RunsProvider — mock in standalone preview, real connectrpc once wired.

// --- status presentation (consistent with the dashboard's status→colour map).

// Status accent hue — the single colour that tints the merged Status+Progress
// cell (dot, inline label, progress fill, and the low-opacity track groove via
// an appended alpha). Values mirror the design tokens in index.css
// (primary/success/destructive/pending) with zinc-500 for cancelled, so the
// cell stays on-palette without introducing any new colours.
const STATUS_TINT: Record<RunStatus, string> = {
  pending: "#6b7280", // --color-pending
  running: "#3b82f6", // --color-primary
  cancelling: "#eab308", // --color-warning
  completed: "#22c55e", // --color-success
  failed: "#ef4444", // --color-destructive
  cancelled: "#71717a", // zinc-500 (muted)
};

const STATUS_LABEL: Record<RunStatus, string> = {
  pending: "Pending",
  running: "Running",
  cancelling: "Cancelling",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Cancelled",
};

const DB_LABEL: Record<Exclude<DbKind, "">, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  ydb: "YDB",
  ydb_managed: "YDB Managed",
  cockroach: "CockroachDB",
  picodata: "Picodata",
  external: "External",
};

// Per-database accent colour. Each engine gets its OWN distinct hue that sits
// well on the near-black table (#080808). Used for the dot + label in the
// Database column so rows are scannable by engine at a glance.
const DB_COLOR: Record<Exclude<DbKind, "">, string> = {
  postgres: "#38bdf8", // sky — Postgres elephant blue
  mysql: "#f59e0b", // amber — MySQL dolphin
  mariadb: "#a78bfa", // violet — MariaDB
  ydb: "#34d399", // emerald — YDB
  ydb_managed: "#2dd4bf", // teal — YDB managed (sibling hue of ydb)
  cockroach: "#f472b6", // pink — CockroachDB
  picodata: "#fb7185", // rose — Picodata
  external: "#9ca3af", // grey — external / unmanaged
};

const PROTOCOL_LABEL: Record<Exclude<Protocol, "">, string> = {
  pg: "PG",
  mysql: "MySQL",
  picodata: "Picodata",
  ydb_grpc: "YDB gRPC",
  ydb_grpcs: "YDB gRPCs",
  cockroach: "CockroachDB",
};

const TRIGGER_LABEL: Record<Exclude<RunTrigger, "">, string> = {
  manual: "Manual",
  cron: "Cron",
  api: "Api",
};

// Per-trigger icon for the thin first column. Each real cloud.v1.common.Trigger
// value gets a distinct lucide glyph; an unknown / UNSPECIFIED trigger falls
// back to a generic dot. The icon carries the trigger name as its title.
const TRIGGER_ICON: Record<Exclude<RunTrigger, "">, LucideIcon> = {
  manual: MousePointerClick, // a person clicked "run"
  cron: CalendarClock, // scheduled / recurring
  api: Webhook, // kicked off programmatically
};

// Stroppy versions we expose in the version facet. These are a fixed catalog in
// the preview; the real provider can derive the option list from a facet RPC.
const STROPPY_VERSIONS = ["5.1.2", "5.1.1", "5.1.0", "5.0.9"];

const PAGE_SIZES = [15, 25, 50, 100, 150];
const DEFAULT_SIZE = 25;

// Auto-refresh intervals (ms). 0 = off. Default 5s.
const REFRESH_OPTIONS: { label: string; ms: number }[] = [
  { label: "Off", ms: 0 },
  { label: "5s", ms: 5_000 },
  { label: "15s", ms: 15_000 },
  { label: "30s", ms: 30_000 },
  { label: "1m", ms: 60_000 },
];
const DEFAULT_REFRESH_MS = 5_000;

// Relative column proportions for the runs table. These are not fixed pixel
// widths: table-auto still lets content negotiate, while colgroup keeps the
// free space distributed by intent instead of letting grouped headers skew it.
const RUN_TABLE_COLUMN_WIDTHS: Record<string, string> = {
  select: "3%",
  name: "11%",
  suite: "8%",
  authorId: "7%",
  dbKind: "13%",
  workload: "15%",
  trigger: "2%",
  status: "23%",
  time: "8%",
  actions: "10%",
};

// --- time / duration formatting (em dash for unset, like the old page). ------

function relTime(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const diff = Date.now() - t;
  const m = Math.round(diff / 60_000);
  const h = Math.round(diff / 3_600_000);
  const d = Math.round(diff / 86_400_000);
  if (d >= 1) return `${d}d ago`;
  if (h >= 1) return `${h}h ago`;
  return `${Math.max(m, 1)}m ago`;
}

function formatDuration(sec?: number): string {
  if (sec === undefined || sec < 0) return "—";
  if (sec < 60) return `${sec}s`;
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  return `${h}h ${m}m`;
}

// ISO datetime (proto Timestamp) <-> <input type="date"> value (YYYY-MM-DD).
function isoToDateInput(iso?: string): string {
  if (!iso) return "";
  return iso.slice(0, 10);
}
function dateInputToIso(value: string, endOfDay: boolean): string | null {
  if (!value) return null;
  return endOfDay ? `${value}T23:59:59.999Z` : `${value}T00:00:00.000Z`;
}

// --- URL <-> query encoding. -------------------------------------------------
//
// Column / sort / paging:
//   ?q=&status=&db=&sv=&proto=&trigger=&sa=&sb=&fa=&fb=&sort=&dir=&size=&page=&iv=
// Added column-header filters:
//   ?author= (csv author ids) &pmin=&pmax= (progress 0..100) &dmin=&dmax= (duration secs)
// Above-table checkbox toggles:
//   ?sa_only=true (Standalone — only non-suite runs) &fav=1 (Favorites) &del=1 (Deleted)
// View-only table preferences:
//   ?ff=0 disables the default "favorites first" partition (no API field).

interface ParsedQuery extends RunsQuery {
  pageSize: number;
  // `sort` is undefined in the NEUTRAL tri-state (no ?sort= in the URL): no sort
  // param is sent and the provider falls back to its own default ordering.
  sort?: SortField;
  desc: boolean;
  refreshMs: number;
  favoritesFirst: boolean;
}

const SORTABLE_FIELDS: readonly SortField[] = [
  "name",
  "created_at",
  "status",
  "db_kind",
  "workload",
  "trigger",
  "duration",
  "started_at",
  "finished_at",
];

function parseQuery(sp: URLSearchParams): ParsedQuery {
  const csv = (key: string): string[] =>
    (sp.get(key) ?? "")
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

  const statuses = csv("status").filter((s): s is RunStatus =>
    (RUN_STATUSES as string[]).includes(s),
  );
  const dbKinds = csv("db").filter((s): s is Exclude<DbKind, ""> =>
    (DB_KINDS as string[]).includes(s),
  );
  const protocols = csv("proto").filter((s): s is Exclude<Protocol, ""> =>
    (PROTOCOLS as string[]).includes(s),
  );
  const triggers = csv("trigger").filter((s): s is Exclude<RunTrigger, ""> =>
    (TRIGGERS as string[]).includes(s),
  );
  const stroppyVersions = csv("sv").filter((s) =>
    STROPPY_VERSIONS.includes(s),
  );
  // Tri-state sort: no ?sort= → NEUTRAL (undefined). Only an explicit, known
  // field counts as an active sort; ?dir= defaults to desc when sort is set.
  const sortRaw = sp.get("sort");
  const sort: SortField | undefined =
    sortRaw && SORTABLE_FIELDS.includes(sortRaw as SortField)
      ? (sortRaw as SortField)
      : undefined;
  const desc = sp.get("dir") ? sp.get("dir") === "desc" : true;
  const size = Number.parseInt(sp.get("size") ?? "", 10);
  const pageSize = PAGE_SIZES.includes(size) ? size : DEFAULT_SIZE;
  const ivRaw = Number.parseInt(sp.get("iv") ?? "", 10);
  const refreshMs = REFRESH_OPTIONS.some((o) => o.ms === ivRaw)
    ? ivRaw
    : DEFAULT_REFRESH_MS;

  const authorIds = csv("author");

  // Bounded integer parse for the progress / duration range params.
  const intIn = (key: string, lo: number, hi: number): number | undefined => {
    const n = Number.parseInt(sp.get(key) ?? "", 10);
    if (Number.isNaN(n)) return undefined;
    return Math.max(lo, Math.min(hi, n));
  };
  const progressMin = intIn("pmin", 0, 100);
  const progressMax = intIn("pmax", 0, 100);
  const durationMinSec = intIn("dmin", 0, Number.MAX_SAFE_INTEGER);
  const durationMaxSec = intIn("dmax", 0, Number.MAX_SAFE_INTEGER);

  // standalone checkbox: ?sa_only=true → only non-suite runs; otherwise all.
  const standalone = sp.get("sa_only") === "true" ? true : undefined;

  return {
    search: sp.get("q") ?? undefined,
    statuses,
    dbKinds,
    stroppyVersions,
    protocols,
    triggers,
    authorIds: authorIds.length ? authorIds : undefined,
    progressMin,
    progressMax,
    durationMinSec,
    durationMaxSec,
    standalone,
    suiteRunId: sp.get("suiteRun") ?? undefined,
    favoritesOnly: sp.get("fav") === "1" ? true : undefined,
    includeDeleted: sp.get("del") === "1" ? true : undefined,
    startedAfter: sp.get("sa") ?? undefined,
    startedBefore: sp.get("sb") ?? undefined,
    finishedAfter: sp.get("fa") ?? undefined,
    finishedBefore: sp.get("fb") ?? undefined,
    sort,
    desc,
    pageSize,
    pageToken: sp.get("page") ?? undefined,
    refreshMs,
    favoritesFirst: sp.get("ff") !== "0",
  };
}

// Left/right divider classes that bracket a grouped block. Applied to the FIRST
// ("left") and LAST ("right") leaf of each group (and to "both" for a single
// member) on both the header <th> and the body <td> so the blocks read as
// visually distinct, contiguous brackets.
function groupEdgeClass(edge?: "left" | "right" | "both"): string {
  switch (edge) {
    case "left":
      return "border-l border-zinc-800/70";
    case "right":
      return "border-r border-zinc-800/70";
    case "both":
      return "border-l border-r border-zinc-800/70";
    default:
      return "";
  }
}

// --- header building blocks. -------------------------------------------------

// Shared hit-area + icon sizing for BOTH the sort and filter header buttons so
// they line up on the same center line across every header and stay tidy. Bumped
// a notch (icons ~14px, h-6/w-6 hit-area) per the polish pass.
const HEADER_BTN =
  "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer shrink-0";
const HEADER_ICON = "h-3.5 w-3.5";

/**
 * SortButton — the LEFT-side tri-state sort control. Cycles
 * ascending → descending → NEUTRAL on click; the icon reflects all three:
 * chevron-up (asc) / chevron-down (desc) / dimmed up-down (neutral, no sort).
 */
function SortButton({
  field,
  active,
  desc,
  onSort,
}: {
  field: SortField;
  active: boolean;
  desc: boolean;
  onSort: (field: SortField) => void;
}) {
  const Icon = !active ? ChevronsUpDown : desc ? ChevronDown : ChevronUp;
  return (
    <button
      type="button"
      onClick={() => onSort(field)}
      className={cn(
        HEADER_BTN,
        active ? "text-primary" : "text-zinc-600 hover:text-zinc-400",
      )}
      title={
        !active
          ? "Sort"
          : desc
            ? "Sorted descending — click for neutral"
            : "Sorted ascending — click for descending"
      }
      aria-label="Toggle sort"
    >
      <Icon className={HEADER_ICON} />
    </button>
  );
}

/**
 * ColumnHeader — a three-slot leaf header: `[⇅ sort]  Label  [⛃ filter]`. The
 * sort button (LEFT) and filter button (RIGHT) share one hit-area/icon size so
 * they align on a single center line across all headers; the label sits centered
 * between them. Columns without `field` omit the sort slot; columns without
 * `children` omit the filter slot — but both slots reserve their width so labels
 * stay aligned column-to-column.
 *
 * The filter popover open-state is LIFTED to the page (`openFilterColumnId`):
 * this header is controlled (`open`/`onOpenChange` keyed by `columnId`), so a
 * table re-render / refetch can't reset it and the input keeps focus while
 * typing. Auto-refresh pauses while any popover is open (page-level).
 */
function ColumnHeader({
  label,
  field,
  sort,
  desc,
  onSort,
  active,
  children,
  columnId,
  openColumnId,
  onOpenChange,
}: {
  label: string;
  field?: SortField;
  sort?: SortField;
  desc: boolean;
  onSort: (field: SortField) => void;
  active?: boolean;
  children?: React.ReactNode;
  columnId?: string;
  openColumnId?: string | null;
  onOpenChange?: (columnId: string, open: boolean) => void;
}) {
  const isActiveSort = field !== undefined && sort === field;
  const isOpen = !!columnId && openColumnId === columnId;
  return (
    <div className="flex items-center justify-start gap-1.5">
      {/* LEFT slot: tri-state sort (reserve width when absent for alignment). */}
      {field ? (
        <SortButton
          field={field}
          active={isActiveSort}
          desc={desc}
          onSort={onSort}
        />
      ) : (
        <span className="h-6 w-6 shrink-0" aria-hidden />
      )}

      <span className="whitespace-nowrap leading-none">{label}</span>

      {/* RIGHT slot: filter popover (reserve width when absent for alignment). */}
      {children && columnId ? (
        <Popover
          open={isOpen}
          onOpenChange={(o) => onOpenChange?.(columnId, o)}
        >
          <PopoverTrigger asChild>
            <button
              type="button"
              className={cn(
                HEADER_BTN,
                active
                  ? "text-primary"
                  : "text-zinc-600 hover:text-zinc-400",
              )}
              title={active ? `${label} filter active` : `Filter ${label}`}
            >
              <Filter
                className={HEADER_ICON}
                fill={active ? "currentColor" : "none"}
              />
            </button>
          </PopoverTrigger>
          <PopoverContent
            align="center"
            className="w-60 p-0 bg-zinc-950 border-zinc-800"
          >
            {children}
          </PopoverContent>
        </Popover>
      ) : (
        <span className="h-6 w-6 shrink-0" aria-hidden />
      )}
    </div>
  );
}

/** Multi-select checklist body (status, db, protocol, trigger, version). */
function ChecklistFilter({
  options,
  selected,
  onChange,
}: {
  options: { value: string; label: string; color?: string }[];
  selected: Set<string>;
  onChange: (next: Set<string>) => void;
}) {
  const allSelected = selected.size === 0;
  const toggle = (value: string) => {
    const next = new Set(selected);
    if (next.has(value)) next.delete(value);
    else next.add(value);
    onChange(next);
  };
  return (
    <div>
      <button
        type="button"
        onClick={() => onChange(new Set())}
        className={cn(
          "w-full flex items-center gap-2 px-3 py-1.5 text-[11px] font-mono hover:bg-zinc-900 transition-colors border-b border-zinc-800 cursor-pointer",
          allSelected && "text-primary",
        )}
      >
        <div
          className={cn(
            "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
            allSelected && "bg-primary border-primary",
          )}
        >
          {allSelected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
        </div>
        All
      </button>
      <div className="max-h-60 overflow-y-auto py-1">
        {options.map((opt) => {
          const isSelected = selected.has(opt.value);
          return (
            <button
              key={opt.value}
              type="button"
              onClick={() => toggle(opt.value)}
              className="w-full flex items-center gap-2 px-3 py-1 text-[11px] font-mono hover:bg-zinc-900 transition-colors cursor-pointer"
            >
              <div
                className={cn(
                  "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
                  isSelected && "bg-primary border-primary",
                )}
              >
                {isSelected && (
                  <Check className="h-2.5 w-2.5 text-primary-foreground" />
                )}
              </div>
              {opt.color && (
                <span
                  className="h-2 w-2 rounded-full shrink-0"
                  style={{ backgroundColor: opt.color }}
                />
              )}
              <span className="truncate text-zinc-300" title={opt.label}>
                {opt.label}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Free-text filter body (used by the Name column header). Debounces into the URL
 * query. The input keeps its OWN local state and a stable ref; after the
 * debounced commit triggers a refetch + table re-render, an effect re-focuses
 * the input and restores the caret so typing is never interrupted (the popover
 * itself no longer remounts because its open-state lives on the page).
 */
function TextFilter({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (next: string) => void;
  placeholder: string;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [local, setLocal] = useState(value);
  useEffect(() => setLocal(value), [value]);
  useEffect(() => {
    if (local === value) return;
    const id = setTimeout(() => onChange(local), 300);
    return () => clearTimeout(id);
  }, [local, value, onChange]);
  // Re-assert focus after any re-render that may have stolen it (debounced
  // refetch). Restore the caret to the end so the next keystroke lands right.
  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    if (document.activeElement !== el) {
      el.focus();
      const end = el.value.length;
      el.setSelectionRange(end, end);
    }
  });
  return (
    <div className="p-2">
      <Input
        ref={inputRef}
        autoFocus
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        placeholder={placeholder}
        className="h-8 w-full text-xs font-mono border-zinc-800 bg-transparent focus-visible:ring-primary/40"
      />
    </div>
  );
}

/** Date-range filter body (used by Started / Finished column headers). */
function DateRangeFilter({
  after,
  before,
  onChange,
}: {
  after?: string;
  before?: string;
  onChange: (after: string | null, before: string | null) => void;
}) {
  return (
    <div className="p-3 flex flex-col gap-2">
      <label className="flex flex-col gap-1">
        <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
          From
        </span>
        <input
          type="date"
          value={isoToDateInput(after)}
          onChange={(e) =>
            onChange(
              dateInputToIso(e.target.value, false),
              before ?? null,
            )
          }
          className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
        />
      </label>
      <label className="flex flex-col gap-1">
        <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
          To
        </span>
        <input
          type="date"
          value={isoToDateInput(before)}
          onChange={(e) =>
            onChange(after ?? null, dateInputToIso(e.target.value, true))
          }
          className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 [color-scheme:dark]"
        />
      </label>
      {(after || before) && (
        <button
          type="button"
          onClick={() => onChange(null, null)}
          className="flex items-center gap-1 text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer self-start"
        >
          <X className="h-3 w-3" /> clear
        </button>
      )}
    </div>
  );
}

/**
 * Numeric min/max range filter body (used for the progress window 0..100 on the
 * Status header and the duration window in seconds on the Time header). Keeps
 * local input state and commits on change; empty input clears that bound.
 */
function NumberRangeFilter({
  min,
  max,
  lo,
  hi,
  unit,
  onChange,
}: {
  min?: number;
  max?: number;
  lo: number;
  hi: number;
  unit?: string;
  onChange: (min: number | null, max: number | null) => void;
}) {
  const parse = (v: string): number | null => {
    if (v.trim() === "") return null;
    const n = Number.parseInt(v, 10);
    if (Number.isNaN(n)) return null;
    return Math.max(lo, Math.min(hi, n));
  };
  return (
    <div className="p-3 flex flex-col gap-2">
      <div className="flex items-end gap-2">
        <label className="flex flex-col gap-1 flex-1 min-w-0">
          <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
            Min{unit ? ` (${unit})` : ""}
          </span>
          <input
            type="number"
            inputMode="numeric"
            value={min ?? ""}
            min={lo}
            max={hi}
            onChange={(e) => onChange(parse(e.target.value), max ?? null)}
            className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 w-full"
          />
        </label>
        <span className="pb-2 text-zinc-600 font-mono text-xs">–</span>
        <label className="flex flex-col gap-1 flex-1 min-w-0">
          <span className="text-[9px] font-mono uppercase tracking-wider text-zinc-500">
            Max{unit ? ` (${unit})` : ""}
          </span>
          <input
            type="number"
            inputMode="numeric"
            value={max ?? ""}
            min={lo}
            max={hi}
            onChange={(e) => onChange(min ?? null, parse(e.target.value))}
            className="h-8 px-2 text-xs font-mono border border-zinc-800 bg-transparent text-zinc-300 focus:outline-none focus:ring-1 focus:ring-primary/40 w-full"
          />
        </label>
      </div>
      {(min !== undefined || max !== undefined) && (
        <button
          type="button"
          onClick={() => onChange(null, null)}
          className="flex items-center gap-1 text-[10px] text-zinc-500 hover:text-zinc-300 font-mono underline underline-offset-2 cursor-pointer self-start"
        >
          <X className="h-3 w-3" /> clear
        </button>
      )}
    </div>
  );
}

/**
 * ToolbarCheckbox — a small square checkbox + label, matching the on-brand
 * square check used by ChecklistFilter, rendered inline ABOVE the table. Drives
 * one boolean filter toggle (Standalone / Favorites / Deleted).
 */
function ToolbarCheckbox({
  checked,
  onChange,
  label,
  title,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
  title?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      title={title}
      aria-pressed={checked}
      className={cn(
        "flex h-5 items-center gap-1.5 text-[10px] font-mono transition-colors cursor-pointer",
        checked ? "text-primary" : "text-zinc-500 hover:text-zinc-300",
      )}
    >
      <span
        className={cn(
          "h-3.5 w-3.5 rounded-sm border flex items-center justify-center shrink-0 transition-colors",
          checked ? "bg-primary border-primary" : "border-zinc-700",
        )}
      >
        {checked && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
      </span>
      {label}
    </button>
  );
}

/**
 * SelectCheckbox — compact dark checkbox matching the filter checklist style.
 * The header uses the same control with a dash when only part of the visible
 * page is selected.
 */
function SelectCheckbox({
  checked,
  indeterminate = false,
  onCheckedChange,
  title,
  ariaLabel,
}: {
  checked: boolean;
  indeterminate?: boolean;
  onCheckedChange: (checked: boolean) => void;
  title: string;
  ariaLabel: string;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={indeterminate ? "mixed" : checked}
      onClick={(e) => {
        e.stopPropagation();
        onCheckedChange(indeterminate ? true : !checked);
      }}
      title={title}
      aria-label={ariaLabel}
      className="mx-auto flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer hover:bg-zinc-800/60"
    >
      <span
        className={cn(
          "flex h-3.5 w-3.5 items-center justify-center rounded-sm border transition-colors",
          checked || indeterminate
            ? "border-primary bg-primary"
            : "border-zinc-700 bg-zinc-950/80",
        )}
      >
        {checked ? (
          <Check className="h-2.5 w-2.5 text-primary-foreground" />
        ) : indeterminate ? (
          <span className="h-px w-2 bg-primary-foreground" />
        ) : null}
      </span>
    </button>
  );
}

// --- row Actions menu. -------------------------------------------------------

// The full action catalog, in menu order. Each entry maps to a RunAction that
// itself resolves to a real TestRunService RPC / route (see actionsForStatus +
// RunsProvider). `danger` flags destructive items for the red treatment.
const ACTION_ITEMS: {
  action: RunAction;
  label: string;
  icon: LucideIcon;
  danger?: boolean;
}[] = [
  { action: "view", label: "View detail", icon: Eye },
  { action: "share", label: "Share link", icon: Link2 },
  { action: "clone", label: "New from", icon: CopyPlus },
  { action: "rerun", label: "Re-run", icon: RotateCcw },
  { action: "extract", label: "Save as preset", icon: Bookmark },
  { action: "cancel", label: "Cancel run", icon: Ban },
  { action: "delete", label: "Delete", icon: Trash2, danger: true },
];

// Why an action is greyed out, per status — shown as the disabled item's title.
const DISABLED_REASON: Record<Exclude<RunAction, "view" | "share" | "clone">, string> = {
  cancel: "Only running or pending runs can be cancelled",
  rerun: "Only finished runs can be re-run",
  extract: "Only completed runs can be saved as a preset",
  delete: "A running run can't be deleted — cancel it first",
};

function disabledReason(action: RunAction): string | undefined {
  if (action === "view" || action === "share" || action === "clone") return undefined;
  return DISABLED_REASON[action];
}

/**
 * ActionsMenu — the compact "⋯" (kebab) cell. Opens a radix dropdown listing
 * EVERY run action; items invalid for the row's current status are DISABLED
 * (greyed, with a reason) rather than hidden, per actionsForStatus(). The
 * trigger stopPropagation's so it never fires the row's name-link navigation,
 * and the menu's open-state is CONTROLLED by the page (keyed by run id) so the
 * 5s auto-refresh re-render can't close it mid-interaction.
 */
function ActionsMenu({
  run,
  open,
  onOpenChange,
  onAction,
}: {
  run: RunVM;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAction: (action: RunAction, run: RunVM) => void;
}) {
  const allowed = actionsForStatus(run.status);
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          // Stop the row's onClick (name-link navigation) from firing.
          onClick={(e) => e.stopPropagation()}
          className={cn(
            "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
            open
              ? "text-zinc-200 bg-zinc-800"
              : "text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800/60",
          )}
          title="Run actions"
          aria-label="Run actions"
        >
          <MoreHorizontal className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        // Stop row-level clicks from bubbling out of the portalled menu.
        onClick={(e) => e.stopPropagation()}
        className="min-w-[11rem] bg-zinc-950 border-zinc-800"
      >
        <DropdownMenuLabel className="truncate" title={run.name || run.id}>
          {run.name || run.id}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        {ACTION_ITEMS.map((item) => {
          const enabled = allowed.has(item.action);
          const reason = disabledReason(item.action);
          return (
            <DropdownMenuItem
              key={item.action}
              disabled={!enabled}
              title={enabled ? undefined : reason}
              onSelect={(e) => {
                // Keep the row click from firing; route through the page handler.
                e.preventDefault();
                if (enabled) onAction(item.action, run);
              }}
              className={cn(
                item.danger &&
                  "text-destructive focus:text-destructive focus:bg-destructive/10",
              )}
            >
              <item.icon className="h-3.5 w-3.5 shrink-0" />
              {item.label}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function Runs() {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const query = useMemo(() => parseQuery(searchParams), [searchParams]);

  const [runs, setRuns] = useState<RunVM[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(true);
  // `refreshing` is a background re-fetch (auto-refresh / manual) that keeps the
  // current rows on screen and only shows a subtle "updating" pulse.
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Token stack so the Prev button can walk back through opaque cursors (the
  // API only hands forward tokens). Reset (in patchParams) whenever a
  // facet/sort/size change sends us back to the first page.
  const tokenStackRef = useRef<string[]>([]);

  // Which column's filter popover is open, keyed by column id (null = none).
  // LIFTED out of the per-cell header render so a table re-render / refetch (or
  // the 5s auto-refresh) can't reset it and close the popover mid-type. A single
  // page-level handler drives the controlled radix Popover for every header.
  const [openFilterColumnId, setOpenFilterColumnId] = useState<string | null>(
    null,
  );
  const handleFilterOpenChange = useCallback(
    (columnId: string, open: boolean) => {
      setOpenFilterColumnId(open ? columnId : (cur) => (cur === columnId ? null : cur));
    },
    [],
  );
  // Which row's Actions (⋯) dropdown is open, keyed by run id (null = none).
  // Controlled with the SAME discipline as the filter popovers: lifted to the
  // page so a table re-render / 5s auto-refresh can't reset it mid-interaction,
  // and refresh pauses while it's open (see popoverOpen below).
  const [openActionRunId, setOpenActionRunId] = useState<string | null>(null);
  const confirm = useConfirm();

  // Distinct author ids present in the data, for the Author column pick-list;
  // sourced through the provider so the mock stays isolated. Fetched once per
  // slug (best-effort: a failure just leaves an empty pick-list).
  const [facets, setFacets] = useState<RunFacets>({ authorIds: [] });
  const [authorDisplays, setAuthorDisplays] = useState<
    Record<string, AuthorDisplay>
  >({});
  const authorDisplaysRef = useRef<Record<string, AuthorDisplay>>({});

  const rememberAuthorDisplays = useCallback(
    (next: Record<string, AuthorDisplay>) => {
      if (Object.keys(next).length === 0) return;
      const merged = { ...authorDisplaysRef.current, ...next };
      authorDisplaysRef.current = merged;
      setAuthorDisplays(merged);
    },
    [],
  );

  const resolveAuthors = useCallback(
    (ids: string[]) => {
      const missing = [...new Set(ids.filter(Boolean))].filter(
        (id) => !authorDisplaysRef.current[id],
      );
      if (missing.length === 0) return;
      void Promise.all(
        missing.map(async (id) => [id, await resolveAuthorDisplay(id)] as const),
      ).then((entries) => {
        const next: Record<string, AuthorDisplay> = {};
        for (const [id, display] of entries) next[id] = display;
        rememberAuthorDisplays(next);
      });
    },
    [rememberAuthorDisplays],
  );

  // Row selection is local UI state for the future Compare page. It intentionally
  // stores stable run ids, not row indexes, so refetches/re-sorts keep selection.
  const [selectedRunIds, setSelectedRunIds] = useState<Set<string>>(
    () => new Set(),
  );
  // A header filter popover OR a row action menu is open → auto-refresh pauses
  // so an in-flight selection / search / action isn't disrupted by a row swap.
  const popoverOpen = openFilterColumnId !== null || openActionRunId !== null;

  const fetchRuns = useCallback(
    async (background = false) => {
      if (!slug) return;
      if (background) setRefreshing(true);
      else setLoading(true);
      setError(null);
      try {
        const page = await getRunsProvider().listRuns(slug, {
          search: query.search,
          statuses: query.statuses,
          dbKinds: query.dbKinds,
          stroppyVersions: query.stroppyVersions,
          protocols: query.protocols,
          triggers: query.triggers,
          authorIds: query.authorIds,
          progressMin: query.progressMin,
          progressMax: query.progressMax,
          durationMinSec: query.durationMinSec,
          durationMaxSec: query.durationMaxSec,
          standalone: query.standalone,
          suiteRunId: query.suiteRunId,
          favoritesOnly: query.favoritesOnly,
          includeDeleted: query.includeDeleted,
          startedAfter: query.startedAfter,
          startedBefore: query.startedBefore,
          finishedAfter: query.finishedAfter,
          finishedBefore: query.finishedBefore,
          sort: query.sort,
          desc: query.desc,
          pageSize: query.pageSize,
          pageToken: query.pageToken,
        });
        setRuns(page.runs);
        setNextPageToken(page.nextPageToken);
        resolveAuthors(page.runs.map((run) => run.authorId));
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load runs");
        setRuns([]);
        setNextPageToken("");
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [
      slug,
      query.search,
      query.statuses,
      query.dbKinds,
      query.stroppyVersions,
      query.protocols,
      query.triggers,
      query.authorIds,
      query.progressMin,
      query.progressMax,
      query.durationMinSec,
      query.durationMaxSec,
      query.standalone,
      query.suiteRunId,
      query.favoritesOnly,
      query.includeDeleted,
      query.startedAfter,
      query.startedBefore,
      query.finishedAfter,
      query.finishedBefore,
      query.sort,
      query.desc,
      query.pageSize,
      query.pageToken,
      resolveAuthors,
    ],
  );

  // Run a per-row action through the provider, then refresh in the background so
  // the row reflects the new state (cancelled / removed / re-run). Errors surface
  // in the page-level error banner; the menu closes regardless.
  const runAction = useCallback(
    async (action: RunAction, run: RunVM) => {
      if (!slug) return;
      setOpenActionRunId(null);
      const provider = getRunsProvider();
      try {
        switch (action) {
          case "view":
            navigate(`/runs/${run.id}`);
            return;
          case "clone":
            navigate(`/runs/new?from=${run.id}`);
            return;
          case "share": {
            const share = await createShare(slug, run.id, { kind: "test_run" });
            if (!share?.token) throw new Error("Share token was not returned");
            const url = new URL(`/shared/${encodeURIComponent(share.token)}`, window.location.origin);
            await navigator.clipboard.writeText(url.href);
            return;
          }
          case "cancel":
            await provider.cancelRun(slug, run.id);
            break;
          case "rerun":
            await provider.rerunRun(slug, run.id);
            break;
          case "extract":
            await provider.extractToPreset(slug, run.id);
            break;
          case "delete": {
            const ok = await confirm({
              title: "Delete run?",
              description: `“${run.name || run.id}” will be permanently removed. This cannot be undone.`,
              danger: true,
              confirmLabel: "Delete",
            });
            if (!ok) return;
            await provider.deleteRun(slug, run.id);
            setSelectedRunIds((current) => {
              const next = new Set(current);
              next.delete(run.id);
              return next;
            });
            break;
          }
        }
        await fetchRuns(true);
      } catch (err) {
        setError(err instanceof Error ? err.message : `Failed to ${action} run`);
      }
    },
    [slug, navigate, confirm, fetchRuns],
  );

  // Foreground fetch whenever the query changes (filters / sort / page).
  useEffect(() => {
    void fetchRuns(false);
  }, [fetchRuns]);

  // Load the author pick-list once per slug. Best-effort: a provider that hasn't
  // wired faceting (the real one throws) just leaves an empty option list, so
  // the Author header falls back to free-text entry without breaking the page.
  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    void getRunsProvider()
      .listFacets(slug)
      .then((f) => {
        if (!cancelled) {
          setFacets(f);
          resolveAuthors(f.authorIds);
        }
      })
      .catch(() => {
        if (!cancelled) setFacets({ authorIds: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [slug, resolveAuthors]);

  // Auto-refresh: background re-fetch on the selected interval. Paused while a
  // header filter popover is open. Cleans up on unmount / dependency change.
  useEffect(() => {
    if (query.refreshMs <= 0) return;
    if (popoverOpen) return;
    const id = setInterval(() => {
      void fetchRuns(true);
    }, query.refreshMs);
    return () => clearInterval(id);
  }, [query.refreshMs, popoverOpen, fetchRuns]);

  // Patch the URL query string. Any facet/sort change clears the page token
  // (back to first page) and the token stack — but NEVER touches `size` (page
  // size) or `iv` (refresh interval), so those survive a filter change.
  const patchParams = useCallback(
    (patch: Record<string, string | null>, resetPage = true) => {
      const next = new URLSearchParams(searchParams);
      for (const [k, v] of Object.entries(patch)) {
        if (v === null || v === "") next.delete(k);
        else next.set(k, v);
      }
      if (resetPage) {
        next.delete("page");
        tokenStackRef.current = [];
      }
      setSearchParams(next);
    },
    [searchParams, setSearchParams],
  );

  const toggleFavoritesFirst = useCallback(() => {
    patchParams({ ff: query.favoritesFirst ? "0" : null }, false);
  }, [patchParams, query.favoritesFirst]);

  // Tri-state cycle: ascending → descending → NEUTRAL → ascending …
  //   - not this column / neutral  → ascending  (sort=field, dir=asc)
  //   - ascending                  → descending (sort=field, dir=desc)
  //   - descending                 → NEUTRAL    (clear sort & dir from URL)
  // `enableSortingRemoval` semantics: the third click removes the sort entirely
  // so no sort param is sent and the provider falls back to its default order.
  function toggleSort(field: SortField) {
    const active = query.sort === field;
    if (!active) {
      patchParams({ sort: field, dir: "asc" });
    } else if (!query.desc) {
      patchParams({ sort: field, dir: "desc" });
    } else {
      patchParams({ sort: null, dir: null });
    }
  }

  // Facet setters — each resets to page 1 but preserves size + interval.
  const setCsv = useCallback(
    (key: string, selected: Set<string>) => {
      patchParams({ [key]: [...selected].join(",") || null });
    },
    [patchParams],
  );
  function setPageSize(size: number) {
    // Page-size change is itself a paging change → reset cursor, keep filters.
    patchParams({ size: size === DEFAULT_SIZE ? null : String(size) });
  }
  function setRefreshMs(ms: number) {
    // Interval change must not disturb paging or filters.
    patchParams({ iv: String(ms) }, false);
  }

  function goNext() {
    if (!nextPageToken) return;
    tokenStackRef.current = [...tokenStackRef.current, query.pageToken ?? ""];
    patchParams({ page: nextPageToken }, false);
  }
  function goPrev() {
    const stack = tokenStackRef.current;
    if (stack.length === 0) return;
    const prevToken = stack[stack.length - 1];
    tokenStackRef.current = stack.slice(0, -1);
    patchParams({ page: prevToken || null }, false);
  }

  const statusSet = useMemo(
    () => new Set<string>(query.statuses ?? []),
    [query.statuses],
  );
  const dbSet = useMemo(
    () => new Set<string>(query.dbKinds ?? []),
    [query.dbKinds],
  );
  const versionSet = useMemo(
    () => new Set<string>(query.stroppyVersions ?? []),
    [query.stroppyVersions],
  );
  const protocolSet = useMemo(
    () => new Set<string>(query.protocols ?? []),
    [query.protocols],
  );
  const triggerSet = useMemo(
    () => new Set<string>(query.triggers ?? []),
    [query.triggers],
  );
  const authorSet = useMemo(
    () => new Set<string>(query.authorIds ?? []),
    [query.authorIds],
  );

  const hasTableFilters =
    !!query.search ||
    (query.statuses?.length ?? 0) > 0 ||
    (query.dbKinds?.length ?? 0) > 0 ||
    (query.stroppyVersions?.length ?? 0) > 0 ||
    (query.protocols?.length ?? 0) > 0 ||
    (query.triggers?.length ?? 0) > 0 ||
    (query.authorIds?.length ?? 0) > 0 ||
    query.progressMin !== undefined ||
    query.progressMax !== undefined ||
    query.durationMinSec !== undefined ||
    query.durationMaxSec !== undefined ||
    !!query.startedAfter ||
    !!query.startedBefore ||
    !!query.finishedAfter ||
    !!query.finishedBefore;
  const hasGlobalFilters =
    query.standalone !== undefined ||
    !!query.suiteRunId ||
    !!query.favoritesOnly ||
    !!query.includeDeleted;
  const hasActiveFilters = hasTableFilters || hasGlobalFilters;

  function clearTableFilters() {
    patchParams({
      q: null,
      status: null,
      db: null,
      sv: null,
      proto: null,
      trigger: null,
      author: null,
      pmin: null,
      pmax: null,
      dmin: null,
      dmax: null,
      sa: null,
      sb: null,
      fa: null,
      fb: null,
      suiteRun: null,
    });
  }

  const pageIndex = tokenStackRef.current.length; // 0-based page number
  const canPrev = pageIndex > 0;
  const canNext = !!nextPageToken;

  const visibleRuns = useMemo(() => {
    if (!query.favoritesFirst) return runs;
    const favoriteRuns: RunVM[] = [];
    const otherRuns: RunVM[] = [];
    for (const run of runs) {
      if (run.favorite) favoriteRuns.push(run);
      else otherRuns.push(run);
    }
    return [...favoriteRuns, ...otherRuns];
  }, [runs, query.favoritesFirst]);

  const selectedRunIdList = useMemo(
    () => [...selectedRunIds],
    [selectedRunIds],
  );
  const selectedCount = selectedRunIds.size;
  const compareTo = useMemo(
    () => ({
      pathname: "/compare",
      search: `?runIds=${selectedRunIdList.map(encodeURIComponent).join(",")}`,
    }),
    [selectedRunIdList],
  );
  const visibleRunIds = useMemo(
    () => visibleRuns.map((run) => run.id),
    [visibleRuns],
  );
  const selectedVisibleCount = useMemo(
    () => visibleRunIds.filter((id) => selectedRunIds.has(id)).length,
    [visibleRunIds, selectedRunIds],
  );
  const allVisibleSelected =
    visibleRunIds.length > 0 && selectedVisibleCount === visibleRunIds.length;
  const someVisibleSelected =
    selectedVisibleCount > 0 && selectedVisibleCount < visibleRunIds.length;

  const toggleRunSelection = useCallback((runId: string, selected: boolean) => {
    setSelectedRunIds((current) => {
      const next = new Set(current);
      if (selected) next.add(runId);
      else next.delete(runId);
      return next;
    });
  }, []);

  const toggleVisibleSelection = useCallback(
    (selected: boolean) => {
      setSelectedRunIds((current) => {
        const next = new Set(current);
        for (const runId of visibleRunIds) {
          if (selected) next.add(runId);
          else next.delete(runId);
        }
        return next;
      });
    },
    [visibleRunIds],
  );

  const toggleFavorite = useCallback(
    async (run: RunVM) => {
      if (!slug) return;
      const nextFavorite = !run.favorite;
      try {
        await getRunsProvider().setFavorite(slug, run.id, nextFavorite);
        setRuns((current) => {
          const next = current.map((item) =>
            item.id === run.id ? { ...item, favorite: nextFavorite } : item,
          );
          if (query.favoritesOnly && !nextFavorite) {
            return next.filter((item) => item.id !== run.id);
          }
          return next;
        });
        if (query.favoritesOnly && !nextFavorite) {
          await fetchRuns(true);
        }
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to update favorite",
        );
      }
    },
    [slug, query.favoritesOnly, fetchRuns],
  );

  // --- columns (each cell renders a real TestRunRecord field). ---------------
  //
  // FOUR grouped-header blocks read left → right; every leaf lives under one
  // parent header (Controls | Run | Target | Execution). Each block carries the
  // same visual treatment: a spanning parent label in row 1 of the sticky
  // header, a subtle background band behind its body cells, and left/right
  // dividers bracketing the block so they read as distinct contiguous groups.
  //
  //   * Controls group
  //       - Select    = local compare selection checkbox
  //   * Run group
  //       - Name      = name (link) + id subtext  (text filter, name sort)
  //       - Author    = identicon avatar + author id  (author sort)
  //   * Target group
  //       - Database  = coloured dot + db_kind + topology/node-count subtext
  //                     (db multi-select filter, db_kind sort)
  //       - Workload  = workload_name (plain foreground, NO per-workload hue) +
  //                     protocol + stroppy_version subtext (protocol+version
  //                     multi-select filters, workload sort)
  //   * Execution group
  //       - Trigger   = NORMAL labeled column — small icon + the trigger name in
  //                     PascalCase ("Manual"/"Cron"/"Api", unknown → "—"); the
  //                     trigger multi-select filter lives in its header popover.
  //       - Status    = lifecycle badge (status multi-select filter)
  //       - Progress  = status-tinted bar + percent (progress sort)
  //       - Time      = started (relative) + finished/duration subtext; started
  //                     AND finished date-range filters both hang off this
  //                     sub-header's popover.
  //       - Actions   = row action menu + per-caller favorite toggle; header
  //                     star toggles favorites-first
  // Each leaf keeps its OWN sort + header filter.

  const columns = useMemo<ColumnDef<RunVM>[]>(
    () => [
      // --- utility column: Compare selection. -------------------------------
      {
        id: "controls",
        header: () => <span className="sr-only">Run controls</span>,
        columns: [
          {
            id: "select",
            enableSorting: false,
            meta: {
              group: "controls",
              className: "px-1 text-center",
              disableRowNavigation: true,
            },
            header: () => (
              <SelectCheckbox
                checked={allVisibleSelected}
                indeterminate={someVisibleSelected}
                onCheckedChange={toggleVisibleSelection}
                title="Select visible runs"
                ariaLabel="Select visible runs"
              />
            ),
            cell: ({ row }) => (
              <SelectCheckbox
                checked={selectedRunIds.has(row.original.id)}
                onCheckedChange={(selected) =>
                  toggleRunSelection(row.original.id, selected)
                }
                title="Select run for compare"
                ariaLabel="Select run for compare"
              />
            ),
          },
        ],
      },
      // --- "Run" group: Name · Author. ---------------------------------------
      {
        id: "run",
        meta: { groupEdge: "right" },
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Run
          </span>
        ),
        columns: [
      {
        accessorKey: "name",
        meta: { group: "run" },
        header: () => (
          <ColumnHeader
            label="Name"
            field="name"
            sort={query.sort}
            desc={query.desc}
            onSort={toggleSort}
            active={!!query.search}
            columnId="name"
            openColumnId={openFilterColumnId}
            onOpenChange={handleFilterOpenChange}
          >
            <TextFilter
              value={query.search ?? ""}
              onChange={(v) => patchParams({ q: v || null })}
              placeholder="Search name / id..."
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const r = row.original;
          return (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                navigate(`/runs/${r.id}`);
              }}
              className="flex flex-col gap-0.5 min-w-0 text-left group/name cursor-pointer"
            >
              <span
                className="block max-w-full text-xs text-primary group-hover/name:underline underline-offset-2 truncate"
                title={r.name || r.id}
              >
                {r.name || r.id}
              </span>
              <span
                className="font-mono text-[10px] text-zinc-600"
                title={r.id}
              >
                {r.id}
              </span>
            </button>
          );
        },
      },
      {
        // Suite membership: the run's suite_run_id (record.suite_run_id) when it
        // belongs to a suite, or a muted dash for standalone runs. No sort/filter
        // of its own (suite scoping is governed by the Standalone checkbox).
        id: "suite",
        accessorKey: "suiteRunId",
        enableSorting: false,
        meta: {
          group: "run",
          className: "text-center",
          disableRowNavigation: true,
        },
        header: () => (
          <div className="flex justify-center">
            <ColumnHeader
              label="Suite"
              sort={query.sort}
              desc={query.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) => {
          const suite = row.original.suiteRunId;
          if (!suite)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          return (
            <Link
              to={`/suites/${suite}`}
              onClick={(e) => e.stopPropagation()}
              className="mx-auto block max-w-full truncate font-mono text-xs text-primary underline-offset-2 hover:underline"
              title={suite}
            >
              {suite}
            </Link>
          );
        },
      },
      {
        accessorKey: "authorId",
        // Last member of the "Run" group → right divider + group band.
        meta: { group: "run", groupEdge: "right" },
        header: () => (
          <ColumnHeader
            label="Author"
            sort={query.sort}
            desc={query.desc}
            onSort={toggleSort}
            active={authorSet.size > 0}
            columnId="author"
            openColumnId={openFilterColumnId}
            onOpenChange={handleFilterOpenChange}
          >
            {facets.authorIds.length > 0 ? (
              <ChecklistFilter
                options={facets.authorIds.map((a) => ({
                  value: a,
                  label: authorDisplays[a]?.label ?? a,
                }))}
                selected={authorSet}
                onChange={(next) => setCsv("author", next)}
              />
            ) : (
              // No author catalog yet (real provider): fall back to free-text
              // entry of a single author id so the facet is still reachable.
              <TextFilter
                value={query.authorIds?.[0] ?? ""}
                onChange={(v) => patchParams({ author: v || null })}
                placeholder="Author id..."
              />
            )}
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const author = row.original.authorId;
          if (!author)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          const display = authorDisplays[author] ?? fallbackAuthorDisplay(author);
          return (
            <div className="flex items-center gap-2 min-w-0">
              <Avatar name={display.avatarName} size={22} className="shrink-0" />
              <span
                className="font-mono text-xs text-zinc-400 truncate"
                title={display.title}
              >
                {display.label}
              </span>
            </div>
          );
        },
      },
        ],
      },
      // --- "Target" group: Database · Workload. ------------------------------
      {
        id: "target",
        meta: { groupEdge: "both" },
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Target
          </span>
        ),
        columns: [
      {
        accessorKey: "dbKind",
        // First member of the "Target" group → left divider + group band.
        meta: { group: "target", groupEdge: "left", className: "pl-4" },
        header: () => (
          <ColumnHeader
            label="Database"
            field="db_kind"
            sort={query.sort}
            desc={query.desc}
            onSort={toggleSort}
            active={dbSet.size > 0}
            columnId="dbKind"
            openColumnId={openFilterColumnId}
            onOpenChange={handleFilterOpenChange}
          >
            <ChecklistFilter
              options={DB_KINDS.map((k) => ({
                value: k,
                label: DB_LABEL[k],
                color: DB_COLOR[k],
              }))}
              selected={dbSet}
              onChange={(next) => setCsv("db", next)}
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const r = row.original;
          if (!r.dbKind)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          const color = DB_COLOR[r.dbKind];
          const nodes = r.nodeCount
            ? `${r.nodeCount} node${r.nodeCount > 1 ? "s" : ""}`
            : null;
          return (
            <div className="flex items-center gap-2">
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: color }}
              />
              <div className="flex flex-col gap-0.5 min-w-0">
                <span
                  className="font-mono text-xs"
                  style={{ color }}
                >
                  {DB_LABEL[r.dbKind]}
                </span>
                {(r.topologyLabel || nodes) && (
                  <span className="font-mono text-[10px] text-zinc-600 truncate">
                    {r.topologyLabel || nodes}
                  </span>
                )}
              </div>
            </div>
          );
        },
      },
      {
        accessorKey: "workload",
        // Last member of the "Target" group → right divider + group band.
        meta: { group: "target", groupEdge: "right" },
        header: () => (
          <ColumnHeader
            label="Workload"
            field="workload"
            sort={query.sort}
            desc={query.desc}
            onSort={toggleSort}
            active={versionSet.size > 0 || protocolSet.size > 0}
            columnId="workload"
            openColumnId={openFilterColumnId}
            onOpenChange={handleFilterOpenChange}
          >
            <div>
              <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500">
                Protocol
              </div>
              <ChecklistFilter
                options={PROTOCOLS.map((p) => ({
                  value: p,
                  label: PROTOCOL_LABEL[p],
                }))}
                selected={protocolSet}
                onChange={(next) => setCsv("proto", next)}
              />
              <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500 border-t border-zinc-800">
                Stroppy version
              </div>
              <ChecklistFilter
                options={STROPPY_VERSIONS.map((v) => ({ value: v, label: v }))}
                selected={versionSet}
                onChange={(next) => setCsv("sv", next)}
              />
            </div>
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const r = row.original;
          if (!r.workload)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          const proto = r.protocol ? PROTOCOL_LABEL[r.protocol] : null;
          const sub = [proto, r.stroppyVersion ? `stroppy ${r.stroppyVersion}` : null]
            .filter(Boolean)
            .join(" · ");
          // Workload renders as plain text in the normal foreground colour — no
          // per-workload hue (database colours stay; workloads do not).
          return (
            <div className="flex flex-col gap-0.5">
              <span className="font-mono text-xs text-zinc-300">
                {r.workload}
              </span>
              {sub && (
                <span className="font-mono text-[10px] text-zinc-600">{sub}</span>
              )}
            </div>
          );
        },
      },
        ],
      },
      // --- "Execution" group: Trigger · Status+Progress · Time read as ONE
      // contiguous block, anchored at the right. A spanning parent header sits
      // over the three sub-columns and a subtle background band + left/right
      // group dividers tie them together. Status & Progress are MERGED into one
      // animated cell. Each sub-column keeps its OWN sort + header filter
      // (trigger multi-select on Trigger, status multi-select + status/progress
      // sort on the merged Status column, started/finished windows on Time).
      {
        id: "execution",
        meta: { groupEdge: "both" },
        // Parent header label, spanning the three leaf columns below it.
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Execution
          </span>
        ),
        columns: [
          {
            id: "trigger",
            // First member of the group → left divider + group band. A NORMAL
            // labeled column (sized like the others): a small icon followed by
            // the trigger name in PascalCase. The trigger multi-select filter
            // lives in this header's popover.
            meta: { group: "execution", groupEdge: "left", className: "pl-4" },
            header: () => (
              <ColumnHeader
                label="Trigger"
                field="trigger"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={triggerSet.size > 0}
                columnId="trigger"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <ChecklistFilter
                  options={TRIGGERS.map((t) => ({
                    value: t,
                    label: TRIGGER_LABEL[t],
                  }))}
                  selected={triggerSet}
                  onChange={(next) => setCsv("trigger", next)}
                />
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const t = row.original.trigger;
              if (!t)
                return (
                  <div className="flex items-center gap-1.5">
                    <Circle
                      className="h-4 w-4 text-zinc-600 shrink-0"
                      aria-label="Unknown trigger"
                    >
                      <title>Unknown trigger</title>
                    </Circle>
                    <span className="text-[11px] font-mono text-zinc-600">
                      —
                    </span>
                  </div>
                );
              const Icon = TRIGGER_ICON[t];
              return (
                <div className="flex items-center gap-1.5">
                  <Icon
                    className="h-4 w-4 text-zinc-500 shrink-0"
                    aria-label={`${TRIGGER_LABEL[t]} trigger`}
                  >
                    <title>{`${TRIGGER_LABEL[t]} trigger`}</title>
                  </Icon>
                  <span className="text-[11px] font-mono text-zinc-400">
                    {TRIGGER_LABEL[t]}
                  </span>
                </div>
              );
            },
          },
          {
            // MERGED Status + Progress: the lifecycle badge on top, a
            // status-tinted progress bar with % beneath it — one cell, one
            // header. The header keeps the status multi-select filter AND sorts
            // on status. The bar width animates (CSS width transition) so it
            // glides on each refresh/update rather than jumping.
            accessorKey: "status",
            id: "status",
            meta: { group: "execution" },
            header: () => (
              <ColumnHeader
                label="Status"
                field="status"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={
                  statusSet.size > 0 ||
                  query.progressMin !== undefined ||
                  query.progressMax !== undefined
                }
                columnId="status"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <div>
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500">
                    Status
                  </div>
                  <ChecklistFilter
                    options={RUN_STATUSES.map((s) => ({
                      value: s,
                      label: STATUS_LABEL[s],
                    }))}
                    selected={statusSet}
                    onChange={(next) => setCsv("status", next)}
                  />
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500 border-t border-zinc-800">
                    Progress %
                  </div>
                  <NumberRangeFilter
                    min={query.progressMin}
                    max={query.progressMax}
                    lo={0}
                    hi={100}
                    onChange={(lo, hi) =>
                      patchParams({
                        pmin: lo === null ? null : String(lo),
                        pmax: hi === null ? null : String(hi),
                      })
                    }
                  />
                </div>
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const r = row.original;
              const s = r.status;
              const pct = Math.max(0, Math.min(100, r.progressPct));
              // One status hue drives the dot, the label text and the bar fill,
              // so the whole cell reads as a single status-tinted element.
              const tint = STATUS_TINT[s];
              const isNonTerminalStatus =
                s === "pending" || s === "running" || s === "cancelling";
              return (
                // ONE cohesive row: [● Label] · [▓▓░ track] · [right-aligned %].
                // The dot, label, track fill and % all share the same baseline
                // and the same status hue; nothing is a chunky stacked badge.
                <div className="flex items-center gap-2 min-w-0">
                  {/* Status: a small filled dot + a compact label in the status
                      hue — tasteful, inline, not a separate boxed badge. */}
                  <span className="flex items-center gap-1.5 shrink-0 w-[4.75rem]">
                    <span
                      className={cn(
                        "h-1.5 w-1.5 rounded-full shrink-0",
                        isNonTerminalStatus && "animate-pulse",
                      )}
                      style={{ backgroundColor: tint }}
                    />
                    <span
                      className="text-[11px] font-mono leading-none truncate"
                      style={{ color: tint }}
                      title={STATUS_LABEL[s]}
                    >
                      {STATUS_LABEL[s]}
                    </span>
                  </span>
                  {/* Slim status-tinted track. The faint inset groove uses the
                      same hue at low opacity so the unfilled remainder still
                      belongs to the status; the fill width animates. */}
                  <div
                    className="flex-1 min-w-0 h-1 overflow-hidden rounded-full"
                    style={{ backgroundColor: `${tint}1f` }}
                  >
                    <div
                      className="h-full rounded-full transition-[width] duration-500 ease-out"
                      style={{ width: `${pct}%`, backgroundColor: tint }}
                    />
                  </div>
                  {/* % aligned right, sharing the row baseline. */}
                  <span className="text-[10px] font-mono text-zinc-500 tabular-nums w-8 text-right shrink-0">
                    {pct}%
                  </span>
                </div>
              );
            },
          },
          {
            // Time column inside the Execution group: started (relative, primary,
            // sorts on started_at) with finished + duration as subtext. Both the
            // started AND finished date-range filters hang off this single header
            // popover, so neither wired window filter is lost.
            // Last column of the group → group banding + right divider.
            id: "time",
            // Relative start + finished/duration subtext — content-rich, sized
            // by table proportions and content rather than fixed pixels.
            meta: { group: "execution" },
            header: () => (
              <ColumnHeader
                label="Time"
                field="started_at"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={
                  !!query.startedAfter ||
                  !!query.startedBefore ||
                  !!query.finishedAfter ||
                  !!query.finishedBefore ||
                  query.durationMinSec !== undefined ||
                  query.durationMaxSec !== undefined
                }
                columnId="time"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <div>
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500">
                    Started
                  </div>
                  <DateRangeFilter
                    after={query.startedAfter}
                    before={query.startedBefore}
                    onChange={(after, before) =>
                      patchParams({ sa: after, sb: before })
                    }
                  />
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500 border-t border-zinc-800">
                    Finished
                  </div>
                  <DateRangeFilter
                    after={query.finishedAfter}
                    before={query.finishedBefore}
                    onChange={(after, before) =>
                      patchParams({ fa: after, fb: before })
                    }
                  />
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500 border-t border-zinc-800">
                    Duration (seconds)
                  </div>
                  <NumberRangeFilter
                    min={query.durationMinSec}
                    max={query.durationMaxSec}
                    lo={0}
                    hi={Number.MAX_SAFE_INTEGER}
                    unit="s"
                    onChange={(lo, hi) =>
                      patchParams({
                        dmin: lo === null ? null : String(lo),
                        dmax: hi === null ? null : String(hi),
                      })
                    }
                  />
                </div>
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const r = row.original;
              const sub = [
                r.finishedAt ? `→ ${relTime(r.finishedAt)}` : null,
                r.durationSec !== undefined ? formatDuration(r.durationSec) : null,
              ]
                .filter(Boolean)
                .join(" · ");
              return (
                <div className="flex flex-col gap-0.5">
                  <span
                    className="text-xs text-zinc-400 font-mono"
                    title={r.startedAt ?? ""}
                  >
                    {relTime(r.startedAt)}
                  </span>
                  {sub && (
                    <span
                      className="text-[10px] text-zinc-600 font-mono tabular-nums"
                      title={r.finishedAt ?? ""}
                    >
                      {sub}
                    </span>
                  )}
                </div>
              );
            },
          },
          {
            // Actions: one trailing cell containing row actions and the favorite
            // toggle, with favorite visually to the right of the kebab. The
            // header star toggles local favorites-first ordering.
            id: "actions",
            enableSorting: false,
            meta: {
              group: "execution",
              groupEdge: "right",
              className: "px-1",
              disableRowNavigation: true,
            },
            header: () => (
              <div className="flex h-full items-center justify-center gap-2">
                <span className="whitespace-nowrap leading-none">Actions</span>
                <button
                  type="button"
                  onClick={toggleFavoritesFirst}
                  className={cn(
                    HEADER_BTN,
                    query.favoritesFirst
                      ? "bg-warning/10 text-warning"
                      : "text-zinc-600 hover:bg-zinc-800/60 hover:text-warning",
                  )}
                  title={
                    query.favoritesFirst
                      ? "Favorites are shown first"
                      : "Show favorites first"
                  }
                  aria-label="Show favorites first"
                  aria-pressed={query.favoritesFirst}
                >
                  <Star
                    className={HEADER_ICON}
                    fill={query.favoritesFirst ? "currentColor" : "none"}
                  />
                </button>
              </div>
            ),
            cell: ({ row }) => {
              const run = row.original;
              return (
                <div className="flex h-full items-center justify-center gap-2">
                  <ActionsMenu
                    run={run}
                    open={openActionRunId === run.id}
                    onOpenChange={(o) => setOpenActionRunId(o ? run.id : null)}
                    onAction={runAction}
                  />
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      void toggleFavorite(run);
                    }}
                    className={cn(
                      "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
                      run.favorite
                        ? "text-warning"
                        : "text-zinc-600 hover:bg-zinc-800/60 hover:text-warning",
                    )}
                    title={
                      run.favorite
                        ? "Remove from favorites"
                        : "Add to favorites"
                    }
                    aria-label={
                      run.favorite
                        ? "Remove from favorites"
                        : "Add to favorites"
                    }
                    aria-pressed={run.favorite}
                  >
                    <Star
                      className="h-3.5 w-3.5"
                      fill={run.favorite ? "currentColor" : "none"}
                    />
                  </button>
                </div>
              );
            },
          },
        ],
      },
    ],
    // Re-build columns when sort/filter state changes so headers reflect it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      query.sort,
      query.desc,
      query.search,
      query.startedAfter,
      query.startedBefore,
      query.finishedAfter,
      query.finishedBefore,
      query.progressMin,
      query.progressMax,
      query.durationMinSec,
      query.durationMaxSec,
      query.authorIds,
      statusSet,
      dbSet,
      versionSet,
      protocolSet,
      triggerSet,
      authorSet,
      facets,
      authorDisplays,
      query.favoritesFirst,
      toggleFavoritesFirst,
      allVisibleSelected,
      someVisibleSelected,
      selectedRunIds,
      toggleVisibleSelection,
      toggleRunSelection,
      toggleFavorite,
      openFilterColumnId,
      openActionRunId,
      runAction,
    ],
  );

  const table = useReactTable({
    data: visibleRuns,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (row) => row.id,
    manualSorting: true,
    manualFiltering: true,
    manualPagination: true,
    // Tri-state sort: allow the sort to be removed entirely (neutral state) —
    // the page mirrors this in the URL (clears ?sort=&dir=).
    enableSortingRemoval: true,
  });

  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      {/* Header bar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">
            Test Runs
          </h1>
          <div className="flex items-center gap-2">
            <Button
              asChild
              size="sm"
              className="h-6 px-2 text-[11px] font-mono"
            >
              <Link to="/runs/new">
                <Plus className="h-3.5 w-3.5" />
                New
              </Link>
            </Button>
            {hasTableFilters && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={clearTableFilters}
                className="h-6 px-2 text-[11px] font-mono border-zinc-800 bg-transparent text-zinc-300 hover:bg-zinc-900 hover:text-zinc-100"
              >
                <X className="h-3.5 w-3.5" />
                Clear filters
              </Button>
            )}
            {selectedCount >= 2 && (
              <>
                <Button
                  asChild
                  variant="outline"
                  size="sm"
                  className="h-6 px-2 text-[11px] font-mono border-zinc-800 bg-transparent text-zinc-300 hover:bg-zinc-900 hover:text-zinc-100"
                >
                  <Link to={compareTo}>
                    <GitCompare className="h-3.5 w-3.5" />
                    Compare
                  </Link>
                </Button>
                <span className="text-[11px] text-zinc-500 font-mono tabular-nums">
                  {selectedCount} selected
                </span>
              </>
            )}
          </div>
          {!loading && (
            <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
              {runs.length} shown
            </span>
          )}
          {refreshing && (
            <span className="flex items-center gap-1 text-[10px] text-primary/70 font-mono">
              <Loader2 className="h-3 w-3 animate-spin" />
              updating
            </span>
          )}
        </div>

        <div className="flex items-center gap-3">
          {/* Inline boolean toggles, visible above the table (NOT in a
              dropdown): Standalone (only non-suite runs), Favorites, Deleted
              (include soft-deleted). Each wires a single URL param + query
              field and is applied by the provider. */}
          <div className="flex items-center gap-3">
            <ToolbarCheckbox
              checked={!!query.standalone}
              onChange={(on) => patchParams({ sa_only: on ? "true" : null })}
              label="Standalone"
              title="Show only standalone runs (not part of a suite)"
            />
            <ToolbarCheckbox
              checked={!!query.favoritesOnly}
              onChange={(on) => patchParams({ fav: on ? "1" : null })}
              label="Favorites"
              title="Show only runs you've favorited"
            />
            <ToolbarCheckbox
              checked={!!query.includeDeleted}
              onChange={(on) => patchParams({ del: on ? "1" : null })}
              label="Deleted"
              title="Include soft-deleted runs"
            />
          </div>

          {/* Auto-refresh: the manual "refresh now" button sits to the LEFT of
              the interval segmented control, sized to match the pills (same
              h-5 height + proportionate padding) so the cluster reads as one
              balanced row: [⟳]  [Off 5s 15s 30s 1m]. */}
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => void fetchRuns(true)}
              disabled={loading || refreshing}
              className="flex h-5 w-6 items-center justify-center text-zinc-500 hover:text-zinc-300 transition-colors cursor-pointer disabled:opacity-50 border border-zinc-800 hover:border-zinc-700"
              title="Refresh now"
            >
              <RefreshCw
                className={cn(
                  "h-3 w-3",
                  (loading || refreshing) && "animate-spin",
                )}
              />
            </button>
            <div className="flex gap-0.5">
              {REFRESH_OPTIONS.map((opt) => (
                <button
                  key={opt.ms}
                  type="button"
                  onClick={() => setRefreshMs(opt.ms)}
                  className={cn(
                    "flex h-5 items-center px-1.5 text-[10px] font-mono border transition-colors cursor-pointer",
                    query.refreshMs === opt.ms
                      ? "border-primary/40 text-primary bg-primary/5"
                      : "border-transparent text-zinc-600 hover:text-zinc-400",
                  )}
                  title={
                    opt.ms === 0
                      ? "Auto-refresh off"
                      : `Auto-refresh every ${opt.label}`
                  }
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs p-2.5 border border-destructive/30 text-destructive font-mono">
          <AlertCircle className="h-3.5 w-3.5 shrink-0" />
          {error}
        </div>
      )}

      {query.suiteRunId && (
        <div className="flex items-center gap-2 text-xs p-2 border border-primary/40 bg-primary/10 text-foreground font-mono">
          <Boxes className="h-3.5 w-3.5 shrink-0 text-primary" />
          <span>
            Scoped to suite run{" "}
            <span className="text-primary">{query.suiteRunId}</span> — child test runs only
          </span>
          <button
            type="button"
            onClick={() => patchParams({ suiteRun: null })}
            className="ml-auto inline-flex items-center gap-1 text-muted-foreground hover:text-foreground"
            title="Clear suite-run scope"
          >
            <X className="h-3.5 w-3.5" /> clear
          </button>
        </div>
      )}

      {/* Table — internal scroll: the body scrolls INSIDE this region with a
          sticky header; the page itself stays put. This region IS the scroll
          container, so the <thead>/<th> stick to its top. The table uses
          proportional colgroup hints with table-auto: content can still
          negotiate, but free space follows the intended column weights. It uses
          border-separate + border-spacing-0 (NOT
          border-collapse) so the sticky <th> keeps its divider — collapsed
          borders drop off sticky cells when they detach. Cell borders live on
          the cells themselves; the header carries an opaque bg + bottom border
          so scrolled rows never show through. */}
      <div className="border border-zinc-800/80 bg-[#080808] flex-1 min-h-0 overflow-auto">
        <table className="w-full table-auto caption-bottom text-sm border-separate border-spacing-0">
          <colgroup>
            {table.getVisibleLeafColumns().map((column) => (
              <col
                key={column.id}
                style={{ width: RUN_TABLE_COLUMN_WIDTHS[column.id] }}
              />
            ))}
          </colgroup>
          <thead>
            {/* Two header rows: the top row carries the parent GROUP labels
                (Controls | Run | Target | Execution), each spanning its leaf columns; the
                bottom row carries every leaf column's own sortable label + filter
                popover. Every leaf belongs to one group, so the grid stays
                rectangular and both rows can stay sticky. Row 1 sticks at top-0
                (h-6 ≈ 24px), row 2 sticks at top-6 right beneath it. */}
            {table.getHeaderGroups().map((hg, hgIndex) => {
              const isGroupRow = hgIndex === 0 && table.getHeaderGroups().length > 1;
              return (
                <tr key={hg.id}>
                  {hg.headers.map((header) => {
                    // Row 1 = the parent group labels (Run | Target | Execution):
                    // each spanning <th> brackets its whole block with BOTH side
                    // dividers + the group band. Row 2 = the leaf headers: each
                    // carries its own edge divider (left on the first leaf, right
                    // on the last) + the band, matching the body cells below.
                    const leafMeta = header.column.columnDef.meta;
                    const edgeClass = groupEdgeClass(leafMeta?.groupEdge);
                    return (
                    <th
                      key={header.id}
                      colSpan={header.colSpan}
                      className={cn(
                        // Headers are LEFT-aligned, pinned to the left edge of
                        // their column: the three group labels (row 1) and every
                        // leaf control row (row 2, rendered by ColumnHeader's own
                        // left-justified flex) sit flush-left over their members,
                        // matching the left-aligned body cells beneath them.
                        "text-left align-middle font-medium px-3",
                        "font-mono uppercase tracking-wider text-zinc-500",
                        "sticky z-20 bg-zinc-900",
                        "border-b border-zinc-800",
                        isGroupRow
                          ? "top-0 h-6 text-[10px]"
                          : "top-6 h-9 text-[11px]",
                        edgeClass,
                        leafMeta?.className,
                      )}
                    >
                      {header.isPlaceholder
                        ? null
                        : flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )}
                    </th>
                    );
                  })}
                </tr>
              );
            })}
          </thead>
          <tbody>
            {table.getRowModel().rows.length ? (
              table.getRowModel().rows.map((row) => (
                <tr
                  key={row.id}
                  className="group/run-row cursor-pointer transition-colors [&>td]:border-b [&>td]:border-zinc-800/50"
                  onClick={() => navigate(`/runs/${row.original.id}`)}
                >
                  {row.getVisibleCells().map((cell) => {
                    const meta = cell.column.columnDef.meta;
                    return (
                    <td
                      key={cell.id}
                      onClick={
                        meta?.disableRowNavigation
                          ? (e) => e.stopPropagation()
                          : undefined
                      }
                      className={cn(
                        // Uniform body-cell box: left-aligned textual content,
                        // vertically centered, consistent padding across columns.
                        "px-3 py-2.5 align-middle text-left",
                        // Subtle band behind EVERY group's body cells so each of
                        // the blocks (Controls | Run | Target | Execution) read as one
                        // unit. Hover lives on the cells themselves because cell
                        // backgrounds cover the <tr> background.
                        meta?.group && "bg-zinc-950/40",
                        "transition-colors group-hover/run-row:bg-zinc-800/60 group-hover/run-row:border-zinc-700/80",
                        meta?.disableRowNavigation && "cursor-default",
                        groupEdgeClass(meta?.groupEdge),
                        meta?.className,
                      )}
                    >
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                    );
                  })}
                </tr>
              ))
            ) : (
              <tr>
                <td
                  colSpan={table.getVisibleLeafColumns().length}
                  className="h-32 text-center px-3 align-middle"
                >
                  <span className="inline-flex items-center gap-2 text-xs text-zinc-600 font-mono">
                    {loading ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        Loading runs...
                      </>
                    ) : hasActiveFilters ? (
                      "No runs matching filters"
                    ) : (
                      "No runs yet"
                    )}
                  </span>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="flex items-center justify-between mt-auto">
        <div className="flex items-center gap-2">
          <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
          <div className="flex gap-0.5">
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                type="button"
                onClick={() => setPageSize(size)}
                className={`px-2 py-0.5 text-[11px] font-mono border transition-colors cursor-pointer ${
                  query.pageSize === size
                    ? "border-zinc-600 text-zinc-300 bg-zinc-800"
                    : "border-transparent text-zinc-600 hover:text-zinc-400"
                }`}
              >
                {size}
              </button>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-1">
          <span className="text-[11px] text-zinc-600 font-mono tabular-nums px-2">
            Page {pageIndex + 1}
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={goPrev}
            disabled={!canPrev || loading}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 w-7 p-0"
            onClick={goNext}
            disabled={!canNext || loading}
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
