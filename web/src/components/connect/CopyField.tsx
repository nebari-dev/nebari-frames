import { useState } from "react";
import { Button } from "@/components/ui/button";

export function CopyField({
  label,
  value,
  copyLabel = "Copy",
}: {
  label?: string;
  value: string;
  copyLabel?: string;
}) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="space-y-2">
      {label && <div className="font-medium">{label}</div>}
      <code className="block text-xs bg-muted rounded px-2 py-1 break-all">{value}</code>
      <Button
        size="sm"
        variant="secondary"
        onClick={() => {
          void navigator.clipboard.writeText(value);
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
        }}
      >
        {copied ? "Copied" : copyLabel}
      </Button>
    </div>
  );
}
