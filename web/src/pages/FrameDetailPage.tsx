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
import { VersionHistory } from "@/components/frame/VersionHistory";
import { UseThisFrame } from "@/components/frame/UseThisFrame";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";
import { ExportMenu } from "@/components/frame/ExportFrameButton";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/collapsible";

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

// A single label/value pair in the details list. Renders nothing when empty
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
    <div className="max-w-6xl space-y-6">
      {/* The header carries the frame's identity - what a reader needs to
          decide whether this frame applies to them. Registry bookkeeping
          (owner, digest, timestamps) lives in the collapsed Details box. */}
      <header className="flex items-start justify-between gap-4">
        <div className="min-w-0 space-y-2">
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
            {doc.visibility && <Badge variant="outline">{doc.visibility}</Badge>}
            {doc.scope && <Badge variant="outline">{doc.scope}</Badge>}
          </div>
          <p className="text-muted-foreground">{frame.description}</p>
          {doc.maintainer && (
            <p className="text-sm text-muted-foreground">Maintained by {doc.maintainer}</p>
          )}
          {resp.extends.length > 0 && (
            <p className="flex flex-wrap items-center gap-1.5 text-sm">
              <span className="text-muted-foreground">Inherits from</span>
              {resp.extends.map((p) => (
                <Badge
                  key={`${p.ref}@${p.version}`}
                  variant="outline"
                  className="font-mono font-normal"
                  render={<Link to={`/frames/${p.ref}`} />}
                >
                  {p.ref}@{p.version}
                </Badge>
              ))}
            </p>
          )}
          {(resp.excludes?.length ?? 0) > 0 && (
            <p className="text-sm text-muted-foreground">
              Excludes <span className="font-mono text-xs">{resp.excludes.join(", ")}</span>
            </p>
          )}
        </div>
        <div className="flex shrink-0 gap-2">
          {resp.permissions?.canEdit && (
            <Button variant="outline" render={<Link to={`/frames/${org}/${name}/edit`} />}>Edit</Button>
          )}
          {/* Export is available to anyone who can read the frame. */}
          <ExportMenu name={frame.name} content={version.content} />
          {resp.permissions?.canDelete && (
            <Button variant="outline" onClick={() => setDeleteOpen(true)}>Delete</Button>
          )}
        </div>
      </header>

      <div className="grid gap-10 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start">
        {/* The document itself is the page's main content, full-width and
            unboxed so it reads like the .frame.md it exports as. */}
        <div className="min-w-0 max-w-3xl">
          <FrameSlots doc={doc} />
        </div>

        <aside className="space-y-6">
          <UseThisFrame org={org} name={name} />

          <VersionHistory versions={versionsQ.data?.versions ?? []} />

          <Card className="p-3">
            <Link
              to={`/?view=hierarchy&focus=${org}/${name}`}
              className="flex items-center gap-2 text-sm text-primary hover:underline"
            >
              <GitFork className="size-4" />
              View in hierarchy
            </Link>
          </Card>

          {/* Registry bookkeeping, collapsed by default: it answers audit
              questions, not "what does this frame say" questions. */}
          <Collapsible className="rounded-lg border border-border bg-card p-3 text-card-foreground shadow-xs">
            <CollapsibleTrigger className="text-sm font-medium">Details</CollapsibleTrigger>
            <CollapsibleContent className="pt-3">
              <dl className="space-y-3">
                <Meta label="Owner" value={frame.ownerSub} />
                <Meta label="Published by" value={version.publishedBy} />
                <Meta label="Published" value={fmtDateTime(version.publishedAt)} />
                <Meta label="Created" value={fmtDateTime(frame.createdAt)} />
                <Meta label="Updated" value={fmtDateTime(frame.updatedAt)} />
                <Meta label="Changelog" value={version.changelog} />
                <Meta label="Size" value={fmtBytes(version.sizeBytes)} />
                <Meta label="Digest" value={version.digest} mono />
              </dl>
            </CollapsibleContent>
          </Collapsible>
        </aside>
      </div>
      <DeleteFrameDialog org={org} name={name} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </div>
  );
}
