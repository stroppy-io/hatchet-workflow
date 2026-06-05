// Database Presets — the Library table for DatabasePresetService.ListDatabasePresets.
//
// Built STRICTLY around cloud.v1.api.ListDatabasePresetsRequest (per schema, no
// invented fields). WIRED slice → column header controls:
//   * filter.search          -> Name header text filter (?q=)
//   * db_kinds[]             -> Database header multi-select (?db=)
//   * is_system              -> System header tri-state (?is_system=true|false)
//   * filter.created_after/before -> Created header date range (?ca=, ?cb=)
//   * filter.updated_after/before -> Updated header date range (?ua=, ?ub=)
//   * sort {entity NAME|CREATED_AT|UPDATED_AT|AUTHOR_ID, kind DB_KIND|IS_SYSTEM}
//                            -> tri-state sort on Name/Author/Database/System/
//                               Created/Updated columns
//   * page {size, token}     -> page-size selector + prev/next paging
// INTENTIONALLY UNWIRED (in the request but no column on this page): tags{},
//   sources[] (SourceKind) — no visible column, so no header filter is offered;
//   filter.ids / include_deleted / favorites_only / author_ids — no column.
//
// COLUMN → PROTO FIELD: Name=entity.name(+description sub), Author=entity.author_id,
// Database=Summary.db_kind(+version/external sub), System=is_system,
// Created=entity.timings.created_at, Updated=entity.timings.updated_at.

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle2, Lock, Plus } from "lucide-react";
import { useNavigate, useTenantSlug } from "@/lib/router";
import { fallbackAuthorDisplay } from "@/lib/author-display";
import { useAuthorDisplays } from "@/hooks/useAuthorDisplays";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import {
  ChecklistFilter,
  ColumnHeader,
  DateRangeFilter,
  LibraryTable,
  TextFilter,
  ToolbarTriToggle,
  useLibraryTableState,
} from "@/components/library-table/LibraryTable";
import { FavoritesOnlyToggle } from "@/components/library-table/RowActions";
import { usePresetRowActions } from "@/components/library-table/usePresetRowActions";
import {
  DB_COLOR,
  DB_KINDS,
  DB_LABEL,
  relTime,
  type DbKind,
} from "@/components/library-table/labels";
import {
  getPresetProvider,
  type DatabasePresetRow,
  type PresetSortField,
} from "@/services/preset";
import { LibraryShell } from "@/pages/library/LibraryShell";

const SORTABLE: readonly PresetSortField[] = [
  "name",
  "author_id",
  "db_kind",
  "is_system",
  "created_at",
  "updated_at",
];

export function DatabasePresets() {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const {
    state,
    patch,
    getCsv,
    setCsv,
    getTriBool,
    toggleSort,
    setPageSize,
    pageIndex,
    goNext,
    goPrev,
  } = useLibraryTableState(SORTABLE);

  const dbKinds = useMemo(
    () => getCsv("db", DB_KINDS) as Exclude<DbKind, "">[],
    [getCsv],
  );
  const isSystem = getTriBool("is_system");
  const favoritesOnly = getCsv("fav")[0] === "1";

  const dbSet = useMemo(() => new Set<string>(dbKinds), [dbKinds]);

  const [rows, setRows] = useState<DatabasePresetRow[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [openColumnId, setOpenColumnId] = useState<string | null>(null);
  const authorIds = useMemo(() => rows.map((row) => row.authorId), [rows]);
  const authorDisplays = useAuthorDisplays(authorIds);
  const handleOpenChange = useCallback((columnId: string, open: boolean) => {
    setOpenColumnId(open ? columnId : (cur) => (cur === columnId ? null : cur));
  }, []);

  // Read the created/updated windows straight off the URL params via getCsv's
  // sibling: use raw param reads through a small helper (state hook exposes only
  // the shared slice, so windows live as their own params here).
  const createdWindow = useMemo(
    () => ({
      after: getCsv("ca")[0],
      before: getCsv("cb")[0],
    }),
    [getCsv],
  );
  const updatedWindow = useMemo(
    () => ({
      after: getCsv("ua")[0],
      before: getCsv("ub")[0],
    }),
    [getCsv],
  );

  const fetchRows = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const page = await getPresetProvider().listDatabasePresetRows(slug, {
        search: state.search,
        favoritesOnly: favoritesOnly || undefined,
        dbKinds,
        isSystem,
        createdAfter: createdWindow.after,
        createdBefore: createdWindow.before,
        updatedAfter: updatedWindow.after,
        updatedBefore: updatedWindow.before,
        sort: state.sort as PresetSortField | undefined,
        desc: state.desc,
        pageSize: state.pageSize,
        pageToken: state.pageToken,
      });
      setRows(page.rows);
      setNextPageToken(page.nextPageToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load presets");
      setRows([]);
      setNextPageToken("");
    } finally {
      setLoading(false);
    }
  }, [
    slug,
    state.search,
    state.sort,
    state.desc,
    state.pageSize,
    state.pageToken,
    favoritesOnly,
    dbKinds,
    isSystem,
    createdWindow.after,
    createdWindow.before,
    updatedWindow.after,
    updatedWindow.before,
  ]);

  useEffect(() => {
    void fetchRows();
  }, [fetchRows]);

  const { actionsColumn } = usePresetRowActions({
    slug,
    kind: "database",
    favoritesOnly,
    rows,
    setRows,
    refetch: fetchRows,
    setError,
  });

  const hasActiveFilters =
    !!state.search ||
    favoritesOnly ||
    dbKinds.length > 0 ||
    isSystem !== undefined ||
    !!createdWindow.after ||
    !!createdWindow.before ||
    !!updatedWindow.after ||
    !!updatedWindow.before;

  const columns = useMemo<ColumnDef<DatabasePresetRow>[]>(
    () => [
      {
        accessorKey: "name",
        meta: { className: "pl-4" },
        header: () => (
          <ColumnHeader
            label="Name"
            sortField="name"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={!!state.search}
            columnId="name"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
          >
            <TextFilter
              value={state.search ?? ""}
              onChange={(v) => patch({ q: v || null })}
              placeholder="Search name / description…"
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const r = row.original;
          return (
            <div className="flex flex-col gap-0.5 min-w-0">
              <span className="text-xs text-primary truncate" title={r.name}>
                {r.name || r.id}
              </span>
              {r.description && (
                <span
                  className="font-mono text-[10px] text-zinc-600 truncate"
                  title={r.description}
                >
                  {r.description}
                </span>
              )}
            </div>
          );
        },
      },
      {
        accessorKey: "dbKind",
        header: () => (
          <ColumnHeader
            label="Database"
            sortField="db_kind"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={dbSet.size > 0}
            columnId="dbKind"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
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
          const sub = [
            r.version ? `v${r.version}` : null,
            r.external ? "external" : null,
          ]
            .filter(Boolean)
            .join(" · ");
          return (
            <div className="flex items-center gap-2">
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: color }}
              />
              <div className="flex flex-col gap-0.5 min-w-0">
                <span className="font-mono text-xs" style={{ color }}>
                  {DB_LABEL[r.dbKind]}
                </span>
                {sub && (
                  <span className="font-mono text-[10px] text-zinc-600 truncate">
                    {sub}
                  </span>
                )}
              </div>
            </div>
          );
        },
      },
      {
        // Version — Summary.version (self-deploy engine version), with an
        // "external" badge from Summary.external for endpoint presets.
        accessorKey: "version",
        meta: { className: "text-center" },
        header: () => (
          <div className="flex justify-center">
            <ColumnHeader
              label="Version"
              sort={state.sort}
              desc={state.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) => {
          const r = row.original;
          if (r.external)
            return (
              <span className="block text-center font-mono text-[11px] text-zinc-400">
                external
              </span>
            );
          if (!r.version)
            return (
              <span className="block text-center font-mono text-xs text-zinc-600">
                —
              </span>
            );
          return (
            <span className="block text-center font-mono text-[11px] text-zinc-300">
              v{r.version}
            </span>
          );
        },
      },
      {
        accessorKey: "isSystem",
        meta: { className: "text-center" },
        header: () => (
          <div className="flex justify-center">
            <ColumnHeader
              label="System"
              sortField="is_system"
              sort={state.sort}
              desc={state.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) =>
          row.original.isSystem ? (
            <div
              className="flex items-center justify-center gap-1 text-[11px] font-mono text-zinc-400"
              title="Platform-seeded, read-only"
            >
              <Lock className="h-3 w-3 text-zinc-500" />
              System
            </div>
          ) : (
            <div
              className="flex items-center justify-center gap-1 text-[11px] font-mono text-zinc-500"
              title="Tenant preset, editable"
            >
              <CheckCircle2 className="h-3 w-3 text-success/70" />
              User
            </div>
          ),
      },
      {
        accessorKey: "authorId",
        header: () => (
          <ColumnHeader
            label="Author"
            sortField="author_id"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
          />
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
                className="text-xs text-zinc-400 truncate"
                title={display.title}
              >
                {display.label}
              </span>
            </div>
          );
        },
      },
      {
        accessorKey: "createdAt",
        header: () => (
          <ColumnHeader
            label="Created"
            sortField="created_at"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={!!createdWindow.after || !!createdWindow.before}
            columnId="created"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
          >
            <DateRangeFilter
              after={createdWindow.after}
              before={createdWindow.before}
              onChange={(after, before) => patch({ ca: after, cb: before })}
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => (
          <span
            className="text-xs text-zinc-400 font-mono"
            title={row.original.createdAt}
          >
            {relTime(row.original.createdAt)}
          </span>
        ),
      },
      {
        accessorKey: "updatedAt",
        header: () => (
          <ColumnHeader
            label="Updated"
            sortField="updated_at"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={!!updatedWindow.after || !!updatedWindow.before}
            columnId="updated"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
          >
            <DateRangeFilter
              after={updatedWindow.after}
              before={updatedWindow.before}
              onChange={(after, before) => patch({ ua: after, ub: before })}
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => (
          <span
            className="text-xs text-zinc-400 font-mono"
            title={row.original.updatedAt}
          >
            {relTime(row.original.updatedAt)}
          </span>
        ),
      },
      actionsColumn,
    ],
    [
      state.sort,
      state.desc,
      state.search,
      dbSet,
      createdWindow.after,
      createdWindow.before,
      updatedWindow.after,
      updatedWindow.before,
      openColumnId,
      handleOpenChange,
      patch,
      setCsv,
      toggleSort,
      actionsColumn,
      authorDisplays,
    ],
  );

  return (
    <LibraryShell shownCount={loading ? undefined : rows.length}>
      <div className="flex flex-col gap-2 h-full min-h-0">
        <div className="flex items-center gap-3">
          <ToolbarTriToggle
            value={isSystem}
            onChange={(next) =>
              patch({ is_system: next === undefined ? null : String(next) })
            }
            labels={["All presets", "System only", "User only"]}
          />
          <FavoritesOnlyToggle
            checked={favoritesOnly}
            onChange={(next) => patch({ fav: next ? "1" : null })}
          />
          <Button
            size="sm"
            className="ml-auto"
            onClick={() => navigate("/presets/database/new")}
          >
            <Plus className="h-3.5 w-3.5" /> New preset
          </Button>
        </div>
        <LibraryTable
          columns={columns}
          rows={rows}
          getRowId={(r) => r.id}
          onRowClick={(r) => navigate(`/presets/database/${r.id}`)}
          columnWidths={DB_PRESET_WIDTHS}
          loading={loading}
          error={error}
          emptyLabel="No database presets yet"
          emptyFilteredLabel="No database presets matching filters"
          hasActiveFilters={hasActiveFilters}
          openColumnId={openColumnId}
          pageIndex={pageIndex}
          canPrev={pageIndex > 0}
          canNext={!!nextPageToken}
          onPrev={goPrev}
          onNext={() => goNext(nextPageToken)}
          pageSize={state.pageSize}
          onPageSize={setPageSize}
        />
      </div>
    </LibraryShell>
  );
}

const DB_PRESET_WIDTHS: Record<string, string> = {
  name: "24%",
  dbKind: "15%",
  version: "9%",
  isSystem: "10%",
  authorId: "13%",
  createdAt: "10%",
  updatedAt: "10%",
  actions: "9%",
};
