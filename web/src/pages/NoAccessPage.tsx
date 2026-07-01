import { AuthLayout } from "@/components/layout/AuthLayout";

export function NoAccessPage() {
  return (
    <AuthLayout
      title="No organization access"
      description="Your account is not yet a member of an organization. Ask an org admin to add you using your email address, then sign in again."
    />
  );
}
