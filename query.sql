SELECT pid,
       now() - query_start AS duration,
       state,
       left(query, 120) AS query
FROM pg_stat_activity
WHERE state != 'idle'
  AND query_start IS NOT NULL
ORDER BY duration DESC
LIMIT 20;
""pid"	"duration"	"state"	"query"
2794997	"00:00:00.269271"	"active"	"SELECT pg_advisory_xact_lock($1)"
2777365	"00:00:00.002223"	"idle in transaction"	"
			UPDATE player_attributes
			SET final_value = $3,
			    source_mods_jsonb = $4::jsonb,
			    lock_version = lock_v"
2777374	"00:00:00.002188"	"idle in transaction"	"
			INSERT INTO achievement_events(event_id, user_id, kind, counter_key, delta, payload, status, available_at)
			VALUES"
2782071	"00:00:00.001583"	"idle in transaction"	"
			UPDATE player_attributes
			SET final_value = $3,
			    source_mods_jsonb = $4::jsonb,
			    lock_version = lock_v"
2777368	"00:00:00.001533"	"idle in transaction"	"
			INSERT INTO player_attributes(
				user_id, attr_key, base_level, skill_point_level, attr_exp, source_mods_jsonb, fi"
2730151	"00:00:00.000256"	"idle in transaction"	"
		SELECT achievement_id, unlocked_at, completed_at, claimed_at, progress_value
		FROM player_achievement_states
		WHERE"
2787827	"00:00:00"	"active"	"SELECT pid,
       now() - query_start AS duration,
       state,
       left(query, 120) AS query
FROM pg_stat_activity"
2730142	"-00:00:00.000149"	"idle in transaction"	"
			UPDATE player_attributes
			SET final_value = $3,
			    source_mods_jsonb = $4::jsonb,
			    lock_version = lock_v"

SELECT relname,
       seq_scan,
       seq_tup_read,
       idx_scan,
       round(seq_tup_read::numeric / NULLIF(seq_scan, 0)) AS avg_rows_per_seqscan,
       n_live_tup
FROM pg_stat_user_tables
WHERE seq_scan > 0
ORDER BY seq_tup_read DESC
LIMIT 15;
"relname"	"seq_scan"	"seq_tup_read"	"idx_scan"	"avg_rows_per_seqscan"	"n_live_tup"
"battle_rooms"	14925	3575779	229903	240	367
"achievement_events"	20	1501202	101121	75060	101505
"player_collectibles"	7636	1469261	1	192	1
"player_kitties"	597	699822	185087	1172	1
"player_equip_slots"	62	512626	8031	8268	2
"market_listings"	20	382460	80404	19123	0
"battle_runs"	6	168848	944288	28141	28132
"market_auctions"	28	142738	35	5098	1
"player_kitty_equip_slots"	158	92746	438	587	0
"users"	16	24032	304316	1502	0
"risk_account_evidence"	1	5688	0	5688	0
"risk_account_links"	1	5496	228	5496	0
"risk_account_signals"	1	3747	1	3747	1
"player_rule_sets"	200	2800	0	14	0
"risk_capability_blocks"	1	1300	0	1300	0


SELECT relname,
       n_tup_ins,
       n_tup_upd,
       n_tup_del,
       n_tup_ins + n_tup_upd + n_tup_del AS total_writes
FROM pg_stat_user_tables
ORDER BY total_writes DESC
LIMIT 15;
"relname"	"n_tup_ins"	"n_tup_upd"	"n_tup_del"	"total_writes"
"player_attributes"	0	366500	0	366500
"achievement_events"	34079	67810	42151	144040
"battle_rooms"	0	77691	0	77691
"player_action_queue"	14	65918	31	65963
"player_achievement_states"	14	28338	0	28352
"battle_runs"	3911	15635	3895	23441
"player_kitty_attributes"	0	22321	0	22321
"battle_settlement_jobs"	3909	7826	3893	15628
"player_derived_stats_cache"	0	7804	0	7804
"battle_run_carry_states"	3907	0	3895	7802
"battle_run_payloads"	3907	0	3895	7802
"battle_skill_exp_jobs"	1772	3532	1773	7077
"player_skills"	0	3171	0	3171
"player_item_instances"	25	3060	21	3106
"player_warehouse_change_logs"	1308	1483	0	2791


SELECT checkpoints_timed,
       checkpoints_req,
       round(checkpoint_write_time / NULLIF(checkpoints_timed + checkpoints_req, 0)) AS avg_write_ms,
       buffers_checkpoint,
       buffers_backend,
       buffers_alloc
FROM pg_stat_bgwriter;
"checkpoints_timed"	"checkpoints_req"	"avg_write_ms"	"buffers_checkpoint"	"buffers_backend"	"buffers_alloc"
0	0		41557	140	1599


SELECT blks_hit,
       blks_read,
       round(blks_hit::numeric / NULLIF(blks_hit + blks_read, 0) * 100, 1) AS cache_hit_pct,
       tup_inserted,
       tup_updated,
       tup_deleted
FROM pg_stat_database
WHERE datname = current_database();
"blks_hit"	"blks_read"	"cache_hit_pct"	"tup_inserted"	"tup_updated"	"tup_deleted"
190727593	522472	99.7	240790	757851	242559


SELECT blocked.pid AS blocked_pid,
       blocking.pid AS blocking_pid,
       left(blocked.query, 80) AS blocked_query,
       left(blocking.query, 80) AS blocking_query,
       now() - blocked.query_start AS wait_duration
FROM pg_stat_activity blocked
JOIN pg_stat_activity blocking
  ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
ORDER BY wait_duration DESC;
"blocked_pid"	"blocking_pid"	"blocked_query"	"blocking_query"	"wait_duration"
2730142	2777369	"SELECT pg_advisory_xact_lock($1)"	"
			UPDATE player_action_queue
			SET payload = $2::jsonb,
			    updated_at = n"	"00:00:00.36881"


SELECT schemaname, relname AS tablename, indexrelname AS indexname, idx_scan
FROM pg_stat_user_indexes
WHERE idx_scan = 0
  AND indexrelname NOT LIKE '%pkey%'
ORDER BY relname;
"schemaname"	"tablename"	"indexname"	"idx_scan"
"public"	"achievement_events"	"idx_achievement_events_user_status"	0
"public"	"battle_room_member_units"	"uq_battle_room_member_units_kitty_active"	0
"public"	"battle_rooms"	"idx_battle_rooms_difficulty_level"	0
"public"	"battle_rooms"	"idx_battle_rooms_owner"	0
"public"	"battle_slow_run_archives"	"idx_battle_slow_run_archives_captured"	0
"public"	"market_auctions"	"uq_market_active_instance_auction"	0
"public"	"market_auctions"	"idx_market_auctions_item"	0
"public"	"market_auctions"	"idx_market_auctions_ends_at"	0
"public"	"market_fee_tiers"	"idx_market_fee_tiers_active"	0
"public"	"market_listings"	"idx_market_listings_status"	0
"public"	"market_listings"	"uq_market_active_instance_listing"	0
"public"	"market_orders"	"uq_market_orders_buyer_idempotency"	0
"public"	"password_reset_tokens"	"password_reset_tokens_token_hash_key"	0
"public"	"password_reset_tokens"	"idx_password_reset_tokens_user"	0
"public"	"player_achievement_states"	"idx_player_achievement_states_user_completed"	0
"public"	"player_action_queue"	"idx_player_action_queue_due"	0
"public"	"player_action_queue"	"idx_player_action_queue_player_next_process"	0
"public"	"player_battle_item_loadout"	"idx_player_battle_item_loadout_user_updated"	0
"public"	"player_equip_slots"	"uq_player_equip_slots_battle_instance"	0
"public"	"player_kitty_commissions"	"idx_player_kitty_commissions_user_kitty_status"	0
"public"	"player_kitty_commissions"	"idx_player_kitty_commissions_user_kitty_started"	0
"public"	"player_kitty_encounters"	"idx_player_kitty_encounters_to_user_created"	0
"public"	"player_kitty_encounters"	"idx_player_kitty_encounters_from_kitty_created"	0
"public"	"player_kitty_encounters"	"idx_player_kitty_encounters_to_kitty_created"	0
"public"	"player_kitty_encounters"	"idx_player_kitty_encounters_from_user_created"	0
"public"	"player_kitty_equip_slots"	"uq_player_kitty_equip_slots_instance"	0
"public"	"player_kitty_journals"	"idx_player_kitty_journals_encounter"	0
"public"	"player_rule_sets"	"player_rule_sets_user_id_rule_kind_slot_key_key"	0
"public"	"player_wardrobe_items"	"idx_wardrobe_items_user"	0
"public"	"risk_account_evidence"	"idx_risk_account_evidence_type_time"	0
"public"	"risk_account_evidence"	"idx_risk_account_evidence_user"	0
"public"	"risk_account_signals"	"idx_risk_account_signals_ip_prefix"	0
"public"	"risk_account_signals"	"idx_risk_account_signals_local_device"	0
"public"	"risk_account_signals"	"idx_risk_account_signals_fingerprint"	0
"public"	"risk_account_signals"	"idx_risk_account_signals_device_family"	0
"public"	"risk_account_transfers"	"idx_risk_account_transfers_from_time"	0
"public"	"risk_account_transfers"	"idx_risk_account_transfers_auction_time"	0
"public"	"risk_account_transfers"	"idx_risk_account_transfers_item_time"	0
"public"	"risk_capability_blocks"	"idx_risk_capability_blocks_user"	0
"public"	"risk_review_cases"	"idx_risk_review_cases_cluster"	0
"public"	"shop_offers"	"shop_offers_item_id_key"	0
"public"	"user_policy_acceptances"	"idx_user_policy_acceptances_user_policy"	0
"public"	"user_policy_acceptances"	"user_policy_acceptances_user_id_policy_key_policy_version_key"	0
"public"	"users"	"idx_users_ban_active"	0
"public"	"users"	"uq_users_email_non_empty"	0
"public"	"wardrobe_equip_slots"	"wardrobe_equip_slots_user_id_item_id_key"	0