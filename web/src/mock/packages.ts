// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Stateful per-tenant package lists so the Library → Packages table renders
// without a backend. Keyed by tenant SLUG so switching orgs shows different
// data. Every PackageRow field maps 1:1 to cloud.v1.models.PackageRecord
// (entity + format + version + target_db_kind + os + arch + size_bytes + status).
//
// Honours the slice of cloud.v1.api.ListPackagesRequest the page wires —
// filter.search, filter.created_after/before, formats[], db_kinds[],
// sort{field,desc}, page{size,token} — applied in-memory so the filters, sort
// and paging are genuinely exercisable in the standalone preview.

import {
  type PackageRow,
  type PackagesPage,
  type PackagesProvider,
  type PackagesQuery,
  type PackageSortField,
  type PackageUploadInput,
  type PackageUploadTarget,
} from "@/services/packages";
import type {
  DbKind,
  PackageFormat,
  PackageStatus,
} from "@/components/library-table/labels";

function delay(): Promise<void> {
  return new Promise((r) => setTimeout(r, 130));
}

function daysAgo(d: number): string {
  return new Date(Date.now() - d * 86_400_000).toISOString();
}

const MiB = 1024 * 1024;

function pkg(p: {
  id: string;
  name: string;
  author: string;
  format: PackageFormat;
  version: string;
  dbKind: DbKind;
  os: string;
  arch: string;
  sizeMiB: number;
  sha256?: string;
  status: PackageStatus;
  createdDaysAgo: number;
  updatedDaysAgo: number;
}): PackageRow {
  // A deterministic, believable 64-hex sha256 when the package is verified.
  const sha =
    p.status === "ready"
      ? (p.sha256 ??
        Array.from(p.id + p.version)
          .map((c) => c.charCodeAt(0).toString(16).padStart(2, "0"))
          .join("")
          .padEnd(64, "0")
          .slice(0, 64))
      : "";
  return {
    id: p.id,
    name: p.name,
    authorId: p.author,
    format: p.format,
    version: p.version,
    dbKind: p.dbKind,
    os: p.os,
    arch: p.arch,
    sizeBytes: Math.round(p.sizeMiB * MiB),
    sha256: sha,
    storageUri: p.status === "ready" ? `/packages/${p.id}/download` : "",
    status: p.status,
    createdAt: daysAgo(p.createdDaysAgo),
    updatedAt: daysAgo(p.updatedDaysAgo),
  };
}

const ACME: PackageRow[] = [
  pkg({ id: "pkg-pg16-deb", name: "postgresql-16-custom", author: "ada", format: "deb", version: "16.2-1", dbKind: "postgres", os: "ubuntu-22.04", arch: "amd64", sizeMiB: 42.5, status: "ready", createdDaysAgo: 30, updatedDaysAgo: 30 }),
  pkg({ id: "pkg-pg16-arm", name: "postgresql-16-custom", author: "ada", format: "deb", version: "16.2-1", dbKind: "postgres", os: "ubuntu-22.04", arch: "arm64", sizeMiB: 41.1, status: "ready", createdDaysAgo: 30, updatedDaysAgo: 30 }),
  pkg({ id: "pkg-crdb-bin", name: "cockroach-patched", author: "grace", format: "binary", version: "24.2.0-rc1", dbKind: "cockroach", os: "linux", arch: "amd64", sizeMiB: 210.0, status: "ready", createdDaysAgo: 12, updatedDaysAgo: 3 }),
  pkg({ id: "pkg-mysql-deb", name: "mysql-8.4-debug", author: "max", format: "deb", version: "8.4.0-dbg", dbKind: "mysql", os: "debian-12", arch: "amd64", sizeMiB: 88.7, status: "ready", createdDaysAgo: 20, updatedDaysAgo: 8 }),
  pkg({ id: "pkg-ydb-bin", name: "ydbd-experimental", author: "ada", format: "binary", version: "25.2-exp", dbKind: "ydb", os: "linux", arch: "amd64", sizeMiB: 512.0, status: "uploading", createdDaysAgo: 0.2, updatedDaysAgo: 0.2 }),
  pkg({ id: "pkg-maria-deb", name: "mariadb-11.4-galera", author: "grace", format: "deb", version: "11.4.2", dbKind: "mariadb", os: "ubuntu-24.04", arch: "amd64", sizeMiB: 64.3, status: "failed", createdDaysAgo: 5, updatedDaysAgo: 5 }),
  pkg({ id: "pkg-pico-bin", name: "picodata-nightly", author: "max", format: "binary", version: "25.3-nightly", dbKind: "picodata", os: "linux", arch: "amd64", sizeMiB: 95.2, status: "ready", createdDaysAgo: 2, updatedDaysAgo: 1 }),
];

const GLOBEX: PackageRow[] = [
  pkg({ id: "g-pkg-pg", name: "postgresql-16-ci", author: "ci", format: "deb", version: "16.1-ci", dbKind: "postgres", os: "ubuntu-22.04", arch: "amd64", sizeMiB: 40.0, status: "ready", createdDaysAgo: 6, updatedDaysAgo: 2 }),
  pkg({ id: "g-pkg-mysql", name: "mysql-8.4-ci", author: "ci", format: "binary", version: "8.4-ci", dbKind: "mysql", os: "linux", arch: "amd64", sizeMiB: 120.0, status: "uploading", createdDaysAgo: 0.1, updatedDaysAgo: 0.1 }),
];

const BY_SLUG: Record<string, PackageRow[]> = { acme: ACME, globex: GLOBEX };

function rows(slug: string): PackageRow[] {
  let list = BY_SLUG[slug];
  if (!list) {
    list = [];
    BY_SLUG[slug] = list;
  }
  return list;
}

function matchesQuery(r: PackageRow, q: PackagesQuery): boolean {
  if (q.search) {
    const s = q.search.toLowerCase();
    if (
      !r.name.toLowerCase().includes(s) &&
      !r.version.toLowerCase().includes(s)
    )
      return false;
  }
  if (q.formats && q.formats.length > 0) {
    if (r.format === "" || !q.formats.includes(r.format)) return false;
  }
  if (q.dbKinds && q.dbKinds.length > 0) {
    if (r.dbKind === "" || !q.dbKinds.includes(r.dbKind)) return false;
  }
  if (q.createdAfter && r.createdAt < q.createdAfter) return false;
  if (q.createdBefore && r.createdAt > q.createdBefore) return false;
  return true;
}

function compare(a: PackageRow, b: PackageRow, sort: PackageSortField | undefined, desc: boolean | undefined): number {
  const dir = desc ? -1 : 1;
  switch (sort) {
    case "name":
      return dir * a.name.localeCompare(b.name);
    case "author_id":
      return dir * a.authorId.localeCompare(b.authorId);
    case "updated_at":
      return dir * a.updatedAt.localeCompare(b.updatedAt);
    case "created_at":
    default:
      return dir * a.createdAt.localeCompare(b.createdAt);
  }
}

const DEFAULT_SIZE = 10;

// A believable 64-hex sha256 derived deterministically from a seed, so a
// completed upload shows a stable, copyable digest in the detail page.
function fakeSha256(seed: string): string {
  let h = 0x811c9dc5;
  const out: string[] = [];
  for (let i = 0; i < 64; i++) {
    const c = seed.charCodeAt(i % seed.length) + i * 31;
    h = (h ^ c) >>> 0;
    h = (h * 0x01000193) >>> 0;
    out.push((h & 0xff).toString(16).padStart(2, "0"));
  }
  return out.join("").slice(0, 64);
}

// Monotonic suffix so two uploads of the same name get distinct ids.
let uploadSeq = 0;

export const mockPackagesProvider: PackagesProvider = {
  async listPackages(tenantSlug: string, query: PackagesQuery): Promise<PackagesPage> {
    await delay();
    const all = rows(tenantSlug);
    const filtered = all
      .filter((r) => matchesQuery(r, query))
      // default ordering = created_at desc, matching the page default sort.
      .sort((a, b) => compare(a, b, query.sort ?? "created_at", query.sort ? query.desc : true));

    const size = query.pageSize && query.pageSize > 0 ? query.pageSize : DEFAULT_SIZE;
    const offset = query.pageToken ? Number.parseInt(query.pageToken, 10) || 0 : 0;
    const slice = filtered.slice(offset, offset + size);
    const nextOffset = offset + size;
    const nextPageToken = nextOffset < filtered.length ? String(nextOffset) : "";

    return { rows: slice, nextPageToken };
  },

  // PackageService.CreatePackageUpload (mock): mint a pending row (uploading)
  // and a fake presigned PUT target, and prepend it to the tenant store so it
  // appears immediately in the Packages list.
  async createPackageUpload(
    tenantSlug: string,
    input: PackageUploadInput,
  ): Promise<PackageUploadTarget> {
    await delay();
    const list = rows(tenantSlug);
    uploadSeq += 1;
    const slugName = (input.name || "package")
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "");
    const id = `pkg-${slugName || "package"}-${Date.now().toString(36)}${uploadSeq}`;
    const now = new Date().toISOString();
    const pending: PackageRow = {
      id,
      name: input.name,
      authorId: "you",
      format: input.format,
      version: input.version,
      dbKind: input.dbKind,
      os: input.os,
      arch: input.arch,
      // size is declared up front; sha256 + storage_uri are filled on complete.
      sizeBytes: Math.max(0, Math.round(input.fileSize)),
      sha256: "",
      storageUri: "",
      status: "uploading",
      createdAt: now,
      updatedAt: now,
    };
    list.unshift(pending);
    const expires = new Date(Date.now() + 15 * 60_000).toISOString();
    return {
      pkg: { ...pending },
      uploadUrl: `https://mock-blob.local/upload/${tenantSlug}/${id}?sig=presigned`,
      uploadUrlExpiresAt: expires,
    };
  },

  // PackageService.CompleteUpload (mock): flip the pending row to READY,
  // filling size_bytes / sha256 / storage_uri as the server would after
  // verifying the blob. Idempotent — completing a READY row is a no-op.
  async completeUpload(tenantSlug: string, id: string): Promise<PackageRow> {
    await delay();
    const list = rows(tenantSlug);
    const row = list.find((r) => r.id === id);
    if (!row) throw new Error(`package ${id} not found`);
    if (row.status !== "ready") {
      row.status = "ready";
      row.sha256 = fakeSha256(id + row.version + row.name);
      row.storageUri = `/packages/${id}/download`;
      row.updatedAt = new Date().toISOString();
    }
    return { ...row };
  },

  // PackageService.GetPackage (mock): return a copy of the row, or null.
  async getPackage(tenantSlug: string, id: string): Promise<PackageRow | null> {
    await delay();
    const row = rows(tenantSlug).find((r) => r.id === id);
    return row ? { ...row } : null;
  },

  // PackageService.DeletePackage (mock): drop the row from its tenant store.
  async deletePackage(tenantSlug: string, id: string): Promise<void> {
    await delay();
    const list = rows(tenantSlug);
    const idx = list.findIndex((r) => r.id === id);
    if (idx >= 0) list.splice(idx, 1);
  },
};
