import { Link } from "react-router";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { ConnectProvider } from "@/lib/connect-providers";

export function ProviderTile({ provider }: { provider: ConnectProvider }) {
  if (provider.status === "available") {
    return (
      <Link to={`/connect/${provider.id}`} className="block">
        <Card className="p-4 h-full hover:border-border-strong transition-colors">
          <div className="font-medium">{provider.name}</div>
          <p className="text-sm text-muted-foreground mt-1">{provider.blurb}</p>
        </Card>
      </Link>
    );
  }
  return (
    <Card className="p-4 h-full opacity-60">
      <div className="flex items-center justify-between gap-2">
        <div className="font-medium">{provider.name}</div>
        <Badge variant="outline">Coming soon</Badge>
      </div>
      <p className="text-sm text-muted-foreground mt-1">{provider.blurb}</p>
    </Card>
  );
}
