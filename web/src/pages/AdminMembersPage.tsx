import { useState } from "react";
import { useQuery, useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Button } from "@/components/ui/button";

const ROLES = ["viewer", "publisher", "admin"] as const;

export function AdminMembersPage() {
  const queryClient = useQueryClient();
  const membersQ = useQuery(FrameService.method.listOrgMembers, {});
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<string>("viewer");
  const [error, setError] = useState<string | null>(null);

  const invalidate = () =>
    queryClient.invalidateQueries({
      queryKey: createConnectQueryKey({ schema: FrameService.method.listOrgMembers, cardinality: "finite" }),
    });

  const add = useMutation(FrameService.method.addOrgMember);
  const setRoleM = useMutation(FrameService.method.setMemberRole);
  const remove = useMutation(FrameService.method.removeOrgMember);

  const onAdd = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    add.mutate(
      { email, role },
      {
        onSuccess: () => {
          setEmail("");
          void invalidate();
        },
        onError: (err) => setError(ConnectError.from(err).rawMessage),
      },
    );
  };

  if (membersQ.isLoading) return <div className="text-muted-foreground">Loading...</div>;
  if (membersQ.error) return <p className="text-destructive">Could not load members.</p>;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Members</h1>

      <form onSubmit={onAdd} className="flex items-end gap-3">
        <label className="flex flex-col text-sm">
          Email
          <input
            aria-label="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1 rounded border border-border bg-background px-2 py-1"
          />
        </label>
        <label className="flex flex-col text-sm">
          Role
          <select value={role} onChange={(e) => setRole(e.target.value)} className="mt-1 rounded border border-border bg-background px-2 py-1">
            {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
          </select>
        </label>
        <Button render={<button type="submit" />} disabled={add.isPending}>Add member</Button>
      </form>
      {error && <p className="text-destructive text-sm">{error}</p>}

      <table className="w-full text-sm">
        <thead>
          <tr className="text-left text-muted-foreground">
            <th className="py-2">Email</th><th>Role</th><th>Status</th><th></th>
          </tr>
        </thead>
        <tbody>
          {(membersQ.data?.members ?? []).map((m) => {
            const key = m.userSub || m.email;
            const target = m.userSub ? { userSub: m.userSub } : { email: m.email };
            return (
              <tr key={key} className="border-t border-border">
                <td className="py-2">{m.email || m.userSub}</td>
                <td>
                  <select
                    aria-label={`role for ${m.email || m.userSub}`}
                    value={m.role}
                    onChange={(e) =>
                      setRoleM.mutate({ ...target, role: e.target.value }, {
                        onSuccess: () => void invalidate(),
                        onError: (err) => setError(ConnectError.from(err).rawMessage),
                      })
                    }
                    className="rounded border border-border bg-background px-2 py-1"
                  >
                    {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                  </select>
                </td>
                <td>{m.userSub ? "active" : "pending"}</td>
                <td className="text-right">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      if (!window.confirm(`Remove ${m.email || m.userSub}?`)) return;
                      remove.mutate(target, {
                        onSuccess: () => void invalidate(),
                        onError: (err) => setError(ConnectError.from(err).rawMessage),
                      });
                    }}
                  >
                    Remove
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
