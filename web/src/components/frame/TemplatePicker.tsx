import type { FrameTemplateSummary } from "@gen/frames/v1/frame_pb";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertAction } from "@/components/ui/alert";

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
  failed,
  onRetry,
}: {
  templates: FrameTemplateSummary[];
  onPick: (id: string) => void;
  // Whether the list failed to load. Without this a failed list renders as the
  // heading, the intro, and no cards - a screen that looks like an answer
  // ("there are no templates") to a question that was never answered.
  //
  // A flag rather than the server's message: read failures across this app
  // state a fixed sentence, because the text on a failed read is storage or
  // wiring detail and retrying is the only thing the reader can act on.
  failed?: boolean;
  onRetry?: () => void;
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
      {failed ? (
        <Alert variant="destructive">
          {/* No cause is stated: the client cannot tell an unreachable
              registry from one that answered with an error, and the case that
              prompted this - a database missing the templates table - is the
              second. */}
          <AlertTitle>Templates could not be loaded</AlertTitle>
          {onRetry && (
            <AlertAction>
              <Button type="button" variant="outline" size="sm" onClick={onRetry}>
                Try again
              </Button>
            </AlertAction>
          )}
        </Alert>
      ) : (
        <>
          <Group heading="Your organization" templates={templates.filter((t) => !t.builtin)} onPick={onPick} />
          <Group heading="Built-in" templates={templates.filter((t) => t.builtin)} onPick={onPick} />
        </>
      )}
    </div>
  );
}
