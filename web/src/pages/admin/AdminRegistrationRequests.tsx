import { useEffect, useMemo, useState } from "react";
import { CheckCircle2, Inbox, MailQuestion, RefreshCw, UserPlus } from "lucide-react";
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
import { getAdminProvider } from "@/services/admin";
import {
  listRegistrationRequests,
  markRegistrationRequestHandled,
  type RegistrationRequestVM,
} from "@/services/registration";

function fmtDate(iso?: string): string {
  if (!iso) return "Not set";
  return new Date(iso).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

// nickname seed from the email local-part, sanitized to the Register pattern
// (^[a-zA-Z0-9_.-]+$). The admin can edit it before creating the account.
function nicknameFromEmail(email: string): string {
  const local = email.split("@")[0] ?? "";
  return local.replace(/[^a-zA-Z0-9_.-]/g, "").slice(0, 64);
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

export function AdminRegistrationRequests() {
  const confirm = useConfirm();
  const [requests, setRequests] = useState<RegistrationRequestVM[] | null>(null);
  const [showHandled, setShowHandled] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // Inline create-account-from-request dialog.
  const [active, setActive] = useState<RegistrationRequestVM | null>(null);
  const [createNickname, setCreateNickname] = useState("");
  const [createPassword, setCreatePassword] = useState("");
  const [createIsAdmin, setCreateIsAdmin] = useState(false);
  const [busy, setBusy] = useState(false);

  async function load() {
    setError(null);
    try {
      setRequests(await listRegistrationRequests());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  useEffect(() => {
    void load();
  }, []);

  const visible = useMemo(() => {
    const list = requests ?? [];
    return showHandled ? list : list.filter((r) => r.status !== "handled");
  }, [requests, showHandled]);

  const stats = useMemo(() => {
    const list = requests ?? [];
    return {
      total: list.length,
      pending: list.filter((r) => r.status === "pending").length,
      handled: list.filter((r) => r.status === "handled").length,
    };
  }, [requests]);

  function openCreate(req: RegistrationRequestVM) {
    setActive(req);
    setCreateNickname(nicknameFromEmail(req.email));
    setCreatePassword("");
    setCreateIsAdmin(false);
  }

  // Create the account from the request, then mark the request handled — the
  // two admin actions the request triage collapses into one.
  async function createAndHandle() {
    if (!active) return;
    setError(null);
    setNotice(null);
    setBusy(true);
    try {
      await getAdminProvider().createAccount({
        email: active.email,
        nickname: createNickname,
        password: createPassword || undefined,
        isAdmin: createIsAdmin,
      });
      await markRegistrationRequestHandled(active.id);
      setActive(null);
      setNotice(`Account created for ${active.email}; request handled.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  // Dismiss without creating an account (spam / declined).
  async function dismiss(req: RegistrationRequestVM) {
    const ok = await confirm({
      title: "Mark handled without creating an account?",
      description: `${req.email} — this only removes it from the pending list.`,
      confirmLabel: "Mark handled",
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await markRegistrationRequestHandled(req.id);
      setNotice("Request marked handled.");
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
            Registration requests
          </h1>
          <div className="mt-1 text-xs text-muted-foreground">
            access requests left while self-registration is disabled
          </div>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <Switch checked={showHandled} onCheckedChange={setShowHandled} />
            Show handled
          </label>
          <Button size="sm" variant="outline" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-4 md:grid-cols-3">
        <Panel label="Total">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">{stats.total}</div>
            <Inbox className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
        <Panel label="Pending">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">{stats.pending}</div>
            <MailQuestion className="h-5 w-5 text-warning" />
          </div>
        </Panel>
        <Panel label="Handled">
          <div className="flex items-end justify-between gap-3">
            <div className="text-3xl font-semibold text-foreground">{stats.handled}</div>
            <CheckCircle2 className="h-5 w-5 text-muted-foreground" />
          </div>
        </Panel>
      </div>

      <div className="mt-6">
        <Panel label="ListRegistrationRequests" bodyClassName="">
          {!requests && !error && (
            <div className="p-6 text-sm text-muted-foreground">Loading...</div>
          )}
          {requests && visible.length === 0 && !error && (
            <div className="p-6 text-sm text-muted-foreground">
              {showHandled ? "No requests." : "No pending requests."}
            </div>
          )}
          {requests && visible.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Email</TableHead>
                  <TableHead>Message</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Submitted</TableHead>
                  <TableHead className="w-[220px] text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visible.map((req) => (
                  <TableRow key={req.id}>
                    <TableCell>
                      <div className="text-sm text-foreground">{req.email}</div>
                      <div className="font-mono text-[10px] text-muted-foreground">
                        id {req.id}
                      </div>
                    </TableCell>
                    <TableCell className="max-w-xs">
                      <div className="truncate text-xs text-muted-foreground" title={req.message}>
                        {req.message || "—"}
                      </div>
                    </TableCell>
                    <TableCell>
                      {req.status === "handled" ? (
                        <Badge variant="secondary">handled</Badge>
                      ) : (
                        <Badge variant="warning">pending</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {fmtDate(req.createdAt)}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1.5">
                        {req.status !== "handled" && (
                          <>
                            <Button size="sm" onClick={() => openCreate(req)}>
                              <UserPlus className="h-3.5 w-3.5" />
                              Create account
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => void dismiss(req)}
                              title="Mark handled without creating an account"
                            >
                              Dismiss
                            </Button>
                          </>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </Panel>
      </div>

      <Dialog
        open={!!active}
        onOpenChange={(open) => {
          if (!open) setActive(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create account from request</DialogTitle>
            <DialogDescription>
              {active
                ? `Provisions ${active.email} and marks the request handled.`
                : "Select a request."}
            </DialogDescription>
          </DialogHeader>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="rr-email">email</Label>
              <Input id="rr-email" value={active?.email ?? ""} readOnly disabled />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="rr-nickname">nickname</Label>
              <Input
                id="rr-nickname"
                value={createNickname}
                onChange={(e) => setCreateNickname(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="rr-password">password</Label>
              <Input
                id="rr-password"
                type="password"
                value={createPassword}
                onChange={(e) => setCreatePassword(e.target.value)}
              />
            </div>
            <div className="flex items-center justify-between border border-border bg-muted/20 px-3 py-2">
              <Label htmlFor="rr-is-admin">is_admin</Label>
              <Switch
                id="rr-is-admin"
                checked={createIsAdmin}
                onCheckedChange={setCreateIsAdmin}
              />
            </div>
          </div>
          {active?.message && (
            <div className="border border-border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
              <span className="font-mono uppercase tracking-wider text-zinc-600">
                note
              </span>
              <div className="mt-1 whitespace-pre-wrap">{active.message}</div>
            </div>
          )}
          <div className="flex justify-end">
            <Button
              onClick={() => void createAndHandle()}
              disabled={busy || !createNickname || !createPassword}
            >
              {busy ? "Creating…" : "Create & mark handled"}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
