#!/usr/bin/env python3
"""
Generate alchemy_groups.json by rarity tier.

Rules (per current alchemy design):
  - Life primary (mining/gathering/livestock raw drops) → life_primary_<rarity>
  - Battle loot (enemy drops) → battle_loot_<rarity>
  - Life secondary (smelted ingots, woven fabric, refined wood) → EXCLUDED
  - Skill books → skillbook_<skill_category>
  - Equipment / currency / collectibles / mediums → EXCLUDED

Within each group, weights are derived from item.economyBaseValue. The convert
action's expected value should track the economic value of each item, so higher
priced drops are rarer rolls inside their tier.

Output: packages/game-data/source/json/alchemy_groups.json (full rewrite).
"""
import json
import math
import random
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ITEMS_PATH = ROOT / "packages/game-data/source/json/items.json"
TRAITS_PATH = ROOT / "packages/game-data/source/json/item_traits.json"
ACTIONS_PATH = ROOT / "packages/game-data/source/json/actions.json"
SKILLS_PATH = ROOT / "packages/game-data/source/json/skills.json"
SKILL_TYPES_PATH = ROOT / "packages/game-data/source/json/skill_type.json"
GROUPS_PATH = ROOT / "packages/game-data/source/json/alchemy_groups.json"
SHARED_CONFIG_PATH = ROOT / "packages/game-config/source/shared-global-config.json"

LIFE_PRIMARY_IDS = {
    # logging
    "wood", "sturdy_wood", "silverleaf_wood", "silkwood", "goldvine_wood", "moonwood",
    # fiber gathering
    "flax_fiber", "cotton_fiber", "silverleaf_fiber", "silkweed_fiber", "goldvine_fiber", "moonvine_fiber",
    # sap / resin gathering
    "tree_sap", "soft_resin", "silverleaf_resin", "clear_resin", "sun_resin", "moon_resin",
    # dew / nectar gathering
    "morning_dew", "sweet_nectar", "silverleaf_dew", "silkweed_nectar", "sunlit_nectar", "moon_dew",
    # mining
    "stone", "coal", "copper_ore", "iron_ore", "silver_ore", "gold_ore", "crystal_chunk",
    # livestock
    "egg", "milk", "wool",
}

LIFE_SECONDARY_IDS = {
    # processed wood blanks
    "wood_blank", "sturdy_wood_blank", "silverleaf_wood_blank", "silkwood_blank",
    "goldvine_wood_blank", "moonwood_blank",
    # smelted ingots
    "copper", "iron", "silver", "gold", "crystal",
    # woven fabrics
    "coarse_linen", "soft_cotton", "silver_vein_silk", "wisp_weave",
    "gilded_brocade", "lunar_veil",
}

# rarity order matches rarity_definitions
RARITY_ORDER = ["common", "uncommon", "rare", "epic", "legendary", "mythic"]

# Required minimum alchemy levels per rarity tier.
RARITY_MIN_LEVEL = {
    "common":    1,
    "uncommon":  8,
    "rare":      15,
    "epic":      20,
    "legendary": 25,
    "mythic":    35,
}

DEFAULT_GROUP_ECONOMY = {
    "common": {"gold": 45, "duration": 14, "exp": 2.0, "upgrade": 0.020, "fail": 0.15, "minFail": 0.15},
    "uncommon": {"gold": 105, "duration": 27, "exp": 3.5, "upgrade": 0.018, "fail": 0.15, "minFail": 0.15},
    "rare": {"gold": 245, "duration": 39, "exp": 5.0, "upgrade": 0.016, "fail": 0.15, "minFail": 0.15},
    "epic": {"gold": 550, "duration": 52, "exp": 6.5, "upgrade": 0.014, "fail": 0.18, "minFail": 0.15},
    "legendary": {"gold": 1200, "duration": 64, "exp": 8.0, "upgrade": 0.000, "fail": 0.22, "minFail": 0.15},
    "mythic": {"gold": 2400, "duration": 78, "exp": 10.0, "upgrade": 0.000, "fail": 0.26, "minFail": 0.15},
}

RARITY_FALLBACK_VALUE = {
    "common": 50,
    "uncommon": 120,
    "rare": 300,
    "epic": 700,
    "legendary": 1500,
    "mythic": 3000,
}

UPGRADE_CHANCE_PER_ENTRY = 0.025
UPGRADE_CHANCE_CAP = 0.10
LOW_TIER_LIFE_CONVERT_RARITIES = {"common", "uncommon"}
LOW_TIER_LIFE_CONVERT_MAIN_TYPES = {"collecting", "mining", "farming_animal"}


def as_number(value, fallback):
    if isinstance(value, (int, float)) and math.isfinite(value):
        return value
    if isinstance(value, str):
        try:
            parsed = float(value.strip())
            if math.isfinite(parsed):
                return parsed
        except ValueError:
            pass
    return fallback


def compact_number(value):
    return int(value) if isinstance(value, float) and value.is_integer() else value


def load_group_economy() -> dict:
    if not SHARED_CONFIG_PATH.exists():
        return DEFAULT_GROUP_ECONOMY
    with open(SHARED_CONFIG_PATH) as f:
        shared = json.load(f)
    raw = (((shared or {}).get("alchemy") or {}).get("convertGroupEconomyByRarity") or {})
    out = {}
    for rarity, fallback in DEFAULT_GROUP_ECONOMY.items():
        row = raw.get(rarity) if isinstance(raw, dict) else None
        row = row if isinstance(row, dict) else {}
        gold = max(0, int(round(as_number(row.get("gold"), fallback["gold"]))))
        duration = max(1, as_number(row.get("duration"), fallback["duration"]))
        exp = max(0, as_number(row.get("exp"), fallback["exp"]))
        upgrade = min(1, max(0, as_number(row.get("upgrade"), fallback["upgrade"])))
        fail = min(1, max(0, as_number(row.get("fail"), fallback["fail"])))
        min_fail = min(1, max(0, as_number(row.get("minFail"), fallback["minFail"])))
        out[rarity] = {
            "gold": gold,
            "duration": duration,
            "exp": exp,
            "upgrade": upgrade,
            "fail": fail,
            "minFail": min_fail,
            "failConfigured": "fail" in row,
            "minFailConfigured": "minFail" in row,
        }
    return out


def is_safe_alchemy_item(item: dict) -> bool:
    iid = str(item.get("id") or "").lower()
    types = item.get("type") if isinstance(item.get("type"), list) else []
    cat = str(item.get("category") or "").lower()
    alchemy = item.get("alchemy") if isinstance(item.get("alchemy"), dict) else {}
    if item.get("disabled") or item.get("isEquipment"):
        return False
    if alchemy.get("convertExcluded"):
        return False
    if cat in {"equipment", "alchemy_medium", "currency", "collectible", "quest", "map", "key"}:
        return False
    if any(t in types for t in ("trigger_item", "collectible", "skillbook", "currency", "map", "key", "quest")):
        return False
    if iid.startswith(("map_", "skillbook_", "key_", "world_boss", "boss_")) or "rumor" in iid:
        return False
    return True


def is_skillbook_alchemy_item(item: dict) -> bool:
    iid = str(item.get("id") or "").lower()
    types = item.get("type") if isinstance(item.get("type"), list) else []
    cat = str(item.get("category") or "").lower()
    alchemy = item.get("alchemy") if isinstance(item.get("alchemy"), dict) else {}
    if item.get("disabled") or item.get("isEquipment"):
        return False
    if alchemy.get("convertExcluded"):
        return False
    if cat not in {"", "skillbook"}:
        return False
    if "skillbook" not in types:
        return False
    if any(t in types for t in ("trigger_item", "collectible", "currency", "map", "key", "quest")):
        return False
    if iid.startswith(("map_", "key_", "boss_")) or "rumor" in iid:
        return False
    return True


def explicit_convert_weight(item: dict, computed_weight: float) -> int:
    alchemy = item.get("alchemy") if isinstance(item.get("alchemy"), dict) else {}
    weight = as_number(alchemy.get("convertWeight"), 0)
    if weight > 0:
        return max(1, int(round(weight)))
    multiplier = as_number(alchemy.get("convertWeightMultiplier"), 1)
    return max(1, int(round(computed_weight * multiplier)))


def has_explicit_convert_weight(item: dict) -> bool:
    alchemy = item.get("alchemy") if isinstance(item.get("alchemy"), dict) else {}
    return as_number(alchemy.get("convertWeight"), 0) > 0 or "convertWeightMultiplier" in alchemy


def entry_weight(item: dict) -> int:
    value = float(item.get("economyBaseValue") or 0)
    if value <= 0 or not math.isfinite(value):
        value = RARITY_FALLBACK_VALUE.get(str(item.get("rarity") or "common"), 50)
    return explicit_convert_weight(item, 1200 / math.sqrt(value))


def skillbook_entry_weight(item: dict) -> int:
    rarity = str(item.get("rarity") or "common")
    value = float(item.get("economyBaseValue") or 0)
    if not math.isfinite(value):
        value = 0
    value = max(value, RARITY_FALLBACK_VALUE.get(rarity, 50))
    return explicit_convert_weight(item, 1200 / math.sqrt(value))


def upgrade_entries_for_kind(
    buckets: dict,
    kind: str,
    rarity_idx: int,
    existing_group: dict,
    items_by_id: dict[str, dict],
) -> list[dict]:
    for next_idx in range(rarity_idx + 1, len(RARITY_ORDER)):
        next_rarity = RARITY_ORDER[next_idx]
        # Mythic materials remain authored rewards, never conversion upgrades.
        if next_rarity == "mythic":
            break
        next_entries = buckets[kind][next_rarity]
        if not next_entries:
            continue
        if existing_group is None:
            return list(next_entries[:4])

        candidates_by_id = {entry["itemId"]: entry for entry in next_entries}
        selected = []
        seen = set()
        for previous in existing_group.get("upgradeEntries", []):
            if not isinstance(previous, dict):
                continue
            item_id = str(previous.get("itemId") or "")
            candidate = candidates_by_id.get(item_id)
            if candidate is None or item_id in seen:
                continue
            selected.append(dict(candidate))
            seen.add(item_id)

        # Existing groups retain their curated upgrade pool. Only explicitly
        # weighted additions may extend it; otherwise a generator rerun would
        # silently expose every historical item in the next rarity bucket.
        for candidate in next_entries:
            item_id = candidate["itemId"]
            if item_id in seen or not has_explicit_convert_weight(items_by_id.get(item_id) or {}):
                continue
            selected.append(dict(candidate))
            seen.add(item_id)
        return selected
    return []


def weighted_pick_entry(entries: list[dict], seed: str):
    if not entries:
        return None
    rng = random.Random(seed)
    total = sum(max(0, float(entry.get("weight") or 0)) for entry in entries)
    if total <= 0:
        return entries[0]
    roll = rng.random() * total
    acc = 0.0
    for entry in entries:
        acc += max(0, float(entry.get("weight") or 0))
        if roll <= acc:
            return entry
    return entries[-1]


def low_tier_life_convert_ids(actions_doc: dict, items_by_id: dict[str, dict]) -> set[str]:
    """Low-tier life conversion is action-output based, not infusion-trait based."""
    out = set()
    for action in actions_doc.get("actions", []):
        main_type = str(action.get("mainType") or action.get("main_type") or action.get("menu") or "")
        if main_type not in LOW_TIER_LIFE_CONVERT_MAIN_TYPES:
            continue
        rewards = action.get("itemReward")
        if not isinstance(rewards, dict):
            continue
        for iid in rewards.keys():
            if iid not in LIFE_PRIMARY_IDS:
                continue
            item = items_by_id.get(iid)
            if item is None or not is_safe_alchemy_item(item):
                continue
            rarity = str(item.get("rarity") or "common")
            if rarity in LOW_TIER_LIFE_CONVERT_RARITIES:
                out.add(iid)
    return out


def external_skillbook_entries(
    skillbook_buckets: dict[str, list[dict]],
    category: str,
    ordered_categories: list[str],
    items_by_id: dict[str, dict],
) -> list[dict]:
    if category not in ordered_categories:
        return []
    start = ordered_categories.index(category)
    out = []
    for offset in range(1, len(ordered_categories)):
        other = ordered_categories[(start + offset) % len(ordered_categories)]
        entries = [
            entry
            for entry in (skillbook_buckets.get(other) or [])
            if str((items_by_id.get(entry["itemId"]) or {}).get("rarity") or "common") != "mythic"
        ]
        if not entries:
            continue
        picked = weighted_pick_entry(entries, f"{category}->{other}")
        if picked:
            out.append(dict(picked))
        if len(out) >= 2:
            break
    return out


def preserve_group_economy(group_id: str, fallback: dict, existing_groups: dict[str, dict]) -> dict:
    """Keep legacy failure curves until config defines them; other economics stay config-owned."""
    previous = existing_groups.get(group_id) or {}
    return {
        "gold": fallback["gold"],
        "duration": compact_number(fallback["duration"]),
        "exp": compact_number(fallback["exp"]),
        "fail": fallback["fail"] if fallback.get("failConfigured") else min(1, max(0, as_number(previous.get("failChanceBase"), fallback["fail"]))),
        "minFail": fallback["minFail"] if fallback.get("minFailConfigured") else min(1, max(0, as_number(previous.get("minFailChance"), fallback["minFail"]))),
    }


def preserve_entry_weights(group_id: str, field: str, entries: list[dict], existing_groups: dict[str, dict], items_by_id: dict[str, dict]) -> list[dict]:
    previous_rows = (existing_groups.get(group_id) or {}).get(field) or []
    previous_weights = {
        str(row.get("itemId") or ""): as_number(row.get("weight"), 0)
        for row in previous_rows
        if isinstance(row, dict)
    }
    entries_by_id = {str(entry.get("itemId") or ""): entry for entry in entries}
    ordered_entries = []
    seen = set()
    for previous in previous_rows:
        item_id = str(previous.get("itemId") or "") if isinstance(previous, dict) else ""
        if item_id and item_id in entries_by_id and item_id not in seen:
            ordered_entries.append(entries_by_id[item_id])
            seen.add(item_id)
    ordered_entries.extend(entry for entry in entries if str(entry.get("itemId") or "") not in seen)

    out = []
    for entry in ordered_entries:
        row = dict(entry)
        item = items_by_id.get(row["itemId"]) or {}
        previous_weight = previous_weights.get(row["itemId"], 0)
        if previous_weight > 0 and not has_explicit_convert_weight(item):
            row["weight"] = previous_weight
        out.append(row)
    return out


def main():
    items_doc = json.load(open(ITEMS_PATH))
    traits_doc = json.load(open(TRAITS_PATH))
    actions_doc = json.load(open(ACTIONS_PATH))
    skills_doc = json.load(open(SKILLS_PATH))
    skill_types_doc = json.load(open(SKILL_TYPES_PATH))
    group_economy = load_group_economy()
    existing_groups = {}
    if GROUPS_PATH.exists():
        existing_doc = json.load(open(GROUPS_PATH))
        existing_groups = {
            row["id"]: row
            for row in existing_doc.get("alchemy_groups", [])
            if isinstance(row, dict) and row.get("id")
        }
    existing_skillbook_ids = {
        str(entry.get("itemId") or "")
        for group_id, group in existing_groups.items()
        if group_id.startswith("group.skillbook_")
        for entry in group.get("entries", [])
        if isinstance(entry, dict) and entry.get("itemId")
    }

    items_by_id = {it["id"]: it for it in items_doc["items"]}
    skills_by_id = {row["id"]: row for row in skills_doc["skills"]}
    skill_type_order = [row["id"] for row in skill_types_doc["skill_type"] if not row.get("disabled")]
    # Trait-bearing items are eligible for infusion and conversion; low-tier life
    # action outputs keep conversion without becoming infusion materials.
    trait_ids = {row["itemId"] for row in traits_doc["item_traits"]}
    low_tier_life_ids = low_tier_life_convert_ids(actions_doc, items_by_id)
    convert_candidate_ids = trait_ids | low_tier_life_ids
    # Bucket items by category and rarity.
    buckets = {
        "battle_loot": {r: [] for r in RARITY_ORDER},
        "life_primary": {r: [] for r in RARITY_ORDER},
    }
    skillbook_buckets = {skill_type: [] for skill_type in skill_type_order}

    for iid in sorted(convert_candidate_ids):
        if iid in LIFE_SECONDARY_IDS:
            continue
        it = items_by_id.get(iid)
        if it is None or not is_safe_alchemy_item(it):
            continue
        rarity = it.get("rarity", "common")
        if rarity not in RARITY_ORDER:
            continue
        weight = entry_weight(it)
        if iid in LIFE_PRIMARY_IDS:
            if rarity in LOW_TIER_LIFE_CONVERT_RARITIES and iid not in low_tier_life_ids:
                continue
            buckets["life_primary"][rarity].append({"itemId": iid, "weight": weight})
        else:
            buckets["battle_loot"][rarity].append({"itemId": iid, "weight": weight})

    for it in sorted(items_doc["items"], key=lambda row: str(row.get("id") or "")):
        if not is_skillbook_alchemy_item(it):
            continue
        if it["id"] not in existing_skillbook_ids and not has_explicit_convert_weight(it):
            continue
        skill_id = str(it.get("useGrantSkillId") or "").strip()
        skill = skills_by_id.get(skill_id)
        if not skill:
            continue
        category = str(skill.get("category") or "").strip()
        if category not in skillbook_buckets:
            continue
        skillbook_buckets[category].append({"itemId": it["id"], "weight": skillbook_entry_weight(it)})

    groups = []
    for kind in ("battle_loot", "life_primary"):
        for rarity in RARITY_ORDER:
            if kind == "battle_loot" and rarity in {"legendary", "mythic"}:
                continue
            entries = buckets[kind][rarity]
            if not entries:
                continue
            gid = f"group.{kind}_{rarity}"
            entries = preserve_entry_weights(gid, "entries", entries, existing_groups, items_by_id)
            rarity_idx = RARITY_ORDER.index(rarity)
            existing_group = existing_groups.get(gid)
            next_entries = upgrade_entries_for_kind(buckets, kind, rarity_idx, existing_group, items_by_id)
            next_entries = preserve_entry_weights(gid, "upgradeEntries", next_entries, existing_groups, items_by_id)
            econ = preserve_group_economy(gid, group_economy[rarity], existing_groups)
            if gid == "group.battle_loot_epic" and next_entries:
                upgrade_chance = 0.05
            elif existing_group is not None:
                upgrade_chance = min(1, max(0, as_number(existing_group.get("upgradeChanceBase"), 0)))
            else:
                upgrade_chance = min(UPGRADE_CHANCE_CAP, UPGRADE_CHANCE_PER_ENTRY * len(next_entries)) if next_entries else 0
            group = {
                "id": gid,
                "nameKey": f"alchemy.group.{kind}_{rarity}.name",
                "descKey": f"alchemy.group.{kind}_{rarity}.desc",
                "minAlchemyLevel": RARITY_MIN_LEVEL[rarity],
                "goldCost": econ["gold"],
                "durationSec": econ["duration"],
                "alchemyExp": econ["exp"],
                "failChanceBase": econ["fail"],
                "minFailChance": econ["minFail"],
                "upgradeChanceBase": upgrade_chance,
                "upgradeEntries": next_entries,
                "entries": entries,
                "disabled": False,
            }
            groups.append(group)

    for category in skill_type_order:
        entries = skillbook_buckets.get(category) or []
        if not entries:
            continue
        gid = f"group.skillbook_{category}"
        entries = preserve_entry_weights(gid, "entries", entries, existing_groups, items_by_id)
        previous_upgrades = (existing_groups.get(gid) or {}).get("upgradeEntries") or []
        external_entries = [
            {"itemId": str(row.get("itemId") or ""), "weight": as_number(row.get("weight"), 0)}
            for row in previous_upgrades
            if isinstance(row, dict)
            and str(row.get("itemId") or "") in items_by_id
            and is_skillbook_alchemy_item(items_by_id[str(row.get("itemId") or "")])
            and str(items_by_id[str(row.get("itemId") or "")].get("rarity") or "common") != "mythic"
            and as_number(row.get("weight"), 0) > 0
        ]
        if not external_entries:
            external_entries = external_skillbook_entries(skillbook_buckets, category, skill_type_order, items_by_id)
        econ = preserve_group_economy(gid, group_economy["rare"], existing_groups)
        existing_group = existing_groups.get(gid)
        upgrade_chance = (
            min(1, max(0, as_number(existing_group.get("upgradeChanceBase"), 0)))
            if existing_group is not None
            else min(UPGRADE_CHANCE_CAP, UPGRADE_CHANCE_PER_ENTRY * len(external_entries))
        )
        groups.append({
            "id": gid,
            "nameKey": f"skill_type.{category}.name",
            "descKey": f"skill_type.{category}.desc",
            "minAlchemyLevel": RARITY_MIN_LEVEL["rare"],
            "goldCost": econ["gold"],
            "durationSec": econ["duration"],
            "alchemyExp": econ["exp"],
            "failChanceBase": econ["fail"],
            "minFailChance": 0.15,
            "upgradeChanceBase": upgrade_chance,
            "upgradeEntries": external_entries,
            "entries": entries,
            "disabled": False,
        })

    out = {"version": 1, "alchemy_groups": groups}
    with open(GROUPS_PATH, "w") as f:
        json.dump(out, f, indent=2, ensure_ascii=False)
        f.write("\n")

    print(f"Wrote {len(groups)} alchemy groups:")
    for g in groups:
        print(f"  {g['id']:36s} entries={len(g['entries']):2d}")


if __name__ == "__main__":
    main()
