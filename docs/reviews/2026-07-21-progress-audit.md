# 2026-07-21 Phase 0/1 进度与证据审计

> 更新时间: 2026-07-21 | 状态: 进行中 | 基线: 本地 `main`

## 1. 审计结论

审计前文档把 Phase 0 标为完成、Phase 1 标为待启动，但代码已经存在未成熟的 IM 实现，同时 Phase 0 的 Python CI、Streams 重领、Shell 脚本和 Compose 可复现性没有达到退出标准。名义进度与实际可验收进度不一致。

本轮按“恢复可信基线 -> 加固 IM -> 同步文档”实施。当前正式状态统一为“Phase 0 本地完成 / Phase 1 后端候选 / Phase 2 后端进行中”；浏览器与发布验收等待 Phase 2 质量门禁。

## 2. 初始差距

| 领域 | 审计前文档 | 审计前实际 | 风险 |
|------|------------|------------|------|
| Phase 0 | 已完成 | Python dev 依赖安装不可复现；Ruff 失败；Streams claim 后未处理且首次失败 ACK；测试脚本语法错误 | CI 与消息可靠性不可信 |
| Compose | 已交付 | latest 镜像、凭证不一致、seed/cleanup 会随初始化执行、无 AI readiness | 新环境结果不确定 |
| Phase 1 | 待启动 | 已有 Chat/WS/UI，但公开 join、近似未读、非事务写、弱 WS 认证、宽松 Origin、无恢复去重 | 文档与代码方向相反，且安全/一致性不足 |
| 全站安全 | 待 CSRF Token | 有副作用 GET logout/view，浏览器写操作没有全局跨源保护 | CSRF 目标未闭环 |
| 任务治理 | GitHub Issues 强制 | 本地 `main` 和文档才是实际工作基线 | 看板与进度可能分叉 |

## 3. 计划—实现—证据矩阵

历史收口工作对应提交 `57b9615`，后续发布状态记录为 `34f1069`，Phase 2 初始切片为 `7a96feb`。

| 计划项 | 实现位置 | 提交 | 自动测试证据 | 手工/环境证据 | 状态 |
|--------|----------|------|--------------|---------------|------|
| Streams 4 次处理、XAUTOCLAIM、最终 ACK | `ai-service/app/workers/consumer.py` | `57b9615` | Python 单测覆盖 ACK/重试/重领/重启；真实 Redis DB 15 的 2 项集成测试通过 | Redis 证据已取得 | 已实现 |
| Python 3.12/uv lock/CI | `pyproject.toml`, `uv.lock`, `python.yml` | `57b9615` | Ruff + format + pytest 通过 | CI 尚未在远端运行 | 加固中 |
| Make/Shell | `Makefile`, `scripts/test_api.sh`, `scripts/test_chat.sh` | `57b9615` | `bash -n` 已通过 | API/IM 脚本通过；API 34/34 | 已实现 |
| Compose/AI readiness | `deploy/docker-compose.yml`, `ai-service/Dockerfile`, `/healthz`, `/readyz` | `57b9615` | health 单测通过 | Compose config/up/ready/reset 通过；MySQL 15 张结构表、users=0；镜像 digest 已记录 | 已实现 |
| slog 收口 | `internal/service`, `repository`, `ws` | `57b9615` | 全仓检索无业务 `log.Printf/log.Println` | — | 已实现 |
| 010 迁移与事务聊天 | `migrations/010_chat_hardening.sql`, `chat_repo.go` | `57b9615` | 009→010 临时库迁移及 MySQL 集成通过；测试库/用户已自动清理 | 真实 MySQL 证据已取得 | 已实现 |
| 邀请制群组/API | `chat_service.go`, `chat_handler.go`, `router.go` | `57b9615` | 权限、限制、错误映射单测通过 | `scripts/test_chat.sh` 通过；双浏览器暂缓 | 已实现（终端） |
| 精确未读/MarkRead | `chat_repo.go` | `57b9615` | 真实 MySQL 验证准确未读、消息归属和单调读标记 | 证据已取得 | 已实现 |
| WS 认证/吊销/在线 | `middleware/auth.go`, `ws/`, auth/admin handlers, `presence.go` | `57b9615` | Hub 多连接与慢连接单测通过 | Redis Compose 已就绪；双浏览器暂缓 | 已实现（终端/单测） |
| 去重/断线恢复/群 UI | `web/templates/chat/chat.html` | `57b9615` | API 恢复路径纳入冒烟脚本 | 页面手工与断网测试暂缓 | 后端候选 |
| Origin/CSRF/无副作用 GET | `cross_origin.go`, `chat_handler.go`, router/templates | `57b9615` | 同源/可信/跨站/CLI 矩阵通过 | 部署域名 trusted origin 待验证 | 后端候选 |
| 文档与任务事实源 | `TASK.md`, `docs/plan.md`, 本审计等 | `34f1069` | 链接检查通过 | 已同步终端验收结果与暂缓决策 | 已实现 |

## 4. 门禁记录

| 门禁 | 2026-07-21 结果 | 说明 |
|------|----------------|------|
| Go 单元测试 | 通过 | `go test ./...` |
| Python lint/format/unit | 通过 | Ruff、format check、10 项非集成 pytest |
| Shell 语法 | 通过 | `start.sh` 与 `scripts/*.sh` |
| `make check` | 通过 | Go fmt/vet/test、Python locked lint/format、Shell 与 Markdown 链接 |
| `go test -race ./...` | 通过 | 最终工作区全包竞态检测通过 |
| MySQL/Redis integration | 通过 | 临时 `*_test` 库完成 009→010 后 Go 集成通过；Redis 2 项真实集成通过；临时数据库和用户已清理 |
| Compose config/up/ready/reset | 通过 | 使用本机 Docker Compose；通过 `make dev-config`、`make dev-up`、`make dev-ready`、`make dev-reset`；MinIO/Qdrant/uv digest 已写入 Compose/Dockerfile |
| API 冒烟 | 通过 | `bash scripts/test_api.sh`：34 PASS / 0 FAIL；`bash scripts/test_chat.sh`：全部通过 |
| 双浏览器 | 暂缓 | 用户确认后再做；清单保留在 `docs/design/im.md`，不阻塞当前后端终端成熟度推进 |
| 文档链接/工作区检查 | 通过 | 文档链接通过；Python 缓存已忽略并清理，无意外生成文件 |

## 5. 完成判定

Phase 0 和 Phase 1 只有在 `TASK.md` 最终门禁全部有可复核证据时才能改为完成。当前没有满足这一条件；后续不得仅依据文件存在、单元测试通过或主观演示标记完成。

## 6. 真实集成暴露并修复的问题

1. 并发 `EnsureDM` 的“INSERT IGNORE 后回查”可能在竞争事务间返回 `record not found`。已改为原子 upsert，并通过 `LAST_INSERT_ID(id)` 让冲突请求取得同一会话 ID。
2. `MarkRead` 对较旧消息执行 `GREATEST` 时，MySQL 合法返回 `RowsAffected=0`，旧实现误判成员不存在。已改为事务内锁定成员、验证消息归属后更新，零变化视为幂等成功。

这两项说明单元测试不能代替真实 MySQL 语义验证，也是阶段仍坚持证据门禁的原因。

## 7. 本轮增量记录（2026-07-21）

- Compose 首次验收发现两个可复现性问题并已修复：计划中的 MinIO 2025-10 标签不存在；MySQL 初始化脚本的 `set -u` 会污染官方 entrypoint。现使用可拉取的 MinIO `RELEASE.2025-09-07T16-13-09Z`、Qdrant `v1.18.2`，并锁定实际 digest；uv 构建基础镜像也锁定 digest。
- 全新卷启动后检查到 15 张结构表且 `users=0`，证明初始化只执行结构迁移，没有自动 seed。`migrations/002_seed.sql` 仅在显式终端验收前手动导入。
- 终端 API 证据：通用接口脚本 34/34；IM 脚本覆盖 DM 幂等、精确未读、MarkRead、`after_id`、分页冲突、邀请幂等、成员退出、群主解散和消息长度限制。
- 当前工作策略已更新：在后端/接口成熟前不继续投入浏览器和前端页面验收；双浏览器清单保留为后续发布前门禁。终端 curl/Shell 是当前正式验收证据，不等同于最终 UI 验收。

本轮环境锁定值：

| 镜像 | 版本 | digest |
|------|------|--------|
| MinIO | `RELEASE.2025-09-07T16-13-09Z` | `sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e` |
| Qdrant | `v1.18.2` | `sha256:75eab8c4ba42096724fdcfde8b4de0b5713d529dde32f285a1f86fdcb2c9e50c` |
| uv Python 3.12 基础镜像 | tag-less digest | `sha256:e5b65587bce7de595f299855d7385fe7fca39b8a74baa261ba1b7147afa78e58` |

## 8. 文档复核与当前发布状态（2026-07-21）

- 复核发现并修正 `docs/plan.md` Phase 1.1 的状态漂移：迁移后的真实 `*_test` MySQL 集成已通过，不再列为缺失证据；保留 CI/发布环境迁移演练作为发布前补充。
- `TASK.md`、`docs/plan.md`、`docs/roadmap.md` 已重构为状态、执行索引和里程碑三层；阶段细节移入 `docs/phases/`。
- 本地 Markdown 链接检查（48 个文件）与 `git diff --check` 已通过；本轮修改已形成本地提交并发布到 `agent/phase0-phase1-closeout`。由于 SSH/HTTPS Git transport 受当前网络限制，远端分支通过 GitHub API 写入等价快照，文件树已与本地 HEAD 核对一致；远端 CI 结果待确认。
- 浏览器和前端手工验收继续按用户确认暂缓，不改变当前后端终端验收结论。

## 9. Homebrew MinIO 故障记录与 Phase 2 启动

- 2026-07-21 检查发现 Homebrew MinIO 未监听 `127.0.0.1:9000`，但 `$HOME/minio_data/shareo` 与 MySQL `post_images` 记录仍存在；图片失效原因为对象存储未启动，不是数据库图片记录被删除。
- 本地启动固定使用 Homebrew 安装的 MinIO 二进制、`$HOME/minio_data`、API `9000`、Console `9001`；Compose named volume 不参与本机数据清理。
- 旧数据按用户确认清理 MySQL `shareo`、Redis DB 0 和 MinIO `shareo` bucket；Phase 2 先实现后端语义搜图闭环，浏览器/前端暂缓。

## 10. Phase 2 后端实现记录（`7a96feb`）

- Go 新增受 `X-Internal-Token` 保护的索引载荷接口和 `GET /api/v1/search/images`；查询侧再次过滤 approved/未删除帖子。
- Python 新增 Chinese-CLIP 懒加载、Qdrant `images` collection、幂等 `index_post` worker；worker 通过 Go 图片代理读 medium 图片，不持有 MinIO 凭证。
- 审核通过/驳回、用户删除、管理员删除和编辑重审进入 Streams；`make backfill-index` 支持 approved 存量回填。
- 新增 `scripts/test_image_search.sh`，采用注册、上传、审核、轮询搜索命中、删除后消失的 curl 验收；需要显式 seed 管理员后运行。
- 初始自动证据为 Go `go test ./...`、Python 20 项非集成测试、Ruff/format、Shell 语法和 Markdown 链接检查；真实 Qdrant + Chinese-CLIP 端到端仍待验收，因此 Phase 2 不标记完成。

## 11. Phase 2 成熟化与文档分阶段重构

- `TASK.md` 只保留当前状态和证据入口；`docs/plan.md` 改为执行索引，Phase 0–4 各自使用独立阶段文档。
- 固定 Chinese-CLIP revision，增加模型后台预热、搜索专用 readiness/meta、并发限制、8 秒调用超时、Qdrant schema 拒绝覆盖和 `post_id` payload index。
- 公网结果按帖子最高分去重且不暴露 `object_key`；增加索引 dry-run/apply 对账和拒绝空标签的 40 条查询评测器。
- 隔离 E2E 固定使用 `shareo_e2e`、Redis DB 15、`shareo-e2e` bucket、`images-e2e` collection，覆盖删除 10 秒内不可见、重复回填和 worker 重启 45 秒内 pending 重领，并自动清理隔离资源。
- 本轮已通过 Python 27 项非集成测试、2 项 Redis 集成测试、Go 单元/真实 MySQL 集成/race、Compose 配置、Shell 语法和 61 个 Markdown 链接；测试后的 `shareo_test` 与 Redis DB 15 均已清空。真实模型/Qdrant E2E 与人工评测尚未取得证据，Phase 2 保持进行中。
