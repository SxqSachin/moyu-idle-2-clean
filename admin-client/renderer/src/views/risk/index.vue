<template>
  <div class="risk-page">
    <el-tabs v-model="activeTab" type="border-card" class="risk-tabs">

      <!-- ── 概览 ────────────────────────────────────────────── -->
      <el-tab-pane label="高风险概览" name="overview">
        <div class="stack">
          <!-- 搜索 -->
          <el-card shadow="never">
            <div class="row-between">
              <span class="section-title">风险搜索</span>
              <div class="row-gap">
                <el-input v-model="query" style="width:400px" placeholder="userId / username / localDeviceId / fingerprint / ip" clearable @keyup.enter="runSearch" />
                <el-button type="primary" :loading="searching" @click="runSearch">搜索</el-button>
              </div>
            </div>
            <el-table v-if="searchItems.length" :data="searchItems" stripe size="small" class="mt-3">
              <el-table-column prop="userId" label="UserID" width="100" />
              <el-table-column label="用户名" min-width="150">
                <template #default="{ row }">
                  {{ row.username }}
                  <el-tag v-if="row.isBanned" type="danger" size="small" class="ml-1">已封禁</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="风险等级" width="110">
                <template #default="{ row }"><el-tag :type="levelTagType(row.riskLevel)" size="small">{{ levelTagLabel(row.riskLevel) }}</el-tag></template>
              </el-table-column>
              <el-table-column label="分数" width="90">
                <template #default="{ row }">
                  <el-progress :percentage="Math.min(row.riskScore, 100)" :color="scoreColor(row.riskScore)" :show-text="false" style="width:60px;display:inline-block" />
                  <span class="ml-1 text-xs">{{ row.riskScore }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="clusterId" label="关联组" width="90" />
              <el-table-column label="操作" width="240">
                <template #default="{ row }">
                  <el-button size="small" @click="openUserDetail(row.userId)">用户详情</el-button>
                  <el-button size="small" :disabled="!row.clusterId" @click="openClusterDetail(row.clusterId)">组详情</el-button>
                  <el-button size="small" type="danger" @click="openBanDialog(row.userId, row.username)">封禁</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>

          <!-- 重置 -->
          <div class="row-between mt-3">
            <span class="text-xs text-gray-400">历史数据会影响风险判断。定期重置可获得干净的基线。</span>
            <el-button size="small" type="danger" plain @click="confirmResetAll">全量重置风险数据</el-button>
          </div>

          <!-- 统计 -->
          <div class="stats-row">
            <div class="stat-card">
              <div class="stat-num">{{ highRiskUsersTotal }}</div>
              <div class="stat-label">高风险玩家</div>
            </div>
            <div class="stat-card">
              <div class="stat-num">{{ clustersTotal }}</div>
              <div class="stat-label">高风险关联组</div>
            </div>
            <div class="stat-card">
              <div class="stat-num" :class="{ 'text-red-500': openCaseCount > 0 }">{{ openCaseCount }}</div>
              <div class="stat-label">待复核案例</div>
            </div>
            <div class="stat-card">
              <div class="stat-num text-yellow-500">{{ passwordGroupsTotal }}</div>
              <div class="stat-label">密码共用组</div>
            </div>
          </div>

          <!-- 两列列表 -->
          <div class="split-grid">
            <el-card shadow="never">
              <template #header>
                <div class="row-between">
                  <span class="section-title">高风险玩家</span>
                  <el-button size="small" @click="loadHighRiskUsers">刷新</el-button>
                </div>
              </template>
              <el-table :data="highRiskUsers" stripe size="small" max-height="440">
                <el-table-column label="用户" min-width="160">
                  <template #default="{ row }">
                    <span class="user-label">{{ row.user.username }}</span>
                    <span class="uid-label">{{ row.user.userId }}</span>
                    <el-tag v-if="row.isBanned" type="danger" size="small" class="ml-1">已封禁</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="风险等级" width="105">
                  <template #default="{ row }"><el-tag :type="levelTagType(row.riskLevel)" size="small">{{ levelTagLabel(row.riskLevel) }}</el-tag></template>
                </el-table-column>
                <el-table-column label="分数" width="60">
                  <template #default="{ row }"><span class="font-mono">{{ row.riskScore }}</span></template>
                </el-table-column>
                <el-table-column label="状态" width="110">
                  <template #default="{ row }"><el-tag :type="statusTagType(row.clusterStatus)" size="small">{{ statusTagLabel(row.clusterStatus) }}</el-tag></template>
                </el-table-column>
                <el-table-column label="" width="130">
                  <template #default="{ row }">
                    <el-button size="small" link type="primary" @click="openUserDetail(row.user.userId)">详情</el-button>
                    <el-button size="small" link type="danger" @click="openBanDialog(row.user.userId, row.user.username)">封禁</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="card-footer">
                <el-pagination layout="prev, pager, next, total" :total="highRiskUsersTotal" :page-size="pageSize" :current-page="highRiskUsersPage" @current-change="onHighRiskUsersPageChange" />
              </div>
            </el-card>

            <el-card shadow="never">
              <template #header>
                <div class="row-between">
                  <span class="section-title">高风险关联组</span>
                  <el-button size="small" @click="loadClusters">刷新</el-button>
                </div>
              </template>
              <el-table :data="clusters" stripe size="small" max-height="440">
                <el-table-column prop="clusterId" label="ID" width="70" />
                <el-table-column label="风险等级" width="105">
                  <template #default="{ row }"><el-tag :type="levelTagType(row.riskLevel)" size="small">{{ levelTagLabel(row.riskLevel) }}</el-tag></template>
                </el-table-column>
                <el-table-column label="分数" width="60">
                  <template #default="{ row }"><span class="font-mono">{{ row.riskScore }}</span></template>
                </el-table-column>
                <el-table-column prop="memberCount" label="成员" width="60" />
                <el-table-column label="主用户" min-width="130">
                  <template #default="{ row }">{{ row.primaryUser?.username ?? "—" }}</template>
                </el-table-column>
                <el-table-column label="" width="70">
                  <template #default="{ row }">
                    <el-button size="small" link type="primary" @click="openClusterDetail(row.clusterId)">详情</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div class="card-footer">
                <el-pagination layout="prev, pager, next, total" :total="clustersTotal" :page-size="pageSize" :current-page="clustersPage" @current-change="onClustersPageChange" />
              </div>
            </el-card>
          </div>
        </div>
      </el-tab-pane>

      <!-- ── 用户详情 ─────────────────────────────────────────── -->
      <el-tab-pane label="用户详情" name="user">
        <el-empty v-if="!userDetail" description="从概览或搜索中打开一个用户" />
        <div v-else class="stack">
          <!-- 用户头部 -->
          <el-card shadow="never">
            <div class="row-between">
              <div>
                <div class="text-lg font-semibold">{{ userDetail.user.username }}</div>
                <div class="text-xs text-gray-400">ID {{ userDetail.user.userId }}</div>
                <div class="text-xs text-gray-400 mt-1">
                  <span>{{ userDetail.user.email || "—" }}</span>
                  <span class="ml-2">注册于 {{ formatTime(userDetail.user.createdAt) }}</span>
                </div>
              </div>
              <div class="row-gap">
                <el-button size="small" type="danger" @click="openBanDialog(userDetail.user.userId, userDetail.user.username)">封禁玩家</el-button>
                <el-button size="small" :disabled="!userDetail.context.clusterId" @click="openClusterDetail(userDetail.context.clusterId!)">查看关联组</el-button>
              </div>
            </div>
            <div class="stats-row mt-3">
              <div class="stat-card">
                <div class="stat-num">{{ userDetail.context.riskScore }}</div>
                <div class="stat-label">风险分数</div>
              </div>
              <div class="stat-card">
                <el-tag :type="levelTagType(userDetail.context.riskLevel)">{{ levelTagLabel(userDetail.context.riskLevel) }}</el-tag>
                <div class="stat-label mt-1">风险等级</div>
              </div>
              <div class="stat-card">
                <div class="stat-num text-sm">{{ userDetail.context.clusterId ?? "—" }}</div>
                <div class="stat-label">关联组</div>
              </div>
              <div class="stat-card">
                <el-tag :type="statusTagType(userDetail.context.clusterStatus)" size="small">{{ statusTagLabel(userDetail.context.clusterStatus) }}</el-tag>
                <div class="stat-label mt-1">组状态</div>
              </div>
              <div class="stat-card">
                <div class="stat-num text-blue-500">{{ multiAccountScore }}</div>
                <div class="stat-label">多开信心指数</div>
              </div>
            </div>
          </el-card>

          <!-- 多开信号分析 -->
          <el-card shadow="never" v-if="typedEvidences.length">
            <template #header><span class="section-title">多开信号分析</span></template>
            <div class="evidence-groups">
              <div v-for="group in evidenceGroups" :key="group.label" class="evidence-group">
                <div class="evidence-group-label">{{ group.label }}</div>
                <div v-for="ev in group.items" :key="ev.peer.userId + ev.evidenceType" class="evidence-item">
                  <div class="row-between">
                    <div>
                      <span class="ev-type-label">{{ evidenceLabel(ev.evidenceType) }}</span>
                      <span class="ev-peer ml-2 text-gray-400">→ {{ ev.peer.username }} ({{ ev.peer.userId }})</span>
                    </div>
                    <div class="row-gap">
                      <el-tag :type="strengthType(ev.strength)" size="small">{{ ev.strength }}</el-tag>
                      <span class="font-mono font-semibold text-sm">+{{ ev.score }}</span>
                    </div>
                  </div>
                  <div class="ev-payload">
                    <span v-for="(val, key) in ev.payload" :key="key" class="ev-kv">
                      <span class="ev-key">{{ key }}</span>
                      <span class="ev-val">{{ val }}</span>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </el-card>

          <!-- 关联对象 + 登录时间线 -->
          <div class="split-grid">
            <el-card shadow="never">
              <template #header><span class="section-title">关联对象</span></template>
              <el-table :data="typedLinkedUsers" size="small" stripe max-height="340">
                <el-table-column label="对象" min-width="150">
                  <template #default="{ row }: { row: LinkedUserItem }">
                    <span class="user-label">{{ row.peer?.username }}</span>
                    <span class="uid-label">{{ row.peer?.userId }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="等级" width="100">
                  <template #default="{ row }: { row: LinkedUserItem }"><el-tag :type="levelTagType(row.riskLevel)" size="small">{{ levelTagLabel(row.riskLevel) }}</el-tag></template>
                </el-table-column>
                <el-table-column prop="totalScore" label="总分" width="70" />
                <el-table-column label="最近命中" min-width="140">
                  <template #default="{ row }: { row: LinkedUserItem }">{{ formatTime(row.lastSeenAt) }}</template>
                </el-table-column>
                <el-table-column label="" width="130">
                  <template #default="{ row }: { row: LinkedUserItem }">
                    <el-button size="small" link type="primary" @click="openUserDetail(row.peer.userId)">详情</el-button>
                    <el-button size="small" link type="danger" @click="openBanDialog(row.peer.userId, row.peer.username, 'multi_account')">封禁</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>

            <el-card shadow="never">
              <template #header><span class="section-title">登录/设备时间线</span></template>
              <el-table :data="typedSignals" size="small" stripe max-height="340">
                <el-table-column prop="eventType" label="事件" width="100" />
                <el-table-column label="时间" min-width="160">
                  <template #default="{ row }: { row: SignalItem }">{{ formatTime(row.occurredAt) }}</template>
                </el-table-column>
                <el-table-column prop="ip" label="IP" min-width="130" />
                <el-table-column prop="localDeviceId" label="设备" min-width="160" show-overflow-tooltip />
              </el-table>
            </el-card>
          </div>

          <!-- 交易对手 + 交易记录 -->
          <div class="split-grid">
            <el-card shadow="never">
              <template #header><span class="section-title">交易对手</span></template>
              <el-table :data="typedTransferPeers" size="small" stripe max-height="340">
                <el-table-column label="对象" min-width="150">
                  <template #default="{ row }: { row: TransferPeerItem }">
                    <span class="user-label">{{ row.peer?.username }}</span>
                    <span class="uid-label">{{ row.peer?.userId }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="tradeCount" label="次" width="55" />
                <el-table-column prop="marketCount" label="市场" width="60" />
                <el-table-column prop="auctionCount" label="拍卖" width="60" />
                <el-table-column prop="outgoingNetAmount" label="流出净额" width="100" />
                <el-table-column prop="incomingNetAmount" label="流入净额" width="100" />
              </el-table>
            </el-card>

            <el-card shadow="never">
              <template #header><span class="section-title">交易记录</span></template>
              <el-table :data="typedTransfers" size="small" stripe max-height="340">
                <el-table-column label="对象" min-width="130">
                  <template #default="{ row }: { row: TransferItem }">
                    <span class="user-label">{{ row.peer?.username }}</span>
                    <span class="uid-label">{{ row.peer?.userId }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="direction" label="方向" width="60" />
                <el-table-column prop="channel" label="渠道" width="110" />
                <el-table-column label="物品" min-width="140">
                  <template #default="{ row }: { row: TransferItem }">
                    <span>{{ row.itemId }}</span>
                    <template v-if="row.channel?.startsWith('auction_') && row.extra?.listingKind === 'instance'">
                      <span v-if="(row.extra?.enhanceLevel as number) > 0" class="ml-1 text-xs text-blue-500">+{{ row.extra?.enhanceLevel }}</span>
                      <span v-if="(row.extra?.affixCount as number) > 0" class="ml-1 text-xs text-yellow-500">{{ row.extra?.affixCount }}词</span>
                    </template>
                  </template>
                </el-table-column>
                <el-table-column prop="quantity" label="数量" width="60" />
                <el-table-column prop="unitPrice" label="单价" width="80" />
                <el-table-column label="偏离倍数" width="90">
                  <template #default="{ row }: { row: TransferItem }">
                    <span :class="isAbnormalRatio(row.priceRatio) ? 'text-red-500 font-semibold' : ''">
                      {{ row.priceRatio != null ? row.priceRatio.toFixed(2) : "—" }}
                    </span>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </div>
        </div>
      </el-tab-pane>

      <!-- ── 关联组详情 ──────────────────────────────────────── -->
      <el-tab-pane label="关联组详情" name="cluster">
        <el-empty v-if="!clusterDetail" description="从概览或用户详情中打开一个关联组" />
        <div v-else class="stack">
          <el-card shadow="never">
            <div class="row-between">
              <div>
                <div class="text-lg font-semibold">关联组 {{ clusterDetail.clusterId }}</div>
                <div class="text-xs text-gray-400 mt-0.5">
                  <el-tag :type="levelTagType(clusterDetail.riskLevel)" size="small">{{ levelTagLabel(clusterDetail.riskLevel) }}</el-tag>
                  <el-tag :type="statusTagType(clusterDetail.status)" size="small" class="ml-2">{{ statusTagLabel(clusterDetail.status) }}</el-tag>
                </div>
              </div>
              <div class="row-gap">
                <el-button size="small" @click="applyClusterAction('recompute')">重算</el-button>
                <el-button size="small" @click="applyClusterAction('release')">解除标记</el-button>
                <el-button size="small" type="danger" @click="confirmBanCluster">整组封禁</el-button>
              </div>
            </div>
            <div class="stats-row mt-3">
              <div class="stat-card">
                <div class="stat-num">{{ clusterDetail.riskScore }}</div>
                <div class="stat-label">风险分数</div>
              </div>
              <div class="stat-card">
                <div class="stat-num">{{ clusterDetail.memberCount }}</div>
                <div class="stat-label">成员数</div>
              </div>
            </div>
          </el-card>

          <el-card shadow="never">
            <template #header>
              <div class="row-between">
                <span class="section-title">成员列表</span>
                <div class="row-gap">
                  <el-input v-model="operatorName" style="width:120px" placeholder="操作人" size="small" />
                  <el-input v-model="operatorNote" style="width:200px" placeholder="备注" size="small" />
                </div>
              </div>
            </template>
            <el-table :data="clusterDetail.members" stripe size="small" max-height="460">
              <el-table-column label="用户" min-width="160">
                <template #default="{ row }">
                  <span class="user-label">{{ row.user?.username }}</span>
                  <span class="uid-label">{{ row.user?.userId }}</span>
                  <el-tag v-if="row.isBanned" type="danger" size="small" class="ml-1">已封禁</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="分数" width="80">
                <template #default="{ row }">
                  <el-progress :percentage="Math.min(row.memberScore, 100)" :color="scoreColor(row.memberScore)" :show-text="false" style="width:50px;display:inline-block" />
                  <span class="ml-1 text-xs font-mono">{{ row.memberScore }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="reasonSummary" label="摘要" min-width="220" />
              <el-table-column label="" width="160">
                <template #default="{ row }">
                  <el-button size="small" link type="primary" @click="openUserDetail(row.user.userId)">查看</el-button>
                  <el-button size="small" link type="danger" @click="openBanDialog(row.user.userId, row.user.username, 'multi_account')">封禁</el-button>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </div>
      </el-tab-pane>

      <!-- ── 密码共用账号 ─────────────────────────────────────── -->
      <el-tab-pane label="密码共用账号" name="passwords">
        <el-card shadow="never">
          <template #header>
            <div class="row-between">
              <span class="section-title">密码共用账号组</span>
              <el-button size="small" @click="loadPasswordGroups">刷新</el-button>
            </div>
          </template>
          <el-empty v-if="!passwordGroups.length" description="暂无密码共用账号组" />
          <div v-else class="stack">
            <div v-for="group in passwordGroups" :key="group.hashDigestHint" class="pw-group-card">
              <div class="pw-group-header">
                <span class="pw-group-count">{{ group.users.length }} 个账号</span>
                <span class="pw-group-hint text-gray-400 text-xs font-mono ml-2">hash: {{ group.hashDigestHint }}…</span>
              </div>
              <div class="pw-group-members">
                <div v-for="member in group.users" :key="member.userId" class="pw-member">
                  <div class="row-gap">
                    <span class="user-label">{{ member.username }}</span>
                    <span class="uid-label">{{ member.userId }}</span>
                    <el-tag v-if="member.isBanned" type="danger" size="small">已封禁</el-tag>
                    <el-tag :type="levelTagType(member.riskLevel)" size="small">{{ levelTagLabel(member.riskLevel) }}</el-tag>
                    <span class="text-xs text-gray-400">{{ member.riskScore }}分</span>
                  </div>
                  <div class="row-gap">
                    <el-button size="small" link type="primary" @click="openUserDetail(member.userId)">详情</el-button>
                    <el-button size="small" link type="danger" @click="openBanDialog(member.userId, member.username, 'multi_account')">封禁</el-button>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="card-footer">
            <el-pagination layout="prev, pager, next, total" :total="passwordGroupsTotal" :page-size="pageSize" :current-page="passwordGroupsPage" @current-change="onPasswordGroupsPageChange" />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ── 拍卖行分析 ──────────────────────────────────────── -->
      <el-tab-pane label="拍卖行分析" name="auction">
        <el-card shadow="never">
          <template #header>
            <div class="row-between">
              <span class="section-title">拍卖行交易记录</span>
              <el-button size="small" @click="loadAuctionTransfers">刷新</el-button>
            </div>
          </template>
          <el-table :data="auctionTransfers" stripe size="small">
            <el-table-column label="时间" width="150">
              <template #default="{ row }: { row: AuctionTransferItem }">{{ formatTime(row.occurredAt) }}</template>
            </el-table-column>
            <el-table-column label="买家（from）" min-width="150">
              <template #default="{ row }: { row: AuctionTransferItem }">
                <span class="user-label">{{ row.fromUser.username }}</span>
                <span class="uid-label">{{ row.fromUser.userId }}</span>
                <el-button size="small" link type="primary" class="ml-1" @click="openUserDetail(row.fromUser.userId)">详情</el-button>
              </template>
            </el-table-column>
            <el-table-column label="卖家（to）" min-width="150">
              <template #default="{ row }: { row: AuctionTransferItem }">
                <span class="user-label">{{ row.toUser.username }}</span>
                <span class="uid-label">{{ row.toUser.userId }}</span>
                <el-button size="small" link type="primary" class="ml-1" @click="openUserDetail(row.toUser.userId)">详情</el-button>
              </template>
            </el-table-column>
            <el-table-column label="物品" min-width="160">
              <template #default="{ row }: { row: AuctionTransferItem }">
                <span>{{ getItemName(row.itemId) }}</span>
                <span class="uid-label ml-1">{{ row.itemId }}</span>
                <template v-if="row.extra?.listingKind === 'instance'">
                  <span v-if="(row.extra?.enhanceLevel as number) > 0" class="ml-1 text-xs text-blue-500 font-semibold">+{{ row.extra?.enhanceLevel }}</span>
                  <span v-if="(row.extra?.affixCount as number) > 0" class="ml-1 text-xs text-yellow-500">{{ row.extra?.affixCount }}词条</span>
                </template>
              </template>
            </el-table-column>
            <el-table-column prop="quantity" label="数量" width="55" />
            <el-table-column label="总价" width="90">
              <template #default="{ row }: { row: AuctionTransferItem }">
                <span class="font-mono">{{ row.totalPrice.toLocaleString() }}</span>
              </template>
            </el-table-column>
            <el-table-column label="偏离倍数" width="90">
              <template #default="{ row }: { row: AuctionTransferItem }">
                <span :class="isAbnormalRatio(row.priceRatio) ? 'text-red-500 font-semibold' : ''">
                  {{ row.priceAnchor > 0 ? row.priceRatio.toFixed(2) : "—" }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="channel" label="渠道" width="110" />
          </el-table>
          <div class="card-footer">
            <el-pagination layout="prev, pager, next, total" :total="auctionTransfersTotal" :page-size="pageSize" :current-page="auctionTransfersPage" @current-change="onAuctionTransfersPageChange" />
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ── 复核案例 ─────────────────────────────────────────── -->
      <el-tab-pane label="复核案例" name="cases">
        <el-card shadow="never">
          <template #header>
            <div class="row-between">
              <span class="section-title">复核案例</span>
              <el-button size="small" @click="loadReviewCases">刷新</el-button>
            </div>
          </template>
          <el-table :data="allReviewCases" stripe size="small">
            <el-table-column prop="id" label="Case" width="80" />
            <el-table-column prop="clusterId" label="Cluster" width="90" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.status === 'open' ? 'danger' : 'info'" size="small">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="decision" label="结论" width="130" />
            <el-table-column prop="operatorName" label="操作人" width="110" />
            <el-table-column label="更新时间" min-width="160">
              <template #default="{ row }">{{ formatTime(row.updatedAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="200">
              <template #default="{ row }">
                <div class="row-gap">
                  <el-button size="small" :disabled="!row.clusterId" @click="openClusterDetail(row.clusterId)">组详情</el-button>
                  <el-button size="small" @click="closeCase(row.id)">关闭</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 封禁弹窗 -->
    <el-dialog v-model="banDialogVisible" title="封禁玩家" width="440px">
      <div class="stack">
        <el-input :model-value="banForm.username ? `${banForm.username} · ${banForm.userId}` : String(banForm.userId || '')" disabled />
        <el-select v-model="banForm.reason" placeholder="封禁理由" style="width:100%">
          <el-option v-for="r in banReasons" :key="r.key" :label="r.key" :value="r.key" />
        </el-select>
        <el-switch v-model="banForm.isPermanent" active-text="永久封禁" inactive-text="临时封禁" />
        <el-input v-model="banForm.banUntil" :disabled="banForm.isPermanent" placeholder="banUntil (RFC3339)" />
      </div>
      <template #footer>
        <el-button @click="banDialogVisible = false">取消</el-button>
        <el-button type="danger" @click="submitBan">确认封禁</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import type { AdminBanReason, ClusterSummary, HighRiskUserItem, PasswordGroup, RiskClusterDetail, RiskSearchItem, RiskUserDetail, ReviewCase } from "@shared/types"
import { banUser, fetchBanReasons } from "@/services/admin-users"
import {
  applyRiskClusterAction as postRiskClusterAction,
  fetchAuctionTransfers,
  fetchHighRiskUsers,
  fetchPasswordGroups,
  fetchRiskClusterDetail,
  fetchRiskClusters,
  fetchRiskReviewCases,
  fetchRiskUserDetail,
  resetAllRiskData,
  resolveRiskReviewCase,
  searchRiskAccounts
} from "@/services/admin-risk"
import { getItemName } from "@/utils/item-names"
import type { AuctionTransferItem } from "@shared/types"

type EvidenceItem = { peer: { userId: number; username: string }; evidenceType: string; strength: string; score: number; payload: Record<string, unknown> }
type LinkedUserItem = { peer: { userId: number; username: string }; riskLevel: string; totalScore: number; lastSeenAt: string }
type SignalItem = { eventType: string; occurredAt: string; ip: string; ipPrefix: string; localDeviceId: string; fingerprintId: string }
type TransferPeerItem = { peer: { userId: number; username: string }; tradeCount: number; marketCount: number; auctionCount: number; outgoingNetAmount: number; incomingNetAmount: number }
type TransferItem = { peer: { userId: number; username: string }; direction: string; channel: string; itemId: string; quantity: number; unitPrice: number; priceRatio: number | null; extra?: Record<string, unknown> }

const activeTab = ref("overview")
const query = ref("")
const searching = ref(false)
const operatorName = ref("admin")
const operatorNote = ref("")
const pageSize = 20

const searchItems = ref<RiskSearchItem[]>([])
const highRiskUsers = ref<HighRiskUserItem[]>([])
const highRiskUsersTotal = ref(0)
const highRiskUsersPage = ref(1)
const clusters = ref<ClusterSummary[]>([])
const clustersTotal = ref(0)
const clustersPage = ref(1)
const userDetail = ref<RiskUserDetail | null>(null)
const clusterDetail = ref<RiskClusterDetail | null>(null)
const allReviewCases = ref<ReviewCase[]>([])
const passwordGroups = ref<PasswordGroup[]>([])
const passwordGroupsTotal = ref(0)
const passwordGroupsPage = ref(1)
const banReasons = ref<AdminBanReason[]>([])
const banDialogVisible = ref(false)
const banForm = reactive({ userId: 0, username: "", reason: "", isPermanent: true, banUntil: "" })

const auctionTransfers = ref<AuctionTransferItem[]>([])
const auctionTransfersTotal = ref(0)
const auctionTransfersPage = ref(1)

const openCaseCount = computed(() => allReviewCases.value.filter((c) => c.status === "open").length)

const typedEvidences = computed(() => (userDetail.value?.evidences ?? []) as unknown as EvidenceItem[])
const typedLinkedUsers = computed(() => (userDetail.value?.linkedUsers ?? []) as unknown as LinkedUserItem[])
const typedSignals = computed(() => (userDetail.value?.signals ?? []) as unknown as SignalItem[])
const typedTransferPeers = computed(() => (userDetail.value?.transferPeers ?? []) as unknown as TransferPeerItem[])
const typedTransfers = computed(() => (userDetail.value?.transfers ?? []) as unknown as TransferItem[])

// Multi-account confidence index: sum of device+password+behavior evidence scores
const multiAccountScore = computed(() => {
  const multiTypes = new Set([
    "password_match", "local_device", "fingerprint_ip", "session_chain_device",
    "cookie", "never_concurrent", "session_handoff", "login_pattern"
  ])
  return typedEvidences.value
    .filter((ev) => multiTypes.has(ev.evidenceType))
    .reduce((sum, ev) => sum + (ev.score ?? 0), 0)
})

const EVIDENCE_GROUPS: { label: string; types: string[] }[] = [
  { label: "设备指纹", types: ["local_device", "fingerprint_ip", "session_chain_device", "cookie", "device_family", "browser_family"] },
  { label: "行为特征", types: ["never_concurrent", "session_handoff", "login_pattern"] },
  { label: "密码 / 账号", types: ["password_match"] },
  { label: "网络特征", types: ["ip_prefix", "locale_family"] },
  { label: "交易行为", types: ["market_trade", "auction_trade", "repeated_trade", "trade_loop", "one_way_transfer", "abnormal_price"] }
]

const evidenceGroups = computed(() => {
  return EVIDENCE_GROUPS
    .map((g) => ({
      label: g.label,
      items: typedEvidences.value.filter((ev) => g.types.includes(ev.evidenceType))
    }))
    .filter((g) => g.items.length > 0)
})

const EVIDENCE_LABELS: Record<string, string> = {
  password_match: "密码匹配",
  local_device: "本地设备",
  fingerprint_ip: "指纹+IP",
  session_chain_device: "会话链+设备",
  cookie: "Cookie",
  device_family: "设备族",
  browser_family: "浏览器族",
  never_concurrent: "从不同时在线",
  session_handoff: "接力登录",
  login_pattern: "登录时段相似",
  ip_prefix: "同网段",
  locale_family: "语言/时区",
  market_trade: "市场交易",
  auction_trade: "拍卖行交易",
  repeated_trade: "重复交易",
  trade_loop: "往复交易",
  one_way_transfer: "单向转移",
  abnormal_price: "异常价格"
}

function evidenceLabel(type: string) {
  return EVIDENCE_LABELS[type] ?? type
}

function strengthType(strength: string) {
  if (strength === "strong") return "danger"
  if (strength === "medium") return "warning"
  return "info"
}

function scoreColor(score: number) {
  if (score >= 80) return "#f56c6c"
  if (score >= 50) return "#e6a23c"
  if (score >= 25) return "#409eff"
  return "#909399"
}

function isAbnormalRatio(ratio: number | null | undefined) {
  if (ratio == null) return false
  return ratio < 0.4 || ratio > 2.5
}

function formatTime(val: unknown) {
  if (!val) return "—"
  const d = new Date(val as string)
  if (isNaN(d.getTime())) return String(val)
  return d.toLocaleString("zh-CN", { hour12: false })
}

onMounted(async () => {
  try {
    const reasonsPayload = await fetchBanReasons()
    banReasons.value = reasonsPayload.reasons || []
    banForm.reason = banReasons.value[0]?.key || ""
    await Promise.all([loadHighRiskUsers(), loadClusters(), loadReviewCases(), loadPasswordGroups(), loadAuctionTransfers()])
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "加载风险数据失败")
  }
})

async function loadHighRiskUsers() {
  const r = await fetchHighRiskUsers(pageSize, (highRiskUsersPage.value - 1) * pageSize)
  highRiskUsers.value = r.items
  highRiskUsersTotal.value = r.total
}

async function loadClusters() {
  const r = await fetchRiskClusters(pageSize, (clustersPage.value - 1) * pageSize)
  clusters.value = r.items
  clustersTotal.value = r.total
}

async function loadReviewCases() {
  const r = await fetchRiskReviewCases()
  allReviewCases.value = r.items || []
}

async function loadPasswordGroups() {
  const r = await fetchPasswordGroups(pageSize, (passwordGroupsPage.value - 1) * pageSize)
  passwordGroups.value = r.items
  passwordGroupsTotal.value = r.total
}

async function runSearch() {
  if (!query.value.trim()) return
  searching.value = true
  try {
    const r = await searchRiskAccounts(query.value.trim())
    searchItems.value = r.items || []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "搜索失败")
  } finally {
    searching.value = false
  }
}

function onHighRiskUsersPageChange(page: number) { highRiskUsersPage.value = page; void loadHighRiskUsers() }
function onClustersPageChange(page: number) { clustersPage.value = page; void loadClusters() }
function onPasswordGroupsPageChange(page: number) { passwordGroupsPage.value = page; void loadPasswordGroups() }
function onAuctionTransfersPageChange(page: number) { auctionTransfersPage.value = page; void loadAuctionTransfers() }

async function loadAuctionTransfers() {
  const r = await fetchAuctionTransfers(pageSize, (auctionTransfersPage.value - 1) * pageSize)
  auctionTransfers.value = r.items
  auctionTransfersTotal.value = r.total
}

async function confirmResetAll() {
  await ElMessageBox.confirm(
    "此操作将清空全部风险数据（信号、转账记录、证据、关联组、密码记录等），不可恢复。确认执行？",
    "全量重置风险数据",
    { type: "warning", confirmButtonText: "确认重置", cancelButtonText: "取消", confirmButtonClass: "el-button--danger" }
  )
  await resetAllRiskData()
  ElMessage.success("风险数据已全量重置")
  await Promise.all([loadHighRiskUsers(), loadClusters(), loadReviewCases(), loadPasswordGroups()])
}

async function openUserDetail(userId: number) {
  userDetail.value = await fetchRiskUserDetail(userId)
  activeTab.value = "user"
}

async function openClusterDetail(clusterId: number) {
  clusterDetail.value = await fetchRiskClusterDetail(clusterId)
  activeTab.value = "cluster"
}

async function applyClusterAction(action: "recompute" | "release" | "ban") {
  if (!clusterDetail.value) return
  await postRiskClusterAction(clusterDetail.value.clusterId, action, operatorName.value.trim(), operatorNote.value.trim())
  ElMessage.success(`已执行 ${action}`)
  await Promise.all([loadHighRiskUsers(), loadClusters(), loadReviewCases()])
  clusterDetail.value = await fetchRiskClusterDetail(clusterDetail.value.clusterId)
}

async function confirmBanCluster() {
  if (!clusterDetail.value) return
  await ElMessageBox.confirm(`确认封禁关联组 ${clusterDetail.value.clusterId} 的全部成员？`, "整组封禁", {
    type: "warning", confirmButtonText: "确认封禁", cancelButtonText: "取消"
  })
  await applyClusterAction("ban")
}

async function closeCase(caseId: number) {
  await resolveRiskReviewCase(caseId, operatorName.value.trim(), operatorNote.value.trim(), "closed")
  ElMessage.success("复核单已关闭")
  await loadReviewCases()
}

function openBanDialog(userId: number, username: string, reason?: string) {
  banForm.userId = userId
  banForm.username = username
  banForm.reason = reason ?? banReasons.value[0]?.key ?? ""
  banForm.isPermanent = true
  banForm.banUntil = ""
  banDialogVisible.value = true
}

async function submitBan() {
  await banUser({ userId: banForm.userId, reason: banForm.reason, isPermanent: banForm.isPermanent, banUntil: banForm.isPermanent ? undefined : banForm.banUntil.trim() })
  ElMessage.success("封禁成功")
  banDialogVisible.value = false
}

const LEVEL_MAP: Record<string, { type: "danger" | "warning" | "primary" | "info" | "success"; label: string }> = {
  sealed:     { type: "danger",  label: "封印" },
  banned:     { type: "danger",  label: "封禁" },
  restricted: { type: "warning", label: "受限" },
  guarded:    { type: "primary", label: "监控" },
  observe:    { type: "info",    label: "观察" }
}
const STATUS_MAP: Record<string, { type: "danger" | "warning" | "success" | "info"; label: string }> = {
  manual_review: { type: "warning", label: "复核中" },
  banned:        { type: "danger",  label: "已封禁" },
  released:      { type: "success", label: "已解除" },
  active:        { type: "info",    label: "活跃" }
}

function levelTagType(level: string | undefined): "danger" | "warning" | "primary" | "info" | "success" {
  return (LEVEL_MAP[level ?? "observe"] ?? LEVEL_MAP.observe).type
}
function levelTagLabel(level: string | undefined) {
  return (LEVEL_MAP[level ?? "observe"] ?? { label: level ?? "—" }).label
}
function statusTagType(status: string | undefined): "danger" | "warning" | "success" | "info" {
  return (STATUS_MAP[status ?? "active"] ?? STATUS_MAP.active).type
}
function statusTagLabel(status: string | undefined) {
  return (STATUS_MAP[status ?? "active"] ?? { label: status ?? "—" }).label
}
</script>


<style scoped>
.risk-page { height: 100%; display: flex; flex-direction: column; }
.risk-tabs { flex: 1; display: flex; flex-direction: column; }
.risk-tabs :deep(.el-tabs__content) { flex: 1; overflow-y: auto; padding: 16px; }

.stack { display: flex; flex-direction: column; gap: 12px; }
.row-between { display: flex; align-items: center; justify-content: space-between; }
.row-gap { display: flex; align-items: center; gap: 8px; }
.section-title { font-weight: 600; font-size: 14px; }
.mt-3 { margin-top: 12px; }
.mt-1 { margin-top: 4px; }
.ml-1 { margin-left: 4px; }
.ml-2 { margin-left: 8px; }

.stats-row { display: flex; gap: 12px; flex-wrap: wrap; }
.stat-card { background: var(--el-fill-color-light); border-radius: 8px; padding: 12px 20px; min-width: 110px; text-align: center; }
.stat-num { font-size: 22px; font-weight: 700; line-height: 1.2; }
.stat-label { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }

.split-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }

.card-footer { display: flex; justify-content: flex-end; padding-top: 12px; }

.user-label { font-weight: 500; }
.uid-label { color: var(--el-text-color-secondary); font-size: 12px; margin-left: 4px; }

/* Evidence groups */
.evidence-groups { display: flex; flex-direction: column; gap: 12px; }
.evidence-group-label { font-size: 12px; font-weight: 600; color: var(--el-text-color-secondary); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px; }
.evidence-item { background: var(--el-fill-color-light); border-radius: 6px; padding: 8px 12px; margin-bottom: 6px; }
.ev-type-label { font-weight: 600; font-size: 13px; }
.ev-peer { font-size: 12px; }
.ev-payload { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 4px; }
.ev-kv { font-size: 11px; }
.ev-key { color: var(--el-text-color-secondary); margin-right: 2px; }
.ev-val { font-family: monospace; }

/* Password groups */
.pw-group-card { background: var(--el-fill-color-light); border-radius: 8px; padding: 12px 16px; }
.pw-group-header { display: flex; align-items: center; margin-bottom: 8px; }
.pw-group-count { font-weight: 600; font-size: 13px; }
.pw-group-members { display: flex; flex-direction: column; gap: 6px; }
.pw-member { display: flex; align-items: center; justify-content: space-between; background: var(--el-bg-color); border-radius: 6px; padding: 6px 10px; }

.text-red-500 { color: #f56c6c; }
.text-blue-500 { color: #409eff; }
.text-yellow-500 { color: #e6a23c; }
.font-semibold { font-weight: 600; }
.font-mono { font-family: monospace; }
.text-xs { font-size: 12px; }
.text-sm { font-size: 13px; }
.text-lg { font-size: 18px; }
.text-gray-400 { color: var(--el-text-color-secondary); }
</style>
