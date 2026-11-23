# game-data-tool（首期）

## 1. 启动

```bash
cd game-data-tool
npm install
npm run dev
```

打开浏览器后，点击“授权并加载 packages/game-data”，选择仓库中的 `packages/game-data` 目录。

## 2. 数据来源与保存

- 初始化优先读取：`packages/game-data/generated/chunks/*.json`。
- 保存目标：`packages/game-data/source/json/*.json`。
- 写入格式：根对象 `{ version, <table> }`，2 空格缩进，稳定 key 排序。

## 3. 能力范围

- 编辑表：`items`、`actions`、`equipment_templates`、`enemies`。
- 校验：`zod` 基础校验 + 跨表聚合错误输出。
- 公式：`quickjs-emscripten + Worker` 执行，保存前必须通过公式校验。
- 安全：编辑态与落盘态分离，支持撤销/重做。

## 4. 构建冲突规则

运行 `node packages/game-data/scripts/build.mjs` 时：
- 若 Excel 与 `source/json` 同表同 `id`，构建报错并退出；
- 错误包含表名、id 与来源信息，便于定位处理。
