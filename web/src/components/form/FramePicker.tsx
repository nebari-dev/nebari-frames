import { useState } from "react";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

export interface ExtendValue {
  ref: string;
  version: string;
}

export function FramePicker({
  value,
  onChange,
  withVersion = false,
}: {
  value: ExtendValue;
  onChange: (v: ExtendValue) => void;
  withVersion?: boolean;
}) {
  const [query, setQuery] = useState("");
  const { data } = useQuery(FrameService.method.listFrames, {});
  const matches = (data?.frames ?? [])
    .map((f) => `${f.orgSlug}/${f.name}`)
    .filter((ref) => ref.toLowerCase().includes(query.toLowerCase()))
    .slice(0, 8);

  const [org = "", name = ""] = value.ref.split("/");
  const versionsQ = useQuery(
    FrameService.method.listFrameVersions,
    { orgSlug: org, name },
    { enabled: withVersion && value.ref !== "" },
  );

  return (
    <div className="flex flex-1 items-start gap-2">
      <div className="relative flex-1">
        <Input
          placeholder="Search frames to inherit..."
          value={value.ref || query}
          onChange={(e) => {
            setQuery(e.target.value);
            onChange({ ref: "", version: "" });
          }}
        />
        {query !== "" && value.ref === "" && matches.length > 0 && (
          <ul className="absolute z-10 mt-1 w-full rounded-md border border-border bg-background shadow">
            {matches.map((ref) => (
              <li key={ref}>
                <button
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-sm hover:bg-muted"
                  onClick={() => {
                    onChange({ ref, version: "" });
                    setQuery("");
                  }}
                >
                  {ref}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      {withVersion && (
        <Select
          aria-label="version"
          className="w-32"
          value={value.version}
          disabled={value.ref === ""}
          onChange={(e) => onChange({ ...value, version: e.target.value })}
        >
          <option value="">version...</option>
          {(versionsQ.data?.versions ?? []).map((v) => (
            <option key={v.version} value={v.version}>{v.version}</option>
          ))}
        </Select>
      )}
    </div>
  );
}
