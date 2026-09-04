import { Link, useParams } from "react-router";
import { ArrowLeft } from "lucide-react";
import { getConnectProvider } from "@/lib/connect-providers";
import { CopyField } from "@/components/connect/CopyField";
import { MarkdownView } from "@/components/MarkdownView";
import { PageHeader } from "@/components/layout/PageHeader";
import { Button } from "@/components/ui/button";

export function ConnectProviderPage() {
  const { provider: id = "" } = useParams();
  const provider = getConnectProvider(id);

  if (!provider || provider.status !== "available" || !provider.steps) {
    return (
      <div className="space-y-4 motion-safe:animate-fade-in">
        <PageHeader
          title="Not available yet"
          description="We don't have connector instructions for this provider yet."
        />
        <Button variant="outline" render={<Link to="/connect" />}>
          <ArrowLeft />
          Back to Connect
        </Button>
      </div>
    );
  }

  const connectorUrl = `${window.location.origin}/mcp`;

  return (
    <div className="max-w-2xl space-y-6 motion-safe:animate-fade-in">
      <PageHeader
        title={provider.name}
        description={`Connect ${provider.name} to the Frames Hub in a few steps.`}
      />

      <CopyField label="Connector URL" value={connectorUrl} copyLabel="Copy URL" />

      <ol className="space-y-4">
        {provider.steps.map((step, i) => (
          <li key={i} className="space-y-1">
            <h2 className="text-sm font-medium text-foreground">
              {i + 1}. {step.title}
            </h2>
            <MarkdownView source={step.body} />
          </li>
        ))}
      </ol>

      {provider.verifyPrompt && (
        <div className="space-y-2">
          <h2 className="text-sm font-medium text-foreground">Verify it worked</h2>
          <p className="text-sm text-muted-foreground">
            Start a new chat and try a Frame-aware prompt:
          </p>
          <CopyField value={provider.verifyPrompt} copyLabel="Copy prompt" />
        </div>
      )}

      {provider.lastVerified && (
        <footer className="border-t border-border pt-3 text-xs text-muted-foreground">
          Last verified: {provider.lastVerified}
        </footer>
      )}
    </div>
  );
}
