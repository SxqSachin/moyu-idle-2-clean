package skills

type SkillRuntime struct {
	SkillID             string
	SkillLevel          int
	SkillExp            int64
	DamageType          string
	DamageCoeff         float64
	DamageFormula       []SkillDamageTerm
	HPCost              float64
	MPCost              float64
	TargetDirection     string
	TargetSeekStrategy  string
	TargetMode          string
	TargetCount         int
	TargetPositions     []string
	SecondarySeek       string
	TargetScope         string
	SelectRule          string
	SelectAttrKey       string
	FallbackSelfIfEmpty bool
	IgnoreUntargetable  bool
	CooldownTurns       int
	CooldownRemain      int
	AttackReasonHint    string
}

type SkillDamageTerm struct {
	AttrKey     string
	Coefficient float64
}

type BasicAttackProfile struct {
	TargetDirection    string
	TargetSeekStrategy string
	DamageType         string
	Formula            []BasicAttackFormulaTerm
	DamageSpec         map[string]any
}

type BasicAttackFormulaTerm struct {
	AttrKey     string
	Coefficient float64
}

type BuffRuntime struct {
	BuffID      string
	RemainTurns int64
	Payload     map[string]any
}

type UnitRuntime struct {
	UnitID             string
	Side               string
	UserID             int64
	EnemyID            string
	EnemyType          string
	EnemyLevel         int
	Position           string
	Alive              bool
	HP                 float64
	MP                 float64
	MaxHP              float64
	MaxMP              float64
	Stats              map[string]float64
	Buffs              []BuffRuntime
	BasicAttackProfile *BasicAttackProfile
}

type BattleRuntime struct {
	Wave             int
	CompletedBattles int
	TurnCursor       int
	Seq              int64
	RNGIndex         int64
	StartedAtMS      int64
	TimestampMS      int64
	Players          []UnitRuntime
	Enemies          []UnitRuntime
}

type SummonRuntime struct {
	UnitID               string             `json:"unitId"`
	Side                 string             `json:"side"`
	Name                 string             `json:"name"`
	NameKey              string             `json:"nameKey,omitempty"`
	Position             string             `json:"position"`
	HP                   float64            `json:"hp"`
	MP                   float64            `json:"mp"`
	MaxHP                float64            `json:"maxHp"`
	MaxMP                float64            `json:"maxMp"`
	DurationTurns        int64              `json:"durationTurns,omitempty"`
	TemplateUnitID       string             `json:"templateUnitId,omitempty"`
	SourceUnitID         string             `json:"sourceUnitId,omitempty"`
	MaxCountPerUnit      int                `json:"maxCountPerUnit,omitempty"`
	PersistAcrossBattles bool               `json:"persistAcrossBattles,omitempty"`
	OnCasterDeath        string             `json:"onCasterDeath,omitempty"`
	OnCapOverflow        string             `json:"onCapOverflow,omitempty"`
	OnDispel             string             `json:"onDispel,omitempty"`
	OnDurationEnd        string             `json:"onDurationEnd,omitempty"`
	SourceEnhanceLevel   int                `json:"sourceEnhanceLevel,omitempty"`
	Stats                map[string]float64 `json:"stats"`
	SkillIDs             []string           `json:"skillIds,omitempty"`
	InitialBuffIDs       []string           `json:"initialBuffIds,omitempty"`
	AllowNoSkills        bool               `json:"allowNoSkills,omitempty"`
}

type SkillEffect struct {
	Kind              string         `json:"kind"`
	TargetID          string         `json:"targetId,omitempty"`
	SourceID          string         `json:"sourceId,omitempty"`
	Amount            float64        `json:"amount,omitempty"`
	Crit              bool           `json:"crit,omitempty"`
	DamageType        string         `json:"damageType,omitempty"`
	Resource          string         `json:"resource,omitempty"`
	BuffID            string         `json:"buffId,omitempty"`
	BuffDurationTurns int64          `json:"buffDurationTurns,omitempty"`
	BuffPayload       map[string]any `json:"buffPayload,omitempty"`
	Summon            *SummonRuntime `json:"summon,omitempty"`
	Reason            string         `json:"reason,omitempty"`
	Meta              map[string]any `json:"meta,omitempty"`
	Applied           bool           `json:"applied,omitempty"`
}

type SkillResult struct {
	Effects []SkillEffect  `json:"effects"`
	Meta    map[string]any `json:"meta,omitempty"`
}

type TargetQuery struct {
	Side               string
	AliveOnly          bool
	IgnoreUntargetable bool
	SortBy             string
	SortOrder          string
	Limit              int
	ExcludeUnitIDs     []string
	IncludeUnitIDs     []string
}

type DamageAction struct {
	SourceID                                     string
	DisplaySourceID                              string
	TargetID                                     string
	Amount                                       float64
	Crit                                         bool
	DamageType                                   string
	ExtraDamageAmplification                     float64
	EnemyOnlyDamageReductionPierce               float64
	EnemyOnlyDamageReductionPierceTargetStatuses []string
	SuppressEquipmentHooks                       bool
	Reason                                       string
	DelayMS                                      int64
	Meta                                         map[string]any
}

type HealAction struct {
	SourceID string
	TargetID string
	Amount   float64
	Reason   string
	DelayMS  int64
	Meta     map[string]any
}

type ResourceAction struct {
	SourceID string
	TargetID string
	Resource string
	Amount   float64
	Reason   string
	DelayMS  int64
	Meta     map[string]any
}

type BuffAction struct {
	SourceID      string
	TargetID      string
	BuffID        string
	DurationTurns int64
	Payload       map[string]any
	Reason        string
	DelayMS       int64
	Meta          map[string]any
}

type SummonAction struct {
	SourceID string
	Summon   SummonRuntime
	Reason   string
	DelayMS  int64
	Meta     map[string]any
}

type ActionRuntime interface {
	Memory() BattleRuntime
	SelectSkillTargets(skill SkillRuntime) []UnitRuntime
	SelectTargets(query TargetQuery) []UnitRuntime
	ApplyDamage(action DamageAction) SkillEffect
	ApplyHeal(action HealAction) SkillEffect
	ApplyResource(action ResourceAction) SkillEffect
	ApplyBuff(action BuffAction) SkillEffect
	ApplySummon(action SummonAction) SkillEffect
	AppliedEffects() []SkillEffect
	ActionLog() []map[string]any
}
