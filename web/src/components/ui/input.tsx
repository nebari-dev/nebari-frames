import { forwardRef, type ComponentProps } from 'react';
import { cn } from '@/lib/utils';

/**
 * Styled text input matching the Nebari design tokens (border, focus ring,
 * muted placeholder). Standard shadcn-style `<input>`; extend via `className`.
 */
export const Input = forwardRef<HTMLInputElement, ComponentProps<'input'>>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        ref={ref}
        type={type}
        data-slot="input"
        className={cn(
          'flex h-8 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-xs outline-none transition-colors',
          'placeholder:text-muted-foreground',
          'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
          'disabled:cursor-not-allowed disabled:opacity-50',
          'file:border-0 file:bg-transparent file:text-sm file:font-medium',
          className,
        )}
        {...props}
      />
    );
  },
);
Input.displayName = 'Input';
