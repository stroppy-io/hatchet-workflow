// Workload Presets — the Library table for WorkloadPresetService.ListWorkloadPresets.
//
// Built STRICTLY around cloud.v1.api.ListWorkloadPresetsRequest. WIRED slice →
// column header controls:
//   * filter.search          -> Name header text filter (?q=)
//   * protocols[]            -> Workload header multi-select: Protocol (?proto=)
//   * stroppy_versions[]     -> Workload header multi-select: Version (?sv=)
//   * is_system              -> System header tri-state (?is_system=)
//   * filter.created_after/before -> Created header date range (?ca=, ?cb=)
//   * filter.updated_after/before -> Updated header date range (?ua=, ?ub=)
//   * sort {entity NAME|CREATED_AT|UPDATED_AT|AUTHOR_ID, kind PROTOCOL|STROPPY_VERSION|IS_SYSTEM}
//   * page {size, token}
// INTENTIONALLY UNWIRED: tags{}, scripts[] (no dedicated column — script shows
//   as Workload subtext but there's no facet catalog to pick from); filter.ids /
//   include_deleted / favorites_only / author_ids — no column.
//
// COLUMN → PROTO FIELD: Name=entity.name(+description), Workload=Summary.protocol
// (+stroppy_version/script sub), System=is_system, Author=entity.author_id,
// Created=timings.created_at, Updated=timings.updated_at.

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckCircle2, Lock, Plus } from "lucide-react";
import { useNavigate, useTenantSlug } from "@/lib/router";
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
  PROTOCOLS,
  PROTOCOL_LABEL,
  relTime,
  type Protocol,
} from "@/components/library-table/labels";
import {
  getPresetProvider,
  type PresetSortField,
  type WorkloadPresetRow,
} from "@/services/preset";
import { LibraryShell } from "@/pages/library/LibraryShell";

// Stroppy versions exposed in the version facet — a fixed catalog in the
// preview (the real provider can derive options from a facet RPC).
const STROPPY_VERSIONS = ["5.1.2", "5.1.1", "5.1.0", "5.0.9"];

const SORTABLE: readonly PresetSortField[] = [
  "name",
  "author_id",
  "protocol",
  "stroppy_version",
  "is_system",
  "created_at",
  "updated_at",
];

export function WorkloadPresets() {
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

  const protocols = useMemo(
    () => getCsv("proto", PROTOCOLS) as Exclude<Protocol, "">[],
    [getCsv],
  );
  const stroppyVersions = useMemo(
    () => getCsv("sv", STROPPY_VERSIONS),
    [getCsv],
  );
  const isSystem = getTriBool("is_system");
  const favoritesOnly = getCsv("fav")[0] === "1";
  const protocolSet = useMemo(() => new Set<string>(protocols), [protocols]);
  const versionSet = useMemo(
    () => new Set<string>(stroppyVersions),
    [stroppyVersions],
  );

  const [rows, setRows] = useState<WorkloadPresetRow[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [openColumnId, setOpenColumnId] = useState<string | null>(null);
  const handleOpenChange = useCallback((columnId: string, open: boolean) => {
    setOpenColumnId(open ? columnId : (cur) => (cur === columnId ? null : cur));
  }, []);

  const createdWindow = useMemo(
    () => ({ after: getCsv("ca")[0], before: getCsv("cb")[0] }),
    [getCsv],
  );
  const updatedWindow = useMemo(
    () => ({ after: getCsv("ua")[0], before: getCsv("ub")[0] }),
    [getCsv],
  );

  const fetchRows = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const page = await getPresetProvider().listWorkloadPresetRows(slug, {
        search: state.search,
        favoritesOnly: favoritesOnly || undefined,
        protocols,
        stroppyVersions,
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
    protocols,
    stroppyVersions,
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
    kind: "workload",
    favoritesOnly,
    rows,
    setRows,
    refetch: fetchRows,
    setError,
  });

  const hasActiveFilters =
    !!state.search ||
    favoritesOnly ||
    protocols.length > 0 ||
    stroppyVersions.length > 0 ||
    isSystem !== undefined ||
    !!createdWindow.after ||
    !!createdWindow.before ||
    !!updatedWindow.after ||
    !!updatedWindow.before;

  const columns = useMemo<ColumnDef<WorkloadPresetRow>[]>(
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
        accessorKey: "protocol",
        header: () => (
          <ColumnHeader
            label="Workload"
            sortField="protocol"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={protocolSet.size > 0 || versionSet.size > 0}
            columnId="workload"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
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
          const proto = r.protocol ? PROTOCOL_LABEL[r.protocol] : null;
          // stroppy_version now has its own Version column; keep protocol +
          // script here so the subtext stays meaningful without duplication.
          const sub = [proto, r.script || null].filter(Boolean).join(" · ");
          return (
            <div className="flex flex-col gap-0.5">
              <span className="font-mono text-xs text-zinc-300">
                {proto ?? "—"}
              </span>
              {sub && (
                <span className="font-mono text-[10px] text-zinc-600 truncate">
                  {sub}
                </span>
              )}
            </div>
          );
        },
      },
      {
        // Version — Summary.stroppy_version, the stroppy build this workload
        // targets. Sortable on its own (kind sort STROPPY_VERSION).
        accessorKey: "stroppyVersion",
        meta: { className: "text-center" },
        header: () => (
          <div className="flex justify-center">
            <ColumnHeader
              label="Version"
              sortField="stroppy_version"
              sort={state.sort}
              desc={state.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) => {
          const v = row.original.stroppyVersion;
          return v ? (
            <span className="block text-center font-mono text-[11px] text-zinc-300">
              {v}
            </span>
          ) : (
            <span className="block text-center font-mono text-xs text-zinc-600">
              —
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
      protocolSet,
      versionSet,
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
            onClick={() => navigate("/presets/workload/new")}
          >
            <Plus className="h-3.5 w-3.5" /> New preset
          </Button>
        </div>
        <LibraryTable
          columns={columns}
          rows={rows}
          getRowId={(r) => r.id}
          onRowClick={(r) => navigate(`/presets/workload/${r.id}`)}
          columnWidths={WORKLOAD_PRESET_WIDTHS}
          loading={loading}
          error={error}
          emptyLabel="No workload presets yet"
          emptyFilteredLabel="No workload presets matching filters"
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

const WORKLOAD_PRESET_WIDTHS: Record<string, string> = {
  name: "24%",
  protocol: "17%",
  stroppyVersion: "9%",
  isSystem: "10%",
  authorId: "13%",
  createdAt: "9%",
  updatedAt: "9%",
  actions: "9%",
};
