import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";

export function AppShell() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <Header />
      <main className="w-full flex-1 px-6 py-6 sm:px-8 lg:px-10">
        <Outlet />
      </main>
    </div>
  );
}
