import { useState } from "react";
import { useNavigate } from "react-router";
import { useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
        <DialogHeader>
          <DialogTitle>Delete {name}?</DialogTitle>
          <DialogDescription>
            {blocking
              ? "Deleting anyway detaches these children; they keep their own content."
              : "This permanently deletes the frame and all its versions."}
          </DialogDescription>
        </DialogHeader>

        {blocking && (
          <div className="space-y-2 text-sm">
            <p>This frame is inherited by:</p>
            <ul className="list-disc pl-5 text-muted-foreground">
              {blocking.map((b) => (
                <li key={b}>{b}</li>
              ))}
            </ul>
          </div>
        )}
        {error && <p className="text-sm text-destructive-foreground">{error}</p>}

        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
          <Button
            variant="destructive"
            loading={del.isPending}
            onClick={() => run(Boolean(blocking))}
          >
            {blocking ? "Delete anyway" : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
