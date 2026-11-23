#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import hashlib
import json
import math
import os
from datetime import datetime, timezone
from pathlib import Path

from PIL import Image


PROJECT_ROOT = Path(__file__).resolve().parents[3]
SOURCE_DIR = PROJECT_ROOT / "web/public/assets/icons/item"
ATLAS_DIR = PROJECT_ROOT / "web/public/assets/icon-atlas"
MANIFEST_DIR = PROJECT_ROOT / "web/public/game-data/icon-atlas"
MANIFEST_PATH = MANIFEST_DIR / "item-atlas.json"

ATLAS_PREFIX = "item."
ATLAS_EXT = ".webp"
CELL_SIZE = 96
MAX_COLUMNS = 16
MAX_ICONS_PER_ATLAS = 64
WEBP_QUALITY = 82


def clean_previous_outputs():
    ATLAS_DIR.mkdir(parents=True, exist_ok=True)
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)
    for file_path in ATLAS_DIR.glob(f"{ATLAS_PREFIX}*{ATLAS_EXT}"):
        file_path.unlink()


def make_entry(index, atlas_id):
    col = index % MAX_COLUMNS
    row = index // MAX_COLUMNS
    return {
        "atlas": atlas_id,
        "x": col * CELL_SIZE,
        "y": row * CELL_SIZE,
        "w": CELL_SIZE,
        "h": CELL_SIZE,
    }


def paste_icon(atlas, source_path, index):
    with Image.open(source_path) as image:
        image = image.convert("RGBA")
        image.thumbnail((CELL_SIZE, CELL_SIZE), Image.Resampling.LANCZOS)
        tile = Image.new("RGBA", (CELL_SIZE, CELL_SIZE), (0, 0, 0, 0))
        offset = ((CELL_SIZE - image.width) // 2, (CELL_SIZE - image.height) // 2)
        tile.alpha_composite(image, offset)
        atlas.alpha_composite(tile, ((index % MAX_COLUMNS) * CELL_SIZE, (index // MAX_COLUMNS) * CELL_SIZE))


def build_manifest_entry_map(files, atlas_id):
    entries = {}
    for index, source_path in enumerate(files):
        entry = make_entry(index, atlas_id)
        icon_id = source_path.stem
        file_name = source_path.name
        keys = (
            f"item.{icon_id}",
            f"/assets/icons/item/{file_name}",
            f"/assets/icons/items/{file_name}",
        )
        for key in keys:
            entries[key] = entry
    return entries


def write_atlas(files, atlas_index):
    columns = min(MAX_COLUMNS, max(1, len(files)))
    rows = max(1, math.ceil(len(files) / MAX_COLUMNS))
    width = columns * CELL_SIZE
    height = rows * CELL_SIZE
    atlas = Image.new("RGBA", (width, height), (0, 0, 0, 0))

    for index, source_path in enumerate(files):
        paste_icon(atlas, source_path, index)

    temp_path = ATLAS_DIR / f"item.{atlas_index}.tmp.webp"
    atlas.save(temp_path, "WEBP", quality=WEBP_QUALITY, method=6)
    digest = hashlib.sha256(temp_path.read_bytes()).hexdigest()[:12]
    atlas_id = f"item-{atlas_index}"
    atlas_name = f"{ATLAS_PREFIX}{atlas_index}.{digest}{ATLAS_EXT}"
    atlas_path = ATLAS_DIR / atlas_name
    os.replace(temp_path, atlas_path)
    return {
        "id": atlas_id,
        "image": f"/assets/icon-atlas/{atlas_name}",
        "width": width,
        "height": height,
        "imageBytes": atlas_path.stat().st_size,
    }


def main():
    if not SOURCE_DIR.exists():
        raise SystemExit(f"icon source directory is missing: {SOURCE_DIR}")

    files = sorted(path for path in SOURCE_DIR.glob("*.png") if path.is_file())
    clean_previous_outputs()

    if not files:
        MANIFEST_PATH.write_text(json.dumps({
            "schemaVersion": 1,
            "type": "item",
            "generatedAt": datetime.now(timezone.utc).isoformat(),
            "image": "",
            "format": "webp",
            "cellSize": CELL_SIZE,
            "width": 0,
            "height": 0,
            "imageBytes": 0,
            "atlases": [],
            "entries": {},
        }, ensure_ascii=False, separators=(",", ":")), encoding="utf-8")
        print("[icon-atlas] no item icons found")
        return

    atlases = []
    entries = {}
    for atlas_index, start in enumerate(range(0, len(files), MAX_ICONS_PER_ATLAS), start=1):
        chunk = files[start:start + MAX_ICONS_PER_ATLAS]
        atlas = write_atlas(chunk, atlas_index)
        atlases.append(atlas)
        entries.update(build_manifest_entry_map(chunk, atlas["id"]))

    first_atlas = atlases[0]
    manifest = {
        "schemaVersion": 1,
        "type": "item",
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "image": first_atlas["image"],
        "format": "webp",
        "cellSize": CELL_SIZE,
        "width": first_atlas["width"],
        "height": first_atlas["height"],
        "imageBytes": sum(atlas["imageBytes"] for atlas in atlases),
        "atlases": atlases,
        "entries": entries,
    }
    MANIFEST_PATH.write_text(json.dumps(manifest, ensure_ascii=False, separators=(",", ":")), encoding="utf-8")
    print(
        f"[icon-atlas] generated {len(files)} item icons -> "
        f"{len(atlases)} atlases ({manifest['imageBytes']} bytes total)"
    )


if __name__ == "__main__":
    main()
