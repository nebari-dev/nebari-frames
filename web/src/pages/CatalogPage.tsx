import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Plus, Search } from "lucide-react";
import { filterFrames } from "@/lib/filter";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

const CARD_GRID = "grid gap-4 [grid-template-columns:repeat(auto-fill,minmax(20rem,1fr))]";

export function CatalogPage() {
  const [query, setQuery] = useState("");
  const { data, isLoading, error } = useQuery(FrameService.method.listFrames, {});

  if (isLoading) {
    return (
      <div className={CARD_GRID}>
        {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-28" />)}
      </div>
    );
  }
  if (error) {
    return <p className="text-destructive">Could not load frames.</p>;
  }

  const frames = filterFrames(data?.frames ?? [], query);
  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Frames</h1>
        <p className="text-sm text-muted-foreground">
          Browse, author, and connect your organization's Frames.
        </p>
      </div>
      <div className="flex items-center justify-between gap-4">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            placeholder="Search frames…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="h-9 pl-9"
          />
        </div>
        {data?.canCreate && (
          <Button render={<Link to="/frames/new" />}>
            <Plus />
            Create new Frame
          </Button>
        )}
      </div>
      {frames.length === 0 ? (
        <p className="text-muted-foreground">No frames found.</p>
      ) : (
        <div className={CARD_GRID}>
          {frames.map((f) => (
            <Link key={`${f.orgSlug}/${f.name}`} to={`/frames/${f.orgSlug}/${f.name}`}>
              <Card className="p-4 h-full hover:border-foreground/30 transition-colors">
                <div className="font-medium">{f.name}</div>
                <div className="text-sm text-muted-foreground line-clamp-2">{f.description}</div>
                <div className="text-xs text-muted-foreground mt-3">
                  {f.ownerSub} - v{f.latestVersion}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
