# -*- coding: utf-8 -*-

import os
from PIL import Image

folder = os.path.join(os.path.dirname(__file__), '..', '..', 'web', 'public', 'assets', 'icons', 'item')

BACKGROUND_REMOVED_PREFIXES = (
    '已移除背景的 ',
    '已移除背景的\u00a0',
    '已移除背景的',
)


def strip_background_removed_prefix(file_name):
    for prefix in BACKGROUND_REMOVED_PREFIXES:
        if file_name.startswith(prefix):
            stripped = file_name[len(prefix):].lstrip()
            return stripped or None
    return None


def normalize_background_removed_names(folder):
    for root, _, files in os.walk(folder):
        for file in files:
            if not file.lower().endswith('.png'):
                continue

            normalized_file = strip_background_removed_prefix(file)
            if not normalized_file:
                continue

            source_path = os.path.join(root, file)
            target_path = os.path.join(root, normalized_file)
            try:
                existed = os.path.exists(target_path)
                os.replace(source_path, target_path)
                action = 'Overwritten' if existed else 'Renamed'
                print(f'{action}: {source_path} -> {target_path}')
            except Exception as e:
                print(f'Error renaming {source_path}: {e}')


def compress_pngs(folder, max_size=128):
    for root, _, files in os.walk(folder):
        for file in files:
            if file.lower().endswith('.png'):
                file_path = os.path.join(root, file)
                try:
                    with Image.open(file_path) as img:
                        # 计算缩放尺寸，最大边为max_size；小图不放大
                        w, h = img.size
                        scale = min(max_size / max(w, h), 1)
                        new_w = max(1, int(w * scale))
                        new_h = max(1, int(h * scale))
                        if (new_w, new_h) != (w, h):
                            img = img.resize((new_w, new_h), Image.Resampling.LANCZOS)
                        img.save(file_path, optimize=True)
                    print(f'Compressed: {file_path} ({w}x{h} -> {new_w}x{new_h})')
                except Exception as e:
                    print(f'Error processing {file_path}: {e}')

if __name__ == '__main__':
    normalize_background_removed_names(folder)
    compress_pngs(folder, max_size=128)
