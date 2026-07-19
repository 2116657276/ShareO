# ShareO v2 详细开发计划

> 更新时间: 2026-07-19 | 状态: 生效 | 上游: [roadmap.md](roadmap.md)（里程碑）/ [architecture.md](architecture.md)（架构）
>
> 选型总原则：**优先标准库，其次成熟稳定的第三方库**，不引实验性依赖。每步给出验收标准；串行推进，Phase 内允许微调顺序。

## Phase 0 — 基建 + 修复清单（2026-07 下旬 ~ 08 中旬）

目标：还清技术债，把 v2 需要的底座（编排/AI 服务骨架/队列/检查工具链）立起来。

### 0.1 评审易修项修复（先做，约 2~3 天）

来源：[2026-07-19 评审报告](reviews/2026-07-19-code-review.md)，20 个"易"项按批次执行，**每项独立提交**，提交信息注明 SR 编号：

| 批次 | 项 | 内容摘要 |
|------|-----|---------|
| 第 1 批 (P1) | SR-01 | 搜索 page_size 除零 panic → clamp |
| | SR-02/03 | 用户主页、设置页 nil 解引用 → 判空 |
| | SR-04 | Web 设置清空头像 → 只更新表单提交字段 |
| 第 2 批 (P2) | SR-05 | Web 登录/注册挂限流 |
| | SR-06 | favorite Toggle 删除错误检查 |
| | SR-08 | Repost 校验原帖已过审 |
| | SR-09 | 评论计数同步包事务 |
| | SR-10 | 管理员删不存在帖返回错误 |
| | SR-11 | 图片代理 Any → GET/HEAD |
| 第 3 批 (P3) | SR-13~15, 17~22, 24 | gofmt 全量、JWT 算法白名单、挂载 RedirectIfAuth、限流文档纠偏、注释修正、Email 校验、URL 转义、模板 glob 兜底、HomePage userData、admin 封禁自保护 |
| 死代码 | SR-16 | 删除 TopicRepo/AdminService/NotificationService/upload 中确认无引用的方法（删前 grep 二次确认） |

**验收**: `go build`/`go vet`/`go test` 全绿；`gofmt -l .` 输出为空；逐条用 curl 验证修复行为。

### 0.2 工具链与工程化（计划易实现项）

| 步骤 | 内容 | 技术选型 | 验收 |
|------|------|----------|------|
| 0.2.1 | Makefile 补 `fmt` / `check`（gofmt -l 报错即失败 + vet + test） | make（现有） | `make check` 一键绿 |
| 0.2.2 | 结构化日志：`log.Printf` → `log/slog`（TASK.md 遗留 P3） | **标准库 slog**，JSON handler，请求日志带 user_id/path | 全仓无裸 log.Printf（main 启动横幅除外） |
| 0.2.3 | 密码修改功能（TASK.md 遗留 P3，缓解 admin123 风险） | bcrypt（现有），`PUT /api/v1/auth/password` + 设置页表单，改后强制重登（删登录缓存） | curl + 页面双路径可用 |
| 0.2.4 | CI 工作流文件备好（有 GitHub 远程即启用） | GitHub Actions：go.yml（fmt/vet/test）+ python.yml（ruff/pytest） | 本地 `make check` 等价兜底 |

### 0.3 Docker Compose 依赖编排（TASK.md 遗留 P3）

- `deploy/docker-compose.yml`: `mysql:8.0`、`redis:7-alpine`、`minio/minio:latest`、`qdrant/qdrant:latest`，带 named volume、healthcheck、端口对齐 config.yaml.example。
- 首次启动自动执行 migrations（mysql 容器 `docker-entrypoint-initdb.d` 挂载）。
- README 快速开始改为 compose 优先，`start.sh`（brew 路径）保留为备选。
- **验收**: 全新环境 `docker compose up -d` + `make run` 即可访问站点。

### 0.4 ai-service 脚手架

- 目录按 [architecture.md §6](architecture.md)；依赖管理 **uv**（`pyproject.toml` + lock 提交）。
- 依赖（本阶段）: `fastapi`、`uvicorn[standard]`、`pydantic-settings`、`redis`、`qdrant-client`、`httpx`；开发依赖 `ruff`、`pytest`。
- 实现: `/healthz`（含 Redis/Qdrant 连通性）、`config.py`（`SHAREO_AI_*` 环境变量）、内部 token 校验依赖项。
- **验收**: `uv run uvicorn app.main:app` 起服务，healthz 返回依赖状态；`uv run pytest` 绿。

### 0.5 队列骨架（Go 生产 → Python 消费打通）

- Go `internal/pkg/queue`: 基于现有 go-redis 封装 `PublishIndexPost(ctx, action, postID)` / `PublishBotTask(ctx, convID, msgID)`（XADD，stream 名与字段按 architecture.md §7）。
- Python `app/workers/`: 消费框架——启动时 `XGROUP CREATE ... MKSTREAM`（幂等）、`XREADGROUP` 循环、成功 `XACK`、启动时 `XAUTOCLAIM` 接管 pending、失败重试 3 次后记日志 XACK 跳过。
- Go `internal/handler/internal_handler.go`: `/internal/*` 路由组 + `X-Internal-Token` 校验中间件（token 走 `SHAREO_INTERNAL_TOKEN`）。
- **验收**: 手工 XADD 一条测试消息，Python worker 消费、ACK、日志可见；kill worker 重启后 pending 消息被重领。

**Phase 0 退出标准**: 0.1~0.5 验收全过 + 文档同步（README/TASK/roadmap）。

---

## Phase 1 — IM：私聊 + 群组（2026-08 中旬 ~ 09 底）

关键设计决策（写入 design/im.md 后执行）：**发消息走 REST，WebSocket 只做下行推送**——一致性简单（消息必先落库）、WS 断线也能发消息、天然复用限流和认证。

| 步骤 | 内容 | 技术选型 | 验收 |
|------|------|----------|------|
| 1.1 | 设计文档 `design/im.md`（按模板，含 CSRF 评估） | — | 评审定稿 |
| 1.2 | 迁移 `009_chat.sql`：`conversations`（type dm/group、title、owner_id）、`conversation_members`（UNIQUE(conv_id,user_id)、role、`last_read_message_id`）、`messages`（conv_id、sender_id、type、content、INDEX(conv_id,id)）；DM 唯一化用 `dm_key = "小uid:大uid"` 唯一列 | MySQL（现有） | 迁移可重复执行 |
| 1.3 | model + repository：全部方法带 `context.Context`，构造函数注入（评审架构结论落地） | GORM（现有） | 单测（纯逻辑部分） |
| 1.4 | `internal/ws`：Hub（按 userID 索引连接集合、注册/注销、向指定用户集扇出）、每连接 read/write pump goroutine、ping/pong 心跳（30s）、写缓冲满即断开 | **gorilla/websocket v1.5.x**（事实标准、维护活跃） | 竞态检测 `go test -race` 过 |
| 1.5 | chat_service：EnsureDM（并发安全，靠 dm_key 唯一约束兜底）、CreateGroup、Join/Leave、SendMessage（校验成员→落库→查会话成员→Hub 推送）、History（`before_id` 游标分页）、MarkRead、UnreadCounts | — | service 单测 |
| 1.6 | 路由：`GET /ws`（握手复用 Cookie JWT + Origin 校验）；REST `POST/GET /api/v1/conversations`、`GET/POST /api/v1/conversations/:id/messages`、`PUT .../read` | — | curl 全链路 |
| 1.7 | 在线状态：`SETEX ws:online:{uid} 60`，心跳续期；会话列表显示在线点 | Redis（现有） | — |
| 1.8 | 前端 `web/templates/chat/chat.html`：会话列表 + 消息窗 + 原生 WebSocket + 指数退避重连 + 断线期间 REST 拉增量 | Alpine.js（现有） | 双浏览器互测 |
| 1.9 | header 未读徽章（复用通知铃铛模式）+ 帖子页"私信作者"入口 | — | — |
| 1.10 | 冒烟脚本 `scripts/test_chat.sh` + 手测清单入 design/im.md | — | 全过 |

**退出标准**: roadmap 验收 + 消息可靠性手测（断网重连不丢不重——重连拉增量按 last message id）。

---

## Phase 2 — 语义搜图（2026-10 ~ 11 中旬）

| 步骤 | 内容 | 技术选型 | 验收 |
|------|------|----------|------|
| 2.1 | 设计文档 `design/image-search.md` | — | 评审定稿 |
| 2.2 | `core/embedding.py`：Chinese-CLIP 封装（懒加载、device 自动 cuda>mps>cpu、批处理接口）；`core/vectorstore.py`：ensure_collection `images`（512 维 cosine，payload: post_id/image_id/created_at） | **transformers + torch**，模型 `OFA-Sys/chinese-clip-vit-base-patch16` | 单测：同图自相似度≈1 |
| 2.3 | Go 事件挂钩：ReviewPost(approve→upsert / reject→delete)、Delete/AdminSoftDelete(→delete)、Update 重审通过后 upsert | queue（0.5 产物） | 事件日志可见 |
| 2.4 | `GET /internal/posts/:id/index-payload`：返回 status/content/images(object keys)，供 worker 拉取 | — | curl 带 token 可用 |
| 2.5 | worker `index_post`：upsert 流程用 **medium 尺寸图**（省算力且 CLIP 输入 224px 足够）；delete 按 post_id 过滤删 | minio-py? → 不引，直接 **httpx 走图片代理** 或 MinIO SDK（设计文档定，倾向 `minio` 官方 SDK 只读凭证） | 发帖过审→向量出现；删帖→向量消失 |
| 2.6 | 搜索链路：Python `POST /v1/search/images`（编码+KNN）→ Go `GET /api/v1/search/images`（补全+可见性过滤）→ 搜索页双模式 tab（关键词/语义） | — | 端到端演示 |
| 2.7 | 回填：`make backfill-index`（Go 侧扫 approved 分页发事件） | — | 存量图可搜 |
| 2.8 | **评测**：`docs/eval/image_search_v1.jsonl`（30~50 query 标注）；`ai-service` 内评测脚本输出 Recall@5/10、MRR；基线=FULLTEXT 关键词搜索 | — | 报告写入 docs/eval/experiments.md |
| 2.9 | 4060 部署试跑 + 吞吐对比（CPU vs MPS vs CUDA，imgs/sec-批大小曲线） | Ollama 无关，纯 torch | 实验记录 |

**退出标准**: roadmap 验收 + 评测报告 v1。

---

## Phase 3 — Bot + RAG（2026-11 中旬 ~ 2027-01 上旬）

| 步骤 | 内容 | 技术选型 | 验收 |
|------|------|----------|------|
| 3.1 | 设计文档 `design/rag-bot.md`（含 prompt 版本管理约定） | — | 评审定稿 |
| 3.2 | 迁移 `010_bot.sql`：`users.is_bot` + seed Bot 账号；messages 加 `meta` JSON 列（存 citations） | — | — |
| 3.3 | 触发：SendMessage 后检测（DM 对端是 bot / 群消息含 @bot）→ PublishBotTask | — | 事件可见 |
| 3.4 | 文本索引：chunker（一帖一块起步，超 512 token 滑窗）→ **bge-small-zh-v1.5**（512 维）→ `post_chunks` collection（挂进 2.5 的 index_post worker） | transformers（复用） | 过审帖文本可检索 |
| 3.5 | `rag/`：retriever（top-k=8 + 分数阈值）、`prompts.py`（带 [1][2] 引用标记规范）、`llm.py`（**openai SDK** 统一封装，base_url 可配 provider：DeepSeek / SiliconFlow / Ollama，见 ADR-004） | openai SDK ≥1.x | 单测 mock LLM |
| 3.6 | worker `bot_task`：拉会话上下文（`GET /internal/conversations/:id/context`，近 20 条）→ RAG → `POST /internal/bot/reply` → Go 以 bot 身份走 SendMessage 管线 | — | 端到端 |
| 3.7 | 前端：bot 气泡样式 + 引用卡片（链接到帖子）；bot 处理中"正在输入"占位 | — | — |
| 3.8 | **评测**：`rag_qa_v1.jsonl` 30 条；引用命中率 + 人工 1~5 分；**对比实验：DeepSeek API vs Qwen2.5-7B-Q4（4060/Ollama）** 质量/延迟/成本 | — | 实验记录 |

**退出标准**: roadmap 验收 + Bot 降级验证（停掉 ai-service，聊天完全正常）。

---

## Phase 4 — Agent 化 + 收尾（2027-01 ~ 03 中旬）

| 步骤 | 内容 | 技术选型 | 验收 |
|------|------|----------|------|
| 4.1 | 设计文档 `design/agent.md`（工具规范、安全边界） | — | 评审定稿 |
| 4.2 | `tools.py`：只读工具 search_images / search_posts / get_post / summarize_thread；写工具 draft_post（生成草稿并回复确认链接，**不直接发帖**） | 复用既有内部接口 | 工具单测 |
| 4.3 | Agent 循环：openai function calling，最多 5 轮，每轮决策日志入 meta | openai SDK（复用） | 多步任务演示 |
| 4.4 | 安全边界：工具白名单、写操作确认制、单任务超时 60s、失败回复兜底话术 | — | 异常路径手测 |
| 4.5 | Agent 评测：15 个多步任务（如"找三张雪山的图并总结这些作者"），完成率/平均步数 | — | 实验记录 |
| 4.6 | 实验补全 + 中期检查材料（直接取自 docs/） | — | — |

---

## 依赖清单汇总（引入即写 ADR 的除外，以下为已批准）

| 侧 | 依赖 | 用途 | 状态 |
|----|------|------|------|
| Go | gorilla/websocket v1.5.x | WS | Phase 1 引入 |
| Go | log/slog（标准库） | 日志 | Phase 0 |
| Python | fastapi / uvicorn / pydantic-settings | 服务框架 | Phase 0 |
| Python | redis / qdrant-client / httpx / minio | 基础客户端 | Phase 0/2 |
| Python | torch / transformers / pillow | Embedding | Phase 2 |
| Python | openai（SDK） | LLM 统一入口 | Phase 3 |
| Python | ruff / pytest | 工具链 | Phase 0 |

**明确不引入**：LangChain/LlamaIndex（ADR-005）、RabbitMQ/Kafka（ADR-002）、前端框架（范围控制）、ORM for Python（AI 服务不碰业务库）。
