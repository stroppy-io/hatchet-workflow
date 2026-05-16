import { useEffect, useState } from "react";
import { clients } from "@/api/clients";
import { TenantRole } from "@/lib/proto/cloud/v1/iam/member_pb";
import { protoTsToISO } from "@/lib/proto-helpers";
import { getTenantId } from "@/api/transport";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Plus, Trash2 } from "lucide-react";
import { useConfirm } from "@/components/ui/confirm-dialog";

interface MemberRow {
  memberId: string;
  userId: string;
  username: string;
  role: string;
  created_at: string;
}

interface AdminUser {
  id: string;
  username: string;
}

function roleToString(role: TenantRole): string {
  switch (role) {
    case TenantRole.VIEWER: return "viewer";
    case TenantRole.MEMBER: return "operator";
    case TenantRole.ADMIN: return "admin";
    case TenantRole.OWNER: return "owner";
    default: return "viewer";
  }
}

function stringToRole(s: string): TenantRole {
  switch (s) {
    case "operator": return TenantRole.MEMBER;
    case "admin": return TenantRole.ADMIN;
    case "owner": return TenantRole.OWNER;
    default: return TenantRole.VIEWER;
  }
}

export function TenantMembers() {
  const { user } = useAuth();
  const [members, setMembers] = useState<MemberRow[]>([]);
  const [allUsers, setAllUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState("");
  const [selectedRole, setSelectedRole] = useState("viewer");
  const [adding, setAdding] = useState(false);
  const confirm = useConfirm();

  async function load() {
    try {
      const tid = getTenantId() ?? "";
      const resp = await clients.member.listByTenant({ value: tid });
      setMembers(
        (resp.members ?? []).map((m) => ({
          memberId: m.id?.value ?? "",
          userId: m.userId?.value ?? "",
          username: m.userId?.value ?? "(unknown)",
          role: roleToString(m.role),
          created_at: protoTsToISO(m.timestamps?.createdAt),
        }))
      );
      // Only root users can list all users for adding members.
      if (user?.is_root) {
        try {
          const usersResp = await clients.admin.listAllUsers({});
          setAllUsers(
            (usersResp.users ?? []).map((u) => ({
              id: u.id?.value ?? "",
              username: u.nickname || u.email,
            }))
          );
        } catch {
          // Not root or endpoint unavailable.
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load members");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function handleAdd() {
    if (!selectedUserId) return;
    setAdding(true);
    setError("");
    try {
      const tid = getTenantId() ?? "";
      await clients.member.addMember({
        tenantId: { value: tid },
        userId: { value: selectedUserId },
        role: stringToRole(selectedRole),
      });
      setSelectedUserId("");
      setSelectedRole("viewer");
      setOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add member");
    }
    setAdding(false);
  }

  async function handleRoleChange(memberId: string, role: string) {
    setError("");
    try {
      await clients.member.updateMemberRole({
        memberId: { value: memberId },
        role: stringToRole(role),
      });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update role");
    }
  }

  async function handleRemove(memberId: string, username: string) {
    if (!(await confirm({ title: `Remove "${username}" from this tenant?`, description: "They will lose access to this tenant's runs and resources.", danger: true, confirmLabel: "Remove" }))) return;
    try {
      await clients.member.removeMember({ value: memberId });
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove member");
    }
  }

  // Users not already members.
  const availableUsers = allUsers.filter(
    (u) => !members.some((m) => m.userId === u.id)
  );

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Members</h1>
          <p className="text-sm text-muted-foreground">
            Manage tenant members
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-3.5 w-3.5" />
              Add Member
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add Member</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              {availableUsers.length > 0 ? (
                <>
                  <div className="space-y-2">
                    <Label>User</Label>
                    <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                      <SelectTrigger>
                        <SelectValue placeholder="Select user" />
                      </SelectTrigger>
                      <SelectContent>
                        {availableUsers.map((u) => (
                          <SelectItem key={u.id} value={u.id}>
                            {u.username}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="space-y-2">
                    <Label>Role</Label>
                    <Select value={selectedRole} onValueChange={setSelectedRole}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="viewer">Viewer</SelectItem>
                        <SelectItem value="operator">Operator</SelectItem>
                        <SelectItem value="owner">Owner</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <Button onClick={handleAdd} disabled={adding || !selectedUserId}>
                    {adding ? "Adding..." : "Add"}
                  </Button>
                </>
              ) : (
                <div className="space-y-4">
                  <p className="text-sm text-muted-foreground">
                    {user?.is_root
                      ? "All users are already members of this tenant."
                      : "Enter a user ID to add."}
                  </p>
                  {!user?.is_root && (
                    <>
                      <div className="space-y-2">
                        <Label>User ID</Label>
                        <Input
                          value={selectedUserId}
                          onChange={(e) => setSelectedUserId(e.target.value)}
                          placeholder="user-uuid"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Role</Label>
                        <Select value={selectedRole} onValueChange={setSelectedRole}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="viewer">Viewer</SelectItem>
                            <SelectItem value="operator">Operator</SelectItem>
                            <SelectItem value="owner">Owner</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <Button onClick={handleAdd} disabled={adding || !selectedUserId}>
                        {adding ? "Adding..." : "Add"}
                      </Button>
                    </>
                  )}
                </div>
              )}
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {error && (
        <div className="text-sm text-destructive border border-destructive/30 p-3">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-sm text-muted-foreground">Loading...</div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Added</TableHead>
              <TableHead className="w-10" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((m) => (
              <TableRow key={m.memberId}>
                <TableCell className="font-medium">{m.username}</TableCell>
                <TableCell>
                  <Select
                    value={m.role}
                    onValueChange={(v) => handleRoleChange(m.memberId, v)}
                  >
                    <SelectTrigger className="h-7 w-28 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="viewer">Viewer</SelectItem>
                      <SelectItem value="operator">Operator</SelectItem>
                      <SelectItem value="owner">Owner</SelectItem>
                    </SelectContent>
                  </Select>
                </TableCell>
                <TableCell className="text-xs text-muted-foreground">
                  {new Date(m.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <button
                    onClick={() => handleRemove(m.memberId, m.username)}
                    className="text-muted-foreground hover:text-destructive transition-colors"
                    title="Remove member"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </TableCell>
              </TableRow>
            ))}
            {members.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  No members
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
