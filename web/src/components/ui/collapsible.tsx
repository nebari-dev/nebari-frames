import { Collapsible as CollapsiblePrimitive } from '@base-ui-components/react/collapsible';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/utils';

/**
 * Collapsible disclosure built on Base UI's Collapsible (the same primitive
 * library the rest of the Nebari design system uses). `Collapsible` is the
 * root (accepts `defaultOpen` / `open` / `onOpenChange`), `CollapsibleTrigger`
 * is the toggle button, and `CollapsibleContent` is the panel that animates
 * open and closed.
 */
function Collapsible({
  className,
  ...props
}: ComponentProps<typeof CollapsiblePrimitive.Root>) {
  return (
    <CollapsiblePrimitive.Root
      data-slot="collapsible"
      className={cn(className)}
      {...props}
    />
  );
}

function CollapsibleTrigger({
  className,
  ...props
}: ComponentProps<typeof CollapsiblePrimitive.Trigger>) {
  return (
    <CollapsiblePrimitive.Trigger
      data-slot="collapsible-trigger"
      className={cn(
        'flex w-full items-center justify-between outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
        className,
      )}
      {...props}
    />
  );
}

function CollapsibleContent({
  className,
  ...props
}: ComponentProps<typeof CollapsiblePrimitive.Panel>) {
  return (
    <CollapsiblePrimitive.Panel
      data-slot="collapsible-content"
      className={cn(className)}
      {...props}
    />
  );
}

export { Collapsible, CollapsibleTrigger, CollapsibleContent };
