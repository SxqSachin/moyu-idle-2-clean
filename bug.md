1. 仓库中的物品每拖动一次，就会出现两个额外的仓库，而且是持久化存在的
2. 游戏内所有的属性、物品、可视的内容，都需要做本地化显示
3. 仓库变动时，并没有及时通知客户端更新数据，比如我商店买个东西，金币扣了，但是页面上并没有更新
4. 添加一个首页，然后执行队列显示在首页上，执行队列按照先进队列的显示在更上面的原则排序。然后每个队列都需要显示此动作剩余的时间（总的，而不是单次的）、已完成/总次数、预计完成时间
5. 添加一个探索菜单，总的也是一样的，探索內都是一个个动作，可以先设计一个探索森林、探索洞穴这些。
6. 添加一个制作菜单，总的也是一样的，探索內都是一个个动作，可以先设计一个制作铁剑、制作布甲这些。
6. 背包需要作为玩家菜单中一个特别的tab存在，背包是一个独立的网格系统，玩家可以把仓库内的东西放到背包中，也可以把背包中的东西放到仓库中。背包的面积根据玩家的背包装备决定
7. 探索、战斗这两个比较特殊，这两类动作获取到的物品，都是需要放到背包中的，然后背包中的物品需要玩家手动落地到仓库才行。
8. 因为是个放置类游戏，所以玩家很可能安排在探索后去战斗，因此很可能出现背包满了+仓库满了的情况，但是因为是放置，所以玩家很可能都不知情导致无法继续获取物品，这个你有什么点子吗

a. 前端玩法的各种布局、交互，完全按照《摸鱼 Idle 1》中的方式来设计吧，当然我们需要做改进，这一点做到UI开发规范文档中，然后直接大刀阔斧的改我们这个项目吧，你可以先阅读那个项目的前端各个玩法的UI设计，然后形成一个详尽的文档，便于后续ai遵循
b. 玩家在线、离线状态如何判断，如何识别玩家在多个地方登陆时离线后的状态，是否需要做一个仅允许单端登录，其他地方登陆后原位置自动踢出登陆的逻辑
c. 强化、附魔、异变这些东西所需要的item信息，完全看不到
d. 现在获取到的物品，在仓库中只能看到格子，不知道这个格子对应的是什么物品，什么数量

**前端**玩法的各种布局、交互，完全按照《摸鱼 Idle 1》中的界面交互方式迁移，当然我们需要做改进，你先阅读那个项目的前端各个菜单的内容以及其中的UI设计，然后形成一个详尽的文档，便于后续ai遵循。

1. 报错：WS_ACTION_QUEUE_FAILED: load action queue failed
2. 点击仓库报错：{"ok":false,"error":{"code":"WAREHOUSE_LAYOUT_FAILED","message":"load warehouse layout failed","debug":"ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)"},"ts":1777864324180,"code":"WAREHOUSE_LAYOUT_FAILED","message":"load warehouse layout failed","debug":"ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)"}
3. 点击背包报错：{"ok":false,"error":{"code":"WAREHOUSE_LAYOUT_FAILED","message":"load warehouse layout failed","debug":"ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)"},"ts":1777864340064,"code":"WAREHOUSE_LAYOUT_FAILED","message":"load warehouse layout failed","debug":"ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification (SQLSTATE 42P10)"}
4. 除了首页之外，其他地方不用展示执行队列
5. 所有的UI设计，必须遵循移动端优先设计

在游戏中添加“属性需求”系统：
1. 在action中，添加statusRequire参数，格式为{"str": 10, "mining": 10}，意为要求力量最低10，挖矿最低10，才可以执行这个动作
2. 在item->equipment中，添加statusRequire参数，格式为{"dex": 25}，意为要求敏捷最低25，才能装备此物品
3. 实现一个类似item/QuickRef.vue的组件，传入角色属性key以及数值，显示为一个el-tag，包含属性图标、属性i18n名称、如果传入了数值，那么还需要显示数值
4. item、action的详细信息中，添加对应的statusRequire要求显示
5. 现在游戏有点乱得很啊，关于角色属性attr这一块，现在属性种类是来源于excel的，前后端除了具体业务（比如战斗时需要获取伤害、暴击率这些东西）之外，其他的比如增加属性经验、等级、计算属性值的逻辑都不可以写死了。然后我发现前端(角色->基础属性)这部分需要依赖game-data来显示各项属性，现在前端显示的是固定的，看看什么问题
6. web/src/views/game/attributes/index.vue做好拆分，现在里面内容太多太多了，可以提取的内容可以单独作为组件、相关的业务可以提取为composable

相关前后端逻辑都需要兼容，excel转json时也需要兼容


panic: concurrent write to websocket connection

goroutine 1927 [running]:
github.com/fasthttp/websocket.(*messageWriter).flushFrame(0x7b8f47ddbbc0, 0x1, {0x0?, 0x7b8f48289e30?, 0x7b8f47e63778?})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:656 +0x428
github.com/fasthttp/websocket.(*messageWriter).Close(0x7b8f47d74600?)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:770 +0x44
github.com/fasthttp/websocket.(*Conn).beginMessage(0x7b8f47f1a6e0, 0x7b8f48289e30, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:513 +0x3c
github.com/fasthttp/websocket.(*Conn).NextWriter(0x7b8f47f1a6e0, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:556 +0x44
github.com/fasthttp/websocket.(*Conn).WriteJSON(0x105883120?, {0x105883120, 0x7b8f47ca85c0})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/json.go:24 +0x2c
mysecondwebgame/server/internal/httpapi.(*Server).writeWSEvent(0x7b8f47d678c8, 0x7b8f48158600, {0x1052df509, 0x10}, {0x10583c080, 0x7b8f48010320})
        ./server/internal/httpapi/server.go:2982 +0x278
mysecondwebgame/server/internal/httpapi.(*Server).handleWS(0x7b8f47d678c8, 0x7b8f48158600)
        ./server/internal/httpapi/server.go:3114 +0x121c
github.com/gofiber/websocket/v2.New.func2.5(0x1058f2c38?)
        $GOMODCACHE/github.com/gofiber/websocket/v2@v2.2.1/websocket.go:130 +0x6c
github.com/fasthttp/websocket.(*FastHTTPUpgrader).Upgrade.func1({0x1058f2c38, 0x7b8f48158840})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/server_fasthttp.go:201 +0x140
github.com/valyala/fasthttp.hijackConnHandler(0x7b8f47d6d208, {0x1058e8018, 0x7b8f47ebc1e0}, {0x1058f27d0, 0x7b8f47fc8088}, 0x7b8f47db4200, 0x7b8f47d0b1c0)
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2499 +0x58
created by github.com/valyala/fasthttp.(*Server).serveConn in goroutine 1926
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2454 +0x12fc
exit status 2
➜  server git:(main) ✗ go run ./cmd/server

 ┌───────────────────────────────────────────────────┐ 
 │                   Fiber v2.52.5                   │ 
 │               http://127.0.0.1:8080               │ 
 │       (bound on host 0.0.0.0 and port 8080)       │ 
 │                                                   │ 
 │ Handlers ............ 88  Processes ........... 1 │ 
 │ Prefork ....... Disabled  PID .............. 7355 │ 
 └───────────────────────────────────────────────────┘ 

^Csignal: interrupt
➜  server git:(main) ✗ 
➜  server git:(main) ✗ 
➜  server git:(main) ✗ go run ./cmd/server

 ┌───────────────────────────────────────────────────┐ 
 │                   Fiber v2.52.5                   │ 
 │               http://127.0.0.1:8080               │ 
 │       (bound on host 0.0.0.0 and port 8080)       │ 
 │                                                   │ 
 │ Handlers ............ 88  Processes ........... 1 │ 
 │ Prefork ....... Disabled  PID ............. 11822 │ 
 └───────────────────────────────────────────────────┘ 

^Csignal: interrupt
➜  server git:(main) ✗ 
➜  server git:(main) ✗ 
➜  server git:(main) ✗ node packages/game-data/scripts/build.mjs
node:internal/modules/cjs/loader:1228
  throw err;
  ^

Error: Cannot find module './server/packages/game-data/scripts/build.mjs'
    at Function._resolveFilename (node:internal/modules/cjs/loader:1225:15)
    at Function._load (node:internal/modules/cjs/loader:1055:27)
    at TracingChannel.traceSync (node:diagnostics_channel:322:14)
    at wrapModuleLoad (node:internal/modules/cjs/loader:220:24)
    at Function.executeUserEntryPoint [as runMain] (node:internal/modules/run_main:170:5)
    at node:internal/main/run_main_module:36:49 {
  code: 'MODULE_NOT_FOUND',
  requireStack: []
}

Node.js v22.14.0
➜  server git:(main) ✗ bash init-db-local.sh
➜  server git:(main) ✗ go run ./cmd/server

 ┌───────────────────────────────────────────────────┐ 
 │                   Fiber v2.52.5                   │ 
 │               http://127.0.0.1:8080               │ 
 │       (bound on host 0.0.0.0 and port 8080)       │ 
 │                                                   │ 
 │ Handlers ............ 88  Processes ........... 1 │ 
 │ Prefork ....... Disabled  PID ............. 16828 │ 
 └───────────────────────────────────────────────────┘ 

panic: concurrent write to websocket connection

goroutine 923 [running]:
github.com/fasthttp/websocket.(*messageWriter).flushFrame(0x817a8558390, 0x1, {0x0?, 0x104d10f20?, 0x817a82ca000?})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:656 +0x428
github.com/fasthttp/websocket.(*messageWriter).Close(0x817a835f828?)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:770 +0x44
github.com/fasthttp/websocket.(*Conn).WriteJSON(0x104cd71e0?, {0x104cd71e0, 0x817a86326c0})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/json.go:29 +0xbc
mysecondwebgame/server/internal/httpapi.(*Server).writeWSEvent(0x817a83f18c8, 0x817a8789bc0, {0x104735a25, 0x10}, {0x104c8ffe0, 0x817a82d4520})
        ./server/internal/httpapi/server.go:2978 +0x278
mysecondwebgame/server/internal/httpapi.(*Server).handleWS(0x817a83f18c8, 0x817a8789bc0)
        ./server/internal/httpapi/server.go:3110 +0x121c
github.com/gofiber/websocket/v2.New.func2.5(0x104d46ad8?)
        $GOMODCACHE/github.com/gofiber/websocket/v2@v2.2.1/websocket.go:130 +0x6c
github.com/fasthttp/websocket.(*FastHTTPUpgrader).Upgrade.func1({0x104d46ad8, 0x817a8789e30})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/server_fasthttp.go:201 +0x140
github.com/valyala/fasthttp.hijackConnHandler(0x817a85f1808, {0x104d3bea8, 0x817a86a6ba0}, {0x104d46670, 0x817a83021e8}, 0x817a843e400, 0x817a82cad80)
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2499 +0x58
created by github.com/valyala/fasthttp.(*Server).serveConn in goroutine 916
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2454 +0x12fc
exit status 2
➜  server git:(main) ✗ go run ./cmd/server

 ┌───────────────────────────────────────────────────┐ 
 │                   Fiber v2.52.5                   │ 
 │               http://127.0.0.1:8080               │ 
 │       (bound on host 0.0.0.0 and port 8080)       │ 
 │                                                   │ 
 │ Handlers ............ 88  Processes ........... 1 │ 
 │ Prefork ....... Disabled  PID ............. 19669 │ 
 └───────────────────────────────────────────────────┘ 

panic: concurrent write to websocket connection

goroutine 708 [running]:
github.com/fasthttp/websocket.(*messageWriter).flushFrame(0x71d53986dbf0, 0x1, {0x0?, 0x71d53979a5a0?, 0x71d5398e9778?})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:656 +0x428
github.com/fasthttp/websocket.(*messageWriter).Close(0x71d5395d8600?)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:770 +0x44
github.com/fasthttp/websocket.(*Conn).beginMessage(0x71d53979c9a0, 0x71d53979a5a0, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:513 +0x3c
github.com/fasthttp/websocket.(*Conn).NextWriter(0x71d53979c9a0, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:556 +0x44
github.com/fasthttp/websocket.(*Conn).WriteJSON(0x1019571e0?, {0x1019571e0, 0x71d539a84380})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/json.go:24 +0x2c
mysecondwebgame/server/internal/httpapi.(*Server).writeWSEvent(0x71d5395cb8c8, 0x71d53979a270, {0x1013b5a25, 0x10}, {0x10190ffe0, 0x71d539648040})
        ./server/internal/httpapi/server.go:2978 +0x278
mysecondwebgame/server/internal/httpapi.(*Server).handleWS(0x71d5395cb8c8, 0x71d53979a270)
        ./server/internal/httpapi/server.go:3110 +0x121c
github.com/gofiber/websocket/v2.New.func2.5(0x1019c6ad8?)
        $GOMODCACHE/github.com/gofiber/websocket/v2@v2.2.1/websocket.go:130 +0x6c
github.com/fasthttp/websocket.(*FastHTTPUpgrader).Upgrade.func1({0x1019c6ad8, 0x71d53979a480})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/server_fasthttp.go:201 +0x140
github.com/valyala/fasthttp.hijackConnHandler(0x71d539b56c08, {0x1019bbea8, 0x71d539726360}, {0x1019c6670, 0x71d5398821e0}, 0x71d539694400, 0x71d5396f0840)
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2499 +0x58
created by github.com/valyala/fasthttp.(*Server).serveConn in goroutine 665
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2454 +0x12fc
exit status 2
➜  server git:(main) ✗ go run ./cmd/server

 ┌───────────────────────────────────────────────────┐ 
 │                   Fiber v2.52.5                   │ 
 │               http://127.0.0.1:8080               │ 
 │       (bound on host 0.0.0.0 and port 8080)       │ 
 │                                                   │ 
 │ Handlers ............ 88  Processes ........... 1 │ 
 │ Prefork ....... Disabled  PID ............. 23667 │ 
 └───────────────────────────────────────────────────┘ 

panic: concurrent write to websocket connection

goroutine 653 [running]:
github.com/fasthttp/websocket.(*messageWriter).flushFrame(0x4c84435a660, 0x1, {0x0?, 0x4c8444230e0?, 0x4c844369778?})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:656 +0x428
github.com/fasthttp/websocket.(*messageWriter).Close(0x4c844274600?)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:770 +0x44
github.com/fasthttp/websocket.(*Conn).beginMessage(0x4c844460160, 0x4c8444230e0, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:513 +0x3c
github.com/fasthttp/websocket.(*Conn).NextWriter(0x4c844460160, 0x1)
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/conn.go:556 +0x44
github.com/fasthttp/websocket.(*Conn).WriteJSON(0x105a271e0?, {0x105a271e0, 0x4c844720240})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/json.go:24 +0x2c
mysecondwebgame/server/internal/httpapi.(*Server).writeWSEvent(0x4c8442678c8, 0x4c8445315f0, {0x105485a25, 0x10}, {0x1059dffe0, 0x4c8444a40c0})
        ./server/internal/httpapi/server.go:2978 +0x278
mysecondwebgame/server/internal/httpapi.(*Server).handleWS(0x4c8442678c8, 0x4c8445315f0)
        ./server/internal/httpapi/server.go:3110 +0x121c
github.com/gofiber/websocket/v2.New.func2.5(0x105a96ad8?)
        $GOMODCACHE/github.com/gofiber/websocket/v2@v2.2.1/websocket.go:130 +0x6c
github.com/fasthttp/websocket.(*FastHTTPUpgrader).Upgrade.func1({0x105a96ad8, 0x4c844531710})
        $GOMODCACHE/github.com/fasthttp/websocket@v1.5.3/server_fasthttp.go:201 +0x140
github.com/valyala/fasthttp.hijackConnHandler(0x4c8441f2008, {0x105a8bea8, 0x4c84446cde0}, {0x105a96670, 0x4c844182300}, 0x4c8442b4400, 0x4c844420900)
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2499 +0x58
created by github.com/valyala/fasthttp.(*Server).serveConn in goroutine 673
        $GOMODCACHE/github.com/valyala/fasthttp@v1.51.0/server.go:2454 +0x12fc
exit status 2
