# ShareO 运行手册

日常启动、关闭和常用 Make 命令请先看[用户使用指南](../USER_GUIDE.md)。本手册只保留开发审查、索引维护和故障排查内容；当前范围和证据边界见[工程审查](../REVIEW.md)。

## 环境与配置

本机运行需要 Go 1.25.1+、Python 3.12、uv、Homebrew PostgreSQL 17（pgvector）、Redis 和 MinIO。AI 评测还需要当前配置的真实 Provider 及本机 47 条图片语料。复制 [`config.yaml.example`](../../config.yaml.example) 为 `config.yaml`，敏感配置只放在未提交的 `.env` 或环境变量中。

```bash
cp config.yaml.example config.yaml
cp .env.example .env
brew install postgresql@17 pgvector redis minio
brew services start postgresql@17
brew services start redis
make bootstrap-postgres
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
```

不要把 API Key、Token、Cookie、Authorization header、代理凭证、个人照片、模型权重或原始 Provider 响应写入仓库。

## 本机启动

推荐使用宿主机 Go/Python 和本机依赖，启动顺序如下：

```bash
make local-doctor
make local-infra-up
SHAREO_OPEN_BROWSER=0 make up
make warm-ai
```

`make up` 会加载当前 Go/AI 源码，按需启动由 ShareO 管理的进程，等待 `/healthz` 和三个 capability readiness。它不会重置 PostgreSQL、MinIO 或 Redis 数据。停止项目进程：

```bash
make down
make local-infra-down
```

常用排查：

```bash
make doctor
make logs
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8000/healthz
curl --fail -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/agent
```

没有 LLM 配置时，RAG 和 Agent readiness 返回不可用属于受控降级；社区、普通私聊和非 LLM 搜索不应因此被误判为失败。

## 当前本机语料

本机图片不进入 Git。已有本机数据需要重建时，执行：

```bash
make local-photo-seed CONFIRM=YES
make prepare-search-eval
make prepare-current-ai-eval VERSION=v2
```

这会根据本机图片和 manifest 建立当前帖子、索引及评测输入；不会生成已退役的旧 Demo 数据。操作前确认当前数据库和对象存储属于 ShareO 本机环境。

## 检查与回归

```bash
make check
go test -race ./...
make test-integration-auto
make test-api
```

`make test-integration` 用于已经准备好的独立测试 DSN；它要求 `SHAREO_TEST_POSTGRES_DSN`、`SHAREO_TEST_AI_DATABASE_URL` 和 `SHAREO_TEST_REDIS_URL`，不会把跳过集成测试算作通过。`make test-integration-auto` 会创建一个带 `_test` 后缀的临时 PostgreSQL 数据库，测试结束后保留以便检查。`make test-api` 只通过 HTTP API 创建临时用户、帖子和合成图片，并在退出时清理测试对象，不重置业务数据。

## 当前评测与证据

单独运行评测：

```bash
make eval-post-search DATASET=.local/shareo/eval/post_search_v1.jsonl
make eval-image-search-local DATASET=.local/shareo/eval/image_search_local_v1.jsonl
SHAREO_RAG_DATASET=.local/shareo/eval/rag_qa_current_v2.jsonl make eval-ai MACHINE_ONLY=1
SHAREO_AGENT_DATASET=.local/shareo/eval/agent_tasks_current_v2.jsonl make eval-agent MACHINE_ONLY=1
```

统一运行并归档：

```bash
make final-evidence
```

当前代码默认评测集合为 38 条搜图（含 8 条 no-match）、32 条搜贴、30 条 v2 RAG 和 36 条 v2 Agent；证据目录的最新报告仍使用 v1 RAG/Agent 输入，不能混用两套指标。统一入口记录运行号、Git SHA、dirty 状态、Provider、模型 revision、数据集 SHA256、命令状态、质量指标和可取得的延迟；AI judge 另记录模型、Prompt SHA、双轮分数和人工复核条目。完整原始日志只留在 `.local/shareo/final-freeze/<run_id>/`；仓库中的 [`docs/evidence/final-freeze/`](../evidence/final-freeze/) 只保存脱敏汇总。

队列等待、回调、工具步骤等字段只有在现有日志能可靠取得时才记录；无法取得时标记 `unavailable`，不通过估算补齐。单条命令失败只执行一次，状态统一为 `pass`、`fail`、`blocked`、`needs_human_review` 或 `not_run`。

## 旧运行时

MySQL、Qdrant、Colima、OrbStack 和 Compose 已从当前运行时退役。旧配置和存储可以人工保留作回滚材料，但启动、健康检查、回填和 seed 脚本不会访问它们。

## 索引运维

```bash
make backfill-index
make reconcile-index
make reconcile-index APPLY=1
```

`reconcile-index` 默认只读检查；确认差异及目标环境后才使用 `APPLY=1`。图片对象是否存在、帖子是否可见和 pgvector 派生索引是否一致，分别以 Go、MinIO 和 AI 对账结果为准。

## 常见故障

| 现象 | 首查 | 处理 |
|---|---|---|
| 页面或图片 404 | Go 日志、MinIO health、对象键 | 确认 MinIO 和 Go 图片代理；数据库记录不等于对象可读 |
| 语义搜图 503 | `/readyz/image-search`、PostgreSQL pgvector | 等待模型预热，确认 pgvector 表和 index consumer |
| Bot 不回复 | `/readyz/rag`、AI 日志、`bot_tasks`、`bot_task_outbox` | 检查 outbox 是否 pending、LLM 配置、Redis Streams 和受控重试；重启 Go 会自动补发 |
| Agent 不可用 | `/readyz/agent`、LLM 配置 | 先确认默认 RAG 和普通私聊仍可用，再检查 Agent readiness |
| 普通消息已落库但无实时推送 | Redis、WebSocket、`after_id` | REST 数据仍是真相；恢复 Redis 后重新连接并补偿 |
| 评论提交或点赞报 `like_count` 不存在 | `comments.like_count`、`comment_likes` | 确认使用 `migrations/postgres/001_schema.sql` 初始化的全新 PostgreSQL 数据库；应用启动不会自动迁移 |
| Bot 消息已落库但任务未发布 | `bot_task_outbox.status`、`next_attempt_at` | 确认 PostgreSQL schema 已初始化，检查 Go publisher 日志；不直接手工重复发送消息 |
| 向量数量不一致 | `make reconcile-index` | 先 dry-run，确认后再 `APPLY=1` |

全新本机数据集使用 `CONFIRM=YES make local-photo-seed`。它会先校验 PostgreSQL 业务库仅含固定 Bot/测试用户、快照并清空指定 MinIO bucket，再上传 47 张图片、创建已审核帖子、发布索引任务并校验代理与语义搜图；不会自动删除已有业务库或猜测凭据。

恢复依赖后的行为和其它发布证据边界不在本手册重复维护，以 [工程审查](../REVIEW.md) 和 Final Freeze 证据归档为准。
