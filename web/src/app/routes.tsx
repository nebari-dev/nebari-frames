import { Route, Routes } from "react-router";
import { RequireAuth } from "./RequireAuth";
import { RequireMembership } from "./RequireMembership";
import { AppShell } from "./AppShell";
import { LoginPage } from "@/pages/LoginPage";
import { CallbackPage } from "@/pages/CallbackPage";
import { NoAccessPage } from "@/pages/NoAccessPage";
import { CatalogPage } from "@/pages/CatalogPage";
import { FrameDetailPage } from "@/pages/FrameDetailPage";
import { FrameAuthoringPage } from "@/pages/FrameAuthoringPage";
import { ConnectHubPage } from "@/pages/ConnectHubPage";
import { ConnectProviderPage } from "@/pages/ConnectProviderPage";

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
            <Route path="/frames/new" element={<FrameAuthoringPage mode="create" />} />
            <Route path="/frames/:org/:name" element={<FrameDetailPage />} />
            <Route path="/frames/:org/:name/edit" element={<FrameAuthoringPage mode="edit" />} />
            <Route path="/connect" element={<ConnectHubPage />} />
            <Route path="/connect/:provider" element={<ConnectProviderPage />} />
          </Route>
        </Route>
      </Route>
    </Routes>
  );
}
