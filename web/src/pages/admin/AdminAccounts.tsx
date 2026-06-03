import { useEffect, useMemo, useState } from "react";
import {
  KeyRound,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserRound,
  UsersRound,
} from "lucide-react";
import { Avatar } from "@/components/Avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useConfirm } from "@/components/ui/confirm-dialog";
import { Panel } from "@/pages/dashboard/Panel";
import {
  getAdminProvider,
  type AdminAccount,
} from "@/services/admin";

function fmtDate(iso?: string): string {
  if (!iso) return "Not set";
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function StatusLine({
  value,
  tone = "default",
}: {
  value: string | null;
  tone?: "default" | "error";
}) {
  if (!value) return null;
  return (
    <div
      className={
        tone === "error"
          ? "border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          : "border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground"
      }
    >
      {value}
    </div>
  );
}

export function AdminAccounts() {
  const confirm = useConfirm();
  const [accounts, setAccounts] = useState<AdminAccount[] | null>(null);
  const [nextPageToken, setNextPageToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [createOpen, setCreateOpen] = useState(false);
  const [createEmail, setCreateEmail] = useState("");
  const [createNickname, setCreateNickname] = useState("");
  const [createPassword, setCreatePassword] = useState("");
  const [createIsAdmin, setCreateIsAdmin] = useState(false);
  const [linkProviderId, setLinkProviderId] = useState("");
  const [linkSubject, setLinkSubject] = useState("");
  const [linkEmail, setLinkEmail] = useState("");

  const [editingAccount, setEditingAccount] = useState<AdminAccount | null>(null);
  const [editEmail, setEditEmail] = useState("");
  const [editNickname, setEditNickname] = useState("");

  const [resetAccount, setResetAccount] = useState<AdminAccount | null>(null);
  const [newPassword, setNewPassword] = useState("");

  const stats = useMemo(() => {
    const list = accounts ?? [];
    return {
      total: list.length,
      admins: list.filter((account) => account.isAdmin).length,
      unverified: list.filter((account) => !account.emailVerified).length,
    };
  }, [accounts]);

  async function load() {
    setError(null);
    try {
      const page = await getAdminProvider().listAccounts({ pageSize: 100 });
      setAccounts(page.accounts);
      setNextPageToken(page.nextPageToken);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    let cancelled = false;
    setError(null);
    getAdminProvider()
      .listAccounts({ pageSize: 100 })
      .then((page) => {
        if (cancelled) return;
        setAccounts(page.accounts);
        setNextPageToken(page.nextPageToken);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  function openCreate() {
    setCreateEmail("");
    setCreateNickname("");
    setCreatePassword("");
    setCreateIsAdmin(false);
    setLinkProviderId("");
    setLinkSubject("");
    setLinkEmail("");
    setCreateOpen(true);
  }

  async function createAccount() {
    setError(null);
    setNotice(null);
    try {
      const hasLink = linkProviderId || linkSubject || linkEmail;
      await getAdminProvider().createAccount({
        email: createEmail,
        nickname: createNickname,
        password: createPassword || undefined,
        isAdmin: createIsAdmin,
        link: hasLink
          ? {
              providerId: linkProviderId,
              subject: linkSubject,
              email: linkEmail,
            }
          : undefined,
      });
      setCreateOpen(false);
      setNotice("Account created.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function openEdit(account: AdminAccount) {
    setEditingAccount(account);
    setEditEmail(account.email);
    setEditNickname(account.nickname);
  }

  async function updateAccount() {
    if (!editingAccount) return;
    setError(null);
    setNotice(null);
    try {
      await getAdminProvider().updateAccount({
        id: editingAccount.id,
        email: editEmail,
        nickname: editNickname,
      });
      setEditingAccount(null);
      setNotice("Account updated.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function openReset(account: AdminAccount) {
    setResetAccount(account);
    setNewPassword("");
  }

  async function resetPassword() {
    if (!resetAccount) return;
    setError(null);
    setNotice(null);
    try {
      await getAdminProvider().resetPassword({
        accountId: resetAccount.id,
        newPassword,
      });
      setResetAccount(null);
      setNotice("Password reset.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function deleteAccount(account: AdminAccount) {
    const ok = await confirm({
      title: "Delete account?",
      description: `${account.nickname} / ${account.id}`,
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await getAdminProvider().deleteAccount(account.id);
      setNotice("Account deleted.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Admin
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight text-foreground">
            Accounts
          </h1>
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span>global identities</span>
            <span className="font-mono">next_page_token {nextPageToken || "empty"}</span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
          <Button size="sm" onClick={openCreate}>
            <Plus className="h-3.5 w-3.5" />
            Create
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Panel label="Accounts">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">
              {stats.total}
            </div>
            <UsersRound className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
        <Panel label="Admins">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">
              {stats.admins}
            </div>
            <ShieldCheck className="h-5 w-5 text-warning" />
          </div>
        </Panel>
        <Panel label="Unverified">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">
              {stats.unverified}
            </div>
            <UserRound className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
      </div>

      <div className="mt-6">
        <Panel label="ListAccounts" bodyClassName="">
          {!accounts && !error && (
            <div className="p-6 text-sm text-muted-foreground">Loading...</div>
          )}
          {accounts && accounts.length === 0 && !error && (
            <div className="p-6 text-sm text-muted-foreground">
              No accounts.
            </div>
          )}
          {accounts && accounts.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Account</TableHead>
                  <TableHead>Flags</TableHead>
                  <TableHead>Created</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="w-[144px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {accounts.map((account) => (
                  <TableRow key={account.id}>
                    <TableCell>
                      <div className="flex min-w-0 items-center gap-3">
                        <Avatar name={account.nickname} size={32} />
                        <div className="min-w-0">
                          <div className="truncate text-sm text-foreground">
                            {account.nickname}
                          </div>
                          <div className="truncate text-xs text-muted-foreground">
                            {account.email}
                          </div>
                          <div className="font-mono text-[10px] text-muted-foreground">
                            id {account.id}
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1.5">
                        {account.isAdmin && (
                          <Badge variant="warning">is_admin</Badge>
                        )}
                        {account.emailVerified ? (
                          <Badge variant="success">email_verified</Badge>
                        ) : (
                          <Badge variant="secondary">unverified</Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {fmtDate(account.createdAt)}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {fmtDate(account.updatedAt)}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => openEdit(account)}
                          title="Edit account"
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => openReset(account)}
                          title="Reset password"
                        >
                          <KeyRound className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => void deleteAccount(account)}
                          title="Delete account"
                          className="text-destructive hover:text-destructive"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Panel>
      </div>

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>CreateAccount</DialogTitle>
            <DialogDescription>
              Password and external identity link are not stored in Account.
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="create-email">email</Label>
              <Input
                id="create-email"
                value={createEmail}
                onChange={(e) => setCreateEmail(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="create-nickname">nickname</Label>
              <Input
                id="create-nickname"
                value={createNickname}
                onChange={(e) => setCreateNickname(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="create-password">password</Label>
              <Input
                id="create-password"
                type="password"
                value={createPassword}
                onChange={(e) => setCreatePassword(e.target.value)}
              />
            </div>
            <div className="flex items-center justify-between border border-border bg-muted/20 px-3 py-2">
              <Label htmlFor="create-is-admin">is_admin</Label>
              <Switch
                id="create-is-admin"
                checked={createIsAdmin}
                onCheckedChange={setCreateIsAdmin}
              />
            </div>
          </div>
          <div className="grid grid-cols-1 gap-4 border-t border-border pt-4 sm:grid-cols-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="link-provider-id">link.provider_id</Label>
              <Input
                id="link-provider-id"
                value={linkProviderId}
                onChange={(e) => setLinkProviderId(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="link-subject">link.subject</Label>
              <Input
                id="link-subject"
                value={linkSubject}
                onChange={(e) => setLinkSubject(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="link-email">link.email</Label>
              <Input
                id="link-email"
                value={linkEmail}
                onChange={(e) => setLinkEmail(e.target.value)}
              />
            </div>
          </div>
          <div className="flex justify-end">
            <Button
              onClick={() => void createAccount()}
              disabled={
                !createEmail ||
                !createNickname ||
                (!createPassword && !linkProviderId && !linkSubject && !linkEmail)
              }
            >
              Create
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!editingAccount}
        onOpenChange={(open) => {
          if (!open) setEditingAccount(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>UpdateAccount</DialogTitle>
            <DialogDescription>
              Only email and nickname are mutable through this request.
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="edit-account-id">id</Label>
              <Input
                id="edit-account-id"
                value={editingAccount?.id ?? ""}
                readOnly
                disabled
                className="font-mono"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="edit-is-admin">is_admin</Label>
              <Input
                id="edit-is-admin"
                value={String(!!editingAccount?.isAdmin)}
                readOnly
                disabled
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="edit-email">email</Label>
              <Input
                id="edit-email"
                value={editEmail}
                onChange={(e) => setEditEmail(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="edit-nickname">nickname</Label>
              <Input
                id="edit-nickname"
                value={editNickname}
                onChange={(e) => setEditNickname(e.target.value)}
              />
            </div>
          </div>
          <div className="flex justify-end">
            <Button
              onClick={() => void updateAccount()}
              disabled={!editEmail || !editNickname}
            >
              Save
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog
        open={!!resetAccount}
        onOpenChange={(open) => {
          if (!open) setResetAccount(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>ResetPassword</DialogTitle>
            <DialogDescription>
              {resetAccount
                ? `${resetAccount.nickname} / ${resetAccount.id}`
                : "Select an account."}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="reset-password">new_password</Label>
            <Input
              id="reset-password"
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex justify-end">
            <Button
              onClick={() => void resetPassword()}
              disabled={!newPassword}
            >
              Reset
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
