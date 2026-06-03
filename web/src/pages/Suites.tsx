import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type RowData,
} from "@tanstack/react-table";

declare module "@tanstack/react-table" {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    className?: string;
    group?:
      | "controls"
      | "run"
      | "target"
      | "execution"
      | "definition"
      | "configuration"
      | "activity";
    groupEdge?: "left" | "right" | "both";
    disableRowNavigation?: boolean;
  }
}
import {
  AlertCircle,
  CalendarCheck,
  CalendarClock,
  CalendarX,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  ChevronsUpDown,
  Copy,
  Eye,
  Filter,
  Loader2,
  MoreHorizontal,
  Play,
  Plus,
  RefreshCw,
  Star,
  Trash2,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { Link, useNavigate, useSearchParams, useTenantSlug } from "@/lib/router";
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
  SUITE_PROVIDERS,
  actionsForSuite,
  getSuitesProvider,
  type RunStatus,
  type SuiteAction,
  type SuiteFacets,
  type SuiteProviderKind,
  type SuiteScheduleFilter,
  type SuiteSortField,
  type SuitesQuery,
  type SuiteVM,
} from "@/services/suites";

const STATUS_TINT: Record<RunStatus, string> = {
  pending: "#6b7280",
  running: "#3b82f6",
  cancelling: "#eab308",
  completed: "#22c55e",
  failed: "#ef4444",
  cancelled: "#71717a",
};

const STATUS_LABEL: Record<RunStatus, string> = {
  pending: "Pending",
  running: "Running",
  cancelling: "Cancelling",
  completed: "Completed",
  failed: "Failed",
  cancelled: "Cancelled",
};

const PROVIDER_LABEL: Record<Exclude<SuiteProviderKind, "">, string> = {
  docker: "Docker",
  yandex: "Yandex",
};

const PROVIDER_COLOR: Record<Exclude<SuiteProviderKind, "">, string> = {
  docker: "#38bdf8",
  yandex: "#a78bfa",
};

const PAGE_SIZES = [15, 25, 50, 100, 150];
const DEFAULT_SIZE = 25;

const REFRESH_OPTIONS: { label: string; ms: number }[] = [
  { label: "Off", ms: 0 },
  { label: "5s", ms: 5_000 },
  { label: "15s", ms: 15_000 },
  { label: "30s", ms: 30_000 },
  { label: "1m", ms: 60_000 },
];
const DEFAULT_REFRESH_MS = 15_000;

const SUITE_TABLE_COLUMN_WIDTHS: Record<string, string> = {
  name: "17%",
  authorId: "7%",
  provider: "8%",
  cells: "10%",
  schedule: "13%",
  nextRunAt: "9%",
  lastRun: "15%",
  runCount: "5%",
  time: "7%",
  actions: "9%",
};

function relTime(iso?: string): string {
  if (!iso) return "-";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "-";
  const diff = Date.now() - t;
  const abs = Math.abs(diff);
  const m = Math.round(abs / 60_000);
  const h = Math.round(abs / 3_600_000);
  const d = Math.round(abs / 86_400_000);
  const s = d >= 1 ? `${d}d` : h >= 1 ? `${h}h` : `${Math.max(m, 1)}m`;
  return diff >= 0 ? `${s} ago` : `in ${s}`;
}

function isoToDateInput(iso?: string): string {
  if (!iso) return "";
  return iso.slice(0, 10);
}

function dateInputToIso(value: string, endOfDay: boolean): string | null {
  if (!value) return null;
  return endOfDay ? `${value}T23:59:59.999Z` : `${value}T00:00:00.000Z`;
}

function tagEntries(suite: SuiteVM): string[] {
  const labels = Object.entries(suite.labels).map(([key, value]) =>
    value ? `${key}=${value}` : key,
  );
  return [...suite.tags, ...labels];
}

function cellSourceLabel(cell: SuiteVM["cells"][number]): string {
  if (cell.presetPair) {
    return `${cell.presetPair.dbPresetId}/${cell.presetPair.workloadPresetId}`;
  }
  if (cell.testPresetId) return cell.testPresetId;
  if (cell.inlineTest) return "inline test";
  return cell.id;
}

interface ParsedQuery extends SuitesQuery {
  pageSize: number;
  sort?: SuiteSortField;
  desc: boolean;
  refreshMs: number;
  favoritesFirst: boolean;
}

const SORTABLE_FIELDS: readonly SuiteSortField[] = [
  "name",
  "created_at",
  "updated_at",
  "provider",
  "schedule_enabled",
  "next_run_at",
  "last_run_at",
  "run_count",
];

function parseQuery(sp: URLSearchParams): ParsedQuery {
  const csv = (key: string): string[] =>
    (sp.get(key) ?? "")
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);

  const providers = csv("provider").filter(
    (p): p is Exclude<SuiteProviderKind, ""> =>
      (SUITE_PROVIDERS as string[]).includes(p),
  );
  const scheduleRaw = sp.get("sched");
  const schedule: SuiteScheduleFilter | undefined =
    scheduleRaw === "enabled" || scheduleRaw === "disabled"
      ? scheduleRaw
      : undefined;
  const sortRaw = sp.get("sort");
  const sort =
    sortRaw && SORTABLE_FIELDS.includes(sortRaw as SuiteSortField)
      ? (sortRaw as SuiteSortField)
      : undefined;
  const desc = sp.get("dir") ? sp.get("dir") === "desc" : true;
  const size = Number.parseInt(sp.get("size") ?? "", 10);
  const pageSize = PAGE_SIZES.includes(size) ? size : DEFAULT_SIZE;
  const ivRaw = Number.parseInt(sp.get("iv") ?? "", 10);
  const refreshMs = REFRESH_OPTIONS.some((o) => o.ms === ivRaw)
    ? ivRaw
    : DEFAULT_REFRESH_MS;
  const authorIds = csv("author");

  return {
    search: sp.get("q") ?? undefined,
    authorIds: authorIds.length ? authorIds : undefined,
    providers,
    schedule,
    favoritesOnly: sp.get("fav") === "1" ? true : undefined,
    includeDeleted: sp.get("del") === "1" ? true : undefined,
    createdAfter: sp.get("ca") ?? undefined,
    createdBefore: sp.get("cb") ?? undefined,
    updatedAfter: sp.get("ua") ?? undefined,
    updatedBefore: sp.get("ub") ?? undefined,
    sort,
    desc,
    pageSize,
    pageToken: sp.get("page") ?? undefined,
    refreshMs,
    favoritesFirst: sp.get("ff") !== "0",
  };
}

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

const HEADER_BTN =
  "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer shrink-0";
const HEADER_ICON = "h-3.5 w-3.5";

function SortButton({
  field,
  active,
  desc,
  onSort,
}: {
  field: SuiteSortField;
  active: boolean;
  desc: boolean;
  onSort: (field: SuiteSortField) => void;
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
            ? "Sorted descending - click for neutral"
            : "Sorted ascending - click for descending"
      }
      aria-label="Toggle sort"
    >
      <Icon className={HEADER_ICON} />
    </button>
  );
}

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
  field?: SuiteSortField;
  sort?: SuiteSortField;
  desc: boolean;
  onSort: (field: SuiteSortField) => void;
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
        <span
          className={cn(
            "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
            allSelected && "bg-primary border-primary",
          )}
        >
          {allSelected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
        </span>
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
              <span
                className={cn(
                  "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
                  isSelected && "bg-primary border-primary",
                )}
              >
                {isSelected && (
                  <Check className="h-2.5 w-2.5 text-primary-foreground" />
                )}
              </span>
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
            onChange(dateInputToIso(e.target.value, false), before ?? null)
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

const ACTION_ITEMS: {
  action: SuiteAction;
  label: string;
  icon: LucideIcon;
  danger?: boolean;
}[] = [
  { action: "view", label: "View detail", icon: Eye },
  { action: "start", label: "Start suite", icon: Play },
  { action: "clone", label: "Clone suite", icon: Copy },
  { action: "enableSchedule", label: "Enable schedule", icon: CalendarCheck },
  { action: "disableSchedule", label: "Disable schedule", icon: CalendarX },
  { action: "delete", label: "Delete", icon: Trash2, danger: true },
];

function ActionsMenu({
  suite,
  open,
  onOpenChange,
  onAction,
}: {
  suite: SuiteVM;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAction: (action: SuiteAction, suite: SuiteVM) => void;
}) {
  const allowed = actionsForSuite(suite);
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          onClick={(e) => e.stopPropagation()}
          className={cn(
            "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
            open
              ? "text-zinc-200 bg-zinc-800"
              : "text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800/60",
          )}
          title="Suite actions"
          aria-label="Suite actions"
        >
          <MoreHorizontal className="h-4 w-4" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        onClick={(e) => e.stopPropagation()}
        className="min-w-[11rem] bg-zinc-950 border-zinc-800"
      >
        <DropdownMenuLabel className="truncate" title={suite.name || suite.id}>
          {suite.name || suite.id}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        {ACTION_ITEMS.map((item) => {
          const enabled = allowed.has(item.action);
          return (
            <DropdownMenuItem
              key={item.action}
              disabled={!enabled}
              onSelect={(e) => {
                e.preventDefault();
                if (enabled) onAction(item.action, suite);
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

export function Suites() {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const query = useMemo(() => parseQuery(searchParams), [searchParams]);
  const confirm = useConfirm();

  const [suites, setSuites] = useState<SuiteVM[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [openFilterColumnId, setOpenFilterColumnId] = useState<string | null>(
    null,
  );
  const [openActionSuiteId, setOpenActionSuiteId] = useState<string | null>(
    null,
  );
  const [facets, setFacets] = useState<SuiteFacets>({ authorIds: [] });
  const tokenStackRef = useRef<string[]>([]);

  const popoverOpen = openFilterColumnId !== null || openActionSuiteId !== null;
  const handleFilterOpenChange = useCallback(
    (columnId: string, open: boolean) => {
      setOpenFilterColumnId(open ? columnId : (cur) => (cur === columnId ? null : cur));
    },
    [],
  );

  const fetchSuites = useCallback(
    async (background = false) => {
      if (!slug) return;
      if (background) setRefreshing(true);
      else setLoading(true);
      setError(null);
      try {
        const page = await getSuitesProvider().listSuites(slug, {
          search: query.search,
          authorIds: query.authorIds,
          providers: query.providers,
          schedule: query.schedule,
          favoritesOnly: query.favoritesOnly,
          includeDeleted: query.includeDeleted,
          createdAfter: query.createdAfter,
          createdBefore: query.createdBefore,
          updatedAfter: query.updatedAfter,
          updatedBefore: query.updatedBefore,
          sort: query.sort,
          desc: query.desc,
          pageSize: query.pageSize,
          pageToken: query.pageToken,
        });
        setSuites(page.suites);
        setNextPageToken(page.nextPageToken);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load suites");
        setSuites([]);
        setNextPageToken("");
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [
      slug,
      query.search,
      query.authorIds,
      query.providers,
      query.schedule,
      query.favoritesOnly,
      query.includeDeleted,
      query.createdAfter,
      query.createdBefore,
      query.updatedAfter,
      query.updatedBefore,
      query.sort,
      query.desc,
      query.pageSize,
      query.pageToken,
    ],
  );

  useEffect(() => {
    void fetchSuites(false);
  }, [fetchSuites]);

  useEffect(() => {
    if (!slug) return;
    let cancelled = false;
    void getSuitesProvider()
      .listFacets(slug)
      .then((f) => {
        if (!cancelled) setFacets(f);
      })
      .catch(() => {
        if (!cancelled) setFacets({ authorIds: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  useEffect(() => {
    if (query.refreshMs <= 0) return;
    if (popoverOpen) return;
    const id = setInterval(() => {
      void fetchSuites(true);
    }, query.refreshMs);
    return () => clearInterval(id);
  }, [query.refreshMs, popoverOpen, fetchSuites]);

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

  function toggleSort(field: SuiteSortField) {
    const active = query.sort === field;
    if (!active) patchParams({ sort: field, dir: "asc" });
    else if (!query.desc) patchParams({ sort: field, dir: "desc" });
    else patchParams({ sort: null, dir: null });
  }

  const setCsv = useCallback(
    (key: string, selected: Set<string>) => {
      patchParams({ [key]: [...selected].join(",") || null });
    },
    [patchParams],
  );

  function setPageSize(size: number) {
    patchParams({ size: size === DEFAULT_SIZE ? null : String(size) });
  }

  function setRefreshMs(ms: number) {
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

  const toggleFavoritesFirst = useCallback(() => {
    patchParams({ ff: query.favoritesFirst ? "0" : null }, false);
  }, [patchParams, query.favoritesFirst]);

  const authorSet = useMemo(
    () => new Set<string>(query.authorIds ?? []),
    [query.authorIds],
  );
  const providerSet = useMemo(
    () => new Set<string>(query.providers ?? []),
    [query.providers],
  );
  const scheduleSet = useMemo(
    () => new Set<string>(query.schedule ? [query.schedule] : []),
    [query.schedule],
  );

  const hasTableFilters =
    !!query.search ||
    (query.authorIds?.length ?? 0) > 0 ||
    (query.providers?.length ?? 0) > 0 ||
    !!query.schedule ||
    !!query.createdAfter ||
    !!query.createdBefore ||
    !!query.updatedAfter ||
    !!query.updatedBefore;
  const hasGlobalFilters = !!query.favoritesOnly || !!query.includeDeleted;
  const hasActiveFilters = hasTableFilters || hasGlobalFilters;

  function clearTableFilters() {
    patchParams({
      q: null,
      author: null,
      provider: null,
      sched: null,
      ca: null,
      cb: null,
      ua: null,
      ub: null,
    });
  }

  const visibleSuites = useMemo(() => {
    if (!query.favoritesFirst) return suites;
    const favoriteSuites: SuiteVM[] = [];
    const otherSuites: SuiteVM[] = [];
    for (const suite of suites) {
      if (suite.favorite) favoriteSuites.push(suite);
      else otherSuites.push(suite);
    }
    return [...favoriteSuites, ...otherSuites];
  }, [suites, query.favoritesFirst]);

  const pageIndex = tokenStackRef.current.length;
  const canPrev = pageIndex > 0;
  const canNext = !!nextPageToken;

  const toggleFavorite = useCallback(
    async (suite: SuiteVM) => {
      if (!slug) return;
      const nextFavorite = !suite.favorite;
      try {
        await getSuitesProvider().setFavorite(slug, suite.id, nextFavorite);
        setSuites((current) => {
          const next = current.map((item) =>
            item.id === suite.id ? { ...item, favorite: nextFavorite } : item,
          );
          if (query.favoritesOnly && !nextFavorite) {
            return next.filter((item) => item.id !== suite.id);
          }
          return next;
        });
        if (query.favoritesOnly && !nextFavorite) {
          await fetchSuites(true);
        }
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to update favorite",
        );
      }
    },
    [slug, query.favoritesOnly, fetchSuites],
  );

  // Quick (non-wizard) create — calls SuiteService.CreateSuite directly with a
  // minimal empty-spec suite, then opens its detail page to fill it in. The
  // wizard at /suites/new is the rich path; this is the one-click affordance.
  const [creating, setCreating] = useState(false);
  const quickCreate = useCallback(async () => {
    if (!slug || creating) return;
    setCreating(true);
    setError(null);
    try {
      const { suiteId } = await getSuitesProvider().createSuite(slug, {
        name: "Untitled suite",
        provider: "docker",
        cells: [],
      });
      if (suiteId) navigate(`/suites/${suiteId}`);
      else await fetchSuites(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create suite");
    } finally {
      setCreating(false);
    }
  }, [slug, creating, navigate, fetchSuites]);

  const runAction = useCallback(
    async (action: SuiteAction, suite: SuiteVM) => {
      if (!slug) return;
      setOpenActionSuiteId(null);
      const provider = getSuitesProvider();
      try {
        switch (action) {
          case "view":
            navigate(`/suites/${suite.id}`);
            return;
          case "start":
            await provider.startSuite(slug, suite.id);
            break;
          case "clone": {
            const result = await provider.cloneSuite(slug, suite.id);
            navigate(`/suites/${result.suiteId}`);
            break;
          }
          case "enableSchedule":
            await provider.setScheduleEnabled(slug, suite.id, true);
            break;
          case "disableSchedule":
            await provider.setScheduleEnabled(slug, suite.id, false);
            break;
          case "delete": {
            const ok = await confirm({
              title: "Delete suite?",
              description: `"${suite.name || suite.id}" will be soft-deleted. Past suite runs stay in run history.`,
              danger: true,
              confirmLabel: "Delete",
            });
            if (!ok) return;
            await provider.deleteSuite(slug, suite.id);
            break;
          }
        }
        await fetchSuites(true);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : `Failed to ${action} suite`,
        );
      }
    },
    [slug, navigate, confirm, fetchSuites],
  );

  const columns = useMemo<ColumnDef<SuiteVM>[]>(
    () => [
      {
        id: "definition",
        meta: { groupEdge: "right" },
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Definition
          </span>
        ),
        columns: [
          {
            accessorKey: "name",
            meta: { group: "definition" },
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
              const s = row.original;
              const tags = tagEntries(s);
              return (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    navigate(`/suites/${s.id}`);
                  }}
                  className="flex flex-col gap-0.5 min-w-0 text-left group/name cursor-pointer"
                >
                  <span
                    className="block max-w-full text-xs text-primary group-hover/name:underline underline-offset-2 truncate"
                    title={s.name || s.id}
                  >
                    {s.name || s.id}
                  </span>
                  <span
                    className="font-mono text-[10px] text-zinc-600 truncate"
                    title={s.id}
                  >
                    ~{s.id}
                  </span>
                  {s.description && (
                    <span
                      className="font-mono text-[10px] text-zinc-500 truncate"
                      title={s.description}
                    >
                      {s.description}
                    </span>
                  )}
                  {tags.length > 0 && (
                    <span className="flex flex-wrap gap-1 pt-0.5">
                      {tags.slice(0, 3).map((tag) => (
                        <span
                          key={tag}
                          className="max-w-[7rem] truncate border border-zinc-800 px-1 py-px text-[9px] font-mono leading-none text-zinc-500"
                          title={tag}
                        >
                          {tag}
                        </span>
                      ))}
                      {tags.length > 3 && (
                        <span className="text-[9px] font-mono leading-none text-zinc-600">
                          +{tags.length - 3}
                        </span>
                      )}
                    </span>
                  )}
                </button>
              );
            },
          },
          {
            accessorKey: "authorId",
            meta: { group: "definition", groupEdge: "right" },
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
                      label: a,
                    }))}
                    selected={authorSet}
                    onChange={(next) => setCsv("author", next)}
                  />
                ) : (
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
                return <span className="font-mono text-xs text-zinc-600">-</span>;
              return (
                <div className="flex items-center gap-2 min-w-0">
                  <Avatar name={author} size={22} className="shrink-0" />
                  <span
                    className="font-mono text-xs text-zinc-400 truncate"
                    title={author}
                  >
                    {author}
                  </span>
                </div>
              );
            },
          },
        ],
      },
      {
        id: "configuration",
        meta: { groupEdge: "both" },
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Configuration
          </span>
        ),
        columns: [
          {
            accessorKey: "provider",
            meta: { group: "configuration", groupEdge: "left" },
            header: () => (
              <ColumnHeader
                label="Provider"
                field="provider"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={providerSet.size > 0}
                columnId="provider"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <ChecklistFilter
                  options={SUITE_PROVIDERS.map((p) => ({
                    value: p,
                    label: PROVIDER_LABEL[p],
                    color: PROVIDER_COLOR[p],
                  }))}
                  selected={providerSet}
                  onChange={(next) => setCsv("provider", next)}
                />
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const p = row.original.provider;
              if (!p)
                return <span className="font-mono text-xs text-zinc-600">-</span>;
              return (
                <div className="flex items-center gap-2">
                  <span
                    className="h-2 w-2 rounded-full shrink-0"
                    style={{ backgroundColor: PROVIDER_COLOR[p] }}
                  />
                  <span
                    className="font-mono text-xs"
                    style={{ color: PROVIDER_COLOR[p] }}
                  >
                    {PROVIDER_LABEL[p]}
                  </span>
                </div>
              );
            },
          },
          {
            id: "cells",
            meta: { group: "configuration" },
            header: () => (
              <ColumnHeader
                label="Cells"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
              />
            ),
            cell: ({ row }) => {
              const s = row.original;
              const visibleCells = s.cells.slice(0, 3);
              const hiddenCellCount = Math.max(s.cells.length - visibleCells.length, 0);
              const cellTitle = s.cells
                .map(
                  (cell) =>
                    `${cell.name || cell.id}: ${cellSourceLabel(cell)}`,
                )
                .join("\n");
              const rating = [
                s.defaultInTenantRating === undefined
                  ? null
                  : s.defaultInTenantRating
                    ? "tenant rating"
                    : "no tenant rating",
                s.defaultInGlobalRating === undefined
                  ? null
                  : s.defaultInGlobalRating
                    ? "global rating"
                    : "no global rating",
              ]
                .filter(Boolean)
                .join(" · ");
              return (
                <div
                  className="flex max-h-24 min-w-0 flex-col gap-1 overflow-hidden"
                  title={cellTitle || undefined}
                >
                  <div className="flex items-baseline gap-1.5">
                    <span className="font-mono text-xs text-zinc-300 tabular-nums">
                      {s.cellCount}/{s.cells.length}
                    </span>
                    <span className="font-mono text-[10px] text-zinc-600">
                      cells
                    </span>
                  </div>
                  <span className="font-mono text-[10px] text-zinc-600 truncate">
                    {s.defaultMaxParallel
                      ? `max ${s.defaultMaxParallel}`
                      : "unlimited"}
                    {rating ? ` · ${rating}` : ""}
                  </span>
                  {visibleCells.length > 0 && (
                    <div className="flex min-w-0 flex-col gap-0.5">
                      {visibleCells.map((cell) => (
                        <span
                          key={cell.id}
                          className="block truncate font-mono text-[10px] leading-tight text-zinc-500"
                        >
                          {cell.name || cell.id}
                        </span>
                      ))}
                      {hiddenCellCount > 0 && (
                        <span className="font-mono text-[10px] leading-tight text-zinc-600">
                          +{hiddenCellCount}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              );
            },
          },
          {
            id: "schedule",
            meta: { group: "configuration", groupEdge: "right" },
            header: () => (
              <ColumnHeader
                label="Schedule"
                field="schedule_enabled"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={scheduleSet.size > 0}
                columnId="schedule"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <ChecklistFilter
                  options={[
                    { value: "enabled", label: "Enabled" },
                    { value: "disabled", label: "Disabled" },
                  ]}
                  selected={scheduleSet}
                  onChange={(next) => {
                    const value =
                      [...next].find((v) => v !== query.schedule) ??
                      [...next][0];
                    patchParams({
                      sched:
                        value === "enabled" || value === "disabled"
                          ? value
                          : null,
                    });
                  }}
                />
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const s = row.original;
              const Icon = s.scheduleEnabled ? CalendarCheck : CalendarX;
              return (
                <div className="flex items-center gap-2 min-w-0">
                  <Icon
                    className={cn(
                      "h-3.5 w-3.5 shrink-0",
                      s.scheduleEnabled ? "text-success" : "text-zinc-600",
                    )}
                  />
                  <div className="flex flex-col gap-0.5 min-w-0">
                    <span
                      className={cn(
                        "font-mono text-xs",
                        s.scheduleEnabled ? "text-success" : "text-zinc-500",
                      )}
                    >
                      {s.scheduleEnabled ? "Enabled" : "Disabled"}
                    </span>
                    <span className="font-mono text-[10px] text-zinc-600 truncate">
                      {s.cron || "-"} {s.timezone ? `· ${s.timezone}` : ""}
                    </span>
                  </div>
                </div>
              );
            },
          },
        ],
      },
      {
        id: "activity",
        meta: { groupEdge: "both" },
        header: () => (
          <span className="text-[10px] font-mono uppercase tracking-[0.15em] text-zinc-600">
            Activity
          </span>
        ),
        columns: [
          {
            id: "nextRunAt",
            meta: { group: "activity", groupEdge: "left" },
            header: () => (
              <ColumnHeader
                label="Next"
                field="next_run_at"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
              />
            ),
            cell: ({ row }) => {
              const s = row.original;
              if (!s.nextRunAt)
                return <span className="font-mono text-xs text-zinc-600">-</span>;
              return (
                <div className="flex items-center gap-1.5">
                  <CalendarClock className="h-3.5 w-3.5 text-zinc-500 shrink-0" />
                  <span className="font-mono text-xs text-zinc-400">
                    {relTime(s.nextRunAt)}
                  </span>
                </div>
              );
            },
          },
          {
            id: "lastRun",
            meta: { group: "activity" },
            header: () => (
              <ColumnHeader
                label="Last Run"
                field="last_run_at"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
              />
            ),
            cell: ({ row }) => {
              const s = row.original;
              if (!s.lastRunStatus)
                return <span className="font-mono text-xs text-zinc-600">-</span>;
              const tint = STATUS_TINT[s.lastRunStatus];
              const isNonTerminal =
                s.lastRunStatus === "pending" ||
                s.lastRunStatus === "running" ||
                s.lastRunStatus === "cancelling";
              return (
                <div className="flex items-center gap-2 min-w-0">
                  <span className="flex items-center gap-1.5 shrink-0 w-[5.25rem]">
                    <span
                      className={cn(
                        "h-1.5 w-1.5 rounded-full shrink-0",
                        isNonTerminal && "animate-pulse",
                      )}
                      style={{ backgroundColor: tint }}
                    />
                    <span
                      className="text-[11px] font-mono leading-none truncate"
                      style={{ color: tint }}
                      title={STATUS_LABEL[s.lastRunStatus]}
                    >
                      {STATUS_LABEL[s.lastRunStatus]}
                    </span>
                  </span>
                  <span className="font-mono text-[10px] text-zinc-600 truncate">
                    {relTime(s.lastRunAt)}
                  </span>
                </div>
              );
            },
          },
          {
            id: "runCount",
            meta: { group: "activity", className: "text-center" },
            header: () => (
              <ColumnHeader
                label="Runs"
                field="run_count"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
              />
            ),
            cell: ({ row }) => (
              <span className="font-mono text-xs text-zinc-300 tabular-nums">
                {row.original.runCount}
              </span>
            ),
          },
          {
            id: "time",
            meta: { group: "activity" },
            header: () => (
              <ColumnHeader
                label="Time"
                field="updated_at"
                sort={query.sort}
                desc={query.desc}
                onSort={toggleSort}
                active={
                  !!query.createdAfter ||
                  !!query.createdBefore ||
                  !!query.updatedAfter ||
                  !!query.updatedBefore
                }
                columnId="time"
                openColumnId={openFilterColumnId}
                onOpenChange={handleFilterOpenChange}
              >
                <div>
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500">
                    Created
                  </div>
                  <DateRangeFilter
                    after={query.createdAfter}
                    before={query.createdBefore}
                    onChange={(after, before) =>
                      patchParams({ ca: after, cb: before })
                    }
                  />
                  <div className="px-3 pt-2 pb-1 text-[9px] font-mono uppercase tracking-wider text-zinc-500 border-t border-zinc-800">
                    Updated
                  </div>
                  <DateRangeFilter
                    after={query.updatedAfter}
                    before={query.updatedBefore}
                    onChange={(after, before) =>
                      patchParams({ ua: after, ub: before })
                    }
                  />
                </div>
              </ColumnHeader>
            ),
            cell: ({ row }) => {
              const s = row.original;
              return (
                <div className="flex flex-col gap-0.5">
                  <span className="font-mono text-xs text-zinc-400">
                    {relTime(s.updatedAt)}
                  </span>
                  <span className="font-mono text-[10px] text-zinc-600">
                    cr {relTime(s.createdAt)}
                  </span>
                  {s.deletedAt && (
                    <span className="font-mono text-[10px] text-destructive/70">
                      del {relTime(s.deletedAt)}
                    </span>
                  )}
                </div>
              );
            },
          },
          {
            id: "actions",
            enableSorting: false,
            meta: {
              group: "activity",
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
              const suite = row.original;
              return (
                <div className="flex h-full items-center justify-center gap-2">
                  <ActionsMenu
                    suite={suite}
                    open={openActionSuiteId === suite.id}
                    onOpenChange={(o) =>
                      setOpenActionSuiteId(o ? suite.id : null)
                    }
                    onAction={runAction}
                  />
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      void toggleFavorite(suite);
                    }}
                    className={cn(
                      "flex h-6 w-6 items-center justify-center rounded transition-colors cursor-pointer",
                      suite.favorite
                        ? "text-warning"
                        : "text-zinc-600 hover:bg-zinc-800/60 hover:text-warning",
                    )}
                    title={
                      suite.favorite
                        ? "Remove from favorites"
                        : "Add to favorites"
                    }
                    aria-label={
                      suite.favorite
                        ? "Remove from favorites"
                        : "Add to favorites"
                    }
                    aria-pressed={suite.favorite}
                  >
                    <Star
                      className="h-3.5 w-3.5"
                      fill={suite.favorite ? "currentColor" : "none"}
                    />
                  </button>
                </div>
              );
            },
          },
        ],
      },
    ],
    [
      query.sort,
      query.desc,
      query.search,
      query.createdAfter,
      query.createdBefore,
      query.updatedAfter,
      query.updatedBefore,
      query.authorIds,
      query.favoritesFirst,
      authorSet,
      providerSet,
      scheduleSet,
      facets,
      openFilterColumnId,
      openActionSuiteId,
      handleFilterOpenChange,
      patchParams,
      setCsv,
      toggleFavoritesFirst,
      toggleFavorite,
      runAction,
      navigate,
    ],
  );

  const table = useReactTable({
    data: visibleSuites,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: (row) => row.id,
    manualSorting: true,
    manualFiltering: true,
    manualPagination: true,
    enableSortingRemoval: true,
  });

  return (
    <div className="p-5 flex flex-col gap-4 h-full min-h-0">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <h1 className="text-base font-semibold font-mono tracking-tight">
            Suites
          </h1>
          <div className="flex items-center gap-2">
            <Button
              asChild
              size="sm"
              className="h-6 px-2 text-[11px] font-mono"
            >
              <Link to="/suites/new">
                <Plus className="h-3.5 w-3.5" />
                New
              </Link>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={quickCreate}
              disabled={creating}
              title="Create an empty suite directly (CreateSuite), then edit it"
              className="h-6 px-2 text-[11px] font-mono border-zinc-800 bg-transparent text-zinc-300 hover:bg-zinc-900 hover:text-zinc-100"
            >
              {creating ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Plus className="h-3.5 w-3.5" />
              )}
              Quick create
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
          </div>
          {!loading && (
            <span className="text-[11px] text-zinc-600 font-mono tabular-nums">
              {suites.length} shown
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
          <div className="flex items-center gap-3">
            <ToolbarCheckbox
              checked={!!query.favoritesOnly}
              onChange={(on) => patchParams({ fav: on ? "1" : null })}
              label="Favorites"
              title="Show only suites you've favorited"
            />
            <ToolbarCheckbox
              checked={!!query.includeDeleted}
              onChange={(on) => patchParams({ del: on ? "1" : null })}
              label="Deleted"
              title="Include soft-deleted suites"
            />
          </div>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={() => void fetchSuites(true)}
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

      <div className="border border-zinc-800/80 bg-[#080808] flex-1 min-h-0 overflow-auto">
        <table className="w-full table-fixed caption-bottom text-sm border-separate border-spacing-0">
          <colgroup>
            {table.getVisibleLeafColumns().map((column) => (
              <col
                key={column.id}
                style={{ width: SUITE_TABLE_COLUMN_WIDTHS[column.id] }}
              />
            ))}
          </colgroup>
          <thead>
            {table.getHeaderGroups().map((hg, hgIndex) => {
              const isGroupRow = hgIndex === 0 && table.getHeaderGroups().length > 1;
              return (
                <tr key={hg.id}>
                  {hg.headers.map((header) => {
                    const meta = header.column.columnDef.meta;
                    return (
                    <th
                      key={header.id}
                      colSpan={header.colSpan}
                      className={cn(
                        "text-left align-middle font-medium px-3",
                        "font-mono uppercase tracking-wider text-zinc-500",
                        "sticky z-20 bg-zinc-900",
                        "border-b border-zinc-800",
                        isGroupRow
                          ? "top-0 h-6 text-[10px]"
                          : "top-6 h-9 text-[11px]",
                        groupEdgeClass(meta?.groupEdge),
                        meta?.className,
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
                  className="group/suite-row cursor-pointer transition-colors [&>td]:border-b [&>td]:border-zinc-800/50"
                  onClick={() => navigate(`/suites/${row.original.id}`)}
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
                        "px-3 py-2.5 align-middle text-left",
                        meta?.group && "bg-zinc-950/40",
                        "transition-colors group-hover/suite-row:bg-zinc-800/60 group-hover/suite-row:border-zinc-700/80",
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
                        Loading suites...
                      </>
                    ) : hasActiveFilters ? (
                      "No suites matching filters"
                    ) : (
                      "No suites yet"
                    )}
                  </span>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between mt-auto">
        <div className="flex items-center gap-2">
          <span className="text-[11px] text-zinc-600 font-mono">Rows</span>
          <div className="flex gap-0.5">
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                type="button"
                onClick={() => setPageSize(size)}
                className={cn(
                  "px-2 py-0.5 text-[11px] font-mono border transition-colors cursor-pointer",
                  query.pageSize === size
                    ? "border-zinc-600 text-zinc-300 bg-zinc-800"
                    : "border-transparent text-zinc-600 hover:text-zinc-400",
                )}
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
