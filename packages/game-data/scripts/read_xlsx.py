#!/usr/bin/env python3
import json
import re
import sys
import zipfile
import xml.etree.ElementTree as ET

STRICT_JSON_COLUMNS = {
    "equipment_templates": {"occupySlots", "statusRequirement", "affixAllowedIds", "baseAttrs", "enhanceAttrMilestoneOverrides", "enhanceAffixMilestonePools"},
    "skills": {"statusRequirement", "weaponRequirement", "targetFormula", "effects", "targeting"},
    "affix_definitions": {"effectParams", "allowedRarities"},
    "affix_enchant_rules": {"rarityWeights", "allowedAffixScopes"},
    "affix_carrier_rules": {"allowedRarityIds"},
    "buildings": {"baseCosts", "stepCosts", "baseStatusRequirement", "stepStatusRequirement"},
    "action_main_types": {"actions"},
    "action_sub_types": {"actions"},
}

GLOBAL_STRICT_JSON_KEYS = {
    "baseAttrs",
    "costs",
    "rewards",
    "itemReward",
    "statusExpReward",
    "attrExp",
    "conditions",
    "effects",
    "payload",
    "extraAttrExp",
    "requirements",
    "modifiers",
    "drops",
    "actions",
    "rarityWeights",
    "allowedRarityIds",
}


def local_name(tag: str) -> str:
    if not isinstance(tag, str):
        return ""
    if "}" in tag:
        return tag.split("}", 1)[1]
    return tag


def col_to_index(cell_ref: str) -> int:
    m = re.match(r"([A-Z]+)", cell_ref or "")
    if not m:
        return 0
    letters = m.group(1)
    idx = 0
    for ch in letters:
        idx = idx * 26 + (ord(ch) - ord("A") + 1)
    return idx - 1


def index_to_col(idx: int) -> str:
    out = ""
    n = idx + 1
    while n > 0:
        n, rem = divmod(n - 1, 26)
        out = chr(ord("A") + rem) + out
    return out


def sanitize_json_text(text: str) -> str:
    if not isinstance(text, str):
        return text
    prev = None
    sanitized = text
    while prev != sanitized:
        prev = sanitized
        sanitized = re.sub(r",(\s*[}\]])", r"\1", sanitized)
    return sanitized


def parse_cell_value(cell, shared_strings):
    t = cell.attrib.get("t", "")
    if t == "inlineStr":
        text_nodes = []
        for node in cell.iter():
            if local_name(node.tag) == "t":
                text_nodes.append(node)
        if text_nodes:
            return "".join((node.text or "") for node in text_nodes)
        return ""
    v = None
    for child in list(cell):
        if local_name(child.tag) == "v":
            v = child
            break
    if v is None or v.text is None:
        return ""
    raw = v.text
    if t == "s":
        return shared_strings[int(raw)] if raw.isdigit() and int(raw) < len(shared_strings) else ""
    if t == "b":
        return raw == "1"
    if t == "str":
        return raw
    if raw.isdigit() or (raw.startswith("-") and raw[1:].isdigit()):
        try:
            return int(raw)
        except ValueError:
            return raw
    try:
        return float(raw)
    except ValueError:
        return raw


def load_shared_strings(zf):
    try:
        data = zf.read("xl/sharedStrings.xml")
    except KeyError:
        return []
    root = ET.fromstring(data)
    out = []
    for si in root.iter():
        if local_name(si.tag) != "si":
            continue
        texts = [t.text or "" for t in si.iter() if local_name(t.tag) == "t"]
        out.append("".join(texts))
    return out


def load_sheet_targets(zf):
    wb = ET.fromstring(zf.read("xl/workbook.xml"))
    rels = ET.fromstring(zf.read("xl/_rels/workbook.xml.rels"))
    rid_to_target = {}
    for rel in rels.iter():
        if local_name(rel.tag) != "Relationship":
            continue
        rid_to_target[rel.attrib.get("Id")] = rel.attrib.get("Target")
    out = []
    for sheet in wb.iter():
        if local_name(sheet.tag) != "sheet":
            continue
        name = sheet.attrib.get("name", "")
        rid = (
            sheet.attrib.get("{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id")
            or sheet.attrib.get("{http://purl.oclc.org/ooxml/officeDocument/relationships}id")
            or sheet.attrib.get("r:id")
        )
        target = rid_to_target.get(rid, "")
        if target.startswith("/"):
            target = target.lstrip("/")
        if target and not target.startswith("xl/"):
            target = f"xl/{target}"
        out.append((name, target))
    return out


def should_parse_strict_json(sheet_name: str, key: str) -> bool:
    sheet_key = sheet_name
    if sheet_name.startswith("equipment_templates"):
        sheet_key = "equipment_templates"
    cols = STRICT_JSON_COLUMNS.get(sheet_key, set())
    if sheet_name == "actions" or sheet_name.startswith("actions_"):
        if key in {"costs", "itemReward", "statusExpReward", "buildingRequirement"}:
            return True
    return key in cols or key in GLOBAL_STRICT_JSON_KEYS


def to_json_value(value, *, sheet_name: str, key: str, row_number: int, col_index: int):
    if should_parse_strict_json(sheet_name, key):
        if isinstance(value, (dict, list)):
            return value
        if not isinstance(value, str):
            cell = f"{index_to_col(col_index)}{row_number}"
            raise ValueError(f"{sheet_name}.{cell} ({key}) 必须是 JSON 字符串")
        text = value.strip()
        if text == "":
            return {} if key in {
                "baseAttrs",
                "costs",
                "itemReward",
                "statusExpReward",
                "statusRequirement",
                "buildingRequirement",
                "effectParams",
                "baseCosts",
                "stepCosts",
                "baseStatusRequirement",
                "stepStatusRequirement",
            } else []
        if not (text.startswith("{") or text.startswith("[")):
            cell = f"{index_to_col(col_index)}{row_number}"
            raise ValueError(f"{sheet_name}.{cell} ({key}) 必须以 '{{' 或 '[' 开头")
        try:
            return json.loads(sanitize_json_text(text))
        except json.JSONDecodeError as err:
            cell = f"{index_to_col(col_index)}{row_number}"
            raise ValueError(f"{sheet_name}.{cell} ({key}) JSON 解析失败: {err.msg}") from err

    if isinstance(value, str):
        text = value.strip()
        if text == "":
            return ""
        if text.startswith("{") or text.startswith("["):
            try:
                return json.loads(sanitize_json_text(text))
            except json.JSONDecodeError:
                return value
    return value


def parse_sheet(zf, target, shared_strings, sheet_name: str):
    root = ET.fromstring(zf.read(target))
    rows = [node for node in root.iter() if local_name(node.tag) == "row"]
    parsed = []
    for row in rows:
        raw_row_num = row.attrib.get("r", "")
        row_num = int(raw_row_num) if raw_row_num.isdigit() else len(parsed) + 1
        cols = {}
        for cell in row:
            if local_name(cell.tag) != "c":
                continue
            idx = col_to_index(cell.attrib.get("r", ""))
            cols[idx] = parse_cell_value(cell, shared_strings)
        parsed.append({"row_num": row_num, "cols": cols})
    if not parsed:
        return []
    header_cols = parsed[0]["cols"]
    max_idx = max(header_cols.keys()) if header_cols else -1
    headers = []
    for i in range(max_idx + 1):
        header = header_cols.get(i, "")
        headers.append(str(header).strip() if header is not None else "")
    out = []
    for row_data in parsed[1:]:
        row = row_data["cols"]
        row_num = row_data["row_num"]
        item = {}
        for i, key in enumerate(headers):
            if not key or key.startswith("_"):
                continue
            value = row.get(i, "")
            item[key] = to_json_value(value, sheet_name=sheet_name, key=key, row_number=row_num, col_index=i)
        if item:
            item["__rowNum"] = row_num
            out.append(item)
    return out


def main():
    if len(sys.argv) < 2:
        print("{}")
        return
    workbook = sys.argv[1]
    with zipfile.ZipFile(workbook, "r") as zf:
        shared_strings = load_shared_strings(zf)
        sheets = load_sheet_targets(zf)
        out = {}
        for name, target in sheets:
            if not name or not target:
                continue
            try:
                out[name] = parse_sheet(zf, target, shared_strings, name)
            except KeyError:
                continue
    print(json.dumps(out, ensure_ascii=False))


if __name__ == "__main__":
    main()
