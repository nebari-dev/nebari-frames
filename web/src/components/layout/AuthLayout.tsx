import type { ReactNode } from "react";
import { Card } from "@/components/ui/card";
import { useTheme } from "@/hooks/theme-provider";
import { brandLogo } from "@/lib/branding";
import { cn } from "@/lib/utils";

type AuthLayoutProps = {
  title: string;
  description?: ReactNode;
  children?: ReactNode;
  className?: string;
};

/**
 * Centered, branded shell for pre-auth screens (login, callback, no-access) so
 * they match the rest of the Nebari ecosystem: logo wordmark over a single
 * card with a title, optional description, and actions. The wordmark honours the
 * deployment's branded logo when one is configured.
 */
export function AuthLayout({ title, description, children, className }: AuthLayoutProps) {
  const { isDarkMode } = useTheme();
  const logo = brandLogo(isDarkMode);
  return (
    <div className="grid min-h-screen place-items-center bg-canvas px-4">
      <div className="flex w-full max-w-sm flex-col items-center gap-6 motion-safe:animate-slide-up-fade">
        <div className="flex items-center gap-2.5">
          <img src={logo.src} alt={logo.alt} className="h-8 w-auto" />
          <span className="border-l border-border pl-2.5 text-lg font-semibold text-foreground">
            Frames
          </span>
        </div>
        <Card className={cn("w-full space-y-4 p-6 text-center", className)}>
          <div className="space-y-2">
            <h1 className="text-xl font-semibold text-foreground">{title}</h1>
            {description && (
              <p className="text-sm leading-relaxed text-muted-foreground">{description}</p>
            )}
          </div>
          {children}
        </Card>
      </div>
    </div>
  );
}
