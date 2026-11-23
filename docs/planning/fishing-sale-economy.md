# 钓鱼批量出售经济模型

## 1. 目标与结论

- 时间价值锚点：`1 秒 = 1～1.5 金币`。
- 以当前实际钓行的等待、咬钩、搏鱼、脱钩、断线和基础耗材成本进行 1 小时模拟，三个水域的中性行情净产值约为：
  - 晴湾浅滩：`4116 金币/小时`，即 `1.14 金币/秒`。
  - 柳溪河谷：`4257 金币/小时`，即 `1.18 金币/秒`。
  - 霜镜深湖：`4789 金币/小时`，即 `1.33 金币/秒`。
- 今日行情限制在 `0.90～1.10`；对应三个水域的完整模拟区间分别为 `1.02～1.26`、`1.06～1.31`、`1.19～1.47 金币/秒`，均落在基础经济锚点内。

## 2. 售价公式

```text
最终金币 = round(鱼种基价 × 个体总系数 × 行情总系数 × 养殖折算)
```

每尾鱼至少售出 `1` 金币。服务端按实际鱼实例重新报价；玩家确认时必须提交所见总价。如果供需在报价与确认之间变化，服务端拒绝旧报价并要求前端刷新，不会静默按另一价格成交。

### 2.1 标准个体

- 标准体长：`最小体长 + (最大体长 - 最小体长) × 0.30`。
- 标准体重：`体型系数 × 标准体长³ / 100000`。
- 标准年龄：`930 天`。
- 标准体况与健康：均为 `1.00`。
- 标准个体的年龄、体长、体重和品质系数均为 `1.00`。

### 2.2 个体系数

| 项目 | 公式 | 下限 | 上限 | 设计意图 |
| --- | --- | ---: | ---: | --- |
| 年龄 | `(max(年龄天数, 1) / 930)^0.06` | 0.90 | 1.10 | 年龄是温和修正，不压过真实尺寸 |
| 体长 | `(体长 / 标准体长)^0.45` | 0.75 | 1.30 | 奖励明显更长的个体 |
| 体重 | `(体重 / 标准体重)^0.15` | 0.80 | 1.25 | 奖励同种更重的个体，但避免立方增长失控 |
| 品质 | `0.78 + 体况 × 0.15 + 健康 × 0.07` | 0.85 | 1.03 | 体况权重大于健康，二者都参与售价 |
| 个体总系数 | `年龄 × 体长 × 体重 × 品质` | 0.55 | 1.75 | 对异常数据和极端大鱼设置总保护栏 |

### 2.3 行情与养殖折算

| 项目 | 公式 | 范围 |
| --- | --- | --- |
| 每日需求 | 按 `鱼种 + UTC 日期` 稳定散列 | 0.96～1.04 |
| 当前供给 | `1.06 - 当日该鱼种累计售出重量 / 1500` | 0.93～1.06 |
| 行情总系数 | `每日需求 × 当前供给` | 0.90～1.10 |
| 野生鱼折算 | 固定 | 1.00 |
| 养殖鱼折算 | `min(0.50, 300 × 体重 / 鱼种基价)` | 0～0.50 |

养殖折算同时限制为每公斤 `300` 基础金币等价，避免高稀有、小体型鱼通过繁殖绕过垂钓时间成本。按个体与行情上限计算，养殖出售每点生长营养最多回收约 `3.21` 金币，低于当前最优饲料的约 `3.98` 金币机会成本，且尚未计入维持消耗。

## 3. 鱼种基价表

下表的基价是每尾鱼的固定鱼种基价，不再是每公斤单价。标准体长和标准体重仅用于计算尺寸系数。

| 鱼种 | ID | 水域 | 稀有度 | 基价 | 标准体长 | 标准体重 |
| --- | --- | --- | --- | ---: | ---: | ---: |
| 鲫鱼 | `crucian_carp` | sunmere_shallows | common | 400 | 21.4 cm | 0.104 kg |
| 鲤鱼 | `common_carp` | sunmere_shallows | uncommon | 570 | 46.9 cm | 1.238 kg |
| 蓝鳃太阳鱼 | `bluegill` | sunmere_shallows | common | 370 | 17.0 cm | 0.071 kg |
| 草鱼 | `grass_carp` | sunmere_shallows | uncommon | 620 | 58.5 cm | 2.102 kg |
| 泥鳅 | `mud_loach` | sunmere_shallows | common | 360 | 12.6 cm | 0.014 kg |
| 乌鳢 | `snakehead` | sunmere_shallows | rare | 960 | 49.0 cm | 1.271 kg |
| 莲纹锦鲤 | `lotus_koi` | sunmere_shallows | epic | 3160 | 42.6 cm | 0.959 kg |
| 烬鳍鱼 | `emberfin` | sunmere_shallows | epic | 3570 | 31.8 cm | 0.360 kg |
| 褐鳟 | `brown_trout` | willowrun_valley | uncommon | 480 | 40.2 cm | 0.663 kg |
| 欧鲢 | `chub` | willowrun_valley | common | 355 | 27.9 cm | 0.235 kg |
| 雅罗鱼 | `dace` | willowrun_valley | common | 320 | 17.0 cm | 0.045 kg |
| 欧洲鲃 | `barbel` | willowrun_valley | uncommon | 520 | 42.5 cm | 0.737 kg |
| 河鲈 | `freshwater_perch` | willowrun_valley | uncommon | 440 | 25.8 cm | 0.227 kg |
| 欧洲鳗鲡 | `river_eel` | willowrun_valley | rare | 860 | 57.1 cm | 1.154 kg |
| 璃背箭鱼 | `glassback_darter` | willowrun_valley | epic | 2900 | 16.5 cm | 0.040 kg |
| 雷须鲶 | `thunder_whisker` | willowrun_valley | epic | 3400 | 68.0 cm | 3.710 kg |
| 湖鳟 | `lake_trout` | frostglass_lake | rare | 780 | 61.5 cm | 2.605 kg |
| 北极红点鲑 | `arctic_char` | frostglass_lake | uncommon | 560 | 40.4 cm | 0.692 kg |
| 江鳕 | `burbot` | frostglass_lake | uncommon | 590 | 49.0 cm | 1.153 kg |
| 湖白鲑 | `whitefish` | frostglass_lake | common | 475 | 34.2 cm | 0.376 kg |
| 白斑狗鱼 | `northern_pike` | frostglass_lake | rare | 890 | 64.5 cm | 2.415 kg |
| 茴鱼 | `grayling` | frostglass_lake | uncommon | 540 | 28.4 cm | 0.234 kg |
| 月镜鲤 | `moonmirror_carp` | frostglass_lake | legendary | 4350 | 62.0 cm | 2.908 kg |
| 霜幕鲟 | `frostveil_sturgeon` | frostglass_lake | legendary | 5400 | 113.0 cm | 19.912 kg |
