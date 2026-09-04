import { useState } from "react";
import { Link } from "react-router";
import { Pencil, Trash2 } from "lucide-react";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import type { FrameSummary } from "@gen/frames/v1/frame_pb";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from "@/components/ui/table";
import { DataTableHeader } from "@/components/table/DataTableHeader";
import { TablePagination } from "@/components/table/TablePagination";
import { usePagination } from "@/hooks/use-pagination";
import { DeleteFrameDialog } from "@/components/frame/DeleteFrameDialog";

function fmtDate(ts?: Timestamp): string {
  if (!ts) return "—";
  return timestampDate(ts).toLocaleDateString();
}

export function FramesTable({ frames }: { frames: FrameSummary[] }) {
  const [deleteTarget, setDeleteTarget] = useState<{ org: string; name: string } | null>(null);
  const pagination = usePagination(frames);

  return (
    <div className="space-y-4">
      <Table aria-label="Frames">
        <DataTableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Description</TableHead>
            <TableHead>Owner</TableHead>
            <TableHead>Version</TableHead>
            <TableHead>Updated</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </DataTableHeader>
        <TableBody>
          {pagination.pageItems.map((f) => (
            <TableRow key={`${f.orgSlug}/${f.name}`}>
              <TableCell>
                <Link
                  to={`/frames/${f.orgSlug}/${f.name}`}
                  className="font-medium text-foreground hover:underline"
                >
                  {f.name}
                </Link>
              </TableCell>
              <TableCell className="max-w-md text-muted-foreground">
                <span className="line-clamp-1">{f.description}</span>
              </TableCell>
              <TableCell className="text-muted-foreground">{f.ownerSub}</TableCell>
              <TableCell>
                <Badge variant="secondary" className="font-mono">
                  v{f.latestVersion}
                </Badge>
              </TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {fmtDate(f.updatedAt)}
              </TableCell>
              <TableCell className="whitespace-nowrap text-right">
                {f.permissions?.canEdit && (
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Edit ${f.name}`}
                    title="Edit"
                    className="text-muted-foreground hover:text-foreground"
                    render={<Link to={`/frames/${f.orgSlug}/${f.name}/edit`} />}
                  >
                    <Pencil />
                  </Button>
                )}
                {f.permissions?.canDelete && (
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Delete ${f.name}`}
                    title="Delete"
                    className="text-muted-foreground hover:text-destructive-foreground"
                    onClick={() => setDeleteTarget({ org: f.orgSlug, name: f.name })}
                  >
                    <Trash2 />
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <TablePagination pagination={pagination} label="frames" />

      {deleteTarget && (
        <DeleteFrameDialog
          org={deleteTarget.org}
          name={deleteTarget.name}
          open
          onOpenChange={(o) => !o && setDeleteTarget(null)}
          onDeleted={() => setDeleteTarget(null)}
        />
      )}
    </div>
  );
}
