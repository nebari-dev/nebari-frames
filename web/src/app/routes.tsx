import { Route, Routes } from "react-router";
import { RequireAuth } from "./RequireAuth";
import { RequireMembership } from "./RequireMembership";
import { AppShell } from "./AppShell";
import { LoginPage } from "@/pages/LoginPage";
import { CallbackPage } from "@/pages/CallbackPage";
import { NoAccessPage } from "@/pages/NoAccessPage";
import { CatalogPage } from "@/pages/CatalogPage";
import { FrameDetailPage } from "@/pages/FrameDetailPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<CallbackPage />} />
      <Route element={<RequireAuth />}>
        <Route path="/no-access" element={<NoAccessPage />} />
        <Route element={<RequireMembership />}>
          <Route element={<AppShell />}>
            <Route path="/" element={<CatalogPage />} />
            <Route path="/frames/:org/:name" element={<FrameDetailPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
