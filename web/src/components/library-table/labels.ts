// Shared label/colour catalogs for the Library tables (Database / Workload /
// Test presets + Packages). These mirror the value unions the proto generates
// (cloud.v1.domain.Database.Kind, cloud.v1.domain.Workload.Protocol,
// cloud.v1.models.PackageRecord.Format / .Status) lower-cased with UNSPECIFIED
// collapsed to "". Kept in one place so all four pages stay visually consistent
// with the Test Runs table (same db hues, same protocol names).

/** Database kind, mapped from cloud.v1.domain.Database.Kind. */
export type DbKind =
  | ""
  | "postgres"
  | "mysql"
  | "mariadb"
  | "ydb"
  | "ydb_managed"
  | "cockroach"
  | "picodata"
  | "orioledb"
  | "external";

/** All selectable db kinds (UNSPECIFIED excluded), for facet controls. */
export const DB_KINDS: Exclude<DbKind, "">[] = [
  "postgres",
  "mysql",
  "mariadb",
  "ydb",
  "ydb_managed",
  "cockroach",
  "picodata",
  "orioledb",
  "external",
];

export const DB_LABEL: Record<Exclude<DbKind, "">, string> = {
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

// Per-database accent colour — identical to the Test Runs table so an engine
// reads the same hue everywhere.
export const DB_COLOR: Record<Exclude<DbKind, "">, string> = {
  postgres: "#38bdf8",
  mysql: "#f59e0b",
  mariadb: "#a78bfa",
  ydb: "#34d399",
  ydb_managed: "#2dd4bf",
  cockroach: "#f472b6",
  picodata: "#fb7185",
  orioledb: "#E8633A",
  external: "#9ca3af",
};

/** Workload wire protocol, mapped from cloud.v1.domain.Workload.Protocol. */
export type Protocol =
  | ""
  | "pg"
  | "mysql"
  | "picodata"
  | "ydb_grpc"
  | "ydb_grpcs"
  | "cockroach";

export const PROTOCOLS: Exclude<Protocol, "">[] = [
  "pg",
  "mysql",
  "picodata",
  "ydb_grpc",
  "ydb_grpcs",
  "cockroach",
];

export const PROTOCOL_LABEL: Record<Exclude<Protocol, "">, string> = {
  pg: "PG",
  mysql: "MySQL",
  picodata: "Picodata",
  ydb_grpc: "YDB gRPC",
  ydb_grpcs: "YDB gRPCs",
  cockroach: "CockroachDB",
};

/** Package format, mapped from cloud.v1.models.PackageRecord.Format. */
export type PackageFormat = "" | "deb" | "binary";

export const PACKAGE_FORMATS: Exclude<PackageFormat, "">[] = ["deb", "binary"];

export const PACKAGE_FORMAT_LABEL: Record<Exclude<PackageFormat, "">, string> = {
  deb: ".deb",
  binary: "Binary",
};

export const PACKAGE_FORMAT_COLOR: Record<Exclude<PackageFormat, "">, string> = {
  deb: "#a78bfa", // violet — apt/dpkg
  binary: "#38bdf8", // sky — raw binary
};

/** Package lifecycle, mapped from cloud.v1.models.PackageRecord.Status. */
export type PackageStatus = "" | "uploading" | "ready" | "failed";

export const PACKAGE_STATUSES: Exclude<PackageStatus, "">[] = [
  "uploading",
  "ready",
  "failed",
];

export const PACKAGE_STATUS_LABEL: Record<Exclude<PackageStatus, "">, string> = {
  uploading: "Uploading",
  ready: "Ready",
  failed: "Failed",
};

// Status accent hues, drawn from the same palette the Test Runs status cell uses
// (warning/success/destructive).
export const PACKAGE_STATUS_TINT: Record<Exclude<PackageStatus, "">, string> = {
  uploading: "#eab308", // --color-warning
  ready: "#22c55e", // --color-success
  failed: "#ef4444", // --color-destructive
};

// --- formatting helpers shared by every Library cell. -----------------------

/** Relative time for a created/updated timestamp (em dash for unset). */
export function relTime(iso?: string): string {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "—";
  const diff = Date.now() - t;
  const m = Math.round(diff / 60_000);
  const h = Math.round(diff / 3_600_000);
  const d = Math.round(diff / 86_400_000);
  if (d >= 1) return `${d}d ago`;
  if (h >= 1) return `${h}h ago`;
  return `${Math.max(m, 1)}m ago`;
}

/** Human byte size (binary units), em dash for unset/zero. */
export function formatBytes(bytes?: number): string {
  if (bytes === undefined || bytes <= 0) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i += 1;
  }
  const out = i === 0 ? String(v) : v.toFixed(v < 10 ? 1 : 0);
  return `${out} ${units[i]}`;
}
