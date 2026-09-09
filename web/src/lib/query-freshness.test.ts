import { describe, expect, it } from "vitest";
import { ConnectError, Code } from "@connectrpc/connect";
import { freshData } from "./query-freshness";

describe("freshData", () => {
  const row = { title: "Our House Style" };

  it("returns data a fetch delivered during this mount", () => {
    expect(freshData({ data: row, error: null, isFetchedAfterMount: true })).toBe(row);
  });

  it("withholds a cached row no fetch has confirmed yet", () => {
    // The state on the first render after mount: cached data present, the
    // forced refetch still in flight.
    expect(freshData({ data: row, error: null, isFetchedAfterMount: false })).toBeUndefined();
  });

  it("withholds a cached row when the fetch that ran failed", () => {
    // query-core counts a failed fetch in isFetchedAfterMount and leaves the
    // cached data in place, so this state is indistinguishable from success
    // without checking `error`.
    expect(
      freshData({
        data: row,
        error: new ConnectError("connection lost", Code.Unavailable),
        isFetchedAfterMount: true,
      }),
    ).toBeUndefined();
  });

  it("has nothing to give when a fresh fetch carried nothing", () => {
    expect(freshData({ data: undefined, error: null, isFetchedAfterMount: true })).toBeUndefined();
  });
});
