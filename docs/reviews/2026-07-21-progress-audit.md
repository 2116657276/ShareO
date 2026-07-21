# 2026-07-21 Phase 0/1 进度与证据审计

> 更新时间: 2026-07-21 | 状态: 进行中 | 基线: 本地 `main`

## 1. 审计结论

审计前文档把 Phase 0 标为完成、Phase 1 标为待启动，但代码已经存在未成熟的 IM 实现，同时 Phase 0 的 Python CI、Streams 重领、Shell 脚本和 Compose 可复现性没有达到退出标准。名义进度与实际可验收进度不一致。

本轮按“恢复可信基线 -> 加固 IM -> 同步文档”实施。当前后端与终端验收已显著前移：真实 MySQL/Redis、Compose、API/IM 冒烟均已取得证据；浏览器与前端手工验收按用户确认暂缓。因此 `TASK.md` 保持“Phase 0 本地门禁完成、远端 CI 待确认 / Phase 1 后端加固中”，不以代码存在代替阶段完成。

## 2. 初始差距

| 领域 | 审计前文档 | 审计前实际 | 风险 |
|------|------------|------------|------|
| Phase 0 | 已完成 | Python dev 依赖安装不可复现；Ruff 失败；Streams claim 后未处理且首次失败 ACK；测试脚本语法错误 | CI 与消息可靠性不可信 |
| Compose | 已交付 | latest 镜像、凭证不一致、seed/cleanup 会随初始化执行、无 AI readiness | 新环境结果不确定 |
| Phase 1 | 待启动 | 已有 Chat/WS/UI，但公开 join、近似未读、非事务写、弱 WS 认证、宽松 Origin、无恢复去重 | 文档与代码方向相反，且安全/一致性不足 |
| 全站安全 | 待 CSRF Token | 有副作用 GET logout/view，浏览器写操作没有全局跨源保护 | CSRF 目标未闭环 |
| 任务治理 | GitHub Issues 强制 | 本地 `main` 和文档才是实际工作基线 | 看板与进度可能分叉 |

## 3. 计划—实现—证据矩阵

“提交”列在用户明确要求提交前记录为当前工作区；后续提交时替换为 commit SHA。

| 计划项 | 实现位置 | 提交 | 自动测试证据 | 手工/环境证据 | 状态 |
|--------|----------|------|--------------|---------------|------|
| Streams 4 次处理、XAUTOCLAIM、最终 ACK | `ai-service/app/workers/consumer.py` | 当前工作区 | Python 单测覆盖 ACK/重试/重领/重启；真实 Redis DB 15 的 2 项集成测试通过 | Redis 证据已取得 | 已实现 |
| Python 3.12/uv lock/CI | `pyproject.toml`, `uv.lock`, `python.yml` | 当前工作区 | Ruff + format + pytest 通过 | CI 尚未在远端运行 | 加固中 |
| Make/Shell | `Makefile`, `scripts/test_api.sh`, `scripts/test_chat.sh` | 当前工作区 | `bash -n` 已通过 | API/IM 脚本通过；API 34/34 | 已实现 |
| Compose/AI readiness | `deploy/docker-compose.yml`, `ai-service/Dockerfile`, `/healthz`, `/readyz` | 当前工作区 | health 单测通过 | Compose config/up/ready/reset 通过；MySQL 15 张结构表、users=0；镜像 digest 已记录 | 已实现 |
| slog 收口 | `internal/service`, `repository`, `ws` | 当前工作区 | 全仓检索无业务 `log.Printf/log.Println` | — | 已实现 |
| 010 迁移与事务聊天 | `migrations/010_chat_hardening.sql`, `chat_repo.go` | 当前工作区 | 009→010 临时库迁移及 MySQL 集成通过；测试库/用户已自动清理 | 真实 MySQL 证据已取得 | 已实现 |
| 邀请制群组/API | `chat_service.go`, `chat_handler.go`, `router.go` | 当前工作区 | 权限、限制、错误映射单测通过 | `scripts/test_chat.sh` 通过；双浏览器暂缓 | 已实现（终端） |
| 精确未读/MarkRead | `chat_repo.go` | 当前工作区 | 真实 MySQL 验证准确未读、消息归属和单调读标记 | 证据已取得 | 已实现 |
| WS 认证/吊销/在线 | `middleware/auth.go`, `ws/`, auth/admin handlers, `presence.go` | 当前工作区 | Hub 多连接与慢连接单测通过 | Redis Compose 已就绪；双浏览器暂缓 | 已实现（终端/单测） |
| 去重/断线恢复/群 UI | `web/templates/chat/chat.html` | 当前工作区 | API 恢复路径纳入冒烟脚本 | 页面手工与断网测试暂缓 | 加固中（页面验收待后） |
| Origin/CSRF/无副作用 GET | `cross_origin.go`, `chat_handler.go`, router/templates | 当前工作区 | 同源/可信/跨站/CLI 矩阵通过 | 部署域名 trusted origin 待验证 | 加固中 |
| 文档与任务事实源 | `TASK.md`, `docs/plan.md`, 本审计等 | 当前工作区 | 链接检查通过 | 已同步终端验收结果与暂缓决策 | 已实现 |

## 4. 门禁记录

| 门禁 | 2026-07-21 结果 | 说明 |
|------|----------------|------|
| Go 单元测试 | 通过 | `go test ./...` |
| Python lint/format/unit | 通过 | Ruff、format check、10 项非集成 pytest |
| Shell 语法 | 通过 | `start.sh` 与 `scripts/*.sh` |
| `make check` | 通过 | Go fmt/vet/test、Python locked lint/format/10 项单测、Shell、48 个 Markdown 文件链接 |
| `go test -race ./...` | 通过 | 最终工作区全包竞态检测通过 |
| MySQL/Redis integration | 通过 | 临时 `*_test` 库完成 009→010 后 Go 集成通过；Redis 2 项真实集成通过；临时数据库和用户已清理 |
| Compose config/up/ready/reset | 通过 | 使用本机 Docker Compose；通过 `make dev-config`、`make dev-up`、`make dev-ready`、`make dev-reset`；MinIO/Qdrant/uv digest 已写入 Compose/Dockerfile |
| API 冒烟 | 通过 | `bash scripts/test_api.sh`：34 PASS / 0 FAIL；`bash scripts/test_chat.sh`：全部通过 |
| 双浏览器 | 暂缓 | 用户确认后再做；清单保留在 `docs/design/im.md`，不阻塞当前后端终端成熟度推进 |
| 文档链接/工作区检查 | 通过 | 48 个 Markdown 文件本地链接通过；Python 缓存已忽略并清理，无意外生成文件 |

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
- `TASK.md`、`docs/plan.md`、`docs/roadmap.md` 统一标记为“Phase 0 本地门禁完成、远端 CI 待确认 / Phase 1 后端加固中”。
- 本地 Markdown 链接检查（48 个文件）与 `git diff --check` 已通过；本轮修改尚未形成提交，推送后由远端 CI 提供下一层证据。
- 浏览器和前端手工验收继续按用户确认暂缓，不改变当前后端终端验收结论。
