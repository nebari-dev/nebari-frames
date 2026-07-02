import { useFormContext, useFieldArray, Controller } from "react-hook-form";
import { FramePicker } from "./FramePicker";
import { Button } from "@/components/ui/button";

export function ExtendsEditor() {
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: "extends" });
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <div key={field.id} className="flex items-center gap-2">
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
      ))}
      <Button type="button" variant="outline" onClick={() => append({ ref: "", version: "" })}>
        + Add parent Frame
      </Button>
    </div>
  );
}
