# ShareO 单上下文

> 更新时间：2026-07-26 | 当前主线：Phase 7C 发布与演示收口

## 项目定位

ShareO 是面向简历展示与毕业答辩的轻量 Go + AI 全栈项目。核心演示链路是：发布图文并审核，异步建立图片和正文向量索引，使用中文语义搜图，最后在一对一私聊中由 `shareo_bot` 基于已审核帖子生成带可访问引用的回答。

项目目标是可复现启动、可解释 AI 链路、可量化评测和五分钟稳定演示，不追求生产级多租户、微服务或平台化能力。

## 最终范围

保留认证、资料、图文帖子、上传、Feed、全文搜索、管理员审核、点赞、关注、评论、通知、一对一私聊、WebSocket、中文语义搜图、正文 RAG 和私聊 Bot。

收藏、转帖、话题、群聊、Agent、工具调用和独立 Bot 页面已经从代码与最终路线图移除，不得作为后续阶段恢复。

## 架构约束

- Go 是运行时业务数据的唯一写者，负责业务权限、审核状态、聊天消息和 Bot 回复；初始化脚本只负责建表并写入固定 Bot 引导记录。
- Python 是 Qdrant 唯一写者，负责 embedding、向量检索和受限回答生成。
- Python AI API 与两个 Redis Streams consumer 运行在同一 FastAPI 单进程中，Uvicorn 固定单 worker。
- MinIO 凭证只由 Go 持有；Python 通过 Go 图片代理读取图片。
- Redis Streams 采用 at-least-once：最多四次处理、`XAUTOCLAIM` 重领、最终记录并 ACK，不引入死信队列。
- 所有公网帖子与 Bot 引用都必须由 Go 再次验证 `approved AND is_deleted=0`。
- 默认真实 LLM provider 是 DeepSeek；mock provider 只用于自动化，Ollama 是可选实验。

## 事实来源

1. [TASK.md](TASK.md)：唯一当前任务与阻塞状态。
2. [docs/plan.md](docs/plan.md)：Phase 7A → 7B → 7C 当前执行顺序。
3. [docs/roadmap.md](docs/roadmap.md)：Phase 0–7 里程碑和完成状态。
4. [docs/phases/](docs/phases/README.md)：阶段边界、门禁与提交证据。
5. [docs/adr/](docs/adr/)：已接受及被替代的技术决策。

状态冲突时按以上顺序处理；路由、schema 和配置细节分别以代码、`migrations/001_init.sql` 和 Compose/config 模板为准。

## 当前主线

- Phase 7A：已完成，可重复 Demo seed 和 40/30 评测数据集已冻结。
- Phase 7B：已完成真实 DeepSeek 质量评测；Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、人工平均分 4.8667。
- Phase 7C：当前主线，进行完整门禁、故障降级验证和五分钟演示收口；不得将 7C 的工作提前宣称完成。

Phase 4 的工程闭环和质量门禁均已完成；最终发布仍依赖 Phase 7C 的故障、演示和全量证据收口。

## 开发模型交接约束

后续开发模型启动时先读取本文件、`AGENTS.md`、`TASK.md`、`docs/plan.md` 和目标阶段文档，再开始修改代码。当前唯一允许推进的主线是 Phase 7C；没有自动测试、真实环境证据和文档同步，不得勾选任务或把阶段标为完成。实现代码时保持 Go 运行时业务写入 MySQL、Python 写入 Qdrant 的单写者边界，不恢复已裁剪能力，也不把浏览器手工验收提前当成后端门禁。每个可完成工作包使用独立提交，并在对应阶段的“提交与环境证据”中记录真实 SHA 和复核命令。
