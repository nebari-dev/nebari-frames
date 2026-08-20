import { Link } from "react-router";
import {
  Card,
  CardAction,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { ConnectProvider } from "@/lib/connect-providers";

export function ProviderTile({ provider }: { provider: ConnectProvider }) {
  if (provider.status === "available") {
    return (
      <Card
        size="sm"
        className="relative h-full motion-safe:transition-colors hover:border-border-strong has-[a:focus-visible]:border-ring has-[a:focus-visible]:ring-2 has-[a:focus-visible]:ring-ring"
      >
        <CardHeader>
          <CardTitle>
            {/* The whole tile is the hit target; the anchor stretches over it so
                the accessible name stays on a single real link. */}
            <Link
              to={`/connect/${provider.id}`}
              className="after:absolute after:inset-0 after:content-[''] focus-visible:outline-none"
            >
              {provider.name}
            </Link>
          </CardTitle>
          <CardDescription>{provider.blurb}</CardDescription>
        </CardHeader>
      </Card>
    );
  }
  return (
    <Card size="sm" className="h-full opacity-60">
      <CardHeader>
        <CardTitle>{provider.name}</CardTitle>
        <CardAction>
          <Badge variant="outline">Coming soon</Badge>
        </CardAction>
        <CardDescription>{provider.blurb}</CardDescription>
      </CardHeader>
    </Card>
  );
}
