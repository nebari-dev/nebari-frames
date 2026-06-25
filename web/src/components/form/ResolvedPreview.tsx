import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { parseFrameContent } from "@/lib/frame-yaml";
import { FrameSlots } from "@/components/slots/FrameSlots";
import { Dialog, DialogContent, DialogTitle, DialogClose } from "@/components/ui/dialog";

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
  else if (error || parseError) body = <p className="text-destructive">{parseError ? "Resolved content could not be displayed." : "Could not resolve this frame."}</p>;
  else if (parsedDoc) body = <FrameSlots doc={parsedDoc} />;

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogTitle>Preview (resolved Frame)</DialogTitle>
        <p className="mb-3 text-xs text-muted-foreground">
          Reflects inheritance from the saved parents; unpublished edits in the form are not included.
        </p>
        {body}
        <div className="mt-4 flex justify-end"><DialogClose onClose={onClose} /></div>
      </DialogContent>
    </Dialog>
  );
}
