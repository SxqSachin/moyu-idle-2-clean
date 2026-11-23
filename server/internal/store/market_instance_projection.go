package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/marketpolicy"
)

var ErrInstanceMarketChanged = errors.New("instance market quote changed")
var ErrInstanceMarketRequest = errors.New("invalid instance market request")
var ErrInstanceMarketUnavailable = errors.New("instance market item unavailable")

const InstanceMarketBatchLimit = 100

type InstanceMarketQuote struct {
	InstanceID int64  `json:"instanceId"`
	ItemID     string `json:"itemId"`
	ProfileID  string `json:"profileId"`
	marketpolicy.Projection
	Snapshot   map[string]any `json:"snapshot"`
	QuoteToken string         `json:"quoteToken"`
}
type InstanceMarketSelection struct {
	InstanceID int64  `json:"instanceId"`
	QuoteToken string `json:"quoteToken"`
}
type marketInstanceRow struct {
	LockedItemInstance
	FishRaw []byte
}

func decodeMarketObject(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var out map[string]any
	if err := d.Decode(&out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// Domain adapters expose a public read model, never arbitrary player data.
// Transactional identity/ownership always comes from the locked database row.
func (s *Store) projectMarketInstance(row marketInstanceRow) (InstanceMarketQuote, error) {
	var out InstanceMarketQuote
	item := s.gameData.ItemByID(row.TemplateItem)
	if item == nil || !item.UsesInstanceMarket() || item.Disabled || item.Stackable || item.HasType("currency") || row.Quantity != 1 || row.ArchivedAt != nil {
		return out, ErrInstanceMarketUnavailable
	}
	profile := s.gameData.MarketProfile(item.MarketPolicy.FixedPrice.ProfileID)
	if profile == nil {
		return out, ErrInstanceMarketUnavailable
	}
	base, err := decodeMarketObject(row.BaseSnapshot)
	if err != nil {
		return out, err
	}
	var affixes any
	if err = json.Unmarshal(row.AffixesRaw, &affixes); err != nil {
		return out, err
	}
	instance := map[string]any{"baseSnapshot": base, "customData": row.CustomData, "enhanceLevel": row.EnhanceLevel, "affixes": affixes, "durability": row.Durability, "maxDurability": row.MaxDurability}
	if len(row.FishRaw) > 0 && string(row.FishRaw) != "{}" && string(row.FishRaw) != "null" {
		var fish FishInstanceData
		if err = json.Unmarshal(row.FishRaw, &fish); err != nil {
			return out, err
		}
		// Undisclosed genetics never enter a public snapshot or a public hash.
		if !fish.GenomeRevealed {
			fish.Genes = FishGenes{Mutations: fish.Genes.Mutations}
			if !profile.UniqueByInstance {
				return out, ErrInstanceMarketUnavailable
			}
		}
		fish.ParentInstanceIDs = nil
		fish.PondCareRevision = 0
		fish.LongevityLimitDays = 0
		fish.NaturalProtectionUntilAgeDays = 0
		publicRaw, e := json.Marshal(fish)
		if e != nil {
			return out, e
		}
		instance["fishData"], err = decodeMarketObject(publicRaw)
		if err != nil {
			return out, err
		}
		if species := s.gameData.FishingSpeciesByID(fish.SpeciesID); species != nil {
			instance["specimenNames"] = species.Names
		}
	}
	// Normalize typed adapter outputs to the same JSON tree used by selectors.
	raw, err := json.Marshal(instance)
	if err != nil {
		return out, err
	}
	instance, err = decodeMarketObject(raw)
	if err != nil {
		return out, err
	}
	sources := map[string]any{"instance": instance}
	needsEffect := false
	for _, k := range append(append([]marketpolicy.Key{}, profile.GroupKeys...), profile.CatalogKeys...) {
		needsEffect = needsEffect || k.Source == "resolved.operationEffect"
	}
	for _, f := range profile.Presentation {
		needsEffect = needsEffect || f.Source == "resolved.operationEffect"
	}
	if needsEffect {
		def, ok, e := s.resolveOperationEffectDefinitionForItem(row.TemplateItem, row.BaseSnapshot)
		if e != nil {
			return out, e
		}
		if !ok {
			return out, ErrInstanceMarketUnavailable
		}
		def.NameKey = ""
		def.DescKey = ""
		def.IconID = ""
		raw, e := json.Marshal(def)
		if e != nil {
			return out, e
		}
		effect, e := decodeMarketObject(raw)
		if e != nil {
			return out, e
		}
		sources["resolved.operationEffect"] = effect
	}
	projection, err := marketpolicy.Project(*profile, row.TemplateItem, row.InstanceID, sources)
	if err != nil {
		return out, errors.Join(ErrInstanceMarketUnavailable, err)
	}
	token, err := marketpolicy.Digest([]any{row.InstanceID, row.UserID, projection.Revision, row.TemplateItem, instance, projection.GroupKey})
	if err != nil {
		return out, err
	}
	out = InstanceMarketQuote{InstanceID: row.InstanceID, ItemID: row.TemplateItem, ProfileID: profile.ID, Projection: projection, Snapshot: instance, QuoteToken: token}
	return out, nil
}

func normalizedMarketSelections(items []InstanceMarketSelection, confirm bool) ([]InstanceMarketSelection, []int64, error) {
	if len(items) < 1 || len(items) > InstanceMarketBatchLimit {
		return nil, nil, ErrBadQuantity
	}
	out := append([]InstanceMarketSelection(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].InstanceID < out[j].InstanceID })
	ids := make([]int64, len(out))
	for i, v := range out {
		if v.InstanceID <= 0 || (i > 0 && out[i-1].InstanceID == v.InstanceID) || (confirm && len(v.QuoteToken) != 64) {
			return nil, nil, ErrInstanceMarketRequest
		}
		ids[i] = v.InstanceID
	}
	return out, ids, nil
}

func (s *Store) lockMarketInstancesTx(ctx context.Context, tx pgx.Tx, owner int64, ids []int64, state string) ([]marketInstanceRow, error) {
	rows, err := tx.Query(ctx, `SELECT instance_id,user_id,template_item_id,quantity,COALESCE(custom_data,'{}'),base_snapshot,affixes,
 enhance_level,roll_seed,durability,max_durability,state,archived_at,COALESCE(fish_data,'{}')
 FROM player_item_instances WHERE user_id=$1 AND instance_id=ANY($2) ORDER BY instance_id FOR UPDATE`, owner, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]marketInstanceRow, 0, len(ids))
	for rows.Next() {
		var r marketInstanceRow
		var custom []byte
		if err = rows.Scan(&r.InstanceID, &r.UserID, &r.TemplateItem, &r.Quantity, &custom, &r.BaseSnapshot, &r.AffixesRaw, &r.EnhanceLevel, &r.RollSeed, &r.Durability, &r.MaxDurability, &r.State, &r.ArchivedAt, &r.FishRaw); err != nil {
			return nil, err
		}
		if r.State != state || r.Quantity != 1 {
			return nil, ErrInvalidInstanceState
		}
		r.CustomData = decodeItemCustomization(custom)
		out = append(out, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) != len(ids) {
		return nil, ErrInstanceNotOwned
	}
	return out, nil
}
func (s *Store) QuoteInstanceMarket(ctx context.Context, userID int64, items []InstanceMarketSelection) ([]InstanceMarketQuote, error) {
	_, ids, err := normalizedMarketSelections(items, false)
	if err != nil {
		return nil, err
	}
	out := make([]InstanceMarketQuote, 0, len(ids))
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		rows, err := s.lockMarketInstancesTx(ctx, tx, userID, ids, "inventory")
		if err != nil {
			return err
		}
		for _, r := range rows {
			q, e := s.projectMarketInstance(r)
			if e != nil {
				return e
			}
			out = append(out, q)
		}
		return nil
	})
	return out, err
}
func validMarketRequestKey(key string) bool {
	return strings.TrimSpace(key) == key && len(key) > 0 && len(key) <= 128
}
