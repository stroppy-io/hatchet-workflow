// Shared proto-enum <-> UI-label converters. The library tables and run views
// all speak the flat string unions from components/library-table/labels; the
// wire speaks numeric proto enums. Keep the mapping in one place.

import { Database_Kind } from "@/lib/proto/cloud/v1/domain/database_pb";
import type { DbKind } from "@/components/library-table/labels";

const DB_KIND_TO_LABEL: Record<Database_Kind, DbKind> = {
  [Database_Kind.UNSPECIFIED]: "",
  [Database_Kind.POSTGRES]: "postgres",
  [Database_Kind.MYSQL]: "mysql",
  [Database_Kind.MARIADB]: "mariadb",
  [Database_Kind.YDB]: "ydb",
  [Database_Kind.YDB_MANAGED]: "ydb_managed",
  [Database_Kind.COCKROACH]: "cockroach",
  [Database_Kind.PICODATA]: "picodata",
  [Database_Kind.ORIOLEDB]: "orioledb",
  [Database_Kind.EXTERNAL]: "external",
  [Database_Kind.NOOP]: "noop",
  [Database_Kind.PG_NOOP]: "pgnoop",
};

const LABEL_TO_DB_KIND: Record<Exclude<DbKind, "">, Database_Kind> = {
  postgres: Database_Kind.POSTGRES,
  mysql: Database_Kind.MYSQL,
  mariadb: Database_Kind.MARIADB,
  ydb: Database_Kind.YDB,
  ydb_managed: Database_Kind.YDB_MANAGED,
  cockroach: Database_Kind.COCKROACH,
  picodata: Database_Kind.PICODATA,
  orioledb: Database_Kind.ORIOLEDB,
  external: Database_Kind.EXTERNAL,
  noop: Database_Kind.NOOP,
  pgnoop: Database_Kind.PG_NOOP,
};

/** numeric proto enum -> UI label ("" for UNSPECIFIED/unknown). */
export function dbKindLabel(k: Database_Kind | undefined): DbKind {
  return k != null ? (DB_KIND_TO_LABEL[k] ?? "") : "";
}

/** Map the JSON-string enum form (e.g. "KIND_POSTGRES") onto the UI label. */
export function dbKindLabelFromJson(s: string | undefined): DbKind {
  switch (s) {
    case "KIND_POSTGRES":
      return "postgres";
    case "KIND_MYSQL":
      return "mysql";
    case "KIND_MARIADB":
      return "mariadb";
    case "KIND_YDB":
      return "ydb";
    case "KIND_YDB_MANAGED":
      return "ydb_managed";
    case "KIND_COCKROACH":
      return "cockroach";
    case "KIND_PICODATA":
      return "picodata";
    case "KIND_ORIOLEDB":
      return "orioledb";
    case "KIND_EXTERNAL":
      return "external";
    case "KIND_NOOP":
      return "noop";
    case "KIND_PG_NOOP":
      return "pgnoop";
    default:
      return "";
  }
}

/** UI label -> numeric proto enum (UNSPECIFIED for ""/unknown). */
export function dbKindProto(label: DbKind): Database_Kind {
  return label ? (LABEL_TO_DB_KIND[label] ?? Database_Kind.UNSPECIFIED) : Database_Kind.UNSPECIFIED;
}

export type DeployProvider = "" | "docker" | "yandex";

/** deployment.Provider JSON-string enum -> UI label. */
export function providerLabelFromJson(s: string | undefined): DeployProvider {
  return s === "PROVIDER_DOCKER" ? "docker" : s === "PROVIDER_YANDEX" ? "yandex" : "";
}

import { Provider as ProviderEnum } from "@/lib/proto/cloud/v1/deployment/provider_pb";

/** UI label -> numeric deployment.Provider enum. */
export function providerProto(label: DeployProvider): ProviderEnum {
  return label === "docker"
    ? ProviderEnum.DOCKER
    : label === "yandex"
      ? ProviderEnum.YANDEX
      : ProviderEnum.UNSPECIFIED;
}

/** common.Trigger JSON-string enum -> lower-cased label. */
export function triggerLabelFromJson(
  s: string | undefined,
): "" | "manual" | "cron" | "api" {
  switch (s) {
    case "TRIGGER_MANUAL":
      return "manual";
    case "TRIGGER_CRON":
      return "cron";
    case "TRIGGER_API":
      return "api";
    default:
      return "";
  }
}

import { Status } from "@/lib/proto/cloud/v1/common/status_pb";
import { Trigger } from "@/lib/proto/cloud/v1/common/trigger_pb";
import { Workload_Protocol } from "@/lib/proto/cloud/v1/domain/workload_pb";
import type { RunStatus } from "@/services/dashboard";

/** RunStatus UI label -> numeric common.Status enum (for list filters). */
export function statusProto(s: RunStatus): Status {
  switch (s) {
    case "pending":
      return Status.PENDING;
    case "running":
      return Status.RUNNING;
    case "cancelling":
      return Status.CANCELLING;
    case "completed":
      return Status.COMPLETED;
    case "failed":
      return Status.FAILED;
    case "cancelled":
      return Status.CANCELLED;
    default:
      return Status.UNSPECIFIED;
  }
}

/** UI label -> numeric common.Trigger enum. */
export function triggerProto(t: "" | "manual" | "cron" | "api"): Trigger {
  switch (t) {
    case "manual":
      return Trigger.MANUAL;
    case "cron":
      return Trigger.CRON;
    case "api":
      return Trigger.API;
    default:
      return Trigger.UNSPECIFIED;
  }
}

export type ProtocolLabel =
  | ""
  | "pg"
  | "mysql"
  | "picodata"
  | "ydb_grpc"
  | "ydb_grpcs"
  | "cockroach"
  | "noop";

/** domain.Workload.Protocol JSON-string enum -> lower-cased label. */
export function protocolLabelFromJson(s: string | undefined): ProtocolLabel {
  switch (s) {
    case "PROTOCOL_PG":
      return "pg";
    case "PROTOCOL_MYSQL":
      return "mysql";
    case "PROTOCOL_PICODATA":
      return "picodata";
    case "PROTOCOL_YDB_GRPC":
      return "ydb_grpc";
    case "PROTOCOL_YDB_GRPCS":
      return "ydb_grpcs";
    case "PROTOCOL_COCKROACH":
      return "cockroach";
    case "PROTOCOL_NOOP":
      return "noop";
    default:
      return "";
  }
}

/** UI label -> numeric domain.Workload.Protocol enum. */
export function protocolProto(p: ProtocolLabel): Workload_Protocol {
  switch (p) {
    case "pg":
      return Workload_Protocol.PG;
    case "mysql":
      return Workload_Protocol.MYSQL;
    case "picodata":
      return Workload_Protocol.PICODATA;
    case "ydb_grpc":
      return Workload_Protocol.YDB_GRPC;
    case "ydb_grpcs":
      return Workload_Protocol.YDB_GRPCS;
    case "cockroach":
      return Workload_Protocol.COCKROACH;
    case "noop":
      return Workload_Protocol.NOOP;
    default:
      return Workload_Protocol.UNSPECIFIED;
  }
}
