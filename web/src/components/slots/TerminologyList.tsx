import type { FrameDoc } from "@/lib/frame-yaml";

type Term = NonNullable<FrameDoc["slots"]["terminology"]>[number];

export function TerminologyList({ terms }: { terms: Term[] }) {
  return (
    <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1">
      {terms.map((t) => (
        <div key={t.term} className="contents">
          <dt className="font-mono text-sm">{t.term}</dt>
          <dd className="text-sm text-muted-foreground">{t.definition}</dd>
        </div>
      ))}
    </dl>
  );
}
