import { Menu } from "@base-ui-components/react/menu";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { ChevronDown, LogIn, LogOut, Monitor, Moon, Sun, User } from "lucide-react";
import { Link, NavLink } from "react-router";
import logoDark from "@/assets/nebari-logo_dark.svg";
import logoLight from "@/assets/nebari-logo_light.svg";
import { useAuth } from "@/lib/auth/useAuth";
import { useTheme } from "@/lib/theme/ThemeContext";
import type { ThemeMode } from "@/lib/theme/useThemePreference";
import { cn } from "@/lib/utils";

function navItemClass({ isActive }: { isActive: boolean }): string {
  return cn(
    "rounded-md px-3 py-1.5 text-sm font-medium outline-none motion-safe:transition-colors",
    "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    isActive
      ? "bg-accent text-accent-foreground"
      : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
  );
}

const THEME_OPTIONS: { value: ThemeMode; label: string; icon: typeof Sun }[] = [
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "system", label: "System", icon: Monitor },
];

const menuItem = cn(
  "flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none",
  "motion-safe:transition-colors data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground",
);

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
    <header className="border-b border-border/60 bg-card">
      <div className="flex h-[60px] w-full items-center justify-between px-6 sm:px-8 lg:px-10">
        <div className="flex items-center gap-6">
          <Link to="/" className="flex items-center" aria-label="Nebari Frames home">
            <img src={isDarkMode ? logoDark : logoLight} alt="Nebari" className="h-8 w-auto" />
          </Link>

          <nav className="flex items-center gap-1">
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
          </nav>
        </div>

        <div className="flex items-center gap-2">
          <Menu.Root>
            <Menu.Trigger
              className={cn(
                "flex items-center gap-2 rounded-md py-1 pl-1 pr-1.5 outline-none",
                "motion-safe:transition-colors hover:bg-accent",
                "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
              )}
              aria-label="Account menu"
            >
              <span className="flex size-8 items-center justify-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                {initials || <User className="size-4" />}
              </span>
              <span className="hidden max-w-[18ch] truncate text-sm font-medium text-foreground sm:inline">
                {displayName}
              </span>
              <ChevronDown className="size-4 text-muted-foreground" />
            </Menu.Trigger>
            <Menu.Portal>
              <Menu.Positioner side="bottom" align="end" sideOffset={8}>
                <Menu.Popup
                  className={cn(
                    "w-64 rounded-lg border border-border bg-popover p-2 text-popover-foreground shadow-md outline-none",
                    "motion-safe:transition-[opacity,transform] motion-safe:duration-[--duration-base] motion-safe:ease-[--ease-emphasized]",
                    "data-[starting-style]:scale-95 data-[starting-style]:opacity-0",
                    "data-[ending-style]:scale-95 data-[ending-style]:opacity-0",
                  )}
                >
                  {email && (
                    <div className="border-b border-border px-2 pb-2">
                      <p className="truncate text-sm font-medium text-foreground">{displayName}</p>
                      <p className="truncate text-xs text-muted-foreground">
                        {me?.role ? `${email} · ${me.role}` : email}
                      </p>
                    </div>
                  )}

                  <Menu.RadioGroup
                    value={themeMode}
                    onValueChange={(value) => setThemeMode(value as ThemeMode)}
                    aria-label="Theme"
                    className="my-1 flex items-center gap-1 rounded-md bg-muted p-1"
                  >
                    {THEME_OPTIONS.map(({ value, label, icon: Icon }) => (
                      <Menu.RadioItem
                        key={value}
                        value={value}
                        closeOnClick={false}
                        className={cn(
                          "flex flex-1 cursor-pointer items-center justify-center gap-1.5 rounded-[6px] px-2 py-1.5 text-xs outline-none",
                          "motion-safe:transition-colors text-muted-foreground hover:text-foreground",
                          "data-[checked]:bg-background data-[checked]:text-foreground data-[checked]:shadow-sm",
                        )}
                      >
                        <Icon className="size-4" />
                        {label}
                      </Menu.RadioItem>
                    ))}
                  </Menu.RadioGroup>

                  <Menu.Separator className="my-1 h-px bg-border" />

                  {isAuthenticated ? (
                    <Menu.Item className={menuItem} onClick={() => void logout()}>
                      <LogOut className="size-4" />
                      Log out
                    </Menu.Item>
                  ) : (
                    <Menu.Item className={menuItem} onClick={() => void login()}>
                      <LogIn className="size-4" />
                      Log in
                    </Menu.Item>
                  )}
                </Menu.Popup>
              </Menu.Positioner>
            </Menu.Portal>
          </Menu.Root>
        </div>
      </div>
    </header>
  );
}
