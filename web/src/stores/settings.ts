import { atom } from "nanostores"
import { settingsApi } from "./api"

export interface HatchetConnection {
  token: string
  host: string
  port: number
}

export interface DockerSettings {
  networkName: string
  edgeWorkerImage: string
  networkCidr: string
  networkPrefix: number
}

export interface YandexProviderSettings {
  token: string
  cloudId: string
  folderId: string
  zone: string
}

export interface YandexNetworkSettings {
  name: string
  externalId: string
}

export interface YandexVmSettings {
  baseImageId: string
  vmUser?: { username: string; sshPublicKey: string }
  enablePublicIps: boolean
  platformId: string
}

export interface YandexCloudSettings {
  providerSettings: YandexProviderSettings
  networkSettings: YandexNetworkSettings
  vmSettings: YandexVmSettings
}

export interface Settings {
  hatchetConnection: HatchetConnection
  docker: DockerSettings
  yandexCloud?: YandexCloudSettings
  preferredTarget: string
  cleanupDelay?: string
}

const defaultSettings: Settings = {
  hatchetConnection: { token: "", host: "localhost", port: 7077 },
  docker: {
    networkName: "stroppy-net",
    edgeWorkerImage: "stroppy-edge:latest",
    networkCidr: "172.28.0.0/16",
    networkPrefix: 24,
  },
  preferredTarget: "TARGET_DOCKER",
}

export const $settings = atom<Settings>(defaultSettings)
export const $settingsLoading = atom(false)
export const $settingsSaving = atom(false)
export const $settingsError = atom("")

export async function loadSettings() {
  $settingsLoading.set(true)
  $settingsError.set("")
  try {
    const resp = await settingsApi.getSettings()
    if (resp.settings) {
      $settings.set(resp.settings as Settings)
    }
  } catch (err) {
    $settingsError.set(err instanceof Error ? err.message : "Failed to load settings")
  } finally {
    $settingsLoading.set(false)
  }
}

export async function saveSettings(settings: Settings) {
  $settingsSaving.set(true)
  $settingsError.set("")
  try {
    const resp = await settingsApi.updateSettings(settings)
    if (resp.settings) {
      $settings.set(resp.settings as Settings)
    }
  } catch (err) {
    $settingsError.set(err instanceof Error ? err.message : "Failed to save settings")
    throw err
  } finally {
    $settingsSaving.set(false)
  }
}
