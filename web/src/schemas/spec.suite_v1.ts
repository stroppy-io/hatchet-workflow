// GENERATED from schemapb schema spec.suite@1 — do not edit.
// SuiteSpec: the cells of a matrix run and how many of them run in parallel.

/** object  */
export interface SpecSuite1Item {
  /** Cell id. Stable cell id; the child run id is derived from parent + cell id. */
  id: string;
  /** RunSpec. A complete spec.run@1 value; the suite hands it to a child run unread. */
  run_spec: unknown;
}

/** object defaults */
export interface SpecSuite1Defaults {
  /** Continue on failure. Keep running the remaining cells after one fails. */
  continue_on_failure?: boolean;
  /** Labels. Labels stamped on every child run. */
  labels?: Record<string, string>;
}

/** root */
export interface SpecSuite1 {
  /** Suite run id. Suite run id minted by the server; the parent Graphene run id. */
  suite_run_id: string;
  /** Tenant. Tenant slug; every child run lives in the same namespace. */
  tenant: string;
  /** Cells. The resolved matrix: one child run per cell. */
  cells: Array<SpecSuite1Item>;
  /** Concurrency. How many cells run at the same time. */
  concurrency?: number | string;
  /** Defaults. Settings shared by every cell. */
  defaults?: SpecSuite1Defaults;
}

