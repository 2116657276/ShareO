# ShareO 运行手册

日常启动、关闭和常用 Make 命令请先看[用户使用指南](../USER_GUIDE.md)。本手册只保留开发审查、索引维护和故障排查内容；当前范围和证据边界见[工程审查](../REVIEW.md)。

## 环境与配置

本机运行需要 Go 1.25.1+、Python 3.12、uv、Homebrew PostgreSQL 17（pgvector）、Redis 和 MinIO。AI 评测还需要当前配置的真实 Provider 及本机 47 条图片语料。复制 [`config.yaml.example`](../../config.yaml.example) 为 `config.yaml`，敏感配置只放在未提交的 `.env` 或环境变量中。默认 Go/AI 端口为 `8080`/`8000`。改变端口时，检查显式配置的跨服务 base URL 是否与生效端口一致。

```bash
cp config.yaml.example config.yaml
cp .env.example .env
brew install postgresql@17 pgvector redis minio
brew services start postgresql@17
brew services start redis
make bootstrap-postgres
export SHAREO_INTERNAL_TOKEN="$(openssl rand -hex 32)"
```

不要把 API Key、Token、Cookie、Authorization header、代理凭证、个人照片、模型权重或原始 Provider 响应写入仓库。

## 本机启动

推荐使用宿主机 Go/Python 和本机依赖，启动顺序如下：

```bash
make local-doctor
make local-infra-up
SHAREO_OPEN_BROWSER=0 make up
```

`make up` 会加载当前 Go/AI 源码，按需启动由 ShareO 管理的进程，完成模型预热并等待 `/healthz` 和三个 capability readiness。`/readyz/rag` 与 `/readyz/agent` 均检查 `index_post` 和 `bot_tasks` consumer。单条 Bot 任务的进度可从 Redis Streams、outbox 和 AI 日志确认。停止项目进程可执行以下命令。

```bash
make down
make local-infra-down
```

常用排查：

```bash
make doctor
make logs
set -a; . ./.env; set +a
curl --fail "http://127.0.0.1:${SHAREO_APP_PORT:-8080}/healthz"
curl --fail "http://127.0.0.1:${SHAREO_AI_PORT:-8000}/healthz"
curl --fail -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  "http://127.0.0.1:${SHAREO_AI_PORT:-8000}/readyz/agent"
```

没有 LLM 配置时，RAG 和 Agent readiness 返回不可用属于受控降级；在 PostgreSQL、Redis 和 Go 会话校验正常的前提下，社区、普通私聊和非 LLM 搜索仍可用，不应把 AI readiness 失败误判为整个站点失败。

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
SHAREO_SKIP_CURRENT_EVIDENCE=1 make check
go test -race ./...
make test-integration-auto
make test-api
```

`make check` 还会执行当前 Final Freeze 证据门禁；在新的权威运行、三轮 AI 复评和稳定源码指纹收口前，完整命令按设计会报告已知阻塞。日常代码检查使用上面的跳过当前证据选项，不通过删改历史证据绕过门禁。

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

仓库根目录的旧 Compose 使用 MySQL 和 Qdrant，已退役。历史配置和记录保留用于对照。本机 Homebrew 路径与下面的 OrbStack 试跑各自使用独立的依赖和数据。

## OrbStack 隔离试跑

在 Apple Silicon 上使用 `orbstack` context 启动独立的 `shareo-orbstack` 项目。运行时包括 Go、单进程 Python、PostgreSQL 17/pgvector、Redis 和 MinIO。MinIO 从固定官方源码版本构建。浏览器入口默认为 `http://127.0.0.1:18080`，其他服务使用项目内网络。

在仓库根目录执行以下命令。首次复制后，按 [`deploy/env.orbstack.example`](../../deploy/env.orbstack.example) 的说明填入独立随机凭据。已有试跑配置可以直接复用。

```bash
mkdir -p .local/shareo/orbstack
if [ ! -e .local/shareo/orbstack/.env ]; then
  install -m 0600 deploy/env.orbstack.example .local/shareo/orbstack/.env
fi
```

```bash
pilot() {
  docker --context orbstack compose --project-name shareo-orbstack \
    --env-file .local/shareo/orbstack/.env -f deploy/compose.orbstack.yaml "$@"
}

pilot config --quiet
pilot up --detach --build --wait
pilot ps --all
curl --fail http://127.0.0.1:18080/healthz
```

`pilot ps --all` 应显示 `postgres-init` 以退出码 `0` 完成，五个长驻服务处于 `healthy`。模型缓存使用独立命名卷，首次加载需要下载模型权重。可用 `pilot --profile warmup run --rm ai-warmup` 先填充该卷。LLM 地址和密钥默认留空，此时 Bot 提示 Provider 不可用，RAG 和 Agent readiness 为降级。做固定响应验证时，将 LLM 地址指向同一试跑网络内的测试服务。

需要与已有试跑并行验收时，使用隔离参数：`SHAREO_ORBSTACK_APP_PORT` 指定不同宿主机端口，`SHAREO_ORBSTACK_VOLUME_PREFIX` 指定独立卷前缀，`SHAREO_ORBSTACK_APP_IMAGE`/`SHAREO_ORBSTACK_AI_IMAGE`/`SHAREO_ORBSTACK_MINIO_IMAGE` 指定独立镜像标签。`--profile fixture` 会启动不发宿主端口的 `llm-fixture` 服务；将 `SHAREO_AI_LLM_BASE_URL` 设为 `http://llm-fixture:8000/v1` 即可走本地固定响应。完整隔离试跑、验收结果和 3 分钟演示见 [隔离演示](orbstack-demo.md)。

试跑仅使用合成账号、合成图片和试跑卷。暂停时执行 `pilot --profile fixture stop`，保留卷以便继续验证。恢复时执行 `pilot --profile fixture up --detach --wait`。

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
| Bot 不回复 | `/readyz/rag`、AI 日志、`bot_tasks`、`bot_task_outbox` | 核对 LLM、Go 与 AI 的内部令牌、队列待处理记录和 outbox。Go 重启会重扫未发布任务，Worker 会重领未 ACK 的任务。 |
| Agent 不可用 | `/readyz/agent`、LLM 配置、两个 consumer 状态 | 先确认默认 RAG 和普通私聊仍可用，再核对 Agent 开关、模型、向量存储与内部接口。 |
| 普通消息已落库但无实时推送 | Redis、WebSocket、`after_id` | REST 数据仍是真相；恢复 Redis 后重新连接并补偿 |
| 评论提交或点赞报 `like_count` 不存在 | `comments.like_count`、`comment_likes` | 确认使用 `migrations/postgres/001_schema.sql` 初始化的全新 PostgreSQL 数据库；应用启动不会自动迁移 |
| Bot 消息已落库但任务未发布 | `bot_task_outbox.status`、`next_attempt_at` | 确认 PostgreSQL schema 已初始化，检查 Go publisher 日志；不直接手工重复发送消息 |
| 向量数量不一致 | `make reconcile-index` | 先 dry-run，确认后再 `APPLY=1` |

全新本机数据集使用 `CONFIRM=YES make local-photo-seed`。它会先校验 PostgreSQL 业务库仅含固定 Bot/测试用户、快照并清空指定 MinIO bucket，再上传 47 张图片、创建已审核帖子、发布索引任务并校验代理与语义搜图；不会自动删除已有业务库或猜测凭据。

恢复依赖后的行为和其它发布证据边界不在本手册重复维护，以 [工程审查](../REVIEW.md) 和 Final Freeze 证据归档为准。
