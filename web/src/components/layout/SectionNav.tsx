import { NavLink } from "react-router";
import { tabsListVariants, tabsTabVariants } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";

export type SectionNavItem = {
  /** Route to navigate to. */
  to: string;
  label: string;
};

/**
 * In-page section navigation that looks like the design system's underline
 * tabs but is built from real links.
 *
 * These "tabs" change the route rather than swapping a panel, so `Tabs` is the
 * wrong primitive: Base UI renders `role="tab"` inside a `role="tablist"`,
 * which promises an associated `tabpanel` that does not exist here, and a
 * button-based tab cannot be cmd-clicked or opened in a new tab. A `<nav>` of
 * links is the honest markup for "these are separate pages", so this composes
 * the exported `tabsListVariants` / `tabsTabVariants` class functions — at
 * their default `pill` variant — to keep the look identical to the real
 * component.
 */
export function SectionNav({
  items,
  ariaLabel,
  className,
}: {
  items: SectionNavItem[];
  ariaLabel: string;
  className?: string;
}) {
  return (
    <nav
      aria-label={ariaLabel}
      className={cn(
        tabsListVariants(),
        // The registry paints the pill track `bg-background`, which the design
        // system intends as a faint gray behind the `bg-card` pills. This app
        // overrides `--background` to pure white (see styles.css), which would
        // flatten the track into the pills, so the track takes `muted` to keep
        // the intended contrast in both themes.
        "bg-muted",
        className,
      )}
    >
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end
          className={({ isActive }) =>
            cn(
              tabsTabVariants(),
              // `data-[active]` is set by Base UI's Tabs, which these links are
              // not inside, so the selected pill's border and lift are applied
              // from the router's own active state instead.
              isActive &&
                "border-border-strong text-foreground shadow-[0_1px_1.5px_rgb(0_0_0_/_0.1)]",
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  );
}
