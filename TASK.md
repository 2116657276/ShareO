# TASK.md — 当前任务

> 更新时间：2026-07-23 | 当前阶段：轻量化重构阶段 7/7

## 最终范围

- [x] 保留认证、帖子、上传、Feed、全文搜索和管理员审核
- [x] 保留点赞、关注、评论、通知
- [x] 保留私聊、WebSocket、语义搜图和 RAG Bot
- [x] 删除收藏、转帖、话题、群聊和 Agent
- [x] 数据库采用可清空的全新 12 表基线

## 实施进度

- [x] 阶段 1：文档范围冻结
- [x] 阶段 2：数据库与业务裁剪
- [x] 阶段 3：AI 单进程与统一 Compose
- [x] 阶段 4：语义搜图修复
- [x] 阶段 5：文本索引与 RAG
- [x] 阶段 6：私聊 Bot（6A 核心链路、6B 页面与 mock E2E）
- [ ] 阶段 7：Demo、评测和最终清理

## 当前风险

- 语义搜图工程闭环已通过，12 条冻结查询及 Recall@5 门禁留到阶段 7 与 Demo 数据一起执行。
- 私聊 Bot 技术链路已完成并通过 mock LLM 跨服务 E2E；真实 provider、质量评测、Demo seed 和最终文件清理仍属于阶段 7。
- 云端 LLM Key 是自然语言回答的必要配置；真实 provider 演示与 15 条质量评测留到阶段 7。

## 阶段评审

每个阶段完成后运行对应测试并在本节追加一条简短结论，不再创建单独的大型评审文档。

- 2026-07-22 阶段 1 通过：入口、架构、功能、Phase 0–3、评测和 ADR 已统一为轻量范围；Markdown 链接与 diff 检查通过。
- 2026-07-22 阶段 2 通过：本地 `shareo` 已用新基线重建并确认仅有 12 张表、0 trigger、0 view；收藏、转帖、话题和群聊的模型、查询、路由与页面入口已删除。评审发现并修复了不可见帖子仍可点赞/评论、点赞列表总数包含不可见帖子、计数同步失败不回滚三个 P2 问题；`make check`、Go race、真实 MySQL 基线与私聊集成测试均通过。
- 2026-07-22 阶段 3 通过：FastAPI lifespan 统一启动 API、图片索引和 Bot 任务消费者，并共享 Chinese-CLIP 与 Qdrant 实例；根目录 Compose 成为唯一运行方式，旧 Homebrew、独立 worker 和示例启动文件已删除。评审修复了 Linux 镜像误装 CUDA 依赖、Redis 阻塞读取超时和 MySQL 冷启动竞态；`make check`、Compose 配置检查及清空卷后的六服务首次冷启动均通过，两个 Stream 各保持单消费者且无 pending。
- 2026-07-22 阶段 4 通过：Chinese-CLIP 后台预热与图片消费者纳入 `/readyz/image-search`，图片编码移入线程，载荷获取、下载、模型加载、编码及 Qdrant 删除/写入均记录分段耗时。真实 Compose E2E 发现并修复了新版 Transformers 返回 `BaseModelOutputWithPooling` 导致搜索 503 的 P1 兼容问题，以及重复事件检查提前成功、应用日志未输出两个 P2 问题；`make check` 和真实模型的审核→索引→命中→重复投递→重启重领→删除隔离链路通过。
- 2026-07-22 阶段 5 通过：同一 `index_post` 已同时维护 `images` 与 `post_chunks`，FastEmbed 中文模型、400/80 分块、UUIDv5 point ID、双 payload index、检索去重、OpenAI-compatible provider、结构化回答和引用白名单已实现。评审修复了文本模型失败可能饿死图片索引、缺少 LLM Key 时空检索可能绕过降级、文档检查误扫 `.venv` 三个 P2 问题；55 个非集成测试、真实 FastEmbed 512 维编码、真实 Qdrant round-trip 及 Compose 图文双索引 E2E 均通过，缺少 Key 时仅 RAG 返回 503。
- 2026-07-23 阶段 6A 完成：Go 私聊消息提交后异步发布 `bot_tasks`，Python Worker 获取严格校验的上下文，RAG 生成结果后由 Go 在同一事务中写入 Bot 消息和 `bot_replies`；重复任务、重复回调和 Worker 重试均由唯一约束吸收，引用经过 Python chunk 白名单和 Go approved/未删除二次校验。阶段提交为 `17b54ec`（`feat: connect private chat bot pipeline`）。
- 2026-07-23 阶段 6B 完成：聊天页增加 AI 助手入口和引用卡片，新增仅测试使用的 OpenAI-compatible mock provider，完整 E2E 覆盖正文索引、Bot 回复、重复任务、普通私聊、删除来源和 LLM 超时兜底。阶段提交为 `0aff22e`（`feat: add bot chat UI and end-to-end coverage`）。
- 2026-07-23 阶段 6 评审通过：复测 `make check`、真实 MySQL/Redis/Qdrant 集成测试、Python 3 项集成测试、`go test -race ./...` 和 `make test-ai-e2e` 均通过。评审修复了其他机器人可能触发任务、回调 4xx 未统一按永久失败处理、群聊边界和异常任务响应校验等问题；未遗留 P0/P1 或影响幂等、认证、可见性和普通私聊可用性的 P2。修复提交为 `fix: close phase 6 review findings`，第七阶段入口为真实 provider 演示、15 条 RAG/12 条搜图评测、Demo seed 和最终无关文件清理。
