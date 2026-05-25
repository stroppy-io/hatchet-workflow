import { useCallback, useMemo, useState } from "react";

import type { PageInfo } from "@/lib/proto/cloud/v1/models/common_pb.ts";

// Offset pagination: random page access (jump to any page) backed by Page.offset
// + PageInfo.total. Mirrors useCursorPagination's surface so useTableState can use
// either. `next` accepts an optional PageInfo for call-site symmetry with cursor.
export function useOffsetPagination(pageSizeDefault = 25) {
  const [pageSize, setPageSizeState] = useState(pageSizeDefault);
  const [index, setIndex] = useState(0);

  const page = useMemo(() => ({ size: pageSize, offset: BigInt(index * pageSize) }), [pageSize, index]);

  const reset = useCallback(() => setIndex(0), []);

  const setPageSize = useCallback((size: number) => {
    setPageSizeState(size);
    setIndex(0);
  }, []);

  const goTo = useCallback((target: number) => setIndex(Math.max(0, target)), []);
  const next = useCallback((_pageInfo?: PageInfo) => setIndex((current) => current + 1), []);
  const previous = useCallback(() => setIndex((current) => Math.max(0, current - 1)), []);

  return { index, page, pageSize, goTo, next, previous, reset, setPageSize };
}
