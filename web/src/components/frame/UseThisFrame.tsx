import { Link } from "react-router";
import { CopyField } from "@/components/connect/CopyField";

export function UseThisFrame({ org, name }: { org: string; name: string }) {
  const uri = `${window.location.origin}/mcp#${org}/${name}`;
  return (
    <div className="border rounded-md p-4 space-y-3">
      <div className="font-medium">Use this Frame</div>
      <CopyField label="MCP resource URI" value={uri} copyLabel="Copy MCP URI" />
      <Link to="/connect" className="text-sm text-primary hover:underline">
        Set up a connector -&gt;
      </Link>
    </div>
  );
}
