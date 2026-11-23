package runtime

type HookTrigger string

const (
	HookTriggerBattleStart  HookTrigger = "battle_start"
	HookTriggerTurnStart    HookTrigger = "turn_start"
	HookTriggerBeforeDamage HookTrigger = "before_damage"
	HookTriggerAfterDamage  HookTrigger = "after_damage"
	HookTriggerOnKill       HookTrigger = "on_kill"
	HookTriggerOnDeath      HookTrigger = "on_death"
	HookTriggerPeriodic     HookTrigger = "periodic"
)

type HookTarget struct {
	Side     string `json:"side"`
	Selector string `json:"selector"`
	Count    int    `json:"count,omitempty"`
	Position string `json:"position,omitempty"`
}

type HookEffect struct {
	Kind       string `json:"kind"`
	Formula    string `json:"formula,omitempty"`
	DamageType string `json:"damageType,omitempty"`
	BuffID     string `json:"buffId,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Stacks     int    `json:"stacks,omitempty"`
	Count      int    `json:"count,omitempty"`
	HP         string `json:"hp,omitempty"`
	MP         string `json:"mp,omitempty"`
}

type HookDefinition struct {
	ID                   string       `json:"id"`
	Trigger              HookTrigger  `json:"trigger"`
	Chance               float64      `json:"chance,omitempty"`
	Priority             int64        `json:"priority,omitempty"`
	ActionDurationMS     *int64       `json:"actionDurationMs,omitempty"`
	MaxTriggersPerBattle int64        `json:"maxTriggersPerBattle,omitempty"`
	CooldownMS           int64        `json:"cooldownMs,omitempty"`
	Target               HookTarget   `json:"target"`
	Effects              []HookEffect `json:"effects"`
}

type TimelineEvent struct {
	Seq              int64          `json:"seq"`
	OffsetMS         int64          `json:"offsetMs"`
	Lane             string         `json:"lane"`
	Phase            string         `json:"phase"`
	ParentSeq        *int64         `json:"parentSeq,omitempty"`
	GroupID          string         `json:"groupId,omitempty"`
	OrderKey         string         `json:"orderKey"`
	ActorID          string         `json:"actorId,omitempty"`
	ActorUserID      int64          `json:"actorUserId,omitempty"`
	HookID           string         `json:"hookId,omitempty"`
	ActionDurationMS int64          `json:"actionDurationMs"`
	Payload          map[string]any `json:"payload"`
}

type CompiledArtifact struct {
	RunID           int64           `json:"runId"`
	RoomID          int64           `json:"roomId"`
	Seed            string          `json:"seed"`
	RulesVersion    string          `json:"rulesVersion"`
	SnapshotHash    string          `json:"snapshotHash"`
	TotalDurationMS int64           `json:"totalDurationMs"`
	Events          []TimelineEvent `json:"events"`
	Result          map[string]any  `json:"result"`
}

type PlayCheckpoint struct {
	RunID        int64 `json:"runId"`
	LastSeq      int64 `json:"lastSeq"`
	LastOffsetMS int64 `json:"lastOffsetMs"`
	UpdatedAtMS  int64 `json:"updatedAtMs"`
}

type EventBatchPayload struct {
	RunID         int64           `json:"runId"`
	FromSeq       int64           `json:"fromSeq"`
	ToSeq         int64           `json:"toSeq"`
	WindowStartMS int64           `json:"windowStartMs"`
	WindowEndMS   int64           `json:"windowEndMs"`
	Events        []TimelineEvent `json:"events"`
}
