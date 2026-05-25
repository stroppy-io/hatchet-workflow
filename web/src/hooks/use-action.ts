import { useCallback, useState } from "react";

import { errorMessage } from "@/lib/errors";

type ActionState = {
  loading: boolean;
  error: string | null;
  done: boolean;
};

// Mutation counterpart to useListQuery: wraps a write RPC with loading/error/
// done state. `run` resolves to the RPC result, or undefined when it failed
// (the error is captured in state), so callers can branch without try/catch.
export function useAction<TArgs extends unknown[], TResult>(
  fn: (...args: TArgs) => Promise<TResult>,
) {
  const [state, setState] = useState<ActionState>({
    loading: false,
    error: null,
    done: false,
  });

  const run = useCallback(
    async (...args: TArgs): Promise<TResult | undefined> => {
      setState({ loading: true, error: null, done: false });
      try {
        const result = await fn(...args);
        setState({ loading: false, error: null, done: true });
        return result;
      } catch (error) {
        setState({ loading: false, error: errorMessage(error), done: false });
        return undefined;
      }
    },
    [fn],
  );

  const reset = useCallback(
    () => setState({ loading: false, error: null, done: false }),
    [],
  );

  return { ...state, run, reset };
}
