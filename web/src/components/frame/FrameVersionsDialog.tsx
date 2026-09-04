import { Link } from "react-router";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Frame, FrameVersion, FrameVersionSummary } from "@gen/frames/v1/frame_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

// Version history for a frame: every published version, newest first, with a
// link to view any of them pinned. Restoring happens from the page header
// once a past version is being viewed.
export function FrameVersionsDialog({
  org,
  name,
  frame,
  version,
  versions,
  open,
  onOpenChange,
}: {
  org: string;
  name: string;
  frame: Frame;
  version: FrameVersion;
  versions: FrameVersionSummary[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Versions</DialogTitle>
          <DialogDescription>
            Every published version of {name}, newest first.
          </DialogDescription>
        </DialogHeader>

        {versions.length === 0 ? (
          <p className="py-2 text-sm text-muted-foreground">No versions found.</p>
        ) : (
          <div className="min-h-0 divide-y divide-border overflow-y-auto">
            {versions.map((v) => {
              const isLatest = v.version === frame.latestVersion;
              const isViewing = v.version === version.version;
              return (
                <div key={v.version} className="flex items-center gap-4 py-3 text-sm">
                  <div className="w-20 shrink-0 font-mono">v{v.version}</div>
                  <div className="min-w-0 flex-1 space-y-0.5">
                    <span className="block truncate">
                      {v.changelog || <span className="text-muted-foreground">No changelog</span>}
                    </span>
                    <div className="text-xs text-muted-foreground">
                      {v.publishedBy}
                      {v.publishedAt ? ` - ${timestampDate(v.publishedAt).toLocaleDateString()}` : ""}
                    </div>
                  </div>
                  {/* Status and action sit flush against the row's right edge. */}
                  <div className="flex shrink-0 items-center justify-end gap-2">
                    {isLatest && <Badge variant="outline">Latest</Badge>}
                    {isViewing && <Badge variant="secondary">Viewing</Badge>}
                    {!isViewing && (
                      <Button
                        variant="ghost"
                        size="sm"
                        aria-label={`View v${v.version}`}
                        onClick={() => onOpenChange(false)}
                        render={
                          <Link
                            to={
                              isLatest
                                ? `/frames/${org}/${name}`
                                : `/frames/${org}/${name}?v=${encodeURIComponent(v.version)}`
                            }
                          />
                        }
                      >
                        View
                      </Button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>Close</DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
