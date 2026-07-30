# ShareO

ShareO 是一个面向毕业答辩和简历展示的 Go + Python 全栈社区项目。它把图文社区、一对一实时私聊、中文搜贴、中文语义搜图、带可访问引用的 RAG Bot 和受限只读知识 Agent 收在同一套业务链路中。

项目已经完工冻结。当前结论、证据等级和已接受缺口见 [完工冻结审查](docs/REVIEW.md)；本次两次真实 Provider 运行的脱敏汇总见 [Final Freeze 证据](docs/evidence/final-freeze/README.md)。项目不宣称通用 Agent、多 Agent、生产级高并发、线上 SLA 或公共克隆一键复现完整图片质量评测。

## 项目范围

已实现的业务能力包括：

- 注册登录、资料、图文发布、管理员审核、Feed、点赞、关注、评论、通知和私人收藏。
- WebSocket 一对一私聊，支持消息落库、未读、断线补偿和 Bot 回复推送。
- 正文混合搜贴：BGE/Qdrant 语义召回与 MySQL 关键词召回独立融合。
- Chinese-CLIP + Qdrant 中文语义搜图，公开结果由 Go 做最终可见性复核。
- 默认私聊 Bot 固定走 RAG；用户显式开启深度分析后，才进入 LangGraph 只读 Agent。
- Agent 仅有语义检索、关键词检索、帖子读取和图片检索四类工具，不具备写入、外部调用、长期记忆或多 Agent 能力。

转帖、话题、群聊、写入型 Agent、外部工具、独立 Bot 页面、音视频和生产级多租户不在范围内。

## 架构

```text
浏览器
  → Go 服务 → MySQL / Redis / MinIO / WebSocket
          → 内部 HTTP + Token → Python FastAPI
                                  → Chinese-CLIP / FastEmbed
                                  → Qdrant
                                  → RAG / LangGraph Agent / LLM
```

Go 是业务数据和 MySQL 的唯一写者，负责认证、权限、审核、社区、聊天和 Bot 回复；Python 是 Qdrant 的唯一写者，负责 embedding、向量检索、RAG、Agent 和 Redis Streams consumer。MinIO 凭证只由 Go 持有，Python 通过 Go 图片代理读取图片。所有公开帖子和引用最终都必须满足 `approved AND is_deleted=0`。

关键链路如下：

```text
发布 → 审核 → index_post → Redis Streams → 图片/正文索引 → Qdrant → 搜索 → Go 可见性复核
私聊 → bot_tasks → RAG → 引用白名单 → Go 二次校验 → Bot 落库 → WebSocket
ai_mode=agent → LangGraph → 四类只读工具 → 脱敏轨迹 → 引用复核 → 回复展示
```

## 当前真实数据

以下结果来自 2026-07-30 的当前 47 条本机帖子语料和真实 Provider；最新运行号为 `20260730T143316Z`，工作区状态为 dirty。它们是特定本机环境下的可复核诊断数据，不是线上 SLA、通用准确率或第三方评审结论。

| 评测 | 规模 | 结果 |
|---|---:|---|
| 搜图开发集 | 34 条 | Recall@5 `0.9222`，Recall@10 `1.0000`，MRR `0.9750` |
| 搜贴开发集 | 32 条 | hybrid Recall@5 `0.8906`，MRR `0.9688`，nDCG@5 `0.9158` |
| RAG 机器评测 | 30 条 | 来源命中率 `0.8333`，引用可访问率 `100%`，虚假引用 `0` |
| Agent 机器评测 | 36 条 | 来源命中率 `1.0000`，必需工具选择率 `1.0000`，安全门禁失败 `0` |

两次当前语料运行均已保留：`20260730T140713Z` 和 `20260730T143316Z`。完整指标、延迟、运行参数、数据集 SHA256 和失败矩阵见 [证据目录](docs/evidence/final-freeze/)。本机图片和完整 Provider 原始输出不进入 Git。

## 快速开始

环境要求：Go 1.25.1+、Python 3.12、uv，以及本机 MySQL、Redis、MinIO 和 Qdrant。日常入口使用宿主机服务；Docker Compose 只作为可选部署方式，不构成当前完工结论的必要条件。

```bash
cp config.yaml.example config.yaml
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
./start.sh
```

不需要打开浏览器时使用 `SHAREO_OPEN_BROWSER=0 make up`。检查和本机回归：

```bash
make check
make test-integration-auto
make test-api
```

当前本机评测需要已建立的 47 条图片语料。评测集生成与运行：

```bash
make prepare-search-eval
make prepare-current-ai-eval
make eval-post-search DATASET=.local/shareo/eval/post_search_v1.jsonl
make eval-image-search-local DATASET=.local/shareo/eval/image_search_local_v1.jsonl
make eval-ai MACHINE_ONLY=1
make eval-agent MACHINE_ONLY=1
```

统一采集完整脱敏证据使用 `make final-evidence`。原始日志和完整评测 JSON 只保存在 `.local/shareo/final-freeze/<run_id>/`，仓库仅保留脱敏汇总。

## 常用命令

| 命令 | 用途 |
|---|---|
| `make up` / `./start.sh` | 启动本机 Go、AI 和依赖服务 |
| `make down` / `make local-stop` | 停止由项目启动的本机进程 |
| `make doctor` | 检查本机依赖、端口、缓存和 readiness |
| `make local-photo-seed CONFIRM=YES` | 重建当前本机图片测试语料及向量 |
| `make check` | Go、Python、Shell 和文档门禁 |
| `make test-api` | HTTP API、聊天和 Agent 边界回归 |
| `make test-integration-auto` | 当前本机 MySQL/Redis/Qdrant 集成测试 |
| `make test-integration` | 使用显式测试 DSN 的集成测试 |
| `make eval-post-search` | 评测 keyword-only、semantic-only、hybrid 搜贴 |
| `make eval-image-search-local` | 评测当前搜图开发集 |
| `make eval-ai` / `make eval-agent` | 运行当前 RAG / Agent 机器评测 |
| `make final-evidence` | 生成一次完整脱敏运行归档 |
| `make backfill-index` / `make reconcile-index` | 索引回填与对账 |
| `make compose-up` / `make compose-down` | 可选 Docker Compose 启停 |

## 文档

- [完工冻结审查](docs/REVIEW.md)：最终定位、关键链路、指标和证据边界。
- [Final Freeze 证据](docs/evidence/final-freeze/README.md)：当前两次运行的脱敏归档入口。
- [架构](docs/architecture.md)、[功能矩阵](docs/features.md)、[API 参考](docs/reference/api.md)、[数据模型](docs/reference/data-model.md)。
- [当前评测规范](docs/eval/README.md)与[运行记录](docs/eval/experiments.md)。
- [运行手册](docs/operations/runbook.md)、[开发规范](docs/standards.md)、[ADR](docs/adr/)。
- [简历项目审查](docs/resume-review.md)：简历和答辩中的推荐表述。

历史过程由 Git 提交记录追溯；当前文档只保留稳定设计、最终状态和可复核证据。
