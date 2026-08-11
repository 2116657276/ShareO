# ShareO 架构

> 更新时间：2026-08-11 | 本文维护稳定架构和证据边界；当前状态见 [`docs/REVIEW.md`](REVIEW.md)

## 服务拓扑

```text
浏览器
  │ HTTP / WebSocket
  ▼
Go 模块化单体 ───────► PostgreSQL 17（15 表，业务真相）
  ├─────────────────► Redis（缓存、登录、在线状态、2 个 Streams）
  ├─────────────────► MinIO（图片对象）
  └─内部 HTTP/token─► Python FastAPI 单进程
                        ├─API + index_post consumer + bot_tasks consumer
                        ├─Chinese-CLIP / FastEmbed
                        ├─RAG pipeline + LangGraph 只读 Agent（已实现）
                        ├────────────────► PostgreSQL `ai` schema / pgvector（图片与正文向量）
                        └────────────────► 配置的 OpenAI-compatible LLM Provider
```

本机默认入口使用 Homebrew PostgreSQL 17、Redis、MinIO 和宿主机 Python；旧 Compose、MySQL、Qdrant、Colima 和 OrbStack 路径均已退役。Uvicorn 固定单 worker，consumer 在 FastAPI lifespan 内启动，并与 API 共享 pgvector 连接池和模型。

## 数据与服务所有权

| 数据 | 唯一写者 | 读取者 | 恢复方式 |
|---|---|---|---|
| 业务表、消息、Bot 回复 | Go（运行时） | Go；Python 经内部 API 读取载荷 | PostgreSQL 事务与外键 |
| 图片对象 | Go | Go；Python 经 Go 代理读取 | MinIO volume 与对象键 |
| 登录、缓存、在线状态 | Go | Go | TTL、回源 PostgreSQL |
| `index_post`、`bot_tasks` | Go 生产、Python 消费 | Go/Python | 四次处理、重领、最终 ACK |
| `bot_task_outbox` | Go 事务写入、Go publisher 标记 | Go | Redis 故障/重启后按退避补发 |
| `ai.image_embeddings`、`ai.post_chunk_embeddings` | Python | Python；Go 获取搜索结果 | 幂等事件、回填、对账 |
| LLM 回答候选 | Python | Go 回调接收 | 重试或固定兜底 |

PostgreSQL 是业务真相；初始化脚本只建立 schema 并写入固定 Bot 引导记录，运行时业务写入由 Go 独占。`ai` schema 中的 pgvector 表是可重建派生数据，Redis Stream 不承担长期业务事实。

## 15 表数据域

- 社区：`users`、`posts`、`post_images`、`comments`、`likes`、`favorites`、`follows`、`notifications`、`system_logs`。
- 评论：`comment_likes`。
- 私聊：`conversations`、`conversation_members`、`messages`。
- Bot：`bot_replies`、`bot_task_outbox`。

详细约束见 [数据模型参考](reference/data-model.md)。

## 异步链路

### 图文索引

```text
审核/编辑/驳回/删除 → index_post
  → Go 索引载荷与图片代理
  → Chinese-CLIP 写 images
  → FastEmbed 写 post_chunks
```

### Bot 回复

```text
用户私聊 shareo_bot → 消息与 bot_task_outbox 同事务
  → publisher 按退避发布 bot_tasks
  → Go 返回严格上下文 → RAG 检索/配置的 LLM Provider
  → Go 二次验证引用并事务写 Bot 消息 → WebSocket
```

## Readiness

| 地址 | 语义 |
|---|---|
| Go `/healthz` | Go 进程存活 |
| AI `/healthz` | AI 进程存活 |
| AI `/readyz` | Redis 和 PostgreSQL/pgvector 基础连接 |
| AI `/readyz/image-search` | 图片模型、`images` 和 `index_post` consumer 可用 |
| AI `/readyz/rag` | 文本模型、`post_chunks`、consumer 和 LLM 配置可用 |
| AI `/readyz/agent` | Agent 工具、状态图、模型和 LLM 配置可用 |

Liveness 不代表模型或业务能力可用；自动化和运维不得用 `/healthz` 替代 capability readiness。

## 降级矩阵

| 故障 | 社区/全文搜索 | 普通私聊 | 语义搜图 | Bot |
|---|---|---|---|---|
| AI 停止 | 正常 | 正常 | 503 | 新任务等待/最终兜底 |
| PostgreSQL/pgvector 停止 | 正常 | 正常 | 503 | RAG 不可用 |
| MinIO 停止 | 文字路径正常，图片代理失败 | 正常 | 新索引失败，搜索结果由 Go 过滤 | 正文 RAG 仍可用 |
| Redis 停止 | 缓存降级，写业务不回滚 | 消息落库可用，在线/推送受影响 | 旧索引可查，新事件延迟 | 新任务延迟 |
| LLM Provider 不可用 | 正常 | 正常 | 正常 | 重试后固定兜底 |

当前本机运行保留了依赖边界和 readiness 语义；完整证据边界见 [工程审查](REVIEW.md)。运行时只支持宿主机服务，旧容器配置仅作为历史参考保留，不再由 Make 目标启动。

### 已实现的恢复语义

| 场景 | 实测结果 | 恢复结果 |
|---|---|---|
| Python AI 服务停止 | 社区和全文搜索 HTTP 200，语义搜图 HTTP 503 | AI 服务启动后两个 capability readiness 恢复 HTTP 200 |
| PostgreSQL/pgvector 停止 | 社区和全文搜索 HTTP 200，语义搜图/RAG HTTP 503 | PostgreSQL/pgvector 恢复后 readiness 恢复 |
| MinIO 停止 | 全文搜索 HTTP 200，图片代理失败且不伪造成功 | MinIO 启动后图片代理 HTTP 200 |
| Redis 停止 | 社区、全文搜索和普通消息 HTTP 200；推送/异步能力进入降级 | Redis 启动后 consumer 自动重建缺失消费组并恢复 readiness |
| 测试 LLM Provider 停止 | 普通私聊 HTTP 200，Bot 返回固定兜底 | Provider 启动后 RAG readiness 恢复 |

只读 Agent 复用上述依赖边界；当前报告的机器门禁和真实 Provider 结果统一见 [Final Freeze 证据归档](evidence/final-freeze/README.md)。源码冷启动和浏览器演示属于已接受的证据边界，不作为当前整改任务。

## 模块边界

Go 保持 Handler → Service → Repository 分层，AI 桥接只负责事件和内部 HTTP。Python 代码显式分为 image search、RAG、隔离的 `agent/` 模块和 worker runtime。默认 RAG Bot 的控制流由代码固定；显式 Agent 模式才使用规划器和工具调用。Agent 只读调用受保护的 Go 内部帖子接口，不获得 PostgreSQL `public`、MinIO 或 pgvector 写权限。

## 只读 Agent 数据流

```text
用户发送 ai_mode=agent → 消息与 bot_task_outbox 同事务
  → publisher 发布 bot_tasks
  → LangGraph StateGraph
  → 按问题意图收窄工具面
  → semantic_search / keyword_search / read_posts / search_images
  → Go 可见性复核后的观察结果
  → 继续调用或最终总结
  → 结构化引用 + 脱敏 agent_trace
  → Go 事务写 Bot 消息 → WebSocket
```

默认 `ai_mode=rag` 不进入该路径。Agent 不保存 checkpoint，不暴露思维链，不执行写工具；Redis Streams、PostgreSQL `bot_replies` 和消息 `meta` 负责现有任务耐久性与幂等。
