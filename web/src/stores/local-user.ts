import { defineStore } from "pinia"
import { apiGet } from "@services/api"
import { mergeCollectibleStates, type PlayerCollectibleDetail, type PlayerCollectibleState } from "@stores/collectibles"

export type { PlayerCollectibleDetail }

export type PublicUserProfile = {
  userId: number
  username: string
  usernameChangeId?: number
  derived?: Record<string, number>
  attributes?: Record<string, Record<string, unknown>>
  attributeProgress?: Record<string, Record<string, unknown>>
  equipped?: Array<Record<string, unknown>>
  updatedAt?: string
  basicAttack?: Record<string, unknown>
  formula?: { derived?: Record<string, string> }
	customSkills?: PlayerProfileCustomSkill[]
}

export type PlayerProfileKitty = {
  id: number
  name: string
  level: number
  status: string
  mood: number
}

export type PlayerProfileSkill = {
  skillId: string
  level: number
  enabled: boolean
  castOrder: number
}

export type PlayerProfileCustomSkill = {
	id: string
	sourceKind: "blank" | "official"
	sourceSkillId?: string
	compiledSkill?: Record<string, unknown>
	budget?: Record<string, unknown>
}

export type CatVillageReputationState = {
  totalReputation: number
  level: number
  levelReputation: number
  nextLevelXp: number
}

export type PlayerNameHistoryEntry = {
  oldName: string
}

export type PlayerProfile = {
  userId: number
  username: string
  usernameChangeId?: number
  nameHistory: PlayerNameHistoryEntry[]
  nameHistoryHasMore?: boolean
  titlePrefixId?: string
  titleSuffixId?: string
  kitties?: PlayerProfileKitty[]
  skills?: PlayerProfileSkill[]
	customSkills?: PlayerProfileCustomSkill[]
  collectiblesCount?: number
  achievementsCompleted?: number
  achievementsTotal?: number
  reputation?: CatVillageReputationState
}

export type PlayerProfileAchievement = {
  id: string
  nameKey: string
  descKey: string
  rarity: string
  iconId: string
  categoryId: string
  condition: Record<string, unknown>
  completedAt?: string
}

type PublicProfileCacheEntry = {
  profile: PublicUserProfile
  cachedAt: number
}

type ProfileCacheEntry = {
  profile: PlayerProfile
  cachedAt: number
}

type CollectiblesCacheEntry = {
  collectibles: PlayerCollectibleDetail[]
  cachedAt: number
}

type AchievementsCacheEntry = {
  achievements: PlayerProfileAchievement[]
  cachedAt: number
}

const inflight = new Map<number, Promise<PublicUserProfile>>()
const profileInflight = new Map<number, Promise<PlayerProfile>>()
const collectiblesInflight = new Map<number, Promise<PlayerCollectibleDetail[]>>()
const achievementsInflight = new Map<number, Promise<PlayerProfileAchievement[]>>()
const PROFILE_CACHE_TTL_MS = 5 * 60 * 1000

export const useLocalUserStore = defineStore("local-user", {
  state: () => ({
    byUserID: {} as Record<number, PublicProfileCacheEntry>,
    profileByUserID: {} as Record<number, ProfileCacheEntry>,
    latestProfileChangeIdByUserID: {} as Record<number, number>,
    authoritativeProfileUsernameByUserID: {} as Record<number, string>,
    collectiblesByUserID: {} as Record<number, CollectiblesCacheEntry>,
    achievementsByUserID: {} as Record<number, AchievementsCacheEntry>
  }),
  actions: {
    applyAuthoritativeProfileUsername(userId: number, username: string, changeId = 0): boolean {
      const normalizedID = Math.floor(Number(userId || 0))
      const normalizedName = String(username || "").trim()
      if (normalizedID <= 0 || !normalizedName) return false
      const normalizedChangeID = Math.max(0, Math.floor(Number(changeId || 0)))
      const currentName = String(this.authoritativeProfileUsernameByUserID[normalizedID] || "").trim()
      const currentChangeID = Math.max(0, Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0)))
      if (currentName) {
        if (normalizedChangeID < currentChangeID) return false
        if (normalizedChangeID === currentChangeID && normalizedName !== currentName) return false
      }
      this.authoritativeProfileUsernameByUserID[normalizedID] = normalizedName
      this.latestProfileChangeIdByUserID[normalizedID] = normalizedChangeID
      return true
    },
    setPreview(userId: number, username: string) {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return
      const normalizedName = String(username || "").trim()
      if (!normalizedName) return
      const existingEntry = this.byUserID[normalizedID]
      if (this.authoritativeProfileUsernameByUserID[normalizedID]) return
      const merged = { ...(existingEntry?.profile || {}), userId: normalizedID, username: normalizedName }
      // Preserve cachedAt from existing full-data entries. New preview-only entries get cachedAt=0
      // so fetchByUserId treats them as expired and will always fetch the complete profile.
      this.byUserID[normalizedID] = { profile: merged, cachedAt: existingEntry?.cachedAt ?? 0 }
    },
    applyProfileChanged(
      userId: number,
      username: string,
      changeId = 0,
      previousUsername = ""
    ): boolean {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return false
      const normalizedName = String(username || "").trim()
      if (!normalizedName) {
        delete this.byUserID[normalizedID]
        delete this.profileByUserID[normalizedID]
        return false
      }
      const previousAuthoritativeChangeID = Math.max(
        0,
        Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
      )
      if (!this.applyAuthoritativeProfileUsername(normalizedID, normalizedName, changeId)) return false
      const authoritativeChangeID = this.latestProfileChangeIdByUserID[normalizedID] || 0
      const revisionAdvanced = authoritativeChangeID > previousAuthoritativeChangeID

      const publicEntry = this.byUserID[normalizedID]
      this.byUserID[normalizedID] = {
        profile: {
          ...(publicEntry?.profile || {}),
          userId: normalizedID,
          username: normalizedName,
          usernameChangeId: authoritativeChangeID
        },
        cachedAt: publicEntry?.cachedAt ?? 0
      }

      const profileEntry = this.profileByUserID[normalizedID]
      if (!profileEntry) return true
      const cachedUsername = String(profileEntry.profile.username || "").trim()
      const previousName = String(previousUsername || "").trim()
      const canPrependHistory = previousName !== "" && previousName !== normalizedName
      if (cachedUsername === normalizedName && !canPrependHistory) {
        this.profileByUserID[normalizedID] = {
          profile: { ...profileEntry.profile, usernameChangeId: authoritativeChangeID },
          cachedAt: revisionAdvanced ? 0 : profileEntry.cachedAt
        }
        return true
      }
      const knownHistory = [...(profileEntry.profile.nameHistory || [])]
      const shouldPrependHistory = canPrependHistory && knownHistory[0]?.oldName !== previousName
      const combinedHistory = shouldPrependHistory ? [{ oldName: previousName }, ...knownHistory] : knownHistory
      const historyHasMore = Boolean(profileEntry.profile.nameHistoryHasMore) || combinedHistory.length > 50
      this.profileByUserID[normalizedID] = {
        profile: {
          ...profileEntry.profile,
          username: normalizedName,
          usernameChangeId: authoritativeChangeID,
          // Preserve known rows while an event-only update expires the cache. The
          // rename response can prepend the missing row without a visible flicker.
          nameHistory: combinedHistory.slice(0, 50),
          nameHistoryHasMore: historyHasMore
        },
        cachedAt: canPrependHistory ? profileEntry.cachedAt : 0
      }
      return true
    },
    applyProfileUsernameSnapshot(userId: number, username: string, changeId = 0): boolean {
      const normalizedID = Math.floor(Number(userId || 0))
      const normalizedName = String(username || "").trim()
      if (normalizedID <= 0 || !normalizedName) return false
      const previousAuthoritativeChangeID = Math.max(
        0,
        Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
      )
      if (!this.applyAuthoritativeProfileUsername(normalizedID, normalizedName, changeId)) return false
      const authoritativeChangeID = this.latestProfileChangeIdByUserID[normalizedID] || 0
      const revisionAdvanced = authoritativeChangeID > previousAuthoritativeChangeID

      const publicEntry = this.byUserID[normalizedID]
      this.byUserID[normalizedID] = {
        profile: {
          ...(publicEntry?.profile || {}),
          userId: normalizedID,
          username: normalizedName,
          usernameChangeId: authoritativeChangeID
        },
        cachedAt: publicEntry?.cachedAt ?? 0
      }

      const profileEntry = this.profileByUserID[normalizedID]
      if (!profileEntry) return true
      if (profileEntry.profile.username === normalizedName) {
        this.profileByUserID[normalizedID] = {
          profile: { ...profileEntry.profile, usernameChangeId: authoritativeChangeID },
          cachedAt: revisionAdvanced ? 0 : profileEntry.cachedAt
        }
        return true
      }
      this.profileByUserID[normalizedID] = {
        profile: {
          ...profileEntry.profile,
          username: normalizedName,
          usernameChangeId: authoritativeChangeID,
          nameHistory: [...(profileEntry.profile.nameHistory || [])]
        },
        cachedAt: 0
      }
      return true
    },
    get(userId: number): PublicUserProfile | null {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return null
      return this.byUserID[normalizedID]?.profile || null
    },
    getProfile(userId: number): PlayerProfile | null {
      const normalizedID = Math.floor(Number(userId || 0))
      if (normalizedID <= 0) return null
      return this.profileByUserID[normalizedID]?.profile || null
    },
    async fetchByUserId(token: string, userId: number, options?: { force?: boolean }): Promise<PublicUserProfile> {
      const normalizedID = Math.floor(Number(userId || 0))
      if (!token || normalizedID <= 0) {
        throw new Error("invalid user profile request")
      }
      const force = Boolean(options?.force)
      if (!force) {
        const entry = this.byUserID[normalizedID]
        const authoritativeUsername = String(this.authoritativeProfileUsernameByUserID[normalizedID] || "").trim()
        const authoritativeChangeID = Math.max(
          0,
          Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
        )
        const cacheMatchesAuthority =
          !authoritativeUsername ||
          (String(entry?.profile.username || "").trim() === authoritativeUsername &&
            Math.max(0, Math.floor(Number(entry?.profile.usernameChangeId || 0))) === authoritativeChangeID)
        if (
          entry?.profile?.username &&
          cacheMatchesAuthority &&
          Date.now() - entry.cachedAt < PROFILE_CACHE_TTL_MS
        ) return entry.profile
      }
      const current = inflight.get(normalizedID)
      if (current) return current
      const task = (async () => {
        const loadedProfile = await apiGet<PublicUserProfile>(`/api/player/public/${normalizedID}`, token)
        const loadedName = String(loadedProfile.username || "").trim()
        const loadedChangeID = Math.max(0, Math.floor(Number(loadedProfile.usernameChangeId || 0)))
        const previousAuthoritativeUsername = String(this.authoritativeProfileUsernameByUserID[normalizedID] || "").trim()
        const previousAuthoritativeChangeID = Math.max(
          0,
          Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
        )
        const accepted = this.applyAuthoritativeProfileUsername(normalizedID, loadedName, loadedChangeID)
        const authoritativeUsername = String(this.authoritativeProfileUsernameByUserID[normalizedID] || loadedName).trim()
        const authoritativeChangeID = Math.max(0, Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0)))
        const authorityAdvanced =
          accepted &&
          (!previousAuthoritativeUsername ||
            authoritativeChangeID > previousAuthoritativeChangeID ||
            authoritativeUsername !== previousAuthoritativeUsername)
        if (authorityAdvanced) {
          const detailedEntry = this.profileByUserID[normalizedID]
          if (detailedEntry) {
            this.profileByUserID[normalizedID] = {
              profile: {
                ...detailedEntry.profile,
                username: authoritativeUsername,
                usernameChangeId: authoritativeChangeID,
                nameHistory: [...(detailedEntry.profile.nameHistory || [])]
              },
              cachedAt: 0
            }
          }
        }
        const profile = {
          ...loadedProfile,
          username: authoritativeUsername,
          usernameChangeId: authoritativeChangeID
        }
        this.byUserID[normalizedID] = { profile, cachedAt: accepted ? Date.now() : 0 }
        return profile
      })()
      inflight.set(normalizedID, task)
      try {
        return await task
      } finally {
        inflight.delete(normalizedID)
      }
    },
    async fetchProfileByUserId(token: string, userId: number, options?: { force?: boolean }): Promise<PlayerProfile> {
      const normalizedID = Math.floor(Number(userId || 0))
      if (!token || normalizedID <= 0) {
        throw new Error("invalid player profile request")
      }
      const force = Boolean(options?.force)
      if (!force) {
        const entry = this.profileByUserID[normalizedID]
        const authoritativeUsername = String(this.authoritativeProfileUsernameByUserID[normalizedID] || "").trim()
        const authoritativeChangeID = Math.max(
          0,
          Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
        )
        const cacheMatchesAuthority =
          !authoritativeUsername ||
          (String(entry?.profile.username || "").trim() === authoritativeUsername &&
            Math.max(0, Math.floor(Number(entry?.profile.usernameChangeId || 0))) === authoritativeChangeID)
        if (entry && cacheMatchesAuthority && Date.now() - entry.cachedAt < PROFILE_CACHE_TTL_MS) {
          return entry.profile
        }
      }
      const current = profileInflight.get(normalizedID)
      if (current) {
        if (!force) return current
        try {
          await current
        } catch {
          // A forced refresh still gets its own request after an older one fails.
        }
        const newerCurrent = profileInflight.get(normalizedID)
        if (newerCurrent && newerCurrent !== current) return newerCurrent
      }
      const task = (async () => {
        const loadedProfile = await apiGet<PlayerProfile>(`/api/player/profile/${normalizedID}`, token)
        const cachedProfile = this.profileByUserID[normalizedID]?.profile
        const loadedName = String(loadedProfile.username || "").trim()
        const loadedChangeID = Math.max(0, Math.floor(Number(loadedProfile.usernameChangeId || 0)))
        const previousAuthoritativeUsername = String(this.authoritativeProfileUsernameByUserID[normalizedID] || "").trim()
        const previousAuthoritativeChangeID = Math.max(
          0,
          Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0))
        )
        const accepted = this.applyAuthoritativeProfileUsername(normalizedID, loadedName, loadedChangeID)
        const authoritativeUsername = String(
          this.authoritativeProfileUsernameByUserID[normalizedID] || loadedName
        ).trim()
        const authoritativeChangeID = Math.max(0, Math.floor(Number(this.latestProfileChangeIdByUserID[normalizedID] || 0)))
        const authorityAdvanced =
          accepted &&
          (!previousAuthoritativeUsername ||
            authoritativeChangeID > previousAuthoritativeChangeID ||
            authoritativeUsername !== previousAuthoritativeUsername)
        if (authorityAdvanced) {
          const publicEntry = this.byUserID[normalizedID]
          if (publicEntry) {
            this.byUserID[normalizedID] = {
              profile: {
                ...publicEntry.profile,
                username: authoritativeUsername,
                usernameChangeId: authoritativeChangeID
              },
              cachedAt: 0
            }
          }
        }
        if (!accepted && authoritativeUsername !== "") {
          const profile = {
            ...loadedProfile,
            username: authoritativeUsername,
            usernameChangeId: authoritativeChangeID,
            nameHistory: cachedProfile?.nameHistory || [],
            nameHistoryHasMore: Boolean(cachedProfile?.nameHistoryHasMore)
          }
          this.profileByUserID[normalizedID] = { profile, cachedAt: 0 }
          return profile
        }
        const profile = {
          ...loadedProfile,
          username: authoritativeUsername,
          usernameChangeId: authoritativeChangeID
        }
        this.profileByUserID[normalizedID] = { profile, cachedAt: Date.now() }
        return profile
      })()
      profileInflight.set(normalizedID, task)
      try {
        return await task
      } finally {
        if (profileInflight.get(normalizedID) === task) profileInflight.delete(normalizedID)
      }
    },
    async fetchProfileCollectibles(token: string, userId: number, options?: { force?: boolean }): Promise<PlayerCollectibleDetail[]> {
      const normalizedID = Math.floor(Number(userId || 0))
      if (!token || normalizedID <= 0) {
        throw new Error("invalid collectibles request")
      }
      const force = Boolean(options?.force)
      if (!force) {
        const entry = this.collectiblesByUserID[normalizedID]
        if (entry && Date.now() - entry.cachedAt < PROFILE_CACHE_TTL_MS) {
          return entry.collectibles
        }
      }
      const current = collectiblesInflight.get(normalizedID)
      if (current) return current
      const task = (async () => {
        const result = await apiGet<{ userId: number; collectibles: PlayerCollectibleState[] }>(
          `/api/player/profile/${normalizedID}/collectibles`, token
        )
        const collectibles = mergeCollectibleStates(result.collectibles || [])
        this.collectiblesByUserID[normalizedID] = { collectibles, cachedAt: Date.now() }
        return collectibles
      })()
      collectiblesInflight.set(normalizedID, task)
      try {
        return await task
      } finally {
        collectiblesInflight.delete(normalizedID)
      }
    },
    async fetchProfileAchievements(token: string, userId: number, options?: { force?: boolean }): Promise<PlayerProfileAchievement[]> {
      const normalizedID = Math.floor(Number(userId || 0))
      if (!token || normalizedID <= 0) {
        throw new Error("invalid achievements request")
      }
      const force = Boolean(options?.force)
      if (!force) {
        const entry = this.achievementsByUserID[normalizedID]
        if (entry && Date.now() - entry.cachedAt < PROFILE_CACHE_TTL_MS) {
          return entry.achievements
        }
      }
      const current = achievementsInflight.get(normalizedID)
      if (current) return current
      const task = (async () => {
        const result = await apiGet<{ userId: number; achievements: PlayerProfileAchievement[] }>(
          `/api/player/profile/${normalizedID}/achievements`, token
        )
        const achievements = result.achievements || []
        this.achievementsByUserID[normalizedID] = { achievements, cachedAt: Date.now() }
        return achievements
      })()
      achievementsInflight.set(normalizedID, task)
      try {
        return await task
      } finally {
        achievementsInflight.delete(normalizedID)
      }
    }
  }
})
