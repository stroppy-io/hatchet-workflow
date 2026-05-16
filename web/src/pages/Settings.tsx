import { useEffect, useState } from "react";
import { clients } from "@/api/clients";
import { getTenantId } from "@/api/transport";
import {
  SettingsItem_Key,
  SettingsItem_Part,
} from "@/lib/proto/cloud/v1/catalog/settings_pb";
import type { SettingsItem } from "@/lib/proto/cloud/v1/catalog/settings_pb";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Save, AlertCircle, Check } from "lucide-react";

// ─── Local flat shape (mirrors legacy REST type) ──────────────────

interface YcSettings {
  token: string;
  cloud_id: string;
  folder_id: string;
  zone: string;
  network_id: string;
  network_name: string;
  subnet_cidr: string;
  platform_id: string;
  image_id: string;
  assign_public_ip: boolean;
  software_accelerated_network: boolean;
  ssh_user: string;
  ssh_public_key: string;
}

interface LocalSettings {
  server_addr: string;
  binary_url: string;
  yandex: YcSettings;
}

const EMPTY_YC: YcSettings = {
  token: "", cloud_id: "", folder_id: "", zone: "", network_id: "",
  network_name: "", subnet_cidr: "", platform_id: "", image_id: "",
  assign_public_ip: false, software_accelerated_network: false,
  ssh_user: "", ssh_public_key: "",
};

const EMPTY: LocalSettings = { server_addr: "", binary_url: "", yandex: { ...EMPTY_YC } };

// ─── Proto <-> local converters ───────────────────────────────────

function itemsToLocal(items: SettingsItem[]): LocalSettings {
  const s: LocalSettings = { ...EMPTY, yandex: { ...EMPTY_YC } };
  for (const item of items) {
    const v = item.value?.value;
    const str = (v as { case?: string; value?: unknown })?.case === "stringValue"
      ? String((v as { value: unknown }).value ?? "")
      : "";
    const bool = (v as { case?: string; value?: unknown })?.case === "boolValue"
      ? Boolean((v as { value: unknown }).value)
      : false;
    switch (item.key) {
      case SettingsItem_Key.YANDEX_CLOUD_TOKEN: s.yandex.token = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_CLOUD_ID: s.yandex.cloud_id = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_FOLDER_ID: s.yandex.folder_id = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_ZONE: s.yandex.zone = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_NETWORK_ID: s.yandex.network_id = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_NETWORK_NAME: s.yandex.network_name = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_SUBNET_CIDR: s.yandex.subnet_cidr = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_PLATFORM_ID: s.yandex.platform_id = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_IMAGE_ID: s.yandex.image_id = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_ASSIGN_PUBLIC_IP: s.yandex.assign_public_ip = (v as { case?: string; value?: unknown })?.case === "boolValue" ? bool : str === "true"; break;
      case SettingsItem_Key.YANDEX_CLOUD_SOFTWARE_ACCELERATED_NETWORK: s.yandex.software_accelerated_network = (v as { case?: string; value?: unknown })?.case === "boolValue" ? bool : str === "true"; break;
      case SettingsItem_Key.YANDEX_CLOUD_SSH_USER: s.yandex.ssh_user = str; break;
      case SettingsItem_Key.YANDEX_CLOUD_SSH_PUBLIC_KEY: s.yandex.ssh_public_key = str; break;
    }
  }
  return s;
}

function localToRequests(s: LocalSettings) {
  const yc = s.yandex;
  const strItem = (key: SettingsItem_Key, val: string) => ({
    id: { value: "" },
    part: SettingsItem_Part.YANDEX_CLOUD,
    key,
    value: { value: { case: "stringValue" as const, value: val } },
  });
  const boolItem = (key: SettingsItem_Key, val: boolean) => ({
    id: { value: "" },
    part: SettingsItem_Part.YANDEX_CLOUD,
    key,
    value: { value: { case: "boolValue" as const, value: val } },
  });
  return [
    strItem(SettingsItem_Key.YANDEX_CLOUD_TOKEN, yc.token),
    strItem(SettingsItem_Key.YANDEX_CLOUD_CLOUD_ID, yc.cloud_id),
    strItem(SettingsItem_Key.YANDEX_CLOUD_FOLDER_ID, yc.folder_id),
    strItem(SettingsItem_Key.YANDEX_CLOUD_ZONE, yc.zone),
    strItem(SettingsItem_Key.YANDEX_CLOUD_NETWORK_ID, yc.network_id),
    strItem(SettingsItem_Key.YANDEX_CLOUD_NETWORK_NAME, yc.network_name),
    strItem(SettingsItem_Key.YANDEX_CLOUD_SUBNET_CIDR, yc.subnet_cidr),
    strItem(SettingsItem_Key.YANDEX_CLOUD_PLATFORM_ID, yc.platform_id),
    strItem(SettingsItem_Key.YANDEX_CLOUD_IMAGE_ID, yc.image_id),
    boolItem(SettingsItem_Key.YANDEX_CLOUD_ASSIGN_PUBLIC_IP, yc.assign_public_ip),
    boolItem(SettingsItem_Key.YANDEX_CLOUD_SOFTWARE_ACCELERATED_NETWORK, yc.software_accelerated_network),
    strItem(SettingsItem_Key.YANDEX_CLOUD_SSH_USER, yc.ssh_user),
    strItem(SettingsItem_Key.YANDEX_CLOUD_SSH_PUBLIC_KEY, yc.ssh_public_key),
  ];
}

// ─── Page ─────────────────────────────────────────────────────────

export function SettingsPage() {
  const { user } = useAuth();
  const canEdit = !!user && (user.is_root || user.role === "owner");
  const [settings, setSettings] = useState<LocalSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState<{
    type: "success" | "error";
    text: string;
  } | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const tid = getTenantId();
        const resp = await clients.settings.listSettings(
          tid ? { tenantId: { value: tid } } : {}
        );
        setSettings(itemsToLocal(resp.settingsItems ?? []));
      } catch (err) {
        setMessage({
          type: "error",
          text: err instanceof Error ? err.message : "Failed to load settings",
        });
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  async function handleSaveSettings() {
    if (!settings) return;
    setSaving(true);
    setMessage(null);
    try {
      const tid = getTenantId();
      const reqs = localToRequests(settings).map((item) => ({
        id: item.id,
        value: item.value,
        // include tenantId if scoped
        ...(tid ? { tenantId: { value: tid } } : {}),
      }));
      await clients.settings.setSettingMany({ settings: reqs });
      setMessage({ type: "success", text: "Settings saved" });
    } catch (err) {
      setMessage({
        type: "error",
        text: err instanceof Error ? err.message : "Failed to save",
      });
    }
    setSaving(false);
  }

  function setYc(key: keyof YcSettings, value: string | boolean) {
    if (!settings) return;
    setSettings({ ...settings, yandex: { ...settings.yandex, [key]: value } });
  }

  if (loading) {
    return (
      <div className="p-6 text-sm text-muted-foreground">
        Loading settings...
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h1 className="text-lg font-semibold">Settings</h1>
        <p className="text-sm text-muted-foreground">
          Server configuration and package management
        </p>
      </div>

      {message && (
        <div
          className={`flex items-center gap-2 text-sm p-3 border ${
            message.type === "success"
              ? "border-success/30 text-success"
              : "border-destructive/30 text-destructive"
          }`}
        >
          {message.type === "success" ? (
            <Check className="h-4 w-4" />
          ) : (
            <AlertCircle className="h-4 w-4" />
          )}
          {message.text}
        </div>
      )}

      <Tabs defaultValue="cloud">
        <TabsList>
          <TabsTrigger value="cloud">Cloud</TabsTrigger>
        </TabsList>

        {/* Cloud settings */}
        <TabsContent value="cloud">
          {settings && (
            <Card>
              <CardHeader>
                <CardTitle>Cloud / Server</CardTitle>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* YC Credentials */}
                <div>
                  <h3 className="text-sm font-medium mb-3">Yandex Cloud Credentials</h3>
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label>
                        Token (YC_TOKEN) <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>
                      </Label>
                      <Input
                        type="password"
                        value={settings.yandex.token}
                        onChange={(e) => setYc("token", e.target.value)}
                        disabled={!canEdit}
                        className="font-mono text-xs"
                        placeholder="OAuth or IAM token"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>
                        Cloud ID <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>
                      </Label>
                      <Input
                        value={settings.yandex.cloud_id}
                        onChange={(e) => setYc("cloud_id", e.target.value)}
                        disabled={!canEdit}
                        className="font-mono text-xs"
                      />
                    </div>
                  </div>
                </div>

                <hr className="border-border" />

                {/* YC Infrastructure */}
                <div>
                  <h3 className="text-sm font-medium mb-3">Yandex Cloud Infrastructure</h3>
                  <div className="grid grid-cols-2 gap-4">
                    {(
                      [
                        ["folder_id", "Folder ID", true],
                        ["zone", "Zone", false],
                        ["network_id", "Network ID (VPC)", true],
                        ["network_name", "Subnet Name", true],
                        ["subnet_cidr", "Subnet CIDR", true],
                        ["platform_id", "Platform ID", false],
                        ["image_id", "Image ID", true],
                      ] as const
                    ).map(([key, label, required]) => (
                      <div key={key} className="space-y-2">
                        <Label>
                          {label}
                          {required && <Badge variant="destructive" className="text-[10px] ml-1">required</Badge>}
                        </Label>
                        <Input
                          value={settings.yandex[key] as string}
                          onChange={(e) => setYc(key, e.target.value)}
                          disabled={!canEdit}
                          className="font-mono text-xs"
                        />
                      </div>
                    ))}
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>Assign Public IP</Label>
                  <div className="flex items-center gap-2 h-9">
                    <input
                      type="checkbox"
                      checked={settings.yandex.assign_public_ip}
                      onChange={(e) => setYc("assign_public_ip", e.target.checked)}
                      disabled={!canEdit}
                      className="accent-primary"
                    />
                    <span className="text-sm text-muted-foreground">
                      Allocate external IP addresses on VMs
                    </span>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>Software Accelerated Network (SAN)</Label>
                  <div className="flex items-center gap-2 h-9">
                    <input
                      type="checkbox"
                      checked={settings.yandex.software_accelerated_network}
                      onChange={(e) => setYc("software_accelerated_network", e.target.checked)}
                      disabled={!canEdit}
                      className="accent-primary"
                    />
                    <span className="text-sm text-muted-foreground">
                      Offload packet processing to dedicated host cores
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label>SSH User</Label>
                    <Input
                      value={settings.yandex.ssh_user}
                      onChange={(e) => setYc("ssh_user", e.target.value)}
                      disabled={!canEdit}
                      className="font-mono text-xs"
                      placeholder="stroppy"
                    />
                    <p className="text-[10px] text-muted-foreground">
                      Login user created on VMs (default: stroppy)
                    </p>
                  </div>
                </div>

                <div className="space-y-2">
                  <Label>SSH Public Key</Label>
                  <textarea
                    className="w-full h-20 bg-transparent border border-input p-3 font-mono text-xs resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                    value={settings.yandex.ssh_public_key}
                    onChange={(e) => setYc("ssh_public_key", e.target.value)}
                    disabled={!canEdit}
                  />
                </div>

                {canEdit && (
                  <Button onClick={handleSaveSettings} disabled={saving}>
                    <Save className="h-3.5 w-3.5" />
                    {saving ? "Saving..." : "Save Settings"}
                  </Button>
                )}
              </CardContent>
            </Card>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}
