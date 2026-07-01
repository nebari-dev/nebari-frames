import { Link } from "react-router";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import type { FrameSummary } from "@gen/frames/v1/frame_pb";
import { Badge } from "@/components/ui/badge";

function fmtDate(ts?: Timestamp): string {
  if (!ts) return "—";
  return timestampDate(ts).toLocaleDateString();
}

export function FramesTable({ frames }: { frames: FrameSummary[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-muted/40 text-left text-xs text-muted-foreground">
            <th className="px-4 py-2 font-medium">Name</th>
            <th className="px-4 py-2 font-medium">Description</th>
            <th className="px-4 py-2 font-medium">Owner</th>
            <th className="px-4 py-2 font-medium">Version</th>
            <th className="px-4 py-2 font-medium">Updated</th>
          </tr>
        </thead>
        <tbody>
          {frames.map((f) => (
            <tr
              key={`${f.orgSlug}/${f.name}`}
              className="border-b last:border-0 motion-safe:transition-colors hover:bg-accent/40"
            >
              <td className="px-4 py-3.5">
                <Link
                  to={`/frames/${f.orgSlug}/${f.name}`}
                  className="font-medium text-foreground hover:underline"
                >
                  {f.name}
                </Link>
              </td>
              <td className="max-w-md px-4 py-3.5 text-muted-foreground">
                <span className="line-clamp-1">{f.description}</span>
              </td>
              <td className="px-4 py-3.5 text-muted-foreground">{f.ownerSub}</td>
              <td className="px-4 py-3.5">
                <Badge variant="secondary" className="font-mono">v{f.latestVersion}</Badge>
              </td>
              <td className="whitespace-nowrap px-4 py-3.5 text-muted-foreground">{fmtDate(f.updatedAt)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
