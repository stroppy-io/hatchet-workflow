// PackageUploadForm — the create/upload flow for PackageService.
//
// Built STRICTLY around the two-step presigned-upload contract in
// cloud/v1/api/package.proto:
//
//   1. CreatePackageUpload  — declares the metadata (name / format / version /
//      target_db_kind / os / arch + the declared blob size) and mints a pending
//      PackageRecord (STATUS_UPLOADING) + a presigned PUT url.
//   2. <client PUTs the blob to upload_url>  — simulated here as a progress bar.
//   3. CompleteUpload       — the server verifies size + sha256 and flips the
//      record to STATUS_READY, filling size_bytes / sha256 / storage_uri.
//
// There is NO UpdatePackage RPC: a package is created via this flow and deleted,
// never edited — so there is no "edit" affordance anywhere (see the note below).
// On a READY result the page navigates to the package detail.

import { useCallback, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  FileUp,
  Loader2,
  UploadCloud,
  X,
} from "lucide-react";
import { Link, useNavigate, useTenantSlug } from "@/lib/router";
import { useAuth } from "@/hooks/useAuth";
import { roleLevel } from "@/lib/roles";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import {
  DB_COLOR,
  DB_KINDS,
  DB_LABEL,
  PACKAGE_FORMATS,
  PACKAGE_FORMAT_COLOR,
  PACKAGE_FORMAT_LABEL,
  formatBytes,
  type DbKind,
  type PackageFormat,
} from "@/components/library-table/labels";
import {
  getPackagesProvider,
  type PackageUploadInput,
} from "@/services/packages";

// The upload runs through a tiny state machine so the UI can show progress and
// each proto step distinctly.
type Phase =
  | { kind: "idle" }
  | { kind: "creating" } // CreatePackageUpload in flight
  | { kind: "uploading"; pct: number; id: string } // PUT to upload_url (simulated)
  | { kind: "completing"; id: string } // CompleteUpload in flight
  | { kind: "done"; id: string }
  | { kind: "error"; message: string };

const OS_SUGGESTIONS = [
  "ubuntu-22.04",
  "ubuntu-24.04",
  "debian-12",
  "linux",
];
const ARCH_SUGGESTIONS = ["amd64", "arm64"];

export function PackageUploadForm() {
  const slug = useTenantSlug();
  const navigate = useNavigate();
  const { user } = useAuth();

  // operator+ (>=2) on the active tenant, or platform admin, may upload.
  const tenant = user?.tenants.find((t) => t.slug === slug);
  const level = user?.isAdmin ? 99 : tenant ? roleLevel[tenant.role] : 0;
  const canUpload = level >= roleLevel.operator;

  // --- metadata form state (mirrors CreatePackageUploadRequest) -------------
  const [name, setName] = useState("");
  const [format, setFormat] = useState<Exclude<PackageFormat, "">>("deb");
  const [version, setVersion] = useState("");
  const [dbKind, setDbKind] = useState<Exclude<DbKind, "">>("postgres");
  const [os, setOs] = useState("ubuntu-22.04");
  const [arch, setArch] = useState("amd64");
  const [file, setFile] = useState<{ name: string; size: number } | null>(null);

  const [phase, setPhase] = useState<Phase>({ kind: "idle" });
  const fileInputRef = useRef<HTMLInputElement>(null);
  const progressTimer = useRef<number | null>(null);

  const busy =
    phase.kind === "creating" ||
    phase.kind === "uploading" ||
    phase.kind === "completing";

  const canSubmit =
    canUpload &&
    !busy &&
    name.trim().length > 0 &&
    version.trim().length > 0 &&
    !!file;

  const input = useMemo<PackageUploadInput>(
    () => ({
      name: name.trim(),
      format,
      version: version.trim(),
      dbKind,
      os: os.trim(),
      arch: arch.trim(),
      fileName: file?.name ?? "",
      fileSize: file?.size ?? 0,
    }),
    [name, format, version, dbKind, os, arch, file],
  );

  const onFilePicked = useCallback((f: File | null | undefined) => {
    if (!f) return;
    setFile({ name: f.name, size: f.size });
    // Best-effort: seed the name from the file if still empty.
    setName((n) => n || f.name.replace(/\.(deb|bin|tar\.gz|tgz|zip)$/i, ""));
    // Infer format from the extension when it is a .deb.
    if (/\.deb$/i.test(f.name)) setFormat("deb");
  }, []);

  const submit = useCallback(async () => {
    if (!slug || !canSubmit) return;
    const provider = getPackagesProvider();
    try {
      // Step 1 — CreatePackageUpload: mint the pending record + upload target.
      setPhase({ kind: "creating" });
      const target = await provider.createPackageUpload(slug, input);
      const id = target.pkg.id;

      // Step 2 — PUT the blob to target.uploadUrl. The backend does this for
      // real; here we simulate a progress bar against the presigned url.
      await new Promise<void>((resolve) => {
        setPhase({ kind: "uploading", pct: 0, id });
        let pct = 0;
        progressTimer.current = window.setInterval(() => {
          pct = Math.min(100, pct + 12 + Math.random() * 14);
          if (pct >= 100) {
            if (progressTimer.current !== null)
              window.clearInterval(progressTimer.current);
            progressTimer.current = null;
            setPhase({ kind: "uploading", pct: 100, id });
            resolve();
          } else {
            setPhase({ kind: "uploading", pct, id });
          }
        }, 180);
      });

      // Step 3 — CompleteUpload: server verifies size + sha256, flips to READY.
      setPhase({ kind: "completing", id });
      const done = await provider.completeUpload(slug, id);
      setPhase({ kind: "done", id });

      if (done.status === "failed") {
        setPhase({
          kind: "error",
          message: "Upload verification failed (size / sha256 mismatch).",
        });
        return;
      }
      // On success, land on the package detail.
      navigate(`/packages/${id}`);
    } catch (err) {
      if (progressTimer.current !== null) {
        window.clearInterval(progressTimer.current);
        progressTimer.current = null;
      }
      setPhase({
        kind: "error",
        message: err instanceof Error ? err.message : "Upload failed",
      });
    }
  }, [slug, canSubmit, input, navigate]);

  const progressPct =
    phase.kind === "uploading"
      ? Math.round(phase.pct)
      : phase.kind === "completing" || phase.kind === "done"
        ? 100
        : 0;

  const stepLabel: string =
    phase.kind === "creating"
      ? "Creating upload…"
      : phase.kind === "uploading"
        ? `Uploading blob… ${Math.round(phase.pct)}%`
        : phase.kind === "completing"
          ? "Verifying (CompleteUpload)…"
          : phase.kind === "done"
            ? "Ready"
            : "";

  return (
    <div className="mx-auto flex h-full min-h-0 w-full max-w-2xl flex-col gap-4 overflow-auto p-5">
      {/* HEADER */}
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <h1 className="flex items-center gap-2 font-mono text-lg font-semibold tracking-tight">
            <UploadCloud className="h-5 w-5 text-primary" />
            Upload package
          </h1>
          <p className="text-xs leading-snug text-muted-foreground">
            A tenant-private custom database build (a{" "}
            <span className="font-mono">.deb</span> or raw binary). Stored via a
            two-step presigned upload — metadata first, then the blob.
          </p>
        </div>
        <Button asChild variant="outline" size="sm">
          <Link to="/packages">
            <ArrowLeft className="h-3.5 w-3.5" /> Packages
          </Link>
        </Button>
      </div>

      {/* No-edit note — packages are create-then-delete; there is no
          UpdatePackage RPC, so this is the only way to (re)author a build. */}
      <div className="flex items-start gap-2 border border-border bg-muted/30 px-3 py-2 text-[11px] leading-snug text-muted-foreground">
        <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-zinc-500" />
        <span>
          Packages have no edit form — there is no{" "}
          <span className="font-mono">UpdatePackage</span> RPC. To change a
          build, upload a new package (e.g. a new version) and delete the old
          one.
        </span>
      </div>

      {!canUpload && (
        <div className="flex items-center gap-2 border border-warning/30 bg-warning/5 px-3 py-2 text-xs text-warning">
          <AlertCircle className="h-3.5 w-3.5" />
          Uploading a package requires the operator role or higher.
        </div>
      )}

      {phase.kind === "error" && (
        <div className="flex items-center justify-between gap-2 border border-destructive/30 bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <span className="flex items-center gap-2">
            <AlertCircle className="h-3.5 w-3.5" />
            {phase.message}
          </span>
          <button
            onClick={() => setPhase({ kind: "idle" })}
            aria-label="Dismiss"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      <Card className="flex flex-col gap-4 p-4">
        {/* name + version */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Name" htmlFor="pkg-name">
            <Input
              id="pkg-name"
              value={name}
              disabled={busy}
              onChange={(e) => setName(e.target.value)}
              placeholder="postgresql-16-custom"
              className="font-mono"
            />
          </Field>
          <Field label="Version" htmlFor="pkg-version">
            <Input
              id="pkg-version"
              value={version}
              disabled={busy}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="16.2-1"
              className="font-mono"
            />
          </Field>
        </div>

        {/* format + target db */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="Format" htmlFor="pkg-format">
            <Select
              value={format}
              onValueChange={(v) => setFormat(v as Exclude<PackageFormat, "">)}
              disabled={busy}
            >
              <SelectTrigger id="pkg-format" className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PACKAGE_FORMATS.map((f) => (
                  <SelectItem key={f} value={f}>
                    <span className="flex items-center gap-2">
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: PACKAGE_FORMAT_COLOR[f] }}
                      />
                      {PACKAGE_FORMAT_LABEL[f]}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Target database" htmlFor="pkg-db">
            <Select
              value={dbKind}
              onValueChange={(v) => setDbKind(v as Exclude<DbKind, "">)}
              disabled={busy}
            >
              <SelectTrigger id="pkg-db" className="h-9">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DB_KINDS.map((k) => (
                  <SelectItem key={k} value={k}>
                    <span className="flex items-center gap-2">
                      <span
                        className="h-2 w-2 rounded-full"
                        style={{ backgroundColor: DB_COLOR[k] }}
                      />
                      {DB_LABEL[k]}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        </div>

        {/* os + arch */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Field label="OS" htmlFor="pkg-os">
            <Input
              id="pkg-os"
              value={os}
              disabled={busy}
              onChange={(e) => setOs(e.target.value)}
              placeholder="ubuntu-22.04"
              list="pkg-os-list"
              className="font-mono"
            />
            <datalist id="pkg-os-list">
              {OS_SUGGESTIONS.map((o) => (
                <option key={o} value={o} />
              ))}
            </datalist>
          </Field>
          <Field label="Architecture" htmlFor="pkg-arch">
            <Input
              id="pkg-arch"
              value={arch}
              disabled={busy}
              onChange={(e) => setArch(e.target.value)}
              placeholder="amd64"
              list="pkg-arch-list"
              className="font-mono"
            />
            <datalist id="pkg-arch-list">
              {ARCH_SUGGESTIONS.map((a) => (
                <option key={a} value={a} />
              ))}
            </datalist>
          </Field>
        </div>

        {/* file picker */}
        <Field label="Package file" htmlFor="pkg-file">
          <input
            ref={fileInputRef}
            id="pkg-file"
            type="file"
            className="sr-only"
            disabled={busy}
            onChange={(e) => onFilePicked(e.target.files?.[0])}
          />
          <button
            type="button"
            disabled={busy}
            onClick={() => fileInputRef.current?.click()}
            className={cn(
              "flex w-full items-center gap-3 border border-dashed border-border px-3 py-3 text-left transition-colors",
              !busy && "hover:border-primary hover:bg-primary/5",
              busy && "cursor-not-allowed opacity-60",
            )}
          >
            <FileUp className="h-5 w-5 shrink-0 text-zinc-500" />
            {file ? (
              <span className="flex min-w-0 flex-col">
                <span className="truncate font-mono text-xs text-foreground">
                  {file.name}
                </span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {formatBytes(file.size)}
                </span>
              </span>
            ) : (
              <span className="text-xs text-muted-foreground">
                Choose the package blob to upload…
              </span>
            )}
          </button>
        </Field>

        {/* progress — visible once an upload starts. */}
        {(busy || phase.kind === "done") && (
          <div className="flex flex-col gap-1.5 border border-border bg-muted/20 px-3 py-2.5">
            <div className="flex items-center justify-between text-[11px] font-mono">
              <span className="flex items-center gap-1.5 text-foreground">
                {phase.kind === "done" ? (
                  <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                ) : (
                  <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
                )}
                {stepLabel}
              </span>
              <span className="tabular-nums text-muted-foreground">
                {progressPct}%
              </span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary transition-all"
                style={{ width: `${progressPct}%` }}
              />
            </div>
          </div>
        )}

        {/* actions */}
        <div className="flex items-center justify-end gap-2 border-t border-border pt-3">
          <Button asChild variant="outline" size="sm" disabled={busy}>
            <Link to="/packages">Cancel</Link>
          </Button>
          <Button size="sm" disabled={!canSubmit} onClick={() => void submit()}>
            {busy ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" /> Uploading…
              </>
            ) : (
              <>
                <UploadCloud className="h-3.5 w-3.5" /> Upload package
              </>
            )}
          </Button>
        </div>
      </Card>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
    </div>
  );
}
