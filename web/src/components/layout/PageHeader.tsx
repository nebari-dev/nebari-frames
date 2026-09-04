import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

type PageHeaderProps = {
  title: ReactNode;
  description?: ReactNode;
  /** Right-aligned actions, kept on the title's baseline row. */
  actions?: ReactNode;
  className?: string;
};

/**
 * The one page-level heading treatment: a `text-2xl` title over a `text-sm`
 * muted description. Pages used to inline these classes, which let the Connect
 * pages drift (no `tracking-tight`, and a description that inherited the 16px
 * body size instead of 14px). Composing this instead keeps every page's header
 * on the same type scale.
 */
export function PageHeader({ title, description, actions, className }: PageHeaderProps) {
  return (
    <div className={cn("flex items-start justify-between gap-4", className)}>
      <div className="min-w-0 space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">{title}</h1>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-3">{actions}</div>}
    </div>
  );
}

/**
 * Section-level heading for a page that already has a `PageHeader` above it —
 * one step down the scale (`text-xl`), used by the admin subpages.
 */
export function SectionHeader({ title, description, actions, className }: PageHeaderProps) {
  return (
    <div className={cn("flex items-start justify-between gap-4", className)}>
      <div className="min-w-0 space-y-1">
        <h2 className="text-xl font-semibold tracking-tight text-foreground">{title}</h2>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-3">{actions}</div>}
    </div>
  );
}
