import { Outlet } from "react-router";
import { Header } from "@/components/layout/Header";

export function AppShell() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <Header />
      {/* A flex column so pages that want to fill the viewport (the frame
          content editor) can stretch with flex-1 instead of subtracting the
          chrome height with hardcoded pixel math. */}
      <main className="flex w-full flex-1 flex-col px-6 py-6 sm:px-8 lg:px-10">
        <Outlet />
      </main>
    </div>
  );
}
