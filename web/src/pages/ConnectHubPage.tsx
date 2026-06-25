import { connectProviders } from "@/lib/connect-providers";
import { ProviderTile } from "@/components/connect/ProviderTile";

export function ConnectHubPage() {
  const ordered = [...connectProviders].sort((a, b) =>
    a.status === b.status ? 0 : a.status === "available" ? -1 : 1,
  );
  return (
    <div className="space-y-6">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold">Connect</h1>
        <p className="text-muted-foreground">
          Add the Frames Hub as a connector in your AI client to use your organization&apos;s
          Frames as context. Pick a provider to see step-by-step setup.
        </p>
      </header>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {ordered.map((p) => (
          <ProviderTile key={p.id} provider={p} />
        ))}
      </div>
    </div>
  );
}
