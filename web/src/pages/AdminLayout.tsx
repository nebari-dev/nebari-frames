import { Outlet } from "react-router";
import { PageHeader } from "@/components/layout/PageHeader";
import { SectionNav } from "@/components/layout/SectionNav";

// Shared frame around the admin section: one heading, then links that navigate
// between the member and template management pages. Frame management
// (edit/delete) lives on the catalog table itself.
const SECTIONS = [
  { to: "/admin/members", label: "Members" },
  { to: "/admin/templates", label: "Templates" },
];

export function AdminLayout() {
  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <PageHeader
        title="Admin"
        description="Manage members and templates for your organization."
      />

      <SectionNav items={SECTIONS} ariaLabel="Admin sections" />

      <Outlet />
    </div>
  );
}
