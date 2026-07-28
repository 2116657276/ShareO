# ShareO 单上下文

## 项目定位

ShareO 是一个用于简历展示和毕业答辩的 Go + Python 全栈项目，主线能力是图文社区、一对一实时私聊、中文语义搜图、带可访问引用的 RAG Bot，以及同一私聊入口中的受限只读知识 Agent。

项目强调数据所有权、异步幂等、可见性校验、受控降级、可审计工具调用和冻结数据集评测。不宣称通用 Agent、生产级高可用、线上 SLA、容量上限或公共克隆可直接复现真实图片质量评测。

## 最终范围

保留认证、资料、图文帖子、上传、Feed、全文搜索、管理员审核、点赞、关注、评论、通知、一对一私聊、WebSocket、中文语义搜图、正文 RAG 和私聊 Bot。

只读 Agent 作为私聊内的受限例外：默认消息仍走固定 RAG，用户显式开启深度分析后，Agent 只能使用语义检索、关键词检索、帖子读取和图片检索四个只读工具。

不实现收藏、转帖、话题、群聊、写入型 Agent、外部工具、长期记忆、多 Agent、独立 Bot 页面、音视频和生产级多租户平台。

## 架构边界

- Go 是运行时业务数据和 MySQL 的唯一写者，负责认证、权限、审核、社区、聊天和 Bot 回复。
- Python AI 服务是 Qdrant 的唯一写者，负责 embedding、向量检索、RAG、只读 Agent 和两个 Redis Streams consumer。
- Python AI API、图片索引 consumer 和 Bot consumer 共用单一 FastAPI 进程和单 Uvicorn worker。
- MinIO 凭证只由 Go 持有；Python 通过 Go 图片代理读取图片。
- Redis Streams 使用 at-least-once 语义，依靠幂等 ID、重试和重领处理重复投递。
- 所有公开帖子和 Bot/Agent 引用必须满足 `approved AND is_deleted=0`，并由 Go 进行最终可见性复核。
- Go/Python 日常开发使用宿主机；MySQL、Redis、MinIO 使用 Homebrew，Qdrant 从指定 `qdrant/` 目录启动。Docker/Compose 是用户人工验收通过后的最终打包和发布复核方式。

## 接口边界

- 公开语义搜图：`GET /api/v1/search/images`。
- AI 内部语义搜图：`POST /v1/search/images`。
- AI 内部 RAG：`POST /v1/rag/answer`。
- Agent 通过既有聊天消息的 `ai_mode=agent` 触发，不新增公开 Agent HTTP API。
- Go↔AI 内部接口使用 `X-Internal-Token`；内部 Token、LLM Key、MinIO 凭证不进入浏览器、日志、报告或 Git。

## 文档交接规则

后续开发开始前读取本文件、`AGENTS.md`、[`TASK.md`](TASK.md)、[`docs/plan.md`](docs/plan.md)、[`docs/standards.md`](docs/standards.md)、相关 ADR 和目标阶段文档。

- 实时状态、阻塞和下一项任务：[`TASK.md`](TASK.md)。
- 执行顺序、门禁和暂停条件：[`docs/plan.md`](docs/plan.md)。
- 架构、功能、接口和评测文档只维护稳定事实、设计约束和历史证据，不复制实时状态。
- 代码、数据库 schema 和配置模板分别是运行行为、数据结构和配置细节的最终事实源。
- 不把人工浏览器验收、已有镜像启动或 API 冒烟写成源码冷启动或最终发布完成。
