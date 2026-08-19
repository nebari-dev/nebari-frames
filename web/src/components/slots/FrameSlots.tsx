import type { FrameDoc } from "@/lib/frame-yaml";
import { SLOT_SECTIONS, sectionHasContent } from "@/lib/slot-sections";
import { MarkdownView } from "@/components/MarkdownView";
import { SlotSection } from "./SlotSection";
import { TerminologyList } from "./TerminologyList";
import { BulletList } from "./BulletList";

// Renders the populated sections of a frame in canonical order, hiding the
// empty ones - the reader sees the document, not the schema.
export function FrameSlots({ doc }: { doc: FrameDoc }) {
  const s = doc.slots;
  return (
    <div className="divide-y">
      {SLOT_SECTIONS.filter((def) => sectionHasContent(def, s)).map((def) => (
        <SlotSection key={def.key} title={def.label}>
          {def.kind === "terms" && <TerminologyList terms={s.terminology ?? []} />}
          {def.kind === "list" && <BulletList items={(s[def.key] as string[]) ?? []} />}
          {def.kind === "prose" && <MarkdownView source={(s[def.key] as string) ?? ""} />}
        </SlotSection>
      ))}
    </div>
  );
}
