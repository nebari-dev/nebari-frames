import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/collapsible";

export function SlotSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Collapsible defaultOpen className="py-4">
      <CollapsibleTrigger className="w-full text-left text-lg font-semibold">{title}</CollapsibleTrigger>
      <CollapsibleContent className="pt-2">{children}</CollapsibleContent>
    </Collapsible>
  );
}
