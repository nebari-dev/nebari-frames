import { useFormContext, useFieldArray, Controller } from "react-hook-form";
import { FramePicker } from "./FramePicker";
import { Button } from "@/components/ui/button";

export function ExcludesEditor() {
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: "excludes" } as never);
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <div key={field.id} className="flex items-center gap-2">
          <Controller
            control={control}
            name={`excludes.${i}`}
            render={({ field: f }) => (
              <FramePicker
                value={{ ref: f.value ?? "", version: "" }}
                onChange={(v) => f.onChange(v.ref)}
              />
            )}
          />
          <Button type="button" variant="ghost" size="sm" aria-label={`remove exclusion ${i}`} onClick={() => remove(i)}>
            Remove
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" onClick={() => append("" as never)}>
        + Add exclusion
      </Button>
    </div>
  );
}
