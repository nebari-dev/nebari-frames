import { useState } from "react";
import { Link } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Button } from "@/components/ui/button";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";

export function AdminFramesPage() {
  const framesQ = useQuery(FrameService.method.listFrames, {});
  const [target, setTarget] = useState<{ org: string; name: string } | null>(null);

  if (framesQ.isLoading) return <div className="text-muted-foreground">Loading...</div>;
  if (framesQ.error) return <p className="text-destructive">Could not load frames.</p>;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Frames</h1>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-muted-foreground"><th className="py-2">Name</th><th>Version</th><th></th></tr>
        </thead>
        <tbody>
          {(framesQ.data?.frames ?? []).map((f) => (
            <tr key={`${f.orgSlug}/${f.name}`} className="border-t border-border">
              <td className="py-2">
                <Link to={`/frames/${f.orgSlug}/${f.name}`} className="text-primary hover:underline">{f.name}</Link>
              </td>
              <td>v{f.latestVersion}</td>
              <td className="text-right">
                <Button variant="ghost" size="sm" onClick={() => setTarget({ org: f.orgSlug, name: f.name })}>Delete</Button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
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
