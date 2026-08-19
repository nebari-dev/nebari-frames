import { useState } from "react";
import { useMutation } from "@connectrpc/connect-query";
import { Download } from "lucide-react";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
} from "@/components/ui/dropdown-menu";

// Exports the frame as a spec-conformant .frame.md file. The conversion runs
// server-side through the same codec the Markdown editor uses, so what a
// reader downloads is exactly what an author would have been editing. One menu
// instead of two buttons keeps the page header to a single row of actions.
export function ExportMenu({ name, content }: { name: string; content: Uint8Array }) {
  const convert = useMutation(FrameService.method.convertFrame);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const withMarkdown = (then: (md: string) => void) => {
    setError(null);
    convert.mutate(
      { source: { case: "yaml", value: content } },
      {
        onSuccess: (res) => then(new TextDecoder().decode(res.markdown)),
        onError: () => setError("Could not export this frame."),
      },
    );
  };

  const download = () =>
    withMarkdown((md) => {
      const url = URL.createObjectURL(new Blob([md], { type: "text/markdown" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = `${name}.frame.md`;
      a.click();
      URL.revokeObjectURL(url);
    });

  const copy = () =>
    withMarkdown((md) => {
      void navigator.clipboard.writeText(md);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });

  return (
    <div className="flex flex-col items-end gap-1">
      <DropdownMenu>
        <DropdownMenuTrigger variant="outline" disabled={convert.isPending}>
          <Download className="size-4" />
          Export
        </DropdownMenuTrigger>
        <DropdownMenuPortal>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onClick={download}>Download .frame.md</DropdownMenuItem>
          <DropdownMenuItem onClick={copy}>Copy as Markdown</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuPortal>
      </DropdownMenu>
      {error && <span className="text-xs text-destructive">{error}</span>}
      {copied && <span className="text-xs text-muted-foreground" role="status">Copied to clipboard</span>}
    </div>
  );
}
