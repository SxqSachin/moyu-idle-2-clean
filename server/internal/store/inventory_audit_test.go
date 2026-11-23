package store

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestInventoryAuditEventInfersCategoryDirectionAndSubject(t *testing.T) {
	event := normalizeInventoryAuditEvent(InventoryAuditEvent{
		UserID:        12,
		EventType:     WarehouseChangeEventSystemToWarehouse,
		TemplateItem:  "wood",
		Quantity:      5,
		SourceSurface: "battle_room",
		TargetSurface: "inventory",
		Details: map[string]any{
			"roomId": int64(99),
			"type":   "finish_reward",
		},
	})

	if event.Category != InventoryAuditCategoryBattle {
		t.Fatalf("category=%q, want %q", event.Category, InventoryAuditCategoryBattle)
	}
	if event.Direction != InventoryAuditDirectionGain {
		t.Fatalf("direction=%q, want %q", event.Direction, InventoryAuditDirectionGain)
	}
	if event.SubjectType != "battle_room" || event.SubjectID != "99" {
		t.Fatalf("subject=(%q,%q), want battle_room/99", event.SubjectType, event.SubjectID)
	}
	if event.Items["wood"] != 5 {
		t.Fatalf("items[wood]=%d, want 5", event.Items["wood"])
	}
}

func TestInventoryAuditEventInfersAlchemyCategory(t *testing.T) {
	event := normalizeInventoryAuditEvent(InventoryAuditEvent{
		UserID:        12,
		EventType:     WarehouseChangeEventWarehouseOp,
		TemplateItem:  "gold_coin",
		Quantity:      20,
		SourceSurface: "inventory",
		TargetSurface: "alchemy_convert",
		Details: map[string]any{
			"queueId": int64(456),
			"type":    "alchemy_convert_gold",
		},
	})

	if event.Category != InventoryAuditCategoryAlchemy {
		t.Fatalf("category=%q, want %q", event.Category, InventoryAuditCategoryAlchemy)
	}
	if event.SubjectType != "alchemy" || event.SubjectID != "456" {
		t.Fatalf("subject=(%q,%q), want alchemy/456", event.SubjectType, event.SubjectID)
	}
}

func TestInventoryAuditBufferMergesQueueEvents(t *testing.T) {
	s := &Store{auditBuffers: make(map[pgx.Tx]*inventoryAuditBuffer)}
	var tx pgx.Tx
	s.registerInventoryAuditBuffer(tx)

	for i := 0; i < 2; i++ {
		s.recordInventoryAuditTx(tx, InventoryAuditEvent{
			UserID:      7,
			Category:    InventoryAuditCategoryActionQueue,
			Direction:   InventoryAuditDirectionGain,
			SubjectType: "queue",
			SubjectID:   "123",
			Items:       map[string]int64{"wood": 3},
			Details: map[string]any{
				"queueId": int64(123),
				"phase":   "online_settle",
			},
		})
	}

	events := s.takeInventoryAuditEvents(tx)
	if len(events) != 1 {
		t.Fatalf("events=%d, want 1", len(events))
	}
	if got := events[0].Items["wood"]; got != 6 {
		t.Fatalf("merged wood=%d, want 6", got)
	}
}

func TestInventoryAuditBufferKeepsOperationEffectOperationsSeparate(t *testing.T) {
	s := &Store{auditBuffers: make(map[pgx.Tx]*inventoryAuditBuffer)}
	var tx pgx.Tx
	s.registerInventoryAuditBuffer(tx)

	for _, operationKey := range []string{"action_queue:1:1:100", "action_queue:1:1:200"} {
		s.recordInventoryAuditTx(tx, InventoryAuditEvent{
			UserID:      7,
			Category:    InventoryAuditCategoryOperationEffect,
			Direction:   InventoryAuditDirectionGain,
			SubjectType: "operation_effect",
			SubjectID:   "test-extra-output",
			Items:       map[string]int64{"wood": 1},
			Details: map[string]any{
				"grantId":      int64(9),
				"operationKey": operationKey,
				"phase":        "offline_fastforward",
			},
		})
	}

	events := s.takeInventoryAuditEvents(tx)
	if len(events) != 2 {
		t.Fatalf("operation effect audit events=%d, want 2", len(events))
	}
}

func TestInventoryAuditBufferKeepsOperationEffectRangesSeparate(t *testing.T) {
	s := &Store{auditBuffers: make(map[pgx.Tx]*inventoryAuditBuffer)}
	var tx pgx.Tx
	s.registerInventoryAuditBuffer(tx)

	for _, rangeKey := range []string{"action_queue:1:1:100..action_queue:1:5:500", "action_queue:1:6:600..action_queue:1:10:1000"} {
		s.recordInventoryAuditTx(tx, InventoryAuditEvent{
			UserID:      7,
			Category:    InventoryAuditCategoryOperationEffect,
			Direction:   InventoryAuditDirectionGain,
			SubjectType: "operation_effect",
			SubjectID:   "test-extra-output",
			Items:       map[string]int64{"wood": 5},
			Details: map[string]any{
				"grantId":           int64(9),
				"operationRangeKey": rangeKey,
				"phase":             "offline_fastforward",
			},
		})
	}

	events := s.takeInventoryAuditEvents(tx)
	if len(events) != 2 {
		t.Fatalf("operation effect audit range events=%d, want 2", len(events))
	}
}
