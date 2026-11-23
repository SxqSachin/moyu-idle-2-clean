# 用户封禁处理说明

## TL;DR
- 封禁入口：`POST /api/admin/users/ban`，鉴权头：`X-Release-Token`。
- 封禁生效方式：先踢 WS 在线连接，再旋转 `session_version`，登录阶段拒绝（`AUTH_ACCOUNT_BANNED`）。
- 封禁期间按“用户不存在”处理：清理市场/拍卖挂单、停动作队列、阻断队列推进、踢出战斗房间、阻断猫舍管理触发写入。

## 何时阅读
- 需要接入后台封号/解封能力。
- 需要排查封号后为何队列、战斗、市场/拍卖状态发生变化。
- 需要对接前端封号提示或后台运营工具。

## 封禁相关接口

### 1) 查询封号理由枚举
- `GET /api/admin/users/ban-reasons`
- 鉴权：`X-Release-Token`
- 说明：返回 `global-config.admin.userBanReasons`，前端只读展示，不在前端硬编码理由。

返回字段（`data`）：
- `reasons[]`
  - `key`: 理由键（如 `abnormal_trade`）
  - `nameKey`: i18n key（如 `admin.banReason.abnormalTrade`）

### 2) 封号
- `POST /api/admin/users/ban`
- 鉴权：`X-Release-Token`

请求参数（JSON）：
- `userId` `number`：目标用户 ID（与 `username` 二选一，可同时传并做一致性校验）
- `username` `string`：目标用户名
- `reason` `string`：封号理由 key，必须命中 `global-config.admin.userBanReasons`
- `isPermanent` `boolean`：是否永久封号
- `banUntil` `string`：非永久封号必填，RFC3339，且必须是未来时间

返回字段（`data`）：
- `ok` `boolean`
- `userId` `number`
- `reason` `string`
- `isPermanent` `boolean`
- `banUntil` `string`（非永久时返回）

### 3) 解封
- `POST /api/admin/users/unban`
- 鉴权：`X-Release-Token`

请求参数（JSON）：
- `userId` `number` 或 `username` `string`

返回字段（`data`）：
- `ok` `boolean`
- `userId` `number`

## 登录与在线会话语义
- 登录链路会读取用户封禁状态：
  - 命中封禁时返回 `AUTH_ACCOUNT_BANNED`。
  - `error.detail` 形如：`reason` 或 `reason|banUntil`（非永久时带到期时间）。
- 封禁时会向目标用户下发 `auth.kicked`（reason: `ACCOUNT_BANNED`）。
- 封禁时会递增 `player_presence.session_version`，旧 token 立即失效（HTTP/WS 都会被拦截）。

## 封禁期间的业务副作用（集中在封号事务内）

### 1) 市场 / 拍卖
- 取消该用户所有 `market_listings` 的 `active` 挂单并返还物品到库存。
- 取消该用户所有 `market_buy_orders` 的 `active` 买单，并按 `remaining_quantity * price` 退款金币。
- 强制取消该用户所有 `market_auctions` 的 `active` 拍卖：
  - **不收取消拍卖手续费**。
  - 若有当前出价者，先退还其押金（`current_price`）。
  - 再把卖家上架中的物品返还回库存。

> 当前不处理“该封禁用户是他人拍卖当前最高出价者”的额外清理（按现策略暂不纳入）。

### 2) 动作队列（玩家 + 猫咪助手）
- 立即删除 `player_action_queue` 中该用户 `pending/running` 队列项。
- 队列推进入口（在线结算、离线快进、worker 推进）会做封禁校验，封禁用户不再推进。

### 3) 猫舍随记 / 随遇（管理触发）
- 后台 `admin/cattery/trigger` 会拒绝封禁用户（含目标用户）参与写入，返回 `ADMIN_CATTERY_TRIGGER_USER_BANNED`。
- 随遇随机目标筛选会排除封禁用户。

### 4) 战斗房间
- 封禁时将该用户从 `battle_room_members` 移除。
- 若房间空了：删除房间。
- 若封禁用户是房主：转移房主给最早加入的剩余成员。
- 若房间当时在 `entering/playing/recovering`：
  - 中断当前 run（标记 `interrupted`）。
  - 房间回到 `preparing`，清空 `current_run_id`，等待剩余成员重新准备。

## 错误码（核心）
- 登录：`AUTH_ACCOUNT_BANNED`
- 封号接口：
  - `ADMIN_USER_BAN_INVALID_REASON`
  - `ADMIN_USER_BAN_INVALID_BAN_UNTIL`
  - `ADMIN_USER_BAN_TARGET_NOT_FOUND`
- 解封接口：
  - `ADMIN_USER_UNBAN_TARGET_NOT_FOUND`
- 猫舍管理触发：
  - `ADMIN_CATTERY_TRIGGER_USER_BANNED`
