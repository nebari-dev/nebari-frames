import { Link } from "react-router";
import { useQuery } from "@connectrpc/connect-query";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { CopyPlus, Pencil, Plus } from "lucide-react";
import { SectionHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
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

// Frames flagged "Offer as a template" - the org-authored entries in the
// authoring form's "start from a template" picker. Built-in starter templates
// ship with the app and are always available alongside these.
export function AdminTemplatesPage() {
  const framesQ = useQuery(FrameService.method.listFrames, {});

  // Both derived before the early return: hooks must run in the same order on
  // every render, including the error render.
  const templates = (framesQ.data?.frames ?? []).filter((f) => f.isTemplate);
  const pagination = usePagination(templates);

  if (framesQ.error)
    return <p className="text-destructive-foreground">Could not load templates.</p>;

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Templates"
        description={
          <>
            Frames offered as starting points when someone creates a new Frame. A Frame
            becomes a template via &quot;Offer as a template&quot; on its edit page.
          </>
        }
        actions={
          framesQ.data?.canCreate && (
            <Button render={<Link to="/frames/new?template=1" />}>
              <Plus />
              New template
            </Button>
          )
        }
      />

      {framesQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : templates.length === 0 ? (
        <Card className="p-10 text-center text-sm text-muted-foreground">
          No templates yet. Publish a Frame with &quot;Offer as a template&quot; checked, and it
          will appear here and in the new-Frame template picker.
        </Card>
      ) : (
        <Table aria-label="Org templates">
          <DataTableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
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
                  {f.description && (
                    <div className="line-clamp-1 text-xs text-muted-foreground">
                      {f.description}
                    </div>
                  )}
                </TableCell>
                <TableCell>
                  <Badge variant="secondary" className="font-mono">
                    v{f.latestVersion}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {f.updatedAt ? timestampDate(f.updatedAt).toLocaleDateString() : ""}
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={`Use ${f.name} as a template`}
                    title="Use as template"
                    className="text-muted-foreground hover:text-foreground"
                    render={<Link to={`/frames/new?from=${f.orgSlug}/${f.name}`} />}
                  >
                    <CopyPlus />
                  </Button>
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
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      <TablePagination pagination={pagination} label="templates" />
    </div>
  );
}
