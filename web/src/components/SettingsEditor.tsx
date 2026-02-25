import { create } from '@bufbuild/protobuf'
import {
  HatchetConnectionSchema,
  DockerSettingsSchema,
  YandexCloudSettingsSchema,
  YandexCloudSettings_ProviderSettingsSchema,
  YandexCloudSettings_NetworkSettingsSchema,
  YandexCloudSettings_VmSettingsSchema
} from '../proto/settings/settings_pb'
import type { Settings } from '../proto/settings/settings_pb'
import { Target } from '../proto/deployment/deployment_pb'
import { Input } from './ui/Input'
import { Select } from './ui/Select'
import { 
  Shield, 
  Server, 
  Globe, 
  Cloud, 
  Cpu, 
  Network, 
  Key, 
  Clock,
  Settings as SettingsIcon,
  ChevronRight
} from 'lucide-react'

interface SettingsEditorProps {
  settings: Settings
  onChange: (updates: Partial<Settings>) => void
}

export const SettingsEditor = ({ settings, onChange }: SettingsEditorProps) => {
  const updateHatchet = (updates: any) => {
    onChange({ hatchetConnection: create(HatchetConnectionSchema, { ...settings.hatchetConnection, ...updates }) })
  }

  const updateDocker = (updates: any) => {
    onChange({ docker: create(DockerSettingsSchema, { ...settings.docker, ...updates }) })
  }

  const updateYandex = (updates: any) => {
    onChange({ yandexCloud: create(YandexCloudSettingsSchema, { ...settings.yandexCloud, ...updates }) })
  }

  return (
    <div className="flex-1 overflow-y-auto custom-scrollbar bg-[#1e1e1e]">
      <div className="max-w-5xl mx-auto p-10 space-y-12">
        {/* Header */}
        <div className="space-y-2">
          <h2 className="text-2xl font-bold text-[#e1e1e1] flex items-center gap-3 uppercase tracking-tight">
            <SettingsIcon className="w-6 h-6 text-primary" /> System Settings
          </h2>
          <div className="h-0.5 w-20 bg-primary/40 rounded-full" />
          <p className="text-xs text-[#858585] uppercase tracking-wider font-mono">Global Configuration Manifest</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-10">
          {/* Hatchet Connection */}
          <section className="space-y-6">
            <div className="flex items-center gap-3 text-primary/80 border-b border-[#2b2b2b] pb-2">
              <Server className="w-4 h-4" />
              <h3 className="text-[11px] font-black uppercase tracking-widest">Hatchet Orchestrator</h3>
            </div>
            <div className="space-y-4 bg-[#252526] p-6 rounded border border-[#2b2b2b] shadow-sm">
              <Input 
                label="Server Host" 
                value={settings.hatchetConnection?.host || ""} 
                onChange={(e) => updateHatchet({ host: e.target.value })} 
              />
              <Input 
                label="Port" 
                value={settings.hatchetConnection?.port.toString() || ""} 
                onChange={(e) => updateHatchet({ port: parseInt(e.target.value) || 0 })} 
              />
              <Input 
                label="Access Token" 
                value={settings.hatchetConnection?.token || ""} 
                onChange={(e) => updateHatchet({ token: e.target.value })} 
                type="password"
              />
            </div>
          </section>

          {/* Docker Settings */}
          <section className="space-y-6">
            <div className="flex items-center gap-3 text-[#4ec9b0] border-b border-[#2b2b2b] pb-2">
              <Globe className="w-4 h-4" />
              <h3 className="text-[11px] font-black uppercase tracking-widest">Docker Engine</h3>
            </div>
            <div className="space-y-4 bg-[#252526] p-6 rounded border border-[#2b2b2b] shadow-sm">
              <Input 
                label="Default Network Name" 
                value={settings.docker?.networkName || ""} 
                onChange={(e) => updateDocker({ networkName: e.target.value })} 
              />
              <Input 
                label="Edge Worker Image" 
                value={settings.docker?.edgeWorkerImage || ""} 
                onChange={(e) => updateDocker({ edgeWorkerImage: e.target.value })} 
              />
              <div className="grid grid-cols-2 gap-4">
                <Input 
                  label="Network CIDR" 
                  value={settings.docker?.networkCidr || ""} 
                  onChange={(e) => updateDocker({ networkCidr: e.target.value })} 
                  placeholder="172.28.0.0/16"
                />
                <Input 
                  label="Subnet Prefix" 
                  value={settings.docker?.networkPrefix?.toString() || ""} 
                  onChange={(e) => updateDocker({ networkPrefix: parseInt(e.target.value) || 0 })} 
                  placeholder="24"
                />
              </div>
            </div>
          </section>

          {/* Target Preference */}
          <section className="space-y-6">
            <div className="flex items-center gap-3 text-accent border-b border-[#2b2b2b] pb-2">
              <Shield className="w-4 h-4" />
              <h3 className="text-[11px] font-black uppercase tracking-widest">Deployment Strategy</h3>
            </div>
            <div className="bg-[#252526] p-6 rounded border border-[#2b2b2b] shadow-sm space-y-4">
              <Select 
                label="Preferred Runtime" 
                value={settings.preferredTarget} 
                onChange={(v) => onChange({ preferredTarget: v })} 
                options={[
                  { label: "Local Docker Engine", value: Target.DOCKER },
                  { label: "Yandex Cloud (VMs)", value: Target.YANDEX_CLOUD }
                ]}
              />
              <div className="flex items-center gap-3 p-3 bg-primary/5 border border-primary/10 rounded-sm">
                <Clock className="w-4 h-4 text-primary/60" />
                <div className="flex flex-col">
                  <span className="text-[10px] font-bold text-[#e1e1e1] uppercase">Cleanup Latency</span>
                  <span className="text-[9px] text-[#858585] font-mono">Automatic resource decommissioning delay</span>
                </div>
              </div>
            </div>
          </section>

          {/* Yandex Cloud Settings */}
          <section className="space-y-6 md:col-span-2">
            <div className="flex items-center justify-between border-b border-[#2b2b2b] pb-2">
              <div className="flex items-center gap-3 text-[#007acc]">
                <Cloud className="w-4 h-4" />
                <h3 className="text-[11px] font-black uppercase tracking-widest">Yandex Cloud Infrastructure</h3>
              </div>
              <input 
                type="checkbox" 
                checked={!!settings.yandexCloud} 
                onChange={(e) => {
                  if (e.target.checked) {
                    onChange({
                      yandexCloud: create(YandexCloudSettingsSchema, {
                        providerSettings: create(YandexCloudSettings_ProviderSettingsSchema, { zone: "ru-central1-a" }),
                        networkSettings: create(YandexCloudSettings_NetworkSettingsSchema, { name: "forge-cloud-net" }),
                        vmSettings: create(YandexCloudSettings_VmSettingsSchema, { platformId: "standard-v2", enablePublicIps: true })
                      })
                    })
                  } else {
                    onChange({ yandexCloud: undefined })
                  }
                }}
                className="accent-primary"
              />
            </div>

            {settings.yandexCloud ? (
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Provider */}
                <div className="space-y-4 bg-[#252526] p-5 rounded border border-[#2b2b2b] shadow-sm">
                  <div className="flex items-center gap-2 text-[10px] font-bold text-[#858585] uppercase tracking-tighter mb-2">
                    <Key className="w-3 h-3" /> Provider Auth
                  </div>
                  <Input 
                    label="OAuth Token" 
                    value={settings.yandexCloud.providerSettings?.token || ""} 
                    onChange={(e) => updateYandex({ providerSettings: { ...settings.yandexCloud?.providerSettings, token: e.target.value } })} 
                    type="password"
                  />
                  <Input 
                    label="Cloud ID" 
                    value={settings.yandexCloud.providerSettings?.cloudId || ""} 
                    onChange={(e) => updateYandex({ providerSettings: { ...settings.yandexCloud?.providerSettings, cloudId: e.target.value } })} 
                  />
                  <Input 
                    label="Folder ID" 
                    value={settings.yandexCloud.providerSettings?.folderId || ""} 
                    onChange={(e) => updateYandex({ providerSettings: { ...settings.yandexCloud?.providerSettings, folderId: e.target.value } })} 
                  />
                </div>

                {/* Network */}
                <div className="space-y-4 bg-[#252526] p-5 rounded border border-[#2b2b2b] shadow-sm">
                  <div className="flex items-center gap-2 text-[10px] font-bold text-[#858585] uppercase tracking-tighter mb-2">
                    <Network className="w-3 h-3" /> VPC Config
                  </div>
                  <Input 
                    label="Network Name" 
                    value={settings.yandexCloud.networkSettings?.name || ""} 
                    onChange={(e) => updateYandex({ networkSettings: { ...settings.yandexCloud?.networkSettings, name: e.target.value } })} 
                  />
                  <Input 
                    label="External ID" 
                    value={settings.yandexCloud.networkSettings?.externalId || ""} 
                    onChange={(e) => updateYandex({ networkSettings: { ...settings.yandexCloud?.networkSettings, externalId: e.target.value } })} 
                  />
                  <Select 
                    label="Region Zone" 
                    value={settings.yandexCloud.providerSettings?.zone || ""} 
                    onChange={(v) => updateYandex({ providerSettings: { ...settings.yandexCloud?.providerSettings, zone: v } })} 
                    options={[
                      { label: "ru-central1-a", value: "ru-central1-a" },
                      { label: "ru-central1-b", value: "ru-central1-b" },
                      { label: "ru-central1-c", value: "ru-central1-c" }
                    ]}
                  />
                </div>

                {/* VM Settings */}
                <div className="space-y-4 bg-[#252526] p-5 rounded border border-[#2b2b2b] shadow-sm">
                  <div className="flex items-center gap-2 text-[10px] font-bold text-[#858585] uppercase tracking-tighter mb-2">
                    <Cpu className="w-3 h-3" /> Compute Presets
                  </div>
                  <Input 
                    label="Base Image ID" 
                    value={settings.yandexCloud.vmSettings?.baseImageId || ""} 
                    onChange={(e) => updateYandex({ vmSettings: { ...settings.yandexCloud?.vmSettings, baseImageId: e.target.value } })} 
                  />
                  <Input 
                    label="Platform ID" 
                    value={settings.yandexCloud.vmSettings?.platformId || ""} 
                    onChange={(e) => updateYandex({ vmSettings: { ...settings.yandexCloud?.vmSettings, platformId: e.target.value } })} 
                  />
                  <div className="flex items-center justify-between p-2 rounded bg-[#1e1e1e] border border-[#333]">
                    <span className="text-[10px] font-bold text-[#858585] uppercase">Public IPs</span>
                    <input 
                      type="checkbox" 
                      checked={settings.yandexCloud.vmSettings?.enablePublicIps} 
                      onChange={(e) => updateYandex({ vmSettings: { ...settings.yandexCloud?.vmSettings, enablePublicIps: e.target.checked } })}
                      className="accent-primary"
                    />
                  </div>
                </div>
              </div>
            ) : (
              <div className="p-8 border border-dashed border-[#333] rounded text-center opacity-40 grayscale group hover:grayscale-0 transition-all">
                <ChevronRight className="w-8 h-8 mx-auto mb-2 text-[#858585]" />
                <p className="text-[10px] font-bold uppercase tracking-widest text-[#858585]">Cloud infrastructure not initialized</p>
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}
