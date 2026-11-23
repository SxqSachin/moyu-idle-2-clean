package httpapi

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/websocket/v2"
	"mysecondwebgame/server/internal/app"
	"mysecondwebgame/server/internal/config"
	"mysecondwebgame/server/internal/store"
	gamews "mysecondwebgame/server/internal/ws"
)

func TestOfflineRecoveryTerminalInventorySync(t *testing.T) {
	raw, err := os.ReadFile("offline_fastforward_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	start := strings.Index(source, "func (s *Server) syncOfflineRecoveryDomains")
	if start < 0 {
		t.Fatal("missing terminal projection")
	}
	body := source[start:]
	read := strings.Index(body, "s.listInventoryInstances(ctx, userID, false)")
	push := strings.Index(body, "s.pushInventoryInstancesPatch(userID, items)")
	if read < 0 || push <= read || !strings.Contains(body[read:push], "return wsPatchMeta{}, err") {
		t.Fatal("terminal recovery must refresh uncached committed inventory and propagate read errors")
	}
}

func TestOfflineRecoveryCombinedReportProtocol(t *testing.T) {
	request := parseOfflineFastForwardRequest(json.RawMessage(`{"protocolVersion":3,"reportPolicy":"combined","syncedReportKey":"members-v2","ackReportKey":"digest","ackTasks":[{"taskId":"9007199254740993","revision":2},{"taskId":"9007199254740994","revision":3}]}`))
	if request.ReportPolicy != "combined" || request.SyncedReportKey != "members-v2" || request.AckReportKey != "digest" || len(request.AckTasks) != 2 || request.AckTasks[0].TaskID != "9007199254740993" {
		t.Fatal("combined acknowledgement lost exact task identity", request)
	}
	response := newOfflineFastForwardResponse(store.ActionFastForwardSummary{}, offlineFastForwardResponseContext{})
	response.ReportKey, response.ReportTasks = request.SyncedReportKey, request.AckTasks
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded offlineFastForwardResponse
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded.ReportKey != response.ReportKey || len(decoded.ReportTasks) != 2 {
		t.Fatal("report membership lost in serialization", decoded, err)
	}
	if old := parseOfflineFastForwardRequest(json.RawMessage(`{"ackTaskId":"15","ackRevision":2}`)); old.ReportPolicy != "" || len(old.AckTasks) != 0 || old.AckTaskID != "15" {
		t.Fatal("old client no longer uses single-task protocol", old)
	}
}

func TestOfflineRecoveryStalledResponseDefersProjection(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		summary := store.ActionFastForwardSummary{
			CompletedSteps: 3, HasMoreDue: !stalled, TargetAtMS: 200, SettledUntilMS: 100,
			ItemRewards: map[string]int64{"wood": 3}, OperationEffectsChanged: true,
		}
		if stalled {
			summary.YieldReason = store.ActionQueueYieldReasonNoProgress
			summary.StalledQueueID = 55
		}
		response := newOfflineFastForwardResponse(compactOfflineRecoverySummary(summary), offlineFastForwardResponseContext{ProjectionDeferred: true})
		want := "progress"
		if stalled {
			want = "stalled"
		}
		if response.Status != want || response.HasMoreDue == stalled || response.SnapshotSynced || !response.ProjectionDeferred || len(response.ItemRewards) != 0 {
			t.Fatal("nonterminal task lost status or asserted terminal projection", response)
		}
		if response.CompletedSteps != 3 || response.StalledQueueID != summary.StalledQueueID || response.TargetAtMS != 200 || response.SettledUntilMS != 100 {
			t.Fatal("compact response lost recovery metadata", response)
		}
	}
	raw, err := os.ReadFile("offline_fastforward_handler.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `terminal := recovery.Status == "complete"`) {
		t.Fatal("stalled recovery must not depend on successful terminal reward/projection reads")
	}
}

func TestOfflineRecoverySnapshotIncludesLateBattleSettlement(t *testing.T) {
	request := parseOfflineFastForwardRequest(json.RawMessage(`{"syncedTaskId":"15","syncedRevision":3,"syncedCombatPendingRuns":1}`))
	if !request.hasSyncedRecoverySnapshot("15", 3, 1) {
		t.Fatal("unchanged polling must not reload the entire inventory")
	}
	for _, row := range []struct {
		task     string
		revision int64
		pending  int
	}{{"15", 3, 0}, {"15", 4, 1}, {"16", 3, 1}} {
		if request.hasSyncedRecoverySnapshot(row.task, row.revision, row.pending) {
			t.Fatal("late battle, action revision or new absence reused an old inventory snapshot", row)
		}
	}
}

func TestOfflineRecoveryPlayerProjection(t *testing.T) {
	for _, row := range []struct {
		name                  string
		summary               store.ActionFastForwardSummary
		equipment, attributes bool
	}{
		{name: "empty receipt"},
		{name: "player experience", summary: store.ActionFastForwardSummary{AttrExp: map[string]float64{"collecting": 1}}, attributes: true},
		{name: "unchanged final loadout", summary: store.ActionFastForwardSummary{FinalWorkLoadoutID: "collecting"}},
		{name: "changed final loadout", summary: store.ActionFastForwardSummary{FinalWorkLoadoutID: "collecting", ActorSummaries: []store.ActionActorSummary{{ActorType: store.ActionActorTypePlayer, EquipmentChanged: true}}}, equipment: true, attributes: true},
		{name: "player enhancement", summary: store.ActionFastForwardSummary{ActorSummaries: []store.ActionActorSummary{{ActorType: store.ActionActorTypePlayer, EquipmentChanged: true}}}, equipment: true, attributes: true},
		{name: "enhancement receipt without actor flag", summary: store.ActionFastForwardSummary{EnhanceResults: []store.EnhanceAttemptResult{{EquipmentInstanceID: 9, AfterLevel: 2}}}, equipment: true, attributes: true},
		{name: "kitty equipment only", summary: store.ActionFastForwardSummary{ActorSummaries: []store.ActionActorSummary{{ActorType: store.ActionActorTypeKitty, EquipmentChanged: true}}}},
		{name: "kitty owner experience", summary: store.ActionFastForwardSummary{ActorSummaries: []store.ActionActorSummary{{ActorType: store.ActionActorTypeKitty, OwnerAttrExp: map[string]float64{"fishing": 2}}}}, attributes: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			equipment, attributes := offlineRecoveryPlayerProjection(row.summary)
			if equipment != row.equipment || attributes != row.attributes {
				t.Fatalf("projection=(%v,%v), want (%v,%v)", equipment, attributes, row.equipment, row.attributes)
			}
		})
	}
}

func TestOfflineRecoveryHTTPMutationGate(t *testing.T) {
	for _, row := range []struct {
		method, path string
		blocked      bool
	}{
		{"GET", "/api/inventory/instances", false},
		{"POST", "/api/player/presence", false},
		{"POST", "/api/chat/send", false},
		{"POST", "/api/market/instances/buy", true},
		{"DELETE", "/api/inventory/instances/1", true},
		{"POST", "/api/action/run", true},
	} {
		if got := recoveryBlocksHTTPMutation(row.method, row.path); got != row.blocked {
			t.Errorf("%s %s: blocked=%v", row.method, row.path, got)
		}
	}
}

func TestOfflineFastForwardProjectionPolicy(t *testing.T) {
	legacy := parseOfflineFastForwardRequest(nil)
	terminal := parseOfflineFastForwardRequest(json.RawMessage(`{"protocolVersion":3,"projectionPolicy":"terminal"}`))
	unknown := parseOfflineFastForwardRequest(json.RawMessage(`{"protocolVersion":4,"projectionPolicy":"terminal"}`))

	progress := store.ActionFastForwardSummary{HasMoreDue: true}
	complete := store.ActionFastForwardSummary{}
	stalled := store.ActionFastForwardSummary{
		HasMoreDue:  true,
		YieldReason: store.ActionQueueYieldReasonNoProgress,
	}

	if !shouldLoadOfflineFastForwardProjection(legacy, progress) {
		t.Fatal("legacy clients must retain per-chunk projection sync")
	}
	if shouldLoadOfflineFastForwardProjection(terminal, progress) {
		t.Fatal("versioned terminal policy should defer intermediate projection sync")
	}
	for name, summary := range map[string]store.ActionFastForwardSummary{
		"executor barrier":  {HasMoreDue: true, RequiresImmediateProjection: true},
		"operation effects": {HasMoreDue: true, OperationEffectsChanged: true},
		"fishing":           {HasMoreDue: true, FishingChanged: true},
		"enhance":           {HasMoreDue: true, EnhanceResults: []store.EnhanceAttemptResult{{QueueID: 1}}},
		"player equipment": {
			HasMoreDue: true,
			ActorSummaries: []store.ActionActorSummary{{
				ActorType:        store.ActionActorTypePlayer,
				EquipmentChanged: true,
			}},
		},
		"kitty attributes": {
			HasMoreDue: true,
			ActorSummaries: []store.ActionActorSummary{{
				ActorType: store.ActionActorTypeKitty,
				ActorID:   9,
				AttrExp:   map[string]float64{"dexterity": 1},
			}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if !shouldLoadOfflineFastForwardProjection(terminal, summary) {
				t.Fatal("specialized domain changes must sync before their invalidations are discarded")
			}
		})
	}
	if !shouldLoadOfflineFastForwardProjection(terminal, complete) {
		t.Fatal("terminal policy must load the final projection")
	}
	if !shouldLoadOfflineFastForwardProjection(terminal, stalled) {
		t.Fatal("terminal policy must load projection when automatic continuation stops")
	}
	if !shouldLoadOfflineFastForwardProjection(unknown, progress) {
		t.Fatal("unknown protocol versions must fall back to per-chunk projection sync")
	}
}

func TestOfflineFastForwardResultStatus(t *testing.T) {
	tests := []struct {
		name        string
		hasMoreDue  bool
		syncPending bool
		yieldReason string
		want        string
	}{
		{name: "complete", want: "complete"},
		{name: "progress", hasMoreDue: true, want: "progress"},
		{name: "committed but projection pending", syncPending: true, want: "committed_sync_pending"},
		{name: "progress committed but projection pending", hasMoreDue: true, syncPending: true, want: "committed_sync_pending"},
		{name: "stalled takes precedence", hasMoreDue: true, syncPending: true, yieldReason: store.ActionQueueYieldReasonNoProgress, want: "stalled"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := offlineFastForwardResultStatus(tc.hasMoreDue, tc.syncPending, tc.yieldReason); got != tc.want {
				t.Fatalf("offlineFastForwardResultStatus(%t, %t, %q) = %q, want %q", tc.hasMoreDue, tc.syncPending, tc.yieldReason, got, tc.want)
			}
		})
	}
}

func TestNewOfflineFastForwardResponseKeepsProtocolAndRetrySemanticsTogether(t *testing.T) {
	summary := store.ActionFastForwardSummary{
		CompletedSteps: 19,
		HasMoreDue:     true,
		YieldReason:    store.ActionQueueYieldReasonNoProgress,
		StalledQueueID: 77,
		TargetAtMS:     123456,
	}
	response := newOfflineFastForwardResponse(summary, offlineFastForwardResponseContext{
		RequestID:              "ws-123",
		OfflineDurationSeconds: 3600,
		CombatItemRewards:      map[string]int64{"iron": 2},
		SnapshotSynced:         true,
	})

	if response.ProtocolVersion != offlineFastForwardProtocolVersion || response.RequestID != "ws-123" {
		t.Fatalf("unexpected protocol correlation: %+v", response)
	}
	if response.Status != "stalled" || response.HasMoreDue {
		t.Fatalf("stalled response must suppress client retry: %+v", response)
	}
	if response.CompletedSteps != 19 || response.StalledQueueID != 77 || response.TargetAtMS != 123456 {
		t.Fatalf("summary fields were not preserved: %+v", response)
	}
}

func TestNewOfflineFastForwardResponseMarksPreCommitFailure(t *testing.T) {
	response := newOfflineFastForwardResponse(store.ActionFastForwardSummary{}, offlineFastForwardResponseContext{
		RequestID:              "ws-failed",
		OfflineDurationSeconds: 1800,
		ErrorCode:              " WS_ACTION_QUEUE_TIMEOUT ",
	})

	if response.Status != "failed_before_commit" || response.ErrorCode != "WS_ACTION_QUEUE_TIMEOUT" {
		t.Fatalf("unexpected failure response: %+v", response)
	}
	if response.RequestID != "ws-failed" || response.OfflineDurationSeconds != 1800 {
		t.Fatalf("failure response lost correlation context: %+v", response)
	}
}

func TestOfflineFastForwardCanConsumeMarker(t *testing.T) {
	complete := store.ActionFastForwardSummary{TargetAtMS: 123}
	if !offlineFastForwardCanConsumeMarker(complete, 123) {
		t.Fatal("completed settlement should consume its marker")
	}
	if offlineFastForwardCanConsumeMarker(store.ActionFastForwardSummary{TargetAtMS: 123, HasMoreDue: true}, 123) {
		t.Fatal("partial settlement must retain its marker")
	}
	if offlineFastForwardCanConsumeMarker(store.ActionFastForwardSummary{TargetAtMS: 123, YieldReason: store.ActionQueueYieldReasonNoProgress}, 123) {
		t.Fatal("stalled settlement must retain its marker")
	}
	if offlineFastForwardCanConsumeMarker(complete, 456) {
		t.Fatal("a settlement for another frozen target must retain this marker")
	}
}

func TestOfflineFastForwardClientHasMoreDue(t *testing.T) {
	if !offlineFastForwardClientHasMoreDue(store.ActionFastForwardSummary{HasMoreDue: true}) {
		t.Fatal("bounded progress should request another client chunk")
	}
	if offlineFastForwardClientHasMoreDue(store.ActionFastForwardSummary{
		HasMoreDue:  true,
		YieldReason: store.ActionQueueYieldReasonNoProgress,
	}) {
		t.Fatal("stalled settlement must suppress automatic retries for legacy clients")
	}
}

func TestOfflineFastForwardMutationCatalogOnlyShipsAtClientTerminal(t *testing.T) {
	legacy := offlineFastForwardRequest{}
	terminal := offlineFastForwardRequest{
		ProtocolVersion:  offlineFastForwardProtocolVersion,
		ProjectionPolicy: offlineFastForwardProjectionPolicyTerminal,
	}
	if shouldIncludeOfflineFastForwardMutationCatalog(terminal, store.ActionFastForwardSummary{HasMoreDue: true}) {
		t.Fatal("continuation chunks must not repeat the static mutation catalog")
	}
	if !shouldIncludeOfflineFastForwardMutationCatalog(terminal, store.ActionFastForwardSummary{}) {
		t.Fatal("complete responses must include the mutation catalog")
	}
	if !shouldIncludeOfflineFastForwardMutationCatalog(terminal, store.ActionFastForwardSummary{
		HasMoreDue:  true,
		YieldReason: store.ActionQueueYieldReasonNoProgress,
	}) {
		t.Fatal("stalled responses must include terminal display metadata")
	}
	if !shouldIncludeOfflineFastForwardMutationCatalog(legacy, store.ActionFastForwardSummary{HasMoreDue: true}) {
		t.Fatal("legacy clients must retain the original per-chunk catalog shape")
	}
}

func TestOfflineFastForwardEnhanceEventsMatchClientHistoryWindow(t *testing.T) {
	rows := make([]store.EnhanceAttemptResult, 0, 140)
	for attempt := 1; attempt <= 140; attempt++ {
		rows = append(rows, store.EnhanceAttemptResult{AttemptNo: attempt})
	}
	got := offlineFastForwardEnhanceAttemptEvents(rows)
	if len(got) != offlineFastForwardEnhanceEventLimit {
		t.Fatalf("event count=%d, want %d", len(got), offlineFastForwardEnhanceEventLimit)
	}
	if got[0].AttemptNo != 41 || got[len(got)-1].AttemptNo != 140 {
		t.Fatalf("event window=%d..%d, want 41..140", got[0].AttemptNo, got[len(got)-1].AttemptNo)
	}
	if got := offlineFastForwardEnhanceAttemptEvents(rows[:3]); len(got) != 3 || got[0].AttemptNo != 1 {
		t.Fatalf("short event window changed: %+v", got)
	}
}

func TestEnqueueWSMessageReportsCompressionFallback(t *testing.T) {
	hub := gamews.NewHub()
	conn := &websocket.Conn{}
	hub.Register(conn)
	s := &Server{deps: app.Dependencies{
		Hub: hub,
		Config: config.Config{GlobalSettings: config.GlobalSettings{
			Transport: config.TransportSettings{Compression: config.CompressionSettings{Enabled: true}},
		}},
	}}

	if err := s.enqueueWSMessage(conn, map[string]any{"unsupported": func() {}}); err == nil {
		t.Fatal("compression fallback must report that the requested payload was not enqueued")
	}
}
