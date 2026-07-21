# ShareO v2 详细开发计划

> 更新时间: 2026-07-21 | 状态: **Phase 0 本地门禁完成、远端 CI 待确认 / Phase 1 后端加固中** | 上游: [roadmap.md](roadmap.md)（里程碑）/ [architecture.md](architecture.md)（架构）
>
> 选型总原则：**优先标准库，其次成熟稳定的第三方库**，不引实验性依赖。每步给出验收标准；串行推进，Phase 内允许微调顺序。
>
> 任务状态以 [TASK.md](../TASK.md) 与本文为准。GitHub Issues 仅是可选协作工具。2026-07-21 审计发现早期“Phase 0 完成”缺少可复现与验收证据，因此在最终门禁通过前不再标记完成。当前按用户确认，后端成熟前以终端自动化（curl/Shell/集成测试）为主，浏览器与前端手工验收暂缓。

## Phase 0 — 基建 + 修复清单（收口中）

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

- `deploy/docker-compose.yml`: `mysql:8.0`、`redis:7-alpine`、固定版本的 MinIO/Qdrant、ai-service/worker，带 named volume、healthcheck，端口与 `config.yaml.example` 对齐。
- 首次启动自动执行 migrations（mysql 容器 `docker-entrypoint-initdb.d` 挂载）。
- README 快速开始改为 compose 优先，`start.sh`（brew 路径）保留为备选。
- **验收**: 全新环境 `docker compose up -d --wait` + `make run` 即可访问站点；2026-07-21 已在本机通过 Compose config/up/ready/reset，MinIO、Qdrant 和 uv 基础镜像记录 digest。

### 0.4 ai-service 脚手架

- 目录按 [architecture.md §6](architecture.md)；依赖管理 **uv**（`pyproject.toml` + lock 提交）。
- 依赖（本阶段）: `fastapi`、`uvicorn[standard]`、`pydantic-settings`、`redis`、`qdrant-client`、`httpx`；开发依赖 `ruff`、`pytest`。
- 实现: `/healthz` 仅表示进程存活；`/readyz` 在 Redis 与 Qdrant 都可用时返回 200，否则 503；`config.py` 使用 `SHAREO_AI_*` 环境变量。
- **验收**: `uv run uvicorn app.main:app` 起服务，liveness/readiness 语义正确；`uv run pytest` 绿。

### 0.5 队列骨架（Go 生产 → Python 消费打通）

- Go `internal/pkg/queue`: 基于现有 go-redis 封装 `PublishIndexPost(ctx, action, postID)` / `PublishBotTask(ctx, convID, msgID)`（XADD，stream 名与字段按 architecture.md §7）。
- Python `app/workers/`: 消费框架——启动时 `XGROUP CREATE ... MKSTREAM`（幂等）、`XREADGROUP` 循环、成功 `XACK`、启动时 `XAUTOCLAIM` 接管 pending、失败重试 3 次后记日志 XACK 跳过。
- Go `internal/handler/internal_handler.go`: `/internal/*` 路由组 + `X-Internal-Token` 校验中间件（token 走 `SHAREO_INTERNAL_TOKEN`）。
- **验收**: 手工 XADD 一条测试消息，Python worker 消费、ACK、日志可见；kill worker 重启后 pending 消息被重领。

### 0.6 2026-07-21 收口项

- Streams 首次失败不 ACK；每 10 秒扫描、30 秒 idle 后 `XAUTOCLAIM`，总计 4 次处理后记录并 ACK；遵守 ADR-002 不设死信队列。
- Python 固定 3.12，dev dependency group 与 `uv.lock` 入库，CI 使用 locked/frozen。
- `make check` 串行执行 Go/Python/Shell；真实服务测试放到 `make test-integration`。
- MySQL 初始化不再执行 seed/cleanup；Compose 增加 config/up/ready/reset/clean 命令。
- 业务、仓储与 WS 日志完成 `slog` 迁移。

**Phase 0 退出标准**: `make check`、真实 Redis 集成、Compose config/up/ready/reset、终端 API 脚本和文档同步全部有证据。上述本地门禁与工作区复核已完成；收口分支已推送，远端 CI 结果待确认，未确认前保持收口状态，不把暂缓的浏览器手工验收混入 Phase 0。

---

## Phase 1 — IM：私聊 + 邀请制群组（加固中）

关键设计决策（写入 design/im.md 后执行）：**发消息走 REST，WebSocket 只做下行推送**——一致性简单（消息必先落库）、WS 断线也能发消息、天然复用限流和认证。

| 步骤 | 当前实现 | 自动证据 | 尚缺证据 |
|------|----------|----------|----------|
| 1.1 数据一致性 | `010_chat_hardening.sql` 前向加约束；DM/建群/发消息/解散事务化；邀请上限并发保护 | repository/service 单测；迁移过的真实 `*_test` 库集成已通过 | CI/发布环境迁移演练（发布前补充） |
| 1.2 群组模型 | 仅群主直接邀请；重复邀请幂等；普通成员退出；群主只能解散 | service 单测、`scripts/test_chat.sh` | 双浏览器验收（暂缓） |
| 1.3 消息与未读 | 1–2000 字；精确未读 SQL；MarkRead 验证归属并单调推进 | 单元测试、MySQL 集成、`scripts/test_chat.sh` | 真实数据库执行已通过 |
| 1.4 WS 与在线 | 完整登录缓存校验；30 秒心跳/60 秒 TTL；最后连接断开清 key；会话吊销主动断线 | Hub 单测、真实 Redis Compose | 双浏览器验收（暂缓） |
| 1.5 恢复协议 | POST 与 WS 按消息 ID 去重；重连后 `after_id` 升序补齐；`before_id` 继续向前分页 | API 冒烟脚本、真实 MySQL | 浏览器断网验收（暂缓） |
| 1.6 安全 | 精确 Origin；全站 `http.CrossOriginProtection`；POST logout；浏览计数 POST | CSRF 单元矩阵 | 部署域名配置验收 |
| 1.7 页面 | `?conv=` 自动打开；建群、邀请、退出、解散入口；在线/未读每 30 秒刷新 | 模板实现（本轮不继续扩展） | 可用性手测（暂缓） |

**退出标准**: `make check`、`make test-integration`、终端 API/IM 冒烟、`go test -race ./...` 和安全/文档门禁全部通过；上述后端门禁当前已通过。双浏览器清单在用户确认后作为发布前补充验收，完成前官方状态保持“Phase 1 后端加固中”。

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
