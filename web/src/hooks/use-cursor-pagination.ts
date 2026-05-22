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

  return {
    canPrevious: index > 0,
    index,
    next,
    page,
    pageSize,
    previous,
    reset,
    setPageSize,
  };
}
