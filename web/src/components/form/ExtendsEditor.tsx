import { useFormContext, useFieldArray, Controller } from "react-hook-form";
import { FramePicker } from "./FramePicker";
import { Button } from "@/components/ui/button";
import { FieldError } from "./FieldError";

export function ExtendsEditor() {
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: "extends" });
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <div key={field.id} className="space-y-1">
          <div className="flex items-center gap-2">
            <Controller
              control={control}
              name={`extends.${i}`}
              render={({ field: f }) => (
                <FramePicker value={f.value} onChange={f.onChange} withVersion />
              )}
            />
            <Button type="button" variant="ghost" size="sm" aria-label={`remove parent ${i}`} onClick={() => remove(i)}>
              Remove
            </Button>
          </div>
          {/* An imported .frame.md may carry a bare, unpinned `inherits` ref;
              both halves of the failure surface here so it can be fixed with
              the picker instead of blocking the import. */}
          <FieldError name={`extends.${i}.ref`} />
          <FieldError name={`extends.${i}.version`} />
        </div>
      ))}
      <Button type="button" variant="outline" onClick={() => append({ ref: "", version: "" })}>
        + Add parent Frame
      </Button>
    </div>
  );
}
