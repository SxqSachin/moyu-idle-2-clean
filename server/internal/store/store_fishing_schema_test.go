package store

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestFishingInitSchemaIncludesCurrentMigrationState(t *testing.T) {
	assertForwardMigrationManifest(t, "fishing", []string{
		"20260831_fishing_v1.sql",
		"20260901_fishing_kitty_anglers.sql",
		"20260901_fishing_pond_v1.sql",
		"20260901_fishing_tackle_wear.sql",
		"20260901_fishing_v1_1.sql",
		"20260902_fishing_fish_customization.sql",
		"20260903_fishing_trip_event_history.sql",
		"20260903_fishing_pond_v5_cutover.sql",
		"20260903_fishing_v2_continuity.sql",
	})

	raw, err := os.ReadFile("../../schema/init.sql")
	if err != nil {
		t.Fatalf("read init schema: %v", err)
	}
	text := strings.ToLower(string(raw))
	compact := strings.Join(strings.Fields(text), " ")

	for _, table := range []string{
		"player_one_time_grants",
		"fishing_loadouts",
		"fishing_spot_pressure",
		"fishing_tackle_wear",
		"fishing_trips",
		"fishing_trip_events",
		"fishing_catches",
		"fishing_species_knowledge",
		"fishing_records",
		"fishing_tournaments",
		"fishing_tournament_entries",
		"fishing_tournament_rewards",
		"fishing_player_wallets",
		"fishing_token_redemptions",
		"fishing_npc_supply",
		"fishing_npc_sales",
		"fishing_ponds",
		"fishing_pond_pairs",
		"fishing_pond_events",
	} {
		if !strings.Contains(compact, "create table if not exists "+table) {
			t.Fatalf("init schema missing current fishing table %s", table)
		}
	}

	for _, required := range []string{
		"add column if not exists fish_data jsonb not null default '{}'::jsonb",
		"chk_player_item_instances_fish_quantity",
		"chk_player_item_instances_fish_shape",
		"actor_type text not null default 'player' check(actor_type in ('player', 'kitty'))",
		"actor_id bigint not null default 0 check((actor_type = 'player' and actor_id = 0) or (actor_type = 'kitty' and actor_id > 0))",
		"supplies_jsonb jsonb not null default '{}'::jsonb",
		"wear double precision not null default 0 check(wear >= 0 and wear < 1)",
		"equip_type text not null check(equip_type in ('battle', 'work'))",
		"create unique index if not exists uq_player_kitty_equip_slots_instance on player_kitty_equip_slots(instance_id)",
		"source_kind text not null default 'catch' check (source_kind in ('catch', 'cultured'))",
		"instance_fish_data jsonb not null default '{}'::jsonb",
		"ruleset_version text not null default 'fishing-pond-v5'",
		"chk_fishing_ponds_ruleset_version",
		"chk_player_item_instances_fishing_pond_zone",
		"coalesce(fish_data->>'pondzone', '') in ('main', 'nursery')",
	} {
		if !strings.Contains(compact, required) {
			t.Fatalf("init schema missing current fishing contract %q", required)
		}
	}
	for _, index := range []string{
		"idx_player_item_instances_user_fish",
		"idx_fishing_loadouts_user_updated",
		"uq_fishing_trips_user_running",
		"idx_fishing_trips_user_started",
		"idx_fishing_trips_queue",
		"idx_fishing_trips_spot_running",
		"idx_fishing_catches_user_caught",
		"idx_fishing_catches_user_id",
		"idx_fishing_catches_inventory_instance",
		"idx_fishing_catches_species_length",
		"idx_fishing_catches_species_weight",
		"idx_fishing_records_length_board",
		"idx_fishing_records_weight_board",
		"idx_fishing_records_best_length_catch",
		"idx_fishing_records_best_weight_catch",
		"uq_fishing_tournaments_week_ruleset",
		"idx_fishing_tournament_entries_board",
		"idx_fishing_tournament_entries_catch",
		"idx_fishing_tournament_entries_fish_instance",
		"idx_fishing_tournament_rewards_user_created",
		"idx_fishing_npc_sales_user_created",
		"idx_fishing_pond_events_user_created",
		"idx_player_item_instances_user_pond_fish",
	} {
		if !strings.Contains(compact, index) {
			t.Fatalf("init schema missing current fishing index %s", index)
		}
	}

	if strings.Contains(compact, "on player_kitty_equip_slots(instance_id) where equip_type = 'battle'") {
		t.Fatal("init schema still limits kitty equipment instance uniqueness to battle gear")
	}
	if strings.Contains(compact, "alter table fishing_trips add column if not exists supplies_jsonb") {
		t.Fatal("fresh init schema must define fishing trip supplies directly instead of replaying an upgrade migration")
	}
	assertFishShapeAllowsCustomization(t, compact, "init schema")
}

func TestFishingPondV5CutoverMigrationPreservesLegacySettlementBoundary(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260903_fishing_pond_v5_cutover.sql")
	if err != nil {
		t.Fatalf("read fishing pond V5 cutover migration: %v", err)
	}
	compact := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
	for _, required := range []string{
		"begin; set local lock_timeout = '5s'",
		"add column if not exists ruleset_version text",
		"set ruleset_version = 'fishing-pond-v4'",
		"where ruleset_version is null or btrim(ruleset_version) = ''",
		"alter column ruleset_version set default 'fishing-pond-v5'",
		"chk_fishing_ponds_ruleset_version",
		"check (ruleset_version in ('fishing-pond-v4', 'fishing-pond-v5')) not valid",
		"jsonb_set(instance.fish_data, '{pondzone}', to_jsonb('main'::text), true)",
		"coalesce(instance.fish_data->>'pondzone', '') not in ('main', 'nursery')",
		"pond.ruleset_version = 'fishing-pond-v4'",
		"chk_player_item_instances_fishing_pond_zone",
		"coalesce(fish_data->>'pondzone', '') in ('main', 'nursery')",
		"validate constraint chk_player_item_instances_fishing_pond_zone",
	} {
		if !strings.Contains(compact, required) {
			t.Fatalf("fishing pond V5 cutover migration missing %q", required)
		}
	}
	if strings.Contains(compact, "delete from player_item_instances") {
		t.Fatal("fishing pond V5 cutover migration must not delete player fish")
	}

	downRaw, err := os.ReadFile("../../schema/migrations/20260903_fishing_pond_v5_cutover_down.sql")
	if err != nil {
		t.Fatalf("read fishing pond V5 cutover down migration: %v", err)
	}
	downCompact := strings.Join(strings.Fields(strings.ToLower(string(downRaw))), " ")
	for _, required := range []string{
		"begin; set local lock_timeout = '5s'",
		"fish_data->>'pondzone' = 'nursery'",
		"raise exception 'cannot roll back fishing pond v5 while nursery fish exist'",
		"drop constraint if exists chk_player_item_instances_fishing_pond_zone",
		"fish_data = fish_data - 'pondzone'",
		"drop column if exists ruleset_version",
	} {
		if !strings.Contains(downCompact, required) {
			t.Fatalf("fishing pond V5 cutover down migration missing %q", required)
		}
	}
	if strings.Index(downCompact, "raise exception") > strings.Index(downCompact, "drop constraint if exists chk_player_item_instances_fishing_pond_zone") {
		t.Fatal("fishing pond V5 rollback must refuse nursery loss before dropping its guard")
	}

	checkRaw, err := os.ReadFile("../../schema/migrations/checks/20260903_fishing_pond_v5_cutover_check.sql")
	if err != nil {
		t.Fatalf("read fishing pond V5 cutover check: %v", err)
	}
	checkCompact := strings.Join(strings.Fields(strings.ToLower(string(checkRaw))), " ")
	for _, required := range []string{
		"unsupported_ruleset_rows",
		"invalid_pond_zone_rows",
		"legacy_pending_rows",
		"nursery_fish_rows",
	} {
		if !strings.Contains(checkCompact, required) {
			t.Fatalf("fishing pond V5 cutover check missing %q", required)
		}
	}
	for _, forbidden := range []string{"insert ", "update ", "delete ", "alter ", "drop "} {
		if strings.Contains(checkCompact, forbidden) {
			t.Fatalf("fishing pond V5 cutover check must stay read-only: found %q", forbidden)
		}
	}
}

func TestFishingV2ContinuityMigrationPreservesPlayerState(t *testing.T) {
	raw, err := os.ReadFile("../../schema/migrations/20260903_fishing_v2_continuity.sql")
	if err != nil {
		t.Fatalf("read fishing v2 continuity migration: %v", err)
	}
	compact := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
	for _, required := range []string{
		"where legacy.ruleset_version = 'fishing-v1'",
		"on conflict (user_id, species_id, ruleset_version) do update",
		"greatest(current_record.best_length_cm, excluded.best_length_cm)",
		"greatest(current_record.best_weight_kg, excluded.best_weight_kg)",
		"where week_key = '2026-08-31' and ruleset_version = 'fishing-v1'",
		"legacy_target_species_id <> 'emberfin'",
		"current_target_species_id = 'emberfin'",
		"set ruleset_version = 'fishing-v2-superseded-20260903'",
		"set ruleset_version = 'fishing-v2' where id = legacy_tournament_id",
	} {
		if !strings.Contains(compact, required) {
			t.Fatalf("fishing v2 continuity migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"update fishing_trips",
		"update fishing_catches",
		"update player_item_instances",
	} {
		if strings.Contains(compact, forbidden) {
			t.Fatalf("fishing v2 continuity migration must preserve provenance: found %q", forbidden)
		}
	}
}

func TestFishingFishCustomizationMigrationRelaxesFishShape(t *testing.T) {
	for _, path := range []string{
		"../../schema/migrations/20260902_fishing_fish_customization.sql",
		"../../schema/init.sql",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		compact := strings.Join(strings.Fields(strings.ToLower(string(raw))), " ")
		assertFishShapeAllowsCustomization(t, compact, path)
	}

	downRaw, err := os.ReadFile("../../schema/migrations/20260902_fishing_fish_customization_down.sql")
	if err != nil {
		t.Fatalf("read fish customization down migration: %v", err)
	}
	downCompact := strings.Join(strings.Fields(strings.ToLower(string(downRaw))), " ")
	downDefinition := fishShapeConstraintDefinition(t, downCompact, "fish customization down migration")
	if !strings.Contains(downDefinition, "custom_data = '{}'::jsonb") {
		t.Fatal("fish customization down migration must restore the previous custom_data restriction")
	}
}

func assertFishShapeAllowsCustomization(t *testing.T, sql, source string) {
	t.Helper()
	definition := fishShapeConstraintDefinition(t, sql, source)
	for _, required := range []string{
		"fish_data = '{}'::jsonb",
		"template_item_id = 'fish_catch'",
		"quantity = 1",
		"enhance_level = 0",
		"affixes = '[]'::jsonb",
	} {
		if !strings.Contains(definition, required) {
			t.Fatalf("%s fish shape constraint missing %q", source, required)
		}
	}
	if strings.Contains(definition, "custom_data") {
		t.Fatalf("%s fish shape constraint still forbids item customization", source)
	}
}

func fishShapeConstraintDefinition(t *testing.T, sql, source string) string {
	t.Helper()
	const marker = "add constraint chk_player_item_instances_fish_shape"
	start := strings.LastIndex(sql, marker)
	if start < 0 {
		t.Fatalf("%s missing fish shape constraint definition", source)
	}
	remainder := sql[start:]
	end := strings.Index(remainder, ") not valid;")
	if end < 0 {
		t.Fatalf("%s fish shape constraint is not declared not valid", source)
	}
	return remainder[:end]
}

func assertForwardMigrationManifest(t *testing.T, feature string, expected []string) {
	t.Helper()
	matches, err := filepath.Glob("../../schema/migrations/*" + feature + "*.sql")
	if err != nil {
		t.Fatalf("list %s migrations: %v", feature, err)
	}
	actual := make([]string, 0, len(matches))
	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasSuffix(name, "_down.sql") {
			continue
		}
		actual = append(actual, name)
	}
	sort.Strings(actual)
	sort.Strings(expected)
	if strings.Join(actual, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("%s migration manifest changed; reconcile the new final state into init.sql\nactual: %v\nexpected: %v", feature, actual, expected)
	}
}
