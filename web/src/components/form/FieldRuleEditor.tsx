import { useFormContext } from "react-hook-form";
import { Requirement } from "@gen/frames/v1/frame_pb";

// One slot's rule: how strongly the template wants it, and what belongs there.
// The note matters as much as the level - it is what the seeded authoring form
// shows in place of the generic hint, and what an MCP client asks as a question.
const LEVELS: { value: Requirement; label: string; help: string }[] = [
  { value: Requirement.OPTIONAL, label: "Optional", help: "Not offered up front." },
  { value: Requirement.RECOMMENDED, label: "Recommended", help: "Shown, but may be left empty." },
  { value: Requirement.REQUIRED, label: "Required", help: "A new Frame cannot be published without it." },
];

export function FieldRuleEditor({ slotKey, label }: { slotKey: string; label: string }) {
  const { register, watch, setValue } = useFormContext();
  const level = (watch(`rules.${slotKey}.level`) as Requirement) ?? Requirement.OPTIONAL;
  return (
    <section className="space-y-2 border-t border-border pt-4">
      <h3 className="text-sm font-medium">{label}</h3>
      <div role="radiogroup" aria-label={`${label} requirement`} className="flex flex-wrap gap-4">
        {LEVELS.map((opt) => (
          <label key={opt.label} className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name={`rules.${slotKey}.level`}
              aria-label={opt.label}
              checked={level === opt.value}
              onChange={() => setValue(`rules.${slotKey}.level`, opt.value, { shouldDirty: true })}
            />
            <span>{opt.label}</span>
            <span className="text-xs text-muted-foreground">{opt.help}</span>
          </label>
        ))}
      </div>
      <label className="block space-y-1">
        <span className="text-xs text-muted-foreground">Note for authors</span>
        <input
          type="text"
          aria-label={`${label} note`}
          className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          placeholder="What belongs in this section?"
          {...register(`rules.${slotKey}.note`)}
        />
      </label>
    </section>
  );
}
