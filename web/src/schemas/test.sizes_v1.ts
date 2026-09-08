// GENERATED from schemapb schema test.sizes@1 — do not edit.
// Per-role machine sizing of a test: a T-shirt size and an optional disk override.

/** object disk */
export interface TestSizes1RolesValueDisk {
  /** Disk type. Provider disk type id; empty keeps the size table's default. */
  type?: string;
  /** Size. Data disk size; empty keeps the size table's default. [GB] */
  gb?: number | string;
}

/** map value roles */
export interface TestSizes1RolesValue {
  /** Size. T-shirt size; the platform size table turns it into a machine. */
  size: "XS" | "S" | "M" | "L" | "XL";
  /** Disk. Overrides the disk the size table would pick; absent means take the default. */
  disk?: TestSizes1RolesValueDisk | null;
}

/** root */
export interface TestSizes1 {
  /** Roles. Keys are topology roles (db, db-replica, proxy, runner, coordinator); the server validates them against the database topology. */
  roles: Record<string, TestSizes1RolesValue>;
}

