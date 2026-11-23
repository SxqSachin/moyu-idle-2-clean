import type { AdminModuleName } from "@shared/admin-api"
import type { AdminOperationLog, AdminProfile, AdminProfileConfig } from "@shared/types"

declare global {
  interface Window {
    admin: {
      getProfiles(): Promise<AdminProfileConfig>
      saveProfile(profile: AdminProfile): Promise<AdminProfileConfig>
      deleteProfile(profileId: string): Promise<AdminProfileConfig>
      setActiveProfile(profileId: string): Promise<AdminProfile | null>
      getActiveProfile(): Promise<AdminProfile | null>
      testConnection(profileId?: string): Promise<unknown>
      apiGet(moduleName: AdminModuleName, resourcePath: string, query?: Record<string, unknown>): Promise<unknown>
      apiPost(moduleName: AdminModuleName, resourcePath: string, payload?: unknown): Promise<unknown>
      getRecentOperations(): Promise<AdminOperationLog[]>
      readGameData(tableName: string): Promise<Record<string, unknown>[]>
    }
  }
}

export {}
