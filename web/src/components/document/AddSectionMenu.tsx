import { Plus } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
} from "@/components/ui/dropdown-menu";
import type { SlotSectionDef } from "@/lib/slot-sections";

// The document grows section by section: only sections the author chose (or
// that already carry content) are on the page, and this menu offers the rest.
// Each item explains what the section is for, which doubles as the schema's
// documentation at the moment it is needed.
export function AddSectionMenu({
  available,
  onAdd,
}: {
  available: SlotSectionDef[];
  onAdd: (def: SlotSectionDef) => void;
}) {
  if (available.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger variant="outline">
        <Plus className="size-4" />
        Add section
      </DropdownMenuTrigger>
      <DropdownMenuPortal>
        <DropdownMenuContent align="start" className="w-96">
        {available.map((def) => (
          <DropdownMenuItem key={def.key} onClick={() => onAdd(def)}>
            <div className="flex flex-col items-start gap-0.5 py-0.5">
              <span className="font-medium">{def.label}</span>
              <span className="text-xs text-muted-foreground">{def.hint}</span>
            </div>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
      </DropdownMenuPortal>
    </DropdownMenu>
  );
}
