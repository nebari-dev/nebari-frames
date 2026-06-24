import type { FrameDoc } from "@/lib/frame-yaml";
import { MarkdownView } from "@/components/MarkdownView";
import { SlotSection } from "./SlotSection";
import { TerminologyList } from "./TerminologyList";
import { BulletList } from "./BulletList";

// Prose slots in schema order, with display labels.
const PROSE: { key: keyof FrameDoc["slots"]; label: string }[] = [
  { key: "tool_specs", label: "Tool Specifications" },
  { key: "goals", label: "Goals" },
  { key: "style", label: "Style" },
  { key: "norms", label: "Norms" },
  { key: "architecture", label: "Architecture" },
  { key: "business_process", label: "Business Process" },
];

export function FrameSlots({ doc }: { doc: FrameDoc }) {
  const s = doc.slots;
  return (
    <div>
      {s.terminology && s.terminology.length > 0 && (
        <SlotSection title="Terminology"><TerminologyList terms={s.terminology} /></SlotSection>
      )}
      {s.rules && s.rules.length > 0 && (
        <SlotSection title="Rules"><BulletList items={s.rules} /></SlotSection>
      )}
      {s.skills && s.skills.length > 0 && (
        <SlotSection title="Skills"><BulletList items={s.skills} /></SlotSection>
      )}
      {s.prompts && s.prompts.length > 0 && (
        <SlotSection title="Prompts"><BulletList items={s.prompts} /></SlotSection>
      )}
      {PROSE.map(({ key, label }) => {
        const val = s[key];
        if (typeof val !== "string" || val.trim() === "") return null;
        return <SlotSection key={key} title={label}><MarkdownView source={val} /></SlotSection>;
      })}
    </div>
  );
}
