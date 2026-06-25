import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  ArrowUpRight,
  CalendarClock,
  CalendarX,
  Check,
  ChevronDown,
  ChevronRight,
  Clock,
  Copy,
  Database,
  GitBranch,
  Loader2,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Star,
  Trash2,
  X,
  XCircle,
} from "lucide-react";
import { Link, useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { roleAtLeast } from "@/lib/roles";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import {
  fallbackAuthorDisplay,
  resolveAuthorDisplay,
  type AuthorDisplay,
} from "@/lib/author-display";
import type { TenantRole } from "@/services/auth";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumField } from "@/components/ui/num-field";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/utils";
import {
  getSuitesProvider,
  SUITE_PROVIDERS,
  type RunStatus,
  type SuiteProviderKind,
  type SuiteVM,
} from "@/services/suites";
import type { DbKind } from "@/services/runs";
import {
  getPresetProvider,
  type DatabasePresetRow,
  type TestPresetRow,
  type WorkloadPresetRow,
} from "@/services/preset";
import {
  getSuiteRunsProvider,
  isSuiteRunActive,
  type SuiteRunVM,
} from "@/services/suiteRuns";

// --- display maps (mirror the Suites list page tokens) ----------------------

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

// Per-db tint for the matrix axis + run db chips.
const DB_TINT: Record<Exclude<DbKind, "">, string> = {
  postgres: "#5B93C4",
  mysql: "#F2A94B",
  mariadb: "#6E91A8",
  ydb: "#facc15",
  ydb_managed: "#fbbf24",
  cockroach: "#60a5fa",
  picodata: "#F06580",
  orioledb: "#E8633A",
  external: "#a1a1aa",
};

const DB_LABEL: Record<Exclude<DbKind, "">, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  mariadb: "MariaDB",
  ydb: "YDB",
  ydb_managed: "YDB Managed",
  cockroach: "CockroachDB",
  picodata: "Picodata",
  orioledb: "OrioleDB",
  external: "External",
};

function dbTint(kind: DbKind): string {
  return kind === "" ? "#a1a1aa" : DB_TINT[kind];
}

function dbLabel(kind: DbKind): string {
  return kind === "" ? "—" : DB_LABEL[kind];
}

// --- formatters -------------------------------------------------------------

function formatTimestamp(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return "—";
  return d.toLocaleString("en-GB", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function relTime(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const diff = Date.now() - t;
  const past = diff >= 0;
  const s = Math.abs(diff) / 1000;
  const fmt = (n: number, u: string) => `${Math.round(n)}${u}`;
  let out: string;
  if (s < 60) out = fmt(s, "s");
  else if (s < 3600) out = fmt(s / 60, "m");
  else if (s < 86400) out = fmt(s / 3600, "h");
  else out = fmt(s / 86400, "d");
  return past ? `${out} ago` : `in ${out}`;
}

function formatDuration(sec?: number): string {
  if (sec === undefined || sec <= 0) return "—";
  const h = Math.floor(sec / 3600);
  const m = Math.floor((sec % 3600) / 60);
  const s = Math.floor(sec % 60);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

// --- small pieces -----------------------------------------------------------

function StatusBadge({ status }: { status: RunStatus }) {
  const tint = STATUS_TINT[status];
  const spin = status === "running" || status === "cancelling";
  return (
    <span
      className="inline-flex items-center gap-1.5 px-2 py-0.5 text-[11px] font-mono"
      style={{ color: tint, backgroundColor: `${tint}1f` }}
    >
      <span
        className={cn("h-1.5 w-1.5 rounded-full", spin && "animate-pulse")}
        style={{ backgroundColor: tint }}
      />
      {STATUS_LABEL[status]}
    </span>
  );
}

function MetaItem({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        {label}
      </span>
      <span className="text-xs text-foreground">{children}</span>
    </div>
  );
}

function ProgressBar({
  run,
}: {
  run: Pick<SuiteRunVM, "completed" | "failed" | "running" | "pending" | "total">;
}) {
  const total = run.total || 1;
  const pct = (n: number) => `${(n / total) * 100}%`;
  return (
    <div className="flex h-1.5 w-full overflow-hidden rounded-full bg-muted">
      <div style={{ width: pct(run.completed), backgroundColor: "#22c55e" }} />
      <div style={{ width: pct(run.failed), backgroundColor: "#ef4444" }} />
      <div style={{ width: pct(run.running), backgroundColor: "#3b82f6" }} />
    </div>
  );
}

// ============================================================================

export function SuiteDetail() {
  const { id } = useParams<{ id: string }>();
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { user } = useAuth();

  const role: TenantRole | undefined = user?.tenants.find(
    (t) => t.slug === slug,
  )?.role;
  const isAdmin = user?.isAdmin ?? false;
  // operator+ (or platform admin) may author/run/destroy; viewers are read-only.
  const canOperate = isAdmin || (role ? roleAtLeast(role, "operator") : false);

  const [suite, setSuite] = useState<SuiteVM | null>(null);
  const [runs, setRuns] = useState<SuiteRunVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [editingSettings, setEditingSettings] = useState(false);
  const [ownerDisplay, setOwnerDisplay] = useState<AuthorDisplay | null>(null);
  // Which suite run is expanded to reveal its child test runs.
  const [expandedRunId, setExpandedRunId] = useState<string | null>(null);

  useBreadcrumbLabel("id", suite?.name);

  const load = useCallback(
    async (showSpinner = false) => {
      if (!id) return;
      if (showSpinner) setLoading(true);
      try {
        const [s, page] = await Promise.all([
          getSuitesProvider().getSuite(slug ?? "", id),
          getSuiteRunsProvider().listSuiteRuns(slug ?? "", { suiteId: id }),
        ]);
        if (!s) {
          setNotFound(true);
          setSuite(null);
        } else {
          setSuite(s);
          setNotFound(false);
        }
        setRuns(page.runs);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load suite");
      } finally {
        setLoading(false);
      }
    },
    [id, slug],
  );

  useEffect(() => {
    void load(true);
  }, [load]);

  useEffect(() => {
    const authorId = suite?.authorId ?? "";
    if (!authorId) {
      setOwnerDisplay(null);
      return;
    }
    let cancelled = false;
    setOwnerDisplay(fallbackAuthorDisplay(authorId));
    void resolveAuthorDisplay(authorId).then((display) => {
      if (!cancelled) setOwnerDisplay(display);
    });
    return () => {
      cancelled = true;
    };
  }, [suite?.authorId]);

  // Live refresh while any run is active (the mock progresses runs on a timer).
  const hasActive = useMemo(
    () => runs.some((r) => isSuiteRunActive(r.status)),
    [runs],
  );
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    if (!hasActive) return;
    const t = setInterval(() => void loadRef.current(false), 2000);
    return () => clearInterval(t);
  }, [hasActive]);

  // --- actions --------------------------------------------------------------

  const runNow = useCallback(async () => {
    if (!suite) return;
    setBusy(true);
    try {
      const { suiteRunId } = await getSuitesProvider().startSuite(
        slug ?? "",
        suite.id,
      );
      await load(false);
      // Reveal the freshly-minted run row (it lands at the top of the list).
      requestAnimationFrame(() => {
        document
          .getElementById(`run-${suiteRunId}`)
          ?.scrollIntoView({ behavior: "smooth", block: "nearest" });
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to start suite");
    } finally {
      setBusy(false);
    }
  }, [suite, slug, load]);

  const duplicate = useCallback(async () => {
    if (!suite) return;
    setBusy(true);
    try {
      const { suiteId } = await getSuitesProvider().cloneSuite(
        slug ?? "",
        suite.id,
      );
      navigate(`/suites/${suiteId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to clone suite");
    } finally {
      setBusy(false);
    }
  }, [suite, slug, navigate]);

  const removeSuite = useCallback(async () => {
    if (!suite) return;
    const ok = await confirm({
      title: `Delete suite "${suite.name}"?`,
      description:
        "The suite definition is soft-deleted. Past suite runs keep their snapshots.",
      danger: true,
    });
    if (!ok) return;
    setBusy(true);
    try {
      await getSuitesProvider().deleteSuite(slug ?? "", suite.id);
      navigate("/suites");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete suite");
      setBusy(false);
    }
  }, [suite, slug, confirm, navigate]);

  const toggleFavorite = useCallback(async () => {
    if (!suite) return;
    const next = !suite.favorite;
    setSuite({ ...suite, favorite: next }); // optimistic
    try {
      await getSuitesProvider().setFavorite(slug ?? "", suite.id, next);
    } catch {
      setSuite((s) => (s ? { ...s, favorite: !next } : s)); // revert
    }
  }, [suite, slug]);

  const toggleCell = useCallback(
    async (cellId: string, enabled: boolean) => {
      if (!suite) return;
      setSuite({
        ...suite,
        cells: suite.cells.map((c) =>
          c.id === cellId ? { ...c, enabled } : c,
        ),
        cellCount: suite.cells.filter((c) =>
          c.id === cellId ? enabled : c.enabled,
        ).length,
      });
      try {
        await getSuitesProvider().setCellEnabled(
          slug ?? "",
          suite.id,
          cellId,
          enabled,
        );
      } catch {
        void load(false);
      }
    },
    [suite, slug, load],
  );

  const removeCell = useCallback(
    async (cellId: string) => {
      if (!suite) return;
      // No confirm — cell removal is cheap/reversible (just re-add it).
      try {
        await getSuitesProvider().removeCell(slug ?? "", suite.id, cellId);
        await load(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to remove cell");
      }
    },
    [suite, slug, load],
  );

  // One-click add at a (database-preset, workload-preset) intersection. The
  // slot already knows BOTH axes — no dialog, no re-pick. The cell is a
  // preset_pair, resolved to its db/workload axes by the provider→mock.
  const addCellAt = useCallback(
    async (dbPresetId: string, workloadPresetId: string) => {
      if (!suite) return;
      try {
        await getSuitesProvider().addCell(slug ?? "", suite.id, {
          name: "",
          enabled: true,
          source: "presetPair",
          dbPresetId,
          workloadPresetId,
        });
        await load(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to add cell");
      }
    },
    [suite, slug, load],
  );

  // Add a test-preset cell (the "other cells" surface below the matrix).
  const addTestPresetCell = useCallback(
    async (testPresetId: string) => {
      if (!suite) return;
      try {
        await getSuitesProvider().addCell(slug ?? "", suite.id, {
          name: "",
          enabled: true,
          source: "testPreset",
          testPresetId,
        });
        await load(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to add cell");
      }
    },
    [suite, slug, load],
  );

  const cancelRun = useCallback(
    async (run: SuiteRunVM) => {
      const ok = await confirm({
        title: `Cancel suite run "${run.id}"?`,
        description:
          "Cancels queued + running child runs. Finished children keep their results.",
        danger: true,
      });
      if (!ok) return;
      try {
        await getSuiteRunsProvider().cancelSuiteRun(slug ?? "", run.id);
        await load(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to cancel run");
      }
    },
    [slug, confirm, load],
  );

  const deleteRun = useCallback(
    async (run: SuiteRunVM) => {
      const ok = await confirm({
        title: `Delete suite run "${run.id}"?`,
        description: "Removes this suite run from the list.",
        danger: true,
      });
      if (!ok) return;
      try {
        await getSuiteRunsProvider().deleteSuiteRun(slug ?? "", run.id);
        await load(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to delete run");
      }
    },
    [slug, confirm, load],
  );

  // --- render states --------------------------------------------------------

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading suite…
      </div>
    );
  }

  if (notFound) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <CalendarX className="h-8 w-8 text-zinc-600" />
        <div className="text-sm text-foreground">Suite not found</div>
        <div className="text-xs text-muted-foreground">
          The suite <span className="font-mono">{id}</span> does not exist or was
          removed.
        </div>
        <Button asChild variant="outline" size="sm">
          <Link to="/suites">
            <ArrowLeft className="h-3.5 w-3.5" /> Back to Suites
          </Link>
        </Button>
      </div>
    );
  }

  if (!suite) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="flex items-center gap-2 border border-destructive/30 p-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error ?? "Failed to load suite"}
        </div>
      </div>
    );
  }

  const provider = suite.provider;
  const settingsEditable = canOperate && !suite.deleted;

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 p-5">
      {/* error banner */}
      {error && (
        <div className="flex items-center justify-between gap-2 border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <span className="flex items-center gap-2">
            <AlertCircle className="h-3.5 w-3.5" />
            {error}
          </span>
          <button onClick={() => setError(null)} aria-label="Dismiss">
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* HEADER (identity + actions) */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <button
            onClick={toggleFavorite}
            className="mt-0.5 shrink-0"
            aria-label={suite.favorite ? "Unfavorite" : "Favorite"}
            title={suite.favorite ? "Remove from favorites" : "Add to favorites"}
          >
            <Star
              className={cn(
                "h-5 w-5 transition-colors",
                suite.favorite
                  ? "fill-yellow-400 text-yellow-400"
                  : "text-zinc-600 hover:text-zinc-400",
              )}
            />
          </button>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="truncate font-mono text-lg font-semibold tracking-tight">
                {suite.name}
              </h1>
              {suite.deleted && (
                <Badge variant="destructive" className="font-mono">
                  Deleted
                </Badge>
              )}
            </div>
            <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
              <span className="font-mono">{suite.id}</span>
              {provider !== "" && (
                <span
                  className="inline-flex items-center gap-1 px-1.5 py-0.5 font-mono"
                  style={{
                    color: PROVIDER_COLOR[provider],
                    backgroundColor: `${PROVIDER_COLOR[provider]}1f`,
                  }}
                >
                  {PROVIDER_LABEL[provider]}
                </span>
              )}
            </div>
          </div>
        </div>

        {/* ACTIONS */}
        <div className="flex shrink-0 items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => void load(false)}
            title="Refresh"
          >
            <RefreshCw className="h-3.5 w-3.5" />
          </Button>
          {canOperate && !suite.deleted && (
            <>
              <Button size="sm" onClick={() => void runNow()} disabled={busy}>
                <Play className="h-3.5 w-3.5" /> Run now
              </Button>
              <Button asChild variant="outline" size="sm">
                <Link to={`/suites/${suite.id}/edit`}>
                  <Pencil className="h-3.5 w-3.5" /> Edit
                </Link>
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void duplicate()}
                disabled={busy}
              >
                <Copy className="h-3.5 w-3.5" /> Duplicate
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void removeSuite()}
                disabled={busy}
                className="text-destructive hover:text-destructive"
              >
                <Trash2 className="h-3.5 w-3.5" /> Delete
              </Button>
            </>
          )}
        </div>
      </div>

      {/* SETTINGS / OVERVIEW — all suite settings consolidated here. */}
      <SettingsCard
        suite={suite}
        ownerDisplay={ownerDisplay}
        editable={settingsEditable}
        editing={editingSettings}
        onEditToggle={setEditingSettings}
        busy={busy}
        onSave={async (patch) => {
          setBusy(true);
          try {
            const p = getSuitesProvider();
            const sid = suite.id;
            const s = slug ?? "";
            // Batch every entity/spec field into ONE update — issuing them in
            // parallel made the txns collide on the same row (serialize-access
            // 40001) and could drop fields. Schedule uses its own RPC, so run it
            // sequentially afterwards rather than concurrently.
            await p.updateSettings(s, sid, {
              name: patch.name,
              description: patch.description,
              provider: patch.provider,
              rating: patch.rating,
              maxParallel: patch.maxParallel,
              tags: patch.tags,
            });
            if (patch.schedule !== undefined) {
              await p.setSchedule(s, sid, patch.schedule);
            }
            setEditingSettings(false);
            await load(false);
          } catch (err) {
            setError(
              err instanceof Error ? err.message : "Failed to save settings",
            );
          } finally {
            setBusy(false);
          }
        }}
      />

      {/* BODY: cells matrix | suite runs — 35% matrix / 65% runs on lg+, stacked on narrow. */}
      <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[35fr_65fr]">
        {/* CELLS */}
        <Card className="flex min-h-0 flex-col">
          <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
            <h2 className="flex items-center gap-2 font-mono text-sm font-semibold">
              <Database className="h-4 w-4 text-zinc-500" /> Cells
              <span className="text-xs font-normal text-muted-foreground">
                database × workload
              </span>
            </h2>
            <span className="text-[11px] font-mono text-muted-foreground">
              {suite.cellCount}/{suite.cells.length} enabled
            </span>
          </div>
          <div className="min-h-0 flex-1 overflow-auto p-4">
            <CellMatrix
              cells={suite.cells}
              tenantSlug={slug ?? ""}
              editable={settingsEditable}
              canToggle={canOperate && !suite.deleted}
              onToggle={(cellId, v) => void toggleCell(cellId, v)}
              onRemove={(cellId) => void removeCell(cellId)}
              onAddAt={(db, wl) => void addCellAt(db, wl)}
              onAddTestPreset={(tp) => void addTestPresetCell(tp)}
            />
          </div>
        </Card>

        {/* SUITE RUNS */}
        <Card className="flex min-h-0 flex-col">
          <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
            <h2 className="flex items-center gap-2 font-mono text-sm font-semibold">
              <Clock className="h-4 w-4 text-zinc-500" /> Suite runs
              <span className="text-xs font-normal text-muted-foreground">
                each run spawns one test run per enabled cell
              </span>
            </h2>
            <span className="text-[11px] font-mono text-muted-foreground">
              {runs.length} shown
            </span>
          </div>
          <div className="min-h-0 flex-1 overflow-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-6" />
                  <TableHead>Run</TableHead>
                  <TableHead>Trigger</TableHead>
                  <TableHead>Progress</TableHead>
                  <TableHead>Test runs</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Finished</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Parallel</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {runs.map((run) => {
                  const expanded = expandedRunId === run.id;
                  return (
                    <Fragment key={run.id}>
                      <TableRow
                        id={`run-${run.id}`}
                        className="cursor-pointer hover:bg-muted/40"
                        onClick={() =>
                          run.children.length > 0 &&
                          setExpandedRunId(expanded ? null : run.id)
                        }
                        title="Show this suite run's child test runs"
                      >
                        <TableCell className="px-1">
                          {run.children.length > 0 && (
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                setExpandedRunId(expanded ? null : run.id);
                              }}
                              className="flex h-6 w-6 items-center justify-center text-zinc-500 hover:text-foreground"
                              aria-label={
                                expanded
                                  ? "Hide child test runs"
                                  : "Show child test runs"
                              }
                              title={
                                expanded
                                  ? "Hide child test runs"
                                  : "Show child test runs"
                              }
                            >
                              {expanded ? (
                                <ChevronDown className="h-3.5 w-3.5" />
                              ) : (
                                <ChevronRight className="h-3.5 w-3.5" />
                              )}
                            </button>
                          )}
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-col gap-1">
                            <StatusBadge status={run.status} />
                            <span
                              className="font-mono text-[10px] text-muted-foreground"
                              title={run.id}
                            >
                              {run.name || run.id}
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <TriggerBadge trigger={run.trigger} />
                        </TableCell>
                        <TableCell className="min-w-[130px]">
                          <div className="flex flex-col gap-1">
                            <ProgressBar run={run} />
                            <span className="font-mono text-[10px] text-muted-foreground">
                              {run.progressPct}%
                            </span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <ChildCounts run={run} />
                        </TableCell>
                        <TableCell
                          className="whitespace-nowrap text-xs"
                          title={formatTimestamp(run.startedAt)}
                        >
                          {relTime(run.startedAt)}
                        </TableCell>
                        <TableCell
                          className="whitespace-nowrap text-xs"
                          title={formatTimestamp(run.finishedAt)}
                        >
                          {run.finishedAt ? relTime(run.finishedAt) : "—"}
                        </TableCell>
                        <TableCell className="whitespace-nowrap font-mono text-xs">
                          {formatDuration(run.durationSec)}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {run.maxParallel === 0 ? "∞" : run.maxParallel}
                        </TableCell>
                        <TableCell>
                          <div className="flex items-center justify-end gap-1.5">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-7 px-2 text-primary hover:text-primary"
                              title="Open this run's test runs in the Runs table"
                              onClick={(e) => {
                                e.stopPropagation();
                                navigate(`/runs?suiteRun=${run.id}`);
                              }}
                            >
                              <ArrowUpRight className="h-3.5 w-3.5" /> Runs
                            </Button>
                            {canOperate && isSuiteRunActive(run.status) && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 text-warning hover:text-warning"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  void cancelRun(run);
                                }}
                              >
                                <XCircle className="h-3.5 w-3.5" /> Cancel
                              </Button>
                            )}
                            {canOperate && !isSuiteRunActive(run.status) && (
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 text-destructive hover:text-destructive"
                                onClick={(e) => {
                                  e.stopPropagation();
                                  void deleteRun(run);
                                }}
                              >
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            )}
                          </div>
                        </TableCell>
                      </TableRow>
                      {expanded && run.children.length > 0 && (
                        <TableRow className="hover:bg-transparent">
                          <TableCell colSpan={10} className="bg-muted/20 p-0">
                            <ChildTestRuns run={run} />
                          </TableCell>
                        </TableRow>
                      )}
                    </Fragment>
                  );
                })}
                {runs.length === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={10}
                      className="py-8 text-center text-xs text-muted-foreground"
                    >
                      No runs yet. {canOperate && "Use Run now to launch this suite."}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </Card>
      </div>
    </div>
  );
}

// --- suite-run table pieces -------------------------------------------------

const TRIGGER_LABEL: Record<Exclude<SuiteRunVM["trigger"], "">, string> = {
  manual: "Manual",
  cron: "Schedule",
  api: "API",
};

function TriggerBadge({ trigger }: { trigger: SuiteRunVM["trigger"] }) {
  if (trigger === "") {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  const isSchedule = trigger === "cron";
  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      {isSchedule ? (
        <CalendarClock className="h-3.5 w-3.5 text-zinc-500" />
      ) : (
        <Play className="h-3 w-3 text-zinc-500" />
      )}
      <span className="font-mono">{TRIGGER_LABEL[trigger]}</span>
    </span>
  );
}

/** Per-status child-run tallies (total / running / ok / fail / pending). */
function ChildCounts({ run }: { run: SuiteRunVM }) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[10px] font-mono">
      <span className="text-foreground">{run.total} total</span>
      {run.running > 0 && (
        <span style={{ color: STATUS_TINT.running }}>{run.running} run</span>
      )}
      {run.completed > 0 && (
        <span style={{ color: STATUS_TINT.completed }}>{run.completed} ok</span>
      )}
      {run.failed > 0 && (
        <span style={{ color: STATUS_TINT.failed }}>{run.failed} fail</span>
      )}
      {run.pending > 0 && (
        <span className="text-muted-foreground">{run.pending} pend</span>
      )}
    </div>
  );
}

/**
 * The child test runs of one suite run. A suite run spawns one child TEST RUN
 * per enabled cell (database × workload); each child is a first-class run
 * reachable at /runs/:testRunId.
 */
function ChildTestRuns({ run }: { run: SuiteRunVM }) {
  return (
    <div className="flex flex-col gap-2 px-4 py-3">
      <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        <GitBranch className="h-3 w-3" />
        Test runs ({run.children.length}) — one per enabled cell
      </div>
      <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
        {run.children.map((c) => (
          <Link
            key={c.testRunId}
            to={`/runs/${c.testRunId}`}
            className="group flex items-center justify-between gap-2 border border-border px-2.5 py-1.5 text-[11px] transition-colors hover:bg-muted"
            title={`${c.name} → /runs/${c.testRunId}`}
          >
            <span className="flex min-w-0 items-center gap-1.5">
              <span
                className="h-1.5 w-1.5 shrink-0 rounded-full"
                style={{ backgroundColor: STATUS_TINT[c.status] }}
              />
              <span className="truncate font-mono text-foreground">
                {c.name}
              </span>
            </span>
            <span className="flex shrink-0 items-center gap-1">
              <span
                className="text-[10px]"
                style={{ color: STATUS_TINT[c.status] }}
              >
                {STATUS_LABEL[c.status]}
              </span>
              <ChevronRight className="h-3 w-3 text-zinc-600 group-hover:text-foreground" />
            </span>
          </Link>
        ))}
      </div>
    </div>
  );
}

// --- consolidated settings card --------------------------------------------

interface SettingsPatch {
  name?: string;
  description?: string;
  provider?: SuiteProviderKind;
  schedule?: { enabled: boolean; cron: string; timezone: string };
  rating?: { inTenant?: boolean; inGlobal?: boolean };
  maxParallel?: number;
  tags?: string[];
}

function SettingsCard({
  suite,
  ownerDisplay,
  editable,
  editing,
  onEditToggle,
  busy,
  onSave,
}: {
  suite: SuiteVM;
  ownerDisplay: AuthorDisplay | null;
  editable: boolean;
  editing: boolean;
  onEditToggle: (v: boolean) => void;
  busy: boolean;
  onSave: (patch: SettingsPatch) => Promise<void>;
}) {
  // Draft state (only meaningful while editing).
  const [name, setName] = useState(suite.name);
  const [description, setDescription] = useState(suite.description);
  const [provider, setProvider] = useState<SuiteProviderKind>(suite.provider);
  const [scheduleEnabled, setScheduleEnabled] = useState(suite.scheduleEnabled);
  const [cron, setCron] = useState(suite.cron);
  const [timezone, setTimezone] = useState(suite.timezone || "UTC");
  const [inTenant, setInTenant] = useState(suite.defaultInTenantRating ?? false);
  const [inGlobal, setInGlobal] = useState(suite.defaultInGlobalRating ?? false);
  const [maxParallel, setMaxParallel] = useState(suite.defaultMaxParallel);
  const [tags, setTags] = useState<string[]>(suite.tags);
  const [tagDraft, setTagDraft] = useState("");

  // Seed drafts ONCE when entering edit mode — NOT on every `suite` change. The
  // page polls (setInterval load) every 2s, so re-seeding on each suite update
  // would clobber the user's in-progress edits every poll (toggles snapping back
  // together, name reverting). The flag resets when edit mode closes.
  const seededRef = useRef(false);
  useEffect(() => {
    if (!editing) {
      seededRef.current = false;
      return;
    }
    if (seededRef.current) return;
    seededRef.current = true;
    setName(suite.name);
    setDescription(suite.description);
    setProvider(suite.provider);
    setScheduleEnabled(suite.scheduleEnabled);
    setCron(suite.cron);
    setTimezone(suite.timezone || "UTC");
    setInTenant(suite.defaultInTenantRating ?? false);
    setInGlobal(suite.defaultInGlobalRating ?? false);
    setMaxParallel(suite.defaultMaxParallel);
    setTags(suite.tags);
    setTagDraft("");
  }, [editing, suite]);

  const addTag = () => {
    const t = tagDraft.trim();
    if (t && !tags.includes(t)) setTags([...tags, t]);
    setTagDraft("");
  };

  const submit = () =>
    void onSave({
      // Skip an empty name so the suite is never wiped to a blank title.
      name: name.trim() || undefined,
      description,
      provider,
      schedule: { enabled: scheduleEnabled, cron, timezone },
      rating: { inTenant, inGlobal },
      maxParallel,
      tags,
    });

  return (
    <Card className="flex flex-col gap-3 p-3">
      <div className="flex items-center justify-between">
        <h2 className="font-mono text-xs font-semibold uppercase tracking-wider text-zinc-500">
          Settings &amp; overview
        </h2>
        {editable &&
          (editing ? (
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => onEditToggle(false)}
                disabled={busy}
              >
                <X className="h-3.5 w-3.5" /> Cancel
              </Button>
              <Button size="sm" onClick={submit} disabled={busy}>
                <Check className="h-3.5 w-3.5" /> Save settings
              </Button>
            </div>
          ) : (
            <Button variant="outline" size="sm" onClick={() => onEditToggle(true)}>
              <Pencil className="h-3.5 w-3.5" /> Edit settings
            </Button>
          ))}
      </div>

      {editing ? (
        <div className="flex flex-col gap-3">
          {/* name */}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="set-name">Name</Label>
            <input
              id="set-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Suite name"
              className="flex h-9 w-full border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>

          {/* description */}
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="set-desc">Description</Label>
            <textarea
              id="set-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
              placeholder="What this suite verifies…"
              className="flex w-full resize-y border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {/* provider */}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="set-provider">Provider</Label>
              <Select
                value={provider || "docker"}
                onValueChange={(v) => setProvider(v as SuiteProviderKind)}
              >
                <SelectTrigger id="set-provider" className="h-9">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SUITE_PROVIDERS.map((p) => (
                    <SelectItem key={p} value={p}>
                      {PROVIDER_LABEL[p]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* max parallel */}
            <div className="flex flex-col gap-1.5">
              <NumField
                id="set-maxp"
                label="Max parallel (0 = unlimited)"
                value={maxParallel}
                onChange={setMaxParallel}
                inputClassName="h-9 font-mono"
              />
            </div>

            {/* timezone */}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="set-tz">Timezone</Label>
              <Input
                id="set-tz"
                value={timezone}
                disabled={!scheduleEnabled}
                placeholder="UTC"
                onChange={(e) => setTimezone(e.target.value)}
                className="h-9 font-mono"
              />
            </div>
          </div>

          {/* schedule */}
          <div className="flex flex-col gap-3 border border-border p-3">
            <div className="flex items-center justify-between">
              <Label htmlFor="set-sched" className="flex items-center gap-1.5">
                <CalendarClock className="h-3.5 w-3.5" /> Schedule enabled
              </Label>
              <Switch
                id="set-sched"
                checked={scheduleEnabled}
                onCheckedChange={setScheduleEnabled}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="set-cron">Cron expression</Label>
              <Input
                id="set-cron"
                value={cron}
                disabled={!scheduleEnabled}
                placeholder="0 2 * * *"
                onChange={(e) => setCron(e.target.value)}
                className="h-9 font-mono"
              />
            </div>
          </div>

          {/* rating flags */}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="flex items-center justify-between border border-border px-3 py-2">
              <Label htmlFor="set-tenant">In-tenant rating</Label>
              <Switch
                id="set-tenant"
                checked={inTenant}
                onCheckedChange={setInTenant}
              />
            </div>
            <div className="flex items-center justify-between border border-border px-3 py-2">
              <Label htmlFor="set-global">In-global rating</Label>
              <Switch
                id="set-global"
                checked={inGlobal}
                onCheckedChange={setInGlobal}
              />
            </div>
          </div>

          {/* tags */}
          <div className="flex flex-col gap-1.5">
            <Label>Tags</Label>
            <div className="flex flex-wrap items-center gap-1.5">
              {tags.map((t) => (
                <Badge
                  key={t}
                  variant="outline"
                  className="font-mono text-[10px]"
                >
                  {t}
                  <button
                    className="ml-1 text-zinc-500 hover:text-destructive"
                    onClick={() => setTags(tags.filter((x) => x !== t))}
                    aria-label={`Remove tag ${t}`}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
              <Input
                value={tagDraft}
                onChange={(e) => setTagDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addTag();
                  }
                }}
                placeholder="add tag…"
                className="h-7 w-32 px-2 text-xs font-mono"
              />
            </div>
          </div>
        </div>
      ) : (
        <>
          {suite.description && (
            <p className="max-w-4xl text-xs leading-snug text-muted-foreground">
              {suite.description}
            </p>
          )}
          <div className="grid grid-cols-2 gap-x-4 gap-y-2 sm:grid-cols-4 lg:grid-cols-6 xl:grid-cols-7">
            <MetaItem label="Provider">
              {provider === "" ? (
                "—"
              ) : (
                <span
                  className="inline-flex items-center gap-1 px-1.5 py-0.5 font-mono"
                  style={{
                    color: PROVIDER_COLOR[provider],
                    backgroundColor: `${PROVIDER_COLOR[provider]}1f`,
                  }}
                >
                  {PROVIDER_LABEL[provider]}
                </span>
              )}
            </MetaItem>
            <MetaItem label="Schedule">
              {suite.scheduleEnabled ? (
                <span className="inline-flex items-center gap-1.5">
                  <CalendarClock className="h-3.5 w-3.5 text-success" />
                  <span className="font-mono">{suite.cron || "—"}</span>
                </span>
              ) : (
                <span className="text-muted-foreground">Disabled</span>
              )}
            </MetaItem>
            <MetaItem label="Next run">
              {suite.scheduleEnabled ? (
                <span title={formatTimestamp(suite.nextRunAt)}>
                  {relTime(suite.nextRunAt)}
                </span>
              ) : (
                "—"
              )}
            </MetaItem>
            <MetaItem label="Timezone">{suite.timezone || "UTC"}</MetaItem>
            <MetaItem label="Max parallel">
              {suite.defaultMaxParallel === 0
                ? "Unlimited"
                : String(suite.defaultMaxParallel)}
            </MetaItem>
            <MetaItem label="Cells">
              <span className="font-mono">
                {suite.cellCount}/{suite.cells.length}
              </span>
            </MetaItem>
            <MetaItem label="Rating">
              <span className="inline-flex items-center gap-2 text-xs">
                <RatingChip label="tenant" on={suite.defaultInTenantRating} />
                <RatingChip label="global" on={suite.defaultInGlobalRating} />
              </span>
            </MetaItem>
            <MetaItem label="Total runs">
              <span className="font-mono">{suite.runCount}</span>
            </MetaItem>
            <MetaItem label="Last run">
              {suite.lastRunStatus ? (
                <StatusBadge status={suite.lastRunStatus} />
              ) : (
                "—"
              )}
            </MetaItem>
            <MetaItem label="Owner">
              <span className="inline-flex min-w-0 items-center gap-1.5" title={ownerDisplay?.title ?? suite.authorId}>
                <Avatar name={ownerDisplay?.avatarName ?? suite.authorId} size={14} />
                <span className="truncate">{ownerDisplay?.label ?? suite.authorId}</span>
              </span>
            </MetaItem>
            <MetaItem label="Updated">
              <span title={formatTimestamp(suite.updatedAt)}>
                {relTime(suite.updatedAt)}
              </span>
            </MetaItem>
            <MetaItem label="Tags">
              {suite.tags.length > 0 ? (
                <span className="flex flex-wrap items-center gap-1">
                  {suite.tags.map((t) => (
                    <Badge
                      key={t}
                      variant="outline"
                      className="font-mono text-[10px]"
                    >
                      {t}
                    </Badge>
                  ))}
                </span>
              ) : (
                <span className="text-muted-foreground">—</span>
              )}
            </MetaItem>
          </div>
        </>
      )}
    </Card>
  );
}

function RatingChip({ label, on }: { label: string; on?: boolean }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 font-mono text-[10px]",
        on ? "text-success" : "text-zinc-600",
      )}
      title={on ? `Included in ${label} rating` : `Excluded from ${label} rating`}
    >
      {on ? (
        <Check className="h-3 w-3" />
      ) : (
        <X className="h-3 w-3" />
      )}
      {label}
    </span>
  );
}

function CellSourceBadge({
  cell,
}: {
  cell: SuiteVM["cells"][number];
}) {
  if (cell.source === "presetPair") {
    return (
      <Badge variant="outline" className="font-mono text-[10px]" title={`${cell.presetPair?.dbPresetId} × ${cell.presetPair?.workloadPresetId}`}>
        preset pair
      </Badge>
    );
  }
  if (cell.source === "testPreset") {
    return (
      <Badge variant="secondary" className="font-mono text-[10px]" title={cell.testPresetId}>
        test preset
      </Badge>
    );
  }
  return (
    <Badge variant="default" className="font-mono text-[10px]">
      inline
    </Badge>
  );
}

// --- cells matrix (database × workload) -------------------------------------

type CellVM = SuiteVM["cells"][number];

/** A preset_pair cell with both preset ids set — the only placeable kind. */
function isPlaceable(cell: CellVM): boolean {
  return (
    cell.source === "presetPair" &&
    !!cell.presetPair?.dbPresetId &&
    !!cell.presetPair?.workloadPresetId
  );
}

/** Stable intersection key from the two preset ids. */
function pairKey(dbPresetId: string, workloadPresetId: string): string {
  return `${dbPresetId} ${workloadPresetId}`;
}

/** A resolved row (database-preset) axis. */
interface DbAxis {
  id: string;
  label: string;
  dbKind: DbKind;
}
/** A resolved column (workload-preset) axis. */
interface WlAxis {
  id: string;
  label: string;
}

function CellMatrix({
  cells,
  tenantSlug,
  editable,
  canToggle,
  onToggle,
  onRemove,
  onAddAt,
  onAddTestPreset,
}: {
  cells: CellVM[];
  tenantSlug: string;
  editable: boolean;
  canToggle: boolean;
  onToggle: (cellId: string, enabled: boolean) => void;
  onRemove: (cellId: string) => void;
  /** One-click add of a preset_pair cell at (dbPresetId, workloadPresetId). */
  onAddAt: (dbPresetId: string, workloadPresetId: string) => void;
  onAddTestPreset: (testPresetId: string) => void;
}) {
  // Preset catalog — the source of truth for axis labels + the +Axis pickers.
  const [dbRows, setDbRows] = useState<DatabasePresetRow[]>([]);
  const [wlRows, setWlRows] = useState<WorkloadPresetRow[]>([]);
  const [tpRows, setTpRows] = useState<TestPresetRow[]>([]);

  useEffect(() => {
    let alive = true;
    const p = getPresetProvider();
    Promise.all([
      p.listDatabasePresetRows(tenantSlug, { pageSize: 200 }),
      p.listWorkloadPresetRows(tenantSlug, { pageSize: 200 }),
      p.listTestPresetRows(tenantSlug, { pageSize: 200 }),
    ])
      .then(([db, wl, tp]) => {
        if (!alive) return;
        setDbRows(db.rows);
        setWlRows(wl.rows);
        setTpRows(tp.rows);
      })
      .catch(() => {
        /* pickers stay empty; the page surfaces add errors itself */
      });
    return () => {
      alive = false;
    };
  }, [tenantSlug]);

  // Axes the user opened with "+ Database" / "+ Workload" that have no cell yet.
  // They live in local state until a cell lands on them; a removal that empties
  // a cell-derived axis simply re-derives it away (the header drops).
  const [extraDbIds, setExtraDbIds] = useState<string[]>([]);
  const [extraWlIds, setExtraWlIds] = useState<string[]>([]);

  const dbRowById = useMemo(() => {
    const m = new Map<string, DatabasePresetRow>();
    for (const r of dbRows) m.set(r.id, r);
    return m;
  }, [dbRows]);
  const wlRowById = useMemo(() => {
    const m = new Map<string, WorkloadPresetRow>();
    for (const r of wlRows) m.set(r.id, r);
    return m;
  }, [wlRows]);

  const placeable = useMemo(() => cells.filter(isPlaceable), [cells]);

  // testPreset cells + any inline cells render below the matrix, not on it.
  const otherCells = useMemo(
    () => cells.filter((c) => !isPlaceable(c)),
    [cells],
  );

  // Resolve a db-preset axis label/tint from the catalog, falling back to the
  // cell's server-derived dbKind for legacy rows whose preset isn't catalogued.
  const dbAxisFor = useCallback(
    (id: string, sample?: CellVM): DbAxis => {
      const row = dbRowById.get(id);
      if (row) return { id, label: row.name, dbKind: row.dbKind };
      const kind = sample?.dbKind ?? "";
      return { id, label: sample ? dbLabel(kind) : id, dbKind: kind };
    },
    [dbRowById],
  );
  const wlAxisFor = useCallback(
    (id: string, sample?: CellVM): WlAxis => {
      const row = wlRowById.get(id);
      if (row) return { id, label: row.name };
      return { id, label: sample?.workload || id };
    },
    [wlRowById],
  );

  // Build the row + column axes: union of (cell-referenced ids) and the
  // locally-added empty axes. A sample cell per id supplies the legacy label.
  const dbAxes = useMemo<DbAxis[]>(() => {
    const sample = new Map<string, CellVM>();
    for (const c of placeable) {
      const id = c.presetPair?.dbPresetId ?? "";
      if (id && !sample.has(id)) sample.set(id, c);
    }
    const ids = new Set<string>([...sample.keys(), ...extraDbIds]);
    return [...ids]
      .map((id) => dbAxisFor(id, sample.get(id)))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [placeable, extraDbIds, dbAxisFor]);

  const wlAxes = useMemo<WlAxis[]>(() => {
    const sample = new Map<string, CellVM>();
    for (const c of placeable) {
      const id = c.presetPair?.workloadPresetId ?? "";
      if (id && !sample.has(id)) sample.set(id, c);
    }
    const ids = new Set<string>([...sample.keys(), ...extraWlIds]);
    return [...ids]
      .map((id) => wlAxisFor(id, sample.get(id)))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [placeable, extraWlIds, wlAxisFor]);

  // Intersection lookup; collisions (a duplicate pair) fall through to "other".
  const { grid, collisions } = useMemo(() => {
    const g = new Map<string, CellVM>();
    const col: CellVM[] = [];
    for (const c of placeable) {
      const key = pairKey(
        c.presetPair?.dbPresetId ?? "",
        c.presetPair?.workloadPresetId ?? "",
      );
      if (g.has(key)) col.push(c);
      else g.set(key, c);
    }
    return { grid: g, collisions: col };
  }, [placeable]);

  const handleAddAt = useCallback(
    (dbId: string, wlId: string) => {
      // The pair now has a backing cell — drop it from the pending extras.
      setExtraDbIds((ids) => ids.filter((x) => x !== dbId));
      setExtraWlIds((ids) => ids.filter((x) => x !== wlId));
      onAddAt(dbId, wlId);
    },
    [onAddAt],
  );

  const fallback = [...otherCells, ...collisions];

  // db / workload presets not yet on the matrix — the +Axis picker options.
  const dbOptions = dbRows.filter((r) => !dbAxes.some((a) => a.id === r.id));
  const wlOptions = wlRows.filter((r) => !wlAxes.some((a) => a.id === r.id));

  const hasMatrix = dbAxes.length > 0 && wlAxes.length > 0;

  return (
    <div className="flex flex-col gap-4">
      {/* axis-extension controls */}
      {editable && (
        <div className="flex flex-wrap items-center gap-2">
          <AxisPicker
            label="Database"
            placeholder="No more database presets"
            options={dbOptions.map((r) => ({
              id: r.id,
              label: `${r.name} ${dbLabel(r.dbKind)}`,
            }))}
            onPick={(id) =>
              setExtraDbIds((ids) => (ids.includes(id) ? ids : [...ids, id]))
            }
          />
          <AxisPicker
            label="Workload"
            placeholder="No more workload presets"
            options={wlOptions.map((r) => ({ id: r.id, label: r.name }))}
            onPick={(id) =>
              setExtraWlIds((ids) => (ids.includes(id) ? ids : [...ids, id]))
            }
          />
        </div>
      )}

      {hasMatrix ? (
        <div className="overflow-x-auto">
          <div
            className="grid gap-0.5"
            style={{
              gridTemplateColumns: `minmax(5.5rem,auto) repeat(${wlAxes.length}, minmax(3rem,1fr))`,
            }}
          >
            {/* corner */}
            <div className="sticky left-0 z-10 flex h-8 items-center px-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              db &times; workload
            </div>
            {/* column headers (workload presets) — centered over the column */}
            {wlAxes.map((wl) => (
              <div
                key={wl.id}
                className="flex h-8 items-center justify-center border-b border-border/50 px-2 font-mono text-[11px] text-foreground"
                title={wl.label}
              >
                <span className="truncate">{wl.label}</span>
              </div>
            ))}

            {/* rows (database presets) */}
            {dbAxes.map((db) => (
              <Fragment key={db.id}>
                <div className="sticky left-0 z-10 flex items-center gap-1.5 border-r border-border/50 bg-card pr-2 text-xs font-medium">
                  <span
                    className="h-2 w-2 shrink-0 rounded-sm"
                    style={{ backgroundColor: dbTint(db.dbKind) }}
                  />
                  <span
                    className="truncate"
                    style={{ color: dbTint(db.dbKind) }}
                    title={db.label}
                  >
                    {db.label}
                  </span>
                </div>
                {wlAxes.map((wl) => {
                  const cell = grid.get(pairKey(db.id, wl.id));
                  return (
                    <MatrixSlot
                      key={pairKey(db.id, wl.id)}
                      cell={cell}
                      editable={editable}
                      canToggle={canToggle}
                      onToggle={onToggle}
                      onRemove={onRemove}
                      onAdd={() => handleAddAt(db.id, wl.id)}
                    />
                  );
                })}
              </Fragment>
            ))}
          </div>
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
          <Database className="h-6 w-6 text-zinc-600" />
          <div className="text-xs text-muted-foreground">
            No matrix cells yet.
            {editable &&
              " Use + Database and + Workload to open an axis, then click an intersection."}
          </div>
        </div>
      )}

      {/* test-preset (and legacy inline) cells live outside the matrix. */}
      {(fallback.length > 0 || editable) && (
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center justify-between">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-600">
              Other cells (test presets)
            </div>
            {editable && (
              <AxisPicker
                label="Test preset"
                placeholder="No test presets"
                options={tpRows.map((r) => ({
                  id: r.id,
                  label: `${r.name} ${dbLabel(r.dbKind)}`,
                }))}
                onPick={(id) => onAddTestPreset(id)}
              />
            )}
          </div>
          {fallback.map((cell) => {
            const isInline = cell.source === "inline";
            return (
              <div
                key={cell.id}
                className={cn(
                  "flex items-center justify-between gap-2 border border-border px-2 py-1.5",
                  !cell.enabled && "opacity-45",
                )}
              >
                <div className="flex min-w-0 items-center gap-2">
                  <CellCheckbox
                    checked={cell.enabled}
                    disabled={!canToggle || isInline}
                    onChange={(v) => onToggle(cell.id, v)}
                  />
                  <span className="truncate font-mono text-xs">
                    {cell.name || cell.id}
                  </span>
                  <CellSourceBadge cell={cell} />
                  {isInline && (
                    <span className="text-[10px] italic text-muted-foreground">
                      inline — edit in the wizard
                    </span>
                  )}
                </div>
                {editable && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-7 w-7 shrink-0 p-0 text-destructive hover:text-destructive"
                    title="Remove cell"
                    onClick={() => onRemove(cell.id)}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                )}
              </div>
            );
          })}
          {fallback.length === 0 && editable && (
            <span className="text-[11px] text-muted-foreground">
              None — add a test-preset cell with the picker above.
            </span>
          )}
        </div>
      )}
    </div>
  );
}

/** A compact "+ <label>" dropdown that adds an axis / cell on pick. */
function AxisPicker({
  label,
  placeholder,
  options,
  onPick,
}: {
  label: string;
  placeholder: string;
  options: { id: string; label: string }[];
  onPick: (id: string) => void;
}) {
  // A controlled Select reset to "" so it always acts like a fresh button.
  return (
    <Select
      value=""
      onValueChange={(v) => {
        if (v) onPick(v);
      }}
    >
      <SelectTrigger className="h-7 w-auto gap-1 px-2 text-[11px] font-mono">
        <Plus className="h-3 w-3" />
        {label}
      </SelectTrigger>
      <SelectContent>
        {options.length === 0 ? (
          <div className="px-2 py-1.5 text-xs text-muted-foreground">
            {placeholder}
          </div>
        ) : (
          options.map((o) => (
            <SelectItem key={o.id} value={o.id}>
              {o.label}
            </SelectItem>
          ))
        )}
      </SelectContent>
    </Select>
  );
}

/** One database×workload intersection: a SuiteCell tile or an empty/add slot. */
// CellCheckbox — the on-brand dark square check from the Runs page
// (ToolbarCheckbox / ChecklistFilter), reused for the cell enable/disable toggle.
// Unchecked = dark square with a zinc border; checked = primary accent (not white).
function CellCheckbox({
  checked,
  disabled,
  onChange,
  title,
}: {
  checked: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
  title?: string;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      title={title}
      onClick={(e) => {
        e.stopPropagation();
        if (!disabled) onChange(!checked);
      }}
      className={cn(
        "flex items-center justify-center transition-colors",
        disabled && "cursor-not-allowed opacity-40",
      )}
    >
      <span
        className={cn(
          "h-3.5 w-3.5 rounded-sm border flex items-center justify-center shrink-0 transition-colors",
          checked
            ? "bg-primary border-primary"
            : "border-zinc-400 bg-zinc-800 hover:border-zinc-200",
        )}
      >
        {checked && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
      </span>
    </button>
  );
}

function MatrixSlot({
  cell,
  editable,
  canToggle,
  onToggle,
  onRemove,
  onAdd,
}: {
  cell?: CellVM;
  editable: boolean;
  canToggle: boolean;
  onToggle: (cellId: string, enabled: boolean) => void;
  onRemove: (cellId: string) => void;
  onAdd: () => void;
}) {
  if (!cell) {
    // Empty intersection — an add slot when editable, otherwise a faint dash.
    if (editable) {
      return (
        <button
          onClick={onAdd}
          title="Add a cell here"
          className="group flex h-9 items-center justify-center border border-dashed border-border text-zinc-500 transition-colors hover:border-primary hover:bg-primary/5 hover:text-primary"
        >
          <Plus className="h-3.5 w-3.5 transition-transform group-hover:scale-110" />
        </button>
      );
    }
    return (
      <div className="flex h-9 items-center justify-center border border-dashed border-border/40 text-zinc-700">
        <span className="text-[11px]">&mdash;</span>
      </div>
    );
  }

  // Compact tile: just an on/off toggle + delete. The row/column headers carry
  // the labels, so the tile itself stays tiny.
  return (
    <div
      className={cn(
        "flex h-9 items-center justify-center gap-1 rounded-sm px-1",
        cell.enabled ? "bg-muted/40" : "bg-transparent",
      )}
    >
      <CellCheckbox
        checked={cell.enabled}
        disabled={!canToggle}
        onChange={(v) => onToggle(cell.id, v)}
        title={cell.enabled ? "Enabled — click to disable" : "Disabled — click to enable"}
      />
      {editable && (
        <Button
          variant="ghost"
          size="sm"
          className="h-6 w-6 shrink-0 p-0 text-destructive hover:text-destructive"
          title="Delete cell"
          onClick={() => onRemove(cell.id)}
        >
          <Trash2 className="h-3 w-3" />
        </Button>
      )}
    </div>
  );
}
