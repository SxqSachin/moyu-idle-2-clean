# 队列/战斗调度设计

## 目标
- 支持不同频率事件（如 3.2s / 1.5s）并保持同玩家串行、跨玩家并行。
- 调度内核可切换：`minheap` 与 `timewheel`。

## 结构
- `Scheduler` 按 `player_id % shards` 分片。
- 每个分片单 goroutine 消费，避免同玩家并发写状态。
- 每个分片内部队列策略可配置：
  - `minheap`：适合触发时间离散、动态插入/取消多的场景。
  - `timewheel`：适合海量定时任务、固定 tick 的高吞吐场景。

## 配置
- `SCHEDULER_ENGINE=minheap|timewheel`
- `SCHEDULER_SHARDS=8`
- `SCHEDULER_TICK_MS=100`
- `SCHEDULER_WHEEL_SIZE=256`

## 扩展点
- 事件幂等：`event_id` 去重表（后续）。
- 失败重试：按事件类型配置重试与死信（后续）。
- 指标：分片队列深度、事件延迟、执行失败率（后续）。

