# Walrus — 饥荒联机版开服管理 Web 服务

现代化的 Don't Starve Together（饥荒联机版）单机多档开服管理面板。采用 **Go 单静态二进制 + 嵌入式 React/TypeScript SPA + 免 CGO SQLite + Go Lua 沙箱** 架构，零外部数据库依赖，支持 Linux Docker 一键部署、原生 Linux 部署以及 Windows 本地 Mock 开发联调。

## 完整功能一览

### 1. 集群生命周期与并发多开
- **并发多开与自动端口池**：支持单台服务器同时运行多个独立存档集群，自动从可配置端口池（默认 `10990–11060`）分配 7 个无冲突 UDP 端口并做冲突检测
- **固定双世界精简体验**：针对森林（`Master`）与可选洞穴（`Caves`）深度优化，支持集群级与分片级独立启停、崩溃自动拉起（5 分钟 3 次熔断保护）及优雅存档关服（`c_shutdown(true)`）
- **磁盘真相源设计**：`cluster.ini`、`server.ini`、`cluster_token.txt`、`leveldataoverride.lua`、`modoverrides.lua`、三名单直接读写磁盘目录，支持手工修改或外部导入不冲突
- **本地存档 ZIP 一键导入**：直接上传单机/客户端导出的 `Cluster_X.zip` 或备份压缩包，自动识别森林/洞穴并重映射空闲端口

### 2. SteamCMD、可视化世界与全功能 Mod 管理
- **SteamCMD 实时更新流**：内置安装与更新进度终端、Klei 官方最新版本检测与侧边栏提醒、`public`/`updatebeta` 分支及 64/32 位架构切换、日志持久化不因刷新丢失
- **世界配置双模式编辑器**：森林/洞穴全分类中英双语可视化表单 + 官方预设模板 + Lua 源码切换，保存前自动校验 Lua 语法且**无损保留 Mod 扩展字段**
- **全功能模组管理**：支持 Steam 工坊 ID / URL / 合集批量导入（免 API Key）、配置 Steam Web API Key 后的工坊搜索、SteamCMD 预下载、`gopher-lua` 安全沙箱按当前界面语言解析 `modinfo.lua` 生成可视化配置表单、纯客户端 Mod 防误装警告、主副世界 Mod 同步开关、自动维护 `dedicated_server_mods_setup.lua`、本地私有 Mod `.zip` 上传

### 3. 实时运维、玩家、自动化与告警
- **无侵入实时状态与玩家系统**：通过日志解析与低频控制台静默探针（前端自动过滤）采集天数/季节/阶段/在线玩家，提供一键复活/设管/踢出/封禁、可视化 `adminlist`/`whitelist`/`blocklist` 管理（带 `KU_ID` 历史昵称映射）及独立游戏聊天视图
- **实时控制台 & GM 工具箱**：双世界 WebSocket 日志流、关键字过滤、上下键指令历史、内置世界维护/四季天气/复活清垃圾快捷按钮及持久化自定义 Lua 指令收藏
- **备份恢复与 Cron 定时任务**：在线备份自动触发 `c_save()`、按保留份数自动清理、恢复前自动创建 `pre_restore` 快照；内置 Cron 引擎支持定时备份、带倒计时广播的定时重启、定时公告、无人在线自动更新服务端
- **受限文件浏览器**：严格限制在集群目录内（阻断路径穿越与符号链接逃逸），支持查看/下载崩溃日志与在线编辑配置文件
- **Webhook 多平台告警**：内置 Discord / 飞书 / 钉钉 / 企业微信 / Telegram / Bark / 通用 JSON（HMAC-SHA256）模板，支持崩溃、启停、玩家进出、备份、版本更新事件推送与投递记录查询
- **多账号 RBAC 与审计**：首次启动 `/setup` 创建超管，支持 `admin` / `operator` / `viewer` 角色与集群粒度授权、登录限流、密钥脱敏及操作审计日志
- **版本检测与 Docker 一键升级**：支持展示当前面板版本号与构建信息、自动检测 GitHub Releases（并兼容 GHCR `latest` 镜像变更）；挂载 `/var/run/docker.sock` 后可在「系统设置」中一键拉取新镜像、优雅存档关服并安全替换容器（异常时自动回滚旧容器）

---

## 快速开始

### 方式一：Linux Docker Compose 一键部署（生产推荐）

默认直接拉取 GitHub Container Registry 预构建镜像（`ghcr.io/keyborad-man/walrus:latest`）：

```bash
cd docker
docker compose up -d
```

后续更新到最新版本可直接点击面板「系统设置」中的 **一键升级镜像**（需保持 `docker-compose.yml` 中的 `/var/run/docker.sock` 挂载），或手动执行：

```bash
cd docker
docker compose pull
docker compose up -d
```

如需从本地源码自行构建镜像，可执行 `make docker-build` 或取消 [docker/docker-compose.yml](docker/docker-compose.yml) 中的 `build` 注释后运行 `docker compose up -d --build`。

默认挂载 `./data:/data`（包含 SQLite 数据库、SteamCMD、饥荒服务端、所有集群存档与备份），监听 `:8080`。

### 方式二：原生 Linux 单二进制部署

```bash
# 编译前端并打包 Linux 静态二进制
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/walrus ./cmd/walrus

# 启动服务
./bin/walrus --addr=:8080 --data-dir=/data --runtime=linux
```

### 方式三：Windows 本地运行（Mock 仿真模式）

```powershell
.\bin\walrus.exe --runtime=mock --data-dir=.\data
```

启动后访问 `http://127.0.0.1:8080` 进入初始化引导页。

## 常用环境变量 / 启动参数

| 环境变量 | CLI 参数 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `WALRUS_ADDR` | `--addr` | `:8080` | HTTP 监听地址 |
| `WALRUS_DATA_DIR` | `--data-dir` | `./data` | 统一持久化数据根目录 |
| `WALRUS_RUNTIME` | `--runtime` | `auto` | 运行驱动：`auto` / `linux` / `mock` |
| `WALRUS_JWT_SECRET` | — | 自动生成持久化 | 可选自定义 JWT 签名密钥 |
| `WALRUS_COOKIE_SECURE` | `--cookie-secure` | `false` | HTTPS 反代下建议开启 |
| `WALRUS_DEV` | `--dev` | `false` | 开发模式（启用 CORS 与 Debug 日志） |
