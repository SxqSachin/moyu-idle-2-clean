package store

import (
	"encoding/json"
	"time"
)

type PlayerState struct {
	UserID               int64                     `json:"userId"`
	Buildings            map[string]int64          `json:"buildings,omitempty"`
	Kitties              []PlayerKitty             `json:"kitties,omitempty"`
	CatVillageReputation CatVillageReputationState `json:"catVillageReputation"`
	UpdatedAt            time.Time                 `json:"updatedAt"`
}

type CatVillageReputationState struct {
	TotalReputation int64 `json:"totalReputation"`
	Level           int   `json:"level"`
	LevelReputation int64 `json:"levelReputation"`
	NextLevelXp     int64 `json:"nextLevelXp"`
}

type PlayerKitty struct {
	ID             int64          `json:"id"`
	Name           string         `json:"name"`
	Status         string         `json:"status,omitempty"`
	Level          int            `json:"level"`
	BattleLevelCap int64          `json:"battleLevelCap"`
	Exp            int64          `json:"exp"`
	NextExp        int64          `json:"nextExp"`
	TotalExp       int64          `json:"totalExp"`
	BondExp        int64          `json:"bondExp"`
	Mood           float64        `json:"mood"`
	Extra          map[string]any `json:"extra,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type PlayerKittyJournal struct {
	ID            int64          `json:"id"`
	UserID        int64          `json:"userId"`
	KittyID       int64          `json:"kittyId"`
	EncounterID   *int64         `json:"encounterId,omitempty"`
	EntryType     string         `json:"entryType"`
	TitleKey      string         `json:"titleKey,omitempty"`
	ContentKey    string         `json:"contentKey"`
	ContentParams map[string]any `json:"contentParams,omitempty"`
	ActionID      string         `json:"actionId,omitempty"`
	Mood          *float64       `json:"mood,omitempty"`
	Reward        map[string]any `json:"reward,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type PlayerKittyEncounter struct {
	ID            int64          `json:"id"`
	FromUserID    int64          `json:"fromUserId"`
	FromKittyID   int64          `json:"fromKittyId"`
	ToUserID      int64          `json:"toUserId"`
	ToKittyID     int64          `json:"toKittyId"`
	ContentKey    string         `json:"contentKey"`
	ContentParams map[string]any `json:"contentParams,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
}

type PlayerKittyCommissionRun struct {
	ID           int64          `json:"id"`
	UserID       int64          `json:"userId"`
	KittyID      int64          `json:"kittyId"`
	CommissionID string         `json:"commissionId"`
	Status       string         `json:"status"`
	Costs        map[string]any `json:"costs,omitempty"`
	Reward       map[string]any `json:"reward,omitempty"`
	StartedAt    time.Time      `json:"startedAt"`
	ReadyAt      time.Time      `json:"readyAt"`
	ClaimedAt    *time.Time     `json:"claimedAt,omitempty"`
}

type BattleSettlementJob struct {
	ID          int64      `json:"id"`
	RoomID      int64      `json:"roomId"`
	RunID       int64      `json:"runId"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"lastError,omitempty"`
	AvailableAt time.Time  `json:"availableAt"`
	LockedBy    string     `json:"lockedBy,omitempty"`
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type BattleItemBagQuantityUpdate struct {
	UserID   int64
	ItemID   string
	Quantity int64
}

type BattleSkillExpJob struct {
	ID          int64      `json:"id"`
	RunID       int64      `json:"runId"`
	Status      string     `json:"status"`
	Payload     []byte     `json:"payload"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"lastError,omitempty"`
	AvailableAt time.Time  `json:"availableAt"`
	LockedBy    string     `json:"lockedBy,omitempty"`
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type BattleAttrExpJob struct {
	ID          int64      `json:"id"`
	RunID       int64      `json:"runId"`
	Status      string     `json:"status"`
	Payload     []byte     `json:"payload"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"lastError,omitempty"`
	AvailableAt time.Time  `json:"availableAt"`
	LockedBy    string     `json:"lockedBy,omitempty"`
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
	AppliedAt   *time.Time `json:"appliedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type ChatMessage struct {
	ID            int64          `json:"id"`
	Channel       string         `json:"channel"`
	UserID        int64          `json:"userId"`
	Username      string         `json:"username"`
	TitlePrefixID string         `json:"titlePrefixId"`
	TitleSuffixID string         `json:"titleSuffixId"`
	DisplayMeta   map[string]any `json:"displayMeta"`
	Attachment    map[string]any `json:"attachment,omitempty"`
	Content       string         `json:"content"`
	SentAt        time.Time      `json:"sentAt"`
}

type ItemInstance struct {
	InstanceID      int64                   `json:"instanceId"`
	UserID          int64                   `json:"userId"`
	TemplateItemID  string                  `json:"templateItemId"`
	CustomData      ItemCustomization       `json:"customData,omitempty"`
	FishData        *FishInstanceData       `json:"fishData,omitempty"`
	Quantity        int64                   `json:"quantity"`
	EnhanceLevel    int                     `json:"enhanceLevel"`
	BaseSnapshot    map[string]any          `json:"baseSnapshot"`
	Affixes         []EquipmentAffixSlot    `json:"affixes"`
	Durability      int                     `json:"durability"`
	MaxDurability   int                     `json:"maxDurability"`
	State           string                  `json:"state"`
	ArchivedAt      *time.Time              `json:"archivedAt,omitempty"`
	CreatedAt       time.Time               `json:"createdAt"`
	UpdatedAt       time.Time               `json:"updatedAt"`
	ForgedSkillbook *ForgedSkillbookSummary `json:"forgedSkillBook,omitempty"`
}

type FishInstanceData struct {
	SpeciesID                     string     `json:"speciesId"`
	AgeDays                       int        `json:"ageDays"`
	BioAgeFraction                float64    `json:"bioAgeFraction,omitempty"`
	Sex                           string     `json:"sex"`
	LengthCM                      float64    `json:"lengthCm"`
	WeightKG                      float64    `json:"weightKg"`
	Condition                     float64    `json:"condition"`
	Health                        float64    `json:"health"`
	WaterID                       string     `json:"waterId"`
	SpotID                        string     `json:"spotId"`
	CaughtAt                      time.Time  `json:"caughtAt"`
	RulesetVersion                string     `json:"rulesetVersion"`
	RecordFlags                   []string   `json:"recordFlags,omitempty"`
	Origin                        string     `json:"origin,omitempty"`
	PondZone                      string     `json:"pondZone,omitempty"`
	LifeState                     string     `json:"lifeState,omitempty"`
	Generation                    int        `json:"generation,omitempty"`
	ParentInstanceIDs             []int64    `json:"parentInstanceIds,omitempty"`
	Genes                         FishGenes  `json:"genes,omitempty"`
	LongevityLimitDays            float64    `json:"longevityLimitDays,omitempty"`
	NaturalProtectionUntilAgeDays float64    `json:"naturalProtectionUntilAgeDays,omitempty"`
	PondCareRevision              int64      `json:"pondCareRevision,omitempty"`
	GenomeRevealed                bool       `json:"genomeRevealed,omitempty"`
	Satiety                       float64    `json:"satiety,omitempty"`
	PreferredFeedHours            float64    `json:"preferredFeedHours,omitempty"`
	StarvationHours               float64    `json:"starvationHours,omitempty"`
	PondEnteredAt                 *time.Time `json:"pondEnteredAt,omitempty"`
	BornAt                        *time.Time `json:"bornAt,omitempty"`
	LastBredAt                    *time.Time `json:"lastBredAt,omitempty"`
	DeathAt                       *time.Time `json:"deathAt,omitempty"`
	DeathCause                    string     `json:"deathCause,omitempty"`
}

type FishGenes struct {
	SizePotential  float64  `json:"sizePotential,omitempty"`
	GrowthRate     float64  `json:"growthRate,omitempty"`
	FeedEfficiency float64  `json:"feedEfficiency,omitempty"`
	Vitality       float64  `json:"vitality,omitempty"`
	Lifespan       float64  `json:"lifespan,omitempty"`
	Fertility      float64  `json:"fertility,omitempty"`
	Mutations      []string `json:"mutations,omitempty"`
}

type ItemCustomization struct {
	Name       string `json:"name,omitempty"`
	LabelColor string `json:"labelColor,omitempty"`
}

type EquipmentAffixSlot struct {
	SlotIndex             int                `json:"slotIndex"`
	AffixID               string             `json:"affixId"`
	RarityID              string             `json:"rarityId"`
	SourceKind            string             `json:"sourceKind,omitempty"`
	AlchemyRulesetVersion string             `json:"alchemyRulesetVersion,omitempty"`
	DisplayNameKey        string             `json:"displayNameKey,omitempty"`
	DisplayNameArgs       map[string]any     `json:"displayNameArgs,omitempty"`
	RollValue             float64            `json:"rollValue,omitempty"`
	ValuePrecision        int64              `json:"valuePrecision,omitempty"`
	AttrMods              map[string]float64 `json:"attrMods,omitempty"`
	EffectType            string             `json:"effectType,omitempty"`
	EffectParams          map[string]any     `json:"effectParams,omitempty"`
}

type EquippedItem struct {
	InstanceID     int64                `json:"instanceId"`
	TemplateItemID string               `json:"templateItemId"`
	CustomData     ItemCustomization    `json:"customData,omitempty"`
	EquipType      string               `json:"equipType"`
	EnhanceLevel   int                  `json:"enhanceLevel"`
	Affixes        []EquipmentAffixSlot `json:"affixes,omitempty"`
	Durability     int                  `json:"durability"`
	MaxDurability  int                  `json:"maxDurability"`
	State          string               `json:"state"`
	EquippedSlot   string               `json:"equippedSlot"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

type PlayerStats struct {
	Derived   map[string]float64 `json:"derived"`
	CacheVer  int64              `json:"cacheVersion"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

type PlayerAttributeSeed struct {
	AttrKey                 string  `json:"attrKey"`
	BaseDefault             float64 `json:"baseDefault"`
	GrowthType              string  `json:"growthType"`
	Unit                    string  `json:"unit"`
	Category                string  `json:"category"`
	Order                   int64   `json:"order"`
	NameKey                 string  `json:"nameKey"`
	DescKey                 string  `json:"descKey"`
	IconID                  string  `json:"iconId"`
	EmojiIcon               string  `json:"emojiIcon"`
	AllowBattlePointLevelUp bool    `json:"allowBattlePointLevelUp"`
}

type PlayerAttributeState struct {
	AttrKey                 string         `json:"attrKey"`
	BaseDefault             float64        `json:"baseDefault"`
	BaseLevel               int64          `json:"baseLevel"`
	SkillPointLevel         int64          `json:"skillPointLevel"`
	AttrExp                 float64        `json:"attrExp"`
	FinalValue              float64        `json:"finalValue"`
	SourceMods              map[string]any `json:"sourceMods"`
	LockVersion             int64          `json:"lockVersion"`
	GrowthType              string         `json:"growthType"`
	Unit                    string         `json:"unit"`
	Category                string         `json:"category"`
	Order                   int64          `json:"order"`
	NameKey                 string         `json:"nameKey"`
	DescKey                 string         `json:"descKey"`
	IconID                  string         `json:"iconId"`
	EmojiIcon               string         `json:"emojiIcon"`
	ProgressLevel           int64          `json:"progressLevel"`
	ProgressExp             int64          `json:"progressExp"`
	ProgressNextExp         int64          `json:"progressNextExp"`
	AllowBattlePointLevelUp bool           `json:"allowBattlePointLevelUp"`
}

type PlayerBattlePointState struct {
	UserID      int64 `json:"userId"`
	TotalPoints int64 `json:"totalPoints"`
	SpentPoints int64 `json:"spentPoints"`
}

type KittyBattlePointState struct {
	KittyID     int64 `json:"kittyId"`
	TotalPoints int64 `json:"totalPoints"`
	SpentPoints int64 `json:"spentPoints"`
}

type BattleRoom struct {
	ID                  int64      `json:"id"`
	OwnerID             int64      `json:"ownerId"`
	DungeonID           string     `json:"dungeonId"`
	RoomName            string     `json:"roomName"`
	AutoReenterOnDefeat bool       `json:"autoReenterOnDefeat"`
	DisableEntryRound   bool       `json:"disableEntryRound"`
	IsPublic            bool       `json:"isPublic"`
	IsFogMode           bool       `json:"isFogMode"`
	Status              string     `json:"status"`
	DifficultyLevel     int        `json:"difficultyLevel"`
	CompletedRuns       int        `json:"completedRuns"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	MemberCount         int        `json:"memberCount,omitempty"`
	CurrentRunID        *int64     `json:"currentRunId,omitempty"`
	Seed                int64      `json:"seed"`
	EnteringStartedAt   *time.Time `json:"enteringStartedAt,omitempty"`
	RecoveringStartedAt *time.Time `json:"recoveringStartedAt,omitempty"`
	RecoveryEndsAt      *time.Time `json:"recoveryEndsAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	// Phase is a backend-only sub-state within status="playing".
	// Not exposed to API/frontend (json:"-").
	Phase string `json:"-"`
	// SchemaVersion partitions rooms between v1 (legacy) and v2 dispatch loops.
	// Not exposed to API/frontend (json:"-").
	SchemaVersion int `json:"-"`
}

type BattleRoomMember struct {
	RoomID    int64      `json:"roomId"`
	UserID    int64      `json:"userId"`
	Username  string     `json:"username,omitempty"`
	Position  string     `json:"position"`
	IsReady   bool       `json:"isReady"`
	ReadyAt   *time.Time `json:"readyAt,omitempty"`
	JoinedAt  time.Time  `json:"joinedAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type BattleRoomUnit struct {
	RoomID                         int64     `json:"roomId"`
	OwnerUserID                    int64     `json:"ownerUserId"`
	Username                       string    `json:"username,omitempty"`
	UnitType                       string    `json:"unitType"`
	UnitID                         int64     `json:"unitId"`
	KittyID                        *int64    `json:"kittyId,omitempty"`
	KittyName                      string    `json:"kittyName,omitempty"`
	Position                       string    `json:"position"`
	PositionLockedByPersonalityID  string    `json:"positionLockedByPersonalityId,omitempty"`
	PositionLockedByPersonalityKey string    `json:"positionLockedByPersonalityKey,omitempty"`
	IsReady                        bool      `json:"isReady"`
	JoinedAt                       time.Time `json:"joinedAt"`
	UpdatedAt                      time.Time `json:"updatedAt"`
}

type BattleRoomDetail struct {
	Room    BattleRoom         `json:"room"`
	Members []BattleRoomMember `json:"members"`
	Units   []BattleRoomUnit   `json:"units,omitempty"`
}

type BattleRun struct {
	ID          int64            `json:"id"`
	RoomID      int64            `json:"roomId"`
	Wave        int              `json:"wave"`
	Status      string           `json:"status"`
	State       map[string]any   `json:"state"`
	Timeline    []map[string]any `json:"timeline,omitempty"`
	FinalResult map[string]any   `json:"finalResult,omitempty"`
	StartedAt   time.Time        `json:"startedAt"`
	EndedAt     *time.Time       `json:"endedAt,omitempty"`
	UpdatedAt   time.Time        `json:"updatedAt"`
}

type BattleRunColdPayload struct {
	RunID        int64
	CompileInput map[string]any
	Timeline     []map[string]any
	FinalResult  map[string]any
}

// BattlePreviousWaveCarry is the minimal predecessor payload needed to seed a
// new wave. CarryStateRaw is intentionally decoded by the battle runtime,
// whose battleUnitState type owns the persisted carry contract.
type BattlePreviousWaveCarry struct {
	RunID         int64
	RoomID        int64
	Wave          int
	Status        string
	CarryStateRaw json.RawMessage
}

type BattleSlowRunArchive struct {
	ID                int64            `json:"id"`
	RoomID            int64            `json:"roomId"`
	RunID             int64            `json:"runId"`
	DungeonID         string           `json:"dungeonId"`
	WorkerID          string           `json:"workerId"`
	Source            string           `json:"source"`
	Wave              int              `json:"wave"`
	Outcome           string           `json:"outcome"`
	QueueSinceStartMs int64            `json:"queueSinceStartMs"`
	CompileElapsedMs  int64            `json:"compileElapsedMs"`
	TotalMs           int64            `json:"totalMs"`
	TotalSteps        int              `json:"totalSteps"`
	TotalTurns        int64            `json:"totalTurns"`
	TimelineEvents    int              `json:"timelineEvents"`
	FinalSeq          int64            `json:"finalSeq"`
	PlayerUnits       int              `json:"playerUnits"`
	GuardTimedOut     bool             `json:"guardTimedOut"`
	GuardStepLimited  bool             `json:"guardStepLimited"`
	TriggerFlags      map[string]any   `json:"triggerFlags"`
	TurnProfile       map[string]any   `json:"turnProfile"`
	CompileInput      map[string]any   `json:"compileInput,omitempty"`
	Checkpoint        map[string]any   `json:"checkpoint,omitempty"`
	Timeline          []map[string]any `json:"timeline,omitempty"`
	FinalResult       map[string]any   `json:"finalResult,omitempty"`
	RunStartedAt      *time.Time       `json:"runStartedAt,omitempty"`
	CapturedAt        time.Time        `json:"capturedAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
}

type BattleSlowRunArchiveSummary struct {
	ID                int64          `json:"id"`
	RoomID            int64          `json:"roomId"`
	RunID             int64          `json:"runId"`
	DungeonID         string         `json:"dungeonId"`
	WorkerID          string         `json:"workerId"`
	Source            string         `json:"source"`
	Wave              int            `json:"wave"`
	Outcome           string         `json:"outcome"`
	QueueSinceStartMs int64          `json:"queueSinceStartMs"`
	CompileElapsedMs  int64          `json:"compileElapsedMs"`
	TotalMs           int64          `json:"totalMs"`
	TotalSteps        int            `json:"totalSteps"`
	TotalTurns        int64          `json:"totalTurns"`
	TimelineEvents    int            `json:"timelineEvents"`
	FinalSeq          int64          `json:"finalSeq"`
	PlayerUnits       int            `json:"playerUnits"`
	GuardTimedOut     bool           `json:"guardTimedOut"`
	GuardStepLimited  bool           `json:"guardStepLimited"`
	TriggerFlags      map[string]any `json:"triggerFlags"`
	TurnProfile       map[string]any `json:"turnProfile"`
	RunStartedAt      *time.Time     `json:"runStartedAt,omitempty"`
	CapturedAt        time.Time      `json:"capturedAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

type BattleSlowRunArchiveFilter struct {
	RoomID   int64 `json:"roomId"`
	RunID    int64 `json:"runId"`
	BeforeID int64 `json:"beforeId"`
	Limit    int   `json:"limit"`
}

type BattleSlowRunArchiveInput struct {
	RoomID            int64
	RunID             int64
	DungeonID         string
	WorkerID          string
	Source            string
	Wave              int
	Outcome           string
	QueueSinceStartMs int64
	CompileElapsedMs  int64
	TotalMs           int64
	TotalSteps        int
	TotalTurns        int64
	TimelineEvents    int
	FinalSeq          int64
	PlayerUnits       int
	GuardTimedOut     bool
	GuardStepLimited  bool
	TriggerFlags      map[string]any
	TurnProfile       map[string]any
	CompileInput      map[string]any
	Checkpoint        map[string]any
	Timeline          []map[string]any
	FinalResult       map[string]any
	RunStartedAt      *time.Time
}

type CatVillageTask struct {
	ID              int64        `json:"id"`
	CommissionID    string       `json:"commissionId"`
	Costs           []CostItem   `json:"costs"`
	RewardItems     []RewardItem `json:"rewardItems"`
	ReputationGrant int64        `json:"reputationGrant"`
	Refreshed       bool         `json:"refreshed"`
	Status          string       `json:"status"`
	CreatedAt       time.Time    `json:"createdAt"`
	CompletedAt     *time.Time   `json:"completedAt,omitempty"`
	IsRumor         bool         `json:"isRumor,omitempty"`
	IsBattle        bool         `json:"isBattle,omitempty"`
	RumorBadge      string       `json:"rumorBadge,omitempty"`
	NotRefreshable  bool         `json:"notRefreshable,omitempty"`
	NotDeletable    bool         `json:"notDeletable,omitempty"`
	PityThreshold   int          `json:"pityThreshold,omitempty"`
	PityCounter     int64        `json:"pityCounter,omitempty"`
}

type PlayerCollectible struct {
	CollectibleID string    `json:"collectibleId"`
	Level         int       `json:"level"`
	ObtainedAt    time.Time `json:"obtainedAt"`
}

type CostItem struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

type RewardItem struct {
	ItemID string `json:"itemId"`
	Count  int64  `json:"count"`
}

type ShopOffer struct {
	ID            int64  `json:"id"`
	ItemID        string `json:"itemId"`
	ShopBuyPrice  int64  `json:"shopBuyPrice"`
	ShopSellPrice int64  `json:"shopSellPrice"`
	Active        bool   `json:"active"`
}

type MarketFeeTierStat struct {
	FeeRateBps int64 `json:"feeRateBps"`
	OrderCount int64 `json:"orderCount"`
	FeeAmount  int64 `json:"feeAmount"`
}

type MarketFeeSummary struct {
	OrdersTotal       int64               `json:"ordersTotal"`
	FeeAmountTotal    int64               `json:"feeAmountTotal"`
	SellerAmountTotal int64               `json:"sellerAmountTotal"`
	Tiers             []MarketFeeTierStat `json:"tiers"`
}

type RuleSet struct {
	RuleKind string `json:"ruleKind"`
	SlotKey  string `json:"slotKey"`
	Enabled  bool   `json:"enabled"`
	Groups   any    `json:"groups"`
}

type CombatConfig struct {
	EquipmentSkills map[string]EquipmentSkillConfig `json:"equipmentSkills,omitempty"`
	SkillSlots      []string                        `json:"skillSlots"`
	UpdatedAt       time.Time                       `json:"updatedAt"`
}

type LoadoutEquipmentSlot struct {
	Slot       string `json:"slot"`
	InstanceID int64  `json:"instanceId"`
}

type LoadoutSkillRule struct {
	SkillID string `json:"skillId"`
	Enabled bool   `json:"enabled"`
	Groups  any    `json:"groups"`
}

type BattleLoadout struct {
	EquipmentSkills map[string]EquipmentSkillConfig `json:"equipmentSkills,omitempty"`
	ID              string                          `json:"id"`
	Name            string                          `json:"name"`
	Equipment       []LoadoutEquipmentSlot          `json:"equipment"`
	SkillSlots      []string                        `json:"skillSlots"`
	SkillRules      []LoadoutSkillRule              `json:"skillRules"`
}

type WorkLoadout struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Equipment []LoadoutEquipmentSlot `json:"equipment"`
}

type PlayerLoadouts struct {
	BattleLoadouts     []BattleLoadout   `json:"battleLoadouts"`
	WorkLoadouts       []WorkLoadout     `json:"workLoadouts"`
	WorkActionBindings map[string]string `json:"workActionBindings"`
}

type WorkLoadoutRuntimeResolution struct {
	LoadoutID         string                 `json:"loadoutId"`
	Available         bool                   `json:"available"`
	FallbackReason    string                 `json:"fallbackReason,omitempty"`
	AttributeTotals   map[string]float64     `json:"attributeTotals,omitempty"`
	AttributeDeltas   map[string]float64     `json:"attributeOverrides,omitempty"`
	EquipmentSnapshot []LoadoutEquipmentSlot `json:"equipmentSnapshot,omitempty"`
}

type WorkLoadoutUsage struct {
	ActionID       string `json:"actionId"`
	LoadoutID      string `json:"loadoutId,omitempty"`
	Applied        bool   `json:"applied"`
	FallbackReason string `json:"fallbackReason,omitempty"`
	Offline        bool   `json:"offline"`
}

type ActivatedWorkLoadout struct {
	ActionID    string `json:"actionId"`
	LoadoutID   string `json:"loadoutId"`
	LoadoutName string `json:"loadoutName,omitempty"`
}

type BattleLoadoutSkippedSkill struct {
	SkillID string `json:"skillId"`
	Reason  string `json:"reason"`
}

type BattleLoadoutApplyResult struct {
	SkippedSkills []BattleLoadoutSkippedSkill `json:"skippedSkills,omitempty"`
}

type BattleItemBagEntry struct {
	SlotNo    int       `json:"slotNo"`
	ItemID    string    `json:"itemId"`
	Quantity  int64     `json:"quantity"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type OwnedSkill struct {
	SkillID string `json:"skillId"`
	Level   int    `json:"level"`
	Exp     int64  `json:"exp"`
}

type PlayerSkill struct {
	EquipmentConfig *EquipmentSkillConfig `json:"equipmentConfig,omitempty"`
	SkillID         string                `json:"skillId"`
	Enabled         bool                  `json:"enabled"`
	CastOrder       int                   `json:"castOrder"`
	RuleEnabled     bool                  `json:"ruleEnabled"`
	RuleGroups      any                   `json:"ruleGroups"`
	Level           int                   `json:"level"`
	Exp             int64                 `json:"exp"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

type SkillbookUseResult struct {
	SkillID     string `json:"skillId"`
	GainedSkill bool   `json:"gainedSkill"`
	GainedExp   int64  `json:"gainedExp"`
	Level       int    `json:"level"`
	Exp         int64  `json:"exp"`
}

type AutoSkillUseResult struct {
	SkillID   string    `json:"skillId"`
	UsedAt    time.Time `json:"usedAt"`
	Level     int       `json:"level"`
	Exp       int64     `json:"exp"`
	NextExp   int64     `json:"nextExp"`
	Triggered bool      `json:"triggered"`
}

type AttributeExp struct {
	Values    map[string]float64 `json:"-"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

func (a AttributeExp) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(a.Values)+1)
	for key, value := range a.Values {
		out[key] = value
	}
	out["updatedAt"] = a.UpdatedAt
	return json.Marshal(out)
}

type ActionSpec struct {
	ID                           string                                       `json:"id"`
	MainType                     string                                       `json:"mainType"`
	SubType                      string                                       `json:"subType"`
	Menu                         string                                       `json:"menu"`
	DurationS                    int64                                        `json:"durationS"`
	Costs                        map[string]int64                             `json:"costs"`
	MaterialInstanceRequirements map[string]ActionMaterialInstanceRequirement `json:"materialInstanceRequirements,omitempty"`
	StatusRequirement            map[string]int64                             `json:"statusRequirement"`
	BuildingRequirement          map[string]int64                             `json:"buildingRequirement"`
	ItemReward                   map[string]ActionItemRewardSpec              `json:"itemReward"`
	MainOutput                   string                                       `json:"mainOutput"`
	StatusExpReward              map[string]float64                           `json:"statusExpReward"`
}

type ActionMaterialInstanceRequirement struct {
	EnhanceLevel           int64  `json:"enhanceLevel"`
	RequireTemplateAffixes bool   `json:"requireTemplateAffixes"`
	Durability             string `json:"durability"`
}

type ActionItemRewardSpec struct {
	Count                     *int64                 `json:"count,omitempty"`
	Range                     *ActionItemRewardRange `json:"range,omitempty"`
	Percent                   *float64               `json:"percent,omitempty"`
	GrantPolicy               string                 `json:"grantPolicy,omitempty"`
	DefaultAffixRarityWeights map[string]float64     `json:"defaultAffixRarityWeights,omitempty"`
}

type ActionItemRewardRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type ActionQueueItem struct {
	ID         int64               `json:"id"`
	ActorType  string              `json:"actorType"`
	ActorID    int64               `json:"actorId"`
	ActionID   string              `json:"actionId"`
	Mode       string              `json:"mode"`
	TotalCount int                 `json:"totalCount"`
	DoneCount  int                 `json:"doneCount"`
	Status     string              `json:"status"`
	CreatedAt  time.Time           `json:"createdAt"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	RunStartAt time.Time           `json:"runStartAt,omitempty"`
	RunEndsAt  time.Time           `json:"runEndsAt,omitempty"`
	Display    *ActionQueueDisplay `json:"display,omitempty"`
}

// ActionQueueScheduleItem is the minimal post-settlement projection needed by
// the action worker to rebuild Redis's derived due index.
type ActionQueueScheduleItem struct {
	ActorType string
	ActorID   int64
	Status    string
	RunEndsAt time.Time
}

type ActionQueueDisplay struct {
	Kind                string                      `json:"kind"`
	EffectiveDurationMs int64                       `json:"effectiveDurationMs,omitempty"`
	Enhance             *ActionQueueEnhanceDisplay  `json:"enhance,omitempty"`
	Building            *ActionQueueBuildingDisplay `json:"building,omitempty"`
	Alchemy             *ActionQueueAlchemyDisplay  `json:"alchemy,omitempty"`
	Fishing             *ActionQueueFishingDisplay  `json:"fishing,omitempty"`
}

type ActionQueueFishingDisplay struct {
	TripID                 string              `json:"tripId"`
	WaterID                string              `json:"waterId"`
	SpotID                 string              `json:"spotId"`
	LoadoutName            string              `json:"loadoutName"`
	Method                 string              `json:"method"`
	Strategy               string              `json:"strategy"`
	Phase                  string              `json:"phase"`
	LineState              string              `json:"lineState,omitempty"`
	PlannedDurationSeconds int64               `json:"plannedDurationSeconds"`
	StartedAtMS            int64               `json:"startedAtMs,omitempty"`
	PlannedEndAtMS         int64               `json:"plannedEndAtMs,omitempty"`
	PhaseEndsAtMS          int64               `json:"phaseEndsAtMs,omitempty"`
	Landed                 int64               `json:"landed"`
	Released               int64               `json:"released"`
	TotalWeightKG          float64             `json:"totalWeightKg"`
	InitialSupplies        FishingSupplyCounts `json:"initialSupplies"`
	RemainingSupplies      FishingSupplyCounts `json:"remainingSupplies"`
}

type ActionQueueAlchemyDisplay struct {
	SubKind           string           `json:"subKind"`
	CarrierInstanceID int64            `json:"carrierInstanceId,omitempty"`
	CarrierItemID     string           `json:"carrierItemId,omitempty"`
	Materials         map[string]int64 `json:"materials,omitempty"`
	ComboRecipeID     string           `json:"comboRecipeId,omitempty"`
	MediumItemID      string           `json:"mediumItemId,omitempty"`
	FishInstanceID    int64            `json:"fishInstanceId,omitempty"`
	SourceItemID      string           `json:"sourceItemId,omitempty"`
	SourceCount       int              `json:"sourceCount,omitempty"`
	GroupID           string           `json:"groupId,omitempty"`
	ItemID            string           `json:"itemId,omitempty"`
	InstanceID        int64            `json:"instanceId,omitempty"`
	TotalCount        int              `json:"totalCount,omitempty"`
	FatherInstanceID  int64            `json:"fatherInstanceId,omitempty"`
	FatherAffixIndex  int              `json:"fatherAffixIndex,omitempty"`
	MotherInstanceID  int64            `json:"motherInstanceId,omitempty"`
	MotherAffixIndex  int              `json:"motherAffixIndex,omitempty"`
	GeneChoiceCount   int              `json:"geneChoiceCount,omitempty"`
	InheritGeneCount  int              `json:"inheritGeneCount,omitempty"`
	SuppressGeneCount int              `json:"suppressGeneCount,omitempty"`
}

type ActionQueueEnhanceDisplay struct {
	EquipmentInstanceID int64  `json:"equipmentInstanceId"`
	TemplateItemID      string `json:"templateItemId"`
	TargetEnhanceLevel  int    `json:"targetEnhanceLevel"`
	UseProtect          bool   `json:"useProtect"`
	ProtectStartLevel   int    `json:"protectStartLevel,omitempty"`
	ProtectItemID       string `json:"protectItemId,omitempty"`
}

type ActionQueueBuildingDisplay struct {
	BuildingID string `json:"buildingId"`
	FromLevel  int64  `json:"fromLevel"`
	ToLevel    int64  `json:"toLevel"`
	DurationS  int64  `json:"durationS"`
}

type WarehouseChangeEvent string

const (
	WarehouseChangeEventSystemToWarehouse WarehouseChangeEvent = "system_to_warehouse"
	WarehouseChangeEventWarehouseOp       WarehouseChangeEvent = "warehouse_operation"
)

type WarehouseChangeLog struct {
	ID            int64                   `json:"id"`
	EventType     WarehouseChangeEvent    `json:"eventType"`
	Category      InventoryAuditCategory  `json:"category"`
	Direction     InventoryAuditDirection `json:"direction"`
	SubjectType   string                  `json:"subjectType"`
	SubjectID     string                  `json:"subjectId"`
	WarehouseID   int64                   `json:"warehouseId"`
	InstanceID    int64                   `json:"instanceId"`
	TemplateItem  string                  `json:"templateItemId"`
	Quantity      int64                   `json:"quantity"`
	Items         map[string]int64        `json:"items"`
	InstanceIDs   []int64                 `json:"instanceIds"`
	SourceSurface string                  `json:"sourceSurface"`
	TargetSurface string                  `json:"targetSurface"`
	Details       map[string]any          `json:"details"`
	CreatedAt     time.Time               `json:"createdAt"`
}

type WarehouseChangeLogFilter struct {
	WarehouseID int64
	EventType   WarehouseChangeEvent
	Category    InventoryAuditCategory
	Limit       int
	Since       time.Time
}

type PresenceState struct {
	UserID         int64     `json:"userId"`
	Status         string    `json:"status"`
	LastSeenAt     time.Time `json:"lastSeenAt"`
	SessionVersion int64     `json:"sessionVersion"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type EquipmentOperationInfo struct {
	InstanceID     int64                  `json:"instanceId"`
	TemplateItemID string                 `json:"templateItemId"`
	EnhanceLevel   int                    `json:"enhanceLevel"`
	AffixSlotCount int64                  `json:"affixSlotCount,omitempty"`
	AffixSlots     []EquipmentAffixSlot   `json:"affixSlots,omitempty"`
	DetachRules    []EquipmentDetachRule  `json:"detachRules,omitempty"`
	EnchantRules   []EquipmentEnchantRule `json:"enchantRules,omitempty"`
	CarrierRules   []EquipmentCarrierRule `json:"carrierRules,omitempty"`
}

type EquipmentDetachRule struct {
	ToolItemID                  string  `json:"toolItemId"`
	DestroyChanceBase           float64 `json:"destroyChanceBase"`
	LuckImpactFactor            float64 `json:"luckImpactFactor"`
	AffixDropBeforeDestroyCheck bool    `json:"affixDropBeforeDestroyCheck"`
}

type EquipmentEnchantRule struct {
	ToolItemID      string             `json:"toolItemId"`
	ConsumeCount    int64              `json:"consumeCount"`
	Costs           map[string]int64   `json:"costs"`
	FailChance      float64            `json:"failChance"`
	RarityWeights   map[string]float64 `json:"rarityWeights"`
	AllowedAffixIDs []string           `json:"allowedAffixIds,omitempty"`
}

type EquipmentCarrierRule struct {
	CarrierItemID    string   `json:"carrierItemId"`
	AllowedRarityIDs []string `json:"allowedRarityIds,omitempty"`
}
