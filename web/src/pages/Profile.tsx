import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Building2,
  CheckCircle2,
  Copy,
  KeyRound,
  Link2Off,
  LockKeyhole,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
  UserRound,
  XCircle,
} from "lucide-react";
import { Avatar } from "@/components/Avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useConfirm } from "@/components/ui/confirm-dialog";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAuth } from "@/hooks/useAuth";
import { Panel, SectionLabel } from "@/pages/dashboard/Panel";
import {
  getAccountProvider,
  type AccountApiToken,
  type AccountExternalIdentity,
  type AccountProfile,
} from "@/services/account";
import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";

const RESOURCES: NonNullable<PermissionJson["resource"]>[] = [
  "RESOURCE_ACCOUNT",
  "RESOURCE_TENANT",
  "RESOURCE_ROLE",
  "RESOURCE_MEMBERSHIP",
  "RESOURCE_SETTINGS",
  "RESOURCE_PRESET",
  "RESOURCE_WIZARD",
  "RESOURCE_TEST_RUN",
  "RESOURCE_SUITE",
  "RESOURCE_SUITE_RUN",
  "RESOURCE_FAVORITE",
  "RESOURCE_AGENT_SHELL",
  "RESOURCE_SHARE",
  "RESOURCE_PACKAGE",
];

const ACTIONS: NonNullable<PermissionJson["action"]>[] = [
  "ACTION_CREATE",
  "ACTION_READ",
  "ACTION_UPDATE",
  "ACTION_DELETE",
  "ACTION_LIST",
  "ACTION_MANAGE",
];

function fmtDate(iso?: string): string {
  if (!iso) return "Not set";
  return new Date(iso).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

function shortEnum(value?: string): string {
  if (!value) return "unspecified";
  return value
    .replace(/^RESOURCE_/, "")
    .replace(/^ACTION_/, "")
    .replace(/^API_TOKEN_TYPE_/, "")
    .toLowerCase()
    .replace(/_/g, "-");
}

function permissionLabel(permission: PermissionJson) {
  return `${shortEnum(permission.resource)}:${shortEnum(permission.action)}`;
}

function tokenTypeLabel(type: AccountApiToken["type"]): string {
  return shortEnum(type);
}

function readOnlyValue(value: string | boolean | undefined) {
  if (value === undefined || value === "") return "Not set";
  return typeof value === "boolean" ? String(value) : value;
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

function PermissionBadges({ permissions }: { permissions: PermissionJson[] }) {
  if (permissions.length === 0) {
    return <span className="text-xs text-muted-foreground">Inherited</span>;
  }

  return (
    <div className="flex max-w-sm flex-wrap gap-1">
      {permissions.map((permission, index) => (
        <Badge key={`${permissionLabel(permission)}-${index}`} variant="outline">
          {permissionLabel(permission)}
        </Badge>
      ))}
    </div>
  );
}

export function Profile() {
  const { user } = useAuth();
  const confirm = useConfirm();

  const [account, setAccount] = useState<AccountProfile | null>(null);
  const [tokens, setTokens] = useState<AccountApiToken[]>([]);
  const [identities, setIdentities] = useState<AccountExternalIdentity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [email, setEmail] = useState("");
  const [nickname, setNickname] = useState("");
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");

  const [tokenDialogOpen, setTokenDialogOpen] = useState(false);
  const [tokenName, setTokenName] = useState("");
  const [tokenType, setTokenType] = useState<AccountApiToken["type"]>(
    "API_TOKEN_TYPE_SERVICE",
  );
  const [tokenTtl, setTokenTtl] = useState("");
  const [permissionDrafts, setPermissionDrafts] = useState<PermissionJson[]>([
    { resource: "RESOURCE_TEST_RUN", action: "ACTION_CREATE" },
  ]);
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  useEffect(() => {
    if (!user) return;
    let cancelled = false;
    setLoading(true);
    setError(null);

    Promise.all([
      getAccountProvider().getMyAccount(),
      getAccountProvider().listApiTokens(user.id),
      getAccountProvider().listExternalIdentities(user.id),
    ])
      .then(([nextAccount, nextTokens, nextIdentities]) => {
        if (cancelled) return;
        setAccount(nextAccount);
        setEmail(nextAccount.email);
        setNickname(nextAccount.nickname);
        setTokens(nextTokens);
        setIdentities(nextIdentities);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [user]);

  const tokenCounts = useMemo(() => {
    const service = tokens.filter(
      (token) => token.type === "API_TOKEN_TYPE_SERVICE",
    ).length;
    const personal = tokens.filter(
      (token) => token.type === "API_TOKEN_TYPE_PERSONAL",
    ).length;
    return { service, personal };
  }, [tokens]);

  if (!user) return null;

  const display = account?.nickname ?? user.username;
  const canSaveAccount =
    !!account && (email !== account.email || nickname !== account.nickname);

  async function saveAccount() {
    if (!account) return;
    setError(null);
    setNotice(null);
    try {
      const next = await getAccountProvider().updateAccount({
        id: account.id,
        email: email.trim(),
        nickname: nickname.trim(),
      });
      setAccount(next);
      setEmail(next.email);
      setNickname(next.nickname);
      setNotice("Account updated.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function changePassword() {
    setError(null);
    setNotice(null);
    try {
      await getAccountProvider().changePassword({
        oldPassword,
        newPassword,
      });
      setOldPassword("");
      setNewPassword("");
      setNotice("Password changed.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function resendVerification() {
    setError(null);
    setNotice(null);
    try {
      await getAccountProvider().resendVerification();
      setNotice("Verification email queued.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function unlinkIdentity(identity: AccountExternalIdentity) {
    const ok = await confirm({
      title: "Unlink external identity?",
      description: `${identity.providerId} / ${identity.subject}`,
      confirmLabel: "Unlink",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await getAccountProvider().unlinkExternalIdentity(identity.id);
      setIdentities((prev) => prev.filter((item) => item.id !== identity.id));
      setNotice("External identity unlinked.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function createToken() {
    if (!account) return;
    setError(null);
    setNotice(null);
    try {
      const result = await getAccountProvider().createApiToken({
        accountId: account.id,
        name: tokenName,
        type: tokenType,
        permissions:
          tokenType === "API_TOKEN_TYPE_SERVICE" ? permissionDrafts : [],
        ttl: tokenTtl.trim() || undefined,
      });
      setTokens((prev) => [result.token, ...prev]);
      setCreatedSecret(result.secret);
      setNotice("API token created.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function revokeToken(token: AccountApiToken) {
    const ok = await confirm({
      title: "Revoke API token?",
      description: `${token.name} / ${token.prefix}`,
      confirmLabel: "Revoke",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      await getAccountProvider().revokeApiToken(token.id);
      setTokens((prev) => prev.filter((item) => item.id !== token.id));
      setNotice("API token revoked.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function resetTokenDialog(open: boolean) {
    setTokenDialogOpen(open);
    if (open) {
      setTokenName("");
      setTokenType("API_TOKEN_TYPE_SERVICE");
      setTokenTtl("");
      setPermissionDrafts([
        { resource: "RESOURCE_TEST_RUN", action: "ACTION_CREATE" },
      ]);
      setCreatedSecret(null);
    }
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Account
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="flex min-w-0 items-center gap-4">
          <Avatar name={display} size={56} />
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="truncate text-xl font-semibold tracking-tight text-foreground">
                Account settings
              </h1>
              {account?.emailVerified ? (
                <Badge variant="success">verified</Badge>
              ) : (
                <Badge variant="warning">unverified</Badge>
              )}
              {account?.isAdmin && <Badge variant="warning">admin</Badge>}
            </div>
            <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="font-mono">@{display}</span>
              <span>{account?.email ?? user.email}</span>
              <span className="font-mono">id {account?.id ?? user.id}</span>
            </div>
          </div>
        </div>
        <Button asChild variant="outline" size="sm">
          <Link to="/orgs">
            <Building2 className="h-3.5 w-3.5" />
            Organizations
          </Link>
        </Button>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      {loading ? (
        <div className="mt-8 text-sm text-muted-foreground">Loading...</div>
      ) : (
        <div className="mt-8 grid grid-cols-1 gap-6 lg:grid-cols-[18rem_1fr]">
          <div className="space-y-4">
            <Panel label="Identity">
              <div className="flex flex-col gap-3">
                <div className="flex items-center gap-3">
                  <span className="flex h-9 w-9 items-center justify-center border border-border bg-muted/40">
                    <UserRound className="h-4 w-4 text-muted-foreground" />
                  </span>
                  <div className="min-w-0">
                    <div className="truncate text-sm text-foreground">
                      {account?.nickname}
                    </div>
                    <div className="truncate text-[11px] text-muted-foreground">
                      {account?.email}
                    </div>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div className="border border-border bg-muted/20 px-3 py-2">
                    <div className="font-mono text-[10px] uppercase text-zinc-500">
                      Tokens
                    </div>
                    <div className="text-foreground">{tokens.length}</div>
                  </div>
                  <div className="border border-border bg-muted/20 px-3 py-2">
                    <div className="font-mono text-[10px] uppercase text-zinc-500">
                      SSO
                    </div>
                    <div className="text-foreground">{identities.length}</div>
                  </div>
                </div>
                <div className="space-y-1 text-[11px] text-muted-foreground">
                  <div>created {fmtDate(account?.createdAt)}</div>
                  <div>updated {fmtDate(account?.updatedAt)}</div>
                </div>
              </div>
            </Panel>

            <Panel label="Token types">
              <div className="space-y-2 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">service</span>
                  <Badge variant="default">{tokenCounts.service}</Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">personal</span>
                  <Badge variant="secondary">{tokenCounts.personal}</Badge>
                </div>
              </div>
            </Panel>
          </div>

          <Tabs defaultValue="account" className="min-w-0">
            <TabsList className="w-full justify-start overflow-x-auto">
              <TabsTrigger value="account">Account</TabsTrigger>
              <TabsTrigger value="security">Security</TabsTrigger>
              <TabsTrigger value="tokens">API tokens</TabsTrigger>
              <TabsTrigger value="organizations">Organizations</TabsTrigger>
            </TabsList>

            <TabsContent value="account" className="mt-4">
              <Panel
                label="GetMyAccount / UpdateAccount"
                action={
                  <Button
                    size="sm"
                    disabled={!canSaveAccount}
                    onClick={() => void saveAccount()}
                  >
                    <Save className="h-3.5 w-3.5" />
                    Save
                  </Button>
                }
              >
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-email">email</Label>
                    <Input
                      id="account-email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-nickname">nickname</Label>
                    <Input
                      id="account-nickname"
                      value={nickname}
                      onChange={(e) => setNickname(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-id">id</Label>
                    <Input
                      id="account-id"
                      value={readOnlyValue(account?.id)}
                      readOnly
                      disabled
                      className="font-mono"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-email-verified">email_verified</Label>
                    <Input
                      id="account-email-verified"
                      value={readOnlyValue(account?.emailVerified)}
                      readOnly
                      disabled
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-is-admin">is_admin</Label>
                    <Input
                      id="account-is-admin"
                      value={readOnlyValue(account?.isAdmin)}
                      readOnly
                      disabled
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="account-updated-at">updated_at</Label>
                    <Input
                      id="account-updated-at"
                      value={fmtDate(account?.updatedAt)}
                      readOnly
                      disabled
                    />
                  </div>
                </div>
              </Panel>
            </TabsContent>

            <TabsContent value="security" className="mt-4 space-y-4">
              <Panel
                label="ChangePassword"
                action={
                  <Button
                    size="sm"
                    disabled={!oldPassword || !newPassword}
                    onClick={() => void changePassword()}
                  >
                    <LockKeyhole className="h-3.5 w-3.5" />
                    Change
                  </Button>
                }
              >
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="old-password">old_password</Label>
                    <Input
                      id="old-password"
                      type="password"
                      value={oldPassword}
                      onChange={(e) => setOldPassword(e.target.value)}
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="new-password">new_password</Label>
                    <Input
                      id="new-password"
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                    />
                  </div>
                </div>
              </Panel>

              <Panel
                label="ResendVerification"
                action={
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!!account?.emailVerified}
                    onClick={() => void resendVerification()}
                  >
                    <RefreshCw className="h-3.5 w-3.5" />
                    Resend
                  </Button>
                }
              >
                <div className="flex items-center gap-2 text-sm">
                  {account?.emailVerified ? (
                    <CheckCircle2 className="h-4 w-4 text-success" />
                  ) : (
                    <XCircle className="h-4 w-4 text-warning" />
                  )}
                  <span className="text-muted-foreground">
                    email_verified {String(!!account?.emailVerified)}
                  </span>
                </div>
              </Panel>

              <Panel label="ListExternalIdentities" bodyClassName="">
                {identities.length === 0 ? (
                  <div className="p-6 text-sm text-muted-foreground">
                    No external identities.
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Provider</TableHead>
                        <TableHead>Subject</TableHead>
                        <TableHead>Email</TableHead>
                        <TableHead className="w-[72px] text-right">
                          Actions
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {identities.map((identity) => (
                        <TableRow key={identity.id}>
                          <TableCell>
                            <div className="font-mono text-xs text-foreground">
                              {identity.providerId}
                            </div>
                            <div className="font-mono text-[10px] text-muted-foreground">
                              id {identity.id}
                            </div>
                          </TableCell>
                          <TableCell className="max-w-[18rem]">
                            <div className="truncate font-mono text-xs text-muted-foreground">
                              {identity.subject}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="text-xs text-muted-foreground">
                              {identity.email || "Not set"}
                            </div>
                            <div className="text-[11px] text-muted-foreground">
                              linked {fmtDate(identity.createdAt)}
                            </div>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-destructive"
                              onClick={() => void unlinkIdentity(identity)}
                            >
                              <Link2Off className="h-3.5 w-3.5" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </Panel>
            </TabsContent>

            <TabsContent value="tokens" className="mt-4">
              <Panel
                label="ListApiTokens"
                action={
                  <Button size="sm" variant="outline" onClick={() => resetTokenDialog(true)}>
                    <Plus className="h-3.5 w-3.5" />
                    New token
                  </Button>
                }
                bodyClassName=""
              >
                {tokens.length === 0 ? (
                  <div className="p-6 text-sm text-muted-foreground">
                    No API tokens.
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Token</TableHead>
                        <TableHead>Permissions</TableHead>
                        <TableHead>Dates</TableHead>
                        <TableHead className="w-[72px] text-right">
                          Actions
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {tokens.map((token) => (
                        <TableRow key={token.id}>
                          <TableCell>
                            <div className="flex min-w-0 items-center gap-3">
                              <span className="flex h-8 w-8 shrink-0 items-center justify-center border border-border bg-muted/40">
                                <KeyRound className="h-4 w-4 text-muted-foreground" />
                              </span>
                              <div className="min-w-0">
                                <div className="flex items-center gap-2">
                                  <span className="truncate text-sm text-foreground">
                                    {token.name}
                                  </span>
                                  <Badge
                                    variant={
                                      token.type === "API_TOKEN_TYPE_SERVICE"
                                        ? "default"
                                        : "secondary"
                                    }
                                  >
                                    {tokenTypeLabel(token.type)}
                                  </Badge>
                                </div>
                                <div className="truncate font-mono text-[11px] text-muted-foreground">
                                  prefix {token.prefix}
                                </div>
                                <div className="truncate font-mono text-[10px] text-muted-foreground">
                                  id {token.id} / account_id {token.accountId}
                                </div>
                              </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <PermissionBadges permissions={token.permissions} />
                          </TableCell>
                          <TableCell>
                            <div className="text-[11px] text-muted-foreground">
                              created {fmtDate(token.createdAt)}
                            </div>
                            <div className="text-[11px] text-muted-foreground">
                              expires {fmtDate(token.expiresAt)}
                            </div>
                            <div className="text-[11px] text-muted-foreground">
                              last_used {fmtDate(token.lastUsedAt)}
                            </div>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-destructive"
                              onClick={() => void revokeToken(token)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </Panel>
            </TabsContent>

            <TabsContent value="organizations" className="mt-4">
              <Panel label="ListMyTenants" bodyClassName="">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Tenant</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead className="w-[96px] text-right">
                        Actions
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {user.tenants.map((tenant) => (
                      <TableRow key={tenant.id}>
                        <TableCell>
                          <div className="font-mono text-xs text-foreground">
                            {tenant.slug}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {tenant.name}
                          </div>
                          <div className="font-mono text-[10px] text-muted-foreground">
                            id {tenant.id}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">{tenant.role}</Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button asChild size="sm" variant="outline">
                            <Link to={`/orgs/${tenant.slug}`}>
                              <ShieldCheck className="h-3.5 w-3.5" />
                              Manage
                            </Link>
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Panel>
            </TabsContent>
          </Tabs>
        </div>
      )}

      <Dialog open={tokenDialogOpen} onOpenChange={resetTokenDialog}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>CreateApiToken</DialogTitle>
            <DialogDescription>
              {createdSecret
                ? "CreateApiTokenResponse.secret"
                : "CreateApiTokenRequest"}
            </DialogDescription>
          </DialogHeader>

          {createdSecret ? (
            <div className="space-y-4">
              <div className="border border-border bg-muted/30 p-3">
                <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
                  secret
                </div>
                <div className="break-all font-mono text-xs text-foreground">
                  {createdSecret}
                </div>
              </div>
              <div className="flex justify-end gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => void navigator.clipboard?.writeText(createdSecret)}
                >
                  <Copy className="h-3.5 w-3.5" />
                  Copy
                </Button>
                <Button size="sm" onClick={() => setTokenDialogOpen(false)}>
                  Done
                </Button>
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="token-account-id">account_id</Label>
                  <Input
                    id="token-account-id"
                    value={account?.id ?? ""}
                    readOnly
                    disabled
                    className="font-mono"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="token-name">name</Label>
                  <Input
                    id="token-name"
                    value={tokenName}
                    onChange={(e) => setTokenName(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label>type</Label>
                  <Select
                    value={tokenType}
                    onValueChange={(value) =>
                      setTokenType(value as AccountApiToken["type"])
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="API_TOKEN_TYPE_PERSONAL">
                        API_TOKEN_TYPE_PERSONAL
                      </SelectItem>
                      <SelectItem value="API_TOKEN_TYPE_SERVICE">
                        API_TOKEN_TYPE_SERVICE
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="token-ttl">ttl</Label>
                  <Input
                    id="token-ttl"
                    value={tokenTtl}
                    placeholder="2592000s"
                    onChange={(e) => setTokenTtl(e.target.value)}
                  />
                </div>
              </div>

              {tokenType === "API_TOKEN_TYPE_SERVICE" && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <SectionLabel>permissions</SectionLabel>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        setPermissionDrafts((prev) => [
                          ...prev,
                          {
                            resource: "RESOURCE_TEST_RUN",
                            action: "ACTION_LIST",
                          },
                        ])
                      }
                    >
                      <Plus className="h-3.5 w-3.5" />
                      Add
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {permissionDrafts.map((permission, index) => (
                      <div
                        key={index}
                        className="grid grid-cols-[1fr_1fr_auto] gap-2"
                      >
                        <Select
                          value={permission.resource}
                          onValueChange={(value) =>
                            setPermissionDrafts((prev) =>
                              prev.map((item, i) =>
                                i === index
                                  ? {
                                      ...item,
                                      resource:
                                        value as PermissionJson["resource"],
                                    }
                                  : item,
                              ),
                            )
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {RESOURCES.map((resource) => (
                              <SelectItem key={resource} value={resource}>
                                {resource}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Select
                          value={permission.action}
                          onValueChange={(value) =>
                            setPermissionDrafts((prev) =>
                              prev.map((item, i) =>
                                i === index
                                  ? {
                                      ...item,
                                      action: value as PermissionJson["action"],
                                    }
                                  : item,
                              ),
                            )
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {ACTIONS.map((action) => (
                              <SelectItem key={action} value={action}>
                                {action}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Button
                          size="icon"
                          variant="ghost"
                          className="h-9 w-9 text-muted-foreground hover:text-destructive"
                          disabled={permissionDrafts.length === 1}
                          onClick={() =>
                            setPermissionDrafts((prev) =>
                              prev.filter((_, i) => i !== index),
                            )
                          }
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div className="flex justify-end gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setTokenDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button size="sm" onClick={() => void createToken()}>
                  Create
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
