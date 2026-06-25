import { useState } from "react";
import { useFormContext, useWatch } from "react-hook-form";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { MarkdownView } from "@/components/MarkdownView";

export function MarkdownField({ name, label }: { name: `slots.${string}`; label: string }) {
  const { register, control } = useFormContext();
  const [preview, setPreview] = useState(false);
  const value = useWatch({ control, name }) as string | undefined;
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{label}</span>
        <Button type="button" variant="ghost" size="sm" onClick={() => setPreview((p) => !p)}>
          {preview ? "Edit" : "Preview"}
        </Button>
      </div>
      {preview ? (
        <div className="rounded-md border border-border p-3">
          <MarkdownView source={value ?? ""} />
        </div>
      ) : (
        <Textarea rows={6} {...register(name)} />
      )}
    </div>
  );
}
