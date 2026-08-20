import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { parseFrameContent } from "@/lib/frame-yaml";
import { MarkdownView } from "@/components/MarkdownView";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export function ResolvedPreview({
  org,
  name,
  version,
  open,
  onClose,
}: {
  org: string;
  name: string;
  version: string;
  open: boolean;
  onClose: () => void;
}) {
  const { data, isLoading, error } = useQuery(
    FrameService.method.resolveFrame,
    { orgSlug: org, name, version },
    { enabled: open && name !== "" },
  );

  let parsedDoc;
  let parseError = false;
  if (data && !isLoading && !error) {
    try {
      parsedDoc = parseFrameContent(data.resolvedContent);
    } catch {
      parseError = true;
    }
  }

  let body;
  if (isLoading) body = <p className="text-muted-foreground">Resolving...</p>;
  else if (error || parseError) body = <p className="text-destructive-foreground">{parseError ? "Resolved content could not be displayed." : "Could not resolve this frame."}</p>;
  else if (parsedDoc) body = <MarkdownView source={parsedDoc.body} />;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Preview (resolved Frame)</DialogTitle>
          <DialogDescription>
            Reflects inheritance from the saved parents; unpublished edits in the form are
            not included.
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-y-auto">{body}</div>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>Close</DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
