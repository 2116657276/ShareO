# ShareO — Go 社区与 AI 检索项目

ShareO 是面向简历展示与毕业答辩的轻量全栈项目，完整主线包括图文社区、一对一实时私聊、中文语义搜图和带帖子引用的 RAG Bot。项目强调可复现启动、清晰数据所有权、受控降级和量化评测。

> 当前状态：Phase 0–7 已完成；Phase 7C 发布与演示收口已完成。实时状态见 [TASK.md](TASK.md)。

## 核心能力

- 注册登录、图文发布、Feed、全文搜索和管理员审核。
- 点赞、关注、评论与通知。
- WebSocket 一对一私聊、准确未读和断线恢复。
- Chinese-CLIP + Qdrant 中文语义搜图。
- FastEmbed + Qdrant + DeepSeek/OpenAI-compatible LLM 的引用式 RAG Bot。

收藏、转帖、话题、群聊和 Agent 已从最终范围移除。

## 架构

Go 负责全部业务和 MySQL写入，Python FastAPI单进程负责模型、Qdrant和两个Redis Streams consumer。MinIO凭证只由Go持有，Bot引用由Python候选白名单和Go可见性二次校验。

详细说明见 [架构文档](docs/architecture.md) 和 [单上下文](CONTEXT.md)。

## 技术栈与项目结构

项目由 Go 主服务、Python AI 服务和 Docker Compose 基础设施组成。Go 使用 Gin、MySQL、Redis、MinIO 和 WebSocket；Python 使用 FastAPI、Chinese-CLIP、FastEmbed、Qdrant 和 OpenAI-compatible LLM。前端为 Go 模板与静态资源，不单独维护 Node.js 构建链。

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

环境要求：Go 1.25.1+、Python 3.12、uv、Docker Compose（或 Colima）。

Docker 运行时至少需要 **3GB 内存**（Colima 用户执行 `colima start --cpu 2 --memory 3 --disk 40`）。`compose.yaml` 已为各服务设置内存上限（总和约 2.6GB），200 倍 demo 数据量内可稳定运行。

```bash
cp config.yaml.example config.yaml
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
make up
docker compose ps
```

发布收口的自动门禁和独立故障验证：

```bash
make check
make test-integration
make test-image-e2e
make test-ai-e2e
make test-degradation
```

Phase 7C 的冷启动、故障矩阵、五分钟演示和 40/30 评测复核见 [`docs/evidence/phase7c/evidence-matrix.md`](docs/evidence/phase7c/evidence-matrix.md)。

首次启动会下载 Chinese-CLIP 和 FastEmbed模型。检查基础服务：

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
| `make up` / `make down` | 启停六服务 |
| `make reset CONFIRM=YES` | 删除数据卷并重建 |
| `make check` | Go、Python、Shell和文档门禁 |
| `make test-integration` | 真实MySQL/Redis集成 |
| `make test-image-e2e` | 真实图片索引E2E |
| `make test-ai-e2e` | mock LLM私聊Bot跨服务E2E |
| `make backfill-index` | approved帖子索引回填 |
| `make reconcile-index` | 图文索引对账，默认dry-run |
| `make demo-seed` | 初始化 26 篇 Demo 帖子和图片 |
| `make eval-ai` | 运行 40 条搜图 + 30 条 RAG 统一评测 |

## 项目文档

- [当前任务](TASK.md)与[执行计划](docs/plan.md)
- [Phase 0–7](docs/phases/README.md)
- [API参考](docs/reference/api.md)与[数据模型](docs/reference/data-model.md)
- [AI评测](docs/eval/README.md)与[五分钟演示](docs/demo.md)
- [技术决策](docs/adr/)

## 演示主线

用户发布图文并经管理员审核，`index_post` 异步建立图片和正文索引；用户可以用中文描述搜图，也可以私聊 `shareo_bot`，由Bot根据已审核正文回答并返回可访问帖子引用。
