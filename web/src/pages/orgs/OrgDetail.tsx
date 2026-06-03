import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  AlertTriangle,
  Building2,
  CheckCircle2,
  Cloud,
  KeyRound,
  LogOut,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
  UserPlus,
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useBreadcrumbLabel } from "@/lib/breadcrumbs";
import { useAuth } from "@/hooks/useAuth";
import { Panel, SectionLabel } from "@/pages/dashboard/Panel";
import type { PermissionJson } from "@/lib/proto/cloud/v1/iam/permission_pb";
import {
  getOrgProvider,
  hasOrgPermission,
  roleListLabel,
  type OrgDetailVM,
  type OrgMemberVM,
  type OrgRole,
  type OrgSettings,
} from "@/services/org";

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
    .replace(/^PROVIDER_/, "")
    .replace(/^ZONE_/, "")
    .replace(/^PLATFORM_ID_/, "")
    .toLowerCase()
    .replace(/_/g, "-");
}

function permissionLabel(permission: PermissionJson) {
  return `${shortEnum(permission.resource)}:${shortEnum(permission.action)}`;
}

function roleBadgeVariant(roles: OrgRole[]): "default" | "success" | "secondary" {
  const names = roles.map((role) => role.name.toLowerCase());
  if (names.some((name) => name === "owner" || name === "admin")) return "success";
  if (names.some((name) => name === "operator")) return "default";
  return "secondary";
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
    return <span className="text-xs text-muted-foreground">No permissions</span>;
  }
  return (
    <div className="flex max-w-xl flex-wrap gap-1">
      {permissions.map((permission, index) => (
        <Badge key={`${permissionLabel(permission)}-${index}`} variant="outline">
          {permissionLabel(permission)}
        </Badge>
      ))}
    </div>
  );
}

function RoleBadges({ roles }: { roles: OrgRole[] }) {
  if (roles.length === 0) {
    return <span className="text-xs text-muted-foreground">No roles</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {roles.map((role) => (
        <Badge
          key={role.id}
          variant={role.isSystem ? "secondary" : "outline"}
          className="font-mono"
        >
          {role.name}
        </Badge>
      ))}
    </div>
  );
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

function defaultYandexSettings(): NonNullable<OrgSettings["yandexSettings"]> {
  return {
    token: "",
    cloudId: "",
    folderId: "",
    zone: "ZONE_RU_CENTRAL1_A",
    networkId: "",
    networkName: "",
    subnetCidr: "",
    platformId: "PLATFORM_ID_STANDARD_V3",
    imageId: "",
    assignPublicIp: false,
    softwareAcceleratedNetwork: false,
    sshUser: "",
    sshPublicKey: "",
  };
}

function roleIdsEqual(a: string[], b: string[]) {
  return a.length === b.length && a.every((value) => b.includes(value));
}

export function OrgDetail() {
  const { slug = "" } = useParams<{ slug: string }>();
  const navigate = useNavigate();
  const confirm = useConfirm();
  const { user } = useAuth();

  const [org, setOrg] = useState<OrgDetailVM | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const [tenantName, setTenantName] = useState("");
  const [tenantSlug, setTenantSlug] = useState("");
  const [settingsDraft, setSettingsDraft] = useState<OrgSettings | null>(null);

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [memberMode, setMemberMode] = useState<"create" | "update">("create");
  const [editingMember, setEditingMember] = useState<OrgMemberVM | null>(null);
  const [memberAccountId, setMemberAccountId] = useState("");
  const [memberRoleIds, setMemberRoleIds] = useState<string[]>([]);

  const [roleDialogOpen, setRoleDialogOpen] = useState(false);
  const [roleMode, setRoleMode] = useState<"create" | "update">("create");
  const [editingRole, setEditingRole] = useState<OrgRole | null>(null);
  const [roleName, setRoleName] = useState("");
  const [roleScope, setRoleScope] = useState<OrgRole["scope"]>("SCOPE_TENANT");
  const [permissionDrafts, setPermissionDrafts] = useState<PermissionJson[]>([
    { resource: "RESOURCE_TEST_RUN", action: "ACTION_LIST" },
  ]);

  const [transferOpen, setTransferOpen] = useState(false);
  const [newOwnerAccountId, setNewOwnerAccountId] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    getOrgProvider()
      .getOrg(slug)
      .then((detail) => {
        if (cancelled) return;
        setOrg(detail);
        setTenantName(detail.tenant.name);
        setTenantSlug(detail.tenant.slug);
        setSettingsDraft(clone(detail.settings));
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
  }, [slug]);

  useBreadcrumbLabel("slug", org?.tenant.name);

  const currentRoles = org?.currentRoles ?? [];
  const canUpdateTenant = hasOrgPermission(
    currentRoles,
    "RESOURCE_TENANT",
    "ACTION_UPDATE",
  );
  const canDeleteTenant = hasOrgPermission(
    currentRoles,
    "RESOURCE_TENANT",
    "ACTION_DELETE",
  );
  const canCreateMembership = hasOrgPermission(
    currentRoles,
    "RESOURCE_MEMBERSHIP",
    "ACTION_CREATE",
  );
  const canUpdateMembership = hasOrgPermission(
    currentRoles,
    "RESOURCE_MEMBERSHIP",
    "ACTION_UPDATE",
  );
  const canDeleteMembership = hasOrgPermission(
    currentRoles,
    "RESOURCE_MEMBERSHIP",
    "ACTION_DELETE",
  );
  const canCreateRole = hasOrgPermission(currentRoles, "RESOURCE_ROLE", "ACTION_CREATE");
  const canUpdateRole = hasOrgPermission(currentRoles, "RESOURCE_ROLE", "ACTION_UPDATE");
  const canDeleteRole = hasOrgPermission(currentRoles, "RESOURCE_ROLE", "ACTION_DELETE");
  const canUpdateSettings = hasOrgPermission(
    currentRoles,
    "RESOURCE_SETTINGS",
    "ACTION_UPDATE",
  );

  const permissionCount = useMemo(() => {
    const seen = new Set<string>();
    for (const role of currentRoles) {
      for (const permission of role.permissions) {
        seen.add(permissionLabel(permission));
      }
    }
    return seen.size;
  }, [currentRoles]);

  const canSaveTenant =
    !!org &&
    canUpdateTenant &&
    (tenantName !== org.tenant.name || tenantSlug !== org.tenant.slug);

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl p-8 text-sm text-muted-foreground">
        Loading...
      </div>
    );
  }

  if (error && !org) {
    return (
      <div className="mx-auto max-w-4xl p-8">
        <Button asChild variant="outline" size="sm">
          <Link to="/orgs">Organizations</Link>
        </Button>
        <div className="mt-6 border border-destructive/40 bg-destructive/10 p-4 text-sm text-destructive">
          {error}
        </div>
      </div>
    );
  }

  if (!org || !settingsDraft) return null;

  const orgDetail = org;
  const settings = settingsDraft;

  async function refresh() {
    setError(null);
    try {
      const detail = await getOrgProvider().getOrg(orgDetail.tenant.slug);
      setOrg(detail);
      setTenantName(detail.tenant.name);
      setTenantSlug(detail.tenant.slug);
      setSettingsDraft(clone(detail.settings));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function acceptDetail(detail: OrgDetailVM) {
    setOrg(detail);
    setTenantName(detail.tenant.name);
    setTenantSlug(detail.tenant.slug);
    setSettingsDraft(clone(detail.settings));
  }

  async function saveTenant() {
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().updateTenant({
        id: orgDetail.tenant.id,
        name: tenantName,
        slug: tenantSlug,
      });
      acceptDetail(next);
      setNotice("Tenant updated.");
      if (next.tenant.slug !== slug) {
        navigate(`/orgs/${next.tenant.slug}`, { replace: true });
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function openCreateMember() {
    setMemberMode("create");
    setEditingMember(null);
    setMemberAccountId("");
    setMemberRoleIds(orgDetail.roles[0] ? [orgDetail.roles[0].id] : []);
    setMemberDialogOpen(true);
  }

  function openEditMember(member: OrgMemberVM) {
    setMemberMode("update");
    setEditingMember(member);
    setMemberAccountId(member.membership.accountId);
    setMemberRoleIds([...member.membership.roleIds]);
    setMemberDialogOpen(true);
  }

  async function saveMember() {
    setError(null);
    setNotice(null);
    try {
      const next =
        memberMode === "create"
          ? await getOrgProvider().createMembership({
              accountId: memberAccountId,
              tenantId: orgDetail.tenant.id,
              roleIds: memberRoleIds,
            })
          : await getOrgProvider().updateMembership({
              id: editingMember?.membership.id ?? "",
              roleIds: memberRoleIds,
            });
      acceptDetail(next);
      setMemberDialogOpen(false);
      setNotice(
        memberMode === "create" ? "Membership created." : "Membership updated.",
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function deleteMember(member: OrgMemberVM) {
    const ok = await confirm({
      title: "Delete membership?",
      description: `${member.account.nickname} / ${member.membership.id}`,
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().deleteMembership(member.membership.id);
      acceptDetail(next);
      setNotice("Membership deleted.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function openCreateRole() {
    setRoleMode("create");
    setEditingRole(null);
    setRoleName("");
    setRoleScope("SCOPE_TENANT");
    setPermissionDrafts([{ resource: "RESOURCE_TEST_RUN", action: "ACTION_LIST" }]);
    setRoleDialogOpen(true);
  }

  function openEditRole(role: OrgRole) {
    setRoleMode("update");
    setEditingRole(role);
    setRoleName(role.name);
    setRoleScope(role.scope);
    setPermissionDrafts(clone(role.permissions));
    setRoleDialogOpen(true);
  }

  async function saveRole() {
    setError(null);
    setNotice(null);
    try {
      const next =
        roleMode === "create"
          ? await getOrgProvider().createRole({
              name: roleName,
              scope: roleScope,
              tenantId: roleScope === "SCOPE_TENANT" ? orgDetail.tenant.id : "",
              permissions: permissionDrafts,
            })
          : await getOrgProvider().updateRole({
              id: editingRole?.id ?? "",
              name: roleName,
              permissions: permissionDrafts,
            });
      acceptDetail(next);
      setRoleDialogOpen(false);
      setNotice(roleMode === "create" ? "Role created." : "Role updated.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function deleteRole(role: OrgRole) {
    const ok = await confirm({
      title: "Delete role?",
      description: `${role.name} / ${role.id}`,
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().deleteRole(role.id);
      acceptDetail(next);
      setNotice("Role deleted.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function saveSettings() {
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().updateTenantSettings({
        tenantId: orgDetail.tenant.id,
        settings: settings,
      });
      acceptDetail(next);
      setNotice("Tenant settings updated.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function transferOwnership() {
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().transferTenantOwnership({
        tenantId: orgDetail.tenant.id,
        newOwnerAccountId,
      });
      acceptDetail(next);
      setTransferOpen(false);
      setNotice("Tenant ownership transferred.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function leaveTenant() {
    const ok = await confirm({
      title: "Leave organization?",
      description: `${orgDetail.tenant.name} / ${orgDetail.tenant.id}`,
      confirmLabel: "Leave",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await getOrgProvider().leaveTenant(orgDetail.tenant.id);
      navigate("/orgs");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function deleteTenant() {
    const ok = await confirm({
      title: "Delete organization?",
      description: `${orgDetail.tenant.name} / ${orgDetail.tenant.id}`,
      confirmLabel: "Delete",
      danger: true,
    });
    if (!ok) return;
    setError(null);
    try {
      await getOrgProvider().deleteTenant(orgDetail.tenant.id);
      navigate("/orgs");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function toggleMemberRole(roleId: string) {
    setMemberRoleIds((prev) =>
      prev.includes(roleId)
        ? prev.filter((id) => id !== roleId)
        : [...prev, roleId],
    );
  }

  function setSettingsValue<K extends keyof OrgSettings>(
    key: K,
    value: OrgSettings[K],
  ) {
    setSettingsDraft((prev) => (prev ? { ...prev, [key]: value } : prev));
  }

  function setYandexValue<K extends keyof NonNullable<OrgSettings["yandexSettings"]>>(
    key: K,
    value: NonNullable<OrgSettings["yandexSettings"]>[K],
  ) {
    setSettingsDraft((prev) => {
      if (!prev) return prev;
      return {
        ...prev,
        yandexSettings: {
          ...(prev.yandexSettings ?? defaultYandexSettings()),
          [key]: value,
        },
      };
    });
  }

  return (
    <div className="mx-auto max-w-6xl p-8">
      <div className="mb-1 text-[10px] font-mono uppercase tracking-wider text-zinc-600">
        Organization
      </div>
      <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div className="flex min-w-0 items-center gap-4">
          <span className="flex h-14 w-14 shrink-0 items-center justify-center border border-border bg-muted/40">
            <Building2 className="h-6 w-6 text-muted-foreground" />
          </span>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="truncate text-xl font-semibold tracking-tight text-foreground">
                {orgDetail.tenant.name}
              </h1>
              <Badge variant={roleBadgeVariant(orgDetail.currentRoles)}>
                {roleListLabel(orgDetail.currentRoles)}
              </Badge>
            </div>
            <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
              <span className="font-mono">slug {orgDetail.tenant.slug}</span>
              <span className="font-mono">id {orgDetail.tenant.id}</span>
              <span className="font-mono">owner {orgDetail.tenant.ownerAccountId}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button asChild size="sm" variant="outline">
            <Link to="/orgs">Organizations</Link>
          </Button>
          <Button size="sm" variant="outline" onClick={() => void refresh()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
        </div>
      </div>

      <div className="mt-5 flex flex-col gap-2">
        <StatusLine value={error} tone="error" />
        <StatusLine value={notice} />
      </div>

      <div className="mt-8 grid grid-cols-1 gap-6 lg:grid-cols-[18rem_1fr]">
        <div className="space-y-4">
          <Panel label="Current access">
            <div className="space-y-3">
              <RoleBadges roles={orgDetail.currentRoles} />
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div className="border border-border bg-muted/20 px-3 py-2">
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    Permissions
                  </div>
                  <div className="text-foreground">{permissionCount}</div>
                </div>
                <div className="border border-border bg-muted/20 px-3 py-2">
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    Members
                  </div>
                  <div className="text-foreground">{orgDetail.memberships.length}</div>
                </div>
              </div>
              <div className="font-mono text-[10px] text-muted-foreground">
                membership_id {orgDetail.currentMembership?.id ?? "not a member"}
              </div>
            </div>
          </Panel>

          <Panel label="Provider">
            <div className="flex items-center gap-2">
              <Cloud className="h-4 w-4 text-muted-foreground" />
              <span className="font-mono text-sm text-foreground">
                {shortEnum(orgDetail.settings.defaultProvider)}
              </span>
            </div>
            <div className="mt-2 text-xs text-muted-foreground">
              max_parallel {orgDetail.settings.defaultMaxParallel ?? 0}
            </div>
            <div className="text-xs text-muted-foreground">
              retention_days {orgDetail.settings.runRetentionDays ?? 0}
            </div>
          </Panel>

          <Panel label="Audit">
            <div className="space-y-1 text-xs text-muted-foreground">
              <div>created {fmtDate(orgDetail.tenant.createdAt)}</div>
              <div>updated {fmtDate(orgDetail.tenant.updatedAt)}</div>
            </div>
          </Panel>
        </div>

        <Tabs defaultValue="tenant" className="min-w-0">
          <TabsList className="w-full justify-start overflow-x-auto">
            <TabsTrigger value="tenant">Tenant</TabsTrigger>
            <TabsTrigger value="members">Members</TabsTrigger>
            <TabsTrigger value="roles">Roles</TabsTrigger>
            <TabsTrigger value="settings">Settings</TabsTrigger>
            <TabsTrigger value="danger">Danger</TabsTrigger>
          </TabsList>

          <TabsContent value="tenant" className="mt-4">
            <Panel
              label="GetTenant / UpdateTenant"
              action={
                <Button
                  size="sm"
                  disabled={!canSaveTenant}
                  onClick={() => void saveTenant()}
                >
                  <Save className="h-3.5 w-3.5" />
                  Save
                </Button>
              }
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="tenant-name">name</Label>
                  <Input
                    id="tenant-name"
                    value={tenantName}
                    disabled={!canUpdateTenant}
                    onChange={(e) => setTenantName(e.target.value)}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="tenant-slug">slug</Label>
                  <Input
                    id="tenant-slug"
                    value={tenantSlug}
                    disabled={!canUpdateTenant}
                    onChange={(e) => setTenantSlug(e.target.value)}
                    className="font-mono"
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="tenant-id">id</Label>
                  <Input id="tenant-id" value={orgDetail.tenant.id} disabled readOnly className="font-mono" />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="tenant-owner">owner_account_id</Label>
                  <Input
                    id="tenant-owner"
                    value={orgDetail.tenant.ownerAccountId}
                    disabled
                    readOnly
                    className="font-mono"
                  />
                </div>
              </div>

              <div className="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <SectionLabel>tags</SectionLabel>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {(orgDetail.tenant.tags?.tags ?? []).length === 0 ? (
                      <span className="text-xs text-muted-foreground">No tags</span>
                    ) : (
                      (orgDetail.tenant.tags?.tags ?? []).map((tag) => (
                        <Badge key={tag} variant="secondary">
                          {tag}
                        </Badge>
                      ))
                    )}
                  </div>
                </div>
                <div>
                  <SectionLabel>labels</SectionLabel>
                  <div className="mt-2 space-y-1">
                    {Object.entries(orgDetail.tenant.tags?.labels ?? {}).length === 0 ? (
                      <span className="text-xs text-muted-foreground">No labels</span>
                    ) : (
                      Object.entries(orgDetail.tenant.tags?.labels ?? {}).map(([key, value]) => (
                        <div
                          key={key}
                          className="flex justify-between gap-3 border border-border bg-muted/20 px-2 py-1 text-xs"
                        >
                          <span className="font-mono text-muted-foreground">{key}</span>
                          <span className="truncate font-mono text-foreground">{value}</span>
                        </div>
                      ))
                    )}
                  </div>
                </div>
              </div>
            </Panel>
          </TabsContent>

          <TabsContent value="members" className="mt-4">
            <Panel
              label="ListMemberships"
              action={
                canCreateMembership ? (
                  <Button size="sm" variant="outline" onClick={openCreateMember}>
                    <UserPlus className="h-3.5 w-3.5" />
                    Add
                  </Button>
                ) : undefined
              }
              bodyClassName=""
            >
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Account</TableHead>
                    <TableHead>Roles</TableHead>
                    <TableHead>Membership</TableHead>
                    <TableHead className="w-[96px] text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {orgDetail.memberships.map((member) => (
                    <TableRow key={member.membership.id}>
                      <TableCell>
                        <div className="flex min-w-0 items-center gap-3">
                          <Avatar name={member.account.nickname} size={28} />
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="truncate text-sm text-foreground">
                                {member.account.nickname}
                              </span>
                              {member.account.emailVerified ? (
                                <CheckCircle2 className="h-3.5 w-3.5 text-success" />
                              ) : (
                                <XCircle className="h-3.5 w-3.5 text-warning" />
                              )}
                              {member.account.isAdmin && (
                                <Badge variant="warning">admin</Badge>
                              )}
                            </div>
                            <div className="truncate text-[11px] text-muted-foreground">
                              {member.account.email}
                            </div>
                            <div className="truncate font-mono text-[10px] text-muted-foreground">
                              account_id {member.account.id}
                            </div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <RoleBadges roles={member.roles} />
                      </TableCell>
                      <TableCell>
                        <div className="font-mono text-[11px] text-foreground">
                          {member.membership.id}
                        </div>
                        <div className="text-[11px] text-muted-foreground">
                          created {fmtDate(member.membership.createdAt)}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="inline-flex gap-1">
                          {canUpdateMembership && (
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground"
                              onClick={() => openEditMember(member)}
                            >
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                          )}
                          {canDeleteMembership && (
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-destructive"
                              onClick={() => void deleteMember(member)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Panel>
          </TabsContent>

          <TabsContent value="roles" className="mt-4">
            <Panel
              label="ListRoles"
              action={
                canCreateRole ? (
                  <Button size="sm" variant="outline" onClick={openCreateRole}>
                    <Plus className="h-3.5 w-3.5" />
                    New role
                  </Button>
                ) : undefined
              }
              bodyClassName=""
            >
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Role</TableHead>
                    <TableHead>Permissions</TableHead>
                    <TableHead>Audit</TableHead>
                    <TableHead className="w-[96px] text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {orgDetail.roles.map((role) => (
                    <TableRow key={role.id}>
                      <TableCell>
                        <div className="flex items-center gap-2">
                          <ShieldCheck className="h-4 w-4 text-muted-foreground" />
                          <span className="font-mono text-sm text-foreground">
                            {role.name}
                          </span>
                          <Badge variant={role.isSystem ? "secondary" : "outline"}>
                            {role.isSystem ? "system" : "custom"}
                          </Badge>
                        </div>
                        <div className="mt-1 font-mono text-[10px] text-muted-foreground">
                          id {role.id}
                        </div>
                        <div className="font-mono text-[10px] text-muted-foreground">
                          scope {role.scope} / tenant_id {role.tenantId || "not set"}
                        </div>
                      </TableCell>
                      <TableCell>
                        <PermissionBadges permissions={role.permissions} />
                      </TableCell>
                      <TableCell>
                        <div className="text-[11px] text-muted-foreground">
                          created {fmtDate(role.createdAt)}
                        </div>
                        <div className="text-[11px] text-muted-foreground">
                          updated {fmtDate(role.updatedAt)}
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="inline-flex gap-1">
                          {canUpdateRole && !role.isSystem && (
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground"
                              onClick={() => openEditRole(role)}
                            >
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                          )}
                          {canDeleteRole && !role.isSystem && (
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7 text-muted-foreground hover:text-destructive"
                              onClick={() => void deleteRole(role)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Panel>
          </TabsContent>

          <TabsContent value="settings" className="mt-4">
            <Panel
              label="UpdateTenantSettings"
              action={
                <Button
                  size="sm"
                  disabled={!canUpdateSettings}
                  onClick={() => void saveSettings()}
                >
                  <Save className="h-3.5 w-3.5" />
                  Save
                </Button>
              }
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label>default_provider</Label>
                  <Select
                    value={settings.defaultProvider}
                    disabled={!canUpdateSettings}
                    onValueChange={(value) => {
                      setSettingsValue(
                        "defaultProvider",
                        value as OrgSettings["defaultProvider"],
                      );
                      if (value === "PROVIDER_YANDEX" && !settings.yandexSettings) {
                        setSettingsValue("yandexSettings", defaultYandexSettings());
                      }
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="PROVIDER_DOCKER">PROVIDER_DOCKER</SelectItem>
                      <SelectItem value="PROVIDER_YANDEX">PROVIDER_YANDEX</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="max-parallel">default_max_parallel</Label>
                  <Input
                    id="max-parallel"
                    type="number"
                    disabled={!canUpdateSettings}
                    value={settings.defaultMaxParallel ?? 0}
                    onChange={(e) =>
                      setSettingsValue(
                        "defaultMaxParallel",
                        Number(e.target.value) || 0,
                      )
                    }
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label htmlFor="retention">run_retention_days</Label>
                  <Input
                    id="retention"
                    type="number"
                    disabled={!canUpdateSettings}
                    value={settings.runRetentionDays ?? 0}
                    onChange={(e) =>
                      setSettingsValue("runRetentionDays", Number(e.target.value) || 0)
                    }
                  />
                </div>
                <div className="flex min-w-0 flex-col gap-1.5">
                  <Label>default_in_tenant_rating</Label>
                  <div className="flex h-9 items-center gap-2 border border-input px-3">
                    <Switch
                      checked={!!settings.defaultInTenantRating}
                      disabled={!canUpdateSettings}
                      onCheckedChange={(checked) =>
                        setSettingsValue("defaultInTenantRating", checked)
                      }
                    />
                    <span className="text-sm text-muted-foreground">
                      {String(!!settings.defaultInTenantRating)}
                    </span>
                  </div>
                </div>
                <div className="flex min-w-0 flex-col gap-1.5">
                  <Label>default_in_global_rating</Label>
                  <div className="flex h-9 items-center gap-2 border border-input px-3">
                    <Switch
                      checked={!!settings.defaultInGlobalRating}
                      disabled={!canUpdateSettings}
                      onCheckedChange={(checked) =>
                        setSettingsValue("defaultInGlobalRating", checked)
                      }
                    />
                    <span className="text-sm text-muted-foreground">
                      {String(!!settings.defaultInGlobalRating)}
                    </span>
                  </div>
                </div>
              </div>

              <div className="mt-6 border-t border-border/70 pt-4">
                <SectionLabel>yandex_settings</SectionLabel>
                {settings.defaultProvider !== "PROVIDER_YANDEX" ? (
                  <div className="mt-3 border border-border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                    Yandex settings are inactive while default_provider is not
                    PROVIDER_YANDEX.
                  </div>
                ) : (
                  <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor="yc-token">token</Label>
                      <Input
                        id="yc-token"
                        disabled={!canUpdateSettings}
                        value={settings.yandexSettings?.token ?? ""}
                        onChange={(e) => setYandexValue("token", e.target.value)}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor="yc-cloud">cloud_id</Label>
                      <Input
                        id="yc-cloud"
                        disabled={!canUpdateSettings}
                        value={settings.yandexSettings?.cloudId ?? ""}
                        onChange={(e) => setYandexValue("cloudId", e.target.value)}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <Label htmlFor="yc-folder">folder_id</Label>
                      <Input
                        id="yc-folder"
                        disabled={!canUpdateSettings}
                        value={settings.yandexSettings?.folderId ?? ""}
                        onChange={(e) => setYandexValue("folderId", e.target.value)}
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <Label>zone</Label>
                      <Select
                        disabled={!canUpdateSettings}
                        value={settings.yandexSettings?.zone ?? "ZONE_RU_CENTRAL1_A"}
                        onValueChange={(value) =>
                          setYandexValue("zone", value as NonNullable<OrgSettings["yandexSettings"]>["zone"])
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="ZONE_RU_CENTRAL1_A">ZONE_RU_CENTRAL1_A</SelectItem>
                          <SelectItem value="ZONE_RU_CENTRAL1_B">ZONE_RU_CENTRAL1_B</SelectItem>
                          <SelectItem value="ZONE_RU_CENTRAL1_D">ZONE_RU_CENTRAL1_D</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {([
                      ["networkId", "network_id"],
                      ["networkName", "network_name"],
                      ["subnetCidr", "subnet_cidr"],
                      ["imageId", "image_id"],
                      ["sshUser", "ssh_user"],
                      ["sshPublicKey", "ssh_public_key"],
                    ] as const).map(([key, label]) => (
                      <div key={key} className="flex flex-col gap-1.5">
                        <Label htmlFor={`yc-${key}`}>{label}</Label>
                        <Input
                          id={`yc-${key}`}
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.[key] ?? ""}
                          onChange={(e) => setYandexValue(key, e.target.value)}
                        />
                      </div>
                    ))}
                    <div className="flex flex-col gap-1.5">
                      <Label>platform_id</Label>
                      <Select
                        disabled={!canUpdateSettings}
                        value={
                          settings.yandexSettings?.platformId ??
                          "PLATFORM_ID_STANDARD_V3"
                        }
                        onValueChange={(value) =>
                          setYandexValue(
                            "platformId",
                            value as NonNullable<OrgSettings["yandexSettings"]>["platformId"],
                          )
                        }
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="PLATFORM_ID_STANDARD_V1">PLATFORM_ID_STANDARD_V1</SelectItem>
                          <SelectItem value="PLATFORM_ID_STANDARD_V2">PLATFORM_ID_STANDARD_V2</SelectItem>
                          <SelectItem value="PLATFORM_ID_STANDARD_V3">PLATFORM_ID_STANDARD_V3</SelectItem>
                          <SelectItem value="PLATFORM_ID_STANDARD_V4A">PLATFORM_ID_STANDARD_V4A</SelectItem>
                          <SelectItem value="PLATFORM_ID_AMD_V1">PLATFORM_ID_AMD_V1</SelectItem>
                          <SelectItem value="PLATFORM_ID_HIGHFREQ_V3">PLATFORM_ID_HIGHFREQ_V3</SelectItem>
                          <SelectItem value="PLATFORM_ID_HIGHFREQ_V4A">PLATFORM_ID_HIGHFREQ_V4A</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="flex min-w-0 flex-col gap-1.5">
                      <Label>assign_public_ip</Label>
                      <div className="flex h-9 items-center gap-2 border border-input px-3">
                        <Switch
                          checked={!!settings.yandexSettings?.assignPublicIp}
                          disabled={!canUpdateSettings}
                          onCheckedChange={(checked) =>
                            setYandexValue("assignPublicIp", checked)
                          }
                        />
                        <span className="text-sm text-muted-foreground">
                          {String(!!settings.yandexSettings?.assignPublicIp)}
                        </span>
                      </div>
                    </div>
                    <div className="flex min-w-0 flex-col gap-1.5">
                      <Label>software_accelerated_network</Label>
                      <div className="flex h-9 items-center gap-2 border border-input px-3">
                        <Switch
                          checked={
                            !!settings.yandexSettings?.softwareAcceleratedNetwork
                          }
                          disabled={!canUpdateSettings}
                          onCheckedChange={(checked) =>
                            setYandexValue("softwareAcceleratedNetwork", checked)
                          }
                        />
                        <span className="text-sm text-muted-foreground">
                          {String(
                            !!settings.yandexSettings
                              ?.softwareAcceleratedNetwork,
                          )}
                        </span>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </Panel>
          </TabsContent>

          <TabsContent value="danger" className="mt-4">
            <Panel label="Tenant destructive actions" className="border-destructive/40">
              <div className="flex flex-col divide-y divide-border/70">
                <div className="flex flex-col gap-3 pb-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div className="text-sm text-foreground">LeaveTenant</div>
                    <div className="text-[11px] text-muted-foreground">
                      tenant_id {orgDetail.tenant.id}
                    </div>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={
                      orgDetail.tenant.ownerAccountId === user?.id || !orgDetail.currentMembership
                    }
                    onClick={() => void leaveTenant()}
                  >
                    <LogOut className="h-3.5 w-3.5" />
                    Leave
                  </Button>
                </div>
                <div className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div className="text-sm text-foreground">
                      TransferTenantOwnership
                    </div>
                    <div className="text-[11px] text-muted-foreground">
                      Requires new_owner_account_id for an existing member.
                    </div>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!canUpdateTenant}
                    onClick={() => {
                      setNewOwnerAccountId("");
                      setTransferOpen(true);
                    }}
                  >
                    <ShieldCheck className="h-3.5 w-3.5" />
                    Transfer
                  </Button>
                </div>
                <div className="flex flex-col gap-3 pt-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div className="flex items-center gap-2 text-sm text-foreground">
                      <AlertTriangle className="h-4 w-4 text-destructive" />
                      DeleteTenant
                    </div>
                    <div className="text-[11px] text-muted-foreground">
                      id {orgDetail.tenant.id}
                    </div>
                  </div>
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={!canDeleteTenant}
                    onClick={() => void deleteTenant()}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    Delete
                  </Button>
                </div>
              </div>
            </Panel>
          </TabsContent>
        </Tabs>
      </div>

      <Dialog open={memberDialogOpen} onOpenChange={setMemberDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {memberMode === "create" ? "CreateMembership" : "UpdateMembership"}
            </DialogTitle>
            <DialogDescription>
              {memberMode === "create"
                ? "CreateMembershipRequest"
                : "UpdateMembershipRequest"}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="member-account-id">account_id</Label>
                <Input
                  id="member-account-id"
                  value={memberAccountId}
                  disabled={memberMode === "update"}
                  onChange={(e) => setMemberAccountId(e.target.value)}
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="member-tenant-id">tenant_id</Label>
                <Input
                  id="member-tenant-id"
                  value={orgDetail.tenant.id}
                  disabled
                  readOnly
                  className="font-mono"
                />
              </div>
            </div>
            <div>
              <SectionLabel>role_ids</SectionLabel>
              <div className="mt-2 flex flex-wrap gap-2">
                {orgDetail.roles.map((role) => {
                  const active = memberRoleIds.includes(role.id);
                  return (
                    <Button
                      key={role.id}
                      size="sm"
                      variant={active ? "default" : "outline"}
                      onClick={() => toggleMemberRole(role.id)}
                    >
                      {role.name}
                    </Button>
                  );
                })}
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="outline" onClick={() => setMemberDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={
                  !memberAccountId ||
                  memberRoleIds.length === 0 ||
                  (memberMode === "update" &&
                    !!editingMember &&
                    roleIdsEqual(memberRoleIds, editingMember.membership.roleIds))
                }
                onClick={() => void saveMember()}
              >
                Save
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={roleDialogOpen} onOpenChange={setRoleDialogOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {roleMode === "create" ? "CreateRole" : "UpdateRole"}
            </DialogTitle>
            <DialogDescription>
              {roleMode === "create" ? "CreateRoleRequest" : "UpdateRoleRequest"}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="role-name">name</Label>
                <Input
                  id="role-name"
                  value={roleName}
                  onChange={(e) => setRoleName(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label>scope</Label>
                <Select
                  value={roleScope}
                  disabled={roleMode === "update"}
                  onValueChange={(value) => setRoleScope(value as OrgRole["scope"])}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="SCOPE_TENANT">SCOPE_TENANT</SelectItem>
                    <SelectItem value="SCOPE_PLATFORM">SCOPE_PLATFORM</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between gap-2">
                <SectionLabel>permissions</SectionLabel>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    setPermissionDrafts((prev) => [
                      ...prev,
                      { resource: "RESOURCE_TEST_RUN", action: "ACTION_LIST" },
                    ])
                  }
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add
                </Button>
              </div>
              <div className="mt-2 space-y-2">
                {permissionDrafts.map((permission, index) => (
                  <div key={index} className="grid grid-cols-[1fr_1fr_auto] gap-2">
                    <Select
                      value={permission.resource}
                      onValueChange={(value) =>
                        setPermissionDrafts((prev) =>
                          prev.map((item, i) =>
                            i === index
                              ? {
                                  ...item,
                                  resource: value as PermissionJson["resource"],
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
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="outline" onClick={() => setRoleDialogOpen(false)}>
                Cancel
              </Button>
              <Button size="sm" disabled={!roleName.trim()} onClick={() => void saveRole()}>
                Save
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={transferOpen} onOpenChange={setTransferOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>TransferTenantOwnership</DialogTitle>
            <DialogDescription>TransferTenantOwnershipRequest</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="transfer-tenant-id">tenant_id</Label>
                <Input
                  id="transfer-tenant-id"
                  value={orgDetail.tenant.id}
                  disabled
                  readOnly
                  className="font-mono"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="transfer-owner-id">new_owner_account_id</Label>
                <Input
                  id="transfer-owner-id"
                  value={newOwnerAccountId}
                  onChange={(e) => setNewOwnerAccountId(e.target.value)}
                  className="font-mono"
                />
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button size="sm" variant="outline" onClick={() => setTransferOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                disabled={!newOwnerAccountId}
                onClick={() => void transferOwnership()}
              >
                Transfer
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
