import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Pagination } from "@/hooks/use-pagination";

/**
 * Page control for the app's tables: a row count on the left, previous/next on
 * the right. Always rendered, even for a single page, so the row count stays
 * visible and the table's footprint does not shift as rows are added.
 *
 * The Nebari registry has no pagination item yet, so this is app-owned and
 * built from the design system's `Button` and semantic tokens.
 */
export function TablePagination({
  pagination: { page, setPage, pageCount, total, rangeStart, rangeEnd },
  label,
}: {
  pagination: Pagination<unknown>;
  /** Plural noun for the rows, e.g. "frames" — used in the count and a11y name. */
  label: string;
}) {
  return (
    <nav
      aria-label={`${label} pagination`}
      className="flex flex-wrap items-center justify-between gap-3"
    >
      <p aria-live="polite" className="text-sm text-muted-foreground">
        Showing {rangeStart}–{rangeEnd} of {total} {label}
      </p>
      <div className="flex items-center gap-2">
        <span className="text-sm text-muted-foreground tabular-nums">
          Page {page} of {pageCount}
        </span>
        {/* Icon-only: the adjacent "Page x of y" already says what these do,
            so the label lives in `aria-label` and the tooltip. */}
        <Button
          variant="outline"
          size="icon-sm"
          aria-label="Previous page"
          title="Previous page"
          disabled={page <= 1}
          onClick={() => setPage(page - 1)}
        >
          <ChevronLeft />
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          aria-label="Next page"
          title="Next page"
          disabled={page >= pageCount}
          onClick={() => setPage(page + 1)}
        >
          <ChevronRight />
        </Button>
      </div>
    </nav>
  );
}
