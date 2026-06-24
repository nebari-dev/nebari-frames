import { useState } from "react";
import { Button } from "@/components/ui/button";

// Connect (per-provider) pages arrive in Web-3; for now show the MCP resource
// URI so users who know what to do with it can copy it.
export function UseThisFrame({ org, name }: { org: string; name: string }) {
  const uri = `${window.location.origin}/mcp#${org}/${name}`;
  const [copied, setCopied] = useState(false);
  return (
    <div className="border rounded-md p-4 space-y-2">
      <div className="font-medium">Use this Frame</div>
      <code className="block text-xs bg-muted rounded px-2 py-1 break-all">{uri}</code>
      <Button
        size="sm"
        variant="secondary"
        onClick={() => {
          void navigator.clipboard.writeText(uri);
          setCopied(true);
        }}
      >
        {copied ? "Copied" : "Copy MCP URI"}
      </Button>
    </div>
  );
}
