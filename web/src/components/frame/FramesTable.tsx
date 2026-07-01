import { Link } from "react-router";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import type { FrameSummary } from "@gen/frames/v1/frame_pb";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";

function fmtDate(ts?: Timestamp): string {
  if (!ts) return "—";
  return timestampDate(ts).toLocaleDateString();
}

export function FramesTable({ frames }: { frames: FrameSummary[] }) {
  return (
    <Card className="overflow-hidden p-0">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/50 text-left text-xs uppercase tracking-wide text-muted-foreground">
            <th className="px-4 py-3 font-medium">Name</th>
            <th className="px-4 py-3 font-medium">Description</th>
            <th className="px-4 py-3 font-medium">Owner</th>
            <th className="px-4 py-3 font-medium">Version</th>
            <th className="px-4 py-3 font-medium">Updated</th>
          </tr>
        </thead>
        <tbody>
          {frames.map((f) => (
            <tr
              key={`${f.orgSlug}/${f.name}`}
              className="border-b border-border last:border-0 motion-safe:transition-colors hover:bg-muted/40"
            >
              <td className="px-4 py-3">
                <Link
                  to={`/frames/${f.orgSlug}/${f.name}`}
                  className="font-medium text-foreground hover:underline"
                >
                  {f.name}
                </Link>
              </td>
              <td className="max-w-md px-4 py-3 text-muted-foreground">
                <span className="line-clamp-1">{f.description}</span>
              </td>
              <td className="px-4 py-3 text-muted-foreground">{f.ownerSub}</td>
              <td className="px-4 py-3">
                <Badge variant="secondary" className="font-mono">v{f.latestVersion}</Badge>
              </td>
              <td className="whitespace-nowrap px-4 py-3 text-muted-foreground">{fmtDate(f.updatedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}
