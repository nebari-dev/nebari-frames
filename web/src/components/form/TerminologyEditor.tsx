import { useFormContext, useFieldArray } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export function TerminologyEditor() {
  const { control, register } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: "slots.terminology" });
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <div key={field.id} className="flex items-center gap-2">
          <Input placeholder="Term" {...register(`slots.terminology.${i}.term` as const)} className="w-48" />
          <Input placeholder="Definition" {...register(`slots.terminology.${i}.definition` as const)} className="flex-1" />
          <Button type="button" variant="ghost" size="sm" aria-label={`remove term ${i}`} onClick={() => remove(i)}>
            Remove
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" onClick={() => append({ term: "", definition: "" })}>
        + Add term
      </Button>
    </div>
  );
}
