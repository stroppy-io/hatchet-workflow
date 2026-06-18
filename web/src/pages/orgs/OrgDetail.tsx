import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  AlertTriangle,
  Building2,
  CheckCircle2,
  Cloud,
  Eye,
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
import { NumField } from "@/components/ui/num-field";
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
  type OrgMembership,
  type OrgMemberVM,
  type OrgPermissionCatalogEntry,
  type OrgRole,
  type OrgSettings,
} from "@/services/org";
import type {
  ProviderJson,
  ProviderSettingsJson,
} from "@/lib/proto/cloud/v1/deployment/provider_pb";

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

type ConfigurableProvider = Exclude<ProviderJson, "PROVIDER_UNSPECIFIED">;

function providerConfigFromSettings(settings: OrgSettings): ConfigurableProvider {
  if (settings.yandexSettings) {
    return "PROVIDER_YANDEX";
  }
  return "PROVIDER_DOCKER";
}

function roleIdsEqual(a: string[], b: string[]) {
  return a.length === b.length && a.every((value) => b.includes(value));
}

/** Stable key for one {resource, action} pair, used by the catalog grid. */
function permKey(resource: string | undefined, action: string | undefined) {
  return `${resource ?? ""}|${action ?? ""}`;
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
  const [providerDraft, setProviderDraft] =
    useState<ConfigurableProvider>("PROVIDER_DOCKER");

  const [memberDialogOpen, setMemberDialogOpen] = useState(false);
  const [memberMode, setMemberMode] = useState<"create" | "update">("create");
  const [editingMember, setEditingMember] = useState<OrgMemberVM | null>(null);
  const [memberEmail, setMemberEmail] = useState("");
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

  // ListPermissions catalog, loaded once and surfaced as a checkbox grid in the
  // role editor (replaces the free-form resource/action dropdown rows).
  const [permCatalog, setPermCatalog] = useState<OrgPermissionCatalogEntry[]>([]);

  // GetRole / GetMembership single-row detail drawer.
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailTitle, setDetailTitle] = useState("");
  const [detailRole, setDetailRole] = useState<OrgRole | null>(null);
  const [detailMembership, setDetailMembership] =
    useState<OrgMembership | null>(null);

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
        setProviderDraft(providerConfigFromSettings(detail.settings));
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

  useEffect(() => {
    let cancelled = false;
    getOrgProvider()
      .listPermissionCatalog()
      .then((catalog) => {
        if (!cancelled) setPermCatalog(catalog);
      })
      .catch(() => {
        /* catalog is best-effort; the role editor falls back to existing drafts */
      });
    return () => {
      cancelled = true;
    };
  }, []);

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
      setProviderDraft(providerConfigFromSettings(detail.settings));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function acceptDetail(detail: OrgDetailVM) {
    setOrg(detail);
    setTenantName(detail.tenant.name);
    setTenantSlug(detail.tenant.slug);
    setSettingsDraft(clone(detail.settings));
    setProviderDraft(providerConfigFromSettings(detail.settings));
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
    setMemberEmail("");
    setMemberRoleIds(orgDetail.roles[0] ? [orgDetail.roles[0].id] : []);
    setMemberDialogOpen(true);
  }

  function openEditMember(member: OrgMemberVM) {
    setMemberMode("update");
    setEditingMember(member);
    setMemberEmail(member.account.email);
    setMemberRoleIds([...member.membership.roleIds]);
    setMemberDialogOpen(true);
  }

  async function saveMember() {
    setError(null);
    setNotice(null);
    try {
      let next;
      if (memberMode === "create") {
        // Invite by email: resolve the exact address to its account, then add
        // it. A clear message beats the raw NotFound when nobody matches.
        let account;
        try {
          account = await getOrgProvider().lookupAccountByEmail(
            memberEmail.trim(),
          );
        } catch {
          setError(`No account found for ${memberEmail.trim()}.`);
          return;
        }
        next = await getOrgProvider().createMembership({
          accountId: account.id,
          tenantId: orgDetail.tenant.id,
          roleIds: memberRoleIds,
        });
      } else {
        next = await getOrgProvider().updateMembership({
          id: editingMember?.membership.id ?? "",
          roleIds: memberRoleIds,
        });
      }
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

  function providerSettingsPayload(): ProviderSettingsJson {
    if (providerDraft !== "PROVIDER_YANDEX") {
      return { docker: {} };
    }
    return { yandex: settings.yandexSettings ?? defaultYandexSettings() };
  }

  function tenantSettingsPayload(): OrgSettings {
    return {
      ...settings,
      yandexSettings: orgDetail.settings.yandexSettings,
    };
  }

  async function saveTenantSettings() {
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().updateTenantSettings({
        tenantId: orgDetail.tenant.id,
        settings: tenantSettingsPayload(),
      });
      acceptDetail(next);
      setNotice("Tenant settings saved.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function saveProviderSettings() {
    setError(null);
    setNotice(null);
    try {
      const next = await getOrgProvider().setProviderSettings({
        tenantSlug: orgDetail.tenant.slug,
        tenantId: orgDetail.tenant.id,
        settings: providerSettingsPayload(),
      });
      acceptDetail(next);
      setNotice("Provider settings saved.");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function viewRoleDetail(role: OrgRole) {
    setError(null);
    try {
      const full = await getOrgProvider().getRole(role.id);
      setDetailMembership(null);
      setDetailRole(full);
      setDetailTitle(`GetRole — ${full.name}`);
      setDetailOpen(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function viewMemberDetail(member: OrgMemberVM) {
    setError(null);
    try {
      const full = await getOrgProvider().getMembership(member.membership.id);
      setDetailRole(null);
      setDetailMembership(full);
      setDetailTitle(`GetMembership — ${member.account.nickname}`);
      setDetailOpen(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  function togglePermission(entry: OrgPermissionCatalogEntry) {
    setPermissionDrafts((prev) => {
      const exists = prev.some(
        (p) => p.resource === entry.resource && p.action === entry.action,
      );
      return exists
        ? prev.filter(
            (p) => !(p.resource === entry.resource && p.action === entry.action),
          )
        : [...prev, { resource: entry.resource, action: entry.action }];
    });
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

  function optionalBoolValue(value: boolean | undefined): "default" | "true" | "false" {
    if (value === undefined) return "default";
    return value ? "true" : "false";
  }

  function setOptionalBool(
    key: "defaultInTenantRating" | "defaultInGlobalRating",
    value: string,
  ) {
    setSettingsValue(
      key,
      value === "default" ? undefined : value === "true",
    );
  }

  function setNonNegativeNumber(
    key: "defaultMaxParallel" | "runRetentionDays",
    raw: string,
  ) {
    const next = Number.parseInt(raw, 10);
    setSettingsValue(key, Number.isFinite(next) && next > 0 ? next : 0);
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
                          <Button
                            size="icon"
                            variant="ghost"
                            className="h-7 w-7 text-muted-foreground"
                            onClick={() => void viewMemberDetail(member)}
                            title="GetMembership"
                          >
                            <Eye className="h-3.5 w-3.5" />
                          </Button>
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
                          <Button
                            size="icon"
                            variant="ghost"
                            className="h-7 w-7 text-muted-foreground"
                            onClick={() => void viewRoleDetail(role)}
                            title="GetRole"
                          >
                            <Eye className="h-3.5 w-3.5" />
                          </Button>
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
              label="Tenant defaults"
              action={
                <Button
                  size="sm"
                  disabled={!canUpdateSettings}
                  onClick={() => void saveTenantSettings()}
                >
                  <Save className="h-3.5 w-3.5" />
                  Save
                </Button>
              }
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label>Default provider</Label>
                  <Select
                    value={settings.defaultProvider ?? "PROVIDER_DOCKER"}
                    disabled={!canUpdateSettings}
                    onValueChange={(value) => {
                      setSettingsValue(
                        "defaultProvider",
                        value as OrgSettings["defaultProvider"],
                      );
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="PROVIDER_DOCKER">Docker</SelectItem>
                      <SelectItem value="PROVIDER_YANDEX">Yandex Cloud</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <NumField
                    id="settings-max-parallel"
                    label="Default max parallel"
                    disabled={!canUpdateSettings}
                    value={settings.defaultMaxParallel ?? 0}
                    onChange={(n) => setNonNegativeNumber("defaultMaxParallel", String(n))}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <NumField
                    id="settings-retention"
                    label="Run retention days"
                    disabled={!canUpdateSettings}
                    value={settings.runRetentionDays ?? 0}
                    onChange={(n) => setNonNegativeNumber("runRetentionDays", String(n))}
                  />
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label>Tenant rating by default</Label>
                  <Select
                    value={optionalBoolValue(settings.defaultInTenantRating)}
                    disabled={!canUpdateSettings}
                    onValueChange={(value) =>
                      setOptionalBool("defaultInTenantRating", value)
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="default">Platform default</SelectItem>
                      <SelectItem value="true">Enabled</SelectItem>
                      <SelectItem value="false">Disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <div className="flex flex-col gap-1.5">
                  <Label>Global rating by default</Label>
                  <Select
                    value={optionalBoolValue(settings.defaultInGlobalRating)}
                    disabled={!canUpdateSettings}
                    onValueChange={(value) =>
                      setOptionalBool("defaultInGlobalRating", value)
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="default">Platform default</SelectItem>
                      <SelectItem value="true">Enabled</SelectItem>
                      <SelectItem value="false">Disabled</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </Panel>

            <Panel
              label="Provider settings"
              className="mt-4"
              action={
                <Button
                  size="sm"
                  disabled={!canUpdateSettings}
                  onClick={() => void saveProviderSettings()}
                >
                  <Save className="h-3.5 w-3.5" />
                  Save
                </Button>
              }
            >
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="flex flex-col gap-1.5">
                  <Label>Provider config</Label>
                  <Select
                    value={providerDraft}
                    disabled={!canUpdateSettings}
                    onValueChange={(value) =>
                      setProviderDraft(value as ConfigurableProvider)
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="PROVIDER_DOCKER">Docker</SelectItem>
                      <SelectItem value="PROVIDER_YANDEX">Yandex Cloud</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>

              {providerDraft === "PROVIDER_YANDEX" ? (
                <div className="mt-6 flex flex-col gap-5 border-t border-border/70 pt-5">
                  <div>
                    <SectionLabel>Connection</SectionLabel>
                    <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="flex flex-col gap-1.5 sm:col-span-2">
                        <Label htmlFor="yc-token">OAuth token</Label>
                        <Input
                          id="yc-token"
                          type="password"
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.token ?? ""}
                          onChange={(e) => setYandexValue("token", e.target.value)}
                          className="font-mono"
                          autoComplete="off"
                        />
                      </div>
                      <div className="flex flex-col gap-1.5">
                        <Label htmlFor="yc-cloud">Cloud ID</Label>
                        <Input
                          id="yc-cloud"
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.cloudId ?? ""}
                          onChange={(e) => setYandexValue("cloudId", e.target.value)}
                          className="font-mono"
                        />
                      </div>
                      <div className="flex flex-col gap-1.5">
                        <Label htmlFor="yc-folder">Folder ID</Label>
                        <Input
                          id="yc-folder"
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.folderId ?? ""}
                          onChange={(e) => setYandexValue("folderId", e.target.value)}
                          className="font-mono"
                        />
                      </div>
                    </div>
                  </div>

                  <div>
                    <SectionLabel>Placement</SectionLabel>
                    <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="flex flex-col gap-1.5">
                        <Label>Availability zone</Label>
                        <Select
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.zone ?? "ZONE_RU_CENTRAL1_A"}
                          onValueChange={(value) =>
                            setYandexValue(
                              "zone",
                              value as NonNullable<OrgSettings["yandexSettings"]>["zone"],
                            )
                          }
                        >
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="ZONE_RU_CENTRAL1_A">ru-central1-a</SelectItem>
                            <SelectItem value="ZONE_RU_CENTRAL1_B">ru-central1-b</SelectItem>
                            <SelectItem value="ZONE_RU_CENTRAL1_D">ru-central1-d</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="flex flex-col gap-1.5">
                        <Label>VM platform</Label>
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
                            <SelectItem value="PLATFORM_ID_STANDARD_V1">standard-v1</SelectItem>
                            <SelectItem value="PLATFORM_ID_STANDARD_V2">standard-v2</SelectItem>
                            <SelectItem value="PLATFORM_ID_STANDARD_V3">standard-v3</SelectItem>
                            <SelectItem value="PLATFORM_ID_STANDARD_V4A">standard-v4a</SelectItem>
                            <SelectItem value="PLATFORM_ID_AMD_V1">amd-v1</SelectItem>
                            <SelectItem value="PLATFORM_ID_HIGHFREQ_V3">highfreq-v3</SelectItem>
                            <SelectItem value="PLATFORM_ID_HIGHFREQ_V4A">highfreq-v4a</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      {([
                        ["networkId", "Network ID"],
                        ["networkName", "Network name"],
                        ["subnetCidr", "Subnet CIDR"],
                        ["imageId", "Image ID"],
                      ] as const).map(([key, label]) => (
                        <div key={key} className="flex flex-col gap-1.5">
                          <Label htmlFor={`yc-${key}`}>{label}</Label>
                          <Input
                            id={`yc-${key}`}
                            disabled={!canUpdateSettings}
                            value={settings.yandexSettings?.[key] ?? ""}
                            onChange={(e) => setYandexValue(key, e.target.value)}
                            className="font-mono"
                          />
                        </div>
                      ))}
                    </div>
                  </div>

                  <div>
                    <SectionLabel>Access</SectionLabel>
                    <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                      <div className="flex flex-col gap-1.5">
                        <Label htmlFor="yc-ssh-user">SSH user</Label>
                        <Input
                          id="yc-ssh-user"
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.sshUser ?? ""}
                          onChange={(e) => setYandexValue("sshUser", e.target.value)}
                        />
                      </div>
                      <div className="flex flex-col gap-1.5 sm:col-span-2">
                        <Label htmlFor="yc-ssh-public-key">SSH public key</Label>
                        <Input
                          id="yc-ssh-public-key"
                          disabled={!canUpdateSettings}
                          value={settings.yandexSettings?.sshPublicKey ?? ""}
                          onChange={(e) =>
                            setYandexValue("sshPublicKey", e.target.value)
                          }
                          className="font-mono"
                        />
                      </div>
                      <div className="flex min-w-0 flex-col gap-1.5">
                        <Label>Public IP for VMs</Label>
                        <div className="flex h-9 items-center gap-2 border border-input px-3">
                          <Switch
                            checked={!!settings.yandexSettings?.assignPublicIp}
                            disabled={!canUpdateSettings}
                            onCheckedChange={(checked) =>
                              setYandexValue("assignPublicIp", checked)
                            }
                          />
                          <span className="text-sm text-muted-foreground">
                            {settings.yandexSettings?.assignPublicIp
                              ? "Enabled"
                              : "Disabled"}
                          </span>
                        </div>
                      </div>
                      <div className="flex min-w-0 flex-col gap-1.5">
                        <Label>Software network acceleration</Label>
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
                            {settings.yandexSettings?.softwareAcceleratedNetwork
                              ? "Enabled"
                              : "Disabled"}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="mt-6 border border-border bg-muted/20 px-3 py-2 text-sm text-muted-foreground">
                  Docker uses the local Docker daemon. There are no provider
                  fields to fill for this tenant.
                </div>
              )}
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
            {memberMode === "create" ? (
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="member-email">User email</Label>
                <Input
                  id="member-email"
                  type="email"
                  placeholder="user@example.com"
                  value={memberEmail}
                  onChange={(e) => setMemberEmail(e.target.value)}
                  autoFocus
                />
                <p className="text-xs text-muted-foreground">
                  Enter the email of an existing account to add it to this
                  organization.
                </p>
              </div>
            ) : (
              <div className="flex flex-col gap-1.5">
                <Label>Member</Label>
                <div className="border border-border bg-muted/20 px-3 py-2 text-sm">
                  <div className="text-foreground">
                    {editingMember?.account.nickname}
                  </div>
                  <div className="text-xs text-muted-foreground">
                    {editingMember?.account.email}
                  </div>
                </div>
              </div>
            )}
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
                  (memberMode === "create" && !memberEmail.trim()) ||
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
                <SectionLabel>permissions (ListPermissions catalog)</SectionLabel>
                <span className="text-[11px] text-muted-foreground">
                  {permissionDrafts.length} selected
                </span>
              </div>
              {permCatalog.length === 0 ? (
                <div className="mt-2 border border-border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                  Permission catalog unavailable. {permissionDrafts.length}{" "}
                  permission(s) preserved as-is.
                </div>
              ) : (
                <div className="mt-2 max-h-72 space-y-3 overflow-y-auto border border-border bg-muted/10 p-3">
                  {RESOURCES.map((resource) => {
                    const group = permCatalog.filter(
                      (e) => e.resource === resource,
                    );
                    if (group.length === 0) return null;
                    return (
                      <div key={resource}>
                        <div className="font-mono text-[10px] uppercase text-zinc-500">
                          {shortEnum(resource)}
                        </div>
                        <div className="mt-1 flex flex-wrap gap-1.5">
                          {group.map((entry) => {
                            const active = permissionDrafts.some(
                              (p) =>
                                p.resource === entry.resource &&
                                p.action === entry.action,
                            );
                            return (
                              <Button
                                key={permKey(entry.resource, entry.action)}
                                size="sm"
                                variant={active ? "default" : "outline"}
                                onClick={() => togglePermission(entry)}
                                title={entry.label}
                              >
                                {shortEnum(entry.action)}
                              </Button>
                            );
                          })}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
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
                <Label htmlFor="transfer-owner-id">New owner</Label>
                <Select
                  value={newOwnerAccountId}
                  onValueChange={setNewOwnerAccountId}
                >
                  <SelectTrigger id="transfer-owner-id">
                    <SelectValue placeholder="Select a member" />
                  </SelectTrigger>
                  <SelectContent>
                    {orgDetail.memberships
                      .filter(
                        (m) => m.account.id !== orgDetail.tenant.ownerAccountId,
                      )
                      .map((m) => (
                        <SelectItem key={m.account.id} value={m.account.id}>
                          {m.account.nickname} — {m.account.email}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
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

      <Dialog open={detailOpen} onOpenChange={setDetailOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{detailTitle}</DialogTitle>
            <DialogDescription>
              {detailRole ? "GetRoleResponse" : "GetMembershipResponse"}
            </DialogDescription>
          </DialogHeader>
          {detailRole && (
            <div className="space-y-3">
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 text-xs">
                <div className="border border-border bg-muted/20 px-3 py-2">
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    name
                  </div>
                  <div className="text-foreground">{detailRole.name}</div>
                </div>
                <div className="border border-border bg-muted/20 px-3 py-2">
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    scope
                  </div>
                  <div className="font-mono text-foreground">
                    {detailRole.scope}
                  </div>
                </div>
                <div className="border border-border bg-muted/20 px-3 py-2 sm:col-span-2">
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    id
                  </div>
                  <div className="font-mono text-foreground">{detailRole.id}</div>
                </div>
              </div>
              <div>
                <SectionLabel>permissions</SectionLabel>
                <div className="mt-2">
                  <PermissionBadges permissions={detailRole.permissions} />
                </div>
              </div>
            </div>
          )}
          {detailMembership && (
            <div className="grid grid-cols-1 gap-2 text-xs">
              {(
                [
                  ["id", detailMembership.id],
                  ["account_id", detailMembership.accountId],
                  ["tenant_id", detailMembership.tenantId],
                  ["role_ids", detailMembership.roleIds.join(", ") || "none"],
                  ["created", fmtDate(detailMembership.createdAt)],
                  ["updated", fmtDate(detailMembership.updatedAt)],
                ] as const
              ).map(([label, value]) => (
                <div
                  key={label}
                  className="border border-border bg-muted/20 px-3 py-2"
                >
                  <div className="font-mono text-[10px] uppercase text-zinc-500">
                    {label}
                  </div>
                  <div className="break-all font-mono text-foreground">
                    {value}
                  </div>
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}
