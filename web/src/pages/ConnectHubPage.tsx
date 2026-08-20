import { connectProviders } from "@/lib/connect-providers";
import { ProviderTile } from "@/components/connect/ProviderTile";
import { PageHeader } from "@/components/layout/PageHeader";

export function ConnectHubPage() {
  const ordered = [...connectProviders].sort((a, b) =>
    a.status === b.status ? 0 : a.status === "available" ? -1 : 1,
  );
  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <PageHeader
        title="Connect"
        description="Add the Frames Hub as a connector in your AI client to use your organization's Frames as context. Pick a provider to see step-by-step setup."
      />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {ordered.map((p) => (
          <ProviderTile key={p.id} provider={p} />
        ))}
      </div>
    </div>
  );
}
