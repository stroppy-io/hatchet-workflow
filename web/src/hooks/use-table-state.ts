import { useCallback, useEffect, useState } from "react";

import { useCursorPagination } from "@/hooks/use-cursor-pagination";
import { useOffsetPagination } from "@/hooks/use-offset-pagination";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";

type ListState = {
  order: SortOrder;
  search: string;
  sortField: number;
};

// Shared list-table state: search (debounced) + typed sort + pagination, plus
// `reload`/`reloadKey` so mutating pages can force a refetch (append reloadKey to
// the useListQuery deps). Reused by every table page. `mode` selects cursor
// (default, cheap forward scan) or offset (random page access + total count).
export function useTableState(
  defaultSortField: number,
  extraResetDeps: unknown[] = [],
  mode: "cursor" | "offset" = "cursor",
) {
  const [state, setState] = useState<ListState>({
    order: SortOrder.DESC,
    search: "",
    sortField: defaultSortField,
  });
  const [reloadKey, setReloadKey] = useState(0);
  const debouncedSearch = useDebouncedValue(state.search);
  const cursor = useCursorPagination();
  const offset = useOffsetPagination();
  const pagination = mode === "offset" ? offset : cursor;

  const setSearch = useCallback(
    (search: string) => {
      setState((current) => ({ ...current, search }));
      pagination.reset();
    },
    [pagination],
  );

  const setSort = useCallback(
    (sortField: number, order: SortOrder) => {
      setState((current) => ({ ...current, sortField, order }));
      pagination.reset();
    },
    [pagination],
  );

  const reload = useCallback(() => setReloadKey((key) => key + 1), []);

  useEffect(() => {
    pagination.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, extraResetDeps);

  return { ...state, debouncedSearch, pagination, setSearch, setSort, reload, reloadKey };
}
