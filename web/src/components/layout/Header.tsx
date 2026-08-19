import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { ChevronDown, LogOut, Monitor, Moon, Sun, User } from "lucide-react";
import type { ReactNode } from "react";
import { Link, useMatch } from "react-router";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  MenuBarActions,
  MenuBarBrand,
  MenuBarNav,
  NavigationMenu,
  NavLink,
} from "@/components/ui/navigation-menu";
import { isThemeMode, type ThemeMode } from "@/hooks/use-theme-preference";
import { useTheme } from "@/hooks/theme-provider";
import { useAuth } from "@/lib/auth/useAuth";
import { brandLogo } from "@/lib/branding";
import { cn } from "@/lib/utils";

/**
 * Registry NavLink driven by the router: `useMatch` supplies the active state
 * the design system uses for the underline, and the anchor is swapped for a
 * router `Link` through Base UI's `render` prop. Hover uses the header token so
 * it reads the same as the account trigger sitting beside it.
 */
function HeaderNavLink({
  to,
  end = false,
  children,
}: {
  to: string;
  end?: boolean;
  children: ReactNode;
}): ReactNode {
  const active = useMatch({ path: to, end }) !== null;

  return (
    <NavLink
      active={active}
      className="text-header-foreground hover:bg-header-action-hover"
      render={<Link to={to} />}
    >
      {children}
    </NavLink>
  );
}

function initialsFor(value?: string | null): string {
  if (!value) return "";
  const base = value.includes("@") ? (value.split("@")[0] ?? value) : value;
  const parts = base.split(/[\s._-]+/).filter(Boolean);
  if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
  return base.slice(0, 2).toUpperCase();
}

export function Header() {
  const { status, user, login, logout } = useAuth();
  const { data: me } = useQuery(FrameService.method.getMe, {});
  const { themeMode, isDarkMode, setThemeMode } = useTheme();

  const isAuthenticated = status === "authenticated";
  const displayName = user?.profile?.name ?? me?.email ?? user?.profile?.email ?? "Account";
  const email = me?.email ?? user?.profile?.email ?? null;
  const initials = initialsFor(displayName || email);
  // Branded logo when the deployment configures one (loaded before mount by
  // main.tsx), otherwise the bundled Nebari wordmark.
  const logo = brandLogo(isDarkMode);

  return (
    <NavigationMenu className="h-14 justify-between border-header-border bg-header-background pl-4 text-header-foreground">
      <div className="flex items-center gap-6">
        <MenuBarBrand href="/" aria-label="Go to homepage">
          <img src={logo.src} alt={logo.alt} className="h-8 w-auto" />
        </MenuBarBrand>

        <MenuBarNav className="flex-none">
          <HeaderNavLink to="/" end>
            Frames
          </HeaderNavLink>
          {me?.role === "admin" && <HeaderNavLink to="/admin">Admin</HeaderNavLink>}
          <HeaderNavLink to="/connect">Connect</HeaderNavLink>
        </MenuBarNav>
      </div>

      <MenuBarActions className="gap-2">
        {isAuthenticated ? (
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger
              variant="ghost"
              aria-label="Account menu"
              className="h-auto px-2.5 py-1 hover:bg-header-action-hover hover:no-underline focus-visible:ring-offset-0 active:bg-header-action-hover data-[popup-open]:bg-header-action-hover data-[popup-open]:no-underline"
            >
              <Avatar>
                <AvatarFallback className="bg-primary font-semibold text-primary-foreground">
                  {initials || <User className="size-4" />}
                </AvatarFallback>
              </Avatar>

              <span className="hidden max-w-[18ch] truncate sm:inline">{displayName}</span>

              <ChevronDown />
            </DropdownMenuTrigger>

            <DropdownMenuPortal>
              <DropdownMenuContent align="end" className="w-[248px] p-2">
                <div className="border-b border-border px-1.5 pb-2">
                  <p className="truncate text-sm font-medium text-foreground">{displayName}</p>
                  {email ? (
                    <p className="truncate text-xs text-muted-foreground">
                      {me?.role ? `${email} · ${me.role}` : email}
                    </p>
                  ) : null}
                </div>

                <div className="py-2">
                  <MenuPrimitive.RadioGroup
                    aria-label="Theme"
                    value={themeMode}
                    onValueChange={(value) => {
                      if (isThemeMode(value)) setThemeMode(value);
                    }}
                    className="flex h-[34px] items-center gap-1 rounded-md bg-muted p-1"
                  >
                    <ThemeOption value="light" label="Light mode" text="Light">
                      <Sun className="h-4 w-4" />
                    </ThemeOption>

                    <ThemeOption value="dark" label="Dark mode" text="Dark">
                      <Moon className="h-4 w-4" />
                    </ThemeOption>

                    <ThemeOption value="system" label="System theme" text="System">
                      <Monitor className="h-4 w-4" />
                    </ThemeOption>
                  </MenuPrimitive.RadioGroup>
                </div>

                <DropdownMenuSeparator />

                <DropdownMenuItem
                  className="leading-5 text-sign-out-foreground no-underline hover:no-underline data-[highlighted]:text-sign-out-foreground data-[highlighted]:no-underline"
                  onClick={() => void logout()}
                >
                  <LogOut className="size-4 shrink-0" aria-hidden="true" />
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenu>
        ) : (
          <Button type="button" className="mr-2" onClick={() => void login()}>
            Sign in
          </Button>
        )}
      </MenuBarActions>
    </NavigationMenu>
  );
}

function ThemeOption({
  value,
  label,
  text,
  children,
}: {
  value: ThemeMode;
  label: string;
  text: string;
  children: ReactNode;
}): ReactNode {
  return (
    <MenuPrimitive.RadioItem
      value={value}
      aria-label={label}
      title={label}
      closeOnClick={false}
      className={cn(
        "flex h-auto flex-1 cursor-pointer items-center justify-center gap-1 rounded-sm border border-transparent px-1.5 py-0.5 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "text-muted-foreground-strong hover:text-foreground",
        "data-checked:border-border-strong data-checked:bg-card data-checked:text-foreground data-checked:shadow-[0_1px_3px_0_rgba(0,0,0,0.10)]",
      )}
    >
      {children}
      <span>{text}</span>
    </MenuPrimitive.RadioItem>
  );
}
