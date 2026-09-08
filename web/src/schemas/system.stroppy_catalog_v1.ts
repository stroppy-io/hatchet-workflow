// GENERATED from schemapb schema system.stroppy_catalog@1 — do not edit.
// Platform catalog of stroppy versions, their images, protocols and workload scripts.

/** object  */
export interface SystemStroppyCatalog1ItemItemItem {
  /** Step id. */
  id: string;
  /** Title. */
  title?: string;
  /** Phase. Where the step sits in a run: preparing, measuring, cleaning up. */
  phase?: "bootstrap" | "workload" | "teardown";
}

/** object  */
export interface SystemStroppyCatalog1ItemItem {
  /** Script id. Workload script id as passed to `stroppy run`. */
  id: string;
  /** Title. */
  title: string;
  /** Description. */
  description?: string;
  /** Protocols. Protocols this script runs on; procs variants are pg/mysql only. */
  protocols?: Array<"pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop">;
  /** Steps. Steps the script declares; the segment form filters on them. */
  steps: Array<SystemStroppyCatalog1ItemItemItem>;
  /** Params schema. schemapb id of script-specific params, when the script has its own form. */
  params_schema?: string;
}

/** object  */
export interface SystemStroppyCatalog1Item {
  /** Version. Stroppy release version. */
  version: string;
  /** Image. Docker image of that release, e.g. ghcr.io/stroppy-io/stroppy:5.1.2. */
  image: string;
  /** Default. The version a new workload starts with; exactly one version carries it. */
  default?: boolean;
  /** Deprecated. Still runnable, hidden from the picker for new workloads. */
  deprecated?: boolean;
  /** Protocols. Database protocols this build has drivers for. */
  protocols: Array<"pg" | "mysql" | "picodata" | "ydb_grpc" | "ydb_grpcs" | "cockroach" | "noop">;
  /** Scripts. Workload scripts this build ships. */
  scripts: Array<SystemStroppyCatalog1ItemItem>;
}

/** root */
export interface SystemStroppyCatalog1 {
  /** Source. Where the catalog came from: hand-written, read from a stroppy release, or probed. */
  source?: "static" | "release" | "probe";
  /** Versions. Every stroppy build the platform offers. */
  versions: Array<SystemStroppyCatalog1Item>;
}

