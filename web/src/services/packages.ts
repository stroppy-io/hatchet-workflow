// Packages data surface for the Library → Packages table.
//
// Mirrors the runs.ts / preset.ts provider pattern: the page depends ONLY on
// this interface, never on the mock or the wire format. The real provider will
// call cloud.v1.api.PackageService.ListPackages (see
// src/lib/proto/cloud/v1/api/package_pb.ts) and map the proto PackageRecord[]
// onto the flat PackageRow[] here. The mock gate in main.tsx injects a stateful
// mock via setPackagesProvider().
//
// PROTO CORRESPONDENCE — one row maps to one cloud.v1.models.PackageRecord:
//   PackageRow field   PackageRecord field
//   ----------------   -------------------------------------------------------
//   id                 entity.id
//   name               entity.name
//   authorId           entity.author_id   (uploaded-by)
//   createdAt          entity.timings.created_at
//   updatedAt          entity.timings.updated_at
//   format             format             (PackageRecord.Format)
//   version            version
//   dbKind             target_db_kind     (domain.Database.Kind)
//   os                 os
//   arch               arch
//   sizeBytes          size_bytes
//   sha256             sha256             (blob checksum)
//   storageUri         storage_uri        (download / gateway blob path)
//   status             status             (PackageRecord.Status)
//
// SCOPE — the page is built STRICTLY around cloud.v1.api.ListPackagesRequest.
// Wired slice:
//   * filter.search                 -> free-text name search (?q=)
//   * filter.created_after/before   -> created window (?ca=, ?cb=, ISO)
//   * formats[]                     -> format facet (?fmt=, repeatable)
//   * db_kinds[]                    -> target-engine facet (?db=, repeatable)
//   * sort {field, desc}            -> EntitySort over name/created/updated/author
//   * page {size, token}            -> page size + opaque next-page token
// INTENTIONALLY UNWIRED (no column / not in ListPackagesRequest):
//   * filter.ids, filter.updated_after/before, filter.include_deleted,
//     filter.author_ids, filter.favorites_only — no visible column on this page,
//     so no header filter is offered.
//   NOTE: there is NO status facet on ListPackagesRequest, so the Status column
//     sorts/displays only; it has no header filter (per schema).

import type {
  DbKind,
  PackageFormat,
  PackageStatus,
} from "@/components/library-table/labels";

/** Sortable columns — all common Entity columns (ListPackagesRequest.sort). */
export type PackageSortField =
  | "name"
  | "created_at"
  | "updated_at"
  | "author_id";

/** A package, flattened from cloud.v1.models.PackageRecord. */
export interface PackageRow {
  id: string;
  name: string;
  authorId: string;
  createdAt: string;
  updatedAt: string;
  format: PackageFormat;
  version: string;
  dbKind: DbKind;
  os: string;
  arch: string;
  sizeBytes: number;
  /** sha256 — blob checksum, "" while uploading / unverified. */
  sha256: string;
  /** storage_uri — the blob download / gateway path, "" until ready. */
  storageUri: string;
  status: PackageStatus;
}

/**
 * PackageAction enumerates the per-row Actions (⋯) menu items. Packages have NO
 * favorite (there is no FAVORITE_KIND_* for packages in the proto), so the only
 * actions are:
 *   * "view"     -> route to the package (no RPC; always available).
 *   * "download" -> open storage_uri / the gateway blob path (ready only).
 *   * "delete"   -> PackageService.DeletePackage (operator+ role required).
 */
export type PackageAction = "view" | "download" | "delete";

/** The page's view of ListPackagesRequest — only the fields we wire. */
export interface PackagesQuery {
  /** filter.search. */
  search?: string;
  /** formats[]. */
  formats?: Exclude<PackageFormat, "">[];
  /** db_kinds[]. */
  dbKinds?: Exclude<DbKind, "">[];
  /** filter.created_after / created_before (ISO). */
  createdAfter?: string;
  createdBefore?: string;
  /** sort target + direction. */
  sort?: PackageSortField;
  desc?: boolean;
  /** page.size / page.token. */
  pageSize?: number;
  pageToken?: string;
}

export interface PackagesPage {
  rows: PackageRow[];
  nextPageToken: string;
}

/**
 * Metadata the client declares when starting an upload — the wired slice of
 * cloud.v1.api.CreatePackageUploadRequest (the page never sets tenant_id here;
 * the provider resolves it from the slug, and size_bytes/sha256 are
 * server-verified on CompleteUpload, so they are NOT part of this input):
 *
 *   name           -> CreatePackageUploadRequest.name
 *   format         -> CreatePackageUploadRequest.format       (PackageRecord.Format)
 *   version        -> CreatePackageUploadRequest.version
 *   dbKind         -> CreatePackageUploadRequest.target_db_kind (domain.Database.Kind)
 *   os             -> CreatePackageUploadRequest.os
 *   arch           -> CreatePackageUploadRequest.arch
 *   fileName/fileSize describe the chosen blob (used to seed size_bytes the
 *   server verifies on CompleteUpload).
 *
 * NOTE: there is NO UpdatePackage RPC — a package is created via this upload
 * flow and deleted, never edited. The UI therefore offers no edit form.
 */
export interface PackageUploadInput {
  name: string;
  format: Exclude<PackageFormat, "">;
  version: string;
  dbKind: Exclude<DbKind, "">;
  os: string;
  arch: string;
  /** The chosen blob's file name (informational). */
  fileName: string;
  /** The chosen blob's size, in bytes (declared; verified on CompleteUpload). */
  fileSize: number;
}

/**
 * The pending upload target returned by CreatePackageUpload — the freshly
 * created PackageRecord (STATUS_UPLOADING) plus the presigned PUT url and its
 * expiry. Mirrors cloud.v1.api.CreatePackageUploadResponse.
 */
export interface PackageUploadTarget {
  /** The pending record (status === "uploading"). */
  pkg: PackageRow;
  /** upload_url — the presigned PUT target the blob is uploaded to. */
  uploadUrl: string;
  /** upload_url_expires_at (ISO), or "" when unset. */
  uploadUrlExpiresAt: string;
}

export interface PackagesProvider {
  listPackages(tenantSlug: string, query: PackagesQuery): Promise<PackagesPage>;
  /**
   * Step 1 of the upload flow -> PackageService.CreatePackageUpload. Mints a
   * pending PackageRecord (STATUS_UPLOADING) + a presigned PUT url. Operator+
   * role required. Not idempotent.
   */
  createPackageUpload(
    tenantSlug: string,
    input: PackageUploadInput,
  ): Promise<PackageUploadTarget>;
  /**
   * Step 2 of the upload flow -> PackageService.CompleteUpload. Run after the
   * blob has been PUT to the upload url; the server verifies size + sha256 and
   * flips the record to READY (or FAILED), filling size_bytes/sha256/storage_uri.
   * Idempotent. Returns the finalized record.
   */
  completeUpload(tenantSlug: string, id: string): Promise<PackageRow>;
  /** Fetch one package by id -> PackageService.GetPackage. Null when absent. */
  getPackage(tenantSlug: string, id: string): Promise<PackageRow | null>;
  /** Delete a package -> PackageService.DeletePackage. Operator+ role required. */
  deletePackage(tenantSlug: string, id: string): Promise<void>;
}

// --- Real backend provider ---------------------------------------------------
//
// Throws until wired so a missing-backend misconfig is loud, not a silent
// fake-success — the same convention as runs.ts / preset.ts.

const NOT_WIRED =
  "real PackagesProvider not wired yet — run with VITE_MOCK=1 to preview the packages table";

const realPackagesProvider: PackagesProvider = {
  async listPackages() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // const { packages, nextPageToken } = await packageClient.listPackages({
    //   tenantId,
    //   filter: { search: query.search, createdAfter, createdBefore },
    //   formats: query.formats?.map(formatToProto),
    //   dbKinds: query.dbKinds?.map(kindToProto),
    //   sort: { field: sortFieldToEntity(query.sort), desc: query.desc },
    //   page: { size: query.pageSize, token: query.pageToken },
    // });  // cloud.v1.api.PackageService.ListPackages
    // return { rows: packages.map(packageRecordToRow), nextPageToken };
    throw new Error(NOT_WIRED);
  },
  async createPackageUpload() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // const { package: rec, uploadUrl, uploadUrlExpiresAt } =
    //   await packageClient.createPackageUpload({
    //     tenantId,
    //     name: input.name,
    //     format: formatToProto(input.format),
    //     version: input.version,
    //     targetDbKind: kindToProto(input.dbKind),
    //     os: input.os,
    //     arch: input.arch,
    //     sizeBytes: BigInt(input.fileSize),
    //     // sha256 is hashed client-side / verified server-side on CompleteUpload.
    //   });  // cloud.v1.api.PackageService.CreatePackageUpload
    // // The client then PUTs the blob to uploadUrl, then calls completeUpload().
    // return {
    //   pkg: packageRecordToRow(rec!),
    //   uploadUrl,
    //   uploadUrlExpiresAt: uploadUrlExpiresAt ? toIso(uploadUrlExpiresAt) : "",
    // };
    throw new Error(NOT_WIRED);
  },
  async completeUpload() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // const { package: rec } = await packageClient.completeUpload({ tenantId, id });
    //   // cloud.v1.api.PackageService.CompleteUpload
    // return packageRecordToRow(rec!);
    throw new Error(NOT_WIRED);
  },
  async getPackage() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // const { package: rec } = await packageClient.getPackage({ tenantId, id });
    //   // cloud.v1.api.PackageService.GetPackage
    // return rec ? packageRecordToRow(rec) : null;
    throw new Error(NOT_WIRED);
  },
  async deletePackage() {
    // const tenantId = await resolveTenantId(tenantSlug);
    // await packageClient.deletePackage({ tenantId, id });  // PackageService.DeletePackage
    throw new Error(NOT_WIRED);
  },
};

// --- Provider injection ------------------------------------------------------
//
// Defaults to the real backend. The mock (and ONLY the mock) overrides it via
// setPackagesProvider() from the single gate in main.tsx.

let active: PackagesProvider = realPackagesProvider;

export function setPackagesProvider(provider: PackagesProvider): void {
  active = provider;
}

export function getPackagesProvider(): PackagesProvider {
  return active;
}
