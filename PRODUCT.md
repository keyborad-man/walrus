# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **主要用户**：面向开源社区的《饥荒联机版》（Don't Starve Together, DST）自建服主与小型亲友/社群运维人员。
- **使用情境**：在单台 Linux VPS、家庭 NAS 或 Docker 宿主机上长期自托管 1 至多个独立存档集群，既需要个人服主单人快速开箱即用，也支持将特定集群的运维或只读权限分享给亲友与小社群协管。
- **核心任务（Jobs）**：一键搭建与升级 DST 专用服务端、安全导入或新建森林/洞穴双世界存档、低门槛可视化配置世界参数与工坊模组、监控宿主机与在线玩家状态、执行实时控制台/GM 指令、管理定时备份与无人自动更新，以及在崩服或版本更新时及时收到告警。

## Product Purpose

Walrus 是一款单机多档《饥荒联机版》开服管理 Web 面板，以 Go 单静态二进制内嵌前端与本地 SQLite 的形态提供完整开服运维闭环。它的存在是为了消除手工编写 Lua/INI 配置、手动管理多开端口冲突、配置外部数据库以及繁琐 CLI 运维的门槛。成功意味着：服主只需一条命令或一次 `docker compose up` 即可稳定托管多个并发存档，日常调参、装 Mod、排障与备份均可安心在浏览器内完成，且随时能与底层磁盘文件无缝互通。

## Positioning

相比通用游戏面板（如 Pterodactyl）或同类饥荒工具及手工脚本，Walrus 的核心差异化由四项相互支撑的机制共同构成：

1. **单静态二进制 + 零外部数据库依赖**：采用 Go 单静态二进制 + 嵌入式 React/TypeScript SPA + 免 CGO SQLite + `gopher-lua` 沙箱，无需部署 MySQL/Redis 或额外运行时，一条命令或 Docker Compose 即可启动。
2. **磁盘配置为唯一真相源 + 无损 Lua/Mod 解析**：直接读写 `cluster.ini`、`server.ini`、`cluster_token.txt`、`leveldataoverride.lua`、`modoverrides.lua` 与三名单文件；手工修改或外部导入与面板编辑互不覆盖，可视化编辑保存前自动校验 Lua 语法并无损保留 Mod 扩展字段，并在安全沙箱中按界面语言解析 `modinfo.lua` 生成配置表单。
3. **单机真多开 + 森林/洞穴双世界深度打磨**：围绕 DST 森林（`Master`）与可选洞穴（`Caves`）结构优化，内置可配置 UDP 端口池（默认 `10990–11060`，每集群 7 端口）自动防冲突分配，支持集群级与分片级独立启停、崩溃自动拉起（5 分钟 3 次熔断）及优雅存档关服（`c_shutdown(true)`）。
4. **无侵入状态感知 + 一体化自动化运维**：通过日志解析与前端过滤的低频静默控制台探针采集天数/季节/阶段/在线玩家，整合在线 `c_save()` 备份（恢复前自动生成 `pre_restore` 快照）、内置 Cron 引擎（定时备份、倒计时重启、定时公告、无人在线自动更新服务端）以及国内外多平台 Webhook 告警。

## Operating Context

- **运行与部署环境**：
  - 生产环境：Linux Docker Compose（推荐 `network_mode: host` 避免 UDP 代理损耗并便于扩展端口池）或原生 Linux 静态二进制，统一挂载单数据目录（默认 `/data` 或 `./data`）存放 SQLite、SteamCMD、DST 服务端、存档与备份。
  - 本地开发与联调：支持 Windows/跨平台 `--runtime=mock` 仿真模式，无需真实 DST 服务端即可验证完整 UI 与运维流。
- **核心外部对象与文件**：
  - 磁盘存档结构：`Cluster` 目录下 `Master/`、`Caves/`、`cluster.ini`、`server.ini`、`cluster_token.txt`、`leveldataoverride.lua`、`modoverrides.lua`、`adminlist.txt`、`whitelist.txt`、`blocklist.txt`、`dedicated_server_mods_setup.lua`、`modinfo.lua`。
  - 外部服务与资产：SteamCMD（支持 `public`/`updatebeta` 分支及 64/32 位切换）、Klei 官方版本检测与 Cluster Token、Steam 创意工坊（支持免 API Key 按 ID/URL/合集导入，亦支持配置 Steam Web API Key 搜索）及本地单机存档 `Cluster_X.zip` / 私有 Mod `.zip`。
  - 通知渠道：Discord、飞书、钉钉、企业微信、Telegram、Bark 及通用 JSON（HMAC-SHA256）。
- **权限角色**：三角色 RBAC（`admin` 超管、`operator` 集群运维、`viewer` 集群只读）与按集群粒度授权，首次启动经 `/setup` 初始化超管并支持配置全局默认 Cluster Token。

## Capabilities and Constraints

### 已确认的核心能力
- **生命周期与多开**：多集群并发运行、自动端口池预览与冲突检测、新建集群与本地存档 ZIP 一键导入、分片独立启停与崩溃熔断保护。
- **服务端与世界/模组配置**：SteamCMD 实时日志流更新与侧边栏更新提示；森林/洞穴双模式编辑器（可视化表单 + 官方预设 + Lua 源码）；工坊批量导入/搜索/预下载、沙箱解析 `modinfo.lua`、纯客户端 Mod 防误装提醒、主副世界 Mod 同步开关、本地私有 Mod 上传。
- **实时运维与自动化**：双世界 WebSocket 控制台、日志过滤与指令历史、GM 快捷指令与自定义 Lua 片段收藏；玩家在线管理（复活/设管/踢出/封禁）、带 `KU_ID` 历史昵称映射的三名单管理、独立游戏聊天视图；在线备份/下载/恢复与 Cron 定时任务；受限集群文件浏览器（阻断路径穿越与符号链接逃逸）；多平台 Webhook 推送与投递记录；登录限流、密钥脱敏与操作审计日志。

### 必须守住的硬约束
- **单机自托管架构**：保持单静态二进制 + 嵌入式前端 + 本地 SQLite 的零外部依赖架构，不引入必须依赖的外部数据库或分布式基础设施。
- **离线与国内网络自包含**：所有前端静态资源必须随 Go 二进制离线打包交付，禁止强依赖外网 CDN（如外部字体库、远程图标/脚本 CDN），确保内网环境与国内网络下开箱即用。
- **磁盘真相源与无损读写**：任何配置编辑器都不得破坏手工维护的磁盘文件结构或丢弃 Lua 中的 Mod 扩展字段。
- **安全边界**：文件浏览器严格限制在集群目录内，敏感令牌与 Webhook 密钥须脱敏处理，高危运维操作留存审计日志。

### 领域术语
- **集群（Cluster）**：一组独立的 DST 存档目录与运行实例，包含森林与可选洞穴分片。
- **分片（Shard）**：`Master`（森林主世界）与 `Caves`（洞穴副世界）。
- **Cluster Token**：Klei 官方颁发的服务器认证令牌，支持全局默认与集群级覆盖。
- **KU_ID**：Klei 玩家唯一标识符，用于三名单与玩家历史昵称关联。
- **Mock 模式**：`--runtime=mock` 下的本地仿真驱动，用于无真实 DST 二进制时的开发与演示。

### 尚未锁定 / 待定事项
- **开源协议**：仓库根目录尚未提交 `LICENSE` 文件，具体开源许可证待定。
- **多语言、多世界拓扑与品牌命名边界**：当前代码库默认内置 `zh-CN` 与 `en-US` 双语、围绕 `Master` + `Caves` 双世界模型设计、界面使用「Walrus / 饥荒联机版开服管理面板」，但这些尚未作为不可更改的永久产品禁区锁定。

## Brand Commitments

- **现用名称**：`Walrus`（中文简称：`海象` / `海象面板`；中文副标题：`饥荒联机版开服管理面板`；英文副标题：`Don't Starve Together Server Manager`）。
- **语言基调**：务实、面向运维清晰准确，默认中文（`zh-CN`）并附带英文（`en-US`）词条（见 `web/src/i18n/zh.ts` 与 `web/src/i18n/en.ts`）。
- **品牌标识**：内置极简几何风格“戴贝雷帽的海象” SVG 图形标（`web/src/components/WalrusLogo.tsx` 与 `web/public/favicon.svg`）。

## Evidence on Hand

- **真实项目文档与代码**：
  - 产品功能与部署说明：[README.md](README.md)
  - Go 服务端、API 路由与业务实现：`cmd/walrus/`、`internal/api/router.go`、`internal/service/`、`internal/dst/`、`internal/lua/`、`internal/runtime/`、`internal/steam/`
  - 完整 15 页 Web 前端实现与双语词条：`web/src/pages/`、`web/src/components/`、`web/src/i18n/zh.ts`、`web/src/i18n/en.ts`
  - Docker 部署编排：`docker/Dockerfile`、`docker/docker-compose.yml`
  - Windows 本地 Mock 仿真驱动：`internal/runtime/mock_driver.go`
- **明确缺失（后续工作严禁编造）**：
  - 尚无公开用户评价、真实社区部署数量、性能基准测试数据、媒体报道或客户案例。
  - 尚无已提交的开源许可证文件、定价或商业授权声明。
  - 尚无现成的产品营销截图、独立品牌 Logo 图片资产或外部官网文案。

## Product Principles

1. **零依赖、即装即用优先**：部署与日常运行必须保持单二进制/单容器自包含，所有前端资源离线随包发布，不为次要特性引入外部数据库或外网 CDN 依赖。
2. **磁盘文件永远是真相源**：面板是磁盘存档的安全操作界面而非黑盒数据库，必须尊重手工编辑与外部导入的存档，并在写入 Lua 配置时做到语法校验与字段无损。
3. **针对饥荒场景做深，而非通用面板做宽**：聚焦森林/洞穴双世界、多开端口池、SteamCMD 更新、工坊模组解析与游戏内玩家/GM 工具，把高频 DST 运维动作做到顺手可靠。
4. **安全可控的自动化与协作**：所有自动任务（备份、重启、无人更新）与高风险操作（恢复快照、文件编辑、踢封玩家）均需具备可预期边界（熔断、恢复前快照、权限隔离、目录沙箱与审计留痕）。
5. **诚实呈现真实状态**：面向运维者优先传递准确、低干扰的宿主机与世界实时状态，过滤探测噪音，不编造未经实测的指标或虚假数据。
