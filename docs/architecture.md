# ShareO 轻量架构

> 更新时间：2026-07-22 | 状态：重构中

## 组件

```text
浏览器
  │ HTTP / WebSocket
  ▼
Go 模块化单体 ──► MySQL
  ├─────────────► Redis（缓存、在线状态、Streams）
  ├─────────────► MinIO（图片）
  └─内部 HTTP──► Python AI 单进程
                    ├─后台消费 index_post / bot_tasks
                    ├─Chinese-CLIP / FastEmbed
                    ├────────────► Qdrant
                    └────────────► OpenAI-compatible LLM
```

## 边界

- Go 是 MySQL 唯一写者，负责权限、审核状态、聊天消息和 Bot 回复。
- Python 是 Qdrant 唯一写者，负责图片与正文 embedding、检索和受限回答生成。
- Redis Stream 只承载 `index_post` 与 `bot_tasks`；AI 失败不阻塞业务写入。
- MinIO 凭证只由 Go 持有，AI 通过 Go 图片代理读取图片。
- 所有公网引用由 Go 再次验证 `approved AND is_deleted=0`。

## 模块

Go 保持 handler/service/repository 分层，业务范围分为 community、social、chat 和 AI bridge。Python 单进程内部拆为 `image_search`、`rag`、`shared`，不引入 RAG 或 Agent 编排框架。

## 运行约束

- Uvicorn 固定单 worker，Redis consumer 在 FastAPI lifespan 中启动。
- `/healthz` 仅表示进程存活；`/readyz/image-search` 与 `/readyz/rag` 分别表示能力可用。
- 缺少 LLM Key 时只有 RAG 不可用；Feed、互动、普通私聊和语义搜图继续工作。
