import { useCallback, useMemo, useState } from "react";

import type { PageInfo } from "@/lib/proto/cloud/v1/models/common_pb.ts";

export function useCursorPagination(pageSizeDefault = 25) {
  const [pageSize, setPageSizeState] = useState(pageSizeDefault);
  const [tokens, setTokens] = useState<string[]>([""]);
  const [index, setIndex] = useState(0);

  const token = tokens[index] ?? "";

  const page = useMemo(() => ({ size: pageSize, token }), [pageSize, token]);

  const reset = useCallback(() => {
    setTokens([""]);
    setIndex(0);
  }, []);

  const setPageSize = useCallback(
    (size: number) => {
      setPageSizeState(size);
      reset();
    },
    [reset],
  );

  const next = useCallback(
    (pageInfo?: PageInfo) => {
      if (!pageInfo?.nextToken) return;
      setTokens((current) => {
        const nextTokens = current.slice(0, index + 1);
        nextTokens[index + 1] = pageInfo.nextToken;
        return nextTokens;
      });
      setIndex((current) => current + 1);
    },
    [index],
  );

  const previous = useCallback(() => {
    setIndex((current) => Math.max(0, current - 1));
  }, []);

  // Jump to a VISITED page (cursor pagination can't seek forward to unvisited
  // pages — only pages already loaded have a token).
  const goTo = useCallback(
    (target: number) => {
      setIndex(Math.max(0, Math.min(target, tokens.length - 1)));
    },
    [tokens.length],
  );

  return {
    canPrevious: index > 0,
    index,
    visitedCount: tokens.length,
    goTo,
    next,
    page,
    pageSize,
    previous,
    reset,
    setPageSize,
  };
}
