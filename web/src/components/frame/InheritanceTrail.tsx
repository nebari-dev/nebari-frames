import { Link } from "react-router";
import type { ParentRef } from "@gen/frames/v1/frame_pb";

// parent refs are "org_slug/frame_name"; link each to its detail page.
export function InheritanceTrail({ parents }: { parents: ParentRef[] }) {
  if (parents.length === 0) return null;
  return (
    <div className="text-sm">
      <div className="font-medium mb-1">Inherits from</div>
      <ul className="space-y-1">
        {parents.map((p) => (
          <li key={`${p.ref}@${p.version}`}>
            <Link to={`/frames/${p.ref}`} className="underline">{p.ref}</Link>
            <span className="text-muted-foreground"> @ {p.version}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
