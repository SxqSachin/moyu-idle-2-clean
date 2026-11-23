package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"mysecondwebgame/server/internal/catalog"

	"github.com/jackc/pgx/v5"
)

var (
	ErrSpecializationTreeNotFound           = errors.New("specialization tree not found")
	ErrSpecializationTreeLocked             = errors.New("specialization tree locked")
	ErrSpecializationNodeNotFound           = errors.New("specialization node not found")
	ErrSpecializationNodeMaxRank            = errors.New("specialization node max rank")
	ErrSpecializationRequirementsUnmet      = errors.New("specialization requirements unmet")
	ErrSpecializationActiveRootLimit        = errors.New("specialization active root limit reached")
	ErrSpecializationExclusiveConflict      = errors.New("specialization exclusive group conflict")
	ErrSpecializationInsufficientPoints     = errors.New("specialization points insufficient")
	ErrSpecializationRevisionConflict       = errors.New("specialization revision conflict")
	ErrSpecializationRulesetConflict        = errors.New("specialization ruleset conflict")
	ErrSpecializationImpactConflict         = errors.New("specialization equipment impact conflict")
	ErrSpecializationIdempotencyKeyRequired = errors.New("specialization idempotency key required")
	ErrSpecializationIdempotencyConflict    = errors.New("specialization idempotency conflict")
	ErrSpecializationDefinitionInvalid      = errors.New("specialization definition invalid")
)

const (
	playerCombatSpecializationTreeID = "player_combat_specialization"
	specializationRulesetV6          = "specialization-v6"
)

// IsValidSpecializationRuntimeIDPart reports whether an author-owned grant,
// hook, or mutation ID can be composed into an unambiguous runtime identity.
func IsValidSpecializationRuntimeIDPart(raw string) bool {
	if raw == "" || raw != strings.TrimSpace(raw) || raw[0] < 'a' || raw[0] > 'z' {
		return false
	}
	for index := 1; index < len(raw); index++ {
		char := raw[index]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

type SpecializationAllocation struct {
	NodeID   string `json:"nodeId"`
	Rank     int    `json:"rank"`
	PaidCost int    `json:"paidCost"`
}

type SpecializationNodeState struct {
	NodeID           string   `json:"nodeId"`
	Rank             int      `json:"rank"`
	MaxRank          int      `json:"maxRank"`
	NextCost         int      `json:"nextCost"`
	CanAllocate      bool     `json:"canAllocate"`
	UnmetReasonCodes []string `json:"unmetReasonCodes"`
}

type SpecializationResetCost struct {
	ItemID string `json:"itemId"`
	Count  int64  `json:"count"`
}

const (
	specializationPointSourceUnlockGrant    = "unlock_grant"
	specializationPointSourceLevelMilestone = "level_milestone"
	specializationPointSourceReward         = "reward"
)

type SpecializationPointSource struct {
	SourceID            string `json:"sourceId"`
	Kind                string `json:"kind"`
	LabelKey            string `json:"labelKey"`
	BandID              string `json:"bandId,omitempty"`
	RequiredBattleLevel *int64 `json:"requiredBattleLevel"`
	Points              int    `json:"points"`
	CountedPoints       int    `json:"countedPoints"`
	Earned              bool   `json:"earned"`
}

type SpecializationPointMilestoneBand struct {
	BandID                   string `json:"bandId"`
	LabelKey                 string `json:"labelKey"`
	FromBattleLevel          int64  `json:"fromBattleLevel"`
	ToBattleLevel            int64  `json:"toBattleLevel"`
	EveryLevels              int64  `json:"everyLevels"`
	PointsPerGrant           int    `json:"pointsPerGrant"`
	EarnedGrantCount         int    `json:"earnedGrantCount"`
	EarnedPoints             int    `json:"earnedPoints"`
	CountedPoints            int    `json:"countedPoints"`
	FirstEarnedBattleLevel   *int64 `json:"firstEarnedBattleLevel"`
	LastEarnedBattleLevel    *int64 `json:"lastEarnedBattleLevel"`
	NextMilestoneBattleLevel *int64 `json:"nextMilestoneBattleLevel"`
}

type SpecializationPointBreakdown struct {
	UnlockGrantPoints    int `json:"unlockGrantPoints"`
	LevelMilestonePoints int `json:"levelMilestonePoints"`
	RewardPoints         int `json:"rewardPoints"`
	CountedRewardPoints  int `json:"countedRewardPoints"`
	// CappedPoints is the earned source total excluded from TotalPoints by MaxPoints.
	CappedPoints         int                                `json:"cappedPoints"`
	MaxPoints            int                                `json:"maxPoints"`
	AtCap                bool                               `json:"atCap"`
	NextPointBattleLevel *int64                             `json:"nextPointBattleLevel"`
	NextPointPoints      int                                `json:"nextPointPoints"`
	MilestoneBands       []SpecializationPointMilestoneBand `json:"milestoneBands"`
	Sources              []SpecializationPointSource        `json:"sources"`
}

type SpecializationStatus struct {
	TreeID          string                       `json:"treeId"`
	RulesetVersion  string                       `json:"rulesetVersion"`
	BalanceRevision int                          `json:"balanceRevision"`
	Revision        int64                        `json:"revision"`
	BattleLevel     int64                        `json:"battleLevel"`
	FormulaPoints   int                          `json:"formulaPoints"`
	BonusPoints     int                          `json:"bonusPoints"`
	TotalPoints     int                          `json:"totalPoints"`
	SpentPoints     int                          `json:"spentPoints"`
	AvailablePoints int                          `json:"availablePoints"`
	PointBreakdown  SpecializationPointBreakdown `json:"pointBreakdown"`
	Allocations     []SpecializationAllocation   `json:"allocations"`
	NodeStates      []SpecializationNodeState    `json:"nodeStates"`
	ResetCost       SpecializationResetCost      `json:"resetCost"`
}

type SpecializationAutoUnequippedItem struct {
	InstanceID     int64    `json:"instanceId"`
	TemplateItemID string   `json:"templateItemId"`
	FromSlots      []string `json:"fromSlots"`
	Reason         string   `json:"reason"`
}

type SpecializationEquipmentImpact struct {
	Digest         string                             `json:"digest"`
	DisabledSlots  []string                           `json:"disabledSlots"`
	EnabledSlots   []string                           `json:"enabledSlots"`
	AutoUnequipped []SpecializationAutoUnequippedItem `json:"autoUnequipped"`
	DisabledSkills []SpecializationDisabledSkill      `json:"disabledSkills"`
}

type SpecializationDisabledSkill struct {
	SkillID string `json:"skillId"`
	Reason  string `json:"reason"`
}

type SpecializationRulesetResetNotice struct {
	Reason                 string                        `json:"reason"`
	PreviousRulesetVersion string                        `json:"previousRulesetVersion"`
	CurrentRulesetVersion  string                        `json:"currentRulesetVersion"`
	PreviousRevision       int64                         `json:"previousRevision"`
	CurrentRevision        int64                         `json:"currentRevision"`
	RemovedAllocations     []SpecializationAllocation    `json:"removedAllocations"`
	Impact                 SpecializationEquipmentImpact `json:"impact"`
}

type SpecializationView struct {
	Tree                any                               `json:"tree"`
	Status              SpecializationStatus              `json:"status"`
	CharacterAttributes map[string]float64                `json:"characterAttributes"`
	EffectiveSlots      []EquipmentSlotDefinition         `json:"effectiveSlots"`
	SlotCapacities      map[string]int                    `json:"slotCapacities"`
	RulesetReset        *SpecializationRulesetResetNotice `json:"rulesetReset,omitempty"`
}

type SpecializationMutationResult struct {
	Committed       bool                          `json:"committed"`
	RequiresConfirm bool                          `json:"requiresConfirmation"`
	Status          SpecializationStatus          `json:"status"`
	EffectiveSlots  []EquipmentSlotDefinition     `json:"effectiveSlots"`
	SlotCapacities  map[string]int                `json:"slotCapacities"`
	Impact          SpecializationEquipmentImpact `json:"impact"`
}

type SpecializationAllocateCommand struct {
	TreeID                 string
	NodeID                 string
	ExpectedRevision       int64
	ExpectedRulesetVersion string
	ImpactDigest           string
	IdempotencyKey         string
}

type SpecializationResetCommand struct {
	TreeID                 string
	ExpectedRevision       int64
	ExpectedRulesetVersion string
	ImpactDigest           string
	IdempotencyKey         string
}

type specializationTreeSpec struct {
	ID                 string                        `json:"id"`
	Disabled           bool                          `json:"disabled"`
	SchemaVersion      int                           `json:"schemaVersion"`
	RulesetVersion     string                        `json:"rulesetVersion"`
	BalanceRevision    int                           `json:"balanceRevision"`
	OwnerType          string                        `json:"ownerType"`
	NameKey            string                        `json:"nameKey"`
	DescKey            string                        `json:"descKey"`
	I18n               map[string]specializationI18n `json:"i18n,omitempty"`
	InitialPoints      int                           `json:"initialPoints"`
	MaxPoints          int                           `json:"maxPoints"`
	MaxActiveRootNodes int                           `json:"maxActiveRootNodes"`
	Unlock             specializationUnlockSpec      `json:"unlock"`
	PointBands         []specializationPointBandSpec `json:"pointBands"`
	Reset              SpecializationResetCost       `json:"reset"`
	Keywords           []specializationKeywordSpec   `json:"keywords,omitempty"`
	Schools            []specializationSchoolSpec    `json:"schools"`
	Nodes              []specializationNodeSpec      `json:"nodes"`
	UI                 specializationTreeUISpec      `json:"ui"`
}

type specializationI18n struct {
	Name     string                               `json:"name"`
	Desc     string                               `json:"desc"`
	Schools  map[string]specializationI18nText    `json:"schools"`
	Nodes    map[string]specializationI18nText    `json:"nodes"`
	Keywords map[string]specializationKeywordI18n `json:"keywords,omitempty"`
}

type specializationKeywordSpec struct {
	ID                 string   `json:"id"`
	MechanicKind       string   `json:"mechanicKind"`
	RuntimeIDs         []string `json:"runtimeIds"`
	SourceNodeIDs      []string `json:"sourceNodeIds"`
	NameKey            string   `json:"nameKey"`
	SummaryKey         string   `json:"summaryKey"`
	MechanicDetailsKey string   `json:"mechanicDetailsKey"`
	SourceHintKey      string   `json:"sourceHintKey"`
}

type specializationKeywordI18n struct {
	Name            string `json:"name"`
	PlayerSummary   string `json:"playerSummary"`
	MechanicDetails string `json:"mechanicDetails"`
	SourceHint      string `json:"sourceHint"`
}

type specializationI18nText struct {
	Name               string     `json:"name"`
	Desc               string     `json:"desc"`
	EffectSummary      []string   `json:"effectSummary,omitempty"`
	LevelEffectSummary [][]string `json:"levelEffectSummary,omitempty"`
}

type specializationSchoolSpec struct {
	ID           string                     `json:"id"`
	NameKey      string                     `json:"nameKey"`
	DescKey      string                     `json:"descKey"`
	AttributeKey string                     `json:"attributeKey"`
	UI           specializationSchoolUISpec `json:"ui"`
}

type specializationSchoolUISpec struct {
	Column int    `json:"column"`
	Color  string `json:"color"`
}

type specializationTreeUISpec struct {
	Layout      string   `json:"layout"`
	SchoolOrder []string `json:"schoolOrder"`
}

type specializationUnlockSpec struct {
	BattleLevel int64 `json:"battleLevel"`
}

type specializationPointBandSpec struct {
	ID             string `json:"id"`
	FromLevel      int64  `json:"fromLevel"`
	ToLevel        int64  `json:"toLevel"`
	EveryLevels    int64  `json:"everyLevels"`
	PointsPerGrant int    `json:"pointsPerGrant"`
}

type specializationNodeSpec struct {
	ID               string                         `json:"id"`
	Kind             string                         `json:"kind"`
	SchoolIDs        []string                       `json:"schoolIds"`
	ExclusiveGroupID string                         `json:"exclusiveGroupId"`
	Requirements     specializationRequirementsSpec `json:"requirements"`
	Levels           []specializationLevelSpec      `json:"levels"`
	NameKey          string                         `json:"nameKey"`
	DescKey          string                         `json:"descKey"`
	KeywordIDs       []string                       `json:"keywordIds,omitempty"`
	UI               specializationNodeUISpec       `json:"ui"`
}

type specializationNodeUISpec struct {
	Column int      `json:"column"`
	Row    int      `json:"row"`
	Branch string   `json:"branch,omitempty"`
	Accent string   `json:"accent,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
}

type specializationRequirementsSpec struct {
	All   []specializationRequirementSpec   `json:"all"`
	AnyOf [][]specializationRequirementSpec `json:"anyOf"`
}

type specializationRequirementSpec struct {
	Kind         string  `json:"kind"`
	NodeID       string  `json:"nodeId"`
	MinRank      int     `json:"minRank"`
	AttributeKey string  `json:"attributeKey"`
	Value        float64 `json:"value"`
	SchoolID     string  `json:"schoolId"`
	Points       int     `json:"points"`
}

type specializationLevelSpec struct {
	Level  int                       `json:"level"`
	Cost   int                       `json:"cost"`
	Grants []specializationGrantSpec `json:"grants"`
}

type specializationGrantSpec struct {
	ID              string                       `json:"id"`
	Kind            string                       `json:"kind"`
	Scope           string                       `json:"scope"`
	Modifiers       []SpecializationStatModifier `json:"modifiers"`
	BattleHooks     []map[string]any             `json:"battleHooks"`
	EffectMutations []map[string]any             `json:"effectMutations"`
	EquipType       string                       `json:"equipType"`
	Deltas          map[string]int               `json:"deltas"`
}

type SpecializationStatModifier struct {
	ID            string  `json:"id,omitempty"`
	SourceNodeID  string  `json:"sourceNodeId,omitempty"`
	SourceNameKey string  `json:"sourceNameKey,omitempty"`
	Scope         string  `json:"scope"`
	AttributeKey  string  `json:"attributeKey"`
	Operation     string  `json:"operation"`
	Value         float64 `json:"value"`
}

type SpecializationBattleContribution struct {
	TreeID          string                       `json:"treeId"`
	RulesetVersion  string                       `json:"rulesetVersion"`
	BalanceRevision int                          `json:"balanceRevision"`
	Revision        int64                        `json:"revision"`
	Digest          string                       `json:"digest"`
	StatModifiers   []SpecializationStatModifier `json:"statModifiers"`
	Hooks           []SpecializationRuntimeGrant `json:"hooks"`
	Mutations       []SpecializationRuntimeGrant `json:"mutations"`
}

type SpecializationRuntimeGrant struct {
	ID            string         `json:"id"`
	SourceNodeID  string         `json:"sourceNodeId"`
	SourceNameKey string         `json:"sourceNameKey"`
	Payload       map[string]any `json:"payload"`
}

type specializationTreeCacheEntry struct {
	tree    specializationTreeSpec
	rawJSON []byte
}

type specializationTreeCache struct {
	ordered          []specializationTreeCacheEntry
	firstEnabledByID map[string]int
}

func buildSpecializationTreeCache(rawTrees []catalog.SpecializationTree) (*specializationTreeCache, error) {
	cache := &specializationTreeCache{
		ordered:          make([]specializationTreeCacheEntry, 0, len(rawTrees)),
		firstEnabledByID: make(map[string]int, len(rawTrees)),
	}
	for index, raw := range rawTrees {
		payload, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: marshal tree index %d: %v", ErrSpecializationDefinitionInvalid, index, err)
		}
		tree, err := specializationTreeFromJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("specialization tree index %d: %w", index, err)
		}
		entryIndex := len(cache.ordered)
		cache.ordered = append(cache.ordered, specializationTreeCacheEntry{
			tree:    tree,
			rawJSON: payload,
		})
		if !tree.Disabled {
			if _, exists := cache.firstEnabledByID[tree.ID]; !exists {
				cache.firstEnabledByID[tree.ID] = entryIndex
			}
		}
	}
	return cache, nil
}

func (s *Store) specializationTrees() (*specializationTreeCache, error) {
	if s == nil {
		return nil, ErrSpecializationDefinitionInvalid
	}
	s.specializationCacheOnce.Do(func() {
		s.specializationCache, s.specializationCacheErr = buildSpecializationTreeCache(s.gameData.SpecializationTrees)
	})
	if s.specializationCacheErr != nil {
		return nil, s.specializationCacheErr
	}
	if s.specializationCache == nil {
		return nil, ErrSpecializationDefinitionInvalid
	}
	return s.specializationCache, nil
}

func (entry *specializationTreeCacheEntry) rawTree() (catalog.SpecializationTree, error) {
	if entry == nil || len(entry.rawJSON) == 0 {
		return catalog.SpecializationTree{}, ErrSpecializationDefinitionInvalid
	}
	var raw catalog.SpecializationTree
	if err := json.Unmarshal(entry.rawJSON, &raw); err != nil {
		return catalog.SpecializationTree{}, fmt.Errorf("%w: decode cached tree: %v", ErrSpecializationDefinitionInvalid, err)
	}
	return raw, nil
}

func specializationTreeFromJSON(payload []byte) (specializationTreeSpec, error) {
	var tree specializationTreeSpec
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tree); err != nil {
		return specializationTreeSpec{}, fmt.Errorf("%w: decode tree: %v", ErrSpecializationDefinitionInvalid, err)
	}
	tree.ID = strings.TrimSpace(tree.ID)
	tree.OwnerType = strings.ToLower(strings.TrimSpace(tree.OwnerType))
	tree.RulesetVersion = strings.TrimSpace(tree.RulesetVersion)
	if tree.ID == "" || tree.SchemaVersion != 2 || tree.RulesetVersion == "" || tree.OwnerType != "player" {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	if tree.RulesetVersion != specializationRulesetV6 || tree.BalanceRevision <= 0 {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	if tree.InitialPoints != 2 || tree.MaxPoints != 65 || tree.MaxActiveRootNodes != 3 || tree.Unlock.BattleLevel != 10 {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	if tree.Reset.Count < 0 {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	seenPointBandIDs := make(map[string]struct{}, len(tree.PointBands))
	previousPointBandEnd := tree.Unlock.BattleLevel
	configuredMaxPoints := tree.InitialPoints
	for index := range tree.PointBands {
		band := &tree.PointBands[index]
		band.ID = strings.TrimSpace(band.ID)
		if band.ID == "" || band.FromLevel <= previousPointBandEnd || band.ToLevel < band.FromLevel || band.ToLevel > 500 || band.EveryLevels <= 0 || (band.ToLevel-band.FromLevel)%band.EveryLevels != 0 || band.PointsPerGrant <= 0 {
			return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
		}
		if _, duplicate := seenPointBandIDs[band.ID]; duplicate {
			return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
		}
		seenPointBandIDs[band.ID] = struct{}{}
		previousPointBandEnd = band.ToLevel
		configuredMaxPoints += (int((band.ToLevel-band.FromLevel)/band.EveryLevels) + 1) * band.PointsPerGrant
	}
	if len(tree.PointBands) > 0 && configuredMaxPoints != tree.MaxPoints {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	if specializationTotalPoints(tree, 200) != 40 || specializationTotalPoints(tree, 300) != 60 || specializationTotalPoints(tree, 500) != 65 {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	seenNodes := make(map[string]struct{}, len(tree.Nodes))
	rootNodeCount := 0
	for idx := range tree.Nodes {
		node := &tree.Nodes[idx]
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" {
			return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
		}
		if _, exists := seenNodes[node.ID]; exists {
			return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
		}
		seenNodes[node.ID] = struct{}{}
		if strings.EqualFold(strings.TrimSpace(node.Kind), "root") {
			rootNodeCount++
		}
		if len(node.Levels) == 0 {
			return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
		}
		previousGrants := map[string]specializationGrantSpec{}
		for levelIdx := range node.Levels {
			level := &node.Levels[levelIdx]
			if level.Level != levelIdx+1 || level.Cost <= 0 || len(level.Grants) == 0 {
				return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
			}
			seenGrants := map[string]specializationGrantSpec{}
			for _, grant := range level.Grants {
				if err := validateSpecializationGrantShape(grant); err != nil {
					return specializationTreeSpec{}, err
				}
				grantID := strings.TrimSpace(grant.ID)
				if _, duplicate := seenGrants[grantID]; duplicate {
					return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
				}
				seenGrants[grantID] = grant
			}
			for grantID, previousGrant := range previousGrants {
				currentGrant, exists := seenGrants[grantID]
				if !exists || validateSpecializationGrantSnapshotRetention(previousGrant, currentGrant) != nil {
					return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
				}
			}
			previousGrants = seenGrants
		}
	}
	if tree.MaxActiveRootNodes > rootNodeCount {
		return specializationTreeSpec{}, ErrSpecializationDefinitionInvalid
	}
	if err := validateSpecializationTreeReferences(tree, seenNodes); err != nil {
		return specializationTreeSpec{}, fmt.Errorf("%w: specialization references", err)
	}
	if err := validateSpecializationV5AuthoringEnvelope(tree); err != nil {
		return specializationTreeSpec{}, fmt.Errorf("%w: specialization v5 authoring envelope", err)
	}
	return tree, nil
}

func validateSpecializationV5AuthoringEnvelope(tree specializationTreeSpec) error {
	for _, node := range tree.Nodes {
		for _, level := range node.Levels {
			for _, grant := range level.Grants {
				for _, mutation := range grant.EffectMutations {
					if err := validateSpecializationV5MutationEnvelope(mutation); err != nil {
						return fmt.Errorf("%w: node %s rank %d grant %s mutation %v", err, node.ID, level.Level, grant.ID, mutation["id"])
					}
				}
				if err := visitSpecializationV5FormulaEnvelope(grant.BattleHooks); err != nil {
					return fmt.Errorf("%w: node %s rank %d grant %s hooks", err, node.ID, level.Level, grant.ID)
				}
				if err := validateSpecializationV5ExtraTurnSafety(grant.BattleHooks); err != nil {
					return fmt.Errorf("%w: node %s rank %d grant %s extra turn safety", err, node.ID, level.Level, grant.ID)
				}
				if err := visitSpecializationV5FormulaEnvelope(grant.EffectMutations); err != nil {
					return fmt.Errorf("%w: node %s rank %d grant %s mutations", err, node.ID, level.Level, grant.ID)
				}
			}
		}
	}
	return nil
}

func normalizeSpecializationDSLKind(raw any) string {
	kind := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
	kind = strings.ReplaceAll(kind, "_", "")
	kind = strings.ReplaceAll(kind, "-", "")
	return kind
}

func validateSpecializationV5ExtraTurnSafety(hooks []map[string]any) error {
	for _, hook := range hooks {
		effects, ok := hook["effects"].([]any)
		if !ok {
			if typed, typedOK := hook["effects"].([]map[string]any); typedOK {
				effects = make([]any, len(typed))
				for index := range typed {
					effects[index] = typed[index]
				}
			} else {
				continue
			}
		}
		directExtraTurn := false
		for _, rawEffect := range effects {
			effect, ok := rawEffect.(map[string]any)
			if !ok {
				return ErrSpecializationDefinitionInvalid
			}
			switch normalizeSpecializationDSLKind(firstSpecializationNonNil(effect["kind"], effect["type"])) {
			case "extraturn", "immediateaction":
				directExtraTurn = true
			case "cyclecounter":
				atMax, _ := firstSpecializationNonNil(effect["atMax"], effect["at_max"]).(map[string]any)
				nestedKind := normalizeSpecializationDSLKind(firstSpecializationNonNil(atMax["kind"], atMax["type"]))
				if nestedKind != "extraturn" && nestedKind != "immediateaction" {
					continue
				}
				maxStacks, maxOK := specializationPositiveInteger(firstSpecializationNonNil(effect["maxStacks"], effect["max_stacks"]))
				consumeStacks, consumeOK := specializationPositiveInteger(firstSpecializationNonNil(atMax["consumeStacks"], atMax["consume_stacks"]))
				if !maxOK || !consumeOK || maxStacks < 3 || consumeStacks < 3 || consumeStacks > maxStacks {
					return ErrSpecializationDefinitionInvalid
				}
			}
		}
		if !directExtraTurn {
			continue
		}
		if specializationHookHasOuterLimit(hook, "perWave", 1) || specializationHookHasOuterLimit(hook, "perBattle", 1) {
			continue
		}
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

func specializationPositiveInteger(raw any) (int, bool) {
	value, ok := numericFromAny(raw)
	if !ok || value <= 0 || value != math.Trunc(value) || value > float64(math.MaxInt) {
		return 0, false
	}
	return int(value), true
}

func specializationHookHasOuterLimit(hook map[string]any, key string, maximum int) bool {
	hasBoundedLimit := func(raw any) bool {
		value, ok := specializationPositiveInteger(raw)
		return ok && value <= maximum
	}
	if limits, ok := hook["limits"].(map[string]any); ok && hasBoundedLimit(limits[key]) {
		return true
	}
	rawConditions, _ := hook["conditions"].([]any)
	for _, rawCondition := range rawConditions {
		condition, ok := rawCondition.(map[string]any)
		if ok && normalizeSpecializationDSLKind(condition["kind"]) == "triggerlimit" && hasBoundedLimit(condition[key]) {
			return true
		}
	}
	return false
}

func validateSpecializationV5MutationEnvelope(mutation map[string]any) error {
	ops, _ := mutation["ops"].(map[string]any)
	if len(ops) == 0 {
		return nil
	}
	rawTerms, hasTerms := ops["amountTermsAppend"]
	if !hasTerms {
		rawTerms, hasTerms = ops["amountFormulaTermsAppend"]
	}
	if !hasTerms {
		return nil
	}
	terms, ok := rawTerms.([]any)
	if !ok {
		if typed, typedOK := rawTerms.([]map[string]any); typedOK {
			terms = make([]any, len(typed))
			for index := range typed {
				terms[index] = typed[index]
			}
		} else {
			return ErrSpecializationDefinitionInvalid
		}
	}
	if len(terms) == 0 || len(terms) > 2 {
		return ErrSpecializationDefinitionInvalid
	}
	scope := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", mutation["scope"])))
	selector, _ := mutation["effectSelector"].(map[string]any)
	selectorMode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", selector["mode"])))
	hasSkillFilter := specializationAnySliceLen(mutation["skillIds"]) > 0 ||
		specializationAnySliceLen(mutation["skillCategories"]) > 0 ||
		specializationAnySliceLen(mutation["weaponClasses"]) > 0 ||
		specializationAnySliceLen(mutation["damageTypes"]) > 0
	wide := scope == "both" || (scope == "skill" && !hasSkillFilter && (selectorMode == "all" || selectorMode == "kind"))
	perTermLimit, sumLimit, coverageLimit := 0.05, 0.08, 0.22
	if wide {
		perTermLimit, sumLimit, coverageLimit = 0.04, 0.05, 0.12
	}
	sum := 0.0
	for _, rawTerm := range terms {
		term, ok := rawTerm.(map[string]any)
		if !ok {
			return ErrSpecializationDefinitionInvalid
		}
		coefficient, ok := numericFromAny(term["coefficient"])
		if !ok || coefficient <= 0 || coefficient > perTermLimit+1e-9 {
			return ErrSpecializationDefinitionInvalid
		}
		sum += coefficient
	}
	if sum > sumLimit+1e-9 || sum*500 > sumLimit*500+1e-9 {
		return ErrSpecializationDefinitionInvalid
	}
	hitDelta, hitOK := specializationOptionalInteger(ops, "hitCountDelta")
	targetDelta, targetOK := specializationOptionalInteger(ops, "targetCountDelta")
	if !hitOK || !targetOK || hitDelta < 0 || hitDelta > 1 || targetDelta < 0 || targetDelta > 1 || (hitDelta > 0 && targetDelta > 0) {
		return ErrSpecializationDefinitionInvalid
	}
	multiplier := 1.0
	if rawMultiplier, exists := ops["amountMultiplier"]; exists {
		parsed, ok := numericFromAny(rawMultiplier)
		if !ok || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return ErrSpecializationDefinitionInvalid
		}
		multiplier = parsed
	}
	if (hitDelta > 0 || targetDelta > 0) && (multiplier < 0.5 || multiplier > 0.8) {
		return ErrSpecializationDefinitionInvalid
	}
	coverage := sum / 0.55 * float64(1+hitDelta) * float64(1+targetDelta) * multiplier
	if coverage > coverageLimit+1e-9 {
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

func specializationAnySliceLen(raw any) int {
	switch typed := raw.(type) {
	case []any:
		return len(typed)
	case []string:
		return len(typed)
	default:
		return 0
	}
}

func specializationOptionalInteger(values map[string]any, key string) (int, bool) {
	raw, exists := values[key]
	if !exists {
		return 0, true
	}
	value, ok := numericFromAny(raw)
	if !ok || math.Trunc(value) != value || value < float64(math.MinInt) || value > float64(math.MaxInt) {
		return 0, false
	}
	return int(value), true
}

func visitSpecializationV5FormulaEnvelope(raw any) error {
	switch typed := raw.(type) {
	case []map[string]any:
		for _, child := range typed {
			if err := visitSpecializationV5FormulaEnvelope(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := visitSpecializationV5FormulaEnvelope(child); err != nil {
				return err
			}
		}
	case map[string]any:
		kind := normalizeSpecializationDSLKind(firstSpecializationNonNil(typed["kind"], typed["type"]))
		if kind == "actionmetergain" {
			return ErrSpecializationDefinitionInvalid
		}
		if rawFormula, exists := typed["valueFormula"]; exists {
			formula, ok := rawFormula.(map[string]any)
			if !ok {
				return ErrSpecializationDefinitionInvalid
			}
			minimum, maximum, err := specializationFormulaCornerEnvelope(formula)
			if err != nil {
				return err
			}
			limits := map[string]float64{
				"mirrorcastskill": .75, "repeatcontexteffect": .75,
				"empowernexthit": .45, "shareddamagereduction": .45,
				"consumecounterfordamageamp": .8, "runtimedamageamp": .8,
				"redirectdamage": .65, "resourceshield": .65,
				"heal": .5, "restoremp": .5, "restorempandtickcooldown": .5, "guardlowesthpally": .5,
				"preventlethal": .45,
			}
			if kind == "resourcedebtcast" {
				if minimum < .05-1e-9 || maximum > .20+1e-9 {
					return ErrSpecializationDefinitionInvalid
				}
			} else if limit, constrained := limits[kind]; constrained && (minimum <= 0 || maximum > limit+1e-9) {
				return ErrSpecializationDefinitionInvalid
			}
		}
		for _, child := range typed {
			if err := visitSpecializationV5FormulaEnvelope(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func specializationFormulaCornerEnvelope(formula map[string]any) (float64, float64, error) {
	base, ok := numericFromAny(formula["baseAdd"])
	if !ok || math.IsNaN(base) || math.IsInf(base, 0) {
		return 0, 0, ErrSpecializationDefinitionInvalid
	}
	minimum, maximum := base, base
	rawTerms := []any{}
	if raw, exists := formula["terms"]; exists {
		var ok bool
		rawTerms, ok = raw.([]any)
		if !ok {
			return 0, 0, ErrSpecializationDefinitionInvalid
		}
	}
	for _, rawTerm := range rawTerms {
		term, ok := rawTerm.(map[string]any)
		if !ok || strings.TrimSpace(fmt.Sprintf("%v", term["attributeKey"])) == "" {
			return 0, 0, ErrSpecializationDefinitionInvalid
		}
		coefficient, ok := numericFromAny(term["coefficient"])
		if !ok || coefficient == 0 || math.IsNaN(coefficient) || math.IsInf(coefficient, 0) {
			return 0, 0, ErrSpecializationDefinitionInvalid
		}
		projection := coefficient * 500
		minimum += math.Min(0, projection)
		maximum += math.Max(0, projection)
	}
	return minimum, maximum, nil
}

func firstSpecializationNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func validateSpecializationTreeReferences(tree specializationTreeSpec, seenNodes map[string]struct{}) error {
	knownSchools := make(map[string]struct{}, len(tree.Schools))
	for _, school := range tree.Schools {
		schoolID := strings.TrimSpace(school.ID)
		if schoolID == "" {
			return ErrSpecializationDefinitionInvalid
		}
		if _, duplicate := knownSchools[schoolID]; duplicate {
			return ErrSpecializationDefinitionInvalid
		}
		knownSchools[schoolID] = struct{}{}
	}
	knownKeywords := make(map[string]struct{}, len(tree.Keywords))
	for _, keyword := range tree.Keywords {
		keywordID := strings.TrimSpace(keyword.ID)
		if keywordID == "" || strings.TrimSpace(keyword.MechanicKind) == "" || len(keyword.RuntimeIDs) == 0 || len(keyword.SourceNodeIDs) == 0 || strings.TrimSpace(keyword.NameKey) == "" || strings.TrimSpace(keyword.SummaryKey) == "" || strings.TrimSpace(keyword.MechanicDetailsKey) == "" || strings.TrimSpace(keyword.SourceHintKey) == "" {
			return ErrSpecializationDefinitionInvalid
		}
		if _, duplicate := knownKeywords[keywordID]; duplicate {
			return ErrSpecializationDefinitionInvalid
		}
		knownKeywords[keywordID] = struct{}{}
		if len(uniqueTrimmedStrings(keyword.RuntimeIDs)) != len(keyword.RuntimeIDs) || len(uniqueTrimmedStrings(keyword.SourceNodeIDs)) != len(keyword.SourceNodeIDs) {
			return ErrSpecializationDefinitionInvalid
		}
	}
	if len(knownKeywords) == 0 {
		return ErrSpecializationDefinitionInvalid
	}
	for _, locale := range tree.I18n {
		for keywordID := range locale.Keywords {
			if _, ok := knownKeywords[strings.TrimSpace(keywordID)]; !ok {
				return ErrSpecializationDefinitionInvalid
			}
		}
		for keywordID := range knownKeywords {
			copy, ok := locale.Keywords[keywordID]
			if !ok || strings.TrimSpace(copy.Name) == "" || strings.TrimSpace(copy.PlayerSummary) == "" || strings.TrimSpace(copy.MechanicDetails) == "" || strings.TrimSpace(copy.SourceHint) == "" {
				return ErrSpecializationDefinitionInvalid
			}
		}
	}

	for _, node := range tree.Nodes {
		kind := strings.ToLower(strings.TrimSpace(node.Kind))
		if kind != "root" && kind != "technique" && kind != "terminal" && kind != "confluence" {
			return ErrSpecializationDefinitionInvalid
		}
		seenSchoolIDs := map[string]struct{}{}
		for _, rawSchoolID := range node.SchoolIDs {
			schoolID := strings.TrimSpace(rawSchoolID)
			if _, ok := knownSchools[schoolID]; schoolID == "" || !ok {
				return ErrSpecializationDefinitionInvalid
			}
			if _, duplicate := seenSchoolIDs[schoolID]; duplicate {
				return ErrSpecializationDefinitionInvalid
			}
			seenSchoolIDs[schoolID] = struct{}{}
		}
		seenKeywordIDs := map[string]struct{}{}
		for _, rawKeywordID := range node.KeywordIDs {
			keywordID := strings.TrimSpace(rawKeywordID)
			if _, ok := knownKeywords[keywordID]; keywordID == "" || !ok {
				return ErrSpecializationDefinitionInvalid
			}
			if _, duplicate := seenKeywordIDs[keywordID]; duplicate {
				return ErrSpecializationDefinitionInvalid
			}
			seenKeywordIDs[keywordID] = struct{}{}
		}
		for _, requirement := range node.Requirements.All {
			if err := validateSpecializationRequirementReference(tree, node.ID, requirement, seenNodes, knownSchools); err != nil {
				return err
			}
		}
		for _, group := range node.Requirements.AnyOf {
			if len(group) == 0 {
				return ErrSpecializationDefinitionInvalid
			}
			for _, requirement := range group {
				if err := validateSpecializationRequirementReference(tree, node.ID, requirement, seenNodes, knownSchools); err != nil {
					return err
				}
			}
		}
	}
	for _, keyword := range tree.Keywords {
		for _, sourceNodeID := range keyword.SourceNodeIDs {
			if _, ok := seenNodes[strings.TrimSpace(sourceNodeID)]; !ok {
				return ErrSpecializationDefinitionInvalid
			}
		}
	}
	if err := validateSpecializationRequirementGraph(tree); err != nil {
		return err
	}

	return nil
}

func validateSpecializationRequirementReference(tree specializationTreeSpec, ownerNodeID string, requirement specializationRequirementSpec, seenNodes, knownSchools map[string]struct{}) error {
	switch strings.ToLower(strings.TrimSpace(requirement.Kind)) {
	case "node_rank":
		nodeID := strings.TrimSpace(requirement.NodeID)
		if nodeID == "" || nodeID == ownerNodeID || requirement.MinRank <= 0 {
			return ErrSpecializationDefinitionInvalid
		}
		if _, ok := seenNodes[nodeID]; !ok {
			return ErrSpecializationDefinitionInvalid
		}
		parent, _ := specializationTreeNode(tree, nodeID)
		if requirement.MinRank > len(parent.Levels) {
			return ErrSpecializationDefinitionInvalid
		}
	case "attribute_gte":
		if strings.TrimSpace(requirement.AttributeKey) == "" || math.IsNaN(requirement.Value) || math.IsInf(requirement.Value, 0) || requirement.Value < 0 {
			return ErrSpecializationDefinitionInvalid
		}
	case "pure_school_points_gte", "school_points_gte":
		schoolID := strings.TrimSpace(requirement.SchoolID)
		if requirement.Points <= 0 {
			return ErrSpecializationDefinitionInvalid
		}
		if _, ok := knownSchools[schoolID]; schoolID == "" || !ok {
			return ErrSpecializationDefinitionInvalid
		}
	default:
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

func validateSpecializationRequirementGraph(tree specializationTreeSpec) error {
	parents := make(map[string][]string, len(tree.Nodes))
	for _, node := range tree.Nodes {
		for _, requirement := range node.Requirements.All {
			if strings.EqualFold(strings.TrimSpace(requirement.Kind), "node_rank") {
				parents[node.ID] = append(parents[node.ID], strings.TrimSpace(requirement.NodeID))
			}
		}
		for _, group := range node.Requirements.AnyOf {
			for _, requirement := range group {
				if strings.EqualFold(strings.TrimSpace(requirement.Kind), "node_rank") {
					parents[node.ID] = append(parents[node.ID], strings.TrimSpace(requirement.NodeID))
				}
			}
		}
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(nodeID string) error {
		switch state[nodeID] {
		case 1:
			return ErrSpecializationDefinitionInvalid
		case 2:
			return nil
		}
		state[nodeID] = 1
		for _, parentID := range parents[nodeID] {
			if err := visit(parentID); err != nil {
				return err
			}
		}
		state[nodeID] = 2
		return nil
	}
	for _, node := range tree.Nodes {
		if err := visit(node.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) findSpecializationTree(treeID string) (*specializationTreeCacheEntry, error) {
	treeID = strings.TrimSpace(treeID)
	cache, err := s.specializationTrees()
	if err != nil {
		return nil, err
	}
	index, exists := cache.firstEnabledByID[treeID]
	if !exists || index < 0 || index >= len(cache.ordered) {
		return nil, ErrSpecializationTreeNotFound
	}
	return &cache.ordered[index], nil
}

func specializationTreeNode(tree specializationTreeSpec, nodeID string) (specializationNodeSpec, bool) {
	nodeID = strings.TrimSpace(nodeID)
	for _, node := range tree.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return specializationNodeSpec{}, false
}

func specializationTotalPoints(tree specializationTreeSpec, battleLevel int64) int {
	if battleLevel < tree.Unlock.BattleLevel {
		return 0
	}
	total := tree.InitialPoints
	for _, band := range tree.PointBands {
		if band.FromLevel <= 0 || band.EveryLevels <= 0 || band.PointsPerGrant <= 0 || battleLevel < band.FromLevel {
			continue
		}
		upper := battleLevel
		if band.ToLevel > 0 && upper > band.ToLevel {
			upper = band.ToLevel
		}
		if upper < band.FromLevel {
			continue
		}
		grants := int((upper-band.FromLevel)/band.EveryLevels) + 1
		total += grants * band.PointsPerGrant
	}
	if total > tree.MaxPoints {
		total = tree.MaxPoints
	}
	if total < 0 {
		return 0
	}
	return total
}

func specializationPointBreakdown(tree specializationTreeSpec, battleLevel int64, bonusPoints int) SpecializationPointBreakdown {
	bonusPoints = maxInt(0, bonusPoints)
	formulaSources := make([]SpecializationPointSource, 0, len(tree.PointBands)+2)
	milestoneBands := make([]SpecializationPointMilestoneBand, 0, len(tree.PointBands))
	milestoneBandIndex := make(map[string]int, len(tree.PointBands))
	unlockLevel := tree.Unlock.BattleLevel
	formulaSources = append(formulaSources, SpecializationPointSource{
		SourceID:            "unlock",
		Kind:                specializationPointSourceUnlockGrant,
		LabelKey:            "game.specialization.pointSource.unlockGrant",
		RequiredBattleLevel: &unlockLevel,
		Points:              tree.InitialPoints,
		Earned:              battleLevel >= unlockLevel,
	})
	for _, band := range tree.PointBands {
		if band.FromLevel <= 0 || band.ToLevel < band.FromLevel || band.EveryLevels <= 0 || band.PointsPerGrant <= 0 {
			continue
		}
		milestoneBandIndex[band.ID] = len(milestoneBands)
		milestoneBands = append(milestoneBands, SpecializationPointMilestoneBand{
			BandID:          band.ID,
			LabelKey:        "game.specialization.pointSource.levelMilestone",
			FromBattleLevel: band.FromLevel,
			ToBattleLevel:   band.ToLevel,
			EveryLevels:     band.EveryLevels,
			PointsPerGrant:  band.PointsPerGrant,
		})
		for level := band.FromLevel; level <= band.ToLevel; level += band.EveryLevels {
			requiredLevel := level
			formulaSources = append(formulaSources, SpecializationPointSource{
				SourceID:            fmt.Sprintf("%s:%d", band.ID, level),
				Kind:                specializationPointSourceLevelMilestone,
				LabelKey:            "game.specialization.pointSource.levelMilestone",
				BandID:              band.ID,
				RequiredBattleLevel: &requiredLevel,
				Points:              band.PointsPerGrant,
				Earned:              battleLevel >= level,
			})
		}
	}
	sort.SliceStable(formulaSources, func(i, j int) bool {
		left := formulaSources[i].RequiredBattleLevel
		right := formulaSources[j].RequiredBattleLevel
		if left != nil && right != nil && *left != *right {
			return *left < *right
		}
		return formulaSources[i].SourceID < formulaSources[j].SourceID
	})

	breakdown := SpecializationPointBreakdown{
		RewardPoints:   bonusPoints,
		MaxPoints:      tree.MaxPoints,
		MilestoneBands: milestoneBands,
		Sources:        make([]SpecializationPointSource, 0, len(formulaSources)+1),
	}
	remaining := maxInt(0, tree.MaxPoints)
	for index := range formulaSources {
		source := &formulaSources[index]
		if source.Earned {
			counted := min(source.Points, remaining)
			source.CountedPoints = counted
			remaining -= counted
			switch source.Kind {
			case specializationPointSourceUnlockGrant:
				breakdown.UnlockGrantPoints += source.Points
			case specializationPointSourceLevelMilestone:
				breakdown.LevelMilestonePoints += source.Points
				if bandIndex, ok := milestoneBandIndex[source.BandID]; ok {
					band := &breakdown.MilestoneBands[bandIndex]
					band.EarnedGrantCount++
					band.EarnedPoints += source.Points
					band.CountedPoints += source.CountedPoints
					if source.RequiredBattleLevel != nil {
						earnedLevel := *source.RequiredBattleLevel
						if band.FirstEarnedBattleLevel == nil {
							band.FirstEarnedBattleLevel = &earnedLevel
						}
						band.LastEarnedBattleLevel = &earnedLevel
					}
				}
			}
		}
		breakdown.Sources = append(breakdown.Sources, *source)
	}
	countedRewardPoints := min(bonusPoints, remaining)
	breakdown.CountedRewardPoints = countedRewardPoints
	breakdown.Sources = append(breakdown.Sources, SpecializationPointSource{
		SourceID:      "reward",
		Kind:          specializationPointSourceReward,
		LabelKey:      "game.specialization.pointSource.reward",
		Points:        bonusPoints,
		CountedPoints: countedRewardPoints,
		Earned:        bonusPoints > 0,
	})
	rawPoints := breakdown.UnlockGrantPoints + breakdown.LevelMilestonePoints + bonusPoints
	totalPoints := min(rawPoints, maxInt(0, tree.MaxPoints))
	breakdown.CappedPoints = maxInt(0, rawPoints-totalPoints)
	breakdown.AtCap = tree.MaxPoints > 0 && totalPoints >= tree.MaxPoints
	if !breakdown.AtCap {
		remainingCapacity := maxInt(0, tree.MaxPoints-totalPoints)
		for _, source := range formulaSources {
			if !source.Earned && source.Kind == specializationPointSourceLevelMilestone && source.RequiredBattleLevel != nil {
				if bandIndex, ok := milestoneBandIndex[source.BandID]; ok && breakdown.MilestoneBands[bandIndex].NextMilestoneBattleLevel == nil {
					nextMilestoneLevel := *source.RequiredBattleLevel
					breakdown.MilestoneBands[bandIndex].NextMilestoneBattleLevel = &nextMilestoneLevel
				}
			}
			if source.Earned || source.RequiredBattleLevel == nil || source.Points <= 0 {
				continue
			}
			nextLevel := *source.RequiredBattleLevel
			breakdown.NextPointBattleLevel = &nextLevel
			breakdown.NextPointPoints = min(source.Points, remainingCapacity)
			break
		}
	}
	return breakdown
}

func (s *Store) specializationIntrinsicAttributesTx(ctx context.Context, tx pgx.Tx, userID int64) (int64, map[string]float64, error) {
	if err := s.ensureAllPlayerAttributesTx(ctx, tx, userID); err != nil {
		return 0, nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT attr_key, base_level, skill_point_level
		FROM player_attributes
		WHERE user_id = $1
		FOR UPDATE
	`, userID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	baseDefaults := make(map[string]float64, len(s.gameData.Attributes))
	for _, definition := range s.gameData.Attributes {
		baseDefaults[strings.TrimSpace(definition.ID)] = definition.BaseDefault
	}
	attributes := make(map[string]float64, len(baseDefaults))
	var battleLevel int64 = 1
	for rows.Next() {
		var key string
		var baseLevel, skillPointLevel int64
		if err := rows.Scan(&key, &baseLevel, &skillPointLevel); err != nil {
			return 0, nil, err
		}
		key = strings.TrimSpace(key)
		value := baseDefaults[key] + float64(maxInt64(0, baseLevel-1)) + float64(maxInt64(0, skillPointLevel))
		attributes[key] = roundStatValue(value)
		if key == "battle" {
			battleLevel = baseLevel
		}
	}
	return battleLevel, attributes, rows.Err()
}

func specializationCharacterAttributesTx(ctx context.Context, tx pgx.Tx, userID int64) (map[string]float64, error) {
	rows, err := tx.Query(ctx, `
		SELECT attr_key, final_value
		FROM player_attributes
		WHERE user_id = $1
		ORDER BY attr_key ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var key string
		var value float64
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		key = strings.TrimSpace(key)
		if key != "" {
			out[key] = roundStatValue(value)
		}
	}
	return out, rows.Err()
}

func loadSpecializationAllocationsFrom(ctx context.Context, queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, userID int64, treeID string) ([]SpecializationAllocation, error) {
	rows, err := queryer.Query(ctx, `
		SELECT node_id, rank, paid_cost
		FROM player_specialization_allocations
		WHERE user_id = $1 AND tree_id = $2
		ORDER BY node_id ASC
	`, userID, treeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SpecializationAllocation, 0, 16)
	for rows.Next() {
		var row SpecializationAllocation
		if err := rows.Scan(&row.NodeID, &row.Rank, &row.PaidCost); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func allocationRankMap(allocations []SpecializationAllocation) map[string]int {
	out := make(map[string]int, len(allocations))
	for _, allocation := range allocations {
		out[strings.TrimSpace(allocation.NodeID)] = allocation.Rank
	}
	return out
}

func specializationActiveRootCount(tree specializationTreeSpec, allocations []SpecializationAllocation) int {
	count := 0
	for _, allocation := range allocations {
		if allocation.Rank <= 0 {
			continue
		}
		node, ok := specializationTreeNode(tree, allocation.NodeID)
		if ok && strings.EqualFold(strings.TrimSpace(node.Kind), "root") {
			count++
		}
	}
	return count
}

func specializationSpentPoints(allocations []SpecializationAllocation) int {
	total := 0
	for _, allocation := range allocations {
		if allocation.PaidCost > 0 {
			total += allocation.PaidCost
		}
	}
	return total
}

func specializationPureSchoolPoints(tree specializationTreeSpec, allocations []SpecializationAllocation) map[string]int {
	out := map[string]int{}
	for _, allocation := range allocations {
		node, ok := specializationTreeNode(tree, allocation.NodeID)
		if !ok || len(node.SchoolIDs) != 1 {
			continue
		}
		out[strings.TrimSpace(node.SchoolIDs[0])] += allocation.PaidCost
	}
	return out
}

func evaluateSpecializationRequirement(requirement specializationRequirementSpec, ranks map[string]int, intrinsic map[string]float64, schoolPoints map[string]int) bool {
	switch strings.ToLower(strings.TrimSpace(requirement.Kind)) {
	case "node_rank":
		minimum := requirement.MinRank
		if minimum <= 0 {
			minimum = 1
		}
		return ranks[strings.TrimSpace(requirement.NodeID)] >= minimum
	case "attribute_gte":
		return intrinsic[strings.TrimSpace(requirement.AttributeKey)] >= requirement.Value
	case "pure_school_points_gte", "school_points_gte":
		return schoolPoints[strings.TrimSpace(requirement.SchoolID)] >= requirement.Points
	default:
		return false
	}
}

func specializationNodeUnmetReasons(tree specializationTreeSpec, node specializationNodeSpec, allocations []SpecializationAllocation, intrinsic map[string]float64, totalPoints int) []string {
	ranks := allocationRankMap(allocations)
	schoolPoints := specializationPureSchoolPoints(tree, allocations)
	reasons := make([]string, 0, 4)
	currentRank := ranks[node.ID]
	if currentRank >= len(node.Levels) {
		return []string{"MAX_RANK"}
	}
	if currentRank == 0 && tree.MaxActiveRootNodes > 0 && strings.EqualFold(strings.TrimSpace(node.Kind), "root") && specializationActiveRootCount(tree, allocations) >= tree.MaxActiveRootNodes {
		reasons = append(reasons, "ACTIVE_ROOT_LIMIT")
	}
	for _, requirement := range node.Requirements.All {
		if !evaluateSpecializationRequirement(requirement, ranks, intrinsic, schoolPoints) {
			reasons = append(reasons, "REQUIREMENT_"+strings.ToUpper(strings.TrimSpace(requirement.Kind)))
		}
	}
	if len(node.Requirements.AnyOf) > 0 {
		allGroupsMatched := true
		for _, group := range node.Requirements.AnyOf {
			groupMatched := false
			for _, requirement := range group {
				if evaluateSpecializationRequirement(requirement, ranks, intrinsic, schoolPoints) {
					groupMatched = true
					break
				}
			}
			if !groupMatched {
				allGroupsMatched = false
				break
			}
		}
		if !allGroupsMatched {
			reasons = append(reasons, "REQUIREMENT_ANY_OF")
		}
	}
	if group := strings.TrimSpace(node.ExclusiveGroupID); group != "" {
		for _, allocation := range allocations {
			if allocation.Rank <= 0 || allocation.NodeID == node.ID {
				continue
			}
			other, ok := specializationTreeNode(tree, allocation.NodeID)
			if ok && strings.TrimSpace(other.ExclusiveGroupID) == group {
				reasons = append(reasons, "EXCLUSIVE_GROUP")
				break
			}
		}
	}
	nextCost := node.Levels[currentRank].Cost
	if totalPoints-specializationSpentPoints(allocations) < nextCost {
		reasons = append(reasons, "INSUFFICIENT_POINTS")
	}
	return uniqueTrimmedStrings(reasons)
}

func buildSpecializationStatus(tree specializationTreeSpec, revision int64, battleLevel int64, bonusPoints int, intrinsic map[string]float64, allocations []SpecializationAllocation) SpecializationStatus {
	formulaPoints := specializationTotalPoints(tree, battleLevel)
	bonusPoints = maxInt(0, bonusPoints)
	totalPoints := formulaPoints + bonusPoints
	if totalPoints > tree.MaxPoints {
		totalPoints = tree.MaxPoints
	}
	spentPoints := specializationSpentPoints(allocations)
	status := SpecializationStatus{
		TreeID:          tree.ID,
		RulesetVersion:  tree.RulesetVersion,
		BalanceRevision: tree.BalanceRevision,
		Revision:        revision,
		BattleLevel:     battleLevel,
		FormulaPoints:   formulaPoints,
		BonusPoints:     bonusPoints,
		TotalPoints:     totalPoints,
		SpentPoints:     spentPoints,
		AvailablePoints: maxInt(0, totalPoints-spentPoints),
		PointBreakdown:  specializationPointBreakdown(tree, battleLevel, bonusPoints),
		Allocations:     append([]SpecializationAllocation(nil), allocations...),
		ResetCost:       tree.Reset,
		NodeStates:      make([]SpecializationNodeState, 0, len(tree.Nodes)),
	}
	for _, node := range tree.Nodes {
		rank := allocationRankMap(allocations)[node.ID]
		nextCost := 0
		if rank < len(node.Levels) {
			nextCost = node.Levels[rank].Cost
		}
		reasons := specializationNodeUnmetReasons(tree, node, allocations, intrinsic, totalPoints)
		if battleLevel < tree.Unlock.BattleLevel {
			reasons = append([]string{"TREE_LOCKED"}, reasons...)
			reasons = uniqueTrimmedStrings(reasons)
		}
		status.NodeStates = append(status.NodeStates, SpecializationNodeState{
			NodeID:           node.ID,
			Rank:             rank,
			MaxRank:          len(node.Levels),
			NextCost:         nextCost,
			CanAllocate:      len(reasons) == 0,
			UnmetReasonCodes: reasons,
		})
	}
	return status
}

type specializationStateAccounting struct {
	FormulaPoints int
	BonusPoints   int
	SpentPoints   int
}

func (s *Store) reconcileSpecializationStateAccountingTx(ctx context.Context, tx pgx.Tx, userID int64, tree specializationTreeSpec, battleLevel int64, allocations []SpecializationAllocation) (specializationStateAccounting, error) {
	formulaPoints := specializationTotalPoints(tree, battleLevel)
	spentPoints := specializationSpentPoints(allocations)
	var bonusPoints int
	if err := tx.QueryRow(ctx, `
		UPDATE player_specialization_states
		SET formula_points = $3,
		    spent_points = $4,
		    updated_at = now()
		WHERE user_id = $1 AND tree_id = $2
		RETURNING bonus_points
	`, userID, tree.ID, formulaPoints, spentPoints).Scan(&bonusPoints); err != nil {
		return specializationStateAccounting{}, err
	}
	if bonusPoints < 0 {
		return specializationStateAccounting{}, ErrSpecializationDefinitionInvalid
	}
	return specializationStateAccounting{
		FormulaPoints: formulaPoints,
		BonusPoints:   bonusPoints,
		SpentPoints:   spentPoints,
	}, nil
}

func specializationCapacityDeltas(tree specializationTreeSpec, allocations []SpecializationAllocation) (map[string]int, error) {
	deltas := map[string]int{}
	for _, allocation := range allocations {
		node, ok := specializationTreeNode(tree, allocation.NodeID)
		if !ok || allocation.Rank <= 0 || allocation.Rank > len(node.Levels) {
			return nil, ErrSpecializationDefinitionInvalid
		}
		// Each level is a complete snapshot of the node's grants. Applying every
		// preceding level would multiply effects when rank 2 is learned.
		for _, grant := range node.Levels[allocation.Rank-1].Grants {
			if strings.ToLower(strings.TrimSpace(grant.Kind)) != "equipment_slot_capacity" {
				continue
			}
			if err := validateSpecializationGrantShape(grant); err != nil {
				return nil, err
			}
			for family, delta := range grant.Deltas {
				deltas[strings.TrimSpace(family)] += delta
			}
		}
	}
	return deltas, nil
}

func specializationStablePayloadIDs(payloads []map[string]any) (map[string]struct{}, error) {
	ids := make(map[string]struct{}, len(payloads))
	for _, payload := range payloads {
		id, ok := payload["id"].(string)
		if !ok || !IsValidSpecializationRuntimeIDPart(id) {
			return nil, ErrSpecializationDefinitionInvalid
		}
		if _, duplicate := ids[id]; duplicate {
			return nil, ErrSpecializationDefinitionInvalid
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func validateSpecializationGrantSnapshotRetention(previous, current specializationGrantSpec) error {
	kind := strings.ToLower(strings.TrimSpace(previous.Kind))
	if kind == "" || kind != strings.ToLower(strings.TrimSpace(current.Kind)) {
		return ErrSpecializationDefinitionInvalid
	}
	switch kind {
	case "stat_modifier":
		if strings.ToLower(strings.TrimSpace(previous.Scope)) != strings.ToLower(strings.TrimSpace(current.Scope)) {
			return ErrSpecializationDefinitionInvalid
		}
		currentSignatures := make(map[string]int, len(current.Modifiers))
		for _, modifier := range current.Modifiers {
			signature := strings.TrimSpace(modifier.AttributeKey) + "\x00" + strings.ToLower(strings.TrimSpace(modifier.Operation))
			currentSignatures[signature]++
		}
		for _, modifier := range previous.Modifiers {
			signature := strings.TrimSpace(modifier.AttributeKey) + "\x00" + strings.ToLower(strings.TrimSpace(modifier.Operation))
			if currentSignatures[signature] <= 0 {
				return ErrSpecializationDefinitionInvalid
			}
			currentSignatures[signature]--
		}
	case "battle_hook":
		previousIDs, err := specializationStablePayloadIDs(previous.BattleHooks)
		if err != nil {
			return err
		}
		currentIDs, err := specializationStablePayloadIDs(current.BattleHooks)
		if err != nil {
			return err
		}
		for id := range previousIDs {
			if _, retained := currentIDs[id]; !retained {
				return ErrSpecializationDefinitionInvalid
			}
		}
	case "effect_mutation":
		previousIDs, err := specializationStablePayloadIDs(previous.EffectMutations)
		if err != nil {
			return err
		}
		currentIDs, err := specializationStablePayloadIDs(current.EffectMutations)
		if err != nil {
			return err
		}
		for id := range previousIDs {
			if _, retained := currentIDs[id]; !retained {
				return ErrSpecializationDefinitionInvalid
			}
		}
	case "equipment_slot_capacity":
		currentFamilies := make(map[string]struct{}, len(current.Deltas))
		for family := range current.Deltas {
			currentFamilies[strings.TrimSpace(family)] = struct{}{}
		}
		for family := range previous.Deltas {
			if _, retained := currentFamilies[strings.TrimSpace(family)]; !retained {
				return ErrSpecializationDefinitionInvalid
			}
		}
	default:
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

func validateSpecializationGrantShape(grant specializationGrantSpec) error {
	if !IsValidSpecializationRuntimeIDPart(grant.ID) {
		return ErrSpecializationDefinitionInvalid
	}
	switch strings.ToLower(strings.TrimSpace(grant.Kind)) {
	case "stat_modifier":
		scope := strings.ToLower(strings.TrimSpace(grant.Scope))
		if (scope != "character" && scope != "battle") || len(grant.Modifiers) == 0 {
			return ErrSpecializationDefinitionInvalid
		}
		for _, modifier := range grant.Modifiers {
			operation := strings.ToLower(strings.TrimSpace(modifier.Operation))
			if strings.TrimSpace(modifier.AttributeKey) == "" || (operation != "add" && operation != "multiply") || math.IsNaN(modifier.Value) || math.IsInf(modifier.Value, 0) {
				return ErrSpecializationDefinitionInvalid
			}
		}
	case "battle_hook":
		if len(grant.BattleHooks) == 0 {
			return ErrSpecializationDefinitionInvalid
		}
		if _, err := specializationStablePayloadIDs(grant.BattleHooks); err != nil {
			return err
		}
	case "effect_mutation":
		if len(grant.EffectMutations) == 0 {
			return ErrSpecializationDefinitionInvalid
		}
		if _, err := specializationStablePayloadIDs(grant.EffectMutations); err != nil {
			return err
		}
	case "equipment_slot_capacity":
		if normalizeEquipType(grant.EquipType) != "battle" || len(grant.Deltas) == 0 {
			return ErrSpecializationDefinitionInvalid
		}
		knownFamilies := map[string]struct{}{}
		for _, rule := range playerBattleSlotFamilyRules {
			knownFamilies[rule.Family] = struct{}{}
		}
		for rawFamily, delta := range grant.Deltas {
			family := strings.TrimSpace(rawFamily)
			if family == "" || delta == 0 {
				return ErrSpecializationDefinitionInvalid
			}
			if _, ok := knownFamilies[family]; !ok {
				return ErrSpecializationDefinitionInvalid
			}
		}
	default:
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

func (s *Store) resolvePlayerBattleSlotProfileForAllocations(tree specializationTreeSpec, allocations []SpecializationAllocation) (EquipmentSlotProfile, error) {
	deltas, err := specializationCapacityDeltas(tree, allocations)
	if err != nil {
		return EquipmentSlotProfile{}, err
	}
	return ResolvePlayerBattleSlotProfile(deltas)
}

func (s *Store) resolvePlayerBattleSlotProfileTx(ctx context.Context, tx pgx.Tx, userID int64) (EquipmentSlotProfile, error) {
	profile := BasePlayerBattleSlotProfile()
	cache, err := s.specializationTrees()
	if err != nil {
		return EquipmentSlotProfile{}, err
	}
	for idx := range cache.ordered {
		tree := cache.ordered[idx].tree
		if tree.Disabled || tree.OwnerType != "player" {
			continue
		}
		_, allocations, err := s.specializationStateReadTx(ctx, tx, userID, tree)
		if err != nil {
			return EquipmentSlotProfile{}, err
		}
		if len(allocations) == 0 {
			continue
		}
		deltas, err := specializationCapacityDeltas(tree, allocations)
		if err != nil {
			return EquipmentSlotProfile{}, err
		}
		for family, delta := range deltas {
			profile.Capacities[family] += delta
		}
	}
	combinedDeltas := map[string]int{}
	base := BasePlayerBattleSlotProfile()
	for family, capacity := range profile.Capacities {
		combinedDeltas[family] = capacity - base.Capacities[family]
	}
	return ResolvePlayerBattleSlotProfile(combinedDeltas)
}

func validateSpecializationExpectedRuleset(tree specializationTreeSpec, expected string) error {
	if strings.TrimSpace(expected) != tree.RulesetVersion {
		return ErrSpecializationRulesetConflict
	}
	return nil
}

func specializationCanonicalPaidCost(node specializationNodeSpec, rank int) (int, error) {
	if rank <= 0 || rank > len(node.Levels) {
		return 0, ErrSpecializationDefinitionInvalid
	}
	paidCost := 0
	for levelIndex := 0; levelIndex < rank; levelIndex++ {
		cost := node.Levels[levelIndex].Cost
		if cost <= 0 || paidCost > math.MaxInt-cost {
			return 0, ErrSpecializationDefinitionInvalid
		}
		paidCost += cost
	}
	return paidCost, nil
}

func validateCurrentSpecializationAllocations(tree specializationTreeSpec, allocations []SpecializationAllocation) error {
	if len(allocations) > len(tree.Nodes) {
		return ErrSpecializationDefinitionInvalid
	}
	seen := make(map[string]struct{}, len(allocations))
	for _, allocation := range allocations {
		if _, duplicate := seen[allocation.NodeID]; duplicate {
			return ErrSpecializationDefinitionInvalid
		}
		seen[allocation.NodeID] = struct{}{}
		node, ok := specializationTreeNode(tree, allocation.NodeID)
		if !ok || allocation.Rank <= 0 || allocation.Rank > len(node.Levels) {
			return ErrSpecializationDefinitionInvalid
		}
		expectedPaidCost, err := specializationCanonicalPaidCost(node, allocation.Rank)
		if err != nil || allocation.PaidCost != expectedPaidCost {
			return ErrSpecializationDefinitionInvalid
		}
	}
	if specializationSpentPoints(allocations) > tree.MaxPoints {
		return ErrSpecializationDefinitionInvalid
	}
	if tree.MaxActiveRootNodes > 0 && specializationActiveRootCount(tree, allocations) > tree.MaxActiveRootNodes {
		return ErrSpecializationDefinitionInvalid
	}
	return nil
}

type specializationStateSnapshot struct {
	Revision     int64
	Allocations  []SpecializationAllocation
	RulesetReset *SpecializationRulesetResetNotice
}

func decodeSpecializationRulesetResetNotice(raw []byte) (*SpecializationRulesetResetNotice, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("{}")) {
		return nil, nil
	}
	var notice SpecializationRulesetResetNotice
	if err := json.Unmarshal(raw, &notice); err != nil {
		return nil, fmt.Errorf("%w: decode specialization ruleset reset notice: %v", ErrSpecializationDefinitionInvalid, err)
	}
	if strings.TrimSpace(notice.CurrentRulesetVersion) == "" || notice.CurrentRevision < 0 {
		return nil, ErrSpecializationDefinitionInvalid
	}
	return &notice, nil
}

func specializationRulesetResetDisabledSlots(impact SpecializationEquipmentImpact, after EquipmentSlotProfile) []string {
	seen := map[string]struct{}{}
	for _, item := range impact.AutoUnequipped {
		for _, rawSlot := range item.FromSlots {
			slot := strings.TrimSpace(rawSlot)
			if slot == "" || after.Enabled(slot) {
				continue
			}
			seen[slot] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for slot := range seen {
		out = append(out, slot)
	}
	sort.Strings(out)
	return out
}

func specializationStateResetReason(tree specializationTreeSpec, ruleset string, allocations []SpecializationAllocation) string {
	// rulesetVersion is a persistence compatibility epoch. Compatible additions
	// keep it unchanged, so only an explicitly incompatible release reaches here.
	if strings.TrimSpace(ruleset) != tree.RulesetVersion {
		return "ruleset_changed"
	}
	if err := validateCurrentSpecializationAllocations(tree, allocations); err != nil {
		return "allocation_invalid"
	}
	return ""
}

func (s *Store) resetIncompatibleSpecializationStateTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	tree specializationTreeSpec,
	previousRuleset string,
	previousRevision int64,
	allocations []SpecializationAllocation,
	reason string,
) (specializationStateSnapshot, error) {
	if previousRevision == math.MaxInt64 {
		return specializationStateSnapshot{}, ErrSpecializationDefinitionInvalid
	}
	afterProfile := BasePlayerBattleSlotProfile()
	impact, err := s.specializationEquipmentImpactTx(
		ctx,
		tx,
		userID,
		tree.ID,
		previousRevision,
		"ruleset_reset",
		"",
		afterProfile,
		afterProfile,
	)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	impact.DisabledSlots = specializationRulesetResetDisabledSlots(impact, afterProfile)

	if err := replaceSpecializationAllocationsTx(ctx, tx, userID, tree.ID, nil); err != nil {
		return specializationStateSnapshot{}, err
	}
	currentRevision := previousRevision + 1
	if _, err := tx.Exec(ctx, `
		UPDATE player_specialization_states
		SET ruleset_version = $3,
		    revision = $4,
		    spent_points = 0,
		    last_ruleset_reset_jsonb = '{}'::jsonb,
		    updated_at = now()
		WHERE user_id = $1 AND tree_id = $2
	`, userID, tree.ID, tree.RulesetVersion, currentRevision); err != nil {
		return specializationStateSnapshot{}, err
	}
	if err := s.applySpecializationEquipmentImpactTx(ctx, tx, userID, impact); err != nil {
		return specializationStateSnapshot{}, err
	}
	if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
		return specializationStateSnapshot{}, err
	}
	cascaded, err := s.reconcileUnmetEquipmentStatusReqTx(ctx, tx, userID)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	impact.AutoUnequipped = mergeSpecializationAutoUnequipped(impact.AutoUnequipped, cascaded)
	impact.DisabledSkills, err = s.specializationDisabledSkillsTx(ctx, tx, userID)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	if err := disableSpecializationSkillsTx(ctx, tx, userID, impact.DisabledSkills); err != nil {
		return specializationStateSnapshot{}, err
	}
	if err := finalizeSpecializationImpactDigest(tree.ID, previousRevision, "ruleset_reset", "", afterProfile, &impact); err != nil {
		return specializationStateSnapshot{}, err
	}
	battleLevel, _, err := s.specializationIntrinsicAttributesTx(ctx, tx, userID)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	notice := &SpecializationRulesetResetNotice{
		Reason:                 strings.TrimSpace(reason),
		PreviousRulesetVersion: strings.TrimSpace(previousRuleset),
		CurrentRulesetVersion:  tree.RulesetVersion,
		PreviousRevision:       previousRevision,
		CurrentRevision:        currentRevision,
		RemovedAllocations:     append([]SpecializationAllocation(nil), allocations...),
		Impact:                 impact,
	}
	noticeRaw, err := json.Marshal(notice)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_specialization_states
		SET formula_points = $3,
		    spent_points = 0,
		    last_ruleset_reset_jsonb = $4::jsonb,
		    updated_at = now()
		WHERE user_id = $1 AND tree_id = $2
	`, userID, tree.ID, specializationTotalPoints(tree, battleLevel), string(noticeRaw)); err != nil {
		return specializationStateSnapshot{}, err
	}
	return specializationStateSnapshot{
		Revision:     currentRevision,
		Allocations:  []SpecializationAllocation{},
		RulesetReset: notice,
	}, nil
}

func (s *Store) specializationStateTx(ctx context.Context, tx pgx.Tx, userID int64, tree specializationTreeSpec, create bool) (specializationStateSnapshot, error) {
	if err := s.lockUserTx(ctx, tx, userID); err != nil {
		return specializationStateSnapshot{}, err
	}
	if create {
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_specialization_states(user_id, tree_id, ruleset_version, revision, created_at, updated_at)
			VALUES($1, $2, $3, 0, now(), now())
			ON CONFLICT (user_id, tree_id) DO NOTHING
		`, userID, tree.ID, tree.RulesetVersion); err != nil {
			return specializationStateSnapshot{}, err
		}
	}
	var ruleset string
	var revision int64
	var noticeRaw []byte
	err := tx.QueryRow(ctx, `
		SELECT ruleset_version, revision, COALESCE(last_ruleset_reset_jsonb, '{}'::jsonb)
		FROM player_specialization_states
		WHERE user_id = $1 AND tree_id = $2
		FOR UPDATE
	`, userID, tree.ID).Scan(&ruleset, &revision, &noticeRaw)
	if errors.Is(err, pgx.ErrNoRows) && !create {
		return specializationStateSnapshot{Allocations: []SpecializationAllocation{}}, nil
	}
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	allocations, err := loadSpecializationAllocationsFrom(ctx, tx, userID, tree.ID)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	reason := specializationStateResetReason(tree, ruleset, allocations)
	if reason != "" {
		return s.resetIncompatibleSpecializationStateTx(ctx, tx, userID, tree, ruleset, revision, allocations, reason)
	}
	notice, err := decodeSpecializationRulesetResetNotice(noticeRaw)
	if err != nil {
		return specializationStateSnapshot{}, err
	}
	return specializationStateSnapshot{
		Revision:     revision,
		Allocations:  allocations,
		RulesetReset: notice,
	}, nil
}

func (s *Store) specializationStateReadTx(ctx context.Context, tx pgx.Tx, userID int64, tree specializationTreeSpec) (int64, []SpecializationAllocation, error) {
	state, err := s.specializationStateTx(ctx, tx, userID, tree, false)
	return state.Revision, state.Allocations, err
}

func (s *Store) specializationStateForUpdateTx(ctx context.Context, tx pgx.Tx, userID int64, tree specializationTreeSpec) (int64, []SpecializationAllocation, error) {
	state, err := s.specializationStateTx(ctx, tx, userID, tree, true)
	return state.Revision, state.Allocations, err
}

func (s *Store) GetPlayerSpecialization(ctx context.Context, userID int64, treeID string) (SpecializationView, error) {
	entry, err := s.findSpecializationTree(treeID)
	if err != nil {
		return SpecializationView{}, err
	}
	raw, err := entry.rawTree()
	if err != nil {
		return SpecializationView{}, err
	}
	tree := entry.tree
	var out SpecializationView
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		state, err := s.specializationStateTx(ctx, tx, userID, tree, true)
		if err != nil {
			return err
		}
		revision := state.Revision
		allocations := state.Allocations
		battleLevel, intrinsic, err := s.specializationIntrinsicAttributesTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		accounting, err := s.reconcileSpecializationStateAccountingTx(ctx, tx, userID, tree, battleLevel, allocations)
		if err != nil {
			return err
		}
		characterAttributes, err := specializationCharacterAttributesTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		profile, err := s.resolvePlayerBattleSlotProfileForAllocations(tree, allocations)
		if err != nil {
			return err
		}
		out = SpecializationView{
			Tree:                raw,
			Status:              buildSpecializationStatus(tree, revision, battleLevel, accounting.BonusPoints, intrinsic, allocations),
			CharacterAttributes: characterAttributes,
			EffectiveSlots:      profile.Slots,
			SlotCapacities:      profile.Capacities,
			RulesetReset:        state.RulesetReset,
		}
		return nil
	})
	return out, err
}

func prospectiveAllocationsWithRank(allocations []SpecializationAllocation, nodeID string, nextRank, addedCost int) []SpecializationAllocation {
	out := append([]SpecializationAllocation(nil), allocations...)
	for idx := range out {
		if out[idx].NodeID == nodeID {
			out[idx].Rank = nextRank
			out[idx].PaidCost += addedCost
			sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
			return out
		}
	}
	out = append(out, SpecializationAllocation{NodeID: nodeID, Rank: nextRank, PaidCost: addedCost})
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	return out
}

func (s *Store) specializationEquipmentImpactTx(ctx context.Context, tx pgx.Tx, userID int64, treeID string, revision int64, operation, nodeID string, before, after EquipmentSlotProfile) (SpecializationEquipmentImpact, error) {
	disabled, enabled := equipmentSlotCapacityDelta(before, after)
	rows, err := tx.Query(ctx, `
		SELECT s.instance_id, i.template_item_id, s.slot
		FROM player_equip_slots s
		JOIN player_item_instances i ON i.instance_id = s.instance_id AND i.user_id = s.user_id
		WHERE s.user_id = $1 AND s.equip_type = 'battle' AND i.state = 'equipped'
		ORDER BY s.instance_id ASC, s.slot ASC
		FOR UPDATE OF s, i
	`, userID)
	if err != nil {
		return SpecializationEquipmentImpact{}, err
	}
	type equipped struct {
		instanceID int64
		templateID string
		slots      []string
	}
	byInstance := map[int64]*equipped{}
	for rows.Next() {
		var instanceID int64
		var templateID, slot string
		if err := rows.Scan(&instanceID, &templateID, &slot); err != nil {
			rows.Close()
			return SpecializationEquipmentImpact{}, err
		}
		entry := byInstance[instanceID]
		if entry == nil {
			entry = &equipped{instanceID: instanceID, templateID: strings.TrimSpace(templateID)}
			byInstance[instanceID] = entry
		}
		entry.slots = append(entry.slots, strings.TrimSpace(slot))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return SpecializationEquipmentImpact{}, err
	}
	rows.Close()
	instanceIDs := make([]int64, 0, len(byInstance))
	for instanceID := range byInstance {
		instanceIDs = append(instanceIDs, instanceID)
	}
	sort.Slice(instanceIDs, func(i, j int) bool { return instanceIDs[i] < instanceIDs[j] })
	autoUnequipped := make([]SpecializationAutoUnequippedItem, 0, len(disabled))
	for _, instanceID := range instanceIDs {
		entry := byInstance[instanceID]
		template := s.gameData.EquipmentTemplateByID(entry.templateID)
		invalid := template == nil
		for _, slot := range entry.slots {
			if !after.Enabled(slot) || !equipmentTemplateAllowsEquipSlotWithProfile(template, "battle", slot, after) || (slot == "right_hand_2" && template != nil && len(template.OccupySlots) > 0) {
				invalid = true
				break
			}
		}
		if !invalid {
			continue
		}
		sort.Strings(entry.slots)
		autoUnequipped = append(autoUnequipped, SpecializationAutoUnequippedItem{
			InstanceID:     entry.instanceID,
			TemplateItemID: entry.templateID,
			FromSlots:      append([]string(nil), entry.slots...),
			Reason:         "slot_disabled_by_specialization",
		})
	}
	impact := SpecializationEquipmentImpact{
		DisabledSlots:  disabled,
		EnabledSlots:   enabled,
		AutoUnequipped: autoUnequipped,
		DisabledSkills: []SpecializationDisabledSkill{},
	}
	if err := finalizeSpecializationImpactDigest(treeID, revision, operation, nodeID, after, &impact); err != nil {
		return SpecializationEquipmentImpact{}, err
	}
	return impact, nil
}

func finalizeSpecializationImpactDigest(treeID string, revision int64, operation, nodeID string, after EquipmentSlotProfile, impact *SpecializationEquipmentImpact) error {
	if impact == nil {
		return ErrSpecializationDefinitionInvalid
	}
	digestPayload := struct {
		TreeID         string                             `json:"treeId"`
		Revision       int64                              `json:"revision"`
		Operation      string                             `json:"operation"`
		NodeID         string                             `json:"nodeId"`
		Capacities     map[string]int                     `json:"capacities"`
		AutoUnequipped []SpecializationAutoUnequippedItem `json:"autoUnequipped"`
		DisabledSkills []SpecializationDisabledSkill      `json:"disabledSkills"`
	}{
		TreeID:         treeID,
		Revision:       revision,
		Operation:      operation,
		NodeID:         nodeID,
		Capacities:     after.Capacities,
		AutoUnequipped: impact.AutoUnequipped,
		DisabledSkills: impact.DisabledSkills,
	}
	payload, err := json.Marshal(digestPayload)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	impact.Digest = hex.EncodeToString(digest[:])
	return nil
}

func (s *Store) applySpecializationEquipmentImpactTx(ctx context.Context, tx pgx.Tx, userID int64, impact SpecializationEquipmentImpact) error {
	seen := make(map[int64]struct{}, len(impact.AutoUnequipped))
	for _, item := range impact.AutoUnequipped {
		if item.InstanceID <= 0 {
			continue
		}
		if _, ok := seen[item.InstanceID]; ok {
			continue
		}
		seen[item.InstanceID] = struct{}{}
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_equip_slots
			WHERE user_id = $1 AND equip_type = 'battle' AND instance_id = $2
		`, userID, item.InstanceID); err != nil {
			return err
		}
		if err := s.setInstanceInventoryWhenFullyUnequippedTx(ctx, tx, userID, item.InstanceID); err != nil {
			return err
		}
	}
	return nil
}

func specializationStringSlice(raw any) []string {
	values := []string{}
	switch typed := raw.(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, value := range typed {
			values = append(values, fmt.Sprintf("%v", value))
		}
	case string:
		values = append(values, typed)
	}
	return uniqueTrimmedStrings(values)
}

func equalSpecializationDisabledSkills(left, right []SpecializationDisabledSkill) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(left[index].SkillID) != strings.TrimSpace(right[index].SkillID) || left[index].Reason != right[index].Reason {
			return false
		}
	}
	return true
}

func mergeSpecializationAutoUnequipped(existing []SpecializationAutoUnequippedItem, cascaded []autoUnequippedEquipmentReceipt) []SpecializationAutoUnequippedItem {
	byInstance := make(map[int64]SpecializationAutoUnequippedItem, len(existing)+len(cascaded))
	for _, item := range existing {
		if item.InstanceID <= 0 {
			continue
		}
		item.FromSlots = uniqueTrimmedStrings(item.FromSlots)
		sort.Strings(item.FromSlots)
		byInstance[item.InstanceID] = item
	}
	for _, item := range cascaded {
		if item.InstanceID <= 0 {
			continue
		}
		if _, exists := byInstance[item.InstanceID]; exists {
			continue
		}
		slots := uniqueTrimmedStrings(item.FromSlots)
		sort.Strings(slots)
		byInstance[item.InstanceID] = SpecializationAutoUnequippedItem{
			InstanceID:     item.InstanceID,
			TemplateItemID: strings.TrimSpace(item.TemplateID),
			FromSlots:      slots,
			Reason:         "requirements_unmet",
		}
	}
	instanceIDs := make([]int64, 0, len(byInstance))
	for instanceID := range byInstance {
		instanceIDs = append(instanceIDs, instanceID)
	}
	sort.Slice(instanceIDs, func(i, j int) bool { return instanceIDs[i] < instanceIDs[j] })
	out := make([]SpecializationAutoUnequippedItem, 0, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		out = append(out, byInstance[instanceID])
	}
	return out
}

func equalSpecializationAutoUnequipped(left, right []SpecializationAutoUnequippedItem) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].InstanceID != right[index].InstanceID ||
			strings.TrimSpace(left[index].TemplateItemID) != strings.TrimSpace(right[index].TemplateItemID) ||
			strings.TrimSpace(left[index].Reason) != strings.TrimSpace(right[index].Reason) ||
			strings.Join(left[index].FromSlots, "\x00") != strings.Join(right[index].FromSlots, "\x00") {
			return false
		}
	}
	return true
}

func directSpecializationAutoUnequipped(items []SpecializationAutoUnequippedItem) []SpecializationAutoUnequippedItem {
	out := make([]SpecializationAutoUnequippedItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Reason) == "slot_disabled_by_specialization" {
			out = append(out, item)
		}
	}
	return out
}

func (s *Store) specializationSkillDefinition(skillID string) map[string]any {
	for _, raw := range s.gameData.Skills {
		id := strings.TrimSpace(fmt.Sprintf("%v", raw["id"]))
		if strings.EqualFold(id, skillID) {
			return raw
		}
	}
	return nil
}

func specializationSkillEligible(raw map[string]any, attrs map[string]float64, weaponClasses map[string]struct{}) bool {
	if requirement, ok := raw["statusRequirement"].(map[string]any); ok {
		for attrKey, rawRequired := range requirement {
			required, valid := numericFromAny(rawRequired)
			if valid && required > 0 && attrs[strings.TrimSpace(attrKey)] < required {
				return false
			}
		}
	}
	weaponRequirements := specializationStringSlice(raw["weaponRequirement"])
	if len(weaponRequirements) == 0 {
		return true
	}
	for _, requirement := range weaponRequirements {
		if _, ok := weaponClasses[strings.ToLower(requirement)]; ok {
			return true
		}
	}
	return false
}

func specializationInferredWeaponClasses(templateID string) []string {
	normalized := strings.ToLower(strings.TrimSpace(templateID))
	for _, candidate := range []string{"crossbow", "bow", "spear", "sword", "staff"} {
		if strings.Contains(normalized, candidate) {
			return []string{candidate}
		}
	}
	return nil
}

func (s *Store) specializationBattleSkillSlotLimit(intelligence float64) int {
	baseSlots, intPerExtra, minSlots, maxSlots := 4, 10, 1, 8
	if len(s.gameData.CombatConfigRules) > 0 {
		raw := s.gameData.CombatConfigRules[0]
		if value, ok := numericFromAny(raw["base_skill_slots"]); ok && value > 0 {
			baseSlots = int(value)
		}
		if value, ok := numericFromAny(raw["int_per_extra_slot"]); ok && value > 0 {
			intPerExtra = int(value)
		}
		if value, ok := numericFromAny(raw["min_skill_slots"]); ok && value > 0 {
			minSlots = int(value)
		}
		if value, ok := numericFromAny(raw["max_skill_slots"]); ok && value > 0 {
			maxSlots = int(value)
		}
	}
	if maxSlots < minSlots {
		maxSlots = minSlots
	}
	baseSlots = maxInt(minSlots, baseSlots)
	if baseSlots > maxSlots {
		baseSlots = maxSlots
	}
	total := baseSlots
	if intPerExtra > 0 {
		total += int(math.Floor(math.Max(0, intelligence) / float64(intPerExtra)))
	}
	total = maxInt(minSlots, total)
	if total > maxSlots {
		total = maxSlots
	}
	return total
}

func (s *Store) specializationDisabledSkillsTx(ctx context.Context, tx pgx.Tx, userID int64) ([]SpecializationDisabledSkill, error) {
	attrs, err := specializationCharacterAttributesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	equipped, err := s.listValidPlayerEquippedItemsTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	weaponClasses := map[string]struct{}{}
	for _, row := range equipped {
		if normalizeEquipType(row.EquipType) != "battle" {
			continue
		}
		template := s.gameData.EquipmentTemplateByID(strings.TrimSpace(row.TemplateItemID))
		classes := []string{}
		if template != nil {
			classes = template.WeaponClass
		}
		if len(classes) == 0 {
			classes = specializationInferredWeaponClasses(row.TemplateItemID)
		}
		for _, classID := range classes {
			classID = strings.ToLower(strings.TrimSpace(classID))
			if classID != "" {
				weaponClasses[classID] = struct{}{}
			}
		}
	}
	rows, err := tx.Query(ctx, `
		SELECT skill_id
		FROM player_skills
		WHERE user_id = $1 AND enabled
		ORDER BY cast_order ASC, skill_id ASC
		FOR UPDATE
	`, userID)
	if err != nil {
		return nil, err
	}
	enabled := []string{}
	for rows.Next() {
		var skillID string
		if err := rows.Scan(&skillID); err != nil {
			rows.Close()
			return nil, err
		}
		enabled = append(enabled, strings.TrimSpace(skillID))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	basicSkillID := s.defaultBattleBasicSkillID()
	eligible := make([]string, 0, len(enabled))
	reasonBySkill := map[string]string{}
	for _, skillID := range enabled {
		if strings.EqualFold(skillID, basicSkillID) {
			eligible = append(eligible, skillID)
			continue
		}
		raw := s.specializationSkillDefinition(skillID)
		if raw == nil || specializationSkillEligible(raw, attrs, weaponClasses) {
			eligible = append(eligible, skillID)
		} else {
			reasonBySkill[strings.ToLower(skillID)] = "requirements_unmet"
		}
	}
	limit := s.specializationBattleSkillSlotLimit(attrs["intelligence"])
	kept := eligible
	if limit >= 0 && len(kept) > limit {
		kept = append([]string(nil), eligible[:limit]...)
		basicIndex := -1
		for index, skillID := range eligible {
			if strings.EqualFold(skillID, basicSkillID) {
				basicIndex = index
				break
			}
		}
		if basicIndex >= limit && limit > 0 {
			kept = append([]string(nil), eligible[:limit-1]...)
			kept = append(kept, eligible[basicIndex])
		}
	}
	keepSet := map[string]struct{}{}
	for _, skillID := range kept {
		keepSet[strings.ToLower(strings.TrimSpace(skillID))] = struct{}{}
	}
	disabled := make([]SpecializationDisabledSkill, 0, len(enabled)-len(kept))
	for _, skillID := range enabled {
		key := strings.ToLower(strings.TrimSpace(skillID))
		if _, ok := keepSet[key]; !ok {
			reason := reasonBySkill[key]
			if reason == "" {
				reason = "slot_capacity_exceeded"
			}
			disabled = append(disabled, SpecializationDisabledSkill{SkillID: skillID, Reason: reason})
		}
	}
	return disabled, nil
}

func disableSpecializationSkillsTx(ctx context.Context, tx pgx.Tx, userID int64, disabled []SpecializationDisabledSkill) error {
	if len(disabled) == 0 {
		return nil
	}
	skillIDs := make([]string, 0, len(disabled))
	for _, skill := range disabled {
		skillIDs = append(skillIDs, skill.SkillID)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE player_skills
		SET enabled = FALSE, cast_order = 0, updated_at = now()
		WHERE user_id = $1 AND skill_id = ANY($2::text[])
	`, userID, skillIDs); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		WITH ranked AS (
			SELECT skill_id, (row_number() OVER (ORDER BY cast_order ASC, skill_id ASC) - 1)::INT AS next_order
			FROM player_skills
			WHERE user_id = $1 AND enabled
		)
		UPDATE player_skills ps
		SET cast_order = ranked.next_order, updated_at = now()
		FROM ranked
		WHERE ps.user_id = $1 AND ps.skill_id = ranked.skill_id
	`, userID)
	return err
}

func replaceSpecializationAllocationsTx(ctx context.Context, tx pgx.Tx, userID int64, treeID string, allocations []SpecializationAllocation) error {
	if _, err := tx.Exec(ctx, `DELETE FROM player_specialization_allocations WHERE user_id=$1 AND tree_id=$2`, userID, treeID); err != nil {
		return err
	}
	for _, allocation := range allocations {
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_specialization_allocations(user_id,tree_id,node_id,rank,paid_cost,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,now(),now())
		`, userID, treeID, allocation.NodeID, allocation.Rank, allocation.PaidCost); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) previewSpecializationConsequencesTx(ctx context.Context, tx pgx.Tx, userID int64, tree specializationTreeSpec, allocations []SpecializationAllocation, equipmentImpact SpecializationEquipmentImpact) ([]SpecializationDisabledSkill, []autoUnequippedEquipmentReceipt, error) {
	if _, err := tx.Exec(ctx, `SAVEPOINT specialization_skill_impact`); err != nil {
		return nil, nil, err
	}
	restore := func() error {
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT specialization_skill_impact`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `RELEASE SAVEPOINT specialization_skill_impact`)
		return err
	}
	if err := replaceSpecializationAllocationsTx(ctx, tx, userID, tree.ID, allocations); err != nil {
		_ = restore()
		return nil, nil, err
	}
	directImpact := equipmentImpact
	directImpact.AutoUnequipped = directSpecializationAutoUnequipped(equipmentImpact.AutoUnequipped)
	if err := s.applySpecializationEquipmentImpactTx(ctx, tx, userID, directImpact); err != nil {
		_ = restore()
		return nil, nil, err
	}
	if err := s.recomputeDerivedStatsTx(ctx, tx, userID); err != nil {
		_ = restore()
		return nil, nil, err
	}
	cascaded, err := s.reconcileUnmetEquipmentStatusReqTx(ctx, tx, userID)
	if err != nil {
		_ = restore()
		return nil, nil, err
	}
	disabled, err := s.specializationDisabledSkillsTx(ctx, tx, userID)
	if restoreErr := restore(); err == nil {
		err = restoreErr
	}
	return disabled, cascaded, err
}

func (s *Store) writeSpecializationLedgerTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	treeID string,
	revision int64,
	operation string,
	nodeID string,
	paidCost int,
	resetCost SpecializationResetCost,
	impact SpecializationEquipmentImpact,
	idempotencyKey string,
	result SpecializationMutationResult,
) error {
	resultPayload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO player_specialization_ledger(
			user_id, tree_id, revision, operation, node_id, paid_cost,
			reset_item_id, reset_item_count, impact_digest, idempotency_key, result_jsonb, created_at
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,now())
	`, userID, treeID, revision, operation, nodeID, paidCost, resetCost.ItemID, resetCost.Count, impact.Digest, idempotencyKey, string(resultPayload))
	return err
}

func replaySpecializationMutationTx(
	ctx context.Context,
	tx pgx.Tx,
	userID int64,
	treeID string,
	expectedRulesetVersion string,
	idempotencyKey string,
	operation string,
	nodeID string,
) (SpecializationMutationResult, bool, error) {
	var storedOperation, storedNodeID string
	var raw []byte
	err := tx.QueryRow(ctx, `
		SELECT operation, node_id, result_jsonb
		FROM player_specialization_ledger
		WHERE user_id = $1 AND tree_id = $2 AND idempotency_key = $3
	`, userID, treeID, idempotencyKey).Scan(&storedOperation, &storedNodeID, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return SpecializationMutationResult{}, false, nil
	}
	if err != nil {
		return SpecializationMutationResult{}, false, err
	}
	if storedOperation != operation || strings.TrimSpace(storedNodeID) != strings.TrimSpace(nodeID) {
		return SpecializationMutationResult{}, false, ErrSpecializationIdempotencyConflict
	}
	var result SpecializationMutationResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return SpecializationMutationResult{}, false, err
	}
	if err := validateSpecializationLedgerReplay(result, treeID, expectedRulesetVersion); err != nil {
		return SpecializationMutationResult{}, false, err
	}
	return result, true, nil
}

func validateSpecializationLedgerReplay(result SpecializationMutationResult, treeID, expectedRulesetVersion string) error {
	if strings.TrimSpace(treeID) == "" ||
		strings.TrimSpace(expectedRulesetVersion) == "" ||
		strings.TrimSpace(result.Status.TreeID) != strings.TrimSpace(treeID) ||
		strings.TrimSpace(result.Status.RulesetVersion) != strings.TrimSpace(expectedRulesetVersion) {
		return ErrSpecializationIdempotencyConflict
	}
	return nil
}

func cloneSpecializationRuntimePayload(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		out[key] = cloneSpecializationRuntimeValue(value)
	}
	return out
}

func cloneSpecializationRuntimeValue(raw any) any {
	switch value := raw.(type) {
	case map[string]any:
		return cloneSpecializationRuntimePayload(value)
	case []any:
		out := make([]any, len(value))
		for index := range value {
			out[index] = cloneSpecializationRuntimeValue(value[index])
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(value))
		for index := range value {
			out[index] = cloneSpecializationRuntimePayload(value[index])
		}
		return out
	default:
		return raw
	}
}

func appendActiveSpecializationNodeContribution(out *SpecializationBattleContribution, tree specializationTreeSpec, node specializationNodeSpec, rank int, scope string) error {
	if out == nil || rank < 0 || rank > len(node.Levels) {
		return ErrSpecializationDefinitionInvalid
	}
	if rank == 0 {
		return nil
	}
	// levels[] contains complete rank snapshots, not incremental deltas.
	// Only the currently learned rank may contribute at runtime.
	level := node.Levels[rank-1]
	for _, grant := range level.Grants {
		if err := validateSpecializationGrantShape(grant); err != nil {
			return err
		}
		grantKind := strings.ToLower(strings.TrimSpace(grant.Kind))
		grantID := strings.TrimSpace(grant.ID)
		sourcePrefix := fmt.Sprintf("%s:%s:%s", tree.ID, node.ID, grantID)
		switch grantKind {
		case "stat_modifier":
			grantScope := strings.ToLower(strings.TrimSpace(grant.Scope))
			if grantScope != scope {
				continue
			}
			for modifierIndex, raw := range grant.Modifiers {
				attributeKey := strings.TrimSpace(raw.AttributeKey)
				operation := strings.ToLower(strings.TrimSpace(raw.Operation))
				if attributeKey == "" || (operation != "add" && operation != "multiply") || math.IsNaN(raw.Value) || math.IsInf(raw.Value, 0) {
					return ErrSpecializationDefinitionInvalid
				}
				out.StatModifiers = append(out.StatModifiers, SpecializationStatModifier{
					ID:            fmt.Sprintf("%s:%d", sourcePrefix, modifierIndex+1),
					SourceNodeID:  node.ID,
					SourceNameKey: node.NameKey,
					Scope:         grantScope,
					AttributeKey:  attributeKey,
					Operation:     operation,
					Value:         raw.Value,
				})
			}
		case "battle_hook":
			if scope != "battle" {
				continue
			}
			for _, payload := range grant.BattleHooks {
				runtimeID, err := specializationRuntimePayloadGrantID(tree.ID, node.ID, grantID, payload)
				if err != nil {
					return err
				}
				out.Hooks = append(out.Hooks, SpecializationRuntimeGrant{
					ID:            runtimeID,
					SourceNodeID:  node.ID,
					SourceNameKey: node.NameKey,
					Payload:       cloneSpecializationRuntimePayload(payload),
				})
			}
		case "effect_mutation":
			if scope != "battle" {
				continue
			}
			for _, payload := range grant.EffectMutations {
				runtimeID, err := specializationRuntimePayloadGrantID(tree.ID, node.ID, grantID, payload)
				if err != nil {
					return err
				}
				out.Mutations = append(out.Mutations, SpecializationRuntimeGrant{
					ID:            runtimeID,
					SourceNodeID:  node.ID,
					SourceNameKey: node.NameKey,
					Payload:       cloneSpecializationRuntimePayload(payload),
				})
			}
		case "equipment_slot_capacity":
			// Equipment topology is consumed by the slot-profile authority,
			// not the battle contribution payload.
		default:
			return ErrSpecializationDefinitionInvalid
		}
	}
	return nil
}

func (s *Store) activeSpecializationContributionTx(ctx context.Context, tx pgx.Tx, userID int64, scope string) (SpecializationBattleContribution, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	out := SpecializationBattleContribution{
		StatModifiers: []SpecializationStatModifier{},
		Hooks:         []SpecializationRuntimeGrant{},
		Mutations:     []SpecializationRuntimeGrant{},
	}
	cache, err := s.specializationTrees()
	if err != nil {
		return SpecializationBattleContribution{}, err
	}
	for treeIndex := range cache.ordered {
		tree := cache.ordered[treeIndex].tree
		if tree.Disabled || tree.OwnerType != "player" {
			continue
		}
		revision, allocations, err := s.specializationStateReadTx(ctx, tx, userID, tree)
		if err != nil {
			return SpecializationBattleContribution{}, err
		}
		if out.TreeID == "" {
			out.TreeID = tree.ID
			out.RulesetVersion = tree.RulesetVersion
			out.BalanceRevision = tree.BalanceRevision
			out.Revision = revision
		}
		if len(allocations) == 0 {
			continue
		}
		ranks := allocationRankMap(allocations)
		for _, node := range tree.Nodes {
			if err := appendActiveSpecializationNodeContribution(&out, tree, node, ranks[node.ID], scope); err != nil {
				return SpecializationBattleContribution{}, err
			}
		}
	}
	digest, err := specializationBattleContributionDigest(out)
	if err != nil {
		return SpecializationBattleContribution{}, err
	}
	out.Digest = digest
	return out, nil
}

func specializationBattleContributionDigest(contribution SpecializationBattleContribution) (string, error) {
	digestPayload, err := json.Marshal(struct {
		TreeID          string                       `json:"treeId"`
		RulesetVersion  string                       `json:"rulesetVersion"`
		BalanceRevision int                          `json:"balanceRevision"`
		Revision        int64                        `json:"revision"`
		StatModifiers   []SpecializationStatModifier `json:"statModifiers"`
		Hooks           []SpecializationRuntimeGrant `json:"hooks"`
		Mutations       []SpecializationRuntimeGrant `json:"mutations"`
	}{contribution.TreeID, contribution.RulesetVersion, contribution.BalanceRevision, contribution.Revision, contribution.StatModifiers, contribution.Hooks, contribution.Mutations})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(digestPayload)
	return hex.EncodeToString(digest[:]), nil
}

func applySpecializationStatModifiers(values map[string]float64, modifiers []SpecializationStatModifier) error {
	for _, modifier := range modifiers {
		key := strings.TrimSpace(modifier.AttributeKey)
		if key == "" || math.IsNaN(modifier.Value) || math.IsInf(modifier.Value, 0) {
			return ErrSpecializationDefinitionInvalid
		}
		switch strings.ToLower(strings.TrimSpace(modifier.Operation)) {
		case "add":
			values[key] = roundStatValue(values[key] + modifier.Value)
		case "multiply":
			values[key] = roundStatValue(values[key] * modifier.Value)
		default:
			return ErrSpecializationDefinitionInvalid
		}
	}
	return nil
}

func specializationRuntimePayloadGrantID(treeID, nodeID, grantID string, payload map[string]any) (string, error) {
	payloadID, ok := payload["id"].(string)
	if !ok || !IsValidSpecializationRuntimeIDPart(grantID) || !IsValidSpecializationRuntimeIDPart(payloadID) {
		return "", ErrSpecializationDefinitionInvalid
	}
	treeID = strings.TrimSpace(treeID)
	nodeID = strings.TrimSpace(nodeID)
	if treeID == "" || nodeID == "" {
		return "", ErrSpecializationDefinitionInvalid
	}
	return strings.Join([]string{treeID, nodeID, grantID, payloadID}, ":"), nil
}

func (s *Store) applyCharacterSpecializationModifiersTx(ctx context.Context, tx pgx.Tx, userID int64, values map[string]float64) error {
	contribution, err := s.activeSpecializationContributionTx(ctx, tx, userID, "character")
	if err != nil {
		return err
	}
	return applySpecializationStatModifiers(values, contribution.StatModifiers)
}

func (s *Store) GetPlayerSpecializationBattleContribution(ctx context.Context, userID int64) (SpecializationBattleContribution, error) {
	var out SpecializationBattleContribution
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var err error
		out, err = s.activeSpecializationContributionTx(ctx, tx, userID, "battle")
		return err
	})
	return out, err
}

func (s *Store) AllocatePlayerSpecialization(ctx context.Context, userID int64, command SpecializationAllocateCommand) (SpecializationMutationResult, error) {
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.IdempotencyKey == "" {
		return SpecializationMutationResult{}, ErrSpecializationIdempotencyKeyRequired
	}
	entry, err := s.findSpecializationTree(command.TreeID)
	if err != nil {
		return SpecializationMutationResult{}, err
	}
	tree := entry.tree
	if err := validateSpecializationExpectedRuleset(tree, command.ExpectedRulesetVersion); err != nil {
		return SpecializationMutationResult{}, err
	}
	node, ok := specializationTreeNode(tree, command.NodeID)
	if !ok {
		return SpecializationMutationResult{}, ErrSpecializationNodeNotFound
	}
	var result SpecializationMutationResult
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		revision, allocations, err := s.specializationStateForUpdateTx(ctx, tx, userID, tree)
		if err != nil {
			return err
		}
		if replay, ok, err := replaySpecializationMutationTx(ctx, tx, userID, tree.ID, tree.RulesetVersion, command.IdempotencyKey, "allocate", node.ID); err != nil {
			return err
		} else if ok {
			result = replay
			return nil
		}
		if revision != command.ExpectedRevision {
			return ErrSpecializationRevisionConflict
		}
		if revision == math.MaxInt64 {
			return ErrSpecializationDefinitionInvalid
		}
		battleLevel, intrinsic, err := s.specializationIntrinsicAttributesTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		accounting, err := s.reconcileSpecializationStateAccountingTx(ctx, tx, userID, tree, battleLevel, allocations)
		if err != nil {
			return err
		}
		if battleLevel < tree.Unlock.BattleLevel {
			return ErrSpecializationTreeLocked
		}
		rank := allocationRankMap(allocations)[node.ID]
		if rank >= len(node.Levels) {
			return ErrSpecializationNodeMaxRank
		}
		totalPoints := accounting.FormulaPoints + accounting.BonusPoints
		if totalPoints > tree.MaxPoints {
			totalPoints = tree.MaxPoints
		}
		reasons := specializationNodeUnmetReasons(tree, node, allocations, intrinsic, totalPoints)
		for _, reason := range reasons {
			switch reason {
			case "MAX_RANK":
				return ErrSpecializationNodeMaxRank
			case "ACTIVE_ROOT_LIMIT":
				return ErrSpecializationActiveRootLimit
			case "EXCLUSIVE_GROUP":
				return ErrSpecializationExclusiveConflict
			case "INSUFFICIENT_POINTS":
				return ErrSpecializationInsufficientPoints
			default:
				return ErrSpecializationRequirementsUnmet
			}
		}
		cost := node.Levels[rank].Cost
		prospective := prospectiveAllocationsWithRank(allocations, node.ID, rank+1, cost)
		beforeProfile, err := s.resolvePlayerBattleSlotProfileForAllocations(tree, allocations)
		if err != nil {
			return err
		}
		afterProfile, err := s.resolvePlayerBattleSlotProfileForAllocations(tree, prospective)
		if err != nil {
			return err
		}
		impact, err := s.specializationEquipmentImpactTx(ctx, tx, userID, tree.ID, revision, "allocate", node.ID, beforeProfile, afterProfile)
		if err != nil {
			return err
		}
		var cascaded []autoUnequippedEquipmentReceipt
		impact.DisabledSkills, cascaded, err = s.previewSpecializationConsequencesTx(ctx, tx, userID, tree, prospective, impact)
		if err != nil {
			return err
		}
		impact.AutoUnequipped = mergeSpecializationAutoUnequipped(impact.AutoUnequipped, cascaded)
		if err := finalizeSpecializationImpactDigest(tree.ID, revision, "allocate", node.ID, afterProfile, &impact); err != nil {
			return err
		}
		currentStatus := buildSpecializationStatus(tree, revision, battleLevel, accounting.BonusPoints, intrinsic, allocations)
		result = SpecializationMutationResult{
			Committed:       false,
			RequiresConfirm: len(impact.AutoUnequipped) > 0 || len(impact.DisabledSkills) > 0,
			Status:          currentStatus,
			EffectiveSlots:  beforeProfile.Slots,
			SlotCapacities:  beforeProfile.Capacities,
			Impact:          impact,
		}
		requiresConfirmation := len(impact.AutoUnequipped) > 0 || len(impact.DisabledSkills) > 0
		if requiresConfirmation && strings.TrimSpace(command.ImpactDigest) == "" {
			return nil
		}
		if requiresConfirmation && !strings.EqualFold(strings.TrimSpace(command.ImpactDigest), impact.Digest) {
			return ErrSpecializationImpactConflict
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO player_specialization_allocations(user_id, tree_id, node_id, rank, paid_cost, created_at, updated_at)
			VALUES($1,$2,$3,$4,$5,now(),now())
			ON CONFLICT (user_id, tree_id, node_id)
			DO UPDATE SET rank = EXCLUDED.rank, paid_cost = EXCLUDED.paid_cost, updated_at = now()
		`, userID, tree.ID, node.ID, rank+1, prospectiveAllocationPaidCost(prospective, node.ID)); err != nil {
			return err
		}
		newRevision := revision + 1
		if _, err := tx.Exec(ctx, `
			UPDATE player_specialization_states
			SET revision = $3,
			    formula_points = $4,
			    spent_points = $5,
			    last_ruleset_reset_jsonb = '{}'::jsonb,
			    updated_at = now()
			WHERE user_id = $1 AND tree_id = $2
		`, userID, tree.ID, newRevision, accounting.FormulaPoints, specializationSpentPoints(prospective)); err != nil {
			return err
		}
		directImpact := impact
		directImpact.AutoUnequipped = directSpecializationAutoUnequipped(impact.AutoUnequipped)
		if err := s.applySpecializationEquipmentImpactTx(ctx, tx, userID, directImpact); err != nil {
			return err
		}
		if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}
		actualCascaded, err := s.reconcileUnmetEquipmentStatusReqTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		actualAutoUnequipped := mergeSpecializationAutoUnequipped(directImpact.AutoUnequipped, actualCascaded)
		if !equalSpecializationAutoUnequipped(actualAutoUnequipped, impact.AutoUnequipped) {
			return ErrSpecializationImpactConflict
		}
		actualDisabledSkills, err := s.specializationDisabledSkillsTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		if !equalSpecializationDisabledSkills(actualDisabledSkills, impact.DisabledSkills) {
			return ErrSpecializationImpactConflict
		}
		if err := disableSpecializationSkillsTx(ctx, tx, userID, actualDisabledSkills); err != nil {
			return err
		}
		result.Committed = true
		result.RequiresConfirm = false
		result.Status = buildSpecializationStatus(tree, newRevision, battleLevel, accounting.BonusPoints, intrinsic, prospective)
		result.EffectiveSlots = afterProfile.Slots
		result.SlotCapacities = afterProfile.Capacities
		if err := s.writeSpecializationLedgerTx(ctx, tx, userID, tree.ID, newRevision, "allocate", node.ID, cost, SpecializationResetCost{}, impact, command.IdempotencyKey, result); err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func prospectiveAllocationPaidCost(allocations []SpecializationAllocation, nodeID string) int {
	for _, allocation := range allocations {
		if allocation.NodeID == nodeID {
			return allocation.PaidCost
		}
	}
	return 0
}

func (s *Store) ResetPlayerSpecialization(ctx context.Context, userID int64, command SpecializationResetCommand) (SpecializationMutationResult, error) {
	command.IdempotencyKey = strings.TrimSpace(command.IdempotencyKey)
	if command.IdempotencyKey == "" {
		return SpecializationMutationResult{}, ErrSpecializationIdempotencyKeyRequired
	}
	entry, err := s.findSpecializationTree(command.TreeID)
	if err != nil {
		return SpecializationMutationResult{}, err
	}
	tree := entry.tree
	if err := validateSpecializationExpectedRuleset(tree, command.ExpectedRulesetVersion); err != nil {
		return SpecializationMutationResult{}, err
	}
	var result SpecializationMutationResult
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		revision, allocations, err := s.specializationStateForUpdateTx(ctx, tx, userID, tree)
		if err != nil {
			return err
		}
		if replay, ok, err := replaySpecializationMutationTx(ctx, tx, userID, tree.ID, tree.RulesetVersion, command.IdempotencyKey, "reset", ""); err != nil {
			return err
		} else if ok {
			result = replay
			return nil
		}
		if revision != command.ExpectedRevision {
			return ErrSpecializationRevisionConflict
		}
		battleLevel, intrinsic, err := s.specializationIntrinsicAttributesTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		accounting, err := s.reconcileSpecializationStateAccountingTx(ctx, tx, userID, tree, battleLevel, allocations)
		if err != nil {
			return err
		}
		beforeProfile, err := s.resolvePlayerBattleSlotProfileForAllocations(tree, allocations)
		if err != nil {
			return err
		}
		afterProfile, err := s.resolvePlayerBattleSlotProfileForAllocations(tree, nil)
		if err != nil {
			return err
		}
		if len(allocations) == 0 {
			result = SpecializationMutationResult{
				Committed:       false,
				RequiresConfirm: false,
				Status:          buildSpecializationStatus(tree, revision, battleLevel, accounting.BonusPoints, intrinsic, allocations),
				EffectiveSlots:  beforeProfile.Slots,
				SlotCapacities:  beforeProfile.Capacities,
				Impact:          SpecializationEquipmentImpact{},
			}
			if err := s.writeSpecializationLedgerTx(ctx, tx, userID, tree.ID, revision, "reset", "", 0, SpecializationResetCost{}, result.Impact, command.IdempotencyKey, result); err != nil {
				return err
			}
			return nil
		}
		if revision == math.MaxInt64 {
			return ErrSpecializationDefinitionInvalid
		}
		impact, err := s.specializationEquipmentImpactTx(ctx, tx, userID, tree.ID, revision, "reset", "", beforeProfile, afterProfile)
		if err != nil {
			return err
		}
		var cascaded []autoUnequippedEquipmentReceipt
		impact.DisabledSkills, cascaded, err = s.previewSpecializationConsequencesTx(ctx, tx, userID, tree, nil, impact)
		if err != nil {
			return err
		}
		impact.AutoUnequipped = mergeSpecializationAutoUnequipped(impact.AutoUnequipped, cascaded)
		if err := finalizeSpecializationImpactDigest(tree.ID, revision, "reset", "", afterProfile, &impact); err != nil {
			return err
		}
		result = SpecializationMutationResult{
			Committed:       false,
			RequiresConfirm: len(impact.AutoUnequipped) > 0 || len(impact.DisabledSkills) > 0,
			Status:          buildSpecializationStatus(tree, revision, battleLevel, accounting.BonusPoints, intrinsic, allocations),
			EffectiveSlots:  beforeProfile.Slots,
			SlotCapacities:  beforeProfile.Capacities,
			Impact:          impact,
		}
		requiresConfirmation := len(impact.AutoUnequipped) > 0 || len(impact.DisabledSkills) > 0
		if requiresConfirmation && strings.TrimSpace(command.ImpactDigest) == "" {
			return nil
		}
		if requiresConfirmation && !strings.EqualFold(strings.TrimSpace(command.ImpactDigest), impact.Digest) {
			return ErrSpecializationImpactConflict
		}
		if strings.TrimSpace(tree.Reset.ItemID) == "" || tree.Reset.Count <= 0 {
			return ErrSpecializationDefinitionInvalid
		}
		if err := s.ConsumeByTemplateTx(ctx, tx, userID, tree.Reset.ItemID, tree.Reset.Count); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM player_specialization_allocations
			WHERE user_id = $1 AND tree_id = $2
		`, userID, tree.ID); err != nil {
			return err
		}
		newRevision := revision + 1
		if _, err := tx.Exec(ctx, `
			UPDATE player_specialization_states
			SET revision = $3,
			    formula_points = $4,
			    spent_points = 0,
			    last_ruleset_reset_jsonb = '{}'::jsonb,
			    updated_at = now()
			WHERE user_id = $1 AND tree_id = $2
		`, userID, tree.ID, newRevision, accounting.FormulaPoints); err != nil {
			return err
		}
		directImpact := impact
		directImpact.AutoUnequipped = directSpecializationAutoUnequipped(impact.AutoUnequipped)
		if err := s.applySpecializationEquipmentImpactTx(ctx, tx, userID, directImpact); err != nil {
			return err
		}
		if err := s.onPlayerAttributesChangedTx(ctx, tx, userID); err != nil {
			return err
		}
		actualCascaded, err := s.reconcileUnmetEquipmentStatusReqTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		actualAutoUnequipped := mergeSpecializationAutoUnequipped(directImpact.AutoUnequipped, actualCascaded)
		if !equalSpecializationAutoUnequipped(actualAutoUnequipped, impact.AutoUnequipped) {
			return ErrSpecializationImpactConflict
		}
		actualDisabledSkills, err := s.specializationDisabledSkillsTx(ctx, tx, userID)
		if err != nil {
			return err
		}
		if !equalSpecializationDisabledSkills(actualDisabledSkills, impact.DisabledSkills) {
			return ErrSpecializationImpactConflict
		}
		if err := disableSpecializationSkillsTx(ctx, tx, userID, actualDisabledSkills); err != nil {
			return err
		}
		result.Committed = true
		result.RequiresConfirm = false
		result.Status = buildSpecializationStatus(tree, newRevision, battleLevel, accounting.BonusPoints, intrinsic, nil)
		result.EffectiveSlots = afterProfile.Slots
		result.SlotCapacities = afterProfile.Capacities
		if err := s.writeSpecializationLedgerTx(ctx, tx, userID, tree.ID, newRevision, "reset", "", specializationSpentPoints(allocations), tree.Reset, impact, command.IdempotencyKey, result); err != nil {
			return err
		}
		return nil
	})
	return result, err
}
