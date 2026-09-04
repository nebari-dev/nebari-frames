import type { ComponentProps } from "react";
import { TableHeader } from "@/components/ui/table";
import { cn } from "@/lib/utils";

/**
 * The app's table header treatment: the design system's `TableHeader` on a
 * muted (gray) band instead of the page background, which is how Frames
 * distinguishes the header row from the body across every table.
 *
 * The gray has to land on the `th` cells, not the `thead` — the registry's
 * `TableHead` paints each cell `bg-background`, which would otherwise sit on
 * top of a background set here. Overriding at the call site like this keeps
 * `ui/table.tsx` upstream-managed and safe to re-run `shadcn add` over.
 */
export function DataTableHeader({
  className,
  ...props
}: ComponentProps<typeof TableHeader>) {
  return <TableHeader className={cn("[&_th]:bg-muted", className)} {...props} />;
}
