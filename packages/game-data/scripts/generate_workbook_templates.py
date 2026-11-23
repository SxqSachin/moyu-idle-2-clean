#!/usr/bin/env python3
import os
import zipfile
from xml.sax.saxutils import escape
from typing import Dict, List

TEMPLATES = {
    "items.xlsx": {
        "items": [
            "id",
            "type",
            "rarity",
            "stackable",
            "carryableInCombat",
            "marketListable",
            "iconId",
            "emojiIcon",
            "nameKey",
            "descKey",
            "effectDescKey",
            "economyBaseValue",
            "useGrantSkillId",
            "useSkillExpGain",
        ],
        "item_types": ["id", "nameKey", "descKey"],
    },
    "actions.xlsx": {
        "actions": [
            "id",
            "mainType",
            "subType",
            "nameKey",
            "descKey",
            "durationS",
            "queueable",
            "economyIndustryTier",
            "economyDifficultyMultiplier",
            "costs",
            "itemReward",
            "statusExpReward",
            "buildingRequirement",
        ],
        "action_main_types": ["id", "nameKey", "descKey", "icon", "color", "order", "actions"],
        "action_sub_types": ["id", "nameKey", "descKey", "icon", "color", "order", "actions"],
    },
    "buildings.xlsx": {
        "buildings": [
            "id",
            "iconId",
            "nameKey",
            "descKey",
            "order",
            "maxLevel",
            "baseDurationS",
            "stepDurationS",
            "baseCosts",
            "stepCosts",
            "baseStatusRequirement",
            "stepStatusRequirement",
        ],
    },
    "combat.xlsx": {
        "skills": ["id", "name", "nameKey", "descKey", "category", "hpCost", "mpCost", "cooldown"],
        "skill_type": ["id", "nameKey", "descKey", "color"],
        "buff": ["id", "nameKey", "descKey", "durationTurns", "permanent", "kind", "dispellable", "type"],
        "buff_type": ["id", "nameKey", "descKey", "color"],
        "buff_effects": ["id", "buffId", "trigger", "priority", "conditions", "effects"],
        "combat_config_rules": ["base_skill_slots", "int_per_extra_slot", "min_skill_slots", "max_skill_slots", "default_basic_skill_id"],
        "skill_rule_sets": ["id", "ruleKind", "groups"],
        "item_rule_sets": ["id", "ruleKind", "groups"],
        "effects": ["id", "effectType", "params"],
    },
    "economy.xlsx": {
        "market_fee_tiers": ["id", "scope", "min_total", "max_total", "fee_bps", "priority", "active"],
        "market_npc_offers": ["id", "item_id", "npc_buy_price", "npc_sell_price", "active"],
        "drop_tables": ["id", "source", "entries"],
        "warehouse_rules": ["id", "warehouseType", "cols", "rows"],
    },
    "growth.xlsx": {
        "attributes": [
            "id",
            "iconId",
            "emojiIcon",
            "nameKey",
            "descKey",
            "baseDefault",
            "growthType",
            "expProfileId",
            "unit",
            "order",
            "category",
            "allowBattlePointLevelUp",
            "relatedActionMainType",
            "relatedActionEffect",
        ],
        "attribute_exp_profiles": ["id", "attr", "level", "expRequired"],
        "daily_tasks": ["id", "title", "description", "requirements", "rewards"],
        "daily_task_refresh_rules": ["id", "ruleType", "value"],
    },
    "equipment.xlsx": {
        "equipment_templates": [
            "id",
            "equipType",
            "slot",
            "occupySlots",
            "statusRequirement",
            "accessoryGroup",
            "affixSlotCount",
            "affixAllowedIds",
            "baseAttrs",
            "maxDurability",
            "decayRate",
            "enhanceAttrMilestoneOverrides",
            "enhanceAffixMilestonePools",
        ],
        "affix_definitions": ["id", "effectType", "effectParams", "valuePrecision", "allowedRarities", "active"],
        "affix_detach_rules": ["id", "toolItemId", "destroyChanceBase", "luckImpactFactor", "affixDropBeforeDestroyCheck", "active"],
        "affix_enchant_rules": ["id", "toolItemId", "consumeCount", "costs", "failChance", "rarityWeights", "allowedAffixIds", "allowedAffixScopes", "active"],
        "affix_carrier_rules": ["id", "carrierItemId", "allowedRarityIds", "active"],
    },
    "common.xlsx": {
        "rarity_definitions": ["id", "nameKey", "order", "color", "economyMultiplier"],
    },
    "world.xlsx": {
        "enemies": ["id", "name", "level", "attrs", "dropTableId"],
        "events_world": ["id", "eventType", "conditions", "effects"],
        "activity_mutex_rules": ["id", "activityA", "activityB", "mutex"],
    },
    "enemy.xlsx": {
        "world": [
            "id",
            "enemyIds",
            "specialEnemyIds",
            "specialWaves",
            "entryDurationSeconds",
            "entryCosts",
            "normalEnemyCount",
        ],
        "enemies": [
            "id",
            "name",
            "position",
            "exp",
            "drops",
            "hp",
            "hp_regen",
            "mp",
            "mp_regen",
            "strength",
            "dexterity",
            "intelligence",
            "stamina",
            "luck",
            "physics_damage",
            "physics_defence",
            "magical_damage",
            "magical_defence",
            "crit_chance",
            "crit_damage",
            "skillIds",
        ],
    },
}


def col_ref(index: int) -> str:
    index += 1
    chars = []
    while index > 0:
        index, rem = divmod(index - 1, 26)
        chars.append(chr(ord("A") + rem))
    return "".join(reversed(chars))


def worksheet_xml(headers):
    cells = []
    for idx, header in enumerate(headers):
        ref = f"{col_ref(idx)}1"
        text = escape(str(header))
        cells.append(f'<c r="{ref}" t="inlineStr"><is><t>{text}</t></is></c>')
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
        '<sheetData><row r="1">'
        + "".join(cells)
        + "</row></sheetData></worksheet>"
    )


def workbook_xml(sheet_names):
    sheets = []
    for i, name in enumerate(sheet_names, start=1):
        safe_name = escape(name)
        sheets.append(
            f'<sheet name="{safe_name}" sheetId="{i}" r:id="rId{i}"/>'
        )
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        "<sheets>"
        + "".join(sheets)
        + "</sheets></workbook>"
    )


def workbook_rels_xml(sheet_count: int):
    rels = []
    for i in range(1, sheet_count + 1):
        rels.append(
            f'<Relationship Id="rId{i}" '
            'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" '
            f'Target="worksheets/sheet{i}.xml"/>'
        )
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        + "".join(rels)
        + "</Relationships>"
    )


def root_rels_xml():
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" '
        'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" '
        'Target="xl/workbook.xml"/>'
        "</Relationships>"
    )


def content_types_xml(sheet_count: int):
    overrides = [
        '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
    ]
    for i in range(1, sheet_count + 1):
        overrides.append(
            f'<Override PartName="/xl/worksheets/sheet{i}.xml" '
            'ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
        )
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        + "".join(overrides)
        + "</Types>"
    )


def build_template(workbook_path: str, sheets: Dict[str, List[str]]):
    sheet_names = list(sheets.keys())
    with zipfile.ZipFile(workbook_path, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("[Content_Types].xml", content_types_xml(len(sheet_names)))
        zf.writestr("_rels/.rels", root_rels_xml())
        zf.writestr("xl/workbook.xml", workbook_xml(sheet_names))
        zf.writestr("xl/_rels/workbook.xml.rels", workbook_rels_xml(len(sheet_names)))
        for idx, (_, headers) in enumerate(sheets.items(), start=1):
            zf.writestr(f"xl/worksheets/sheet{idx}.xml", worksheet_xml(headers))


def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    data_root = os.path.dirname(script_dir)
    output_dir = os.path.join(data_root, "source", "workbooks")
    os.makedirs(output_dir, exist_ok=True)
    for filename, sheets in TEMPLATES.items():
        build_template(os.path.join(output_dir, filename), sheets)
    print(f"generated workbook templates at: {output_dir}")


if __name__ == "__main__":
    main()
