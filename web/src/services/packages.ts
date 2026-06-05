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

import { toJson } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import type {
  DbKind,
  PackageFormat,
  PackageStatus,
} from "@/components/library-table/labels";
import {
  PackageRecordSchema,
  PackageRecord_Format,
  type PackageRecord,
} from "@/lib/proto/cloud/v1/models/package_pb";
import { EntitySortField } from "@/lib/proto/cloud/v1/common/entity_pb";
import { packageClient } from "@/services/client";
import { resolveTenantId } from "@/services/tenant";
import { dbKindLabelFromJson, dbKindProto } from "@/services/enums";

// --- enum <-> label converters (package-local) -------------------------------

function formatLabel(s: string | undefined): PackageFormat {
  return s === "FORMAT_DEB" ? "deb" : s === "FORMAT_BINARY" ? "binary" : "";
}
function formatProto(f: Exclude<PackageFormat, "">): PackageRecord_Format {
  return f === "deb" ? PackageRecord_Format.DEB : PackageRecord_Format.BINARY;
}
function statusLabel(s: string | undefined): PackageStatus {
  switch (s) {
    case "STATUS_UPLOADING":
      return "uploading";
    case "STATUS_READY":
      return "ready";
    case "STATUS_FAILED":
      return "failed";
    default:
      return "";
  }
}
function sortFieldToEntity(f?: PackageSortField): EntitySortField {
  switch (f) {
    case "name":
      return EntitySortField.NAME;
    case "created_at":
      return EntitySortField.CREATED_AT;
    case "updated_at":
      return EntitySortField.UPDATED_AT;
    case "author_id":
      return EntitySortField.AUTHOR_ID;
    default:
      return EntitySortField.UNSPECIFIED;
  }
}

// One PackageRecord -> one flat PackageRow (timestamps as ISO via toJson).
function packageRecordToRow(rec: PackageRecord): PackageRow {
  const j = toJson(PackageRecordSchema, rec) as {
    entity?: {
      id?: string;
      name?: string;
      authorId?: string;
      timings?: { createdAt?: string; updatedAt?: string };
    };
    format?: string;
    version?: string;
    targetDbKind?: string;
    os?: string;
    arch?: string;
    sizeBytes?: string | number;
    sha256?: string;
    storageUri?: string;
    status?: string;
  };
  return {
    id: j.entity?.id ?? "",
    name: j.entity?.name ?? "",
    authorId: j.entity?.authorId ?? "",
    createdAt: j.entity?.timings?.createdAt ?? "",
    updatedAt: j.entity?.timings?.updatedAt ?? "",
    format: formatLabel(j.format),
    version: j.version ?? "",
    dbKind: dbKindLabelFromJson(j.targetDbKind),
    os: j.os ?? "",
    arch: j.arch ?? "",
    sizeBytes: Number(j.sizeBytes ?? 0),
    sha256: j.sha256 ?? "",
    storageUri: j.storageUri ?? "",
    status: statusLabel(j.status),
  };
}

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
 * the provider resolves it from the slug):
 *
 *   name           -> CreatePackageUploadRequest.name
 *   format         -> CreatePackageUploadRequest.format       (PackageRecord.Format)
 *   version        -> CreatePackageUploadRequest.version
 *   dbKind         -> CreatePackageUploadRequest.target_db_kind (domain.Database.Kind)
 *   os             -> CreatePackageUploadRequest.os
 *   arch           -> CreatePackageUploadRequest.arch
 *   fileName/fileSize/sha256 describe the chosen blob. The server verifies
 *   size_bytes and sha256 on PUT and again on CompleteUpload.
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
  /** The chosen blob's SHA-256 hex digest (declared; verified on upload). */
  sha256: string;
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

const realPackagesProvider: PackagesProvider = {
  async listPackages(tenantSlug, query) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { packages, nextPageToken } = await packageClient.listPackages({
      tenantId,
      filter: {
        search: query.search,
        createdAfter: query.createdAfter
          ? timestampFromDate(new Date(query.createdAfter))
          : undefined,
        createdBefore: query.createdBefore
          ? timestampFromDate(new Date(query.createdBefore))
          : undefined,
      },
      formats: query.formats?.map(formatProto),
      dbKinds: query.dbKinds?.map(dbKindProto),
      sort: { field: sortFieldToEntity(query.sort), desc: query.desc ?? false },
      page: { size: query.pageSize ?? 0, token: query.pageToken ?? "" },
    });
    return { rows: packages.map(packageRecordToRow), nextPageToken };
  },

  async createPackageUpload(tenantSlug, input) {
    const tenantId = await resolveTenantId(tenantSlug);
    const {
      package: rec,
      uploadUrl,
      uploadUrlExpiresAt,
    } = await packageClient.createPackageUpload({
      tenantId,
      name: input.name,
      format: formatProto(input.format),
      version: input.version,
      targetDbKind: dbKindProto(input.dbKind),
      os: input.os,
      arch: input.arch,
      sizeBytes: BigInt(input.fileSize),
      sha256: input.sha256,
    });
    if (!rec) throw new Error("createPackageUpload returned no package");
    return {
      pkg: packageRecordToRow(rec),
      uploadUrl,
      uploadUrlExpiresAt: uploadUrlExpiresAt
        ? new Date(Number(uploadUrlExpiresAt.seconds) * 1000).toISOString()
        : "",
    };
  },

  async completeUpload(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { package: rec } = await packageClient.completeUpload({ tenantId, id });
    if (!rec) throw new Error("completeUpload returned no package");
    return packageRecordToRow(rec);
  },

  async getPackage(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    const { package: rec } = await packageClient.getPackage({ tenantId, id });
    return rec ? packageRecordToRow(rec) : null;
  },

  async deletePackage(tenantSlug, id) {
    const tenantId = await resolveTenantId(tenantSlug);
    await packageClient.deletePackage({ tenantId, id });
  },
};

export async function sha256File(file: File): Promise<string> {
  const bytes = await file.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
}

export function uploadPackageBlob(
  uploadUrl: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", uploadUrl);
    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable || event.total <= 0) return;
      onProgress?.(Math.min(100, (event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        onProgress?.(100);
        resolve();
        return;
      }
      reject(
        new Error(
          xhr.responseText || `Package upload failed with HTTP ${xhr.status}`,
        ),
      );
    };
    xhr.onerror = () => reject(new Error("Package upload failed"));
    xhr.onabort = () => reject(new Error("Package upload was aborted"));
    xhr.send(file);
  });
}

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
