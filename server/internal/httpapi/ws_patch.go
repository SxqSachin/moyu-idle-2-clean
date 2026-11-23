package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"mysecondwebgame/server/internal/store"
)

type attributePatchSnapshot struct {
	Attributes map[string]string
	Progress   map[string]string
}

type wsPatchMeta struct {
	Version int64  `json:"version"`
	Epoch   string `json:"epoch,omitempty"`
}

func newWSPatchEpoch() string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
}

func (s *Server) nextPatchVersionLocked(userID int64, eventType string) int64 {
	key := fmt.Sprintf("%d:%s", userID, strings.TrimSpace(eventType))
	next := s.patchVersion[key] + 1
	s.patchVersion[key] = next
	return next
}

func (s *Server) nextRolePatchVersionLocked(userID int64, eventType string, roleKey string) int64 {
	key := fmt.Sprintf("%d:%s:%s", userID, strings.TrimSpace(eventType), strings.TrimSpace(roleKey))
	next := s.patchVersion[key] + 1
	s.patchVersion[key] = next
	return next
}

func rolePatchKey(roleType string, roleID any) string {
	normalizedType := strings.TrimSpace(strings.ToLower(roleType))
	if normalizedType == "" || normalizedType == "player" {
		return "player:self"
	}
	return fmt.Sprintf("%s:%v", normalizedType, roleID)
}

func (s *Server) resetRealtimePatchState(userID int64) {
	if userID <= 0 {
		return
	}
	lock := &s.inventoryPatchMu[uint64(userID)%uint64(len(s.inventoryPatchMu))]
	lock.Lock()
	defer lock.Unlock()
	s.patchMu.Lock()
	defer s.patchMu.Unlock()
	delete(s.inventorySnap, userID)
	delete(s.inventoryPatchEpoch, userID)
	delete(s.equipmentSnap, userID)
	delete(s.attrSnap, userID)
	delete(s.patchUpdatedAt, userID)
	prefix := fmt.Sprintf("%d:", userID)
	for key := range s.patchVersion {
		if strings.HasPrefix(key, prefix) {
			delete(s.patchVersion, key)
		}
	}
	for key := range s.roleEquipmentSnap {
		if strings.HasPrefix(key, prefix) {
			delete(s.roleEquipmentSnap, key)
		}
	}
	for key := range s.roleAttrSnap {
		if strings.HasPrefix(key, prefix) {
			delete(s.roleAttrSnap, key)
		}
	}
}

func marshalStable(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

const inventoryInlinePatchMaxBytes = 256 << 10

func (s *Server) pushInventoryInstancesPatch(userID int64, items []store.ItemInstance) wsPatchMeta {
	if userID <= 0 {
		return wsPatchMeta{}
	}
	lock := &s.inventoryPatchMu[uint64(userID)%uint64(len(s.inventoryPatchMu))]
	lock.Lock()
	defer lock.Unlock()
	current := make(map[int64]string, len(items))
	upsert := make([]store.ItemInstance, 0)
	remove := make([]int64, 0)
	// Large inventories must not serialize JSON while holding every user's
	// patch lock. The striped lock preserves this user's publication order.
	for _, item := range items {
		if item.InstanceID > 0 {
			current[item.InstanceID] = marshalStable(item)
		}
	}

	s.patchMu.Lock()
	prev := s.inventorySnap[userID]
	if prev == nil {
		if s.inventoryPatchEpoch == nil {
			s.inventoryPatchEpoch = make(map[int64]string)
		}
		// Cache eviction/rebind resets the version too, not just process restart.
		s.inventoryPatchEpoch[userID] = newWSPatchEpoch()
	}
	epoch := s.inventoryPatchEpoch[userID]
	bytes := 128
	for _, item := range items {
		id := item.InstanceID
		if id <= 0 {
			continue
		}
		encoded := current[id]
		if prev == nil || prev[id] != encoded {
			upsert = append(upsert, item)
			bytes += len(encoded) + 1
		}
	}
	for id := range prev {
		if _, ok := current[id]; !ok {
			remove = append(remove, id)
			bytes += 21
		}
	}
	if prev != nil && len(upsert) == 0 && len(remove) == 0 {
		meta := wsPatchMeta{Version: s.patchVersion[fmt.Sprintf("%d:%s", userID, inventoryInstancesUpdatedEventType)], Epoch: epoch}
		s.patchMu.Unlock()
		return meta
	}
	s.inventorySnap[userID] = current
	s.patchUpdatedAt[userID] = time.Now()
	version := s.nextPatchVersionLocked(userID, "inventory.instances.updated")
	s.patchMu.Unlock()
	meta := wsPatchMeta{Version: version, Epoch: epoch}
	s.pushUserEventCluster(userID, inventoryInstancesUpdatedEventType, inventoryPatchPayload(upsert, remove, meta, bytes, prev == nil && len(items) == 0))
	return meta
}

func inventoryPatchPayload(upsert []store.ItemInstance, remove []int64, meta wsPatchMeta, bytes int, emptyBaseline bool) any {
	if bytes > inventoryInlinePatchMaxBytes || emptyBaseline {
		return inventoryRefreshRequiredPayload{RefreshRequired: true, Meta: meta}
	}
	sort.Slice(upsert, func(i, j int) bool { return upsert[i].InstanceID < upsert[j].InstanceID })
	sort.Slice(remove, func(i, j int) bool { return remove[i] < remove[j] })
	return fiber.Map{
		"upsert": upsert,
		"remove": remove,
		"meta":   meta,
	}
}

func (s *Server) pushEquipmentInstancesPatch(userID int64, items []store.EquippedItem) (fiber.Map, bool) {
	if userID <= 0 {
		return nil, false
	}
	return s.pushRoleEquipmentPatch(userID, "equipment.instances.updated", "player", "self", items)
}

func (s *Server) pushEquipmentInstancesSnapshot(userID int64, items []store.EquippedItem) fiber.Map {
	if userID <= 0 {
		return nil
	}
	return s.pushRoleEquipmentSnapshot(userID, "equipment.instances.updated", "player", "self", items)
}

func (s *Server) pushKittyEquipmentPatch(userID int64, kittyID int64, items []store.EquippedItem) (fiber.Map, bool) {
	if userID <= 0 || kittyID <= 0 {
		return nil, false
	}
	return s.pushRoleEquipmentPatch(userID, "kitty.equipment.updated", "kitty", kittyID, items)
}

func (s *Server) pushKittyEquipmentSnapshot(userID int64, kittyID int64, items []store.EquippedItem) fiber.Map {
	if userID <= 0 || kittyID <= 0 {
		return nil
	}
	return s.pushRoleEquipmentSnapshot(userID, "kitty.equipment.updated", "kitty", kittyID, items)
}

func (s *Server) pushRoleEquipmentPatch(userID int64, eventType string, roleType string, roleID any, items []store.EquippedItem) (fiber.Map, bool) {
	if userID <= 0 {
		return nil, false
	}
	current := make(map[int64]string, len(items))
	upsert := make([]store.EquippedItem, 0)
	remove := make([]int64, 0)
	roleKey := rolePatchKey(roleType, roleID)
	cacheKey := fmt.Sprintf("%d:%s", userID, roleKey)

	s.patchMu.Lock()
	prev := s.roleEquipmentSnap[cacheKey]
	if prev == nil && roleKey == "player:self" {
		prev = s.equipmentSnap[userID]
	}
	for _, item := range items {
		id := item.InstanceID
		if id <= 0 {
			continue
		}
		encoded := marshalStable(item)
		current[id] = encoded
		if prev == nil || prev[id] != encoded {
			upsert = append(upsert, item)
		}
	}
	for id := range prev {
		if _, ok := current[id]; !ok {
			remove = append(remove, id)
		}
	}
	if len(upsert) == 0 && len(remove) == 0 {
		s.patchMu.Unlock()
		return nil, false
	}
	s.roleEquipmentSnap[cacheKey] = current
	if roleKey == "player:self" {
		s.equipmentSnap[userID] = current
	}
	s.patchUpdatedAt[userID] = time.Now()
	version := s.nextRolePatchVersionLocked(userID, eventType, roleKey)
	s.patchMu.Unlock()

	sort.Slice(upsert, func(i, j int) bool { return upsert[i].InstanceID < upsert[j].InstanceID })
	sort.Slice(remove, func(i, j int) bool { return remove[i] < remove[j] })
	payload := fiber.Map{
		"roleType": strings.TrimSpace(strings.ToLower(roleType)),
		"roleId":   roleID,
		"upsert":   upsert,
		"remove":   remove,
		"meta":     wsPatchMeta{Version: version, Epoch: s.patchEpoch},
	}
	s.pushUserEventCluster(userID, eventType, payload)
	return payload, true
}

func (s *Server) pushRoleEquipmentSnapshot(userID int64, eventType string, roleType string, roleID any, items []store.EquippedItem) fiber.Map {
	if userID <= 0 {
		return nil
	}
	current := make(map[int64]string, len(items))
	sortedItems := make([]store.EquippedItem, 0, len(items))
	for _, item := range items {
		if item.InstanceID <= 0 {
			continue
		}
		current[item.InstanceID] = marshalStable(item)
		sortedItems = append(sortedItems, item)
	}
	roleKey := rolePatchKey(roleType, roleID)
	cacheKey := fmt.Sprintf("%d:%s", userID, roleKey)

	s.patchMu.Lock()
	s.roleEquipmentSnap[cacheKey] = current
	if roleKey == "player:self" {
		s.equipmentSnap[userID] = current
	}
	s.patchUpdatedAt[userID] = time.Now()
	version := s.nextRolePatchVersionLocked(userID, eventType, roleKey)
	s.patchMu.Unlock()

	sort.Slice(sortedItems, func(i, j int) bool { return sortedItems[i].InstanceID < sortedItems[j].InstanceID })
	payload := fiber.Map{
		"roleType": strings.TrimSpace(strings.ToLower(roleType)),
		"roleId":   roleID,
		"items":    sortedItems,
		"meta":     wsPatchMeta{Version: version, Epoch: s.patchEpoch},
	}
	s.pushUserEventCluster(userID, eventType, payload)
	return payload
}

func (s *Server) pushPlayerAttributesPatch(userID int64, attributes fiber.Map, progress fiber.Map) (fiber.Map, bool) {
	if userID <= 0 {
		return nil, false
	}
	return s.pushRoleAttributesPatch(userID, "player.attributes.updated", "player", "self", attributes, progress)
}

func (s *Server) pushKittyAttributesPatch(userID int64, kittyID int64, attributes fiber.Map, progress fiber.Map) (fiber.Map, bool) {
	if userID <= 0 || kittyID <= 0 {
		return nil, false
	}
	return s.pushRoleAttributesPatch(userID, "kitty.attributes.updated", "kitty", kittyID, attributes, progress)
}

func (s *Server) pushRoleAttributesPatch(userID int64, eventType string, roleType string, roleID any, attributes fiber.Map, progress fiber.Map) (fiber.Map, bool) {
	if userID <= 0 {
		return nil, false
	}
	currentAttributes := map[string]string{}
	currentProgress := map[string]string{}
	upsert := fiber.Map{}
	remove := make([]string, 0)
	progressUpsert := fiber.Map{}
	progressRemove := make([]string, 0)

	roleKey := rolePatchKey(roleType, roleID)
	cacheKey := fmt.Sprintf("%d:%s", userID, roleKey)

	s.patchMu.Lock()
	prev, ok := s.roleAttrSnap[cacheKey]
	if !ok {
		if roleKey == "player:self" {
			if legacyPrev, legacyOK := s.attrSnap[userID]; legacyOK {
				prev = legacyPrev
				ok = true
			}
		}
		if !ok {
			prev = attributePatchSnapshot{
				Attributes: map[string]string{},
				Progress:   map[string]string{},
			}
		}
	}
	for key, node := range attributes {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		encoded := marshalStable(node)
		currentAttributes[normalized] = encoded
		if prev.Attributes[normalized] != encoded {
			upsert[normalized] = node
		}
	}
	for key := range prev.Attributes {
		if _, exists := currentAttributes[key]; !exists {
			remove = append(remove, key)
		}
	}
	for key, node := range progress {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		encoded := marshalStable(node)
		currentProgress[normalized] = encoded
		if prev.Progress[normalized] != encoded {
			progressUpsert[normalized] = node
		}
	}
	for key := range prev.Progress {
		if _, exists := currentProgress[key]; !exists {
			progressRemove = append(progressRemove, key)
		}
	}
	if len(upsert) == 0 && len(remove) == 0 && len(progressUpsert) == 0 && len(progressRemove) == 0 {
		s.patchMu.Unlock()
		return nil, false
	}
	nextSnapshot := attributePatchSnapshot{
		Attributes: currentAttributes,
		Progress:   currentProgress,
	}
	s.roleAttrSnap[cacheKey] = nextSnapshot
	if roleKey == "player:self" {
		s.attrSnap[userID] = nextSnapshot
	}
	s.patchUpdatedAt[userID] = time.Now()
	version := s.nextRolePatchVersionLocked(userID, eventType, roleKey)
	s.patchMu.Unlock()

	sort.Strings(remove)
	sort.Strings(progressRemove)
	payload := fiber.Map{
		"roleType":                strings.TrimSpace(strings.ToLower(roleType)),
		"roleId":                  roleID,
		"upsert":                  upsert,
		"remove":                  remove,
		"attributeProgressUpsert": progressUpsert,
		"attributeProgressRemove": progressRemove,
		"meta":                    wsPatchMeta{Version: version, Epoch: s.patchEpoch},
	}
	s.pushUserEventCluster(userID, eventType, payload)
	return payload, true
}
