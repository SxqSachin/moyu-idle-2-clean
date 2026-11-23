export type AdminProfile = {
  profileId: string
  profileName: string
  serverBaseURL: string
  releaseToken: string
  rememberToken: boolean
  timeoutMs: number
}

export type AdminProfileConfig = {
  activeProfileId: string
  profiles: AdminProfile[]
}

export type AdminOperationLog = {
  id: string
  at: string
  profileName: string
  moduleName: string
  method: string
  resourcePath: string
}

export type AdminBanReason = {
  key: string
  nameKey: string
}

export type AdminUserDetail = {
  userId: number
  username: string
  email: string
  emailVerified: boolean
  banReason: string
  banUntil?: string
  banIsPermanent: boolean
  createdAt: string
}

export type RiskSearchItem = {
  userId: number
  username: string
  clusterId?: number
  riskLevel: string
  riskScore: number
  localDeviceId: string
  fingerprintId: string
  ip: string
  occurredAt?: string
  isBanned: boolean
}

export type HighRiskUserItem = {
  user: { userId: number; username: string }
  clusterId?: number
  riskScore: number
  riskLevel: string
  clusterStatus: string
  lastSeenAt?: string
  isBanned: boolean
}

export type PaginatedResult<T> = {
  items: T[]
  total: number
  limit: number
  offset: number
}

export type ClusterSummary = {
  clusterId: number
  riskScore: number
  riskLevel: string
  status: string
  memberCount: number
  primaryUser?: { userId: number; username: string }
  updatedAt: string
}

export type CapabilityBlock = {
  capability: string
  blockLevel: string
  reasonCode: string
  source: string
}

export type ReviewCase = {
  id: number
  clusterId?: number
  userId?: number
  status: string
  decision: string
  operatorName: string
  operatorNote: string
  createdAt: string
  updatedAt: string
}

export type RiskUserDetail = {
  user: { userId: number; username: string; email: string; createdAt: string }
  context: {
    riskScore: number
    riskLevel: string
    clusterId?: number
    clusterStatus: string
  }
  blocks: CapabilityBlock[]
  signals: Array<Record<string, unknown>>
  transfers: Array<Record<string, unknown>>
  transferPeers: Array<Record<string, unknown>>
  evidences: Array<Record<string, unknown>>
  linkedUsers: Array<Record<string, unknown>>
  reviewCases: ReviewCase[]
}

export type RiskClusterDetail = {
  clusterId: number
  riskScore: number
  riskLevel: string
  status: string
  memberCount: number
  members: Array<Record<string, unknown>>
  reviewCases: ReviewCase[]
}

export type PasswordGroupMember = {
  userId: number
  username: string
  riskLevel: string
  riskScore: number
  isBanned: boolean
}

export type AuctionTransferItem = {
  id: number
  fromUser: { userId: number; username: string }
  toUser: { userId: number; username: string }
  channel: string
  referenceId: number
  itemId: string
  quantity: number
  unitPrice: number
  totalPrice: number
  feeAmount: number
  netAmount: number
  priceAnchor: number
  priceRatio: number
  occurredAt: string
  extra: Record<string, unknown>
}

export type PasswordGroup = {
  hashDigestHint: string
  users: PasswordGroupMember[]
}

export type ReleaseAnnouncement = {
  releaseId: string
  kind: "soft" | "force"
  messageI18nKey: string
  recommendedDelaySec: number
  forceAtMs?: number
  minAcceptedBuild?: string
  announcedAtMs?: number
}
