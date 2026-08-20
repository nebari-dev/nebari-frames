import { useState } from "react";
import { useQuery, useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { UserPlus, Trash2 } from "lucide-react";
import { AddMemberDialog } from "@/components/member/AddMemberDialog";
import { SectionHeader } from "@/components/layout/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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

const ROLES = ["viewer", "publisher", "admin"] as const;

export function AdminMembersPage() {
  const queryClient = useQueryClient();
  const membersQ = useQuery(FrameService.method.listOrgMembers, {});
  const [addOpen, setAddOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const invalidate = () =>
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({ schema: FrameService.method.listOrgMembers, cardinality: "finite" }),
    });

  const setRoleM = useMutation(FrameService.method.setMemberRole);
  const remove = useMutation(FrameService.method.removeOrgMember);

  // Both derived before the early return: hooks must run in the same order on
  // every render, including the error render.
  const members = membersQ.data?.members ?? [];
  const pagination = usePagination(members);

  if (membersQ.error)
    return <p className="text-destructive-foreground">Could not load members.</p>;

  return (
    <div className="space-y-6">
      <SectionHeader
        title="Members"
        description="Invite people to your organization and manage their access."
        actions={
          <Button onClick={() => setAddOpen(true)}>
            <UserPlus />
            Add member
          </Button>
        }
      />

      {error && <p className="text-sm text-destructive-foreground">{error}</p>}

      {membersQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : members.length === 0 ? (
        <Card className="p-10 text-center text-sm text-muted-foreground">No members yet.</Card>
      ) : (
        <Table aria-label="Organization members">
          <DataTableHeader>
            <TableRow>
              <TableHead>Member</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </DataTableHeader>
          <TableBody>
            {pagination.pageItems.map((m) => {
              const key = m.userSub || m.email;
              const target = m.userSub ? { userSub: m.userSub } : { email: m.email };
              const label = m.email || m.userSub;
              const active = Boolean(m.userSub);
              return (
                <TableRow key={key}>
                  <TableCell className="font-medium text-foreground">{label}</TableCell>
                  <TableCell>
                    <Select
                      value={m.role}
                      onValueChange={(v) =>
                        setRoleM.mutate(
                          { ...target, role: String(v) },
                          {
                            onSuccess: () => void invalidate(),
                            onError: (err) => setError(ConnectError.from(err).rawMessage),
                          },
                        )
                      }
                    >
                      <SelectTrigger aria-label={`role for ${label}`} className="w-36">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {ROLES.map((r) => (
                          <SelectItem key={r} value={r}>
                            {r}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell>
                    <Badge variant={active ? "secondary" : "outline"}>
                      {active ? "active" : "pending"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Remove ${label}`}
                      title="Remove"
                      className="text-muted-foreground hover:text-destructive-foreground"
                      onClick={() => {
                        if (!window.confirm(`Remove ${label}?`)) return;
                        remove.mutate(target, {
                          onSuccess: () => void invalidate(),
                          onError: (err) => setError(ConnectError.from(err).rawMessage),
                        });
                      }}
                    >
                      <Trash2 />
                    </Button>
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}

      <TablePagination pagination={pagination} label="members" />

      <AddMemberDialog open={addOpen} onOpenChange={setAddOpen} />
    </div>
  );
}
