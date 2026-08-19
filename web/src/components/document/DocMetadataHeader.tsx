import { useFormContext, useWatch } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { FieldError, useFieldError, errorProps } from "@/components/form/FieldError";
import { VISIBILITY_VALUES } from "@/lib/frame-yaml";
import { cn } from "@/lib/utils";

// Inputs styled to read as document text until pointed at: the frame's name is
// its title and the description its subtitle, so the editor keeps the shape of
// the page the reader will see. A visible border appears on hover/focus (and on
// error) so the fields stay discoverable as fields.
const quiet =
  "border-transparent bg-transparent shadow-none " +
  "hover:border-input focus-visible:border-input aria-invalid:border-destructive";

// The metadata header of the document editor: title, description, and the
// spec metadata (visibility / scope / maintainer) as one compact row.
export function DocMetadataHeader({ nameReadOnly }: { nameReadOnly: boolean }) {
  const { register, control } = useFormContext();
  // Watched (not getValues) so the async edit-mode prefill re-renders the title.
  const name = useWatch({ control, name: "name" }) as string;
  const nameError = useFieldError("name");
  const descError = useFieldError("description");
  const visibilityError = useFieldError("visibility");

  return (
    <div className="space-y-1">
      {nameReadOnly ? (
        // Identity is fixed after creation; render it as the plain title it is
        // (the page label above is the document's h1).
        <div className="text-2xl font-semibold">{name}</div>
      ) : (
        <div>
          <Input
            {...register("name")}
            aria-label="Frame name"
            placeholder="frame-name"
            className={cn(quiet, "-mx-2 h-auto w-full py-1 font-mono text-2xl font-semibold")}
            {...errorProps("name", nameError)}
          />
          <FieldError name="name" />
        </div>
      )}

      <div>
        <Input
          {...register("description")}
          aria-label="Description"
          placeholder="What is this frame for? One or two sentences."
          className={cn(quiet, "-mx-2 h-auto w-full py-1 text-base text-muted-foreground")}
          {...errorProps("description", descError)}
        />
        <FieldError name="description" />
      </div>

      <div className="flex flex-wrap items-end gap-x-4 gap-y-2 pt-2">
        <label className="block space-y-0.5">
          <span className="text-xs font-medium text-muted-foreground">Visibility</span>
          <Select
            {...register("visibility")}
            title="Declared intent that travels with the frame. Access is still governed by this org's roles and grants."
            className="h-8 w-32 text-xs"
            {...errorProps("visibility", visibilityError)}
          >
            {VISIBILITY_VALUES.map((v) => (
              <option key={v} value={v}>
                {v}
              </option>
            ))}
          </Select>
        </label>
        <label className="block space-y-0.5">
          <span className="text-xs font-medium text-muted-foreground">Scope</span>
          <Input {...register("scope")} placeholder="company" className="h-8 w-36 text-xs" />
        </label>
        <label className="block space-y-0.5">
          <span className="text-xs font-medium text-muted-foreground">Maintainer</span>
          <Input {...register("maintainer")} placeholder="team or person" className="h-8 w-44 text-xs" />
        </label>
      </div>
      <FieldError name="visibility" />
    </div>
  );
}
