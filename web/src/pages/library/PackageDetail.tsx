// PackageDetail — the full read view of one cloud.v1.models.PackageRecord,
// fetched via PackageService.GetPackage.
//
// Surfaces every PackageRecord field: name/version, format, target db kind,
// os/arch, size_bytes (human), sha256 (copyable + truncated), status badge,
// storage_uri, author (entity.author_id), created/updated (entity.timings).
//
// ACTIONS:
//   * Download — opens storage_uri (the gateway serves the blob). Disabled until
//     the blob is verified (STATUS_READY).
//   * Delete   — PackageService.DeletePackage behind a confirm dialog, then
//     navigates back to the Packages list. Role-gated to operator+ / admin.
//
// There is NO edit affordance: there is no UpdatePackage RPC — a package is
// created via the upload flow and deleted, never edited.

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  Check,
  Copy,
  Download,
  HardDrive,
  Loader2,
  PackageX,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { Link, useNavigate, useParams, useTenantSlug } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { roleLevel } from "@/lib/roles";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { Avatar } from "@/components/Avatar";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { cn } from "@/lib/utils";
import {
  DB_COLOR,
  DB_LABEL,
  PACKAGE_FORMAT_COLOR,
  PACKAGE_FORMAT_LABEL,
  PACKAGE_STATUS_LABEL,
  PACKAGE_STATUS_TINT,
  formatBytes,
  type PackageStatus,
} from "@/components/library-table/labels";
import { getPackagesProvider, type PackageRow } from "@/services/packages";

function formatTimestamp(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime()) || d.getFullYear() < 2000) return "—";
  return d.toLocaleString("en-GB", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function StatusBadge({ status }: { status: PackageStatus }) {
  if (!status)
    return <span className="font-mono text-xs text-muted-foreground">—</span>;
  const tint = PACKAGE_STATUS_TINT[status];
  const pulse = status === "uploading";
  return (
    <span
      className="inline-flex items-center gap-1.5 px-2 py-0.5 font-mono text-[11px]"
      style={{ color: tint, backgroundColor: `${tint}1f` }}
    >
      <span
        className={cn("h-1.5 w-1.5 rounded-full", pulse && "animate-pulse")}
        style={{ backgroundColor: tint }}
      />
      {PACKAGE_STATUS_LABEL[status]}
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
    <div className="flex flex-col gap-1">
      <span className="font-mono text-[10px] uppercase tracking-wider text-zinc-600">
        {label}
      </span>
      <span className="text-xs text-foreground">{children}</span>
    </div>
  );
}

export function PackageDetail() {
  const { id } = useParams<{ id: string }>();
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { user } = useAuth();

  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canMutate = level >= roleLevel.operator;

  const [pkg, setPkg] = useState<PackageRow | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<number | null>(null);

  useBreadcrumbLabel("id", pkg?.name);

  const load = useCallback(
    async (showSpinner = false) => {
      if (!id) return;
      if (showSpinner) setLoading(true);
      try {
        const row = await getPackagesProvider().getPackage(slug ?? "", id);
        if (!row) {
          setNotFound(true);
          setPkg(null);
        } else {
          setPkg(row);
          setNotFound(false);
        }
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load package");
      } finally {
        setLoading(false);
      }
    },
    [id, slug],
  );

  useEffect(() => {
    void load(true);
  }, [load]);

  // While the blob is still uploading, poll so the detail flips to Ready live
  // (the mock completes via the upload form; this keeps a deep-linked view fresh).
  const uploading = pkg?.status === "uploading";
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    if (!uploading) return;
    const t = window.setInterval(() => void loadRef.current(false), 2500);
    return () => window.clearInterval(t);
  }, [uploading]);

  const copySha = useCallback((sha: string) => {
    void navigator.clipboard?.writeText(sha).then(() => {
      setCopied(true);
      if (copyTimer.current !== null) window.clearTimeout(copyTimer.current);
      copyTimer.current = window.setTimeout(() => setCopied(false), 1200);
    });
  }, []);

  const download = useCallback(() => {
    if (pkg?.storageUri) window.open(pkg.storageUri, "_blank", "noopener");
  }, [pkg]);

  const remove = useCallback(async () => {
    if (!pkg) return;
    const ok = await confirm({
      title: "Delete package?",
      description: `“${pkg.name || pkg.id}” will be permanently removed. This cannot be undone.`,
      danger: true,
      confirmLabel: "Delete",
    });
    if (!ok) return;
    setBusy(true);
    try {
      await getPackagesProvider().deletePackage(slug ?? "", pkg.id);
      navigate("/packages");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete package");
      setBusy(false);
    }
  }, [pkg, slug, confirm, navigate]);

  const shortSha = useMemo(
    () => (pkg?.sha256 ? pkg.sha256.slice(0, 16) : ""),
    [pkg],
  );

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading package…
      </div>
    );
  }

  if (notFound) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
        <PackageX className="h-8 w-8 text-zinc-600" />
        <div className="text-sm text-foreground">Package not found</div>
        <div className="text-xs text-muted-foreground">
          The package <span className="font-mono">{id}</span> does not exist or
          was removed.
        </div>
        <Button asChild variant="outline" size="sm">
          <Link to="/packages">
            <ArrowLeft className="h-3.5 w-3.5" /> Back to Packages
          </Link>
        </Button>
      </div>
    );
  }

  if (!pkg) {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="flex items-center gap-2 border border-destructive/30 p-3 text-sm text-destructive">
          <AlertCircle className="h-4 w-4" />
          {error ?? "Failed to load package"}
        </div>
      </div>
    );
  }

  const formatColor = pkg.format ? PACKAGE_FORMAT_COLOR[pkg.format] : undefined;
  const dbColor = pkg.dbKind ? DB_COLOR[pkg.dbKind] : undefined;
  const ready = pkg.status === "ready";

  return (
    <div className="mx-auto flex h-full min-h-0 w-full max-w-3xl flex-col gap-4 overflow-auto p-5">
      {error && (
        <div className="flex items-center gap-2 border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <AlertCircle className="h-3.5 w-3.5" />
          {error}
        </div>
      )}

      {/* HEADER */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          <Link
            to="/packages"
            className="mt-0.5 text-zinc-500 hover:text-foreground"
            aria-label="Back to Packages"
            title="Back to Packages"
          >
            <ArrowLeft className="h-5 w-5" />
          </Link>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="truncate font-mono text-lg font-semibold tracking-tight">
                {pkg.name || pkg.id}
              </h1>
              <StatusBadge status={pkg.status} />
            </div>
            <div className="mt-0.5 flex items-center gap-2 text-xs text-muted-foreground">
              <span className="font-mono">{pkg.id}</span>
              {pkg.version && (
                <span className="font-mono text-zinc-400">· {pkg.version}</span>
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
          <Button
            size="sm"
            onClick={download}
            disabled={!ready || !pkg.storageUri}
            title={
              ready
                ? "Download the package blob"
                : "Available once the package is uploaded (Ready)"
            }
          >
            <Download className="h-3.5 w-3.5" /> Download
          </Button>
          {canMutate && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => void remove()}
              disabled={busy}
              className="text-destructive hover:text-destructive"
            >
              <Trash2 className="h-3.5 w-3.5" /> Delete
            </Button>
          )}
        </div>
      </div>

      {/* METADATA */}
      <Card className="flex flex-col gap-4 p-4">
        <h2 className="font-mono text-xs font-semibold uppercase tracking-wider text-zinc-500">
          Package
        </h2>
        <div className="grid grid-cols-2 gap-x-4 gap-y-4 sm:grid-cols-3 lg:grid-cols-4">
          <MetaItem label="Format">
            {pkg.format ? (
              <span className="inline-flex items-center gap-1.5">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: formatColor }}
                />
                <span className="font-mono" style={{ color: formatColor }}>
                  {PACKAGE_FORMAT_LABEL[pkg.format]}
                </span>
              </span>
            ) : (
              "—"
            )}
          </MetaItem>
          <MetaItem label="Target database">
            {pkg.dbKind ? (
              <span className="inline-flex items-center gap-1.5">
                <span
                  className="h-2 w-2 rounded-full"
                  style={{ backgroundColor: dbColor }}
                />
                <span className="font-mono" style={{ color: dbColor }}>
                  {DB_LABEL[pkg.dbKind]}
                </span>
              </span>
            ) : (
              "—"
            )}
          </MetaItem>
          <MetaItem label="Version">
            <span className="font-mono">{pkg.version || "—"}</span>
          </MetaItem>
          <MetaItem label="Status">
            <StatusBadge status={pkg.status} />
          </MetaItem>
          <MetaItem label="OS">
            <span className="font-mono">{pkg.os || "—"}</span>
          </MetaItem>
          <MetaItem label="Architecture">
            <span className="font-mono">{pkg.arch || "—"}</span>
          </MetaItem>
          <MetaItem label="Size">
            <span className="font-mono tabular-nums">
              {formatBytes(pkg.sizeBytes)}
            </span>
          </MetaItem>
          <MetaItem label="Uploaded by">
            <span className="inline-flex items-center gap-1.5">
              <Avatar name={pkg.authorId || "unknown"} size={16} />
              <span className="font-mono">{pkg.authorId || "—"}</span>
            </span>
          </MetaItem>
          <MetaItem label="Created">
            <span title={pkg.createdAt}>{formatTimestamp(pkg.createdAt)}</span>
          </MetaItem>
          <MetaItem label="Updated">
            <span title={pkg.updatedAt}>{formatTimestamp(pkg.updatedAt)}</span>
          </MetaItem>
        </div>
      </Card>

      {/* BLOB — checksum + storage location. */}
      <Card className="flex flex-col gap-4 p-4">
        <h2 className="flex items-center gap-2 font-mono text-xs font-semibold uppercase tracking-wider text-zinc-500">
          <HardDrive className="h-3.5 w-3.5" /> Blob
        </h2>
        <div className="flex flex-col gap-4">
          <MetaItem label="SHA-256">
            {pkg.sha256 ? (
              <button
                type="button"
                onClick={() => copySha(pkg.sha256)}
                title={`${pkg.sha256} — click to copy`}
                className="inline-flex items-center gap-2 font-mono text-xs text-zinc-300 transition-colors hover:text-foreground"
              >
                {copied ? (
                  <Check className="h-3.5 w-3.5 text-success" />
                ) : (
                  <Copy className="h-3.5 w-3.5 text-zinc-600" />
                )}
                <span className="break-all">{shortSha}…</span>
              </button>
            ) : (
              <span className="text-muted-foreground">
                — (computed on upload completion)
              </span>
            )}
          </MetaItem>
          <MetaItem label="Storage URI">
            {pkg.storageUri ? (
              <span className="break-all font-mono text-xs text-zinc-400">
                {pkg.storageUri}
              </span>
            ) : (
              <span className="text-muted-foreground">
                — (assigned once Ready)
              </span>
            )}
          </MetaItem>
        </div>
      </Card>
    </div>
  );
}
