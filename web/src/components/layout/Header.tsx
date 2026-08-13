import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { ChevronDown, LogOut, Monitor, Moon, Sun, User } from "lucide-react";
import type { ReactNode } from "react";
import { NavLink } from "react-router";
import logoDark from "@/assets/nebari-logo_dark.svg";
import logoLight from "@/assets/nebari-logo_light.svg";
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
import { MenuBarActions, MenuBarBrand, MenuBarNav, NavigationMenu } from "@/components/ui/navigation-menu";
import { isThemeMode, type ThemeMode } from "@/hooks/use-theme-preference";
import { useTheme } from "@/hooks/theme-provider";
import { useAuth } from "@/lib/auth/useAuth";
import { cn } from "@/lib/utils";

function navItemClass({ isActive }: { isActive: boolean }): string {
  return cn(
    "rounded-md px-3 py-1.5 text-sm font-medium outline-none motion-safe:transition-colors",
    "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-0",
    isActive
      ? "bg-header-action-hover text-header-foreground"
      : "text-muted-foreground hover:bg-header-action-hover/60 hover:text-header-foreground",
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

  return (
    <NavigationMenu className="h-14 justify-between border-header-border bg-header-background pl-4 text-header-foreground">
      <div className="flex items-center gap-6">
        <MenuBarBrand href="/" aria-label="Go to homepage">
          <img src={isDarkMode ? logoDark : logoLight} alt="Nebari" className="h-8 w-auto" />
        </MenuBarBrand>

        <MenuBarNav className="flex-none">
          <NavLink to="/" end className={navItemClass}>
            Frames
          </NavLink>
          {me?.role === "admin" && (
            <NavLink to="/admin" className={navItemClass}>
              Admin
            </NavLink>
          )}
          <NavLink to="/connect" className={navItemClass}>
            Connect
          </NavLink>
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
                <div className="border-b px-1.5 pb-2">
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
                  className="leading-5 text-sign-out-foreground data-[highlighted]:text-sign-out-foreground"
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
