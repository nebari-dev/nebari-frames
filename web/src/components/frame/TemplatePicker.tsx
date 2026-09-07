import type { FrameTemplateSummary } from "@gen/frames/v1/frame_pb";
import { Button } from "@/components/ui/button";

// Org templates and built-ins are shown in separate groups rather than merged
// or shadowed. An org template sharing a built-in's title sits beside it: a
// precedence rule would be a permanent source of "why am I seeing the wrong
// one", and grouping answers the question visually instead.
function Group({
  heading,
  templates,
  onPick,
}: {
  heading: string;
  templates: FrameTemplateSummary[];
  onPick: (id: string) => void;
}) {
  if (templates.length === 0) return null;
  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-muted-foreground">{heading}</h2>
      <div className="grid gap-3 sm:grid-cols-2">
        {templates.map((t) => (
          <Button
            key={t.id}
            type="button"
            variant="outline"
            className="h-auto flex-col items-start gap-1 whitespace-normal p-4 text-left"
            onClick={() => onPick(t.id)}
          >
            <span className="font-medium">{t.title}</span>
            <span className="text-xs font-normal text-muted-foreground">{t.description}</span>
          </Button>
        ))}
      </div>
    </section>
  );
}

export function TemplatePicker({
  templates,
  onPick,
}: {
  templates: FrameTemplateSummary[];
  onPick: (id: string) => void;
}) {
  return (
    <div className="mx-auto max-w-3xl space-y-8 py-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-semibold">What kind of Frame is this?</h1>
        <p className="text-sm text-muted-foreground">
          A template gives you a starting shape. Everything it fills in stays fully editable, and a
          Frame keeps no link to the template it started from.
        </p>
      </div>
      <Group heading="Your organization" templates={templates.filter((t) => !t.builtin)} onPick={onPick} />
      <Group heading="Built-in" templates={templates.filter((t) => t.builtin)} onPick={onPick} />
    </div>
  );
}
