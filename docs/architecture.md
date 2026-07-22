# ShareO v2 架构文档

> 更新时间: 2026-07-22 | 状态: Phase 0 本地完成 / Phase 1 后端候选 / Phase 2 真实 E2E 门禁阻塞

## 1. 演进目标

v1 是 Go 单体的摄影社区（Feed/帖子/评论/关注/话题/通知/审核流）。v2 在**不重写**的前提下增加四条能力线：

1. **IM**：私聊 + 群组（WebSocket 实时消息）
2. **语义搜图**：发布图片异步向量化，自然语言搜索相关图片
3. **Bot + RAG**：Bot 作为特殊用户接入 IM，基于站内内容检索增强回答
4. **Agent（远期）**：Bot 大脑升级为可调用站内工具的 Agent

## 2. 总体架构

```
                ┌──────────────────────────────────┐
                │        Go 主服务 (Gin, 现有单体扩展)   │
   浏览器 ◄──────┤  业务 API + 模板页面 + WebSocket Hub  │◄────── MySQL (唯一写者: Go)
     ▲          │  IM 模块 (私聊/群组/Bot 触发)          │◄────── Redis (缓存/在线状态/Streams)
     │          └──────┬──────────────────▲────────┘◄────── MinIO
     │      事件/任务    │ (Redis Streams)   │ 内部 HTTP 回调
     │          ┌──────▼──────────────────┴────────┐
     └─ Bot 回复 │      ai-service (Python/FastAPI)   │◄────── Qdrant (唯一写者: Python)
       经 WS 下发 │  API 进程: 文本编码 / 搜索 / RAG 问答  │◄────── MinIO (只读拉图)
                │  Worker 进程: 图片向量化 / Bot 任务    │◄────── LLM API (DeepSeek 等)
                └──────────────────────────────────┘
```

## 3. 组件与职责

| 组件 | 职责 | 边界约束 |
|------|------|----------|
| Go 主服务 | 全部业务逻辑、页面、WebSocket、事件生产 | MySQL 的唯一写者（含聊天消息、Bot 回复落库） |
| ai-service API 进程 | 文本编码、向量搜索、RAG 问答 | 不对公网暴露，仅内网被 Go 调用 |
| ai-service Worker 进程 | 消费 Streams：图片/文本向量化、Bot 任务 | 与 API 进程同一代码库；Phase 2 首轮通过 Go 图片代理读图 |
| MySQL | 业务数据 | 只有 Go 访问 |
| Redis | 缓存、限流、在线状态、**Redis Streams 队列** | 双方共用 |
| MinIO | 图片对象存储 | Go 读写；Phase 2 Python 通过 Go 图片代理读取，不持有 MinIO 凭证 |
| Qdrant | 向量库：`images`（图片向量）、`post_chunks`（文本块向量） | 只有 Python 访问 |
| LLM API | Bot 对话生成 | 经 ai-service 统一封装，provider 可切换 |

## 4. 三条关键数据流

本地开发的 MinIO 运行约定与 Compose 存储隔离见 [本地存储说明](operations/local-storage.md)。Go 通过 `/api/v1/images/...` 提供图片代理；AI worker 首轮通过该代理读取图片，不新增 MinIO 凭证。

### 4.1 帖子索引管线（向量化）

```
帖子审核通过 ──► Go: XADD shareo:stream:index_post {action: upsert, post_id}
删帖/驳回   ──► Go: XADD shareo:stream:index_post {action: delete, post_id}

Worker 消费 (consumer group: ai-workers):
  upsert: 调 Go 内部接口取索引载荷(文本+图片 object_key 列表)
          → Go 图片代理拉 medium 图 → Chinese-CLIP 编码 → upsert Qdrant `images`
          → 正文分块 → BGE 编码 → upsert Qdrant `post_chunks`
  delete: 按 post_id 过滤删除两个 collection 中的全部向量
```

- **触发点选在审核通过而非上传**：pending/rejected 的内容不允许被搜到。
- **幂等**：向量点 ID 使用 image_id，重复消费先按 post_id 清理再 upsert，安全可重放。
- **回填**：提供管理命令遍历存量 approved 帖子批量投递 upsert 事件。

### 4.2 自然语言搜图

```
用户 query ──► Go /api/v1/search/images ──► ai-service /v1/search/images
  (Python: query 编码 → Qdrant KNN → 返回 [{post_id, image_id, score}])
──► Go 按 ID 从 MySQL 补全帖子并二次过滤可见性(approved 且未删) ──► 渲染结果页
```

可见性双保险：事件驱动清理向量 + 查询侧兜底过滤。

### 4.3 Bot 对话（RAG）

```
用户私聊 Bot 或群内 @Bot
──► Go: 消息正常落库下发 → 识别 Bot 触发 → XADD shareo:stream:bot_tasks {conversation_id, message_id}
──► Worker: 调 Go 内部接口取会话上下文 → RAG(检索 post_chunks → 拼 prompt → LLM)
──► 回调 Go POST /internal/bot/reply {conversation_id, content, citations}
──► Go 以 Bot 账号落库 + WebSocket Hub 下发
```

- Bot 是 `users` 表中 `is_bot=true` 的特殊用户，天然复用 IM 全部管线。
- 降级：ai-service 不可用时主站与 IM 完全不受影响；Phase 3 要求 Bot 任务最终返回固定可理解兜底消息，并通过 source message 幂等。

## 5. 技术栈与决策索引

| 领域 | 选型 | 决策记录 |
|------|------|----------|
| 总体架构 | Go 单体扩展 + Python AI 服务，内部 HTTP/JSON | [ADR-001](adr/ADR-001-overall-architecture.md) |
| 任务队列 | Redis Streams + consumer group | [ADR-002](adr/ADR-002-task-queue.md) |
| 向量数据库 | Qdrant（单容器） | [ADR-003](adr/ADR-003-vector-db.md) |
| 模型推理 | 本地 embedding + 云 LLM，4060 为部署/对比实验平台 | [ADR-004](adr/ADR-004-inference-strategy.md) |
| RAG 实现 | 自研 pipeline，不用 LangChain | [ADR-005](adr/ADR-005-rag-no-framework.md) |
| IM | REST 写消息 + gorilla/websocket 下行 + MySQL 事务 + Redis presence | [Phase 1 设计文档](design/im.md) |
| 浏览器写安全 | Go 1.25 `http.CrossOriginProtection` + 精确 trusted origins | 标准库优先，不维护 Token 库 |
| 前端 | 维持 Go Templates + Alpine.js | 范围控制，不引 SPA |

## 6. 当前结构与目标结构

当前代码已经包含 `internal/ws`、`internal/pkg/queue`、Phase 2 Go handler、`ai-service/app/core`、worker 和运维命令；尚未建立 RAG、Agent、独立 aiclient 包和搜索页面。下列目录是完成 Phase 4 后的目标结构，不代表当前全部存在。

```
ShareO/
├── cmd/server/                    # Go 主服务入口（现有）
├── internal/                      # 沿用现有按层组织，新功能按层追加
│   ├── handler/                   # + chat_handler.go, search_handler.go, internal_handler.go
│   ├── service/                   # + chat_service.go, bot_service.go, search_service.go
│   ├── repository/                # chat_repo.go + presence.go
│   ├── model/                     # chat.go（Conversation/Member/Message）
│   ├── ws/                        # WebSocket Hub（按用户管理多连接、扇出、吊销）
│   └── pkg/
│       ├── queue/                 # 新: Redis Streams 生产者
│       └── aiclient/              # 新: 调 ai-service 的 HTTP 客户端
├── ai-service/                    # 新: Python AI 服务（uv 管理）
│   ├── pyproject.toml
│   ├── app/
│   │   ├── config.py
│   │   ├── main.py                # /healthz 存活；/readyz 依赖就绪
│   │   ├── api/                   # Phase 2+ 的 /v1/search/images, /v1/rag/answer
│   │   ├── core/                  # embedding 模型封装、LLM provider、Qdrant 客户端
│   │   ├── rag/                   # chunk / retrieve / rerank / prompt
│   │   └── workers/               # Streams 消费者入口（Worker 进程）
│   ├── tests/
│   ├── Dockerfile
│   └── uv.lock
├── deploy/
│   └── docker-compose.yml         # MySQL + Redis + MinIO + Qdrant + AI API/Worker
├── migrations/                    # 009 chat；010 chat 前向约束加固
├── docs/                          # 见 docs/README.md
└── web/                           # + templates/chat/, templates/search/
```

## 7. 服务间约定

- **内部认证**：Go ↔ ai-service 互调带 `X-Internal-Token`（Go 使用 `SHAREO_INTERNAL_TOKEN`，Python 使用同值的 `SHAREO_AI_INTERNAL_TOKEN`）；ai-service 监听内网地址。
- **Streams 命名**：`shareo:stream:index_post`、`shareo:stream:bot_tasks`；消费组统一 `ai-workers`。
- **可靠性约定**：消费成功才 XACK；每 10 秒扫描并重领 idle 30 秒的 pending；首次处理后最多重试 3 次（共 4 次），最终记录 stream/message/fields/exception/attempts 后 XACK。遵守 ADR-002，不设死信队列。
- **配置**：沿用 `SHAREO_*` 环境变量注入敏感值的现有惯例，ai-service 侧用 `SHAREO_AI_*` 前缀。
- **IM 安全**：WS 与 HTTP 共用 JWT+Redis 登录缓存；Origin 仅同源或精确 trusted origin；浏览器写请求受标准库 CrossOriginProtection 保护。

## 8. 设计原则

1. **增量演进，不重写**：v1 的分层与惯例是资产，新功能按既有模式追加。
2. **单写者**：每个存储只有一个服务写入（MySQL→Go，Qdrant→Python），杜绝双写一致性问题。
3. **事件解耦**：耗时的 AI 处理一律异步化，主站路径不等待 AI。
4. **接口即未来工具**：搜图、搜帖、发帖等接口按"将来会被 Agent 当工具调用"的标准设计（清晰入参出参、无副作用歧义）。
5. **一切选型有 ADR**：见 [docs/README.md](README.md) 的三条硬规矩。
