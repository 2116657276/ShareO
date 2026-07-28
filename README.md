# ShareO — Go 社区与可评测引用式 RAG 项目

ShareO 是面向简历展示与毕业答辩的轻量全栈项目，主线包括图文社区、一对一实时私聊、中文语义搜图、带帖子引用的 RAG Bot，以及同一私聊入口中的只读社区知识 Agent。项目强调清晰数据所有权、受控降级、可审计工具调用和量化评测。

> 项目定位：本地优先开发的 Go + Python 引用式 RAG 工程，并包含同一私聊入口中的受限只读 Agent。实时状态、当前阻塞和下一项任务统一见 [TASK.md](TASK.md)。

## 核心能力

- 注册登录、图文发布、Feed、全文搜索和管理员审核。
- 点赞、关注、评论与通知。
- WebSocket 一对一私聊、准确未读和断线恢复。
- Chinese-CLIP + Qdrant 中文语义搜图 API；独立 `/search/images` 页面已实现，浏览器验收列入后续人工阶段。
- FastEmbed + Qdrant + DeepSeek/OpenAI-compatible LLM 的引用式 RAG Bot。
- Phase 8 的 LangGraph `StateGraph` 只读知识 Agent：显式深度分析、多步工具选择、步骤轨迹和安全门禁；测试 Provider、协议、集成和故障恢复已有工程证据，真实机器门禁和最终人工验收以 [TASK.md](TASK.md) 为准。

这是固定流程的引用式 RAG 系统，不是通用 Agent：默认路径仍是固定流程 RAG，深度分析模式才进入受限 Agent。Agent 只读扩展没有写入工具、外部工具、长期记忆或多 Agent 能力。当前主线是后端门禁、Go Template 前端补齐、用户人工验收，最后才做 Docker/Compose 打包。收藏、转帖、话题和群聊不在范围内。简历可用表述和证据限制见 [简历项目审查](docs/resume-review.md)。

## 架构

Go 负责全部业务和 MySQL写入，Python FastAPI单进程负责模型、Qdrant、RAG、LangGraph Agent 和两个 Redis Streams consumer。MinIO凭证只由Go持有，Bot引用和 Agent 工具结果由 Python 初筛、Go 最终执行可见性二次校验。

详细说明见 [架构文档](docs/architecture.md) 和 [单上下文](CONTEXT.md)。

## 技术栈与项目结构

项目由 Go 主服务、Python AI 服务和可本机运行或 Compose 运行的基础设施组成。Go 使用 Gin、MySQL、Redis、MinIO 和 WebSocket；Python 使用 FastAPI、Chinese-CLIP、FastEmbed、Qdrant 和 OpenAI-compatible LLM。前端为 Go 模板与静态资源，不单独维护 Node.js 构建链。日常开发不要求 Docker；Compose 用于真实集成、故障验证和源码冷启动。

```text
cmd/                    Go 可执行程序：主服务、索引回填、工具
internal/
  config/               配置加载
  handler/              HTTP/API 处理器
  middleware/           认证、限流、跨域等中间件
  model/                领域模型
  repository/           MySQL、Redis 和状态持久化
  router/               路由注册
  service/              业务服务与跨服务编排
  ws/                   WebSocket 连接管理与消息投递
  pkg/                  JWT、响应、上传、队列等通用组件
ai-service/
  app/main.py            FastAPI 路由和应用入口
  app/config.py          AI 服务配置
  app/core/             AI 基础设施、embedding 和向量存储
  app/rag/              分块、索引、检索和回答管线
  app/agent/            Phase 8 LangGraph 状态图、只读工具和轨迹模型
  app/workers/          Redis Streams consumer 与 Bot worker
  app/commands/         评测、回填和对账命令
  app/testing/           测试用 mock provider
  tests/                Python 单元测试与集成测试
web/                    Go 模板、CSS 和浏览器静态资源
migrations/             当前基线数据库 schema
deploy/                 Docker 部署配置与 MySQL 初始化脚本
resources/              Demo 静态资源与图片
scripts/                启动、校验、seed 和跨服务 E2E 脚本
docs/                   架构、设计、API、数据模型、阶段、运行和评测文档
```

关键数据边界是：Go 负责业务数据和 MySQL 写入，Python 负责向量数据和 Qdrant 写入；所有公开帖子和 Bot 引用最终由 Go 校验可见性。目录职责和依赖规则如需调整，应同步更新架构文档或 ADR。

## 文档事实来源

- [`TASK.md`](TASK.md)：当前任务、阻塞状态和阶段进度。
- [`docs/plan.md`](docs/plan.md)：当前阶段的执行顺序和退出门禁。
- [`CONTEXT.md`](CONTEXT.md)：项目单上下文、最终范围和架构边界。
- [`docs/architecture.md`](docs/architecture.md)：服务关系、数据所有权和降级矩阵。
- [`docs/standards.md`](docs/standards.md)：开发、API、安全和测试规范。
- [`docs/adr/`](docs/adr/)：技术决策及其历史替代关系。

状态、计划、架构和代码不一致时，分别以以上对应事实来源、实际代码、数据库 schema 和配置模板为准。

## 快速开始

环境要求：Go 1.25.1+、Python 3.12、uv、Homebrew 服务和已编译的 Qdrant。当前开发与自动验收优先使用本机服务；Docker Compose（或 Colima）暂缓到用户人工验收之后，只用于最终打包和发布复核。

Docker 运行时至少需要 **3GB 内存**（Colima 用户执行 `colima start --cpu 2 --memory 3 --disk 40`）。`compose.yaml` 已为各服务设置内存上限（总和约 2.6GB）；项目没有归档压力测试，不对吞吐量、并发用户数或容量上限作承诺。

```bash
cp config.yaml.example config.yaml
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
make start-local
```

`make start-local` 是本机一键入口：按需启动 Homebrew 的 `mysql@8.0`、Redis、MinIO，从 `qdrant/` 指定目录启动 Qdrant，使用宿主机 `uv` 和 Go 启动 AI/业务服务，等待图片检索、RAG、Agent readiness 和基础服务全部通过后打开 `http://127.0.0.1:8080/home`。它优先复用本地 Go/uv/模型缓存，不删除数据；停止时只回收由脚本启动的进程。需要分步排查时仍可使用 `make local-doctor`、`make local-infra-up` 和 `make dev-local`。完整 Compose 启动使用 `make up`，不作为日常开发入口。

本机后端门禁的自动验证：

```bash
make check
make test-integration
make test-image-e2e
make test-ai-e2e
make test-degradation
```

Phase 7C 的新卷启动、故障矩阵、API 计时冒烟和 40/30 评测复核见 [`docs/evidence/phase7c/evidence-matrix.md`](docs/evidence/phase7c/evidence-matrix.md)。其中镜像构建和人工浏览器演示仍有明确证据缺口。

首次启动会下载 Chinese-CLIP 和 FastEmbed 模型。Agent 依赖锁定后与 RAG 共用 OpenAI-compatible provider；没有 LLM Key 时深度分析只返回受控不可用，不影响社区和普通私聊。检查基础服务：

```bash
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8000/healthz
curl --fail -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/image-search
```

没有 LLM Key时 RAG readiness返回503是预期行为，不影响社区、普通私聊或语义搜图。DeepSeek配置和故障排查见 [运行手册](docs/operations/runbook.md)。

## 已可用命令

| 命令 | 说明 |
|---|---|
| `make up` / `make down` | 最终 Docker/Compose 打包复核时启停六服务；当前暂缓 |
| `make local-doctor` | 检查本机服务、Qdrant、缓存、代理和 readiness，不输出密钥 |
| `make local-infra-up` / `make local-infra-down` | 管理 Homebrew 基础服务和本机 Qdrant |
| `make dev-local` | 使用本机 Go 与 uv 启动业务和 AI 服务 |
| `make start-local` | 一键启动本机服务，等待完整 readiness 后打开网页 |
| `make local-photo-seed CONFIRM=YES` | 重建 47 张本机人工测试照片样本并完成向量化 |
| `make local-stop` | 停止本机脚本启动的 Go、AI、Qdrant 进程 |
| `make reset CONFIRM=YES` | 删除数据卷并重建 |
| `make check` | Go、Python、Shell和文档门禁 |
| `make test-integration` | 真实MySQL/Redis集成 |
| `make test-api` | 本机 API 契约和功能回归；使用临时数据，不重置 Demo |
| `make test-image-e2e` | 真实图片索引E2E |
| `make test-ai-e2e` | mock LLM私聊Bot跨服务E2E |
| `make test-degradation` | 最终 Docker/Compose 故障与恢复复核；当前暂缓 |
| `make backfill-index` | approved帖子索引回填 |
| `make reconcile-index` | 图文索引对账，默认dry-run |
| `make demo-seed` | 初始化 26 篇 Demo 帖子和图片 |
| `make eval-ai` | 运行 40 条搜图 + 30 条 RAG 统一评测 |
| `make test-agent-e2e` | Phase 8 Agent mock provider 跨服务 E2E |
| `make eval-agent` | Phase 8 Agent 机器指标和人工评分复核 |

`make demo-seed` 和冻结数据集质量评测需要 `resources/static/pictures/` 下脚本指定的 26 张 JPEG。个人照片和图片本体不进入 Git，因此公共克隆默认不具备这部分素材；缺少图片时脚本会直接失败。图片/Bot 工程 E2E 使用测试内生成素材，不依赖这些照片。

## 项目文档

- [当前任务](TASK.md)与[执行计划](docs/plan.md)
- [Phase 0–8](docs/phases/README.md)
- [API参考](docs/reference/api.md)与[数据模型](docs/reference/data-model.md)
- [AI评测](docs/eval/README.md)与[五分钟演示](docs/demo.md)
- [简历项目审查与可信表述](docs/resume-review.md)
- [技术决策](docs/adr/)

## 演示主线

用户发布图文并经管理员审核，`index_post` 异步建立图片和正文索引；后端提供中文语义搜图 API，并由独立 `/search/images` 页面调用。用户也可以私聊 `shareo_bot`，由 Bot 根据已审核正文回答并返回可访问帖子引用。深度分析入口在同一私聊中调用只读 Agent，完成后展示脱敏工具步骤和引用；阶段是否完成以门禁和证据为准。
