import { Link, useParams } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { Code, ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { parseFrameContent } from "@/lib/frame-yaml";
import { FrameSlots } from "@/components/slots/FrameSlots";
import { InheritanceTrail } from "@/components/frame/InheritanceTrail";
import { VersionHistory } from "@/components/frame/VersionHistory";
import { UseThisFrame } from "@/components/frame/UseThisFrame";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

export function FrameDetailPage() {
  const { org = "", name = "" } = useParams();
  const frameQ = useQuery(FrameService.method.getFrame, { orgSlug: org, name });
  const versionsQ = useQuery(FrameService.method.listFrameVersions, { orgSlug: org, name });

  if (frameQ.isLoading) {
    return <div className="space-y-4"><Skeleton className="h-8 w-64" /><Skeleton className="h-40" /></div>;
  }
  if (frameQ.error) {
    const code = ConnectError.from(frameQ.error).code;
    if (code === Code.NotFound) {
      return <p className="text-muted-foreground">Frame not found, or you do not have access.</p>;
    }
    return <p className="text-destructive">Could not load this frame.</p>;
  }

  const resp = frameQ.data!;
  let doc;
  try {
    doc = parseFrameContent(resp.version!.content);
  } catch {
    return <p className="text-destructive">This frame&apos;s content could not be displayed.</p>;
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_18rem]">
      <div className="space-y-4">
        <header className="flex items-start justify-between">
          <div className="space-y-1">
            <h1 className="text-2xl font-semibold">{resp.frame!.name}</h1>
            <p className="text-muted-foreground">{resp.frame!.description}</p>
            <p className="text-xs text-muted-foreground">v{resp.version!.version} - {resp.frame!.ownerSub}</p>
          </div>
          {resp.permissions?.canEdit && (
            <Button variant="outline" render={<Link to={`/frames/${org}/${name}/edit`} />}>Edit</Button>
          )}
        </header>
        <FrameSlots doc={doc} />
        <VersionHistory versions={versionsQ.data?.versions ?? []} />
      </div>
      <aside className="space-y-4">
        <UseThisFrame org={org} name={name} />
        <InheritanceTrail parents={resp.extends} />
      </aside>
    </div>
  );
}
