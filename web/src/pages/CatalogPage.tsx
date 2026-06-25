import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { filterFrames } from "@/lib/filter";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export function CatalogPage() {
  const [query, setQuery] = useState("");
  const { data, isLoading, error } = useQuery(FrameService.method.listFrames, {});

  if (isLoading) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-28" />)}
      </div>
    );
  }
  if (error) {
    return <p className="text-destructive">Could not load frames.</p>;
  }

  const frames = filterFrames(data?.frames ?? [], query);
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <Input
          placeholder="Search frames..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="max-w-sm"
        />
        {data?.canCreate && (
          <Button render={<Link to="/frames/new" />}>Create new Frame</Button>
        )}
      </div>
      {frames.length === 0 ? (
        <p className="text-muted-foreground">No frames found.</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
