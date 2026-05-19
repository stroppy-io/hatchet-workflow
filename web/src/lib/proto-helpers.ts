import type { Timestamp } from "@bufbuild/protobuf/wkt";

/**
 * placeholderId — 26-char zero string used by Create* RPCs.
 *
 * All entity IDs are 26-char ULIDs (validate.string.len = 26). On Create the
 * server unconditionally overrides the id with a fresh ULID, but the proto
 * validator still requires Id.value to be present and exactly 26 chars. The
 * client sends "0".repeat(26) as a placeholder so validation passes and the
 * server assigns the real id.
 */
export const PLACEHOLDER_ID = "0".repeat(26);
export function placeholderId(): { value: string } {
  return { value: PLACEHOLDER_ID };
}

/** Convert a proto Timestamp to ISO string. Returns "" if undefined. */
export function protoTsToISO(ts?: Timestamp): string {
  if (!ts) return "";
  return new Date(
    Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000)
  ).toISOString();
}
