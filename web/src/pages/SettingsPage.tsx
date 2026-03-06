import React, { useEffect, useState } from "react"
import { useStore } from "@nanostores/react"
import { motion } from "framer-motion"
import { Save, Loader2, AlertCircle, Check, Server, Container, Cloud, Wrench } from "lucide-react"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  $settings,
  $settingsLoading,
  $settingsSaving,
  $settingsError,
  loadSettings,
  saveSettings,
  type Settings,
  type HatchetConnection,
  type DockerSettings,
  type YandexCloudSettings,
} from "@/stores/settings"

function FieldGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </label>
      {children}
    </div>
  )
}

function SectionCard({ title, icon: Icon, children }: { title: string; icon: React.ComponentType<{ className?: string }>; children: React.ReactNode }) {
  return (
    <div className="border border-border bg-card">
      <div className="flex items-center gap-2 border-b border-border bg-secondary/20 px-3 py-2">
        <Icon className="h-3.5 w-3.5 text-primary" />
        <span className="text-[12px] font-medium">{title}</span>
      </div>
      <div className="p-4 space-y-3">
        {children}
      </div>
    </div>
  )
}

function GeneralTab({ settings, onChange }: { settings: Settings; onChange: (s: Settings) => void }) {
  return (
    <div className="space-y-4">
      <SectionCard title="Deployment Target" icon={Wrench}>
        <FieldGroup label="Preferred Target">
          <select
            value={settings.preferredTarget}
            onChange={(e) => onChange({ ...settings, preferredTarget: e.target.value })}
            className="flex h-8 w-full border border-input bg-background px-2 py-1 text-[13px] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            <option value="TARGET_DOCKER">Docker</option>
            <option value="TARGET_YANDEX_CLOUD">Yandex Cloud</option>
          </select>
        </FieldGroup>
        <FieldGroup label="Cleanup Delay">
          <Input
            value={settings.cleanupDelay ?? ""}
            onChange={(e) => onChange({ ...settings, cleanupDelay: e.target.value || undefined })}
            placeholder="30s"
            className="bg-background max-w-[200px]"
          />
          <p className="text-[11px] text-muted-foreground">Delay before destroying resources after test completion (e.g. 30s, 5m)</p>
        </FieldGroup>
      </SectionCard>
    </div>
  )
}

function HatchetTab({ connection, onChange }: { connection: HatchetConnection; onChange: (c: HatchetConnection) => void }) {
  return (
    <div className="space-y-4">
      <SectionCard title="Hatchet Connection" icon={Server}>
        <FieldGroup label="Host">
          <Input
            value={connection.host}
            onChange={(e) => onChange({ ...connection, host: e.target.value })}
            placeholder="localhost"
            className="bg-background"
          />
        </FieldGroup>
        <FieldGroup label="Port">
          <Input
            type="number"
            value={connection.port}
            onChange={(e) => onChange({ ...connection, port: parseInt(e.target.value) || 0 })}
            placeholder="7077"
            className="bg-background max-w-[140px]"
          />
        </FieldGroup>
        <FieldGroup label="API Token">
          <Input
            type="password"
            value={connection.token}
            onChange={(e) => onChange({ ...connection, token: e.target.value })}
            placeholder="Enter Hatchet API token"
            className="bg-background font-mono"
          />
        </FieldGroup>
      </SectionCard>
    </div>
  )
}

function DockerTab({ docker, onChange }: { docker: DockerSettings; onChange: (d: DockerSettings) => void }) {
  return (
    <div className="space-y-4">
      <SectionCard title="Docker Configuration" icon={Container}>
        <FieldGroup label="Network Name">
          <Input
            value={docker.networkName}
            onChange={(e) => onChange({ ...docker, networkName: e.target.value })}
            placeholder="stroppy-net"
            className="bg-background"
          />
        </FieldGroup>
        <FieldGroup label="Edge Worker Image">
          <Input
            value={docker.edgeWorkerImage}
            onChange={(e) => onChange({ ...docker, edgeWorkerImage: e.target.value })}
            placeholder="stroppy-edge:latest"
            className="bg-background font-mono"
          />
        </FieldGroup>
        <div className="grid grid-cols-2 gap-3">
          <FieldGroup label="Network CIDR">
            <Input
              value={docker.networkCidr}
              onChange={(e) => onChange({ ...docker, networkCidr: e.target.value })}
              placeholder="172.28.0.0/16"
              className="bg-background font-mono"
            />
          </FieldGroup>
          <FieldGroup label="Network Prefix">
            <Input
              type="number"
              value={docker.networkPrefix}
              onChange={(e) => onChange({ ...docker, networkPrefix: parseInt(e.target.value) || 0 })}
              placeholder="24"
              className="bg-background max-w-[100px]"
            />
          </FieldGroup>
        </div>
      </SectionCard>
    </div>
  )
}

function YandexTab({ yandex, onChange }: { yandex?: YandexCloudSettings; onChange: (y: YandexCloudSettings) => void }) {
  const settings = yandex ?? {
    providerSettings: { token: "", cloudId: "", folderId: "", zone: "ru-central1-a" },
    networkSettings: { name: "", externalId: "" },
    vmSettings: { baseImageId: "", enablePublicIps: false, platformId: "standard-v2" },
  }

  const updateProvider = (patch: Partial<typeof settings.providerSettings>) =>
    onChange({ ...settings, providerSettings: { ...settings.providerSettings, ...patch } })
  const updateNetwork = (patch: Partial<typeof settings.networkSettings>) =>
    onChange({ ...settings, networkSettings: { ...settings.networkSettings, ...patch } })
  const updateVm = (patch: Partial<typeof settings.vmSettings>) =>
    onChange({ ...settings, vmSettings: { ...settings.vmSettings, ...patch } })

  return (
    <div className="space-y-4">
      <SectionCard title="Provider" icon={Cloud}>
        <FieldGroup label="OAuth Token">
          <Input
            type="password"
            value={settings.providerSettings.token}
            onChange={(e) => updateProvider({ token: e.target.value })}
            placeholder="Enter Yandex Cloud OAuth token"
            className="bg-background font-mono"
          />
        </FieldGroup>
        <div className="grid grid-cols-2 gap-3">
          <FieldGroup label="Cloud ID">
            <Input
              value={settings.providerSettings.cloudId}
              onChange={(e) => updateProvider({ cloudId: e.target.value })}
              placeholder="b1g..."
              className="bg-background font-mono"
            />
          </FieldGroup>
          <FieldGroup label="Folder ID">
            <Input
              value={settings.providerSettings.folderId}
              onChange={(e) => updateProvider({ folderId: e.target.value })}
              placeholder="b1g..."
              className="bg-background font-mono"
            />
          </FieldGroup>
        </div>
        <FieldGroup label="Zone">
          <Input
            value={settings.providerSettings.zone}
            onChange={(e) => updateProvider({ zone: e.target.value })}
            placeholder="ru-central1-a"
            className="bg-background max-w-[200px]"
          />
        </FieldGroup>
      </SectionCard>

      <SectionCard title="Network" icon={Server}>
        <div className="grid grid-cols-2 gap-3">
          <FieldGroup label="Network Name">
            <Input
              value={settings.networkSettings.name}
              onChange={(e) => updateNetwork({ name: e.target.value })}
              placeholder="stroppy-network"
              className="bg-background"
            />
          </FieldGroup>
          <FieldGroup label="External Network ID">
            <Input
              value={settings.networkSettings.externalId}
              onChange={(e) => updateNetwork({ externalId: e.target.value })}
              placeholder="enp..."
              className="bg-background font-mono"
            />
          </FieldGroup>
        </div>
      </SectionCard>

      <SectionCard title="Virtual Machines" icon={Container}>
        <FieldGroup label="Base Image ID">
          <Input
            value={settings.vmSettings.baseImageId}
            onChange={(e) => updateVm({ baseImageId: e.target.value })}
            placeholder="fd8..."
            className="bg-background font-mono"
          />
        </FieldGroup>
        <FieldGroup label="Platform ID">
          <Input
            value={settings.vmSettings.platformId}
            onChange={(e) => updateVm({ platformId: e.target.value })}
            placeholder="standard-v2"
            className="bg-background max-w-[200px]"
          />
        </FieldGroup>
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="enablePublicIps"
            checked={settings.vmSettings.enablePublicIps}
            onChange={(e) => updateVm({ enablePublicIps: e.target.checked })}
            className="h-3.5 w-3.5 accent-primary"
          />
          <label htmlFor="enablePublicIps" className="text-[12px] text-foreground cursor-pointer">
            Enable public IPs
          </label>
        </div>
        {settings.vmSettings.vmUser && (
          <>
            <FieldGroup label="VM Username">
              <Input
                value={settings.vmSettings.vmUser.username}
                onChange={(e) => updateVm({ vmUser: { ...settings.vmSettings.vmUser!, username: e.target.value } })}
                className="bg-background"
              />
            </FieldGroup>
            <FieldGroup label="SSH Public Key">
              <textarea
                value={settings.vmSettings.vmUser.sshPublicKey}
                onChange={(e) => updateVm({ vmUser: { ...settings.vmSettings.vmUser!, sshPublicKey: e.target.value } })}
                rows={3}
                className="flex w-full border border-input bg-background px-2 py-1.5 text-[12px] font-mono focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-none"
              />
            </FieldGroup>
          </>
        )}
        {!settings.vmSettings.vmUser && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => updateVm({ vmUser: { username: "ubuntu", sshPublicKey: "" } })}
          >
            Add VM User
          </Button>
        )}
      </SectionCard>
    </div>
  )
}

export function SettingsPage() {
  const settings = useStore($settings)
  const loading = useStore($settingsLoading)
  const saving = useStore($settingsSaving)
  const error = useStore($settingsError)
  const [draft, setDraft] = useState<Settings>(settings)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    loadSettings()
  }, [])

  useEffect(() => {
    setDraft(settings)
  }, [settings])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaved(false)
    try {
      await saveSettings(draft)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch {
      // error is set in store
    }
  }

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const tabItems = [
    { value: "general", label: "General", icon: Wrench },
    { value: "hatchet", label: "Hatchet", icon: Server },
    { value: "docker", label: "Docker", icon: Container },
    { value: "yandex", label: "Yandex Cloud", icon: Cloud },
  ]

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex h-full flex-col"
    >
      <form onSubmit={handleSave} className="flex flex-col h-full">
        {/* Settings toolbar */}
        <div className="flex items-center justify-between border-b border-border bg-secondary/20 px-4 py-2">
          <h1 className="text-[14px] font-semibold">Settings</h1>
          <div className="flex items-center gap-2">
            {error && (
              <div className="flex items-center gap-1.5 text-[12px] text-destructive">
                <AlertCircle className="h-3.5 w-3.5" />
                <span>{error}</span>
              </div>
            )}
            {saved && (
              <motion.div
                initial={{ opacity: 0, x: 8 }}
                animate={{ opacity: 1, x: 0 }}
                className="flex items-center gap-1.5 text-[12px] text-chart-2"
              >
                <Check className="h-3.5 w-3.5" />
                <span>Saved</span>
              </motion.div>
            )}
            <Button type="submit" size="sm" disabled={saving}>
              {saving ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <Save className="h-3.5 w-3.5" />
              )}
              {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        </div>

        {/* Tabbed content */}
        <Tabs defaultValue="general" className="flex-1 flex flex-col min-h-0">
          <TabsList>
            {tabItems.map((tab) => {
              const Icon = tab.icon
              return (
                <TabsTrigger key={tab.value} value={tab.value} className="gap-1.5">
                  <Icon className="h-3 w-3" />
                  {tab.label}
                </TabsTrigger>
              )
            })}
          </TabsList>

          <div className="flex-1 overflow-auto p-4">
            <div className="max-w-2xl">
              <TabsContent value="general">
                <GeneralTab settings={draft} onChange={setDraft} />
              </TabsContent>
              <TabsContent value="hatchet">
                <HatchetTab
                  connection={draft.hatchetConnection}
                  onChange={(c) => setDraft({ ...draft, hatchetConnection: c })}
                />
              </TabsContent>
              <TabsContent value="docker">
                <DockerTab
                  docker={draft.docker}
                  onChange={(d) => setDraft({ ...draft, docker: d })}
                />
              </TabsContent>
              <TabsContent value="yandex">
                <YandexTab
                  yandex={draft.yandexCloud}
                  onChange={(y) => setDraft({ ...draft, yandexCloud: y })}
                />
              </TabsContent>
            </div>
          </div>
        </Tabs>
      </form>
    </motion.div>
  )
}
