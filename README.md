# ShareO — Go 社区与 AI 检索练手项目

ShareO 是一个面向简历展示的小型全栈项目，核心能力是图文社区、实时私聊、中文语义搜图和带帖子引用的 RAG Bot。项目强调清晰模块边界、可运行演示和可解释的 AI 链路，不追求生产级平台复杂度。

> 当前状态：阶段 6 私聊 Bot 技术链路和 mock LLM E2E 已完成；真实 provider 演示、评测数据、Demo seed 和最终清理属于阶段 7。范围和实施顺序以 [TASK.md](TASK.md) 与 [docs/plan.md](docs/plan.md) 为准。

## 核心功能

- 用户注册登录、图文发布、Feed、全文搜索和管理员审核。
- 点赞、关注、评论及对应通知。
- WebSocket 私聊、消息恢复和未读计数。
- Chinese-CLIP + Qdrant 中文语义搜图。
- FastEmbed + Qdrant + OpenAI-compatible LLM 的引用式 RAG Bot。

收藏、转帖、话题、群聊和 Agent 不属于最终范围。

## 技术栈

| 模块 | 技术 |
|------|------|
| Web 主服务 | Go、Gin、GORM、Go Templates、Alpine.js |
| 业务数据 | MySQL |
| 缓存与任务 | Redis、Redis Streams |
| 图片存储 | MinIO |
| AI 服务 | Python、FastAPI、Chinese-CLIP、FastEmbed |
| 向量检索 | Qdrant |
| 实时通信 | gorilla/websocket |

## 开发命令

当前可运行 `make check` 验证 Go、Python、Shell 和文档。重构完成后的统一入口为：

```bash
make up
make demo-seed
make test-ai-e2e
make eval-ai
make down
```

其中 `make test-ai-e2e` 使用临时 mock LLM 验证跨服务链路；`make demo-seed` 和 `make eval-ai` 在阶段 7 完成真实演示数据与 provider 评测后启用。

配置模板见 [config.yaml.example](config.yaml.example)，架构说明见 [docs/architecture.md](docs/architecture.md)。

## 简历展示主线

1. 用户发布图文，管理员审核通过。
2. Redis Stream 异步驱动图片和正文向量索引。
3. 用户通过中文描述搜索图片。
4. 用户私聊 `shareo_bot`，Bot 基于 approved 帖子回答并返回可访问引用。
