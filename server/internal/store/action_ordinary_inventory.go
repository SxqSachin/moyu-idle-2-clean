package store

import (
	"context"
	"encoding/json"
	"math"
	randv2 "math/rand/v2"
	"sort"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/actionsettlement"
)

const ordinarySegmentLimit = 2048
const ordinaryInstancePage = 512

type ordinaryInventoryRow struct {
	id           int64
	item         string
	quantity     int64
	archived     bool
	durability   int64
	affixes      []EquipmentAffixSlot
	dirty        bool
	commodity    bool
	enhanceLevel int
	base         json.RawMessage
}

// Existing row identity and unarchived-first consumption are preserved. A
// depleted stack is never resurrected by netting a later grant against costs.
type ordinaryInventory struct {
	rows     map[string][]*ordinaryInventoryRow
	original []*ordinaryInventoryRow
	created  []*ordinaryInventoryRow
	rules    map[string]ordinaryGrantRule
	random   *actionsettlement.Random
}

func (s *Store) loadOrdinaryInventoryTx(ctx context.Context, tx pgx.Tx, userID int64, state *ordinarySettlementState, random *actionsettlement.Random) (*ordinaryInventory, error) {
	ids := make([]string, 0, len(state.Grants))
	for id, rule := range state.Grants {
		if state.Spec.Costs[id] > 0 || rule.Stackable {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	locked, err := s.lockTemplateCommodityRowsByTemplateIDsTx(ctx, tx, userID, ids)
	if err != nil {
		return nil, err
	}
	inv := &ordinaryInventory{rows: make(map[string][]*ordinaryInventoryRow), rules: state.Grants, random: random}
	for _, row := range locked {
		entry := &ordinaryInventoryRow{id: row.InstanceID, item: row.TemplateItem, quantity: row.Quantity, archived: row.ArchivedAt != nil, commodity: true}
		inv.rows[row.TemplateItem] = append(inv.rows[row.TemplateItem], entry)
		inv.original = append(inv.original, entry)
	}
	return inv, nil
}

func (inv *ordinaryInventory) affordable(costs map[string]int64) bool {
	for id, required := range costs {
		if required <= 0 {
			continue
		}
		remaining := required
		for _, row := range inv.rows[id] {
			if !row.commodity {
				continue
			}
			if row.quantity >= remaining {
				remaining = 0
				break
			}
			remaining -= row.quantity
		}
		if remaining > 0 {
			return false
		}
	}
	return true
}

func (inv *ordinaryInventory) affordableStarts(costs map[string]int64, limit int) int {
	for id, cost := range costs {
		if cost <= 0 {
			continue
		}
		var available int64
		for _, row := range inv.rows[id] {
			if !row.commodity {
				continue
			}
			// Saturating the read is sufficient for a bounded affordability query.
			if row.quantity > math.MaxInt64-available {
				available = math.MaxInt64
				break
			}
			available += row.quantity
		}
		limit = min(limit, int(min(int64(limit), available/cost)))
	}
	return limit
}

func (inv *ordinaryInventory) consume(costs map[string]int64) error {
	if !inv.affordable(costs) {
		return ErrInsufficientItem
	}
	for id, required := range costs {
		for _, row := range inv.rows[id] {
			if required <= 0 {
				break
			}
			if !row.commodity || row.quantity <= 0 {
				continue
			}
			n := min(required, row.quantity)
			row.quantity -= n
			row.dirty = true
			required -= n
		}
	}
	return nil
}

func (inv *ordinaryInventory) grant(id string, quantity int64, ordinal int, stream string, rolled bool, inherited bool) error {
	if quantity <= 0 {
		return nil
	}
	rule, ok := inv.rules[id]
	if !ok {
		return ErrBadQuantity
	}
	stackable := rule.Stackable && (!rolled || len(rule.Rolls) == 0) && (!inherited || rule.EnhanceLevel <= 0)
	if stackable {
		var keep *ordinaryInventoryRow
		merged := quantity
		for _, row := range inv.rows[id] {
			if row.quantity <= 0 || row.archived || !row.commodity {
				continue
			}
			if row.quantity > math.MaxInt64-merged {
				return ErrInventoryQuantityOverflow
			}
			merged += row.quantity
			if keep == nil {
				keep = row
			}
			row.quantity = 0
			row.dirty = true
		}
		if keep == nil {
			keep = inv.add(id, 0, rule, nil, true)
		}
		keep.quantity, keep.durability, keep.affixes, keep.dirty = merged, rule.Durability, nil, true
		return nil
	}
	rng := randv2.New(inv.random.Source("grant:"+stream+":"+id, int64(ordinal)))
	for i := int64(0); i < quantity; i++ {
		affixes := rule.affixes(rng, rolled)
		// Default authored affixes are template-commodity inputs; rolled affixes
		// may only re-enter template costs when they exactly match that contract.
		commodity := rule.Commodity && sameAffixSlots(affixes, rule.DefaultAffixes)
		row := inv.add(id, 1, rule, affixes, commodity)
		if inherited && rule.EnhanceLevel > 0 {
			row.enhanceLevel = rule.EnhanceLevel
			row.commodity = false
			base, err := rule.enhancedSnapshot(affixes)
			if err != nil {
				return err
			}
			row.base = base
		}
	}
	return nil
}

func sameAffixSlots(a, b []EquipmentAffixSlot) bool {
	// Encoded slots have canonical struct fields and sorted JSON map keys.
	return string(encodeAffixSlots(a)) == string(encodeAffixSlots(b))
}

func (inv *ordinaryInventory) add(id string, quantity int64, rule ordinaryGrantRule, affixes []EquipmentAffixSlot, commodity bool) *ordinaryInventoryRow {
	row := &ordinaryInventoryRow{item: id, quantity: quantity, durability: rule.Durability, affixes: affixes, commodity: commodity, dirty: true}
	// Newly granted unarchived rows precede archived input stacks, but follow
	// existing unarchived IDs, exactly as the authoritative inventory query.
	rows := inv.rows[id]
	at := sort.Search(len(rows), func(i int) bool { return rows[i].archived })
	rows = append(rows, nil)
	copy(rows[at+1:], rows[at:])
	rows[at] = row
	inv.rows[id] = rows
	inv.created = append(inv.created, row)
	return row
}

func (inv *ordinaryInventory) flushTx(ctx context.Context, tx pgx.Tx, userID int64) error {
	var deleted, changed, quantities, durabilities []int64
	for _, row := range inv.original {
		if !row.dirty {
			continue
		}
		if row.quantity == 0 {
			deleted = append(deleted, row.id)
			continue
		}
		changed = append(changed, row.id)
		quantities = append(quantities, row.quantity)
		// A consume-only row retains its snapshot and durability. Grants to a
		// surviving commodity stack reset both, like AddInventoryItemTx.
		durabilities = append(durabilities, row.durability)
	}
	if len(deleted) > 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM player_item_instances WHERE user_id=$1 AND instance_id=ANY($2::bigint[])`, userID, deleted); err != nil {
			return err
		}
	}
	if len(changed) > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE player_item_instances p SET quantity=v.quantity,
			 durability=CASE WHEN v.durability>0 THEN v.durability ELSE p.durability END,
			 max_durability=CASE WHEN v.durability>0 THEN v.durability ELSE p.max_durability END,
			 base_snapshot=CASE WHEN v.durability>0 THEN '{}'::jsonb ELSE p.base_snapshot END,
			 affixes=CASE WHEN v.durability>0 THEN '[]'::jsonb ELSE p.affixes END, updated_at=now()
			FROM unnest($2::bigint[], $3::bigint[], $4::bigint[]) v(id,quantity,durability)
			WHERE p.user_id=$1 AND p.instance_id=v.id`, userID, changed, quantities, durabilities); err != nil {
			return err
		}
	}
	type insertRow struct {
		Item       string          `json:"item"`
		Quantity   int64           `json:"quantity"`
		Durability int64           `json:"durability"`
		Affixes    json.RawMessage `json:"affixes"`
	}
	page := make([]insertRow, 0, ordinaryInstancePage)
	flush := func() error {
		if len(page) == 0 {
			return nil
		}
		raw, err := json.Marshal(page)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO player_item_instances(user_id,template_item_id,enhance_level,roll_seed,base_snapshot,affixes,durability,max_durability,quantity,state,created_at,updated_at)
			SELECT $1,v.item,0,0,'{}'::jsonb,v.affixes,v.durability,v.durability,v.quantity,'inventory',now(),now()
			FROM jsonb_to_recordset($2::jsonb) AS v(item text,quantity bigint,durability bigint,affixes jsonb)`, userID, string(raw))
		page = page[:0]
		return err
	}
	for _, row := range inv.created {
		if row.quantity <= 0 {
			continue
		}
		if row.enhanceLevel > 0 {
			if err := flush(); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `INSERT INTO player_item_instances(user_id,template_item_id,enhance_level,roll_seed,base_snapshot,affixes,durability,max_durability,quantity,state,created_at,updated_at)
			 VALUES($1,$2,$3,0,$4::jsonb,$5::jsonb,$6,$6,1,'inventory',now(),now()) RETURNING instance_id`, userID, row.item, row.enhanceLevel, string(row.base), string(encodeAffixSlots(row.affixes)), row.durability).Scan(&row.id); err != nil {
				return err
			}
			continue
		}
		page = append(page, insertRow{row.item, row.quantity, row.durability, encodeAffixSlots(row.affixes)})
		if len(page) == cap(page) {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	return flush()
}
