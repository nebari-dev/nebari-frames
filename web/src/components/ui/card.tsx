import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

/**
 * Card surface using the Nebari `card` design tokens. A plain `div` container
 * with a rounded border; extend at the call site via `className`.
 */
export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      data-slot="card"
      className={cn(
        'rounded-lg border border-border bg-card text-card-foreground shadow-xs',
        className,
      )}
      {...props}
    />
  );
}
