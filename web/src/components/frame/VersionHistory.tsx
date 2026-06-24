import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import type { FrameVersionSummary } from "@gen/frames/v1/frame_pb";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

function fmt(ts?: Timestamp): string {
  if (!ts) return "";
  return timestampDate(ts).toLocaleDateString();
}

export function VersionHistory({ versions }: { versions: FrameVersionSummary[] }) {
  if (versions.length === 0) return null;
  return (
    <Collapsible className="border rounded-md p-3">
      <CollapsibleTrigger className="font-medium">Version history ({versions.length})</CollapsibleTrigger>
      <CollapsibleContent className="pt-2 space-y-2">
        {versions.map((v) => (
          <div key={v.version} className="text-sm border-b last:border-0 pb-2">
            <div className="font-mono">v{v.version}</div>
            <div className="text-muted-foreground">{v.changelog || "No changelog"}</div>
            <div className="text-xs text-muted-foreground">{v.publishedBy} - {fmt(v.publishedAt)}</div>
          </div>
        ))}
      </CollapsibleContent>
    </Collapsible>
  );
}
