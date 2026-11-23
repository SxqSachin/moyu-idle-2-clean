#!/usr/bin/env python3
"""
Generate deterministic alchemy trait DSL + name affixes.

The alchemy bench is intentionally a material sink: players spend stackable
materials for custom affixes, successful crafts roll within a range, and failed
crafts collapse every effect to its minimum value. This script therefore treats
the minimum as the "bad but still useful" floor and the maximum as the exciting
roll.

Outputs:
  packages/game-data/source/json/item_traits.json   (full rewrite)
  packages/game-data/source/json/items.json         (rewrite owned alchemy.nameAffix)

Use --special-overrides-only to merge hand-authored dungeon traits without
touching items.json or regenerating existing trait rows.
"""
import argparse
import copy
import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
from json.encoder import _make_iterencode, encode_basestring, encode_basestring_ascii
from pathlib import Path
from typing import Optional

ROOT = Path(__file__).resolve().parents[1]
ITEMS_PATH = Path(os.environ.get("ALCHEMY_ITEMS_PATH", ROOT / "packages/game-data/source/json/items.json"))
TRAITS_PATH = Path(os.environ.get("ALCHEMY_TRAITS_PATH", ROOT / "packages/game-data/source/json/item_traits.json"))
ATTRIBUTES_PATH = ROOT / "packages/game-data/source/json/attributes.json"
SCORING_PATH = ROOT / "packages/game-config/source/shared-global-config.json"


class TraitJSONEncoder(json.JSONEncoder):
    """Keep the source file's readable decimal notation for small trait values."""

    def iterencode(self, obj, _one_shot=False):
        markers = {} if self.check_circular else None
        encoder = encode_basestring_ascii if self.ensure_ascii else encode_basestring

        def floatstr(value):
            if value != value:
                text = "NaN"
            elif value == float("inf"):
                text = "Infinity"
            elif value == -float("inf"):
                text = "-Infinity"
            elif value and 0.000001 <= abs(value) < 0.0001:
                return format(value, ".15f").rstrip("0")
            else:
                return repr(value)
            if not self.allow_nan:
                raise ValueError(f"Out of range float values are not JSON compliant: {text}")
            return text

        iterator = _make_iterencode(
            markers,
            self.default,
            encoder,
            self.indent,
            floatstr,
            self.key_separator,
            self.item_separator,
            self.sort_keys,
            self.skipkeys,
            _one_shot,
        )
        return iterator(obj, 0)


def write_traits_document(document: dict) -> None:
    with open(TRAITS_PATH, "w") as file:
        file.write(TraitJSONEncoder(indent=2, ensure_ascii=False).encode(document))
        file.write("\n")

RARITY_ORDER = ["common", "uncommon", "rare", "epic", "legendary", "mythic"]

MATERIAL_EFFECTIVE_BODY_LIMIT_BY_RARITY = {
    "common": 5,
    "uncommon": 5,
    "rare": 5,
    "epic": 4,
    "legendary": 3,
    "mythic": 2,
}

TIER = {
    "common":    {"level": 1,  "fail": 0.08, "fail_per_lvl": 0.006, "budget": 0.058},
    "uncommon":  {"level": 8,  "fail": 0.11, "fail_per_lvl": 0.006, "budget": 0.082},
    "rare":      {"level": 15, "fail": 0.16, "fail_per_lvl": 0.006, "budget": 0.104},
    "epic":      {"level": 20, "fail": 0.24, "fail_per_lvl": 0.005, "budget": 0.148},
    "legendary": {"level": 25, "fail": 0.34, "fail_per_lvl": 0.005, "budget": 0.210},
    "mythic":    {"level": 35, "fail": 0.42, "fail_per_lvl": 0.004, "budget": 0.280},
}

WORK_ATTRS = {
    "collecting_speed", "mining_speed", "farming_animal_speed",
    "crafting_speed", "sewing_speed", "forging_speed",
    "alchemy_precision", "alchemy_craft_success", "alchemy_efficiency",
}

EFFICIENCY_ATTRS = {
    "collecting_speed", "mining_speed", "farming_animal_speed",
    "crafting_speed", "sewing_speed", "forging_speed", "alchemy_efficiency",
}

PENETRATION_ATTRS = {
    "physics_flat_defense_penetration", "magicial_flat_defense_penetration",
    "physics_percent_defense_penetration", "magicial_percent_defense_penetration",
}

CRITICAL_OUTPUT_ATTRS = {"crit_chance", "magic_crit_chance", "crit_damage", "magic_crit_damage"}
DEFENSE_OUTPUT_ATTRS = {"physics_defence", "magical_defence", "physics_damage_resist", "magical_damage_resist"}
RARE_PLUS = {"rare", "epic", "legendary", "mythic"}

MYTHIC_BOLD_IDS = {
    "wolf_king_fang",
    "luminous_berry_core",
    "assassination_core",
    "glacial_phantom_crystal",
}

POSITIVE_EFFICIENCY_RANGE_MULT = 1.70
MATERIAL_GOLD_COST_MULT = 0.67

POSITIVE_NUMBER_RANGE_MULT_BY_RARITY = {
    "common": 1.75,
    "uncommon": 1.90,
    "rare": 2.10,
    "epic": 2.30,
    "legendary": 2.50,
    "mythic": 2.50,
}

NEGATIVE_RANGE_MULT_BY_RARITY = {
    "common": 1.10,
    "uncommon": 1.12,
    "rare": 1.15,
    "epic": 1.18,
    "legendary": 1.22,
    "mythic": 1.25,
}

BATTLE_ATTRS = {
    "hp", "mp", "stamina", "strength", "dexterity", "intelligence",
    "perception", "luck", "hp_regen", "mp_regen", "speed",
    "physics_damage", "magical_damage", "physics_defence", "magical_defence",
    "physics_flat_defense_penetration", "magicial_flat_defense_penetration",
    "physics_damage_amplification", "physics_damage_resist",
    "physics_percent_defense_penetration", "magical_damage_amplification",
    "magical_damage_resist", "magicial_percent_defense_penetration",
    "crit_chance", "magic_crit_chance", "crit_damage", "magic_crit_damage",
    "crit_damage_resist",
    "battle_drop_rate_bonus", "battle_exp_bonus",
}

HIGH_SENSITIVITY_REWARD_ATTRS = {"battle_drop_rate_bonus", "battle_exp_bonus"}

ALCHEMY_ATTRS = WORK_ATTRS | BATTLE_ATTRS

# These caps are per material unit. The budget model chooses values from score,
# then caps keep very cheap attributes (HP) and very expensive attributes
# (percent damage) readable inside the alchemy UI.
ATTR_VALUE_CAP = {
    "hp": 0.18,
    "mp": 0.12,
    "stamina": 0.030,
    "strength": 0.030,
    "dexterity": 0.030,
    "intelligence": 0.030,
    "perception": 0.030,
    "luck": 0.016,
    "hp_regen": 0.018,
    "mp_regen": 0.014,
    "speed": 0.009,
    "physics_damage": 0.035,
    "magical_damage": 0.035,
    "physics_defence": 0.045,
    "magical_defence": 0.045,
    "physics_flat_defense_penetration": 0.020,
    "magicial_flat_defense_penetration": 0.020,
    "collecting_speed": 0.00180,
    "mining_speed": 0.00180,
    "farming_animal_speed": 0.00180,
    "crafting_speed": 0.00160,
    "sewing_speed": 0.00160,
    "forging_speed": 0.00160,
    "alchemy_precision": 0.00070,
    "alchemy_craft_success": 0.00016,
    "alchemy_efficiency": 0.00070,
    "physics_damage_amplification": 0.00055,
    "physics_damage_resist": 0.00055,
    "physics_percent_defense_penetration": 0.00055,
    "magical_damage_amplification": 0.00055,
    "magical_damage_resist": 0.00055,
    "magicial_percent_defense_penetration": 0.00055,
    "crit_chance": 0.00028,
    "magic_crit_chance": 0.00028,
    "crit_damage": 0.00120,
    "magic_crit_damage": 0.00120,
    "crit_damage_resist": 0.00085,
    "battle_drop_rate_bonus": 0.00022,
    "battle_exp_bonus": 0.00024,
}

FALLBACK_ATTR_UNIT_VALUES = {
    "collecting_speed": 1.8,
    "mining_speed": 1.8,
    "farming_animal_speed": 1.8,
    "crafting_speed": 2.0,
    "sewing_speed": 2.0,
    "forging_speed": 2.0,
    "alchemy_precision": 2.0,
    "alchemy_craft_success": 8,
    "alchemy_lucky_chance": 80,
    "alchemy_efficiency": 2.0,
    "battle_drop_rate_bonus": 16,
    "battle_exp_bonus": 12,
}

VALUE_FACTOR_MIN = 0.90
VALUE_FACTOR_MAX = 1.32
EXTREME_VALUE_FACTOR = 1.42

NEG_RATIO_BY_RARITY = {
    "common": 0.45,
    "uncommon": 0.40,
    "rare": 0.35,
    "epic": 0.28,
    "legendary": 0.20,
    "mythic": 0.16,
}

NEGATIVE_PENALTY_GLOBAL_MULT = 1.50
ALCHEMY_CORE_POSITIVE_ATTRS = {
    "alchemy_efficiency",
    "alchemy_craft_success",
    "alchemy_precision",
}
ALCHEMY_CORE_POSITIVE_MULT = 0.75
ALCHEMY_CORE_PENALTY_MULT = 1.25

# Values are authored around one valid material step, then divided back into
# per-item valuePerUnit for the DSL. Without this, common materials with
# unitBasis=50 overpower rare materials whose first valid step is much smaller.
BASIS_OUTPUT_MULT_BY_RARITY = {
    "common": 5.0,
    "uncommon": 6.25,
    "rare": 8.0,
    "epic": 10.0,
    "legendary": 12.0,
    "mythic": 14.0,
}

FAIL_PER_EXTRA_BASIS_BY_RARITY = {
    "common": 0.0018,
    "uncommon": 0.0023,
    "rare": 0.0030,
    "epic": 0.0039,
    "legendary": 0.0048,
    "mythic": 0.0060,
}

FAIL_QUANTITY_CAP_BY_RARITY = {
    "common": 0.060,
    "uncommon": 0.070,
    "rare": 0.090,
    "epic": 0.110,
    "legendary": 0.130,
    "mythic": 0.150,
}

FULL_POSITIVE_CAP = {
    "alchemy_precision": 0.18,
    "alchemy_craft_success": 0.18,
    "alchemy_efficiency": 0.18,
    "collecting_speed": 0.18,
    "mining_speed": 0.18,
    "farming_animal_speed": 0.18,
    "crafting_speed": 0.18,
    "sewing_speed": 0.18,
    "forging_speed": 0.18,
    "battle_drop_rate_bonus": 0.045,
    "battle_exp_bonus": 0.045,
    "crit_chance": 0.08,
    "magic_crit_chance": 0.08,
    "crit_damage": 0.20,
    "magic_crit_damage": 0.20,
    "physics_damage_amplification": 0.14,
    "magical_damage_amplification": 0.14,
    "physics_damage_resist": 0.14,
    "magical_damage_resist": 0.14,
    "physics_percent_defense_penetration": 0.14,
    "magicial_percent_defense_penetration": 0.14,
    "crit_damage_resist": 0.16,
}

EFFICIENCY_FULL_OUTPUT_CAP_BY_RARITY = {
    "common": 0.080,
    "uncommon": 0.105,
    "rare": 0.125,
    "epic": 0.162,
    "legendary": 0.210,
    "mythic": 0.205,
}
EFFICIENCY_MULTI_POSITIVE_CAP_MULT = 0.88
EFFICIENCY_MULTI_ATTR_CAP_MULT = 0.78

REWARD_FULL_OUTPUT_CAP_BY_RARITY = {
    "common": 0.010,
    "uncommon": 0.014,
    "rare": 0.018,
    "epic": 0.026,
    "legendary": 0.035,
    "mythic": 0.045,
}

ALCHEMY_PRECISION_FULL_OUTPUT_CAP_BY_RARITY = {
    "common": 0.14,
    "uncommon": 0.16,
    "rare": 0.19,
    "epic": 0.22,
    "legendary": 0.25,
    "mythic": 0.28,
}

# Efficiency affixes are deliberately not available in every rarity band. If
# every tier can carry the same work-speed stat, players can stack the common,
# uncommon, rare, epic and legendary versions together and bypass the intended
# material sink. These bands give each efficiency a few clear homes while
# leaving other tiers to pick alchemy or combat-flavored alternatives.
EFFICIENCY_POSITIVE_RARITY_BANDS = {
    "collecting_speed": {"common", "legendary"},
    "mining_speed": {"common", "legendary"},
    "farming_animal_speed": {"common", "legendary"},
    "crafting_speed": {"rare", "legendary"},
    "sewing_speed": {"uncommon", "legendary"},
    "forging_speed": {"epic", "legendary"},
    "alchemy_efficiency": {"common", "legendary"},
}

LIFE_PRIMARY_TRAIT_EXCLUDED_RARITIES = {"common", "uncommon"}
COMMON_LIFE_TRAIT_EXCLUDED_IDS = {
    "wood_blank",
    "sturdy_wood_blank",
    "silverleaf_wood_blank",
    "copper",
    "iron",
    "coarse_linen",
    "soft_cotton",
}

MEDIUM_ONLY_TRAIT_EXCLUDED_IDS = set()

TRAIT_EXCLUDED_IDS = COMMON_LIFE_TRAIT_EXCLUDED_IDS | MEDIUM_ONLY_TRAIT_EXCLUDED_IDS

FULL_POSITIVE_NUMBER_CAP = {
    "hp": 36.0,
    "mp": 24.0,
    "stamina": 6.0,
    "strength": 6.0,
    "dexterity": 6.0,
    "intelligence": 6.0,
    "perception": 6.0,
    "hp_regen": 4.0,
    "mp_regen": 3.0,
    "speed": 1.8,
    "physics_damage": 10.0,
    "magical_damage": 10.0,
    "physics_defence": 12.0,
    "magical_defence": 12.0,
    "physics_flat_defense_penetration": 4.0,
    "magicial_flat_defense_penetration": 4.0,
}

FULL_NEGATIVE_CAP = {
    "alchemy_precision": 0.24,
    "alchemy_craft_success": 0.22,
    "alchemy_efficiency": 0.22,
    "collecting_speed": 0.35,
    "mining_speed": 0.35,
    "farming_animal_speed": 0.35,
    "crafting_speed": 0.35,
    "sewing_speed": 0.35,
    "forging_speed": 0.35,
    "battle_drop_rate_bonus": 0.16,
    "battle_exp_bonus": 0.16,
    "crit_chance": 0.12,
    "magic_crit_chance": 0.12,
    "crit_damage": 0.28,
    "magic_crit_damage": 0.28,
    "physics_damage_amplification": 0.18,
    "magical_damage_amplification": 0.18,
    "physics_damage_resist": 0.18,
    "magical_damage_resist": 0.18,
    "physics_percent_defense_penetration": 0.18,
    "magicial_percent_defense_penetration": 0.18,
    "crit_damage_resist": 0.22,
}

FULL_NEGATIVE_NUMBER_CAP = {
    "hp": 24.0,
    "mp": 18.0,
    "stamina": 8.0,
    "strength": 8.0,
    "dexterity": 8.0,
    "intelligence": 8.0,
    "perception": 8.0,
    "hp_regen": 5.0,
    "mp_regen": 4.0,
    "speed": 0.45,
    "physics_damage": 12.0,
    "magical_damage": 12.0,
    "physics_defence": 14.0,
    "magical_defence": 14.0,
    "physics_flat_defense_penetration": 5.0,
    "magicial_flat_defense_penetration": 5.0,
}

# Raw life primary drops are plentiful, so keep them slightly below crafted or
# dungeon materials as affix fuel.
LIFE_PRIMARY_MULTIPLIER = 0.85

LIFE_PRIMARY_IDS = {
    "wood", "sturdy_wood", "silverleaf_wood", "silkwood", "goldvine_wood", "moonwood",
    "flax_fiber", "cotton_fiber", "silverleaf_fiber", "silkweed_fiber", "goldvine_fiber", "moonvine_fiber",
    "tree_sap", "soft_resin", "silverleaf_resin", "clear_resin", "sun_resin", "moon_resin",
    "morning_dew", "sweet_nectar", "silverleaf_dew", "silkweed_nectar", "sunlit_nectar", "moon_dew",
    "stone", "coal", "copper_ore", "iron_ore", "silver_ore", "gold_ore", "crystal_chunk",
    "egg", "milk", "wool",
}

ALCHEMY_PRECISION_SOURCE_IDS = {
    "tree_sap", "soft_resin", "silverleaf_resin", "clear_resin", "sun_resin", "moon_resin",
    "morning_dew", "sweet_nectar", "silverleaf_dew", "silkweed_nectar", "sunlit_nectar", "moon_dew",
    "frost_ledger_page",
}

FORCED_LIFE_IDS = {
    "warm_thread_core",
    "frost_ledger_page",
    "snowseal_wax",
    "base2_moss_spore",
    "holy_cat_blessed_fur",
}

FORCED_COMBAT_IDS = {
    "stone",
    "coal",
    "copper_ore",
    "iron_ore",
    "silver_ore",
    "gold_ore",
    "crystal_chunk",
    "copper",
    "iron",
    "silver",
    "gold",
    "crystal",
    "milk",
    "sturdy_wood",
    "silverleaf_wood",
    "soft_resin",
    "silverleaf_fiber",
    "silverleaf_resin",
    "sweet_nectar",
    "silverleaf_dew",
    "silkwood",
    "silkweed_nectar",
    "clear_resin",
    "silkweed_fiber",
    "goldvine_fiber",
    "sunlit_nectar",
    "base4_shadow_silk",
    "base4_shadow_iron",
    "holy_cat_luminous_thread",
    "afterglow",
    "glow_core",
    "strange_wolf_fur",
    "wolf_king_fang",
    "luminous_berry_core",
    "assassination_core",
    "glacial_phantom_crystal",
    "wood_blank",
    "sturdy_wood_blank",
    "silverleaf_wood_blank",
    "silkwood_blank",
    "goldvine_wood_blank",
    "moonwood_blank",
    "coarse_linen",
    "soft_cotton",
    "silver_vein_silk",
    "wisp_weave",
    "gilded_brocade",
    "lunar_veil",
}

# No penalty: rare brag drops should feel premium without being absurd.
PURE_BONUS_IDS = {
    "glacial_phantom_crystal",
    "base4_golem_dark_core",
    "warm_thread_core",
}

# Stronger upside with a meaningful downside.
OVERCHARGED_IDS = {
    "wolf_king_fang",
    "luminous_berry_core",
    "assassination_core",
    "base4_abyssal_core",
}

LUCK_TEMPTATION_IDS = {
    "holy_cat_royal_jewel",
}

CONSTITUTION_IDS = {
    "strange_wolf_fur",
    "rimebone_handle",
}

COMBAT_PROFILE_BY_ID = {
    "sturdy_wood": "shadow",
    "silverleaf_wood": "physical",
    "soft_resin": "arcane",
    "silverleaf_fiber": "shadow",
    "silverleaf_resin": "arcane",
    "sweet_nectar": "vitality",
    "silverleaf_dew": "mystic_crit",
    "silkwood": "shadow",
    "silkweed_nectar": "fang_crit",
    "clear_resin": "arcane",
    "silkweed_fiber": "scale_ward",
    "goldvine_fiber": "physical",
    "sunlit_nectar": "mystic_crit",
    "wood_blank": "physical",
    "sturdy_wood_blank": "vitality",
    "silverleaf_wood_blank": "fang_crit",
    "silkwood_blank": "shadow",
    "goldvine_wood_blank": "physical",
    "moonwood_blank": "mystic_crit",
    "coarse_linen": "vitality",
    "soft_cotton": "arcane",
    "silver_vein_silk": "mystic_crit",
    "wisp_weave": "shadow",
    "gilded_brocade": "arcane",
    "base4_phantom_claw": "spoils_hunter",
    "luminous_berry_core": "battle_insight",
}

PRODUCTION_COMBAT_MATERIAL_IDS = {
    "copper_ore",
    "iron_ore",
    "silver_ore",
    "gold_ore",
    "crystal_chunk",
    "copper",
    "iron",
    "silver",
    "gold",
    "crystal",
}

CONVERTED_LIFE_COMBAT_IDS = set(COMBAT_PROFILE_BY_ID.keys()) | PRODUCTION_COMBAT_MATERIAL_IDS
CONVERTED_LIFE_COMBAT_IDS -= {"base4_phantom_claw", "luminous_berry_core"}

REWARD_TRAIT_ALLOWED_IDS = {
    "base4_phantom_claw",
    "condensed_stardust",
    "luminous_berry_core",
    "parade_king_medal",
}


def _special_combat_trait(
    trait_id: str,
    family: str,
    effects: list[tuple[str, float, float]],
    unit_basis: int,
    max_basis_multiplier: int,
    gold_cost_per_unit: int,
    duration_sec_per_unit: float,
    alchemy_exp_per_unit: float,
    min_alchemy_level: int,
    fail_base_chance: float,
    fail_chance_per_level_above: float,
    fail_chance_per_extra_basis: float,
    fail_chance_quantity_cap: float,
    max_effective_per_equipment: int,
    max_effective_per_body: int,
) -> dict:
    return {
        "traitId": trait_id,
        "kind": "combat",
        "family": family,
        "effectType": "add_stat",
        "effects": [
            {
                "attr": attr,
                "valuePerUnit": {"min": min_value, "max": max_value},
            }
            for attr, min_value, max_value in effects
        ],
        "unitBasis": unit_basis,
        "maxBasisMultiplier": max_basis_multiplier,
        "goldCostPerUnit": gold_cost_per_unit,
        "durationSecPerUnit": duration_sec_per_unit,
        "alchemyExpPerUnit": alchemy_exp_per_unit,
        "minAlchemyLevel": min_alchemy_level,
        "failBaseChance": fail_base_chance,
        "failChancePerLevelAbove": fail_chance_per_level_above,
        "failChancePerExtraBasis": fail_chance_per_extra_basis,
        "failChanceQuantityCap": fail_chance_quantity_cap,
        "maxEffectivePerEquipment": max_effective_per_equipment,
        "maxEffectivePerBody": max_effective_per_body,
    }


# Hand-authored dungeon materials deliberately do not use name heuristics. Keep
# every gameplay field here so future generator runs retain the approved shape.
SPECIAL_TRAIT_OVERRIDES = {
    "star_rail_copper_plate": _special_combat_trait(
        "star_rail_copper_plate", "defense",
        [
            ("physics_defence", 0.025, 0.060),
            ("speed", 0.0016, 0.004),
            ("magical_defence", -0.030, -0.018),
        ],
        20, 10, 9, 0.109, 0.0098, 20, 0.14, 0.006, 0.0018, 0.06, 2, 5,
    ),
    "refraction_lens_shard": _special_combat_trait(
        "refraction_lens_shard", "magic_crit",
        [
            ("magic_crit_damage", 0.00028, 0.00078),
            ("magicial_percent_defense_penetration", 0.00006, 0.00016),
            ("physics_damage_resist", -0.00012, -0.00006),
        ],
        8, 25, 26, 0.188, 0.0169, 24, 0.22, 0.006, 0.003, 0.09, 2, 5,
    ),
    "meteor_core_fragment": _special_combat_trait(
        "meteor_core_fragment", "arcane",
        [
            ("magical_damage_amplification", 0.000035, 0.00009),
            ("physics_flat_defense_penetration", 0.009, 0.024),
            ("hp", -0.36, -0.20),
        ],
        8, 25, 28, 0.194, 0.0175, 25, 0.24, 0.006, 0.003, 0.09, 2, 5,
    ),
    "condensed_stardust": _special_combat_trait(
        "condensed_stardust", "magic_crit",
        [
            ("battle_exp_bonus", 0.00006, 0.00015),
            ("mp_regen", 0.006, 0.016),
            ("physics_defence", -0.05, -0.028),
            ("speed", -0.004, -0.0022),
        ],
        5, 40, 37, 0.223, 0.0201, 28, 0.30, 0.005, 0.0039, 0.11, 1, 3,
    ),
    "timekeeper_core": _special_combat_trait(
        "timekeeper_core", "defense",
        [
            ("speed", 0.004, 0.011),
            ("crit_damage_resist", 0.00045, 0.0012),
            ("physics_damage_amplification", -0.00018, -0.00010),
            ("magical_damage_amplification", -0.00018, -0.00010),
        ],
        4, 30, 58, 0.278, 0.025, 32, 0.38, 0.005, 0.0048, 0.13, 1, 3,
    ),
    "hushed_ink_crystal": _special_combat_trait(
        "hushed_ink_crystal", "defense",
        [
            ("magical_defence", 0.024, 0.058),
            ("perception", 0.009, 0.022),
            ("magical_damage", -0.020, -0.011),
        ],
        20, 10, 11, 0.124, 0.0112, 24, 0.16, 0.006, 0.0018, 0.06, 2, 5,
    ),
    "wandering_page_silk": _special_combat_trait(
        "wandering_page_silk", "shadow",
        [
            ("speed", 0.002, 0.0055),
            ("perception", 0.018, 0.048),
            ("hp", -0.22, -0.12),
        ],
        8, 25, 28, 0.194, 0.0175, 28, 0.26, 0.006, 0.004, 0.12, 2, 5,
    ),
    "blue_night_audit_feather": _special_combat_trait(
        "blue_night_audit_feather", "defense",
        [
            ("perception", 0.016, 0.044),
            ("crit_damage_resist", 0.0002, 0.00055),
            ("mp_regen", -0.01, -0.0055),
        ],
        8, 25, 29, 0.2, 0.018, 29, 0.27, 0.006, 0.004, 0.12, 2, 4,
    ),
    "spell_seal_lock_plate": _special_combat_trait(
        "spell_seal_lock_plate", "defense",
        [
            ("magical_defence", 0.04, 0.095),
            ("crit_damage_resist", 0.00035, 0.0009),
            ("speed", -0.006, -0.0033),
            ("magical_damage", -0.06, -0.032),
        ],
        5, 40, 39, 0.227, 0.0204, 32, 0.34, 0.005, 0.0045, 0.12, 1, 3,
    ),
    "final_volume_key_sigil": _special_combat_trait(
        "final_volume_key_sigil", "arcane",
        [
            ("magicial_percent_defense_penetration", 0.0003, 0.0008),
            ("magical_damage_resist", 0.00025, 0.0007),
            ("mp", -0.18, -0.1),
            ("crit_chance", -0.00035, -0.00018),
        ],
        4, 30, 60, 0.279, 0.0251, 36, 0.42, 0.005, 0.005, 0.14, 1, 3,
    ),
    "wayward_carousel_spring": _special_combat_trait(
        "wayward_carousel_spring", "shadow",
        [
            ("speed", 0.0025, 0.0068),
            ("physics_damage_amplification", 0.00003, 0.000085),
            ("perception", -0.018, -0.009),
        ],
        8, 25, 28, 0.196, 0.0176, 32, 0.28, 0.006, 0.004, 0.12, 2, 5,
    ),
    "candy_bomb": _special_combat_trait(
        "candy_bomb", "crit",
        [
            ("magical_damage_amplification", 0.00004, 0.00011),
            ("crit_chance", 0.00018, 0.0005),
            ("hp_regen", -0.015, -0.007),
        ],
        8, 25, 26, 0.186, 0.0167, 33, 0.30, 0.006, 0.004, 0.12, 2, 5,
    ),
    "plush_fur": _special_combat_trait(
        "plush_fur", "brute",
        [
            ("hp_regen", 0.012, 0.032),
            ("physics_damage_resist", 0.00012, 0.00035),
            ("speed", -0.0042, -0.0022),
        ],
        8, 25, 24, 0.182, 0.0164, 34, 0.30, 0.006, 0.004, 0.12, 2, 5,
    ),
    "bottled_starlight": _special_combat_trait(
        "bottled_starlight", "magic_crit",
        [
            ("mp_regen", 0.006, 0.017),
            ("magical_defence", 0.025, 0.065),
            ("stamina", -0.055, -0.030),
        ],
        8, 25, 29, 0.2, 0.018, 35, 0.32, 0.006, 0.004, 0.12, 2, 5,
    ),
    "ghost_essence": _special_combat_trait(
        "ghost_essence", "shadow",
        [
            ("physics_percent_defense_penetration", 0.0002, 0.00055),
            ("magical_damage_resist", 0.00018, 0.0005),
            ("hp", -0.45, -0.25),
            ("mp", -0.15, -0.08),
        ],
        5, 40, 40, 0.23, 0.0207, 38, 0.36, 0.005, 0.0045, 0.12, 1, 4,
    ),
    "parade_king_medal": _special_combat_trait(
        "parade_king_medal", "shadow",
        [
            ("battle_drop_rate_bonus", 0.00008, 0.00022),
            ("physics_damage_resist", 0.00018, 0.0005),
            ("physics_damage_amplification", -0.00016, -0.00008),
            ("magical_damage_amplification", -0.00016, -0.00008),
        ],
        4, 25, 64, 0.285, 0.0256, 42, 0.44, 0.005, 0.005, 0.14, 1, 3,
    ),
    "faceless_understudy_mask": _special_combat_trait(
        "faceless_understudy_mask", "crit",
        [
            ("crit_damage", 0.0014, 0.0038),
            ("magic_crit_damage", 0.0014, 0.0038),
            ("crit_chance", -0.00045, -0.00022),
            ("magic_crit_chance", -0.00045, -0.00022),
        ],
        5, 10, 511, 0.339, 0.0305, 45, 0.46, 0.004, 0.006, 0.15, 1, 2,
    ),
}

SPECIAL_DROP_ID_ALLOWLIST = frozenset({"final_volume_key_sigil"})

CONVERTED_LIFE_COMBAT_OUTPUT_MULT_BY_RARITY = {
    "common": 0.42,
    "uncommon": 0.45,
    "rare": 0.50,
    "epic": 0.56,
    "legendary": 0.60,
    "mythic": 0.65,
}

CONVERTED_LIFE_COMBAT_ATTR_MULT = {
    "battle_drop_rate_bonus": 0.28,
    "battle_exp_bonus": 0.28,
    "crit_chance": 0.62,
    "magic_crit_chance": 0.62,
    "crit_damage": 0.64,
    "magic_crit_damage": 0.64,
    "physics_damage": 0.72,
    "magical_damage": 0.72,
    "physics_defence": 0.70,
    "magical_defence": 0.70,
    "physics_flat_defense_penetration": 0.66,
    "magicial_flat_defense_penetration": 0.66,
    "physics_percent_defense_penetration": 0.58,
    "magicial_percent_defense_penetration": 0.58,
    "physics_damage_amplification": 0.58,
    "magical_damage_amplification": 0.58,
    "physics_damage_resist": 0.66,
    "magical_damage_resist": 0.66,
    "speed": 0.66,
    "hp_regen": 0.68,
    "mp_regen": 0.68,
}

FORCED_POSITIVE_ATTRS_BY_ID = {
    "moonwood": ["crafting_speed", "forging_speed"],
    "moonvine_fiber": ["sewing_speed", "collecting_speed"],
    "moon_resin": ["alchemy_precision", "mining_speed"],
    "moon_dew": ["alchemy_efficiency", "farming_animal_speed"],
    "warm_thread_core": ["sewing_speed", "alchemy_craft_success"],
    "base4_phantom_claw": ["battle_drop_rate_bonus", "crit_damage"],
    "luminous_berry_core": ["battle_exp_bonus", "magicial_flat_defense_penetration"],
}

UNIT_BASIS_BY_RARITY = {
    "common": 20,
    "uncommon": 10,
    "rare": 8,
    "epic": 5,
    "legendary": 4,
    "mythic": 5,
}

RARITY_FULL_CAP_MULT_BY_RARITY = {
    "common": 1.00,
    "uncommon": 1.08,
    "rare": 1.18,
    "epic": 1.35,
    "legendary": 1.55,
    "mythic": 1.80,
}

OLD_MAX_QUANTITY_DEFAULT = 1000
OLD_MAX_QUANTITY_STRICT = 300
MAX_QUANTITY_DEFAULT = 200
MAX_QUANTITY_STRICT = 50
MAX_QUANTITY_LOW_RARITY_LIFE = 500

STRICT_LIMIT_IDS = {
    "glacial_phantom_crystal",
    "wolf_king_fang",
    "luminous_berry_core",
    "assassination_core",
    "base4_abyssal_core",
}

LIFE_PROFILES = {
    "woodwork": {
        "family": "gathering",
        "pos": ["crafting_speed", "sewing_speed", "collecting_speed"],
        "neg": ["mining_speed", "forging_speed", "farming_animal_speed"],
        "prefix": {"common": "rugged", "uncommon": "stout", "rare": "verdant", "epic": "bulwark", "legendary": "titanic"},
    },
    "fiberwork": {
        "family": "sewing",
        "pos": ["sewing_speed", "crafting_speed", "collecting_speed", "alchemy_craft_success"],
        "neg": ["forging_speed", "mining_speed", "farming_animal_speed"],
        "prefix": {"common": "swift", "uncommon": "fleet", "rare": "windborn", "epic": "phantom", "legendary": "tempest"},
    },
    "alchemy_sap": {
        "family": "alchemy",
        "pos": ["alchemy_precision", "alchemy_efficiency", "alchemy_craft_success", "collecting_speed", "crafting_speed"],
        "neg": ["alchemy_efficiency", "mining_speed", "forging_speed", "sewing_speed"],
        "prefix": {"common": "lucid", "uncommon": "wellspring", "rare": "starbound", "epic": "arcane", "legendary": "celestial"},
    },
    "mineralwork": {
        "family": "mining",
        "pos": ["mining_speed", "forging_speed", "crafting_speed", "alchemy_precision"],
        "neg": ["collecting_speed", "sewing_speed", "farming_animal_speed"],
        "prefix": {"common": "heavy", "uncommon": "ironheart", "rare": "thunder", "epic": "ironclad", "legendary": "godhammer"},
    },
    "livestock": {
        "family": "livestock",
        "pos": ["farming_animal_speed", "sewing_speed", "alchemy_craft_success", "alchemy_efficiency", "crafting_speed"],
        "neg": ["mining_speed", "forging_speed", "collecting_speed"],
        "prefix": {"common": "vital", "uncommon": "hale", "rare": "lively", "epic": "vigorous", "legendary": "primordial"},
    },
    "craft_blank": {
        "family": "crafting",
        "pos": ["crafting_speed", "forging_speed", "sewing_speed", "alchemy_precision"],
        "neg": ["collecting_speed", "mining_speed", "farming_animal_speed"],
        "prefix": {"common": "rugged", "uncommon": "stout", "rare": "ironclad", "epic": "bulwark", "legendary": "titanic"},
    },
}

COMBAT_PROFILES = {
    "constitution": {
        "family": "brute",
        "pos": ["stamina", "hp", "hp_regen", "physics_defence", "magical_defence"],
        "neg": ["speed", "mp", "magic_crit_chance", "magical_damage_amplification"],
        "prefix": {"common": "vital", "uncommon": "hale", "rare": "ironheart", "epic": "aegis", "legendary": "titanic", "mythic": "primordial"},
    },
    "vitality": {
        "family": "brute",
        "pos": ["hp_regen", "hp", "stamina", "mp_regen", "physics_damage_resist"],
        "neg": ["speed", "magical_damage", "crit_chance", "magic_crit_chance"],
        "prefix": {"common": "vital", "uncommon": "lifeflow", "rare": "wellspring", "epic": "ambrosial", "legendary": "radiant"},
    },
    "physical": {
        "family": "physical",
        "pos": ["physics_damage", "strength", "physics_flat_defense_penetration", "crit_damage", "physics_damage_amplification"],
        "neg": ["hp", "speed", "magical_defence", "magical_damage_resist", "mp"],
        "prefix": {"common": "sharp", "uncommon": "honed", "rare": "edged", "epic": "lethal", "legendary": "executor"},
    },
    "mineral_breaker": {
        "family": "physical",
        "pos": ["physics_flat_defense_penetration", "physics_damage", "strength", "physics_percent_defense_penetration", "crit_damage"],
        "neg": ["speed", "hp", "magical_damage_resist", "mp_regen", "magical_defence"],
        "prefix": {"common": "sharp", "uncommon": "honed", "rare": "edged", "epic": "lethal", "legendary": "executor"},
    },
    "fang_crit": {
        "family": "crit",
        "pos": ["crit_chance", "crit_damage", "physics_damage", "dexterity", "physics_flat_defense_penetration"],
        "neg": ["hp", "physics_defence", "magical_defence", "mp_regen"],
        "prefix": {"common": "edge", "uncommon": "precise", "rare": "hunter", "epic": "deadeye", "legendary": "apex"},
    },
    "spoils_hunter": {
        "family": "crit",
        "pos": ["battle_drop_rate_bonus", "crit_damage", "crit_chance", "dexterity"],
        "neg": ["hp", "physics_defence", "magical_defence", "mp_regen"],
        "prefix": {"common": "edge", "uncommon": "precise", "rare": "hunter", "epic": "deadeye", "legendary": "apex"},
    },
    "arcane": {
        "family": "arcane",
        "pos": ["magical_damage", "intelligence", "magicial_flat_defense_penetration", "magic_crit_damage", "magical_damage_amplification"],
        "neg": ["hp", "speed", "physics_defence", "physics_damage_resist", "stamina"],
        "prefix": {"common": "studied", "uncommon": "scholarly", "rare": "sage", "epic": "archmagus", "legendary": "cosmic"},
    },
    "ruin_core": {
        "family": "arcane",
        "pos": ["magicial_flat_defense_penetration", "magical_damage", "intelligence", "magical_damage_amplification", "magic_crit_damage"],
        "neg": ["hp", "speed", "physics_damage_resist", "stamina", "physics_defence"],
        "prefix": {"common": "ruin", "uncommon": "ruin", "rare": "ruin", "epic": "sage", "legendary": "cosmic"},
    },
    "mystic_crit": {
        "family": "magic_crit",
        "pos": ["magic_crit_chance", "magic_crit_damage", "magicial_percent_defense_penetration", "perception", "mp_regen", "magical_damage"],
        "neg": ["hp", "speed", "physics_defence", "stamina", "magical_damage_resist"],
        "prefix": {"common": "flicker", "uncommon": "spark", "rare": "starlit", "epic": "radiant", "legendary": "supernova"},
    },
    "battle_insight": {
        "family": "magic_crit",
        "pos": ["battle_exp_bonus", "magicial_flat_defense_penetration", "magic_crit_damage", "perception"],
        "neg": ["hp", "speed", "physics_defence", "stamina", "magical_damage_resist"],
        "prefix": {"common": "flicker", "uncommon": "spark", "rare": "starlit", "epic": "radiant", "legendary": "supernova", "mythic": "supernova"},
    },
    "ward": {
        "family": "defense",
        "pos": ["physics_defence", "magical_defence", "hp_regen", "physics_damage_resist", "magical_damage_resist"],
        "neg": ["speed", "crit_chance", "magic_crit_chance", "physics_percent_defense_penetration"],
        "prefix": {"common": "guarded", "uncommon": "warded", "rare": "barrier", "epic": "aegis", "legendary": "eternal"},
    },
    "scale_ward": {
        "family": "defense",
        "pos": ["magical_defence", "physics_damage_resist", "physics_defence", "hp_regen", "magical_damage_resist"],
        "neg": ["speed", "crit_chance", "magic_crit_chance", "physics_percent_defense_penetration"],
        "prefix": {"common": "dragon", "uncommon": "dragon", "rare": "dragon", "epic": "aegis", "legendary": "eternal"},
    },
    "shadow": {
        "family": "shadow",
        "pos": ["speed", "dexterity", "crit_chance", "physics_percent_defense_penetration", "mp_regen"],
        "neg": ["hp", "physics_defence", "magical_defence", "mp"],
        "prefix": {"common": "shadow", "uncommon": "dusk", "rare": "phantom", "epic": "cursed", "legendary": "tempest"},
    },
    "beast": {
        "family": "brute",
        "pos": ["hp", "stamina", "hp_regen", "strength", "physics_damage_resist", "physics_defence"],
        "neg": ["mp", "magical_damage", "magic_crit_chance", "magical_defence"],
        "prefix": {"common": "crude", "uncommon": "heavy", "rare": "thunder", "epic": "ironheart", "legendary": "godhammer"},
    },
    "ancestor_jade": {
        "family": "shadow",
        "pos": ["dexterity"],
        "neg": ["speed", "hp"],
        "prefix": {"common": "ancestor", "uncommon": "ancestor", "rare": "ancestor", "epic": "ancestor", "legendary": "ancestor"},
    },
    "fortune": {
        "family": "apex",
        "pos": ["luck"],
        "neg": ["hp", "physics_defence", "magical_defence", "speed"],
        "prefix": {"common": "royal", "uncommon": "royal", "rare": "royal", "epic": "royal", "legendary": "royal"},
    },
}

ALL_LIFE_ATTRS = sorted(WORK_ATTRS)
ALL_COMBAT_ATTRS = sorted(BATTLE_ATTRS)

PREFIX_BY_ID = {
    "base2_moss_spore": "glowing",
    "holy_cat_blessed_fur": "blessed",
    "base4_shadow_silk": "shadow",
    "frost_ledger_page": "frost",
    "white_coin_shard": "white_coin",
    "snowseal_wax": "snowseal",
    "base2_cave_membrane": "cave",
    "base2_ruin_core_shard": "ruin",
    "base4_phantom_claw": "phantom",
    "base4_cursed_wing_membrane": "cursed",
    "base4_shadow_orb": "dusk",
    "holy_cat_incense": "solemn",
    "holy_cat_guardian_medal": "guarded",
    "base2_dragon_scale_chip": "dragon",
    "rimebone_handle": "rimebone",
    "ice_abacus_bead": "ice_abacus",
    "warm_thread_core": "warm",
    "base4_golem_dark_core": "dark_golem",
    "base4_abyssal_core": "abyssal",
    "base4_ancestor_shadow_jade": "ancestor",
    "holy_cat_sage_scroll": "sage",
    "strange_wolf_fur": "feral",
    "assassination_core": "assassin",
    "holy_cat_royal_jewel": "royal",
    "wolf_king_fang": "wolf_king",
    "luminous_berry_core": "luminous",
    "glacial_phantom_crystal": "glacial",
}


def stable_int(seed: str) -> int:
    return int(hashlib.sha1(seed.encode("utf-8")).hexdigest()[:12], 16)


def stable_pick(options: list[str], seed: str, count: int, required: Optional[list[str]] = None) -> list[str]:
    required = required or []
    out = []
    for attr in required:
        if attr in options and attr not in out:
            out.append(attr)
    ranked = sorted(
        [attr for attr in options if attr not in out],
        key=lambda attr: (stable_int(f"{seed}:{attr}"), attr),
    )
    for attr in ranked:
        if len(out) >= count:
            break
        out.append(attr)
    return out


def clamp(v: float, lo: float, hi: float) -> float:
    return max(lo, min(hi, v))


def round_smart(v: float, sig: int = 4) -> float:
    if v == 0:
        return 0
    from math import floor, log10
    digits = sig - int(floor(log10(abs(v)))) - 1
    return round(v, max(2, digits))


def load_attribute_units() -> dict[str, str]:
    doc = json.load(open(ATTRIBUTES_PATH))
    out = {}
    for row in doc.get("attributes", []):
        attr_id = str(row.get("id") or "").strip()
        if not attr_id:
            continue
        out[attr_id] = "percent" if str(row.get("unit") or "").lower() == "percent" else "number"
    return out


def load_attribute_unit_values() -> dict[str, float]:
    doc = json.load(open(SCORING_PATH))
    raw = doc.get("combatScoring", {}).get("attributeUnitValues", {})
    out = {}
    for attr in ALCHEMY_ATTRS:
        value = raw.get(attr, FALLBACK_ATTR_UNIT_VALUES.get(attr))
        if not isinstance(value, (int, float)) or value <= 0:
            raise ValueError(f"Missing positive attributeUnitValues for alchemy attr: {attr}")
        out[attr] = float(value)
    return out


def as_type_list(value) -> list[str]:
    if isinstance(value, list):
        return [str(x) for x in value]
    if value:
        return [str(value)]
    return []


def is_drop(it: dict) -> bool:
    types = as_type_list(it.get("type"))
    iid = str(it.get("id") or "").lower()
    if it.get("disabled") or it.get("isEquipment"):
        return False
    cat = it.get("category", "")
    if cat in ("alchemy_medium", "equipment"):
        return False
    if any(t in types for t in ("food", "trigger_item", "collectible", "skillbook", "currency", "map", "key", "quest")):
        return False
    if iid.startswith("key_"):
        return False
    if iid not in SPECIAL_DROP_ID_ALLOWLIST and any(
        token in iid for token in ("map_", "skillbook_", "rumor", "world_boss", "boss_")
    ):
        return False
    return "loot" in types or "material" in types


def allows_trait_material(it: dict) -> bool:
    item_id = str(it.get("id") or "")
    if item_id in TRAIT_EXCLUDED_IDS:
        return False
    if item_id in LIFE_PRIMARY_IDS and str(it.get("rarity") or "common") in LIFE_PRIMARY_TRAIT_EXCLUDED_RARITIES:
        return False
    return True


def value_quantiles(items: list[dict]) -> dict[str, float]:
    by_rarity: dict[str, list[float]] = {}
    for it in items:
        rarity = it.get("rarity", "common")
        by_rarity.setdefault(rarity, []).append(float(it.get("economyBaseValue") or 0))
    for rarity in by_rarity:
        by_rarity[rarity].sort()
    out = {}
    for it in items:
        rarity = it.get("rarity", "common")
        values = by_rarity.get(rarity, [0])
        value = float(it.get("economyBaseValue") or 0)
        if len(values) <= 1:
            q = 0.5
        else:
            below = sum(1 for v in values if v <= value) - 1
            q = clamp(below / (len(values) - 1), 0, 1)
        out[it["id"]] = q
    return out


def pick_kind(item_id: str, item: dict) -> str:
    s = item_id.lower()
    if item_id in FORCED_LIFE_IDS:
        return "life"
    if item_id in FORCED_COMBAT_IDS:
        return "combat"
    if item.get("category") == "alchemy_material":
        # Most explicit alchemy loot comes from combat, unless a semantic rule
        # above marks it as a work-flavored reagent.
        return "combat"
    if any(k in s for k in ("claw", "fang", "medal", "jade", "scroll", "orb", "core", "membrane", "scale", "incense")):
        return "combat"
    return "life"


def pick_life_profile(item_id: str) -> str:
    s = item_id.lower()
    if "thread" in s or "fiber" in s or "linen" in s or "cotton" in s or "silk" in s or "weave" in s or "brocade" in s or "veil" in s:
        return "fiberwork"
    if "ore" in s or "stone" in s or "coal" in s or s in {"copper", "iron", "silver", "gold", "crystal", "crystal_chunk"}:
        return "mineralwork"
    if "blank" in s:
        return "craft_blank"
    if "resin" in s or "sap" in s or "dew" in s or "nectar" in s or "spore" in s or "ledger" in s or "wax" in s:
        return "alchemy_sap"
    if s in {"egg", "milk", "wool"} or "fur" in s:
        return "livestock"
    if "wood" in s:
        return "woodwork"
    return "woodwork"


def pick_combat_profile(item_id: str) -> str:
    s = item_id.lower()
    if item_id in COMBAT_PROFILE_BY_ID:
        return COMBAT_PROFILE_BY_ID[item_id]
    if item_id in LUCK_TEMPTATION_IDS:
        return "fortune"
    if item_id == "base4_ancestor_shadow_jade":
        return "ancestor_jade"
    if item_id in CONSTITUTION_IDS:
        return "constitution"
    if s in {"milk", "egg", "wool"}:
        return "vitality"
    if "ruin_core" in s:
        return "ruin_core"
    if "scale" in s:
        return "scale_ward"
    if "core" in s or "incense" in s or "scroll" in s or "ledger" in s:
        return "arcane"
    if "stone" in s or "membrane" in s or "wax" in s:
        return "ward"
    if s.endswith("_ore") or "coal" in s or s in {"copper", "iron", "silver", "gold", "crystal"}:
        return "mineral_breaker"
    if "fang" in s or "claw" in s or "fur" in s or "wolf" in s:
        return "fang_crit"
    if "blank" in s:
        return "physical"
    if "orb" in s or "crystal" in s or "jewel" in s or "berry_core" in s or "luminous" in s or "glow" in s:
        return "mystic_crit"
    if "shadow" in s or "assassin" in s or "phantom" in s or "cursed" in s:
        return "shadow"
    if "medal" in s:
        return "ward"
    if "iron" in s or "handle" in s or "coin" in s or "abacus" in s:
        return "physical"
    return "beast"


def pick_suffix(item_id: str) -> str:
    s = item_id.lower()
    if "wood_blank" in s: return "blank"
    if "wood" in s: return "timber"
    if "fiber" in s: return "fiber"
    if "linen" in s: return "linen"
    if "cotton" in s: return "cotton"
    if "silk" in s or "weave" in s: return "weave"
    if "brocade" in s: return "brocade"
    if "veil" in s: return "veil"
    if "resin" in s: return "resin"
    if "sap" in s: return "sap"
    if "nectar" in s: return "nectar"
    if "dew" in s: return "dew"
    if "stone" in s: return "stone"
    if "coal" in s: return "coal"
    if "berry_core" in s: return "berry_core"
    if "core" in s: return "core"
    if "crystal" in s: return "crystal"
    if s.endswith("_ore"): return "ore"
    if "shadow_iron" in s: return "shadow_iron"
    if "luminous_thread" in s: return "luminous_thread"
    if "afterglow" in s: return "afterglow"
    if "glow_core" in s: return "glow_core"
    if s == "copper": return "copper_ingot"
    if s == "iron": return "iron_ingot"
    if s == "silver": return "silver_ingot"
    if s == "gold": return "gold_ingot"
    if s == "egg": return "egg"
    if s == "milk": return "milk"
    if s == "wool": return "wool"
    if "moss" in s or "spore" in s: return "moss"
    if "fur" in s: return "wolf_fur" if "wolf" in s else "fur"
    if "page" in s: return "page"
    if "wax" in s: return "wax"
    if "membrane" in s: return "membrane"
    if "scale" in s: return "scale"
    if "claw" in s: return "claw"
    if "wing" in s: return "wing"
    if "orb" in s: return "orb"
    if "incense" in s: return "incense"
    if "medal" in s: return "medal"
    if "handle" in s: return "handle"
    if "bead" in s: return "bead"
    if "thread" in s: return "thread"
    if "jade" in s: return "jade"
    if "scroll" in s: return "scroll"
    if "jewel" in s: return "jewel"
    if "fang" in s: return "fang"
    return "essence"


def positive_effect_count(rarity: str, item_id: str, value_q: float) -> int:
    if item_id in LUCK_TEMPTATION_IDS:
        return 1
    if item_id == "base4_ancestor_shadow_jade":
        return 1
    if rarity in ("common", "uncommon"):
        return 2
    if rarity == "rare":
        return 2
    return 2


def has_penalty(rarity: str, item_id: str, value_q: float) -> bool:
    if item_id in LUCK_TEMPTATION_IDS:
        return True
    if item_id in PURE_BONUS_IDS:
        return False
    if item_id in OVERCHARGED_IDS:
        return True
    return True


def penalty_count(rarity: str, item_id: str) -> int:
    if item_id in LUCK_TEMPTATION_IDS:
        return 3
    if item_id == "base4_ancestor_shadow_jade":
        return 1
    if item_id in OVERCHARGED_IDS or item_id in MYTHIC_BOLD_IDS:
        return 2
    if rarity in {"epic", "legendary", "mythic"}:
        return 2
    return 1


def value_budget(item: dict, value_q: float) -> float:
    rarity = item.get("rarity", "common")
    tier = TIER.get(rarity, TIER["common"])
    factor = VALUE_FACTOR_MIN + (VALUE_FACTOR_MAX - VALUE_FACTOR_MIN) * value_q
    value = float(item.get("economyBaseValue") or 0)
    if value >= 10000:
        factor = EXTREME_VALUE_FACTOR
    return tier["budget"] * factor


def attr_score_value(attr: str, value: float, attr_units: dict[str, str], attr_unit_values: dict[str, float]) -> float:
    unit_multiplier = 100 if attr_units.get(attr) == "percent" else 1
    return abs(value) * unit_multiplier * attr_unit_values[attr]


def budget_to_value(attr: str, budget: float, attr_units: dict[str, str], attr_unit_values: dict[str, float]) -> float:
    unit_multiplier = 100 if attr_units.get(attr) == "percent" else 1
    raw = budget / max(0.000001, unit_multiplier * attr_unit_values[attr])
    return min(raw, ATTR_VALUE_CAP.get(attr, raw))


def positive_pool_share(count: int) -> float:
    if count <= 1:
        return 1.0
    return 0.62 if count == 2 else 0.48


def efficiency_output_cap(rarity: str, efficiency_count: int, positive_count: int) -> float:
    cap = EFFICIENCY_FULL_OUTPUT_CAP_BY_RARITY.get(rarity, EFFICIENCY_FULL_OUTPUT_CAP_BY_RARITY["common"])
    if positive_count > 1:
        cap *= EFFICIENCY_MULTI_POSITIVE_CAP_MULT
    if efficiency_count > 1:
        cap *= EFFICIENCY_MULTI_ATTR_CAP_MULT ** (efficiency_count - 1)
    return cap


def full_value_cap(attr: str, positive: bool, attr_units: dict[str, str]) -> float:
    if positive:
        if attr_units.get(attr) == "percent":
            return FULL_POSITIVE_CAP.get(attr, 0.18)
        return FULL_POSITIVE_NUMBER_CAP.get(attr, 6.0)
    if attr_units.get(attr) == "percent":
        return FULL_NEGATIVE_CAP.get(attr, 0.24)
    return FULL_NEGATIVE_NUMBER_CAP.get(attr, 8.0)


def positive_range_multiplier(attr: str, rarity: str, attr_units: dict[str, str]) -> float:
    multiplier = 1.0
    if attr in EFFICIENCY_ATTRS:
        multiplier = POSITIVE_EFFICIENCY_RANGE_MULT
    elif attr != "luck" and attr_units.get(attr) != "percent":
        multiplier = POSITIVE_NUMBER_RANGE_MULT_BY_RARITY.get(rarity, POSITIVE_NUMBER_RANGE_MULT_BY_RARITY["common"])
    if attr in EFFICIENCY_ATTRS or attr in PENETRATION_ATTRS:
        multiplier *= 0.80
    if attr in CRITICAL_OUTPUT_ATTRS:
        multiplier *= 0.80
    if rarity in RARE_PLUS and attr in EFFICIENCY_ATTRS:
        multiplier *= 0.80
    if rarity in RARE_PLUS and attr in DEFENSE_OUTPUT_ATTRS:
        multiplier *= 0.80
    return multiplier


def material_quantity_output_scale(item_id: str, rarity: str) -> float:
    if item_id in LUCK_TEMPTATION_IDS:
        return 1.0
    old_cap = OLD_MAX_QUANTITY_STRICT if item_id in STRICT_LIMIT_IDS else OLD_MAX_QUANTITY_DEFAULT
    return old_cap / max(1, material_max_quantity_cap(item_id, rarity))


def material_max_quantity_cap(item_id: str, rarity: str) -> int:
    if item_id in LUCK_TEMPTATION_IDS:
        return 100
    if item_id in STRICT_LIMIT_IDS:
        return MAX_QUANTITY_STRICT
    if rarity in {"common", "uncommon"} and item_id in LIFE_PRIMARY_IDS:
        return MAX_QUANTITY_LOW_RARITY_LIFE
    return MAX_QUANTITY_DEFAULT


def mythic_bold_attr_multiplier(attr: str, attr_units: dict[str, str], attr_unit_values: dict[str, float]) -> float:
    if attr not in attr_unit_values:
        return 4.0
    from math import log10
    unit_multiplier = 100 if attr_units.get(attr) == "percent" else 1
    score_weight = max(1.0, attr_unit_values[attr] * unit_multiplier)
    normalized = clamp(log10(score_weight) / log10(6000), 0, 1)
    multiplier = 5.2 - normalized * 2.6
    if attr == "speed":
        multiplier *= 0.42
    return multiplier


def negative_range_multiplier(rarity: str) -> float:
    return NEGATIVE_RANGE_MULT_BY_RARITY.get(rarity, NEGATIVE_RANGE_MULT_BY_RARITY["common"])


def allows_alchemy_precision(item_id: str) -> bool:
    return item_id in ALCHEMY_PRECISION_SOURCE_IDS


def remove_disallowed_precision(item_id: str, attrs: list[str]) -> list[str]:
    return [attr for attr in attrs if attr != "alchemy_precision" or allows_alchemy_precision(item_id)]


def allows_positive_attr_in_rarity(attr: str, rarity: str) -> bool:
    allowed = EFFICIENCY_POSITIVE_RARITY_BANDS.get(attr)
    return allowed is None or rarity in allowed


def filter_positive_attrs_for_rarity(attrs: list[str], rarity: str) -> list[str]:
    return [attr for attr in attrs if allows_positive_attr_in_rarity(attr, rarity)]


def positive_attrs_compatible(attrs: list[str]) -> bool:
    if len(set(attrs)) != len(attrs):
        return False
    if "alchemy_precision" in attrs and ({"alchemy_efficiency", "alchemy_craft_success"} & set(attrs)):
        return False
    if "alchemy_craft_success" in attrs and ({"alchemy_efficiency", "alchemy_precision"} & set(attrs)):
        return False
    return True


def positive_combo_key(kind: str, attrs: list[str]) -> str:
    return f"{kind}|{' + '.join(sorted(attrs))}"


def diversify_positive_attrs(
    kind: str,
    item_id: str,
    desired_count: int,
    pos_attrs: list[str],
    profile_pos: list[str],
    usable_all_attrs: list[str],
    used_positive_combos: set[str],
    required_attrs: Optional[list[str]] = None,
) -> list[str]:
    required_attrs = required_attrs or []
    if desired_count <= 0 or not used_positive_combos:
        return pos_attrs
    current_key = positive_combo_key(kind, pos_attrs)
    if current_key not in used_positive_combos and positive_attrs_compatible(pos_attrs):
        return pos_attrs

    pool = list(dict.fromkeys(profile_pos + usable_all_attrs))
    if len(pool) < desired_count:
        return pos_attrs
    profile_set = set(profile_pos)

    from itertools import combinations
    candidates = []
    for combo in combinations(pool, desired_count):
        attrs = list(combo)
        if any(attr not in attrs for attr in required_attrs):
            continue
        if not positive_attrs_compatible(attrs):
            continue
        key = positive_combo_key(kind, attrs)
        if key in used_positive_combos:
            continue
        profile_hits = sum(1 for attr in attrs if attr in profile_set)
        stability = stable_int(f"{item_id}:diversify:{key}")
        candidates.append((-profile_hits, stability, attrs))
    if not candidates:
        return pos_attrs
    candidates.sort(key=lambda row: (row[0], row[1]))
    return candidates[0][2]


def item_attr_cap_multiplier(item_id: str, attr: str, rarity: str) -> float:
    if attr == "collecting_speed" and pick_life_profile(item_id) == "woodwork":
        return 0.70
    if item_id in CONVERTED_LIFE_COMBAT_IDS:
        # Life materials are more plentiful than dungeon combat drops. When a
        # life material is intentionally redirected into a combat profile for
        # variety, keep its combat ceiling below true combat loot so stacking
        # several custom affixes does not outgrow the equipment economy.
        return converted_life_combat_output_multiplier(item_id, attr, rarity)
    return 1.0


def converted_life_combat_output_multiplier(item_id: str, attr: str, rarity: str) -> float:
    if item_id not in CONVERTED_LIFE_COMBAT_IDS:
        return 1.0
    base = CONVERTED_LIFE_COMBAT_OUTPUT_MULT_BY_RARITY.get(rarity, 0.55)
    return base * CONVERTED_LIFE_COMBAT_ATTR_MULT.get(attr, 0.88)


def adjusted_full_value_cap(attr: str, positive: bool, rarity: str, item_id: str, attr_units: dict[str, str], attr_unit_values: dict[str, float]) -> float:
    cap = full_value_cap(attr, positive, attr_units)
    if not positive:
        return cap
    multiplier = RARITY_FULL_CAP_MULT_BY_RARITY.get(rarity, 1.0)
    if item_id in PURE_BONUS_IDS:
        multiplier *= 1.10
    if item_id in MYTHIC_BOLD_IDS:
        multiplier *= mythic_bold_attr_multiplier(attr, attr_units, attr_unit_values)
    cap *= multiplier
    cap *= item_attr_cap_multiplier(item_id, attr, rarity)
    cap *= positive_range_multiplier(attr, rarity, attr_units)
    if attr_units.get(attr) == "percent":
        if attr in EFFICIENCY_ATTRS:
            limit = 0.45 if item_id in PURE_BONUS_IDS else 0.40
            if rarity in RARE_PLUS:
                limit *= 0.80
            if item_id in MYTHIC_BOLD_IDS:
                limit *= mythic_bold_attr_multiplier(attr, attr_units, attr_unit_values)
            return min(cap, limit)
        if attr == "alchemy_precision":
            return min(cap, ALCHEMY_PRECISION_FULL_OUTPUT_CAP_BY_RARITY.get(rarity, ALCHEMY_PRECISION_FULL_OUTPUT_CAP_BY_RARITY["common"]))
        if attr in HIGH_SENSITIVITY_REWARD_ATTRS:
            return min(cap, REWARD_FULL_OUTPUT_CAP_BY_RARITY.get(rarity, REWARD_FULL_OUTPUT_CAP_BY_RARITY["common"]))
        limit = 0.25 if item_id in PURE_BONUS_IDS else 0.22
        if rarity in RARE_PLUS and attr in DEFENSE_OUTPUT_ATTRS:
            limit *= 0.80
        if item_id in MYTHIC_BOLD_IDS:
            limit *= mythic_bold_attr_multiplier(attr, attr_units, attr_unit_values)
        return min(cap, limit)
    return cap


def build_effect(
    attr: str,
    budget: float,
    positive: bool,
    rarity: str,
    item_id: str,
    unit_basis: int,
    max_qty: int,
    positive_count: int,
    efficiency_count: int,
    attr_units: dict[str, str],
    attr_unit_values: dict[str, float],
) -> dict:
    basis_multiplier = BASIS_OUTPUT_MULT_BY_RARITY.get(rarity, BASIS_OUTPUT_MULT_BY_RARITY["common"])
    max_value = budget_to_value(attr, budget, attr_units, attr_unit_values) * basis_multiplier / max(1, unit_basis)
    if positive and attr == "luck" and item_id in LUCK_TEMPTATION_IDS:
        # Luck is intentionally not score-normal: its derived effects make it
        # feel far more valuable than ordinary primaries. This one material is
        # the explicit "temptation" case: one great upside, several hard costs.
        max_value = 1.0 / max(1, max_qty)
        min_value = 0.4 / max(1, max_qty)
        return {"attr": attr, "valuePerUnit": {"min": min_value, "max": max_value}}

    if positive:
        max_value *= positive_range_multiplier(attr, rarity, attr_units)
        max_value *= material_quantity_output_scale(item_id, rarity)
        max_value *= converted_life_combat_output_multiplier(item_id, attr, rarity)
        if item_id in MYTHIC_BOLD_IDS:
            max_value *= mythic_bold_attr_multiplier(attr, attr_units, attr_unit_values)
        cap = adjusted_full_value_cap(attr, True, rarity, item_id, attr_units, attr_unit_values) * positive_pool_share(positive_count) / max(1, max_qty)
        if attr in EFFICIENCY_ATTRS:
            cap = min(cap, efficiency_output_cap(rarity, efficiency_count, positive_count) / max(1, max_qty))
        jitter = 0.96 + (stable_int(f"{item_id}:{attr}:positive:jitter") % 9) / 100
        if item_id in MYTHIC_BOLD_IDS:
            jitter = 1.00 + (stable_int(f"{item_id}:{attr}:mythic:jitter") % 10) / 100
        max_value = min(max_value * jitter, cap)
        floor_ratio = 0.36 + (stable_int(f"{item_id}:{attr}:floor") % 9) / 100
        return {
            "attr": attr,
            "valuePerUnit": {
                "min": round_smart(max_value * floor_ratio),
                "max": round_smart(max_value),
            },
        }

    neg_multiplier = negative_range_multiplier(rarity)
    max_value *= neg_multiplier
    cap = full_value_cap(attr, False, attr_units) * neg_multiplier / max(1, max_qty)
    max_value *= material_quantity_output_scale(item_id, rarity)
    if item_id == "base4_ancestor_shadow_jade" and attr == "speed":
        max_value *= 36.0
        cap *= 36.0
    max_value = min(max_value, cap)
    relief_ratio = 0.58 + (stable_int(f"{item_id}:{attr}:negative:relief") % 15) / 100
    nmin = -round_smart(max_value)
    nmax = -round_smart(max_value * relief_ratio)
    return {"attr": attr, "valuePerUnit": {"min": nmin, "max": nmax}}


def split_budgets(total: float, attrs: list[str], seed: str) -> list[float]:
    if not attrs:
        return []
    if len(attrs) == 1:
        return [total]
    raw_weights = [0.90 + (stable_int(f"{seed}:{attr}") % 31) / 100 for attr in attrs]
    raw_total = sum(raw_weights)
    return [total * w / raw_total for w in raw_weights]


def soften_bound_alchemy_penalties(effects: list[dict], positive_attr: str, penalty_attrs: list[str], max_qty: int, seed: str) -> None:
    positive = next((eff for eff in effects if eff["attr"] == positive_attr and eff["valuePerUnit"]["max"] > 0), None)
    if not positive:
        return
    positive_full_max = positive["valuePerUnit"]["max"] * max(1, max_qty)
    if positive_full_max <= 0:
        return
    for penalty_attr in penalty_attrs:
        penalty = next((eff for eff in effects if eff["attr"] == penalty_attr and eff["valuePerUnit"]["max"] < 0), None)
        if not penalty:
            continue
        value = penalty["valuePerUnit"]
        current_downside = abs(value["min"] * max(1, max_qty))
        if current_downside <= 0:
            continue
        target_ratio = 0.40 + (stable_int(f"{seed}:{positive_attr}:{penalty_attr}") % 41) / 100
        target_downside = positive_full_max * target_ratio
        if current_downside <= target_downside:
            continue
        scale = target_downside / current_downside
        value["min"] = round_smart(value["min"] * scale)
        value["max"] = round_smart(value["max"] * scale)


def minimum_penalty_score_ratio(item_id: str, rarity: str, effects: list[dict]) -> float:
    positive_attrs = {
        eff["attr"]
        for eff in effects
        if eff["valuePerUnit"]["max"] > 0
    }
    if positive_attrs & {"alchemy_precision", "alchemy_craft_success"}:
        return 0.0
    if item_id == "base4_ancestor_shadow_jade":
        return 0.0
    if item_id in MYTHIC_BOLD_IDS:
        return 0.08
    if item_id in OVERCHARGED_IDS:
        return 0.18
    if item_id in LUCK_TEMPTATION_IDS:
        return 0.35
    if rarity in {"epic", "legendary", "mythic"}:
        return 0.18
    if rarity == "rare":
        return 0.16
    return 0.12


def penalty_score_scale_cap(item_id: str, rarity: str) -> float:
    if item_id in MYTHIC_BOLD_IDS:
        return 40.0
    if item_id in OVERCHARGED_IDS or item_id in LUCK_TEMPTATION_IDS:
        return 10.0
    if rarity in {"rare", "epic", "legendary", "mythic"}:
        return 8.0
    return 6.0


def strengthen_underpowered_penalties(
    effects: list[dict],
    item_id: str,
    rarity: str,
    max_qty: int,
    attr_units: dict[str, str],
    attr_unit_values: dict[str, float],
) -> None:
    target_ratio = minimum_penalty_score_ratio(item_id, rarity, effects)
    if target_ratio <= 0:
        return
    positive_score = 0.0
    negative_score = 0.0
    negatives: list[dict] = []
    for eff in effects:
        value = eff["valuePerUnit"]
        full_value = (value["max"] if abs(value["max"]) >= abs(value["min"]) else value["min"]) * max(1, max_qty)
        score = attr_score_value(eff["attr"], full_value, attr_units, attr_unit_values)
        if value["max"] > 0:
            positive_score += score
        else:
            negative_score += score
            negatives.append(eff)
    if positive_score <= 0 or negative_score <= 0 or not negatives:
        return
    current_ratio = negative_score / positive_score
    if current_ratio >= target_ratio:
        return
    scale = min(target_ratio / max(0.000001, current_ratio), penalty_score_scale_cap(item_id, rarity))
    if scale <= 1:
        return
    for eff in negatives:
        value = eff["valuePerUnit"]
        value["min"] = round_smart(value["min"] * scale)
        value["max"] = round_smart(value["max"] * scale)


def rebalance_core_alchemy_effects(effects: list[dict]) -> None:
    has_core_positive = any(
        eff["attr"] in ALCHEMY_CORE_POSITIVE_ATTRS and eff["valuePerUnit"]["max"] > 0
        for eff in effects
    )
    if not has_core_positive:
        return
    for eff in effects:
        value = eff["valuePerUnit"]
        if eff["attr"] in ALCHEMY_CORE_POSITIVE_ATTRS and value["max"] > 0:
            value["min"] = round_smart(value["min"] * ALCHEMY_CORE_POSITIVE_MULT)
            value["max"] = round_smart(value["max"] * ALCHEMY_CORE_POSITIVE_MULT)
        elif value["max"] < 0:
            value["min"] = round_smart(value["min"] * ALCHEMY_CORE_PENALTY_MULT)
            value["max"] = round_smart(value["max"] * ALCHEMY_CORE_PENALTY_MULT)


def build_special_trait_row(item_id: str) -> dict:
    trait = SPECIAL_TRAIT_OVERRIDES.get(item_id)
    if trait is None:
        raise KeyError(f"No special trait override for item: {item_id}")
    return {
        "id": f"traitset.{item_id}",
        "itemId": item_id,
        "traits": [copy.deepcopy(trait)],
        "disabled": False,
    }


def build_trait(
    item: dict,
    value_q_by_id: dict[str, float],
    attr_units: dict[str, str],
    attr_unit_values: dict[str, float],
    used_positive_combos: set[str],
) -> dict:
    iid = item["id"]
    if iid in SPECIAL_TRAIT_OVERRIDES:
        trait = SPECIAL_TRAIT_OVERRIDES[iid]
        positive_attrs = [
            effect["attr"]
            for effect in trait["effects"]
            if effect["valuePerUnit"]["max"] > 0
        ]
        used_positive_combos.add(positive_combo_key("combat", positive_attrs))
        return build_special_trait_row(iid)

    rarity = item.get("rarity", "common")
    if rarity not in TIER:
        rarity = "common"
    value_q = value_q_by_id.get(iid, 0.5)
    kind = pick_kind(iid, item)
    if kind == "life":
        profile_id = pick_life_profile(iid)
        profile = LIFE_PROFILES[profile_id]
        all_attrs = ALL_LIFE_ATTRS
    else:
        profile_id = pick_combat_profile(iid)
        profile = COMBAT_PROFILES[profile_id]
        all_attrs = ALL_COMBAT_ATTRS

    unit_basis = UNIT_BASIS_BY_RARITY.get(rarity, 50)
    max_basis_multiplier = material_max_basis_multiplier(iid, rarity, unit_basis)
    max_qty = unit_basis * max_basis_multiplier
    profile_pos = filter_positive_attrs_for_rarity(remove_disallowed_precision(iid, list(profile["pos"])), rarity)
    usable_all_attrs = filter_positive_attrs_for_rarity(remove_disallowed_precision(iid, list(all_attrs)), rarity)
    desired_count = positive_effect_count(rarity, iid, value_q)
    forceable_attrs = remove_disallowed_precision(iid, list(dict.fromkeys(list(profile["pos"]) + list(all_attrs))))
    forced_pos_attrs = [
        attr for attr in FORCED_POSITIVE_ATTRS_BY_ID.get(iid, [])
        if attr in forceable_attrs
    ]
    if len(forced_pos_attrs) > desired_count:
        forced_pos_attrs = forced_pos_attrs[:desired_count]
    profile_pos = list(dict.fromkeys(forced_pos_attrs + profile_pos))
    if not profile_pos:
        profile_pos = usable_all_attrs

    pos_attrs = stable_pick(profile_pos, f"{iid}:pos", desired_count, required=forced_pos_attrs or profile_pos[:1])
    if "alchemy_precision" in pos_attrs:
        pos_attrs = [attr for attr in pos_attrs if attr not in {"alchemy_efficiency", "alchemy_craft_success"}]
    if "alchemy_craft_success" in pos_attrs:
        pos_attrs = [attr for attr in pos_attrs if attr not in {"alchemy_efficiency", "alchemy_precision"}]
    if len(pos_attrs) < desired_count:
        pos_pool = list(dict.fromkeys(profile_pos + usable_all_attrs))
        if "alchemy_precision" in pos_attrs:
            pos_pool = [attr for attr in pos_pool if attr not in {"alchemy_efficiency", "alchemy_craft_success"}]
        if "alchemy_craft_success" in pos_attrs:
            pos_pool = [attr for attr in pos_pool if attr not in {"alchemy_efficiency", "alchemy_precision"}]
        pos_attrs = stable_pick(pos_pool, f"{iid}:pos:fallback", desired_count, required=forced_pos_attrs + pos_attrs)
    pos_attrs = diversify_positive_attrs(kind, iid, desired_count, pos_attrs, profile_pos, usable_all_attrs, used_positive_combos, forced_pos_attrs)
    used_positive_combos.add(positive_combo_key(kind, pos_attrs))
    precision_positive = "alchemy_precision" in pos_attrs
    stability_positive = "alchemy_craft_success" in pos_attrs

    total_budget = value_budget(item, value_q)
    if iid in LIFE_PRIMARY_IDS:
        total_budget *= LIFE_PRIMARY_MULTIPLIER
    has_downside = has_penalty(rarity, iid, value_q) or precision_positive or stability_positive
    if has_downside:
        total_budget *= 1.24
    if iid in PURE_BONUS_IDS:
        total_budget *= 1.12
    if iid in OVERCHARGED_IDS:
        total_budget *= 1.28
    if iid in MYTHIC_BOLD_IDS:
        total_budget *= 1.45

    efficiency_count = sum(1 for attr in pos_attrs if attr in EFFICIENCY_ATTRS)
    pos_budgets = split_budgets(total_budget, pos_attrs, f"{iid}:pos:budget")
    effects = [
        build_effect(attr, budget, True, rarity, iid, unit_basis, max_qty, len(pos_attrs), efficiency_count, attr_units, attr_unit_values)
        for attr, budget in zip(pos_attrs, pos_budgets)
    ]

    if has_downside:
        required_neg_attrs = []
        if precision_positive:
            required_neg_attrs.extend(["alchemy_efficiency", "alchemy_craft_success"])
        if stability_positive:
            required_neg_attrs.extend(["alchemy_efficiency", "alchemy_precision"])
        if iid == "base4_ancestor_shadow_jade":
            required_neg_attrs.append("speed")
        required_neg_attrs = [attr for attr in dict.fromkeys(required_neg_attrs) if attr not in pos_attrs]
        neg_pool = [attr for attr in list(dict.fromkeys(required_neg_attrs + profile["neg"])) if attr not in pos_attrs]
        if not neg_pool:
            neg_pool = [attr for attr in list(dict.fromkeys(required_neg_attrs + profile["neg"] + usable_all_attrs)) if attr not in pos_attrs]
        neg_count = min(max(penalty_count(rarity, iid), len(required_neg_attrs)), len(neg_pool))
        neg_attrs = stable_pick(neg_pool, f"{iid}:neg", neg_count, required=required_neg_attrs)
        neg_ratio = NEG_RATIO_BY_RARITY.get(rarity, 0.35)
        if iid in OVERCHARGED_IDS:
            neg_ratio = 0.70
        if iid in LUCK_TEMPTATION_IDS:
            neg_ratio = 1.80
        neg_ratio *= NEGATIVE_PENALTY_GLOBAL_MULT
        neg_budgets = split_budgets(total_budget * neg_ratio, neg_attrs, f"{iid}:neg:budget")
        effects.extend(
            build_effect(attr, budget, False, rarity, iid, unit_basis, max_qty, len(pos_attrs), efficiency_count, attr_units, attr_unit_values)
            for attr, budget in zip(neg_attrs, neg_budgets)
        )
        if precision_positive:
            soften_bound_alchemy_penalties(effects, "alchemy_precision", ["alchemy_efficiency", "alchemy_craft_success"], max_qty, iid)
        if stability_positive:
            soften_bound_alchemy_penalties(effects, "alchemy_craft_success", ["alchemy_efficiency", "alchemy_precision"], max_qty, iid)
        strengthen_underpowered_penalties(effects, iid, rarity, max_qty, attr_units, attr_unit_values)
    rebalance_core_alchemy_effects(effects)

    tier = TIER[rarity]
    max_per_equipment, max_per_body = material_affix_limits(iid, rarity)
    gold_cost, duration_sec, alchemy_exp = material_economy(item, value_q)
    return {
        "id": f"traitset.{iid}",
        "itemId": iid,
        "traits": [{
            "traitId": iid,
            "kind": kind,
            "family": profile["family"],
            "effectType": "add_stat",
            "effects": effects,
            "unitBasis": unit_basis,
            "maxBasisMultiplier": max_basis_multiplier,
            "goldCostPerUnit": gold_cost,
            "durationSecPerUnit": duration_sec,
            "alchemyExpPerUnit": alchemy_exp,
            "minAlchemyLevel": tier["level"],
            "failBaseChance": tier["fail"],
            "failChancePerLevelAbove": tier["fail_per_lvl"],
            "failChancePerExtraBasis": FAIL_PER_EXTRA_BASIS_BY_RARITY.get(rarity, FAIL_PER_EXTRA_BASIS_BY_RARITY["common"]),
            "failChanceQuantityCap": FAIL_QUANTITY_CAP_BY_RARITY.get(rarity, FAIL_QUANTITY_CAP_BY_RARITY["common"]),
            "maxEffectivePerEquipment": max_per_equipment,
            "maxEffectivePerBody": max_per_body,
        }],
    }


def material_affix_limits(item_id: str, rarity: str) -> tuple[int, int]:
    body_limit = MATERIAL_EFFECTIVE_BODY_LIMIT_BY_RARITY.get(rarity, MATERIAL_EFFECTIVE_BODY_LIMIT_BY_RARITY["common"])
    if item_id in LUCK_TEMPTATION_IDS:
        return (1, body_limit)
    if item_id in STRICT_LIMIT_IDS:
        return (1, body_limit)
    if rarity in {"epic", "legendary"} and (item_id in PURE_BONUS_IDS or item_id in OVERCHARGED_IDS):
        return (1, body_limit)
    return (2, body_limit)


def material_max_basis_multiplier(item_id: str, rarity: str, unit_basis: int) -> int:
    cap = material_max_quantity_cap(item_id, rarity)
    return max(1, cap // max(1, unit_basis))


def material_economy(item: dict, value_q: float) -> tuple[int, float, float]:
    rarity = item.get("rarity", "common")
    rarity_index = max(0, RARITY_ORDER.index(rarity) if rarity in RARITY_ORDER else 0)
    value = max(1.0, float(item.get("economyBaseValue") or 1))
    value_scale = value ** 0.5
    rarity_scale = 1.0 + rarity_index * 0.62
    q_scale = 0.95 + value_q * 0.45
    gold = max(1, int(round((7.5 + value_scale * 0.55) * rarity_scale * q_scale * MATERIAL_GOLD_COST_MULT)))
    duration = round((0.12 + rarity_index * 0.035) * (0.85 + value_q * 0.30), 3)
    exp = round(duration / 8 * 0.72, 4)
    return gold, duration, exp


def pick_prefix(item_id: str, rarity: str, kind: str) -> str:
    if item_id in PREFIX_BY_ID:
        return PREFIX_BY_ID[item_id]
    if kind == "life":
        profile = LIFE_PROFILES[pick_life_profile(item_id)]
    else:
        profile = COMBAT_PROFILES[pick_combat_profile(item_id)]
    return profile["prefix"].get(rarity, profile["prefix"]["common"])


def validate_trait_rows(
    trait_rows: list[dict],
    attr_units: dict[str, str],
    attr_unit_values: dict[str, float],
) -> dict[str, float]:
    max_pos = 0
    max_effects = 0
    scored_rows = []
    level_to_rarity = {cfg["level"]: rarity for rarity, cfg in TIER.items()}
    positive_efficiency_rarities: dict[str, set[str]] = {}
    positive_efficiency_sources: dict[str, list[str]] = {}
    positive_reward_sources: dict[str, list[str]] = {}
    seen_positive_combos: dict[str, str] = {}
    for row in trait_rows:
        iid = row["itemId"]
        trait = row["traits"][0]
        rarity = level_to_rarity.get(trait.get("minAlchemyLevel"), "common")
        for key in ("goldCostPerUnit", "durationSecPerUnit", "alchemyExpPerUnit"):
            value = trait.get(key)
            if not isinstance(value, (int, float)) or value < 0:
                raise ValueError(f"Invalid {key} on trait {iid}: {value}")
        for key in ("failChancePerExtraBasis", "failChanceQuantityCap"):
            value = trait.get(key)
            if not isinstance(value, (int, float)) or value < 0 or value > 1:
                raise ValueError(f"Invalid {key} on trait {iid}: {value}")
        seen = set()
        pos = 0
        neg = 0
        pos_score = 0.0
        neg_score = 0.0
        for eff in trait["effects"]:
            attr = eff["attr"]
            if attr in seen:
                raise ValueError(f"Duplicate attr in trait {iid}: {attr}")
            seen.add(attr)
            if trait["kind"] == "life" and attr not in WORK_ATTRS:
                raise ValueError(f"Life trait used non-work attr {iid}.{attr}")
            if trait["kind"] == "combat" and attr not in BATTLE_ATTRS:
                raise ValueError(f"Combat trait used non-battle attr {iid}.{attr}")
            lo = eff["valuePerUnit"]["min"]
            hi = eff["valuePerUnit"]["max"]
            if lo > hi:
                raise ValueError(f"Invalid value range {iid}.{attr}: {lo}>{hi}")
            score = attr_score_value(attr, hi if abs(hi) >= abs(lo) else lo, attr_units, attr_unit_values)
            if hi > 0:
                pos += 1
                pos_score += score
                if attr in HIGH_SENSITIVITY_REWARD_ATTRS:
                    if iid not in REWARD_TRAIT_ALLOWED_IDS:
                        raise ValueError(f"Reward attr {attr} may only appear on explicit combat loot materials: {iid}")
                    if trait["kind"] != "combat":
                        raise ValueError(f"Reward attr {attr} may only appear on combat traits: {iid}")
                    positive_reward_sources.setdefault(attr, []).append(iid)
                if attr in EFFICIENCY_ATTRS:
                    forced_attrs = set(FORCED_POSITIVE_ATTRS_BY_ID.get(iid, []))
                    if not allows_positive_attr_in_rarity(attr, rarity) and attr not in forced_attrs:
                        raise ValueError(f"Efficiency attr {attr} is not allowed as positive on {rarity} material {iid}")
                    if attr not in forced_attrs:
                        positive_efficiency_rarities.setdefault(attr, set()).add(rarity)
                    positive_efficiency_sources.setdefault(attr, []).append(iid)
            else:
                neg += 1
                neg_score += score
            if attr == "luck":
                if iid not in LUCK_TEMPTATION_IDS:
                    raise ValueError(f"luck may only appear on explicit temptation materials: {iid}")
                if pos != 1 or len([e for e in trait["effects"] if e["valuePerUnit"]["max"] > 0]) != 1:
                    raise ValueError(f"luck material must have exactly one positive effect: {iid}")
        precision_positive = any(eff["attr"] == "alchemy_precision" and eff["valuePerUnit"]["max"] > 0 for eff in trait["effects"])
        stability_positive = any(eff["attr"] == "alchemy_craft_success" and eff["valuePerUnit"]["max"] > 0 for eff in trait["effects"])
        positive_attrs = [eff["attr"] for eff in trait["effects"] if eff["valuePerUnit"]["max"] > 0]
        combo_key = positive_combo_key(trait["kind"], positive_attrs)
        if combo_key in seen_positive_combos:
            raise ValueError(f"Duplicate positive alchemy combo {combo_key}: {seen_positive_combos[combo_key]} and {iid}")
        seen_positive_combos[combo_key] = iid
        if precision_positive and iid not in ALCHEMY_PRECISION_SOURCE_IDS:
            raise ValueError(f"alchemy_precision may only appear on precision source materials: {iid}")
        if precision_positive and not any(eff["attr"] == "alchemy_efficiency" and eff["valuePerUnit"]["max"] < 0 for eff in trait["effects"]):
            raise ValueError(f"alchemy_precision material must reduce alchemy_efficiency: {iid}")
        if precision_positive and not any(eff["attr"] == "alchemy_craft_success" and eff["valuePerUnit"]["max"] < 0 for eff in trait["effects"]):
            raise ValueError(f"alchemy_precision material must reduce alchemy_craft_success: {iid}")
        if stability_positive and not any(eff["attr"] == "alchemy_efficiency" and eff["valuePerUnit"]["max"] < 0 for eff in trait["effects"]):
            raise ValueError(f"alchemy_craft_success material must reduce alchemy_efficiency: {iid}")
        if stability_positive and not any(eff["attr"] == "alchemy_precision" and eff["valuePerUnit"]["max"] < 0 for eff in trait["effects"]):
            raise ValueError(f"alchemy_craft_success material must reduce alchemy_precision: {iid}")
        if pos > 2:
            raise ValueError(f"Too many positive effects on {iid}: {pos}")
        if iid in LUCK_TEMPTATION_IDS and neg < 2:
            raise ValueError(f"luck temptation material needs multiple penalties: {iid}")
        max_pos = max(max_pos, pos)
        max_effects = max(max_effects, len(trait["effects"]))
        scored_rows.append((pos_score, neg_score, iid))
    for attr, rarities in positive_efficiency_rarities.items():
        allowed = EFFICIENCY_POSITIVE_RARITY_BANDS.get(attr, set())
        if not rarities <= allowed:
            raise ValueError(f"Efficiency attr {attr} leaked into unexpected rarities: {sorted(rarities - allowed)}")
        if len(rarities) > 2:
            raise ValueError(f"Efficiency attr {attr} spans too many rarity bands: {sorted(rarities)}")
    for attr, sources in positive_efficiency_sources.items():
        if len(sources) > 3:
            raise ValueError(f"Efficiency attr {attr} has too many material sources ({len(sources)}): {sources}")
    for attr, sources in positive_reward_sources.items():
        if len(sources) > 2:
            raise ValueError(f"Reward attr {attr} has too many material sources ({len(sources)}): {sources}")
    scored_rows.sort(reverse=True)
    return {
        "max_pos": float(max_pos),
        "max_effects": float(max_effects),
        "top_pos_score": scored_rows[0][0] if scored_rows else 0.0,
    }


def merge_special_trait_overrides(existing_rows: list[dict]) -> list[dict]:
    merged = []
    replaced = set()
    for row in existing_rows:
        item_id = row.get("itemId")
        if item_id in SPECIAL_TRAIT_OVERRIDES:
            merged.append(build_special_trait_row(item_id))
            replaced.add(item_id)
        else:
            merged.append(row)
    for item_id in SPECIAL_TRAIT_OVERRIDES:
        if item_id not in replaced:
            merged.append(build_special_trait_row(item_id))
    return merged


def validate_special_trait_overrides(
    items: list[dict],
    existing_rows: list[dict],
    attr_units: dict[str, str],
    attr_unit_values: dict[str, float],
) -> dict[str, float]:
    if len(SPECIAL_TRAIT_OVERRIDES) != 17:
        raise ValueError(f"Expected 17 special trait overrides, got {len(SPECIAL_TRAIT_OVERRIDES)}")

    items_by_id = {item["id"]: item for item in items}
    missing_items = [item_id for item_id in SPECIAL_TRAIT_OVERRIDES if item_id not in items_by_id]
    if missing_items:
        raise ValueError(f"Special trait items are missing from items.json: {missing_items}")
    rejected_items = [
        item_id
        for item_id in SPECIAL_TRAIT_OVERRIDES
        if not is_drop(items_by_id[item_id]) or not allows_trait_material(items_by_id[item_id])
    ]
    if rejected_items:
        raise ValueError(f"Special trait items are not eligible material drops: {rejected_items}")

    special_rows = [build_special_trait_row(item_id) for item_id in SPECIAL_TRAIT_OVERRIDES]
    audit = validate_trait_rows(special_rows, attr_units, attr_unit_values)

    existing_combo_sources: dict[str, list[str]] = {}
    for row in existing_rows:
        item_id = row.get("itemId")
        if item_id in SPECIAL_TRAIT_OVERRIDES:
            continue
        traits = row.get("traits")
        if not isinstance(traits, list) or not traits:
            continue
        trait = traits[0]
        positive_attrs = [
            effect["attr"]
            for effect in trait.get("effects", [])
            if effect.get("valuePerUnit", {}).get("max", 0) > 0
        ]
        combo_key = positive_combo_key(trait.get("kind", ""), positive_attrs)
        existing_combo_sources.setdefault(combo_key, []).append(str(item_id))

    for row in special_rows:
        trait = row["traits"][0]
        if trait["kind"] != "combat" or trait["effectType"] != "add_stat":
            raise ValueError(f"Special trait must be combat/add_stat: {row['itemId']}")
        positive_attrs = [
            effect["attr"]
            for effect in trait["effects"]
            if effect["valuePerUnit"]["max"] > 0
        ]
        combo_key = positive_combo_key(trait["kind"], positive_attrs)
        if combo_key in existing_combo_sources:
            raise ValueError(
                f"Special trait positive combo collides with existing materials: "
                f"{row['itemId']} and {existing_combo_sources[combo_key]}"
            )
    return audit


def capture_special_name_affixes(items_by_id: dict[str, dict]) -> dict[str, dict[str, str]]:
    captured = {}
    for item_id in SPECIAL_TRAIT_OVERRIDES:
        item = items_by_id.get(item_id)
        alchemy_meta = item.get("alchemy") if isinstance(item, dict) else None
        name_affix = alchemy_meta.get("nameAffix") if isinstance(alchemy_meta, dict) else None
        if not isinstance(name_affix, dict):
            raise ValueError(f"Special trait item needs an explicit alchemy.nameAffix: {item_id}")
        prefix_key = name_affix.get("prefixKey")
        suffix_key = name_affix.get("suffixKey")
        if not isinstance(prefix_key, str) or not prefix_key.startswith("alchemy.affix.prefix."):
            raise ValueError(f"Invalid explicit prefixKey on special trait item {item_id}: {prefix_key}")
        if not isinstance(suffix_key, str) or not suffix_key.startswith("alchemy.affix.suffix."):
            raise ValueError(f"Invalid explicit suffixKey on special trait item {item_id}: {suffix_key}")
        captured[item_id] = copy.deepcopy(name_affix)
    return captured


def file_sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def self_check_special_name_affixes() -> None:
    source_doc = json.load(open(ITEMS_PATH))
    source_items_by_id = {item["id"]: item for item in source_doc["items"]}
    expected_affixes = capture_special_name_affixes(source_items_by_id)

    with tempfile.TemporaryDirectory(prefix="alchemy-traits-self-check-") as temp_dir:
        temp_root = Path(temp_dir)
        temp_items_path = temp_root / "items.json"
        temp_traits_path = temp_root / "item_traits.json"
        shutil.copy2(ITEMS_PATH, temp_items_path)
        shutil.copy2(TRAITS_PATH, temp_traits_path)

        env = os.environ.copy()
        env["ALCHEMY_ITEMS_PATH"] = str(temp_items_path)
        env["ALCHEMY_TRAITS_PATH"] = str(temp_traits_path)
        command = [sys.executable, str(Path(__file__).resolve())]

        subprocess.run(command, env=env, check=True, capture_output=True, text=True)
        first_doc = json.load(open(temp_items_path))
        first_items_by_id = {item["id"]: item for item in first_doc["items"]}
        first_affixes = capture_special_name_affixes(first_items_by_id)
        if first_affixes != expected_affixes:
            raise ValueError("Full generation changed one or more special alchemy nameAffix keys")
        first_traits_doc = json.load(open(temp_traits_path))
        for row in first_traits_doc["item_traits"]:
            item_id = row["itemId"]
            if item_id in SPECIAL_TRAIT_OVERRIDES:
                continue
            item = first_items_by_id[item_id]
            trait = row["traits"][0]
            prefix_slug = pick_prefix(item_id, item.get("rarity", "common"), trait["kind"])
            expected_name_affix = {
                "prefixKey": f"alchemy.affix.prefix.{prefix_slug}",
                "suffixKey": f"alchemy.affix.suffix.{pick_suffix(item_id)}",
            }
            actual_name_affix = item.get("alchemy", {}).get("nameAffix")
            if actual_name_affix != expected_name_affix:
                raise ValueError(f"Non-special nameAffix behavior changed for item: {item_id}")
        first_hashes = (file_sha256(temp_items_path), file_sha256(temp_traits_path))

        subprocess.run(command, env=env, check=True, capture_output=True, text=True)
        second_doc = json.load(open(temp_items_path))
        second_items_by_id = {item["id"]: item for item in second_doc["items"]}
        second_affixes = capture_special_name_affixes(second_items_by_id)
        second_hashes = (file_sha256(temp_items_path), file_sha256(temp_traits_path))
        if second_affixes != expected_affixes:
            raise ValueError("Second full generation changed one or more special alchemy nameAffix keys")
        if second_hashes != first_hashes:
            raise ValueError(
                f"Full generation is not idempotent: first={first_hashes}, second={second_hashes}"
            )

    print(
        f"Self-check passed: {len(expected_affixes)} explicit special nameAffix pairs preserved; "
        f"isolated full generation is idempotent ({first_hashes[0][:12]}/{first_hashes[1][:12]})."
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--special-overrides-only",
        action="store_true",
        help="merge the 17 hand-authored dungeon traits without rewriting items.json or existing traits",
    )
    parser.add_argument(
        "--self-check-special-affixes",
        action="store_true",
        help="run full generation twice in an isolated temp copy and verify special nameAffix stability",
    )
    args = parser.parse_args()

    if args.self_check_special_affixes:
        self_check_special_name_affixes()
        return

    items_doc = json.load(open(ITEMS_PATH))
    items = items_doc["items"]
    attr_units = load_attribute_units()
    attr_unit_values = load_attribute_unit_values()

    if args.special_overrides_only:
        traits_doc = json.load(open(TRAITS_PATH))
        existing_rows = traits_doc["item_traits"]
        audit = validate_special_trait_overrides(items, existing_rows, attr_units, attr_unit_values)
        trait_rows = merge_special_trait_overrides(existing_rows)
        write_traits_document({**traits_doc, "item_traits": trait_rows})

        items_by_id = {item["id"]: item for item in items}
        missing_affixes = []
        for item_id in SPECIAL_TRAIT_OVERRIDES:
            alchemy_meta = items_by_id[item_id].get("alchemy")
            if not isinstance(alchemy_meta, dict) or not isinstance(alchemy_meta.get("nameAffix"), dict):
                missing_affixes.append(item_id)
        print(f"Merged {len(SPECIAL_TRAIT_OVERRIDES)} special trait overrides; total rows={len(trait_rows)}.")
        print(
            f"Audit: max positive effects={audit['max_pos']:.0f}, "
            f"max total effects={audit['max_effects']:.0f}, "
            f"top positive score={audit['top_pos_score']:.4f}"
        )
        if missing_affixes:
            print(f"Items still need alchemy.nameAffix metadata: {missing_affixes}")
        return

    by_item_id = {it["id"]: it for it in items}
    special_name_affixes = capture_special_name_affixes(by_item_id)
    drops = [it for it in items if is_drop(it) and allows_trait_material(it)]
    value_q_by_id = value_quantiles(drops)

    used_positive_combos = {
        positive_combo_key(
            "combat",
            [
                effect["attr"]
                for effect in trait["effects"]
                if effect["valuePerUnit"]["max"] > 0
            ],
        )
        for trait in SPECIAL_TRAIT_OVERRIDES.values()
    }
    trait_rows = [build_trait(it, value_q_by_id, attr_units, attr_unit_values, used_positive_combos) for it in drops]
    audit = validate_trait_rows(trait_rows, attr_units, attr_unit_values)
    write_traits_document({"version": 1, "item_traits": trait_rows})

    new_prefixes = set()
    new_suffixes = set()
    trait_item_ids = {row["itemId"] for row in trait_rows}
    for item in items:
        if item["id"] in trait_item_ids:
            continue
        alchemy_meta = item.get("alchemy")
        if not isinstance(alchemy_meta, dict) or "nameAffix" not in alchemy_meta:
            continue
        alchemy_meta = dict(alchemy_meta)
        alchemy_meta.pop("nameAffix", None)
        if alchemy_meta:
            item["alchemy"] = alchemy_meta
        else:
            item.pop("alchemy", None)

    for row in trait_rows:
        iid = row["itemId"]
        item = by_item_id[iid]
        trait = row["traits"][0]
        if iid in special_name_affixes:
            name_affix = copy.deepcopy(special_name_affixes[iid])
            prefix_slug = name_affix["prefixKey"].removeprefix("alchemy.affix.prefix.")
            suffix_slug = name_affix["suffixKey"].removeprefix("alchemy.affix.suffix.")
        else:
            prefix_slug = pick_prefix(iid, item.get("rarity", "common"), trait["kind"])
            suffix_slug = pick_suffix(iid)
            name_affix = {
                "prefixKey": f"alchemy.affix.prefix.{prefix_slug}",
                "suffixKey": f"alchemy.affix.suffix.{suffix_slug}",
            }
        alchemy_meta = item.get("alchemy") if isinstance(item.get("alchemy"), dict) else {}
        item["alchemy"] = {
            **alchemy_meta,
            "nameAffix": name_affix,
        }
        new_prefixes.add(prefix_slug)
        new_suffixes.add(suffix_slug)

    with open(ITEMS_PATH, "w") as f:
        json.dump(items_doc, f, indent=2, ensure_ascii=False)
        f.write("\n")

    print(f"Generated {len(trait_rows)} trait rows.")
    print(f"Audit: max positive effects={audit['max_pos']:.0f}, max total effects={audit['max_effects']:.0f}, top positive score={audit['top_pos_score']:.4f}")
    print(f"Prefix slugs ({len(new_prefixes)}): {sorted(new_prefixes)}")
    print(f"Suffix slugs ({len(new_suffixes)}): {sorted(new_suffixes)}")


if __name__ == "__main__":
    main()
