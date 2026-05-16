import type { Timestamp } from "@bufbuild/protobuf/wkt";

/** Convert a proto Timestamp to ISO string. Returns "" if undefined. */
export function protoTsToISO(ts?: Timestamp): string {
  if (!ts) return "";
  return new Date(
    Number(ts.seconds) * 1000 + Math.floor(ts.nanos / 1_000_000)
  ).toISOString();
}
