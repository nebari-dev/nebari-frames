import { useRef, useState } from "react";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/alert";
import { cn } from "@/lib/utils";
import { fillTextareaSlot } from "./fill-height";

// The advanced editor: the frame as a single spec-conformant .frame.md
// document. This is also the import surface, since pasting or dropping a file
// and editing one are the same activity.
export function MarkdownSourceEditor({
  value,
  onChange,
  errors,
  busy,
}: {
  value: string;
  onChange: (next: string) => void;
  errors: string[];
  busy?: boolean;
}) {
  const [dragging, setDragging] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const readFile = (file: File | undefined) => {
    if (!file) return;
    void file.text().then(onChange);
  };

  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-3 rounded-md border border-dashed p-3 transition-colors",
        // The editor fills the height left below the page chrome.
        fillTextareaSlot,
        dragging ? "border-ring bg-accent/40" : "border-transparent",
      )}
      onDragOver={(e) => {
        e.preventDefault();
        setDragging(true);
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        e.preventDefault();
        setDragging(false);
        readFile(e.dataTransfer.files?.[0]);
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">
          Frame Spec v0.2 &mdash; YAML frontmatter, then a free-form Markdown body.
        </p>
        <div className="flex shrink-0 gap-2">
          <input
            ref={fileRef}
            type="file"
            accept=".md,.markdown,text/markdown"
            className="hidden"
            aria-label="Load a .frame.md file"
            onChange={(e) => readFile(e.target.files?.[0])}
          />
          <Button type="button" variant="outline" size="sm" onClick={() => fileRef.current?.click()}>
            Load file
          </Button>
        </div>
      </div>

      {errors.length > 0 && (
        <Alert variant="destructive">
          <ul className="list-disc space-y-1 pl-4 text-sm">
            {errors.map((e) => (
              <li key={e}>{e}</li>
            ))}
          </ul>
        </Alert>
      )}

      <Textarea
        aria-label="Frame markdown source"
        spellCheck={false}
        className="min-h-[20rem] flex-1 font-mono text-xs"
        placeholder={"---\ntype: frame [0.2]\nname: my-frame\ndescription: What this frame is for.\nvisibility: internal\nversion: 1.0.0\n---\n\nThe context this frame carries, as free-form Markdown.\n\nOr drop a .frame.md file here."}
        value={value}
        disabled={busy}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}
