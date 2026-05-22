import { Check, Loader2 } from "lucide-react";
import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAuth } from "@/contexts/auth-context";
import { useListQuery } from "@/hooks/use-list-query";
import { useTenantId } from "@/hooks/use-tenant-id";
import { api } from "@/lib/connect";
import { formatTimestamp } from "@/lib/format";
import { ROLE_OWNER, canAccess } from "@/lib/rbac";
import { SortOrder } from "@/lib/proto/cloud/v1/models/common_pb.ts";
import {
  SettingsItem_Key,
  SettingsItem_Part,
  YandexCloudPlatformId,
  YandexCloudZone,
  type SettingsItem,
  type SettingsItem_Value,
} from "@/lib/proto/cloud/v1/models/settings_pb.ts";
import { ListSettingsItemsRequest_SortField } from "@/lib/proto/cloud/v1/api/ui/settings_pb.ts";

type SettingKind = "bool" | "platform" | "secret" | "text" | "textarea" | "zone";

type SettingSpec = {
  description: string;
  key: SettingsItem_Key;
  kind: SettingKind;
  label: string;
  placeholder?: string;
};

type SaveState = {
  error?: string;
  saved?: boolean;
  saving?: boolean;
};

const YANDEX_SETTINGS: SettingSpec[] = [
  {
    description: "OAuth or service account token used by the control plane.",
    key: SettingsItem_Key.YANDEX_CLOUD_TOKEN,
    kind: "secret",
    label: "Token",
    placeholder: "••••••••••••",
  },
  { description: "Cloud account identifier.", key: SettingsItem_Key.YANDEX_CLOUD_CLOUD_ID, kind: "text", label: "Cloud ID" },
  { description: "Folder/project resource ID.", key: SettingsItem_Key.YANDEX_CLOUD_FOLDER_ID, kind: "text", label: "Folder ID" },
  { description: "Default availability zone for generated resources.", key: SettingsItem_Key.YANDEX_CLOUD_ZONE, kind: "zone", label: "Zone" },
  { description: "Existing VPC network ID.", key: SettingsItem_Key.YANDEX_CLOUD_NETWORK_ID, kind: "text", label: "Network ID" },
  { description: "Display name for the VPC network.", key: SettingsItem_Key.YANDEX_CLOUD_NETWORK_NAME, kind: "text", label: "Network name" },
  { description: "Subnet CIDR used for generated subnets.", key: SettingsItem_Key.YANDEX_CLOUD_SUBNET_CIDR, kind: "text", label: "Subnet CIDR" },
  { description: "VM platform architecture tier.", key: SettingsItem_Key.YANDEX_CLOUD_PLATFORM_ID, kind: "platform", label: "Platform" },
  { description: "Compute image identifier for provisioned hosts.", key: SettingsItem_Key.YANDEX_CLOUD_IMAGE_ID, kind: "text", label: "Image ID" },
  { description: "Assign public IPs to provisioned hosts.", key: SettingsItem_Key.YANDEX_CLOUD_ASSIGN_PUBLIC_IP, kind: "bool", label: "Assign public IP" },
  {
    description: "Enable software accelerated networking.",
    key: SettingsItem_Key.YANDEX_CLOUD_SOFTWARE_ACCELERATED_NETWORK,
    kind: "bool",
    label: "Software accelerated network",
  },
  { description: "SSH username installed on provisioned hosts.", key: SettingsItem_Key.YANDEX_CLOUD_SSH_USER, kind: "text", label: "SSH user" },
  {
    description: "SSH public key authorized on provisioned hosts.",
    key: SettingsItem_Key.YANDEX_CLOUD_SSH_PUBLIC_KEY,
    kind: "textarea",
    label: "SSH public key",
    placeholder: "ssh-ed25519 ...",
  },
];

const SECRET_KEYS = new Set<SettingsItem_Key>([SettingsItem_Key.YANDEX_CLOUD_TOKEN]);

export function SettingsPage() {
  const tenantId = useTenantId();
  const { account, roleForTenant } = useAuth();
  const canWrite = Boolean(account?.isAdmin) || canAccess(roleForTenant(tenantId), ROLE_OWNER);
  const [drafts, setDrafts] = useState<Record<number, string | boolean>>({});
  const [saveState, setSaveState] = useState<Record<number, SaveState>>({});

  const result = useListQuery(
    () =>
      api.settings.listSettingsItems({
        tenantId: { value: tenantId },
        part: SettingsItem_Part.YANDEX_CLOUD,
        sortField: ListSettingsItemsRequest_SortField.KEY,
        order: SortOrder.ASC,
        page: { size: 100, token: "" },
      }),
    [tenantId],
  );

  const itemsByKey = useMemo(() => {
    const map = new Map<SettingsItem_Key, SettingsItem>();
    for (const item of result.data?.settingsItems ?? []) {
      map.set(item.key, item);
    }
    return map;
  }, [result.data?.settingsItems]);

  async function save(spec: SettingSpec) {
    const value = drafts[spec.key] ?? valueFromItem(itemsByKey.get(spec.key), spec);
    setSaveState((current) => ({ ...current, [spec.key]: { saving: true } }));

    try {
      await api.settings.setSettingsItem({
        tenantId: { value: tenantId },
        part: SettingsItem_Part.YANDEX_CLOUD,
        key: spec.key,
        value: valueMessage(spec, value),
      });
      setDrafts((current) => {
        const next = { ...current };
        delete next[spec.key];
        return next;
      });
      setSaveState((current) => ({ ...current, [spec.key]: { saved: true } }));
    } catch (error) {
      setSaveState((current) => ({
        ...current,
        [spec.key]: { error: error instanceof Error ? error.message : "Save failed" },
      }));
    }
  }

  return (
    <section className="flex min-h-0 flex-col gap-5 p-6">
      <div>
        <h1 className="text-xl font-semibold tracking-normal">Settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Tenant-scoped configuration. Values are changed in place; settings are not deleted from this UI.
        </p>
      </div>

      {result.error ? <div className="border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">{result.error}</div> : null}
      {!canWrite ? <div className="border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">Read-only: OWNER role is required to change settings.</div> : null}

      <div className="rounded-md border bg-card">
        <div className="border-b px-4 py-4">
          <h2 className="text-base font-semibold">Yandex Cloud</h2>
          <p className="mt-1 text-sm text-muted-foreground">Provider credentials and defaults used by the wizard and provisioning flow.</p>
        </div>
        <div className="divide-y">
          {YANDEX_SETTINGS.map((spec) => (
            <SettingRow
              canWrite={canWrite}
              draft={drafts[spec.key]}
              item={itemsByKey.get(spec.key)}
              key={spec.key}
              loading={result.loading}
              onChange={(value) => {
                setDrafts((current) => ({ ...current, [spec.key]: value }));
                setSaveState((current) => ({ ...current, [spec.key]: {} }));
              }}
              onSave={() => save(spec)}
              saveState={saveState[spec.key]}
              spec={spec}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

function SettingRow({
  canWrite,
  draft,
  item,
  loading,
  onChange,
  onSave,
  saveState,
  spec,
}: {
  canWrite: boolean;
  draft: string | boolean | undefined;
  item?: SettingsItem;
  loading: boolean;
  onChange: (value: string | boolean) => void;
  onSave: () => void;
  saveState?: SaveState;
  spec: SettingSpec;
}) {
  const current = valueFromItem(item, spec);
  const value = draft ?? current;
  const dirty = draft !== undefined && draft !== current;
  const disabled = !canWrite || loading || Boolean(saveState?.saving);

  return (
    <div className="grid gap-3 px-4 py-4 lg:grid-cols-[minmax(13rem,18rem)_minmax(0,1fr)_7rem] lg:items-start">
      <div>
        <div className="text-sm font-medium">{spec.label}</div>
        <div className="mt-1 text-xs leading-5 text-muted-foreground">{spec.description}</div>
        {item?.timestamps?.updatedAt ? <div className="mt-1 text-[11px] text-muted-foreground">Updated {formatTimestamp(item.timestamps.updatedAt)}</div> : null}
      </div>

      <div>
        <SettingInput disabled={disabled} onChange={onChange} spec={spec} value={value} />
        {saveState?.error ? <div className="mt-2 text-xs text-danger">{saveState.error}</div> : null}
      </div>

      <div className="flex justify-start lg:justify-end">
        <Button disabled={disabled || !dirty} onClick={onSave} size="sm" variant={dirty ? "default" : "outline"}>
          {saveState?.saving ? <Loader2 className="animate-spin" /> : saveState?.saved ? <Check /> : null}
          Save
        </Button>
      </div>
    </div>
  );
}

function SettingInput({
  disabled,
  onChange,
  spec,
  value,
}: {
  disabled: boolean;
  onChange: (value: string | boolean) => void;
  spec: SettingSpec;
  value: string | boolean;
}) {
  if (spec.kind === "bool") {
    return (
      <label className="flex h-9 items-center gap-3 text-sm">
        <Checkbox checked={Boolean(value)} disabled={disabled} onCheckedChange={(next) => onChange(Boolean(next))} />
        <span className="text-muted-foreground">{Boolean(value) ? "Enabled" : "Disabled"}</span>
      </label>
    );
  }

  if (spec.kind === "zone") {
    return (
      <Select disabled={disabled} onValueChange={(next) => onChange(next)} value={String(value || YandexCloudZone.UNSPECIFIED)}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={String(YandexCloudZone.UNSPECIFIED)}>Not set</SelectItem>
          <SelectItem value={String(YandexCloudZone.RU_CENTRAL1_A)}>ru-central1-a</SelectItem>
          <SelectItem value={String(YandexCloudZone.RU_CENTRAL1_B)}>ru-central1-b</SelectItem>
          <SelectItem value={String(YandexCloudZone.RU_CENTRAL1_D)}>ru-central1-d</SelectItem>
        </SelectContent>
      </Select>
    );
  }

  if (spec.kind === "platform") {
    return (
      <Select disabled={disabled} onValueChange={(next) => onChange(next)} value={String(value || YandexCloudPlatformId.UNSPECIFIED)}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={String(YandexCloudPlatformId.UNSPECIFIED)}>Not set</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.STANDARD_V1)}>standard-v1</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.STANDARD_V2)}>standard-v2</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.STANDARD_V3)}>standard-v3</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.STANDARD_V4A)}>standard-v4a</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.AMD_V1)}>amd-v1</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.HIGHFREQ_V3)}>highfreq-v3</SelectItem>
          <SelectItem value={String(YandexCloudPlatformId.HIGHFREQ_V4A)}>highfreq-v4a</SelectItem>
        </SelectContent>
      </Select>
    );
  }

  if (spec.kind === "textarea") {
    return (
      <textarea
        className="min-h-24 w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        placeholder={spec.placeholder}
        value={String(value)}
      />
    );
  }

  return (
    <Input
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
      placeholder={spec.placeholder}
      type={spec.kind === "secret" ? "password" : "text"}
      value={String(value)}
    />
  );
}

function valueFromItem(item: SettingsItem | undefined, spec: SettingSpec): string | boolean {
  if (!item?.value || SECRET_KEYS.has(spec.key)) return spec.kind === "bool" ? false : "";
  const value = item.value.value;
  switch (value.case) {
    case "boolValue":
      return value.value;
    case "yandexCloudPlatformId":
    case "yandexCloudZone":
      return String(value.value);
    case "stringValue":
      return value.value;
    default:
      return spec.kind === "bool" ? false : "";
  }
}

function valueMessage(spec: SettingSpec, value: string | boolean): SettingsItem_Value {
  if (spec.kind === "bool") {
    return { value: { case: "boolValue", value: Boolean(value) } } as SettingsItem_Value;
  }
  if (spec.kind === "platform") {
    return { value: { case: "yandexCloudPlatformId", value: Number(value) as YandexCloudPlatformId } } as SettingsItem_Value;
  }
  if (spec.kind === "zone") {
    return { value: { case: "yandexCloudZone", value: Number(value) as YandexCloudZone } } as SettingsItem_Value;
  }
  return { value: { case: "stringValue", value: String(value) } } as SettingsItem_Value;
}
