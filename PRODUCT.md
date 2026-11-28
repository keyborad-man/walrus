# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

- **主要用户**：个人服主。在自己的 Linux VPS、家用 NAS 或 Docker 宿主机上，为一小群朋友开《饥荒联机版》（Don't Starve Together, DST）专用服务器。
- **使用情境**：服一旦开好就长期挂着，服主只在几个时刻打开面板：开新档、朋友想换世界设置或加 Mod、有人掉线或崩服、游戏更新之后。他要的是服稳定在线、改动不出错，而不是一份运维工作。
- **技术水平跨度大，两端都要照顾**：有的服主只会照教程跑 `docker compose`，不想碰 Lua/INI，也记不住控制台命令；有的服主熟悉 Linux，只是嫌手工运维麻烦。默认用游戏里的语言和概念表达；Lua 源码、磁盘文件、端口、控制台这些底层细节保留下来，给懂行的人随时可达。
- **次要用户**：服主可以把某个集群的运维或只读权限分给朋友协管。这是顺带支持的能力，不是主线。

## Product Purpose

Walrus 是一个自托管的饥荒联机版开服管理 Web 面板。它把开服、调世界、装 Mod、看玩家、备份、更新这些原本需要手写配置文件和敲命令的事情，变成浏览器里的操作。

成功意味着：服主用一条命令把面板跑起来，之后开档、改设置、装 Mod、处理崩服和更新都在面板里完成，不需要打开终端；朋友上线时服是好的。

## Positioning

**为饥荒而生，懂游戏。** Walrus 不是在通用容器面板（如 Pterodactyl、MCSManager）上套一层壳。世界配置、Mod、玩家、控制台都按 DST 自己的结构和说法来做：

- 世界按森林（`Master`）+ 可选洞穴（`Caves`）双分片组织，世界设置对应 DST 客户端的选项分类与官方预设。
- Mod 在沙箱里解析 `modinfo.lua`，按界面语言生成配置表单，并能识别纯客户端 Mod，避免误装。
- 玩家管理、三名单（管理员/白名单/黑名单）、GM 快捷指令、游戏天数/季节/阶段，都用游戏里的概念呈现。

通用面板做不到这种程度的 DST 专精，这是邻近产品没法照搬的定位。

## Operating Context

- **部署形态**：Linux 上用 Docker Compose（镜像 `ghcr.io/keyborad-man/walrus`，推荐 `network_mode: host`）或原生静态二进制运行，所有数据放在一个数据目录里（默认 `/data`），默认监听 `:8080`。开发时可用 `--runtime=mock` 在没有真实 DST 服务端的情况下跑完整 UI。
- **磁盘存档结构**：每个集群是一个 DST `Cluster` 目录，包含 `Master/`、`Caves/`、`cluster.ini`、`server.ini`、`cluster_token.txt`、`leveldataoverride.lua`、`modoverrides.lua`、`adminlist.txt`、`whitelist.txt`、`blocklist.txt`。服主可能手工改过这些文件，也可能从单机存档 `Cluster_X.zip` 导入。
- **外部依赖**：SteamCMD（安装和更新 DST 服务端与 Mod）、Klei 的版本检测与 Cluster Token、Steam 创意工坊；可选配置网络代理、Steam Web API Key。
- **通知渠道**：Discord、飞书、钉钉、企业微信、Telegram、Bark，以及通用 JSON Webhook。

## Capabilities and Constraints

### 已有能力（以代码为准）

- **开服与多开**：同一台机器跑多个集群，自动从端口池分配端口并检测冲突；新建集群或导入本地存档 ZIP；集群级和分片级启停；崩溃自动拉起，带熔断。
- **服务端与 Mod**：通过 SteamCMD 安装和更新服务端，有版本提醒；创意工坊按 ID/URL/合集导入或搜索；启动时自动补下缺失的 Mod、更新过期的 Mod，Mod 页面提示可用更新；上传私有 Mod；DST 与 Mod 的定时自动更新。
- **世界配置**：森林/洞穴的可视化表单、官方预设与 Lua 源码三种编辑方式，保存前校验语法。
- **运行中**：WebSocket 实时控制台与 GM 快捷指令、在线玩家操作（复活/设管/踢出/封禁）、三名单管理、游戏聊天视图、公网 IP 直连信息。
- **保底与自动化**：在线备份与恢复（恢复前自动快照）、Cron 定时任务（备份、重启、公告、更新）、Webhook 告警。
- **面板自身**：账号与按集群授权（`admin` / `operator` / `viewer`）、操作审计、受限的集群文件浏览器、面板版本检测与 Docker 一键升级。

### 硬约束

没有强制性硬约束。当前实现的做法（单二进制 + 本地 SQLite、前端资源离线打包、默认 `zh-CN` 并附 `en-US` 词条、配置直接读写磁盘文件）都只是现状，以后可以改。

### 领域术语

- **集群（Cluster）**：一份独立的 DST 存档及其运行实例。
- **分片（Shard）**：`Master`（森林）与 `Caves`（洞穴）。
- **Cluster Token**：Klei 颁发的服务器令牌。
- **KU_ID**：Klei 玩家唯一标识。

### 尚未决定

- 开源许可证：仓库里还没有 `LICENSE` 文件。
- 名称、图标、副标题等品牌元素均未锁定（见下文）。

## Brand Commitments

没有锁定的品牌元素。当前在用（以后可以换）：

- 名称 `Walrus`（中文 `海象`），副标题「饥荒联机版开服管理面板」/「Don't Starve Together Server Manager」。
- 图形标：戴贝雷帽的海象（`web/src/components/WalrusLogo.tsx`、`web/public/favicon.svg`）。
- 语言风格：务实、准确。默认说游戏里的话，涉及底层操作时不含糊。

## Evidence on Hand

- **阶段**：以自用为主，顺便开源，不主动推广。
- **可用材料**：[README.md](README.md)（功能与部署说明）、`web/src/`（前端页面与双语词条）、`internal/`（服务端实现）、`docker/`（部署编排）。
- **不存在、不得编造**：用户评价、部署数量、社区规模、性能数据、媒体报道、营销截图、许可证与定价声明。

## Product Principles

1. **服主不该需要终端**：日常开档、调参、装 Mod、排障、更新都能在面板里完成；需要打开终端才能做的事，算产品缺口。
2. **说游戏的话，留住底层**：默认用 DST 的概念和词汇表达，让不懂运维的服主也能看懂；源码、文件、端口、日志对懂行的人一步可达，不藏起来。
3. **在饥荒上做深，不做通用面板**：优先把 DST 特有的高频动作做顺手（世界、Mod、玩家、更新），不为了泛化而牺牲专精。
4. **不弄坏朋友的存档**：高风险操作（恢复、覆盖、踢封、更新）都要有可预期的保护（自动快照、校验、确认）。
5. **诚实呈现状态**：只展示真实采集到的状态，不编造指标，不用探测噪音打扰服主。
