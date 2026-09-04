import { useFormContext } from "react-hook-form";
import { useId } from "react";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { FieldError, useFieldError, errorProps } from "@/components/form/FieldError";

// Version and changelog are publish-time decisions, not document content, so
// they live here instead of as permanent form fields. The dialog stays open on
// a publish failure so a version conflict (AlreadyExists -> error on `version`)
// is corrected where it happened.
export function PublishDialog({
  open,
  onOpenChange,
  onConfirm,
  pending,
  // In Markdown mode the version comes from the document's own frontmatter,
  // so offering a second input here would just create two sources of truth.
  versionFromSource,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  pending: boolean;
  versionFromSource?: boolean;
}) {
  const { register } = useFormContext();
  const versionError = useFieldError("version");
  const versionId = useId();
  const changelogId = useId();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Publish version</DialogTitle>
          <DialogDescription>
            Publishing adds a new immutable version to this Frame&apos;s history.
          </DialogDescription>
        </DialogHeader>

        {versionFromSource ? (
          <p className="text-sm text-muted-foreground">
            The version number is taken from the document&apos;s <code>version:</code>{" "}
            frontmatter.
          </p>
        ) : (
          <div className="space-y-1.5">
            <Label htmlFor={versionId}>Version</Label>
            <Input
              id={versionId}
              {...register("version")}
              placeholder="1.0.0"
              className="w-40 font-mono"
              {...errorProps("version", versionError)}
            />
          </div>
        )}
        <FieldError name="version" />

        <div className="space-y-1.5">
          <Label htmlFor={changelogId}>Changelog</Label>
          <Textarea
            id={changelogId}
            rows={3}
            {...register("changelog")}
            placeholder="What changed in this version?"
          />
        </div>

        <DialogFooter className="pt-2">
          <DialogClose render={<Button type="button" variant="outline" />}>
            Cancel
          </DialogClose>
          <Button type="button" loading={pending} onClick={onConfirm}>
            Publish
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
