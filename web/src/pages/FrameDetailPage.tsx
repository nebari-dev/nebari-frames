import { useId, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { ArrowLeft, History, RotateCcw } from "lucide-react";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { Code, ConnectError } from "@connectrpc/connect";
import type { ReactNode } from "react";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { parseFrameContent, serializeFrameDoc } from "@/lib/frame-yaml";
import { suggestNextVersion } from "@/lib/authoring-schema";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";
import { ExportMenu } from "@/components/frame/ExportFrameButton";
import { FrameVersionsDialog } from "@/components/frame/FrameVersionsDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { fillTextareaSlot } from "@/components/form/fill-height";
import { cn } from "@/lib/utils";

// One labeled read-only field, mirroring the edit form's Field styling. The
// control gets the generated id so the Label associates with it properly
// instead of relying on implicit wrapping.
function ViewField({
  label,
  children,
}: {
  label: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      {children(id)}
    </div>
  );
}

export function FrameDetailPage() {
  const { org = "", name = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [versionsOpen, setVersionsOpen] = useState(false);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoreError, setRestoreError] = useState<string | null>(null);
  // ?v= pins the page to a past version; without it the latest is shown.
  const [params] = useSearchParams();
  const viewedVersion = params.get("v") ?? "";

  const frameQ = useQuery(FrameService.method.getFrame, {
    orgSlug: org,
    name,
    version: viewedVersion,
  });
  const versionsQ = useQuery(FrameService.method.listFrameVersions, { orgSlug: org, name });
  const publish = useMutation(FrameService.method.publishFrame);

  if (frameQ.error) {
    const code = ConnectError.from(frameQ.error).code;
    if (code === Code.NotFound) {
      return <p className="text-muted-foreground">Frame not found, or you do not have access.</p>;
    }
    return <p className="text-destructive-foreground">Could not load this frame.</p>;
  }
  const resp = frameQ.data;
  const frame = resp?.frame;
  const version = resp?.version;
  // Covers isLoading plus in-between states (e.g. a paused retry) where the
  // query has neither data nor an error yet.
  if (!resp || !frame || !version) {
    return <div className="space-y-4"><Skeleton className="h-8 w-64" /><Skeleton className="h-40" /></div>;
  }

  let doc;
  try {
    doc = parseFrameContent(version.content);
  } catch {
    return <p className="text-destructive-foreground">This frame&apos;s content could not be displayed.</p>;
  }

  const isLatest = !frame.latestVersion || frame.latestVersion === version.version;
  const versions = versionsQ.data?.versions ?? [];
  const nextVersion = suggestNextVersion(frame.latestVersion || version.version);

  // Restoring republishes the viewed version's content as a new version on
  // top of the history - past versions themselves stay immutable.
  const restore = () => {
    setRestoreError(null);
    const content = new TextEncoder().encode(
      serializeFrameDoc({ ...doc, version: nextVersion }),
    );
    publish.mutate(
      { content, changelog: `Restore of v${version.version}` },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries();
          setRestoreOpen(false);
          navigate(`/frames/${org}/${name}`);
        },
        onError: (err) => setRestoreError(ConnectError.from(err).rawMessage),
      },
    );
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-6">
      <header className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label="Back to frames"
            render={<Link to="/" />}
          >
            <ArrowLeft />
          </Button>
          <h1 className="text-2xl font-semibold">{frame.name}</h1>
          <Badge variant="secondary" className="font-mono">v{version.version}</Badge>
          {isLatest ? (
            <Badge variant="outline">Latest</Badge>
          ) : (
            <>
              <Badge variant="outline" title={`Latest is v${frame.latestVersion}`}>
                latest: v{frame.latestVersion}
              </Badge>
              <Link
                to={`/frames/${org}/${name}`}
                className="text-sm text-primary hover:underline"
              >
                View latest
              </Link>
            </>
          )}
          {doc.visibility && <Badge variant="outline">{doc.visibility}</Badge>}
          {frame.isTemplate && <Badge>Template</Badge>}
        </div>
        <div className="flex shrink-0 gap-2">
          {!isLatest && resp.permissions?.canEdit && (
            <Button onClick={() => setRestoreOpen(true)}>
              <RotateCcw />
              Restore this version
            </Button>
          )}
          <Button
            variant="ghost"
            title="Start a new Frame with this Frame's content as the starting point"
            render={<Link to={`/frames/new?from=${org}/${name}`} />}
          >
            Use as template
          </Button>
          {resp.permissions?.canEdit && (
            <Button variant="outline" render={<Link to={`/frames/${org}/${name}/edit`} />}>Edit</Button>
          )}
          <Button variant="outline" onClick={() => setVersionsOpen(true)}>
            <History />
            Versions{versions.length > 0 ? ` (${versions.length})` : ""}
          </Button>
          {/* Export is available to anyone who can read the frame. */}
          <ExportMenu name={frame.name} content={version.content} />
          {resp.permissions?.canDelete && (
            <Button variant="outline" onClick={() => setDeleteOpen(true)}>Delete</Button>
          )}
        </div>
      </header>

      {/* The read view is the edit form, disabled: the same two-column layout
          and controls, so reading and editing are one surface. */}
      <div className="grid min-h-0 gap-x-10 gap-y-6 lg:flex-1 lg:grid-cols-[minmax(0,32rem)_minmax(0,1fr)] lg:items-stretch">
        <div className="space-y-6">
          <div className="space-y-4">
            <ViewField label="Name">
              {(id) => <Input id={id} readOnly value={frame.name} />}
            </ViewField>
            <ViewField label="Description">
              {(id) => <Textarea id={id} readOnly rows={3} value={frame.description} />}
            </ViewField>
            <ViewField label="Visibility">
              {(id) => {
                const visibility = doc.visibility || "internal";
                return (
                  <Select disabled value={visibility}>
                    <SelectTrigger id={id}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value={visibility}>{visibility}</SelectItem>
                    </SelectContent>
                  </Select>
                );
              }}
            </ViewField>
            <ViewField label="Scope">
              {(id) => <Input id={id} readOnly value={doc.scope} placeholder="—" />}
            </ViewField>
            <ViewField label="Maintainer">
              {(id) => <Input id={id} readOnly value={doc.maintainer} placeholder="—" />}
            </ViewField>
            <Checkbox
              disabled
              checked={frame.isTemplate}
              description='List this Frame in the "start from a template" picker when creating new Frames.'
            >
              Offer as a template
            </Checkbox>
          </div>

          <div className="space-y-4 border-t border-border pt-4">
            <div className="space-y-1.5">
              <h3 className="text-sm font-medium text-foreground">Inherits from</h3>
              {resp.extends.length === 0 ? (
                <p className="text-sm text-muted-foreground">No parent Frames.</p>
              ) : (
                <div className="space-y-2">
                  {resp.extends.map((p) => (
                    <Input
                      key={`${p.ref}@${p.version}`}
                      readOnly
                      value={`${p.ref}@${p.version}`}
                      className="font-mono"
                    />
                  ))}
                  <Link
                    to={`/?view=hierarchy&focus=${org}/${name}`}
                    className="inline-block text-sm text-primary hover:underline"
                  >
                    View in hierarchy
                  </Link>
                </div>
              )}
            </div>
            <div className="space-y-1.5">
              <h3 className="text-sm font-medium text-foreground">Excludes</h3>
              {(resp.excludes?.length ?? 0) === 0 ? (
                <p className="text-sm text-muted-foreground">No exclusions.</p>
              ) : (
                <div className="space-y-2">
                  {resp.excludes.map((x) => (
                    <Input key={x} readOnly value={x} className="font-mono" />
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* The panel fills whatever height is left below the page chrome. */}
        <section className={cn("flex min-h-0 flex-col gap-2", fillTextareaSlot)}>
          <h2 className="text-lg font-semibold">Content</h2>
          <p className="text-xs text-muted-foreground">
            Free-form Markdown: the context this Frame carries into AI conversations.
          </p>
          <Textarea
            readOnly
            aria-label="Content"
            value={doc.body}
            className="min-h-[20rem] flex-1"
          />
        </section>
      </div>

      <FrameVersionsDialog
        org={org}
        name={name}
        frame={frame}
        version={version}
        versions={versions}
        open={versionsOpen}
        onOpenChange={setVersionsOpen}
      />

      <Dialog open={restoreOpen} onOpenChange={setRestoreOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Restore v{version.version}?</DialogTitle>
            <DialogDescription>
              This publishes the content of v{version.version} as a new version (v
              {nextVersion}). Nothing is deleted — every published version stays in the
              history.
            </DialogDescription>
          </DialogHeader>
          {restoreError && (
            <p className="text-sm text-destructive-foreground">{restoreError}</p>
          )}
          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
            <Button onClick={restore} loading={publish.isPending}>
              Restore
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <DeleteFrameDialog org={org} name={name} open={deleteOpen} onOpenChange={setDeleteOpen} />
    </div>
  );
}
