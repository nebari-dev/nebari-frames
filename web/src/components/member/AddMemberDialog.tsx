import { useState } from "react";
import { useMutation, createConnectQueryKey } from "@connectrpc/connect-query";
import { useQueryClient } from "@tanstack/react-query";
import { ConnectError } from "@connectrpc/connect";
import { FrameService } from "@gen/frames/v1/frame_service_pb";
import { Dialog, DialogClose, DialogContent, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const ROLES = ["viewer", "publisher", "admin"] as const;

export function AddMemberDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const add = useMutation(FrameService.method.addOrgMember);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<string>("viewer");
  const [error, setError] = useState<string | null>(null);

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setEmail("");
      setRole("viewer");
      setError(null);
    }
    onOpenChange(next);
  };

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    add.mutate(
      { email, role },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries({
            queryKey: createConnectQueryKey({
              schema: FrameService.method.listOrgMembers,
              cardinality: "finite",
            }),
          });
          handleOpenChange(false);
        },
        onError: (err) => setError(ConnectError.from(err).rawMessage),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogTitle>Add member</DialogTitle>
        <form onSubmit={onSubmit} className="space-y-4">
          <label className="flex flex-col gap-1.5 text-sm font-medium text-foreground">
            Email
            <Input
              aria-label="email"
              type="email"
              required
              placeholder="person@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </label>
          <div className="flex flex-col gap-1.5 text-sm font-medium text-foreground">
            Role
            <Select value={role} onValueChange={(value) => value && setRole(value)}>
              <SelectTrigger aria-label="Role">
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
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <DialogFooter className="pt-2">
            <DialogClose render={<Button variant="outline" />}>Cancel</DialogClose>
            <Button render={<button type="submit" />} disabled={add.isPending}>
              Add member
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
