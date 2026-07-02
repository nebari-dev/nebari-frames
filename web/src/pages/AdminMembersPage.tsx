import { useState } from "react";
import { useQuery, useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { UserPlus, Trash2 } from "lucide-react";
import { AddMemberDialog } from "@/components/member/AddMemberDialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";

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

  if (membersQ.error) return <p className="text-destructive">Could not load members.</p>;

  const members = membersQ.data?.members ?? [];

  return (
    <div className="space-y-6 motion-safe:animate-fade-in">
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">Members</h1>
          <p className="text-sm text-muted-foreground">
            Invite people to your organization and manage their access.
          </p>
        </div>
        <Button onClick={() => setAddOpen(true)}>
          <UserPlus className="size-4" />
          Add member
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {membersQ.isLoading ? (
        <Skeleton className="h-48 w-full" />
      ) : members.length === 0 ? (
        <Card className="p-10 text-center text-sm text-muted-foreground">No members yet.</Card>
      ) : (
        <Card className="overflow-hidden p-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/50 text-left text-xs uppercase tracking-wide text-muted-foreground">
                <th className="px-4 py-3 font-medium">Member</th>
                <th className="px-4 py-3 font-medium">Role</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody>
              {members.map((m) => {
                const key = m.userSub || m.email;
                const target = m.userSub ? { userSub: m.userSub } : { email: m.email };
                const label = m.email || m.userSub;
                const active = Boolean(m.userSub);
                return (
                  <tr
                    key={key}
                    className="border-b border-border last:border-0 transition-colors hover:bg-muted/40"
                  >
                    <td className="px-4 py-3 font-medium text-foreground">{label}</td>
                    <td className="px-4 py-3">
                      <Select
                        aria-label={`role for ${label}`}
                        value={m.role}
                        className="w-36"
                        onChange={(e) =>
                          setRoleM.mutate(
                            { ...target, role: e.target.value },
                            {
                              onSuccess: () => void invalidate(),
                              onError: (err) => setError(ConnectError.from(err).rawMessage),
                            },
                          )
                        }
                      >
                        {ROLES.map((r) => (
                          <option key={r} value={r}>
                            {r}
                          </option>
                        ))}
                      </Select>
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={active ? "secondary" : "outline"}>
                        {active ? "active" : "pending"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-muted-foreground hover:text-destructive-foreground"
                        onClick={() => {
                          if (!window.confirm(`Remove ${label}?`)) return;
                          remove.mutate(target, {
                            onSuccess: () => void invalidate(),
                            onError: (err) => setError(ConnectError.from(err).rawMessage),
                          });
                        }}
                      >
                        <Trash2 className="size-4" />
                        Remove
                      </Button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </Card>
      )}

      <AddMemberDialog open={addOpen} onOpenChange={setAddOpen} />
    </div>
  );
}
