import { useEffect, useEffectEvent, useState } from "react";

export type QueryState<T> =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "success"; data: T };

export function useApiQuery<T>(
  fetcher: () => Promise<T>,
  key: unknown[] = [],
): QueryState<T> {
  const [state, setState] = useState<QueryState<T>>({ status: "loading" });
  const runFetcher = useEffectEvent(fetcher);

  // value identity for the key, same idea as TanStack's hashKey
  const serialisedKey = JSON.stringify(key);

  useEffect(() => {
    let cancelled = false;
    setState({ status: "loading" });

    runFetcher().then(
      (data) => {
        if (!cancelled) setState({ status: "success", data });
      },
      (err: unknown) => {
        if (!cancelled) setState({ status: "error", message: String(err) });
      },
    );
    return () => {
      cancelled = true;
    };
  }, [serialisedKey]);

  return state;
}
