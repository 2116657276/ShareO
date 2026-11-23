# ShareO 架构

> 运行入口更新于 2026-11-23。当前状态见 [`TASK.md`](../TASK.md)，验收与评测证据见 [工程审查](REVIEW.md)。

## 服务拓扑

```text
浏览器
  │ HTTP / WebSocket
  ▼
  Go 模块化单体 ───────► PostgreSQL 17（16 张业务表，业务真相）
  ├─────────────────► Redis（缓存、当前会话校验、在线状态、2 个 Streams）
  ├─────────────────► MinIO（图片对象）
  └─内部 HTTP/token─► Python FastAPI 单进程
                        ├─API + index_post consumer + bot_tasks consumer
                        ├─Chinese-CLIP / FastEmbed
                        ├─RAG pipeline + LangGraph 只读 Agent（已实现）
                        ├────────────────► PostgreSQL `ai` schema / pgvector（图片与正文向量）
                        └────────────────► 配置的 OpenAI-compatible LLM Provider
```

默认开发入口使用 Homebrew PostgreSQL 17、Redis、MinIO 和宿主机 Python。[OrbStack 隔离试跑](operations/runbook.md#orbstack-隔离试跑) 在内部网络复用相同的服务拓扑，只向宿主机发布 Go 端口。两种入口均使用单 Uvicorn worker，consumer 在 FastAPI lifespan 内启动，并与 API 共享 pgvector 连接池和模型。

## 数据与服务所有权

| 数据 | 唯一写者 | 读取者 | 恢复方式 |
|---|---|---|---|
| 业务表、消息、Bot 回复 | Go（运行时） | Go；Python 经内部 API 读取载荷 | PostgreSQL 事务与外键 |
| 图片对象 | Go | Go；Python 经 Go 代理读取 | MinIO volume 与对象键 |
| 登录令牌、缓存、在线状态 | Go | Go | Feed/在线状态使用 TTL；受保护请求当前必须先通过 Redis 令牌校验，再用 PostgreSQL `session_version` 做持久化撤销判断 |
| `index_post`、`bot_tasks` | Go 生产、Python 消费 | Go/Python | 四次处理、重领、最终 ACK |
| `bot_task_outbox` | Go 事务写入、Go publisher 标记 | Go | Redis 故障/重启后按退避补发 |
| `ai.image_embeddings`、`ai.post_chunk_embeddings` | Python | Python；Go 获取搜索结果 | 幂等事件、回填、对账 |
| LLM 回答候选 | Python | Go 回调接收 | 重试或固定兜底 |

PostgreSQL 是业务真相；初始化脚本只建立 schema 并写入固定 Bot 引导记录，运行时业务写入由 Go 独占。`ai` schema 中的 pgvector 表是可重建派生数据，Redis Stream 不承担长期业务事实。

## 16 张表数据域

- 社区：`users`、`posts`、`post_images`、`comments`、`likes`、`favorites`、`follows`、`notifications`、`system_logs`。
- 评论：`comment_likes`。
- 私聊：`conversations`、`conversation_members`、`messages`。
- Bot：`bot_replies`、`bot_task_outbox`。
- 上传生命周期：`upload_artifacts`（owner、批次、过期和帖子绑定）。

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
| AI `/readyz/rag` | 文本模型已加载、`post_chunks` 可访问、`index_post` consumer 运行且 LLM 已配置；当前不检查 `bot_tasks` consumer |
| AI `/readyz/agent` | Agent 开关、LLM 配置和文本向量存储元数据可用；当前不检查两个 Streams consumer，也不保证模型已经完成加载 |

Liveness 不代表模型或业务能力可用；自动化和运维不得用 `/healthz` 替代 capability readiness。

当前实现的就绪边界需要特别注意：`/readyz/rag` 只观察 `shareo:stream:index_post`，`/readyz/agent` 目前不观察 `shareo:stream:index_post` 或 `shareo:stream:bot_tasks`。因此 readiness 返回 200 不能证明 Bot 任务 consumer 正常；该误报由 `TASK.md` 的 `AI-01` 跟踪，修复后应同步更新本表和运行手册。

## 降级矩阵

| 故障 | 社区/全文搜索 | 普通私聊 | 语义搜图 | Bot |
|---|---|---|---|---|
| AI 停止 | 正常 | 正常 | 503 | 新任务等待/最终兜底 |
| PostgreSQL 业务库停止 | 依赖数据库的读取和写入失败 | 不可用 | 不可用 | 消息和 outbox 无法持久化 |
| `ai` schema/pgvector 不可用（业务库仍正常） | 社区和关键词搜贴正常 | 正常 | 503 | RAG/Agent 不可用或进入兜底 |
| MinIO 停止 | 文字路径正常，图片代理失败 | 正常 | 新索引失败，搜索结果由 Go 过滤 | 正文 RAG 仍可用 |
| Redis 停止 | 匿名公开读在 PostgreSQL 正常时可用；缓存和限流受影响 | 受保护 REST/WS 会话校验 fail-closed；REST 通常返回 503，WS 握手当前按 401 处理；不能据此声称消息落库可用 | 已有索引可查，新事件发布延迟 | outbox 发布和消费延迟 |
| LLM Provider 不可用 | 正常 | 正常 | 正常 | 重试后固定兜底 |

当前本机运行保留了依赖边界和 readiness 语义；完整证据边界见 [工程审查](REVIEW.md)。运行时只支持宿主机服务，旧容器配置仅作为历史参考保留，不再由 Make 目标启动。

### 当前代码约定与已有恢复证据

下表汇总当前代码约定和已有运行记录；未在本轮重新执行的场景不视为新的故障注入或发布级证明。

| 场景 | 观察/约定 | 恢复结果 |
|---|---|---|
| Python AI 服务停止 | 社区和关键词搜贴可继续；语义搜图 HTTP 503 | AI 服务启动并完成对应模型/consumer readiness 后恢复；Bot pending 任务继续处理 |
| PostgreSQL 业务库停止 | Feed、帖子、私聊和写入接口按依赖失败，不应记录为 HTTP 200 | PostgreSQL 恢复并重新建立连接后恢复业务请求 |
| `ai` schema/pgvector 停止 | 社区和关键词搜贴可继续；语义搜图/RAG/Agent 受控不可用 | AI 数据库恢复后对应 readiness 恢复 |
| MinIO 停止 | 全文搜索 HTTP 200，图片代理失败且不伪造成功 | MinIO 启动后图片代理 HTTP 200 |
| Redis 停止 | 匿名公开读在业务库正常时可继续；受保护 REST/WS 会话校验 fail-closed，不能记录为普通消息 HTTP 200 | Redis 恢复后登录校验、Streams consumer 和 outbox 发布恢复；客户端需重新建立 WebSocket 并用 `after_id` 补偿 |
| 测试 LLM Provider 停止 | 普通私聊 HTTP 200，Bot 返回固定兜底 | Provider 启动后 RAG readiness 恢复 |

只读 Agent 复用上述依赖边界；当前报告的机器门禁和真实 Provider 结果统一见 [Final Freeze 证据归档](evidence/final-freeze/README.md)。源码冷启动和浏览器演示属于已接受的证据边界，不作为当前整改任务。

## 模块边界

Go 保持 Handler → Service → Repository 分层，AI 桥接只负责事件和内部 HTTP。Python 代码显式分为 image search、RAG、隔离的 `agent/` 模块和 worker runtime。默认 RAG Bot 的控制流由代码固定；显式 Agent 模式才使用规划器和工具调用。Agent 只读调用受保护的 Go 内部帖子接口，不获得 PostgreSQL `public`、MinIO 或 pgvector 写权限。

## 只读 Agent 数据流

ShareO 的默认在线推理是固定 RAG，而不是自由规划：查询改写、检索、引用白名单、回答生成和故障兜底均由代码定义，便于评测与降级。`ai_mode=agent` 只在用户显式选择“深度分析”时启用；它复用相同的只读检索能力，不获得写工具、数据库连接、MinIO 凭证或任意网络访问。

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

无论 RAG 还是 Agent，Python 返回的帖子、图片和引用都只是候选。Go 在公开展示和 Bot 消息落库前按当前业务事实重新检查 `approved AND is_deleted=0`、用户可见性和引用归属；候选为空、越权、格式错误、依赖不可用或引用复核失败时，进入固定兜底或明确 503，不把未验证内容伪装成成功结果。这个 Go 最终门禁和故障降级语义是项目的稳定边界。
