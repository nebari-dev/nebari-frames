import { Link, Outlet } from "react-router";
import { useAuth } from "@/lib/auth/useAuth";
import { Button } from "@/components/ui/button";

export function AppShell() {
  const { logout, user } = useAuth();
  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b">
        <div className="mx-auto max-w-6xl flex items-center justify-between px-4 h-14">
          <Link to="/" className="font-semibold">Nebari Frames</Link>
          <div className="flex items-center gap-3 text-sm">
            <span className="text-muted-foreground">{user?.profile?.email ?? ""}</span>
            <Button variant="ghost" size="sm" onClick={() => void logout()}>Log out</Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl w-full px-4 py-6 flex-1">
        <Outlet />
      </main>
    </div>
  );
}
