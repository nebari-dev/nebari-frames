import { useState } from "react";
import { Check, Copy } from "lucide-react";
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
      <div className="relative">
        <code className="block break-all rounded bg-muted px-2 py-2.5 pr-9 text-xs">{value}</code>
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={copied ? "Copied" : copyLabel}
          title={copied ? "Copied" : copyLabel}
          className="absolute right-1 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          onClick={() => {
            void navigator.clipboard.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
          }}
        >
          {copied ? <Check className="text-green-600" /> : <Copy />}
        </Button>
      </div>
    </div>
  );
}
