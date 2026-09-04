import { act } from "react";
import { renderHook } from "@testing-library/react";
import { expect, it } from "vitest";
import { usePagination } from "./use-pagination";

const range = (n: number) => Array.from({ length: n }, (_, i) => i + 1);

it("pages ten rows at a time", () => {
  const { result } = renderHook(() => usePagination(range(23)));

  expect(result.current.pageItems).toEqual(range(10));
  expect(result.current.pageCount).toBe(3);
  expect(result.current.total).toBe(23);
  expect([result.current.rangeStart, result.current.rangeEnd]).toEqual([1, 10]);

  act(() => result.current.setPage(3));
  expect(result.current.pageItems).toEqual([21, 22, 23]);
  expect([result.current.rangeStart, result.current.rangeEnd]).toEqual([21, 23]);
});

it("reports a single page when everything fits", () => {
  const { result } = renderHook(() => usePagination(range(10)));
  expect(result.current.pageCount).toBe(1);
  expect(result.current.pageItems).toHaveLength(10);
});

it("clamps the page when the list shrinks underneath it", () => {
  // Searching or deleting can drop the row count while a later page is open;
  // the visible page has to stay valid rather than render empty.
  const { result, rerender } = renderHook(({ items }) => usePagination(items), {
    initialProps: { items: range(30) },
  });

  act(() => result.current.setPage(3));
  expect(result.current.page).toBe(3);

  rerender({ items: range(12) });
  expect(result.current.page).toBe(2);
  expect(result.current.pageItems).toEqual([11, 12]);
});

it("handles an empty list without a zero-th page", () => {
  const { result } = renderHook(() => usePagination<number>([]));
  expect(result.current.page).toBe(1);
  expect(result.current.pageCount).toBe(1);
  expect(result.current.pageItems).toEqual([]);
  expect(result.current.rangeStart).toBe(0);
});
