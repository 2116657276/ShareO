# ShareO 运行手册

本手册只保留完工版本仍会用到的本机运行、检查、评测和 Compose 启停入口。最终范围和证据边界见 [完工冻结审查](../REVIEW.md)。

## 环境与配置

本机运行需要 Go 1.25.1+、Python 3.12、uv、MySQL 8、Redis、MinIO 和 Qdrant。AI 评测还需要当前配置的真实 Provider 及本机 47 条图片语料。复制 [`config.yaml.example`](../../config.yaml.example) 为 `config.yaml`，敏感配置只放在未提交的 `.env` 或环境变量中。

```bash
cp config.yaml.example config.yaml
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

`make up` 会加载当前 Go/AI 源码，按需启动由 ShareO 管理的进程，等待 `/healthz` 和三个 capability readiness。它不会重置 MySQL、MinIO 或 Qdrant 数据。停止项目进程：

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
make prepare-current-ai-eval
```

这会根据本机图片和 manifest 建立当前帖子、索引及评测输入；不会生成已退役的旧 Demo 数据。操作前确认当前数据库和对象存储属于 ShareO 本机环境。

## 检查与回归

```bash
make check
go test -race ./...
make test-integration-auto
make test-api
```

`make test-integration` 用于已经准备好的独立测试 DSN；它要求 `SHAREO_TEST_MYSQL_DSN` 和 `SHAREO_TEST_REDIS_URL`，不会把跳过集成测试算作通过。`make test-api` 只通过 HTTP API 创建临时用户、帖子和合成图片，并在退出时清理测试对象，不重置业务数据。

## 当前评测与证据

单独运行评测：

```bash
make eval-post-search DATASET=.local/shareo/eval/post_search_v1.jsonl
make eval-image-search-local DATASET=.local/shareo/eval/image_search_local_v1.jsonl
make eval-ai MACHINE_ONLY=1
make eval-agent MACHINE_ONLY=1
```

统一运行并归档：

```bash
make final-evidence
```

当前有效集合为 34 条搜图、32 条搜贴、30 条 RAG 和 36 条 Agent。统一入口记录运行号、Git SHA、dirty 状态、Provider、模型 revision、数据集 SHA256、命令状态、质量指标和可取得的延迟。完整原始日志只留在 `.local/shareo/final-freeze/<run_id>/`；仓库中的 [`docs/evidence/final-freeze/`](../evidence/final-freeze/) 只保存脱敏汇总。

队列等待、回调、工具步骤等字段只有在现有日志能可靠取得时才记录；无法取得时标记 `unavailable`，不通过估算补齐。单条命令失败只执行一次，状态统一为 `pass`、`fail`、`blocked` 或 `not_run`。

## Compose 启停

Compose 是可选部署方式，不代表当前已有空缓存冷启动或完整发布复核证据：

```bash
make compose-up
make compose-logs
make compose-down
```

需要重建 AI 镜像时使用 `make compose-reload-ai`。`make compose-reset CONFIRM=YES` 会删除 Compose 数据卷并重建，属于破坏性操作；执行前应确认项目名、卷和备份位置。

## 索引运维

```bash
make backfill-index
make reconcile-index
make reconcile-index APPLY=1
```

`reconcile-index` 默认只读检查；确认差异及目标环境后才使用 `APPLY=1`。图片对象是否存在、帖子是否可见和 Qdrant 派生索引是否一致，分别以 Go、MinIO 和 AI 对账结果为准。

## 常见故障

| 现象 | 首查 | 处理 |
|---|---|---|
| 页面或图片 404 | Go 日志、MinIO health、对象键 | 确认 MinIO 和 Go 图片代理；数据库记录不等于对象可读 |
| 语义搜图 503 | `/readyz/image-search`、Qdrant | 等待模型预热，确认 Qdrant 和 index consumer |
| Bot 不回复 | `/readyz/rag`、AI 日志、`bot_tasks` | 检查 LLM 配置、Redis Streams 和受控重试 |
| Agent 不可用 | `/readyz/agent`、LLM 配置 | 先确认默认 RAG 和普通私聊仍可用，再检查 Agent readiness |
| 普通消息已落库但无实时推送 | Redis、WebSocket、`after_id` | REST 数据仍是真相；恢复 Redis 后重新连接并补偿 |
| 向量数量不一致 | `make reconcile-index` | 先 dry-run，确认后再 `APPLY=1` |

恢复依赖后的行为、未运行的 Compose 故障注入和其它发布证据缺口，不在本手册重复维护，以 [完工冻结审查](../REVIEW.md) 和 Final Freeze 证据为准。
