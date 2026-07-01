import { forwardRef, type ComponentProps } from "react";
import { cn } from "@/lib/utils";

// Native <select> styled to the design tokens. Sufficient for the version
// picker; a richer combobox is YAGNI for MVP.
export const Select = forwardRef<HTMLSelectElement, ComponentProps<"select">>(
  ({ className, ...props }, ref) => (
    <select
      ref={ref}
      data-slot="select"
      className={cn(
        "flex h-9 w-full rounded-md border border-input bg-background px-2 text-sm shadow-xs outline-none transition-colors",
        "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  ),
);
Select.displayName = "Select";
