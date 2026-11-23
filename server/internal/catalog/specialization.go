package catalog

import "strings"

type SpecializationTree struct {
	ID                 string                        `json:"id"`
	Disabled           bool                          `json:"disabled,omitempty"`
	SchemaVersion      int                           `json:"schemaVersion"`
	RulesetVersion     string                        `json:"rulesetVersion"`
	BalanceRevision    int                           `json:"balanceRevision"`
	OwnerType          string                        `json:"ownerType"`
	NameKey            string                        `json:"nameKey"`
	DescKey            string                        `json:"descKey"`
	I18n               map[string]SpecializationI18n `json:"i18n,omitempty"`
	InitialPoints      int                           `json:"initialPoints"`
	MaxPoints          int                           `json:"maxPoints"`
	MaxActiveRootNodes int                           `json:"maxActiveRootNodes"`
	Unlock             SpecializationUnlock          `json:"unlock"`
	PointBands         []SpecializationPointBand     `json:"pointBands"`
	Reset              SpecializationReset           `json:"reset"`
	Keywords           []SpecializationKeyword       `json:"keywords,omitempty"`
	Schools            []SpecializationSchool        `json:"schools"`
	Nodes              []SpecializationNode          `json:"nodes"`
	UI                 SpecializationTreeUI          `json:"ui"`
}

type SpecializationI18n struct {
	Name     string                               `json:"name"`
	Desc     string                               `json:"desc"`
	Schools  map[string]SpecializationI18nText    `json:"schools"`
	Nodes    map[string]SpecializationI18nText    `json:"nodes"`
	Keywords map[string]SpecializationKeywordI18n `json:"keywords,omitempty"`
}

type SpecializationKeyword struct {
	ID                 string   `json:"id"`
	MechanicKind       string   `json:"mechanicKind"`
	RuntimeIDs         []string `json:"runtimeIds"`
	SourceNodeIDs      []string `json:"sourceNodeIds"`
	NameKey            string   `json:"nameKey"`
	SummaryKey         string   `json:"summaryKey"`
	MechanicDetailsKey string   `json:"mechanicDetailsKey"`
	SourceHintKey      string   `json:"sourceHintKey"`
}

type SpecializationKeywordI18n struct {
	Name            string `json:"name"`
	PlayerSummary   string `json:"playerSummary"`
	MechanicDetails string `json:"mechanicDetails"`
	SourceHint      string `json:"sourceHint"`
}

type SpecializationI18nText struct {
	Name               string     `json:"name"`
	Desc               string     `json:"desc"`
	EffectSummary      []string   `json:"effectSummary,omitempty"`
	LevelEffectSummary [][]string `json:"levelEffectSummary,omitempty"`
}

type SpecializationUnlock struct {
	BattleLevel int `json:"battleLevel"`
}

type SpecializationPointBand struct {
	ID             string `json:"id"`
	FromLevel      int    `json:"fromLevel"`
	ToLevel        int    `json:"toLevel"`
	EveryLevels    int    `json:"everyLevels"`
	PointsPerGrant int    `json:"pointsPerGrant"`
}

type SpecializationReset struct {
	ItemID string `json:"itemId"`
	Count  int64  `json:"count"`
}

type SpecializationSchool struct {
	ID           string                 `json:"id"`
	NameKey      string                 `json:"nameKey"`
	DescKey      string                 `json:"descKey"`
	AttributeKey string                 `json:"attributeKey"`
	UI           SpecializationSchoolUI `json:"ui"`
}

type SpecializationSchoolUI struct {
	Column int    `json:"column"`
	Color  string `json:"color"`
}

type SpecializationNode struct {
	ID               string                     `json:"id"`
	Kind             string                     `json:"kind"`
	SchoolIDs        []string                   `json:"schoolIds"`
	NameKey          string                     `json:"nameKey"`
	DescKey          string                     `json:"descKey"`
	KeywordIDs       []string                   `json:"keywordIds,omitempty"`
	ExclusiveGroupID string                     `json:"exclusiveGroupId,omitempty"`
	Requirements     SpecializationRequirements `json:"requirements"`
	Levels           []SpecializationLevel      `json:"levels"`
	UI               SpecializationNodeUI       `json:"ui"`
}

type SpecializationRequirements struct {
	All   []SpecializationRequirement   `json:"all"`
	AnyOf [][]SpecializationRequirement `json:"anyOf"`
}

type SpecializationRequirement struct {
	Kind         string  `json:"kind"`
	NodeID       string  `json:"nodeId,omitempty"`
	MinRank      int     `json:"minRank,omitempty"`
	AttributeKey string  `json:"attributeKey,omitempty"`
	Value        float64 `json:"value,omitempty"`
	SchoolID     string  `json:"schoolId,omitempty"`
	Points       int     `json:"points,omitempty"`
}

type SpecializationLevel struct {
	Level  int                   `json:"level"`
	Cost   int                   `json:"cost"`
	Grants []SpecializationGrant `json:"grants"`
}

type SpecializationGrant struct {
	ID              string                       `json:"id"`
	Kind            string                       `json:"kind"`
	Scope           string                       `json:"scope,omitempty"`
	Modifiers       []SpecializationStatModifier `json:"modifiers,omitempty"`
	BattleHooks     []map[string]any             `json:"battleHooks,omitempty"`
	EffectMutations []map[string]any             `json:"effectMutations,omitempty"`
	EquipType       string                       `json:"equipType,omitempty"`
	Deltas          map[string]int               `json:"deltas,omitempty"`
}

type SpecializationStatModifier struct {
	AttributeKey string  `json:"attributeKey"`
	Operation    string  `json:"operation"`
	Value        float64 `json:"value"`
}

type SpecializationTreeUI struct {
	Layout      string   `json:"layout"`
	SchoolOrder []string `json:"schoolOrder"`
}

type SpecializationNodeUI struct {
	Column int      `json:"column"`
	Row    int      `json:"row"`
	Branch string   `json:"branch,omitempty"`
	Accent string   `json:"accent,omitempty"`
	X      *float64 `json:"x,omitempty"`
	Y      *float64 `json:"y,omitempty"`
}

func (d Data) SpecializationTreeByID(id string) *SpecializationTree {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for index := range d.SpecializationTrees {
		if d.SpecializationTrees[index].ID == trimmed {
			return &d.SpecializationTrees[index]
		}
	}
	return nil
}

func (t SpecializationTree) NodeByID(id string) *SpecializationNode {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for index := range t.Nodes {
		if t.Nodes[index].ID == trimmed {
			return &t.Nodes[index]
		}
	}
	return nil
}

func (t SpecializationTree) KeywordByID(id string) *SpecializationKeyword {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return nil
	}
	for index := range t.Keywords {
		if t.Keywords[index].ID == trimmed {
			return &t.Keywords[index]
		}
	}
	return nil
}

func (t SpecializationTree) PointsAtBattleLevel(level int) int {
	if level < t.Unlock.BattleLevel {
		return 0
	}
	points := t.InitialPoints
	for _, band := range t.PointBands {
		if level < band.FromLevel || band.EveryLevels <= 0 || band.PointsPerGrant <= 0 {
			continue
		}
		lastLevel := level
		if lastLevel > band.ToLevel {
			lastLevel = band.ToLevel
		}
		points += ((lastLevel-band.FromLevel)/band.EveryLevels + 1) * band.PointsPerGrant
	}
	if points > t.MaxPoints {
		return t.MaxPoints
	}
	return points
}
