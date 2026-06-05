// Packages — the Library table for PackageService.ListPackages.
//
// Built STRICTLY around cloud.v1.api.ListPackagesRequest. WIRED slice → column
// header controls:
//   * filter.search          -> Name header text filter (?q=)
//   * formats[]              -> Format header multi-select (?fmt=)
//   * db_kinds[]             -> Target header multi-select (?db=)
//   * filter.created_after/before -> Created header date range (?ca=, ?cb=)
//   * sort (EntitySort: NAME|CREATED_AT|UPDATED_AT|AUTHOR_ID)
//                            -> tri-state sort on Name/Uploaded-by/Created/Updated
//   * page {size, token}     -> page-size selector + prev/next paging
// INTENTIONALLY UNWIRED: ListPackagesRequest has NO status facet, so the Status
//   column is display + (status is not an EntitySortField) display-only — no
//   header filter / sort, per schema. filter.ids / updated_after-before /
//   include_deleted / author_ids / favorites_only — no column.
//
// COLUMN → PROTO FIELD: Name=entity.name(+version sub), Format=format,
// Target=target_db_kind(+os/arch sub), Size=size_bytes, Status=status,
// Uploaded by=entity.author_id, Created=timings.created_at, Updated=timings.updated_at.

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { Check, Copy, Download, Eye, Trash2, UploadCloud } from "lucide-react";
import { Link, useNavigate, useTenantSlug } from "@/lib/router";
import { fallbackAuthorDisplay } from "@/lib/author-display";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/hooks/useAuth";
import { useAuthorDisplays } from "@/hooks/useAuthorDisplays";
import { roleLevel } from "@/lib/roles";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Avatar } from "@/components/Avatar";
import {
  ChecklistFilter,
  ColumnHeader,
  DateRangeFilter,
  LibraryTable,
  TextFilter,
  useLibraryTableState,
} from "@/components/library-table/LibraryTable";
import {
  RowActionsMenu,
  type RowActionItem,
} from "@/components/library-table/RowActions";
import {
  DB_COLOR,
  DB_KINDS,
  DB_LABEL,
  PACKAGE_FORMATS,
  PACKAGE_FORMAT_COLOR,
  PACKAGE_FORMAT_LABEL,
  PACKAGE_STATUS_LABEL,
  PACKAGE_STATUS_TINT,
  formatBytes,
  relTime,
  type DbKind,
  type PackageFormat,
} from "@/components/library-table/labels";
import {
  getPackagesProvider,
  type PackageAction,
  type PackageRow,
  type PackageSortField,
} from "@/services/packages";
import { LibraryShell } from "@/pages/library/LibraryShell";

/** Truncate a 64-hex sha256 to a copyable short form. */
function shortSha(sha: string): string {
  return sha ? sha.slice(0, 12) : "";
}

function packageDetailPath(id: string): string {
  return `/packages/${encodeURIComponent(id)}`;
}

const SORTABLE: readonly PackageSortField[] = [
  "name",
  "author_id",
  "created_at",
  "updated_at",
];

export function Packages() {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const { user } = useAuth();
  const confirm = useConfirm();
  const {
    state,
    patch,
    getCsv,
    setCsv,
    toggleSort,
    setPageSize,
    pageIndex,
    goNext,
    goPrev,
  } = useLibraryTableState(SORTABLE);

  const formats = useMemo(
    () => getCsv("fmt", PACKAGE_FORMATS) as Exclude<PackageFormat, "">[],
    [getCsv],
  );
  const dbKinds = useMemo(
    () => getCsv("db", DB_KINDS) as Exclude<DbKind, "">[],
    [getCsv],
  );
  const formatSet = useMemo(() => new Set<string>(formats), [formats]);
  const dbSet = useMemo(() => new Set<string>(dbKinds), [dbKinds]);

  const [rows, setRows] = useState<PackageRow[]>([]);
  const [nextPageToken, setNextPageToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [openColumnId, setOpenColumnId] = useState<string | null>(null);
  const authorIds = useMemo(() => rows.map((row) => row.authorId), [rows]);
  const authorDisplays = useAuthorDisplays(authorIds);
  const handleOpenChange = useCallback((columnId: string, open: boolean) => {
    setOpenColumnId(open ? columnId : (cur) => (cur === columnId ? null : cur));
  }, []);
  // Which row's Actions (⋯) menu is open, and which sha was just copied.
  const [openActionId, setOpenActionId] = useState<string | null>(null);
  const [copiedSha, setCopiedSha] = useState<string | null>(null);

  // operator+ (>=2) on the active tenant, or platform admin, may delete.
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canMutate = level >= roleLevel.operator;

  const createdWindow = useMemo(
    () => ({ after: getCsv("ca")[0], before: getCsv("cb")[0] }),
    [getCsv],
  );

  const fetchRows = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    try {
      const page = await getPackagesProvider().listPackages(slug, {
        search: state.search,
        formats,
        dbKinds,
        createdAfter: createdWindow.after,
        createdBefore: createdWindow.before,
        sort: state.sort as PackageSortField | undefined,
        desc: state.desc,
        pageSize: state.pageSize,
        pageToken: state.pageToken,
      });
      setRows(page.rows);
      setNextPageToken(page.nextPageToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load packages");
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
    formats,
    dbKinds,
    createdWindow.after,
    createdWindow.before,
  ]);

  useEffect(() => {
    void fetchRows();
  }, [fetchRows]);

  const runAction = useCallback(
    async (action: PackageAction, row: PackageRow) => {
      if (!slug) return;
      setOpenActionId(null);
      try {
        switch (action) {
          case "view": {
            navigate(packageDetailPath(row.id));
            return;
          }
          case "download": {
            // Open the blob behind storage_uri (the gateway serves the binary).
            if (row.storageUri)
              window.open(row.storageUri, "_blank", "noopener");
            return;
          }
          case "delete": {
            if (row.isBuiltin) {
              setError("Built-in packages cannot be deleted");
              return;
            }
            const ok = await confirm({
              title: "Delete package?",
              description: `“${row.name || row.id}” will be permanently removed. This cannot be undone.`,
              danger: true,
              confirmLabel: "Delete",
            });
            if (!ok) return;
            await getPackagesProvider().deletePackage(slug, row.id);
            setRows((current) => current.filter((r) => r.id !== row.id));
            await fetchRows();
            return;
          }
        }
      } catch (err) {
        setError(
          err instanceof Error ? err.message : `Failed to ${action} package`,
        );
      }
    },
    [slug, confirm, fetchRows, navigate],
  );

  const actionItemsFor = useCallback(
    (row: PackageRow): RowActionItem<PackageAction>[] => {
      const notReady = "Available once the package is uploaded (Ready)";
      const roleReason = "Requires the operator role or higher";
      const builtinReason = "Built-in packages are immutable";
      return [
        { action: "view", label: "View detail", icon: Eye },
        {
          action: "download",
          label: "Download",
          icon: Download,
          disabled: !row.storageUri,
          disabledReason: row.isBuiltin
            ? "Built-in packages have no tenant-uploaded blob"
            : notReady,
        },
        {
          action: "delete",
          label: "Delete",
          icon: Trash2,
          danger: true,
          disabled: row.isBuiltin || !canMutate,
          disabledReason: row.isBuiltin ? builtinReason : roleReason,
        },
      ];
    },
    [canMutate],
  );

  const hasActiveFilters =
    !!state.search ||
    formats.length > 0 ||
    dbKinds.length > 0 ||
    !!createdWindow.after ||
    !!createdWindow.before;

  const columns = useMemo<ColumnDef<PackageRow>[]>(
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
              placeholder="Search name / version…"
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const r = row.original;
          return (
            <div className="flex flex-col gap-0.5 min-w-0">
              <div className="flex min-w-0 items-center gap-1.5">
                <span
                  className="min-w-0 truncate text-xs text-primary"
                  title={r.name}
                >
                  {r.name || r.id}
                </span>
                {r.isBuiltin && (
                  <Badge
                    variant="secondary"
                    className="shrink-0 px-1 py-0 font-mono text-[8px] uppercase"
                  >
                    built-in
                  </Badge>
                )}
              </div>
              {r.version && (
                <span className="font-mono text-[10px] text-zinc-600 truncate">
                  {r.version}
                </span>
              )}
            </div>
          );
        },
      },
      {
        accessorKey: "format",
        header: () => (
          <ColumnHeader
            label="Format"
            sort={state.sort}
            desc={state.desc}
            onSort={toggleSort}
            filterActive={formatSet.size > 0}
            columnId="format"
            openColumnId={openColumnId}
            onOpenChange={handleOpenChange}
          >
            <ChecklistFilter
              options={PACKAGE_FORMATS.map((f) => ({
                value: f,
                label: PACKAGE_FORMAT_LABEL[f],
                color: PACKAGE_FORMAT_COLOR[f],
              }))}
              selected={formatSet}
              onChange={(next) => setCsv("fmt", next)}
            />
          </ColumnHeader>
        ),
        cell: ({ row }) => {
          const f = row.original.format;
          if (!f)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          const color = PACKAGE_FORMAT_COLOR[f];
          return (
            <div className="flex items-center gap-2">
              <span
                className="h-2 w-2 rounded-full shrink-0"
                style={{ backgroundColor: color }}
              />
              <span className="font-mono text-xs" style={{ color }}>
                {PACKAGE_FORMAT_LABEL[f]}
              </span>
            </div>
          );
        },
      },
      {
        accessorKey: "dbKind",
        header: () => (
          <ColumnHeader
            label="Target"
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
          const sub = [r.os || null, r.arch || null]
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
        accessorKey: "sizeBytes",
        meta: { className: "text-right" },
        header: () => (
          <div className="flex justify-end">
            <ColumnHeader
              label="Size"
              sort={state.sort}
              desc={state.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) => (
          <span className="font-mono text-xs text-zinc-400 tabular-nums">
            {row.original.isBuiltin ? "—" : formatBytes(row.original.sizeBytes)}
          </span>
        ),
      },
      {
        accessorKey: "status",
        meta: { className: "text-center" },
        header: () => (
          <div className="flex justify-center">
            <ColumnHeader
              label="Status"
              sort={state.sort}
              desc={state.desc}
              onSort={toggleSort}
            />
          </div>
        ),
        cell: ({ row }) => {
          const s = row.original.status;
          if (!s)
            return (
              <span className="font-mono text-xs text-zinc-600 text-center block">
                —
              </span>
            );
          const tint = PACKAGE_STATUS_TINT[s];
          return (
            <div className="flex items-center justify-center gap-1.5">
              <span
                className="h-1.5 w-1.5 rounded-full shrink-0"
                style={{ backgroundColor: tint }}
              />
              <span
                className="text-[11px] font-mono leading-none"
                style={{ color: tint }}
              >
                {PACKAGE_STATUS_LABEL[s]}
              </span>
            </div>
          );
        },
      },
      {
        // SHA-256 — the blob checksum (sha256). Truncated + click-to-copy; the
        // full digest is the title. Empty (em dash) until the blob is verified.
        accessorKey: "sha256",
        header: () => (
          <span className="block whitespace-nowrap leading-none">SHA-256</span>
        ),
        enableSorting: false,
        cell: ({ row }) => {
          const sha = row.original.sha256;
          if (!sha)
            return <span className="font-mono text-xs text-zinc-600">—</span>;
          const copied = copiedSha === row.original.id;
          return (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                void navigator.clipboard?.writeText(sha).then(() => {
                  setCopiedSha(row.original.id);
                  setTimeout(
                    () =>
                      setCopiedSha((cur) =>
                        cur === row.original.id ? null : cur,
                      ),
                    1200,
                  );
                });
              }}
              title={`${sha} — click to copy`}
              className="flex items-center gap-1.5 font-mono text-[11px] text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer"
            >
              {copied ? (
                <Check className="h-3 w-3 text-success" />
              ) : (
                <Copy className="h-3 w-3 text-zinc-600" />
              )}
              {shortSha(sha)}
            </button>
          );
        },
      },
      {
        accessorKey: "authorId",
        header: () => (
          <ColumnHeader
            label="Uploaded by"
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
          const display =
            authorDisplays[author] ?? fallbackAuthorDisplay(author);
          return (
            <div className="flex items-center gap-2 min-w-0">
              <Avatar
                name={display.avatarName}
                size={22}
                className="shrink-0"
              />
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
        id: "actions",
        enableSorting: false,
        meta: { className: "px-1", disableRowNavigation: true },
        header: () => (
          <span className="block text-center whitespace-nowrap leading-none">
            Actions
          </span>
        ),
        cell: ({ row }) => {
          const r = row.original;
          return (
            <div className="flex h-full items-center justify-center">
              <RowActionsMenu
                title={r.name || r.id}
                items={actionItemsFor(r)}
                open={openActionId === r.id}
                onOpenChange={(o) => setOpenActionId(o ? r.id : null)}
                onAction={(action) => void runAction(action, r)}
                ariaLabel="Package actions"
              />
            </div>
          );
        },
      },
    ],
    [
      state.sort,
      state.desc,
      state.search,
      formatSet,
      dbSet,
      createdWindow.after,
      createdWindow.before,
      openColumnId,
      handleOpenChange,
      patch,
      setCsv,
      toggleSort,
      copiedSha,
      openActionId,
      actionItemsFor,
      runAction,
      authorDisplays,
    ],
  );

  return (
    <LibraryShell
      shownCount={loading ? undefined : rows.length}
      action={
        canMutate ? (
          <Button asChild size="sm">
            <Link to="/packages/new">
              <UploadCloud className="h-3.5 w-3.5" /> Upload package
            </Link>
          </Button>
        ) : undefined
      }
    >
      <LibraryTable
        columns={columns}
        rows={rows}
        getRowId={(r) => r.id}
        onRowClick={(r) => navigate(packageDetailPath(r.id))}
        columnWidths={PACKAGE_WIDTHS}
        loading={loading}
        error={error}
        emptyLabel="No packages yet"
        emptyFilteredLabel="No packages matching filters"
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
    </LibraryShell>
  );
}

const PACKAGE_WIDTHS: Record<string, string> = {
  name: "18%",
  format: "9%",
  dbKind: "15%",
  sizeBytes: "8%",
  status: "10%",
  sha256: "11%",
  authorId: "12%",
  createdAt: "10%",
  actions: "7%",
};
