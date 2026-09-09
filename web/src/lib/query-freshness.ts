import type { ConnectError } from "@connectrpc/connect";

/**
 * The data a query fetched during *this* mount, or undefined.
 *
 * A form that seeds once from server data and then belongs to the author needs
 * exactly this, and none of the three conditions is optional:
 *
 * - React Query returns a cached row synchronously when a query mounts and
 *   refetches behind it, so `data` alone can be a copy left by an earlier open
 *   of the same form, or by another page reading the same key.
 * - A *failed* fetch also satisfies `isFetchedAfterMount` (it is
 *   `dataUpdateCount > initial || errorUpdateCount > initial`), and the error
 *   reducer keeps the cached data rather than clearing it - so "a fetch
 *   completed" is not "fresh data arrived".
 * - `error` clears on the next success, so a form gated on this seeds as soon
 *   as a retry works.
 *
 * Pair it with `refetchOnMount: "always"` on the query itself: without that,
 * whether a fetch happens at all depends on staleness, which is configured
 * globally and elsewhere (`web/src/lib/query.ts`).
 */
export function freshData<T>(query: {
  data?: T;
  error: ConnectError | null;
  isFetchedAfterMount: boolean;
}): T | undefined {
  return query.isFetchedAfterMount && !query.error ? query.data : undefined;
}
