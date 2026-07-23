# ShareO 已实现架构

> 更新时间：2026-07-23 | 状态：Phase 0–3、5–6 已完成，Phase 4 质量待验，Phase 7 进行中

## 六服务拓扑

```text
浏览器
  │ HTTP / WebSocket
  ▼
Go 模块化单体 ───────► MySQL（12 表，业务真相）
  ├─────────────────► Redis（缓存、登录、在线状态、2 个 Streams）
  ├─────────────────► MinIO（图片对象）
  └─内部 HTTP/token─► Python FastAPI 单进程
                        ├─API + index_post consumer + bot_tasks consumer
                        ├─Chinese-CLIP / FastEmbed
                        ├────────────────► Qdrant（images / post_chunks）
                        └────────────────► DeepSeek/OpenAI-compatible LLM
```

Compose 启动 MySQL、Redis、MinIO、Qdrant、Go app 和 Python ai-service。Uvicorn 固定单 worker，consumer 在 FastAPI lifespan 内启动并与 API 共享模型和连接。

## 数据与服务所有权

| 数据 | 唯一写者 | 读取者 | 恢复方式 |
|---|---|---|---|
| 业务表、消息、Bot 回复 | Go | Go；Python 经内部 API只读载荷 | MySQL事务与外键 |
| 图片对象 | Go | Go；Python 经 Go 代理读取 | MinIO volume与对象键 |
| 登录、缓存、在线状态 | Go | Go | TTL、回源 MySQL |
| `index_post`、`bot_tasks` | Go生产、Python消费 | Go/Python | 四次处理、重领、最终 ACK |
| `images`、`post_chunks` | Python | Python；Go 获取搜索结果 | 幂等事件、回填、对账 |
| LLM 回答候选 | Python | Go 回调接收 | 重试或固定兜底 |

MySQL 是业务真相，Qdrant 是可重建派生数据。Redis Stream不承担长期业务事实。

## 12 表数据域

- 社区：`users`、`posts`、`post_images`、`comments`、`likes`、`follows`、`notifications`、`system_logs`。
- 私聊：`conversations`、`conversation_members`、`messages`。
- Bot 幂等：`bot_replies`。

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
用户私聊 shareo_bot → 用户消息事务完成 → bot_tasks
  → Go 返回严格上下文 → RAG 检索/DeepSeek
  → Go 二次验证引用并事务写 Bot 消息 → WebSocket
```

## Readiness

| 地址 | 语义 |
|---|---|
| Go `/healthz` | Go 进程存活 |
| AI `/healthz` | AI 进程存活 |
| AI `/readyz` | Redis 和 Qdrant 基础连接 |
| AI `/readyz/image-search` | 图片模型、`images` 和 `index_post` consumer 可用 |
| AI `/readyz/rag` | 文本模型、`post_chunks`、consumer 和 LLM配置可用 |

Liveness 不代表模型或业务能力可用；自动化和运维不得用 `/healthz` 替代 capability readiness。

## 降级矩阵

| 故障 | 社区/全文搜索 | 普通私聊 | 语义搜图 | Bot |
|---|---|---|---|---|
| AI 停止 | 正常 | 正常 | 503 | 新任务等待/最终兜底 |
| Qdrant停止 | 正常 | 正常 | 503 | RAG不可用 |
| MinIO停止 | 文字路径正常，图片代理失败 | 正常 | 新索引失败，搜索结果由 Go过滤 | 正文 RAG仍可用 |
| Redis停止 | 缓存降级，写业务不回滚 | 消息落库可用，在线/推送受影响 | 旧索引可查，新事件延迟 | 新任务延迟 |
| DeepSeek不可用 | 正常 | 正常 | 正常 | 重试后固定兜底 |

Phase 7C 必须在真实环境复核此矩阵。

## 模块边界

Go 保持 Handler → Service → Repository 分层，AI桥接只负责事件和内部 HTTP。Python显式分为 image search、RAG 和 worker runtime，不引入 LangChain、LlamaIndex、Agent或多 provider路由。
