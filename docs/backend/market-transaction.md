# 市场事务一致性设计

## 关键约束
- 上架时扣减卖家库存，防止“无库存上架”。
- 求购时冻结买家金币（`price * quantity`），撤销求购单时按剩余数量原路退回。
- 实例装备上架时冻结实例状态为 `market`，避免重复穿戴或重复出售。
- 市场与拍卖都必须满足 `items.json.marketListable=true` 才允许上架或发起求购。
- 市场与拍卖的全部接口（含只读查询）都要求用户已完成邮箱验证（`users.email_verified=true`）。
- 装备实例进入玩家市场需满足“模板一致”基线：`enhanceLevel=0` 且 affix 与 `equipment_templates.defaultAffixSlots` 在 `slotIndex + affixId` 维度全量一致；不满足时必须走拍卖链路。
- 购买时必须使用事务执行：扣买家金币、按费率分账（卖家入账 + 手续费回收）、发放道具、扣减挂单、写订单、写台账。
- 卖家响应求购单时必须使用事务执行：扣卖家道具、买家收货、卖家收款、扣减求购剩余数量、状态推进（`active -> filled`）。
- 卖家可主动下架自己的上架单；下架后剩余未售数量（或实例）必须原路返还库存。
- 使用 `X-Idempotency-Key` 保证客户端重试不重复扣款。
- 手续费采用 `market_fee_tiers` 阶梯配置（默认基准 5%，默认去向为系统回收）。
- 同一玩家同一物品在 `active` 状态下只允许一个市场方向：**要么上架单，要么求购单**，不可同时存在。
- 同一玩家同一物品若已有同方向 `active` 订单：**不同价格拒绝**；**同价格自动合并到已有订单数量**（求购单合并 `quantity + remaining_quantity`）。

## 并发控制
- 对 `market_listings` 行 `FOR UPDATE` 锁，防超卖。
- 对买家金币行 `FOR UPDATE` 锁，防并发透支。
- `market_orders(buyer_id, idempotency_key)` 唯一索引，防重复订单。
- 对同玩家同物品的 `market_listings / market_buy_orders` 活跃行加锁检查，防并发下创建互斥订单或价格分叉订单。

## API
- `POST /api/market/list`
- `POST /api/market/list-instance`
- `POST /api/market/buy`（必须带 `Authorization` 与 `X-Idempotency-Key`）
  - 响应附带 `feeRateBps`、`feeAmount`、`sellerAmount`
- `GET /api/market/listings`
  - 查询参数：
    - `scope=public|mine`（默认 `public`）
    - `itemId`（仅 `public` 生效）
    - `includeSold=true|false`
    - `limit`（默认 30，最大 100）
  - 用途：前端玩家市场页拉取“在售列表 + 我的上架记录”。
  - 排序规则：`active` 状态优先；在售单按价格升序（同价按 id 升序）。
- `GET /api/market/buy-orders`
  - 查询参数：
    - `scope=public|mine`（默认 `public`）
    - `itemId`（仅 `public` 生效）
    - `includeFilled=true|false`
    - `limit`（默认 30，最大 100）
  - 排序规则：`active` 状态优先；在售求购单按价格降序（同价按 id 升序）。
- `POST /api/market/buy-orders`（创建求购单，冻结金币）
- `POST /api/market/buy-orders/fill`（卖家向求购单卖出）
- `POST /api/market/buy-orders/cancel`（撤销求购单并退回剩余冻结金币）
- `POST /api/market/listings/cancel`（卖家下架自己的上架单并退回剩余库存）

## 拍卖 API（实例装备）
- `GET /api/auction/listings`
  - 查询参数：
    - `scope=public|mine`（默认 `public`）
    - `itemId`（仅 `public` 生效）
    - `includeEnded=true|false`
    - `limit`（默认 30，最大 100）
- `POST /api/auction/list-instance`（发起拍卖，实例状态切换为 `auction`）
- `GET /api/auction/detail?auctionId=...`（拍卖详情，含卖家、数量、最小出价、手续费字段）
- `POST /api/auction/bid`（出价，自动退回上一位竞价者冻结金币）
- `POST /api/auction/cancel`（仅无竞价时允许取消并返还实例）

> 拍卖到期采用惰性结算：在拍卖读/写路径执行到期结算。  
> 结算结果：有最高出价则实例转给最高出价者并给卖家结算金币；无出价则实例返还卖家并标记 `expired`。
> 实例拍卖在 `active` 阶段保持 `instance_id` 强引用保护；进入 `sold/cancelled/expired` 终态时会在同一事务内将 `instance_id` 置空，避免历史记录反向阻塞库存实例生命周期。

> 实例拍卖会在上架事务内固化公开快照（强化、词条、耐久、基础快照），`/api/auction/listings` 与 `/api/auction/detail` 直接返回该快照供所有玩家查看；不通过放宽 `/api/items/detail` 所有权校验来取数，避免越权读取。

### 拍卖规则补充（2026-05）
- 拍卖支持两种上架模式：
  - `instance`：单件实例（含不可堆叠、带 affix 装备）
  - `stack`：可堆叠道具按数量拍卖
- 创建拍卖时长前端按“小时”输入（整数）；提交到后端仍使用 `durationSeconds`。
- 仅允许 `marketListable=true` 物品参与拍卖。
- 同一时间每个玩家只能拥有一场 `active` 拍卖；取消或到期后才可发起下一场。
- 拍卖时长最小 4 小时（`14400` 秒），最大 72 小时（`259200` 秒）；输入超界会被钳制到合法区间。
- 起拍价要求：`startPrice > 真实价格 * 数量`；若是带 affix 的实例，再额外乘 `5`。
- 一口价要求：`buyoutPrice > startPrice * 1.5`（严格大于）。
- 竞拍资金流程：新出价先冻结金币；被超价后自动原路返还上一名最高出价者。
- 竞价最小增幅：创建拍卖时由卖家输入 `minBidIncrement`，并且必须在 `[ceil(startPrice*10%), startPrice]` 区间内；最新出价必须至少比当前最高价高出该值。
- 结算手续费：成交后按 `shared-global-config.json.auction.feeRateBps` 扣除，卖家到账净额写入拍卖记录。
- 取消手续费：已有出价时允许卖家取消，但需按当前最高价与 `feeRateBps` 扣除手续费；手续费不足则取消失败。
- 反狙击：结束前 `auction.antiSnipeTriggerSeconds` 内有新出价时，自动延长 `auction.antiSnipeExtendSeconds`，可重复叠加。
