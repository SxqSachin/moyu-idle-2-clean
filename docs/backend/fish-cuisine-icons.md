# 鱼料理独立图标

2026-09-05：使用内置 imagegen 为每个物品分别生成透明 PNG，按原 alpha 使用 sips 缩放为 128×128；没有通过调色复用同一张图。物品、制作动作、当前效果定义的 iconId 均为 `item.<id>`。

风格：卡通幻想 RPG 食物、干净轮廓、柔和明暗、适中细节、三分之二俯视、透明背景，无文字、无边框。七组分别使用苔绿陶器、星蓝器皿、红陶、竹器、铜锅、芦编/青瓷、霜月白瓷形成区分。

## 新增文件

- [starlight_silverfish_grill.png](../../web/public/assets/icons/item/starlight_silverfish_grill.png)
- [mossfire_dual_fish_soup.png](../../web/public/assets/icons/item/mossfire_dual_fish_soup.png)
- [steadyfin_grill.png](../../web/public/assets/icons/item/steadyfin_grill.png)
- [steadyfin_soup.png](../../web/public/assets/icons/item/steadyfin_soup.png)
- [frugal_river_grill.png](../../web/public/assets/icons/item/frugal_river_grill.png)
- [frugal_river_soup.png](../../web/public/assets/icons/item/frugal_river_soup.png)
- [harvest_river_grill.png](../../web/public/assets/icons/item/harvest_river_grill.png)
- [harvest_river_soup.png](../../web/public/assets/icons/item/harvest_river_soup.png)
- [reedbank_grill.png](../../web/public/assets/icons/item/reedbank_grill.png)
- [reedbank_soup.png](../../web/public/assets/icons/item/reedbank_soup.png)
- [frostmoon_dual_fish_grill.png](../../web/public/assets/icons/item/frostmoon_dual_fish_grill.png)
- [frostmoon_dual_fish_soup.png](../../web/public/assets/icons/item/frostmoon_dual_fish_soup.png)

## 生成原图

内置生成原图仅为追溯记录；游戏加载上面的项目内资源，不依赖 Codex 目录。

- `starlight_silverfish_grill`: `（生成工作文件）`
- `mossfire_dual_fish_soup`: `（生成工作文件）`
- `steadyfin_grill`: `（生成工作文件）`
- `steadyfin_soup`: `（生成工作文件）`
- `frugal_river_grill`: `（生成工作文件）`
- `frugal_river_soup`: `（生成工作文件）`
- `harvest_river_grill`: `（生成工作文件）`
- `harvest_river_soup`: `（生成工作文件）`
- `reedbank_grill`: `（生成工作文件）`
- `reedbank_soup`: `（生成工作文件）`
- `frostmoon_dual_fish_grill`: `（生成工作文件）`
- `frostmoon_dual_fish_soup`: `（生成工作文件）`

## 保留文件

- [mossfire_dual_fish_grill.png](../../web/public/assets/icons/item/mossfire_dual_fish_grill.png)
- [starlight_silverfish_soup.png](../../web/public/assets/icons/item/starlight_silverfish_soup.png)

## 验证

`fish-food.test.mjs` 校验 14 组物品/动作/效果 iconId 一致且唯一，文件为 128×128 RGBA；game-data 构建重新生成图集。桌面与移动端浏览器组件验证以实际 ItemIcon 和 ActionPanel 为准，不能用生成大图代替小尺寸检查。

