import { useState } from "react";
import { useNavigate } from "react-router";
import { useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Dialog, DialogContent, DialogTitle, DialogClose } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { mapDeleteError } from "@/lib/delete-errors";

export function DeleteFrameDialog({
  org,
  name,
  open,
  onOpenChange,
  onDeleted,
}: {
  org: string;
  name: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted?: () => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const del = useMutation(FrameService.method.deleteFrame);
  const [blocking, setBlocking] = useState<string[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setBlocking(null);
      setError(null);
    }
    onOpenChange(next);
  };

  const run = (force: boolean) => {
    setError(null);
    del.mutate(
      { orgSlug: org, name, force },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({ schema: FrameService.method.listFrames, cardinality: "finite" }),
          });
          onDeleted?.();
          handleOpenChange(false);
          navigate("/");
        },
        onError: (err) => {
          const res = mapDeleteError(err);
          if (res.blockingFrames && res.blockingFrames.length > 0) {
            setBlocking(res.blockingFrames);
          } else {
            setError(res.message ?? "Could not delete the frame.");
          }
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-md">
        <DialogTitle>Delete {name}?</DialogTitle>
        {blocking ? (
          <div className="space-y-3 text-sm">
            <p>This frame is inherited by:</p>
            <ul className="list-disc pl-5">{blocking.map((b) => <li key={b}>{b}</li>)}</ul>
            <p className="text-muted-foreground">Deleting anyway detaches these children; they keep their own content.</p>
            <div className="flex justify-end gap-2">
              <DialogClose onClose={() => handleOpenChange(false)} />
              <Button variant="destructive" onClick={() => run(true)} disabled={del.isPending}>Delete anyway</Button>
            </div>
          </div>
        ) : (
          <div className="space-y-3 text-sm">
            <p>This permanently deletes the frame and all its versions.</p>
            {error && <p className="text-destructive">{error}</p>}
            <div className="flex justify-end gap-2">
              <DialogClose onClose={() => handleOpenChange(false)} />
              <Button variant="destructive" onClick={() => run(false)} disabled={del.isPending}>Delete</Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
