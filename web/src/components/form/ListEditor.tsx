import { useFormContext, useFieldArray } from "react-hook-form";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { FieldError, useFieldError, errorProps } from "./FieldError";

// Singular label for the add button ("Rules" -> "rule").
function singular(label: string): string {
  return label.replace(/s$/i, "").toLowerCase();
}

function Row({ name, label, index, onRemove }: { name: string; label: string; index: number; onRemove: () => void }) {
  const { register } = useFormContext();
  const path = `${name}.${index}`;
  const error = useFieldError(path);
  return (
    <div className="space-y-1">
      <div className="flex items-start gap-2">
        <Textarea className="min-h-10 flex-1" {...register(path)} {...errorProps(path, error)} />
        <Button type="button" variant="ghost" size="sm" aria-label={`remove ${singular(label)} ${index}`} onClick={onRemove}>
          Remove
        </Button>
      </div>
      <FieldError name={path} />
    </div>
  );
}

export function ListEditor({ name, label }: { name: `slots.${"rules" | "skills" | "prompts"}`; label: string }) {
  const { control } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name } as never);
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <Row key={field.id} name={name} label={label} index={i} onRemove={() => remove(i)} />
      ))}
      {/* A violation on the slot itself (rather than a row) has nowhere else to go. */}
      <FieldError name={name} />
      <Button type="button" variant="outline" onClick={() => append("" as never)}>
        + Add {singular(label)}
      </Button>
    </div>
  );
}
