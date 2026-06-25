import { useFormContext, useFieldArray } from "react-hook-form";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";

// Singular label for the add button ("Rules" -> "rule").
function singular(label: string): string {
  return label.replace(/s$/i, "").toLowerCase();
}

export function ListEditor({ name, label }: { name: `slots.${"rules" | "skills" | "prompts"}`; label: string }) {
  const { control, register } = useFormContext();
  const { fields, append, remove } = useFieldArray({ control, name } as never);
  return (
    <div className="space-y-2">
      {fields.map((field, i) => (
        <div key={field.id} className="flex items-start gap-2">
          <Textarea className="min-h-10 flex-1" {...register(`${name}.${i}` as const)} />
          <Button type="button" variant="ghost" size="sm" aria-label={`remove ${singular(label)} ${i}`} onClick={() => remove(i)}>
            Remove
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={() => append("" as never)}>
        + Add {singular(label)}
      </Button>
    </div>
  );
}
