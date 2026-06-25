import { Link, useParams } from "react-router";
import { getConnectProvider } from "@/lib/connect-providers";
import { CopyField } from "@/components/connect/CopyField";
import { MarkdownView } from "@/components/MarkdownView";

export function ConnectProviderPage() {
  const { provider: id = "" } = useParams();
  const provider = getConnectProvider(id);

  if (!provider || provider.status !== "available" || !provider.steps) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-semibold">Not available yet</h1>
        <p className="text-muted-foreground">
          We don&apos;t have connector instructions for this provider yet.
        </p>
        <Link to="/connect" className="text-sm text-primary hover:underline">
          Back to Connect
        </Link>
      </div>
    );
  }

  const connectorUrl = `${window.location.origin}/mcp`;

  return (
    <div className="space-y-6 max-w-2xl">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold">{provider.name}</h1>
        <p className="text-muted-foreground">
          Connect {provider.name} to the Frames Hub in a few steps.
        </p>
      </header>

      <CopyField label="Connector URL" value={connectorUrl} copyLabel="Copy URL" />

      <ol className="space-y-4">
        {provider.steps.map((step, i) => (
          <li key={i} className="space-y-1">
            <div className="font-medium">
              {i + 1}. {step.title}
            </div>
            <MarkdownView source={step.body} />
          </li>
        ))}
      </ol>

      {provider.verifyPrompt && (
        <div className="space-y-2">
          <div className="font-medium">Verify it worked</div>
          <p className="text-sm text-muted-foreground">
            Start a new chat and try a Frame-aware prompt:
          </p>
          <CopyField value={provider.verifyPrompt} copyLabel="Copy prompt" />
        </div>
      )}

      {provider.lastVerified && (
        <footer className="text-xs text-muted-foreground border-t pt-3">
          Last verified: {provider.lastVerified}
        </footer>
      )}
    </div>
  );
}
