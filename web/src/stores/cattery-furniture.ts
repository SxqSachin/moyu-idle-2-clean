import { defineStore } from 'pinia'
import { ref } from 'vue'
import { APIClientError } from '@services/api'
import { getFurniture, commandFurniture, type FurnitureProjection, type FurnitureCommand } from '@services/cattery-furniture'

export const useCatteryFurnitureStore = defineStore('cattery-furniture', () => {
  const projection = ref<FurnitureProjection | null>(null)
  const busy = ref(false)
  const loading = ref(false)
  const stale = ref(false)
  const errorCode = ref('')
  const pending = ref<FurnitureCommand | null>(null)
  let token = '', epoch = 0, receivedAt = 0, receivedWall = 0
  let request: Promise<void> | null = null
  let requestDetails = false
  function bind(next: string) {
    if (token === next) return
    token = next; epoch++; projection.value = null; pending.value = null
    busy.value = false; loading.value = false; stale.value = false; errorCode.value = ''; request = null
  }
  function apply(p: FurnitureProjection) {
    if (projection.value && (p.revision < projection.value.revision || (p.revision === projection.value.revision && Date.parse(p.serverNow) < Date.parse(projection.value.serverNow)))) return
    projection.value = p; receivedAt = performance.now(); receivedWall = Date.now(); stale.value = false
  }
  function serverNow() { return projection.value ? Date.parse(projection.value.serverNow) + Math.max(0, performance.now() - receivedAt, Date.now() - receivedWall) : Date.now() }
  function hydrate(next: string, p: FurnitureProjection) { bind(next); if (token && p && Array.isArray(p.items) && Number.isSafeInteger(p.revision) && Number.isFinite(Date.parse(p.serverNow))) apply(p) }
  function refresh(next: string, options: { force?: boolean; details?: boolean } = {}): Promise<void> {
    bind(next)
    if (!token) return Promise.resolve()
    if (request) {
      if (options.details && !requestDetails) {
        const current = epoch
        return request.then(() => { if (current === epoch) return refresh(next, options) })
      }
      return request
    }
    if (!options.force && !stale.value && projection.value && (!options.details || !projection.value.summary) && Math.max(performance.now() - receivedAt, Date.now() - receivedWall) < 30000) return Promise.resolve()
    const current = epoch, auth = token
    requestDetails = !!options.details; loading.value = true
    const task = (async () => {
      try { const p = await getFurniture(auth, !!options.details); if (current === epoch) apply(p) }
      catch { if (current === epoch) { stale.value = true; errorCode.value = 'FURNITURE_UNAVAILABLE' } }
      finally { if (current === epoch) { request = null; loading.value = false } }
    })()
    request = task; return task
  }
  async function submit(command: FurnitureCommand): Promise<boolean> {
    if (!token || busy.value || (pending.value && JSON.stringify(pending.value) !== JSON.stringify(command))) return false
    const current = epoch, auth = token
    busy.value = true; errorCode.value = ''; pending.value = command
    try {
      const p = await commandFurniture(auth, command)
      if (current !== epoch) return false
      apply(p); pending.value = null; return true
    } catch (e) {
      if (current !== epoch) return false
      errorCode.value = e instanceof APIClientError ? e.code || 'FURNITURE_UNKNOWN' : 'FURNITURE_UNKNOWN'
      // A definite business rejection did not spend materials. Network/5xx
      // results retain the exact command identity for safe reconciliation.
      if (e instanceof APIClientError && e.status && e.status < 500) pending.value = null
      stale.value = true
      return false
    } finally { if (current === epoch) busy.value = false }
  }
  return { projection, busy, loading, stale, errorCode, pending, bind, refresh, submit, serverNow, hydrate }
})
