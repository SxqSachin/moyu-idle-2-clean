#!/usr/bin/env python3
import csv
import json
import os
import zipfile
from typing import Any, Dict, List, Optional
from xml.sax.saxutils import escape

from generate_workbook_templates import TEMPLATES


GENERATED_CATALOG_CACHE: Optional[Dict[str, Any]] = None
EQUIPMENT_SLOT_BY_TYPE: Dict[str, str] = {
    "weapon": "weapon",
    "armor": "armor",
    "helmet": "helmet",
    "cloak": "cloak",
    "pants": "pants",
    "boots": "boots",
    "accessory": "accessory",
}
ACCESSORY_SLOTS = {"accessory_1", "accessory_2", "accessory_3", "accessory_4"}


def col_ref(index: int) -> str:
    index += 1
    chars = []
    while index > 0:
        index, rem = divmod(index - 1, 26)
        chars.append(chr(ord("A") + rem))
    return "".join(reversed(chars))


def encode_cell(ref: str, value: Any) -> str:
    if value is None:
        value = ""
    if isinstance(value, bool):
        return f'<c r="{ref}" t="b"><v>{"1" if value else "0"}</v></c>'
    if isinstance(value, (int, float)):
        return f'<c r="{ref}"><v>{value}</v></c>'
    if isinstance(value, (dict, list)):
        value = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    text = escape(str(value))
    return f'<c r="{ref}" t="inlineStr"><is><t>{text}</t></is></c>'


def worksheet_xml(headers: List[str], rows: List[Dict[str, Any]]) -> str:
    all_rows = [headers]
    for source in rows:
        row = []
        for key in headers:
            row.append(source.get(key, ""))
        all_rows.append(row)

    xml_rows = []
    for r_index, row in enumerate(all_rows, start=1):
        cells = []
        for c_index, value in enumerate(row):
            cells.append(encode_cell(f"{col_ref(c_index)}{r_index}", value))
        xml_rows.append(f'<row r="{r_index}">{"".join(cells)}</row>')
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
        f'<sheetData>{"".join(xml_rows)}</sheetData>'
        "</worksheet>"
    )


def workbook_xml(sheet_names: List[str]) -> str:
    sheets = []
    for i, name in enumerate(sheet_names, start=1):
        safe_name = escape(name)
        sheets.append(f'<sheet name="{safe_name}" sheetId="{i}" r:id="rId{i}"/>')
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" '
        'xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        f"<sheets>{''.join(sheets)}</sheets>"
        "</workbook>"
    )


def workbook_rels_xml(sheet_count: int) -> str:
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
        f"{''.join(rels)}"
        "</Relationships>"
    )


def root_rels_xml() -> str:
    return (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" '
        'Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" '
        'Target="xl/workbook.xml"/>'
        "</Relationships>"
    )


def content_types_xml(sheet_count: int) -> str:
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
        f"{''.join(overrides)}"
        "</Types>"
    )


def parse_csv(path: str) -> List[Dict[str, Any]]:
    with open(path, "r", encoding="utf-8", newline="") as f:
        reader = csv.DictReader(f)
        return [dict(row) for row in reader]


def load_table_rows(source_dir: str, table_name: str) -> List[Dict[str, Any]]:
    if table_name == "actions":
        return load_table_rows_basic(source_dir, table_name)
    if table_name == "action_main_types":
        canonical_rows = load_table_rows_basic(source_dir, table_name)
        if canonical_rows:
            return canonical_rows
        actions = load_table_rows(source_dir, "actions")
        by_type: Dict[str, Dict[str, Any]] = {}
        for row in actions:
            main_type = str(row.get("mainType") or "").strip()
            action_id = str(row.get("id") or "").strip()
            if not main_type:
                continue
            if main_type not in by_type:
                by_type[main_type] = {
                    "id": main_type,
                    "nameKey": f"game.dynamicActions.{main_type}",
                    "icon": "",
                    "color": "#333",
                    "order": len(by_type),
                    "actions": [],
                }
            if action_id:
                by_type[main_type]["actions"].append(action_id)
        return list(by_type.values())
    if table_name == "action_sub_types":
        canonical_rows = load_table_rows_basic(source_dir, table_name)
        if canonical_rows:
            return canonical_rows
        actions = load_table_rows(source_dir, "actions")
        by_type: Dict[str, Dict[str, Any]] = {}
        for row in actions:
            sub_type = str(row.get("subType") or row.get("sub_type") or "").strip() or "common"
            action_id = str(row.get("id") or "").strip()
            if sub_type not in by_type:
                by_type[sub_type] = {
                    "id": sub_type,
                    "nameKey": f"action_sub_type.{sub_type}.name",
                    "descKey": f"action_sub_type.{sub_type}.desc",
                    "icon": "",
                    "color": "#333",
                    "order": len(by_type),
                    "actions": [],
                }
            if action_id:
                by_type[sub_type]["actions"].append(action_id)
        return list(by_type.values())
    return load_table_rows_basic(source_dir, table_name)


def load_table_rows_basic(source_dir: str, table_name: str) -> List[Dict[str, Any]]:
    json_path = os.path.join(source_dir, f"{table_name}.json")
    csv_path = os.path.join(source_dir, f"{table_name}.csv")
    if os.path.exists(json_path):
        with open(json_path, "r", encoding="utf-8") as f:
            data = json.load(f)
        if isinstance(data, list):
            return data
        return []
    if os.path.exists(csv_path):
        return parse_csv(csv_path)
    generated = load_generated_catalog(source_dir)
    if not generated:
        return []
    raw_table = generated.get(table_name)
    if isinstance(raw_table, list):
        return raw_table
    return []


def load_generated_catalog(source_dir: str) -> Dict[str, Any]:
    global GENERATED_CATALOG_CACHE
    if GENERATED_CATALOG_CACHE is not None:
        return GENERATED_CATALOG_CACHE
    generated_path = os.path.join(os.path.dirname(source_dir), "generated", "game-data.json")
    if not os.path.exists(generated_path):
        GENERATED_CATALOG_CACHE = {}
        return GENERATED_CATALOG_CACHE
    with open(generated_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    GENERATED_CATALOG_CACHE = data if isinstance(data, dict) else {}
    return GENERATED_CATALOG_CACHE


def normalize_string_array(raw: Any) -> List[str]:
    if isinstance(raw, list):
        return [str(v).strip() for v in raw if str(v).strip()]
    if isinstance(raw, str) and raw.strip():
        return [part.strip() for part in raw.split("|") if part.strip()]
    return []


def infer_slot(raw_slot: Any, item_type: str, equip_allowed_slots: List[str]) -> str:
    slot = str(raw_slot or "").strip()
    if slot:
        return slot
    allowed = sorted(set(equip_allowed_slots))
    if allowed == ["left_hand", "right_hand"]:
        return "weapon"
    if allowed and set(allowed).issubset(ACCESSORY_SLOTS):
        return "accessory"
    if len(allowed) == 1:
        return allowed[0]
    return EQUIPMENT_SLOT_BY_TYPE.get(item_type, "")


def merge_item_equipment_fields_into_templates(
    items: List[Dict[str, Any]], equipment_templates: List[Dict[str, Any]]
) -> List[Dict[str, Any]]:
    item_type_by_id: Dict[str, str] = {}
    for item in items:
        item_id = str(item.get("id") or "").strip()
        if not item_id:
            continue
        item_type_by_id[item_id] = str(item.get("type") or "").strip()

    by_id: Dict[str, Dict[str, Any]] = {}
    out: List[Dict[str, Any]] = []
    for row in equipment_templates:
        template_id = str(row.get("id") or "").strip()
        if not template_id:
            continue
        normalized = dict(row)
        by_id[template_id] = normalized
        out.append(normalized)

    for item in items:
        item_id = str(item.get("id") or "").strip()
        if not item_id:
            continue
        item_type = str(item.get("type") or "").strip()
        has_equip_type = item_type in EQUIPMENT_SLOT_BY_TYPE
        equip_allowed_slots = normalize_string_array(item.get("equipAllowedSlots"))
        equip_occupy_slots = normalize_string_array(item.get("equipOccupySlots"))
        equip_status_requirement = item.get("equipStatusRequirement")
        has_status_requirement = isinstance(equip_status_requirement, dict) and len(equip_status_requirement) > 0
        accessory_group = str(item.get("accessoryGroup") or "").strip()
        has_equip_fields = bool(equip_allowed_slots or equip_occupy_slots or accessory_group or has_status_requirement)
        if not has_equip_type and not has_equip_fields:
            continue
        target = by_id.get(item_id)
        if target is None:
            if not has_equip_type:
                continue
            target = {
                "id": item_id,
                "slot": "",
                "occupySlots": [],
                "statusRequirement": {},
                "accessoryGroup": "",
                "affixSlotCount": 0,
                "baseAttrs": {},
                "maxDurability": 100,
                "decayRate": 0,
            }
            by_id[item_id] = target
            out.append(target)
        target["slot"] = infer_slot(target.get("slot"), item_type, equip_allowed_slots)
        if not isinstance(target.get("occupySlots"), list):
            target["occupySlots"] = []
        if not isinstance(target.get("statusRequirement"), dict):
            target["statusRequirement"] = {}
        if not isinstance(target.get("baseAttrs"), dict):
            target["baseAttrs"] = {}
        if "maxDurability" not in target:
            target["maxDurability"] = 100
        if "decayRate" not in target:
            target["decayRate"] = 0
        raw_base = item.get("baseAttrs")
        if isinstance(raw_base, dict):
            target["baseAttrs"] = raw_base
        if equip_occupy_slots:
            target["occupySlots"] = equip_occupy_slots
        if has_status_requirement:
            target["statusRequirement"] = equip_status_requirement
        if accessory_group:
            target["accessoryGroup"] = accessory_group

    filtered: List[Dict[str, Any]] = []
    for row in out:
        template_id = str(row.get("id") or "").strip()
        if not template_id:
            continue
        slot = str(row.get("slot") or "").strip()
        item_type = item_type_by_id.get(template_id, "")
        if not slot and item_type not in EQUIPMENT_SLOT_BY_TYPE:
            continue
        filtered.append(row)
    return filtered


def strip_item_equipment_fields(items: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    dropped_keys = {"baseAttrs", "equipAllowedSlots", "equipOccupySlots", "equipStatusRequirement", "accessoryGroup"}
    out: List[Dict[str, Any]] = []
    for row in items:
        if not isinstance(row, dict):
            continue
        out.append({k: v for k, v in row.items() if k not in dropped_keys})
    return out


def normalize_rarity_definitions(items: List[Dict[str, Any]], rarity_rows: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    by_id: Dict[str, Dict[str, Any]] = {}
    order: List[str] = []
    for row in rarity_rows:
        rarity_id = str(row.get("id") or "").strip()
        if not rarity_id:
            continue
        if rarity_id not in by_id:
            order.append(rarity_id)
        by_id[rarity_id] = dict(row)

    for row in items:
        rarity_id = str(row.get("rarity") or "").strip()
        if not rarity_id:
            continue
        if rarity_id not in by_id:
            by_id[rarity_id] = {"id": rarity_id}
            order.append(rarity_id)

    out: List[Dict[str, Any]] = []
    for idx, rarity_id in enumerate(order, start=1):
        row = by_id[rarity_id]
        name_key = str(row.get("nameKey") or "").strip() or f"rarity.{rarity_id}"
        order_value = row.get("order", idx)
        color = str(row.get("color") or "").strip()
        out.append({
            "id": rarity_id,
            "nameKey": name_key,
            "order": order_value,
            "color": color,
        })
    return out


def write_workbook(workbook_path: str, sheets: Dict[str, List[str]], rows_by_sheet: Dict[str, List[Dict[str, Any]]]) -> None:
    sheet_names = list(sheets.keys())
    os.makedirs(os.path.dirname(workbook_path), exist_ok=True)
    with zipfile.ZipFile(workbook_path, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("[Content_Types].xml", content_types_xml(len(sheet_names)))
        zf.writestr("_rels/.rels", root_rels_xml())
        zf.writestr("xl/workbook.xml", workbook_xml(sheet_names))
        zf.writestr("xl/_rels/workbook.xml.rels", workbook_rels_xml(len(sheet_names)))
        for i, (sheet_name, headers) in enumerate(sheets.items(), start=1):
            rows = rows_by_sheet.get(sheet_name, [])
            zf.writestr(f"xl/worksheets/sheet{i}.xml", worksheet_xml(headers, rows))


def main() -> None:
    script_dir = os.path.dirname(os.path.abspath(__file__))
    data_root = os.path.dirname(script_dir)
    source_dir = os.path.join(data_root, "source")
    workbooks_dir = os.path.join(source_dir, "workbooks")

    rows_by_sheet: Dict[str, List[Dict[str, Any]]] = {}
    for workbook_sheets in TEMPLATES.values():
        for sheet_name in workbook_sheets.keys():
            rows_by_sheet[sheet_name] = load_table_rows(source_dir, sheet_name)

    rows_by_sheet["equipment_templates"] = merge_item_equipment_fields_into_templates(
        rows_by_sheet.get("items", []), rows_by_sheet.get("equipment_templates", [])
    )
    rows_by_sheet["items"] = strip_item_equipment_fields(rows_by_sheet.get("items", []))
    rows_by_sheet["rarity_definitions"] = normalize_rarity_definitions(
        rows_by_sheet.get("items", []), rows_by_sheet.get("rarity_definitions", [])
    )

    for workbook_name, workbook_sheets in TEMPLATES.items():
        workbook_path = os.path.join(workbooks_dir, workbook_name)
        write_workbook(workbook_path, workbook_sheets, rows_by_sheet)
    print(f"migrated JSON/CSV tables to workbooks: {workbooks_dir}")


if __name__ == "__main__":
    main()
