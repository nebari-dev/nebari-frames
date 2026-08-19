import { useFormContext } from "react-hook-form";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md space-y-4">
        <DialogTitle>Publish version</DialogTitle>

        {versionFromSource ? (
          <p className="text-sm text-muted-foreground">
            The version number is taken from the document&apos;s <code>version:</code> frontmatter.
          </p>
        ) : (
          <label className="block space-y-1">
            <span className="text-sm font-medium">Version</span>
            <Input
              {...register("version")}
              placeholder="1.0.0"
              className="w-40 font-mono"
              {...errorProps("version", versionError)}
            />
          </label>
        )}
        <FieldError name="version" />

        <label className="block space-y-1">
          <span className="text-sm font-medium">Changelog</span>
          <Textarea rows={3} {...register("changelog")} placeholder="What changed in this version?" />
        </label>

        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" loading={pending} onClick={onConfirm}>
            Publish
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
