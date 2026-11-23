package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"mysecondwebgame/server/internal/catteryfurniture"
	"mysecondwebgame/server/internal/operationeffect"
)

var (
	ErrFurnitureConflict = errors.New("furniture state changed")
	ErrFurnitureInvalid  = errors.New("invalid furniture command")
	ErrFurnitureCapacity = errors.New("furniture refill exceeds capacity")
	ErrFurnitureHistory  = errors.New("furniture recovery history full")
	ErrFurniturePending  = errors.New("furniture change awaiting activation")
)

type FurnitureView struct {
	Pending       *FurniturePending           `json:"pending,omitempty"`
	Upgrade       *FurnitureUpgradeQuote      `json:"upgrade,omitempty"`
	MaxLevel      int                         `json:"maxLevel"`
	Definition    catteryfurniture.Definition `json:"definition"`
	Installed     bool                        `json:"installed"`
	RefillAllowed bool                        `json:"refillAllowed"`
	RemainingMS   int64                       `json:"remainingMs"`
	EnabledUntil  *time.Time                  `json:"enabledUntil,omitempty"`
}
type FurnitureStoredState struct {
	Definition   catteryfurniture.Definition `json:"definition"`
	Installed    bool                        `json:"installed"`
	RemainingMS  int64                       `json:"remainingMs"`
	EnabledUntil *time.Time                  `json:"enabledUntil,omitempty"`
}
type FurniturePending struct {
	FurnitureStoredState
	EffectiveAt time.Time `json:"effectiveAt"`
}
type FurnitureProjection struct {
	ActivationDelaySeconds int64                         `json:"activationDelaySeconds"`
	Summary                bool                          `json:"summary,omitempty"`
	Contributions          operationeffect.Contributions `json:"contributions,omitempty"`
	Revision               int64                         `json:"revision"`
	ServerNow              time.Time                     `json:"serverNow"`
	Available              bool                          `json:"available"`
	Items                  []FurnitureView               `json:"items"`
	Inventory              map[string]int64              `json:"inventory"`
}
type FurnitureCommand struct {
	UpgradeQuote *FurnitureQuoteIdentity `json:"upgradeQuote,omitempty"`
	RequestID    string                  `json:"requestId"`
	FurnitureID  string                  `json:"furnitureId"`
	Version      string                  `json:"version"`
	Revision     int64                   `json:"revision"`
	Action       string                  `json:"action"`
	Count        int64                   `json:"count"`
}

type FurnitureQuoteIdentity struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expiresAt"`
	RemainingMS int64     `json:"remainingMs"`
}
type FurnitureUpgradeQuote struct {
	FurnitureQuoteIdentity
	Target          catteryfurniture.Definition `json:"target"`
	UpgradeCosts    map[string]int64            `json:"upgradeCosts"`
	SupplementCosts map[string]int64            `json:"supplementCosts"`
	TotalCosts      map[string]int64            `json:"totalCosts"`
}

func furnitureUpgradeQuote(userID, revision int64, old, target catteryfurniture.Definition, costs map[string]int64, remaining int64, expiry time.Time) (*FurnitureUpgradeQuote, error) {
	additional, err := catteryfurniture.UpgradeSupplement(old, target, remaining)
	if err != nil {
		return nil, err
	}
	total := map[string]int64{}
	for id, n := range costs {
		total[id] += n
	}
	for id, n := range additional {
		total[id] += n
	}
	q := &FurnitureUpgradeQuote{FurnitureQuoteIdentity: FurnitureQuoteIdentity{ExpiresAt: expiry, RemainingMS: remaining}, Target: target, UpgradeCosts: costs, SupplementCosts: additional, TotalCosts: total}
	raw, err := json.Marshal([]any{userID, revision, old, q})
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(raw)
	q.Token = hex.EncodeToString(hash[:])
	return q, nil
}

func (s *Store) furnitureDefinition(id string) (catteryfurniture.Definition, bool) {
	for _, d := range s.gameData.CatteryFurniture {
		if d.ID == id {
			return d, true
		}
	}
	return catteryfurniture.Definition{}, false
}

func (s *Store) GetFurniture(ctx context.Context, userID int64) (FurnitureProjection, error) {
	var out FurnitureProjection
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		var err error
		out, err = s.furnitureProjectionTx(ctx, tx, userID)
		return err
	})
	return out, err
}

// Card/action previews need neither inventory nor quotes nor reward accounting.
// Repeatable-read keeps the revision and rows consistent without a user lock.
func (s *Store) GetFurnitureSummary(ctx context.Context, userID int64) (FurnitureProjection, error) {
	var out FurnitureProjection
	err := s.withReadOnlyTx(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = s.furnitureProjectionTx(ctx, tx, userID, true)
		return err
	})
	return out, err
}

func (s *Store) furnitureProjectionTx(ctx context.Context, tx pgx.Tx, userID int64, summaryOnly ...bool) (FurnitureProjection, error) {
	out := FurnitureProjection{ServerNow: time.Now().UTC(), Items: []FurnitureView{}, Inventory: map[string]int64{}}
	out.ActivationDelaySeconds = int64(s.furnitureRuntimePolicy().delay / time.Second)
	out.Summary = len(summaryOnly) > 0 && summaryOnly[0]
	level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, "cattery")
	if err != nil {
		return out, err
	}
	out.Available = level > 0
	var contributionRaw []byte
	if out.Summary {
		err = tx.QueryRow(ctx, `SELECT revision FROM player_cattery_furniture_state WHERE user_id=$1`, userID).Scan(&out.Revision)
	} else {
		err = tx.QueryRow(ctx, `SELECT revision,contribution_totals FROM player_cattery_furniture_state WHERE user_id=$1`, userID).Scan(&out.Revision, &contributionRaw)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if len(contributionRaw) > 0 {
		if err = json.Unmarshal(contributionRaw, &out.Contributions); err != nil {
			return out, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT furniture_id,definition,remaining_ms,enabled_until,effective_at,previous_state FROM player_cattery_furniture WHERE user_id=$1`, userID)
	if err != nil {
		return out, err
	}
	installed := map[string]FurnitureView{}
	for rows.Next() {
		var id string
		var raw []byte
		var v FurnitureView
		var effectiveAt *time.Time
		var previousRaw []byte
		if err = rows.Scan(&id, &raw, &v.RemainingMS, &v.EnabledUntil, &effectiveAt, &previousRaw); err != nil {
			rows.Close()
			return out, err
		}
		if err = json.Unmarshal(raw, &v.Definition); err != nil {
			rows.Close()
			return out, err
		}
		if err = v.Definition.Validate(); err != nil {
			rows.Close()
			return out, err
		}
		v.Installed = true
		if effectiveAt != nil && effectiveAt.After(out.ServerNow) {
			v.Pending = &FurniturePending{FurnitureStoredState: FurnitureStoredState{Definition: v.Definition, Installed: true, RemainingMS: v.RemainingMS, EnabledUntil: v.EnabledUntil}, EffectiveAt: *effectiveAt}
			var previous FurnitureStoredState
			if err = json.Unmarshal(previousRaw, &previous); err != nil {
				rows.Close()
				return out, err
			}
			if err = previous.Definition.Validate(); err != nil {
				rows.Close()
				return out, err
			}
			v.Definition, v.Installed, v.RemainingMS, v.EnabledUntil = previous.Definition, previous.Installed, previous.RemainingMS, previous.EnabledUntil
		}
		v.RemainingMS = catteryfurniture.Remaining(v.RemainingMS, v.EnabledUntil, out.ServerNow)
		installed[id] = v
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	materials := map[string]bool{}
	for _, d := range s.gameData.CatteryFurniture {
		v, ok := installed[d.ID]
		level := v.Definition.CurrentLevel()
		current, err := d.AtLevel(level)
		if err != nil {
			return out, err
		}
		if !ok || !v.Installed {
			v.Definition = current
		} else if v.RemainingMS == 0 && !v.Definition.Permanent() {
			// Already-paid service keeps its old contract until exhausted.
			v.Definition = current
		}
		v.Definition.Level = level
		v.MaxLevel = len(d.Levels) + 1
		if !out.Summary && ok && v.Installed && v.Pending == nil && level < v.MaxLevel {
			next, err := d.AtLevel(level + 1)
			if err != nil {
				return out, err
			}
			quotedRemaining := catteryfurniture.Remaining(v.RemainingMS, v.EnabledUntil, out.ServerNow.Add(s.furnitureRuntimePolicy().delay))
			v.Upgrade, err = furnitureUpgradeQuote(userID, out.Revision, v.Definition, next, d.Levels[level-1].UpgradeCosts, quotedRemaining, out.ServerNow.Add(30*time.Second))
			if err != nil {
				return out, err
			}
			for id := range v.Upgrade.TotalCosts {
				materials[id] = true
			}
			for id := range next.RefillCosts {
				materials[id] = true
			}
		}
		v.RefillAllowed = v.Pending == nil && !v.Definition.Permanent() && v.Definition.Version == d.Version
		out.Items = append(out.Items, v)
		if out.Summary {
			continue
		}
		for id := range v.Definition.InstallCosts {
			materials[id] = true
		}
		for id := range v.Definition.RefillCosts {
			materials[id] = true
		}
	}
	if out.Summary {
		return out, nil
	}
	// Read all displayed materials together, independent of catalog size.
	materialIDs := make([]string, 0, len(materials))
	for id := range materials {
		materialIDs = append(materialIDs, id)
		out.Inventory[id] = 0
	}
	rows, err = tx.Query(ctx, `SELECT template_item_id,SUM(quantity) FROM player_item_instances
	 WHERE user_id=$1 AND template_item_id=ANY($2::text[]) AND state='inventory' GROUP BY template_item_id`, userID, materialIDs)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var quantity int64
		if err = rows.Scan(&id, &quantity); err != nil {
			return out, err
		}
		out.Inventory[id] = quantity
	}
	return out, rows.Err()
}

func (s *Store) CommandFurniture(ctx context.Context, userID int64, cmd FurnitureCommand) (FurnitureProjection, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var out FurnitureProjection
	if len(cmd.RequestID) < 8 || len(cmd.RequestID) > 96 || cmd.Revision < 0 || cmd.Count < 0 || cmd.Count > 100 {
		return out, ErrFurnitureInvalid
	}
	def, ok := s.furnitureDefinition(cmd.FurnitureID)
	if !ok {
		return out, ErrFurnitureInvalid
	}
	root := def
	def, err := root.AtLevel(1)
	if err != nil {
		return out, err
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return out, err
	}
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if err := s.lockUserTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := s.assertUserNotBannedTx(ctx, tx, userID); err != nil {
			return err
		}
		var existing bool
		err := tx.QueryRow(ctx, `SELECT request=$3::jsonb FROM player_cattery_furniture_commands WHERE user_id=$1 AND request_id=$2`, userID, cmd.RequestID, string(raw)).Scan(&existing)
		if err == nil {
			if !existing {
				return ErrFurnitureConflict
			}
			out, err = s.furnitureProjectionTx(ctx, tx, userID)
			return err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO player_cattery_furniture_state(user_id) VALUES($1) ON CONFLICT DO NOTHING`, userID); err != nil {
			return err
		}
		var revision int64
		if err = tx.QueryRow(ctx, `SELECT revision FROM player_cattery_furniture_state WHERE user_id=$1 FOR UPDATE`, userID).Scan(&revision); err != nil {
			return err
		}
		if revision != cmd.Revision {
			return ErrFurnitureConflict
		}
		var saved int64
		var until *time.Time
		var storedRaw []byte
		var pendingAt *time.Time
		err = tx.QueryRow(ctx, `SELECT definition,remaining_ms,enabled_until,effective_at FROM player_cattery_furniture WHERE user_id=$1 AND furniture_id=$2 FOR UPDATE`, userID, cmd.FurnitureID).Scan(&storedRaw, &saved, &until, &pendingAt)
		installed := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if installed {
			if err = json.Unmarshal(storedRaw, &def); err != nil {
				return err
			}
			if err = def.Validate(); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		if pendingAt != nil && pendingAt.After(now) {
			return ErrFurniturePending
		}
		remaining := catteryfurniture.Remaining(saved, until, now)
		if installed && remaining == 0 && !def.Permanent() {
			def, err = root.AtLevel(def.CurrentLevel())
			if err != nil {
				return err
			}
		}
		def.Level = def.CurrentLevel()
		previous := FurnitureStoredState{Definition: def, Installed: installed, RemainingMS: saved, EnabledUntil: until}
		if cmd.Version != def.Version {
			return ErrFurnitureConflict
		}
		active := installed && def.Permanent() || until != nil && until.After(now)
		wasActive := active
		effectiveAt := now.Add(s.furnitureRuntimePolicy().delay)
		remaining = catteryfurniture.Remaining(saved, until, effectiveAt)
		costs := map[string]int64{}
		if def.Permanent() && cmd.Action != "install" && cmd.Action != "upgrade" {
			return ErrFurnitureInvalid
		}
		switch cmd.Action {
		case "install":
			if installed || cmd.Count != 0 {
				return ErrFurnitureInvalid
			}
			level, err := s.getPlayerBuildingLevelTx(ctx, tx, userID, "cattery")
			if err != nil {
				return err
			}
			if level <= 0 {
				return ErrCatteryUnavailable
			}
			costs = def.InstallCosts
			remaining = def.CapacitySeconds * 1000
			active = def.Permanent()
		case "refill":
			if !installed || cmd.Count <= 0 {
				return ErrFurnitureInvalid
			}
			current, _ := s.furnitureDefinition(cmd.FurnitureID)
			if current.Version != def.Version {
				return ErrFurnitureConflict
			}
			add := cmd.Count * def.RefillSeconds * 1000
			if remaining+add > def.CapacitySeconds*1000 {
				return ErrFurnitureCapacity
			}
			remaining += add
			for id, n := range def.RefillCosts {
				costs[id] = n * cmd.Count
			}
		case "enable":
			if !installed || remaining <= 0 || cmd.Count != 0 {
				return ErrFurnitureInvalid
			}
			active = true
		case "upgrade":
			if !installed || cmd.Count != 0 || cmd.UpgradeQuote == nil || def.CurrentLevel() > len(root.Levels) {
				return ErrFurnitureInvalid
			}
			q := cmd.UpgradeQuote
			if !q.ExpiresAt.After(now) || q.ExpiresAt.After(now.Add(31*time.Second)) || q.RemainingMS < remaining || q.RemainingMS > def.CapacitySeconds*1000 {
				return ErrFurnitureConflict
			}
			next, err := root.AtLevel(def.CurrentLevel() + 1)
			if err != nil {
				return err
			}
			base := root.Levels[def.CurrentLevel()-1].UpgradeCosts
			expected, err := furnitureUpgradeQuote(userID, revision, def, next, base, q.RemainingMS, q.ExpiresAt)
			if err != nil {
				return err
			}
			if expected.Token != q.Token {
				return ErrFurnitureConflict
			}
			actual, err := furnitureUpgradeQuote(userID, revision, def, next, base, remaining, q.ExpiresAt)
			if err != nil {
				return err
			}
			for id, n := range actual.TotalCosts {
				if n > expected.TotalCosts[id] {
					return ErrFurnitureConflict
				}
			}
			costs = actual.TotalCosts
			def = next
		case "pause":
			if !installed || cmd.Count != 0 {
				return ErrFurnitureInvalid
			}
			active = false
		default:
			return ErrFurnitureInvalid
		}
		if len(costs) > 0 {
			if err = s.ensureOperationEffectChargeReplenishmentCaughtUpTx(ctx, tx, userID, now); err != nil {
				return err
			}
		}
		ids := make([]string, 0, len(costs))
		for id := range costs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			if err = s.LedgerConsumeTemplateTx(ctx, tx, userID, id, costs[id], InventoryLedgerMeta{Category: InventoryAuditCategoryKitty, Direction: InventoryAuditDirectionConsume, SubjectType: "furniture", SubjectID: cmd.FurnitureID, SourceSurface: "inventory", TargetSurface: "cattery_furniture", Details: map[string]any{"furnitureId": cmd.FurnitureID, "action": cmd.Action, "requestId": cmd.RequestID}}); err != nil {
				return err
			}
		}
		changeWindow := cmd.Action == "install" || cmd.Action == "refill" || cmd.Action == "upgrade" || active != wasActive
		// Inventory locks precede source locks, matching battle entry. Taking this
		// before charging could deadlock a battle holding the same material row.
		if err := lockFurnitureSourcesTx(ctx, tx, []int64{userID}, false); err != nil {
			return err
		}
		// Old intervals are redundant once all running lanes have advanced past
		// them: in-flight operations already carry their own immutable snapshot.
		if _, err = tx.Exec(ctx, `DELETE FROM player_cattery_furniture_periods WHERE user_id=$1
			AND valid_until<=LEAST($2,COALESCE((SELECT MIN(COALESCE(current_run_started_at,current_run_ends_at))
		 FROM player_action_queue WHERE user_id=$1 AND status='running'),$2))`, userID, now); err != nil {
			return err
		}
		if changeWindow && active {
			var count int
			if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM player_cattery_furniture_periods WHERE user_id=$1`, userID).Scan(&count); err != nil {
				return err
			}
			if count >= 4096 {
				return ErrFurnitureHistory
			}
		}
		// Never rewrite history. Close only the unelapsed tail, then append a
		// replacement interval. Already-started operations retain their snapshot.
		if changeWindow {
			if _, err = tx.Exec(ctx, `UPDATE player_cattery_furniture_periods SET valid_until=GREATEST(valid_from,$3) WHERE user_id=$1 AND furniture_id=$2 AND (valid_until IS NULL OR valid_until>$3)`, userID, cmd.FurnitureID, effectiveAt); err != nil {
				return err
			}
			until = nil
			if active {
				if !def.Permanent() {
					end := effectiveAt.Add(time.Duration(remaining) * time.Millisecond)
					until = &end
				}
				sourceRaw, _ := json.Marshal(def.Source())
				if _, err = tx.Exec(ctx, `INSERT INTO player_cattery_furniture_periods(user_id,furniture_id,valid_from,valid_until,source) VALUES($1,$2,$3,$4,$5)`, userID, cmd.FurnitureID, effectiveAt, until, string(sourceRaw)); err != nil {
					return err
				}
			}
		}
		defRaw, _ := json.Marshal(def)
		previousRaw, err := json.Marshal(previous)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO player_cattery_furniture(user_id,furniture_id,definition,remaining_ms,enabled_until,updated_at,effective_at,previous_state) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(user_id,furniture_id) DO UPDATE SET definition=EXCLUDED.definition,remaining_ms=EXCLUDED.remaining_ms,enabled_until=EXCLUDED.enabled_until,updated_at=EXCLUDED.updated_at,effective_at=EXCLUDED.effective_at,previous_state=EXCLUDED.previous_state`, userID, cmd.FurnitureID, string(defRaw), remaining, until, now, effectiveAt, string(previousRaw)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE player_cattery_furniture_state SET revision=revision+1 WHERE user_id=$1`, userID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO player_cattery_furniture_commands(user_id,request_id,request,revision) VALUES($1,$2,$3,$4)`, userID, cmd.RequestID, string(raw), revision+1); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM player_cattery_furniture_commands WHERE user_id=$1 AND revision<$2 AND created_at<$3`, userID, revision, now.Add(-7*24*time.Hour)); err != nil {
			return err
		}
		out, err = s.furnitureProjectionTx(ctx, tx, userID)
		return err
	})
	if err == nil {
		s.InvalidateFurnitureEffects(userID)
		if s.furnitureChanged != nil {
			s.furnitureChanged(ctx, userID, out.Revision)
		}
	}
	return out, err
}

func (s *Store) loadPassiveSessionTx(ctx context.Context, tx pgx.Tx, userID int64, from, through time.Time, pendingOwner ...bool) (*operationeffect.PassiveSession, error) {
	if len(s.gameData.CatteryFurniture) == 0 {
		return nil, nil
	}
	rows, err := s.furnitureWindowsTx(ctx, tx, []int64{userID}, from, through)
	if err != nil {
		return nil, err
	}
	p := &operationeffect.PassiveSession{}
	for _, window := range rows[userID] {
		if !window.From.After(through) {
			p.Windows = append(p.Windows, window)
		}
	}
	needsRemainder := len(pendingOwner) > 0 && pendingOwner[0]
	for _, w := range p.Windows {
		for _, r := range w.Source.Rules {
			if r.Kind == operationeffect.KindOwnerExperience {
				needsRemainder = true
			}
		}
	}
	if len(p.Windows) == 0 && !needsRemainder {
		return nil, nil
	}
	if needsRemainder {
		var raw []byte
		if err = tx.QueryRow(ctx, `SELECT exp_remainders FROM player_cattery_furniture_state WHERE user_id=$1`, userID).Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &p.Remainders); err != nil {
			return nil, err
		}
	}
	return p, nil
}
