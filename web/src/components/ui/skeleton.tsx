import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

/**
 * Loading placeholder. A pulsing muted block; size it via `className`
 * (e.g. `<Skeleton className="h-4 w-32" />`).
 */
export function Skeleton({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="skeleton"
      className={cn('animate-pulse rounded-md bg-muted', className)}
      {...props}
    />
  );
}
