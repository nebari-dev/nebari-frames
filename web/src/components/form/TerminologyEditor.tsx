import { useFormContext, useFieldArray } from "react-hook-form";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FieldError, useFieldError, errorProps } from "./FieldError";

function Row({ index, onRemove }: { index: number; onRemove: () => void }) {
  const { register } = useFormContext();
  const termPath = `slots.terminology.${index}.term`;
  const defPath = `slots.terminology.${index}.definition`;
  const termError = useFieldError(termPath);
  const defError = useFieldError(defPath);
  return (
    <div className="space-y-1">
      <div className="flex items-center gap-2">
        <Input placeholder="Term" className="w-48" {...register(termPath)} {...errorProps(termPath, termError)} />
        <Input placeholder="Definition" className="flex-1" {...register(defPath)} {...errorProps(defPath, defError)} />
        <Button type="button" variant="ghost" size="sm" aria-label={`remove term ${index}`} onClick={onRemove}>
          Remove
        </Button>
      </div>
      <FieldError name={termPath} />
      <FieldError name={defPath} />
    </div>
  );
}

export function TerminologyEditor() {
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name: "slots.terminology" });
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <Row key={field.id} index={i} onRemove={() => remove(i)} />
      ))}
      {/* Duplicate-term detection is a refinement on the array, not on a row. */}
      <FieldError name="slots.terminology" />
      <Button type="button" variant="outline" onClick={() => append({ term: "", definition: "" })}>
        + Add term
      </Button>
    </div>
  );
}
