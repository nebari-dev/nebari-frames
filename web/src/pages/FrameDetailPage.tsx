import { useState } from "react";
import { Link, useParams } from "react-router";
import { GitFork } from "lucide-react";
import { useQuery } from "@connectrpc/connect-query";
import { Code, ConnectError } from "@connectrpc/connect";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { parseFrameContent } from "@/lib/frame-yaml";
import { FrameSlots } from "@/components/slots/FrameSlots";
import { InheritanceTrail } from "@/components/frame/InheritanceTrail";
import { VersionHistory } from "@/components/frame/VersionHistory";
import { UseThisFrame } from "@/components/frame/UseThisFrame";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

function fmtDateTime(ts?: Timestamp): string {
  if (!ts) return "";
  return timestampDate(ts).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function fmtBytes(bytes?: bigint): string {
  if (bytes === undefined) return "";
  const n = Number(bytes);
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

// A single label/value pair in the metadata grid. Renders nothing when empty
// so partial data (or minimal test fixtures) never leaves dangling labels.
function Meta({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  if (!value) return null;
  return (
    <div className="space-y-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={mono ? "font-mono text-xs break-all" : "text-sm"}>{value}</dd>
    </div>
  );
}

export function FrameDetailPage() {
  const { org = "", name = "" } = useParams();
  const [deleteOpen, setDeleteOpen] = useState(false);
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
  const frame = resp.frame!;
  const version = resp.version!;
  let doc;
  try {
    doc = parseFrameContent(version.content);
  } catch {
    return <p className="text-destructive">This frame&apos;s content could not be displayed.</p>;
  }

  const isLatest = !frame.latestVersion || frame.latestVersion === version.version;

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_18rem]">
      <div className="space-y-4">
        <header className="flex items-start justify-between gap-4">
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold">{frame.name}</h1>
              <Badge variant="secondary" className="font-mono">v{version.version}</Badge>
              {isLatest ? (
                <Badge variant="outline">Latest</Badge>
              ) : (
                <Badge variant="outline" title={`Latest is v${frame.latestVersion}`}>
                  latest: v{frame.latestVersion}
                </Badge>
              )}
            </div>
            <p className="text-muted-foreground">{frame.description}</p>
          </div>
          <div className="flex gap-2">
            {resp.permissions?.canEdit && (
              <Button variant="outline" render={<Link to={`/frames/${org}/${name}/edit`} />}>Edit</Button>
            )}
            {resp.permissions?.canDelete && (
              <Button variant="outline" onClick={() => setDeleteOpen(true)}>Delete</Button>
            )}
          </div>
        </header>

        <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-md border p-4 sm:grid-cols-3">
          <Meta label="Owner" value={frame.ownerSub} />
          <Meta label="Published by" value={version.publishedBy} />
          <Meta label="Published" value={fmtDateTime(version.publishedAt)} />
          <Meta label="Created" value={fmtDateTime(frame.createdAt)} />
          <Meta label="Updated" value={fmtDateTime(frame.updatedAt)} />
          <Meta label="Size" value={fmtBytes(version.sizeBytes)} />
          <Meta label="Digest" value={version.digest} mono />
        </dl>

        {version.changelog && (
          <div className="rounded-md border p-3 text-sm">
            <div className="mb-1 font-medium">Changelog</div>
            <p className="text-muted-foreground">{version.changelog}</p>
          </div>
        )}

        <FrameSlots doc={doc} />
        <VersionHistory versions={versionsQ.data?.versions ?? []} />
      </div>
      <aside className="space-y-4">
        <UseThisFrame org={org} name={name} />
        <Link
          to={`/hierarchy?focus=${org}/${name}`}
          className="flex items-center gap-2 text-sm text-primary hover:underline"
        >
          <GitFork className="size-4" />
          View in hierarchy
        </Link>
        <InheritanceTrail parents={resp.extends} />
        {(resp.excludes?.length ?? 0) > 0 && (
          <div className="text-sm">
            <div className="mb-1 font-medium">Excludes</div>
            <ul className="space-y-1 text-muted-foreground">
              {resp.excludes.map((ex) => (
                <li key={ex} className="font-mono text-xs">{ex}</li>
              ))}
            </ul>
          </div>
        )}
      </aside>
      <DeleteFrameDialog org={org} name={name} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </div>
  );
}
