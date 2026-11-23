# base_3 雪契行者内容设计（未写入 Excel 的部分）

## 设计定位

雪契行者不是群控流派，而是“单点记账、单点收债”的冰雪副本流派。玩家通过镰刀、白币饰品和技能给一个目标挂上 `ice_debt`，再用镰刀斩击、白币占算或救援后的短暂增益打出节奏。

已写入：`combat.xlsx` 的技能、Buff、技能类型；`equipment.xlsx` 的装备模板和词条。

未写入：掉落物、掉落表、装备 item 行、合成动作。

## 掉落物 Item 行建议

| id | type | rarity | stackable | marketListable | 用途 |
| --- | --- | --- | --- | --- | --- |
| `frost_ledger_page` | material | uncommon | TRUE | TRUE | 雪契账页，基础材料 |
| `white_coin_shard` | material | rare | TRUE | TRUE | 白币碎片，饰品核心材料 |
| `snowseal_wax` | material | rare | TRUE | TRUE | 雪封蜡，防御饰品材料 |
| `rimebone_handle` | material | rare | TRUE | TRUE | 霜骨柄，镰刀材料 |
| `ice_abacus_bead` | material | epic | TRUE | TRUE | 冰算盘珠，法系饰品材料 |
| `warm_thread_core` | material | epic | TRUE | TRUE | 暖线芯，生活饰品材料 |

## 必须手动补的装备 Item 行

这些 id 已作为装备模板写入 `equipment.xlsx`。如果不在 `items.xlsx` 增加同名 equipment item，完整构建会报 `equipment_templates.id not found in items`。

| id | type | rarity | stackable | carryableInCombat | marketListable | economyBaseValue |
| --- | --- | --- | --- | --- | --- | --- |
| `base3_rime_ledger_scythe` | equipment | epic | FALSE | FALSE | TRUE | 1 |
| `base3_white_coin_charm` | equipment | rare | FALSE | FALSE | TRUE | 1 |
| `base3_snowseal_ring` | equipment | rare | FALSE | FALSE | TRUE | 1 |
| `base3_ice_account_abacus` | equipment | epic | FALSE | FALSE | TRUE | 1 |
| `base3_frostpick_charm` | equipment | rare | FALSE | FALSE | TRUE | 1 |
| `base3_warm_thread_spool` | equipment | rare | FALSE | FALSE | TRUE | 1 |

## base_3 掉落建议

| enemy id | 建议 drops JSON 增量 |
| --- | --- |
| `ice_slime_001` | `frost_ledger_page` 8%, `snowseal_wax` 1.5%, `skillbook_snowpact_receipt` 0.12% |
| `snow_wolf_001` | `rimebone_handle` 2.5%, `frost_ledger_page` 6%, `skillbook_rime_reaper_cut` 0.10% |
| `frost_goblin_001` | `white_coin_shard` 2.2%, `frost_ledger_page` 7%, `skillbook_snowpact_receipt` 0.10% |
| `ice_bat_001` | `white_coin_shard` 1.8%, `ice_abacus_bead` 0.7%, `skillbook_white_coin_divination` 0.08% |
| `snow_rabbit_001` | `warm_thread_core` 0.8%, `frost_ledger_page` 5%, `skillbook_hearthside_thaw` 0.08% |
| `frost_spirit_001` | `ice_abacus_bead` 1.3%, `white_coin_shard` 3%, `skillbook_white_coin_divination` 0.12% |
| `snowfield_boss_001` | `base3_rime_ledger_scythe` 0.8%, `rimebone_handle` 12%, `skillbook_rime_reaper_cut` 0.8% |
| `snowfield_boss_002` | `base3_ice_account_abacus` 0.6%, `ice_abacus_bead` 10%, `warm_thread_core` 8%, `skillbook_hearthside_thaw` 0.8% |

掉落 JSON 形态示例：

```json
{
  "frost_ledger_page": {"percent": 0.08, "range": {"min": 1, "max": 2}},
  "skillbook_snowpact_receipt": {"percent": 0.0012, "range": {"min": 1, "max": 1}}
}
```

## 合成动作建议

| action id | sheet | sub_type | durationS | costs | itemReward | statusRequirement | statusExpReward |
| --- | --- | --- | ---: | --- | --- | --- | --- |
| `forging_base3_rime_ledger_scythe` | actions_forging | forging_weapon | 75 | `{"rimebone_handle":3,"frost_ledger_page":18,"white_coin_shard":4,"silver":2}` | `{"base3_rime_ledger_scythe":1}` | `{"forging":55}` | `{"forging":18}` |
| `craft_base3_white_coin_charm` | actions_crafting | craft_accessory | 46 | `{"white_coin_shard":8,"frost_ledger_page":10,"gold_coin":1500}` | `{"base3_white_coin_charm":1}` | `{"crafting":45}` | `{"crafting":12}` |
| `craft_base3_snowseal_ring` | actions_crafting | craft_accessory | 48 | `{"snowseal_wax":6,"white_coin_shard":4,"frost_ledger_page":10}` | `{"base3_snowseal_ring":1}` | `{"crafting":45}` | `{"crafting":12}` |
| `craft_base3_ice_account_abacus` | actions_crafting | craft_accessory | 62 | `{"ice_abacus_bead":5,"white_coin_shard":10,"frost_ledger_page":16}` | `{"base3_ice_account_abacus":1}` | `{"crafting":55}` | `{"crafting":16}` |
| `forging_base3_frostpick_charm` | actions_forging | forging_tool | 52 | `{"rimebone_handle":2,"snowseal_wax":4,"iron":2}` | `{"base3_frostpick_charm":1}` | `{"forging":45}` | `{"forging":13}` |
| `sewing_base3_warm_thread_spool` | actions_sewing | sewing_accessory | 55 | `{"warm_thread_core":4,"frost_ledger_page":12,"silver_vein_silk":2}` | `{"base3_warm_thread_spool":1}` | `{"sewing":45}` | `{"sewing":13}` |

如果 `action_sub_types` 里没有 `craft_accessory`、`forging_tool`、`sewing_accessory`，可以先复用现有相近分类，或者再补 sub type。
