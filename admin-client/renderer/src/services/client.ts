import type { AdminModuleName } from "@shared/admin-api"
import { getAdminBridge } from "./bridge"

export async function adminGet<T>(moduleName: AdminModuleName, resourcePath: string, query?: Record<string, unknown>) {
  return (await getAdminBridge().apiGet(moduleName, resourcePath, query)) as T
}

export async function adminPost<T>(moduleName: AdminModuleName, resourcePath: string, payload?: unknown) {
  return (await getAdminBridge().apiPost(moduleName, resourcePath, payload)) as T
}
