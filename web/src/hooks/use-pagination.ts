import { useState } from "react";

/** Rows shown per page across the app's tables. */
export const DEFAULT_PAGE_SIZE = 10;

export type Pagination<T> = {
  /** The current page, 1-based and already clamped to the row count. */
  page: number;
  setPage: (page: number) => void;
  /** The rows to render for the current page. */
  pageItems: T[];
  pageCount: number;
  /** Total rows across all pages. */
  total: number;
  /** 1-based index of the first row shown, or 0 when there are none. */
  rangeStart: number;
  /** 1-based index of the last row shown. */
  rangeEnd: number;
};

/**
 * Client-side paging over an in-memory list.
 *
 * The page is clamped on read rather than reset in an effect: the lists here
 * shrink underneath the control all the time — searching the catalog, deleting
 * a frame, removing a member — and clamping keeps the last page valid without a
 * render-then-correct pass that would briefly show an empty table.
 */
export function usePagination<T>(items: T[], pageSize = DEFAULT_PAGE_SIZE): Pagination<T> {
  const [page, setPage] = useState(1);

  const total = items.length;
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const safePage = Math.min(Math.max(page, 1), pageCount);
  const start = (safePage - 1) * pageSize;
  const pageItems = items.slice(start, start + pageSize);

  return {
    page: safePage,
    setPage,
    pageItems,
    pageCount,
    total,
    rangeStart: total === 0 ? 0 : start + 1,
    rangeEnd: start + pageItems.length,
  };
}
