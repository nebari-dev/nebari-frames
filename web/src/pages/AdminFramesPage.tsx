import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";

export function AdminFramesPage() {
  const framesQ = useQuery(FrameService.method.listFrames, {});
  const [target, setTarget] = useState<{ org: string; name: string } | null>(null);

  if (framesQ.error) return <p className="text-destructive">Could not load frames.</p>;

  const frames = framesQ.data?.frames ?? [];

  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-foreground">Frames</h1>
        <p className="text-sm text-muted-foreground">
          Manage every Frame published across your organization.
        </p>
      </div>

      {framesQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : frames.length === 0 ? (
        <Card className="p-10 text-center text-sm text-muted-foreground">No frames yet.</Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/50 text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th className="px-4 py-3 font-medium">Name</th>
                <th className="px-4 py-3 font-medium">Organization</th>
                <th className="px-4 py-3 font-medium">Version</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {frames.map((f) => (
                <tr
                  key={`${f.orgSlug}/${f.name}`}
                  className="border-b border-border last:border-0 transition-colors hover:bg-muted/40"
                >
                  <td className="px-4 py-3">
                    <Link
                      to={`/frames/${f.orgSlug}/${f.name}`}
                      className="font-medium text-foreground hover:text-primary hover:underline"
                    >
                      {f.name}
                    </Link>
                    {f.description && (
                      <div className="text-xs text-muted-foreground line-clamp-1">
                        {f.description}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{f.orgSlug}</td>
                  <td className="px-4 py-3">
                    <Badge variant="secondary">v{f.latestVersion}</Badge>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-muted-foreground hover:text-destructive-foreground"
                      onClick={() => setTarget({ org: f.orgSlug, name: f.name })}
                    >
                      <Trash2 className="size-4" />
                      Delete
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}

      {target && (
        <DeleteFrameDialog
          org={target.org}
          name={target.name}
          open
          onOpenChange={(o) => !o && setTarget(null)}
          onDeleted={() => setTarget(null)}
        />
      )}
    </div>
  );
}
