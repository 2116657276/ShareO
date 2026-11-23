# ShareO OrbStack 隔离演示

本文记录从干净提交启动隔离试跑、用合成数据验收，以及 3 分钟面试演示的固定步骤。运行时事实和交付边界以 [`docs/REVIEW.md`](../REVIEW.md) 为准；本文不宣称真实 LLM 质量、容量或 SLA。

## 本次验收条件

- 代码：`origin/orbstack` 的 `5888b63`，试跑经历的支持提交为 `d445a34`。
- 主机：Apple Silicon，OrbStack `linux/arm64`，Compose 项目 `shareo-orbstack-clean-20261123`。
- 入口：`http://127.0.0.1:18090`；原有试跑继续使用 `18080`，两者同时运行。
- 数据：独立 PostgreSQL 17/pgvector、Redis、MinIO、模型缓存卷，以及新生成的数据库/JWT/内部令牌/MinIO 凭据。
- LLM：仓库内 `app.testing.mock_llm` 固定响应服务，仅用于验证程序链路、引用权限和恢复语义。
- 模型缓存：新卷从本机已有 `shareo-orbstack-ai-model-cache` 复制，约 1.5 GiB；模型权重没有重新下载。
- 镜像：Go 与 AI 镜像从本次源码新建；MinIO 复用同一提交此前构建的 `shareo-orbstack-minio:latest`，本次没有重建该镜像。

## 从零启动

先确认旧试跑和 Homebrew 服务的状态，不停止它们：

```bash
docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'
brew services list
```

准备独立 env。下面的 `SHAREO_*` 值只作为本地示例，请自行生成随机凭据，并保持文件权限为 `0600`：

```bash
umask 077
mkdir -p .local/shareo/orbstack-clean-20261123
cp deploy/env.orbstack.example .local/shareo/orbstack-clean-20261123/.env
chmod 0600 .local/shareo/orbstack-clean-20261123/.env
```

至少修改：

```dotenv
SHAREO_ORBSTACK_APP_PORT=18090
SHAREO_ORBSTACK_VOLUME_PREFIX=shareo-orbstack-clean-20261123
SHAREO_ORBSTACK_APP_IMAGE=shareo-orbstack-clean-20261123-app:local
SHAREO_ORBSTACK_AI_IMAGE=shareo-orbstack-clean-20261123-ai:local
SHAREO_ORBSTACK_MINIO_IMAGE=shareo-orbstack-minio:latest
SHAREO_AI_LLM_BASE_URL=http://llm-fixture:8000/v1
SHAREO_AI_LLM_API_KEY=fixture-key
SHAREO_AI_LLM_MODEL=fixture-model
```

如果本机已有模型缓存卷，可复制到新卷；没有缓存时跳过复制，首次索引会下载模型：

```bash
docker volume create shareo-orbstack-clean-20261123-ai-model-cache
docker run --rm \
  -v shareo-orbstack-ai-model-cache:/from:ro \
  -v shareo-orbstack-clean-20261123-ai-model-cache:/to \
  alpine:3.22 sh -lc 'cp -a /from/. /to/'
```

解析并启动。`fixture` profile 只在显式启用时加入本地固定响应服务：

```bash
pilot() {
  docker --context orbstack compose \
    --project-name shareo-orbstack-clean-20261123 \
    --env-file .local/shareo/orbstack-clean-20261123/.env \
    -f deploy/compose.orbstack.yaml "$@"
}

pilot config --quiet
pilot --profile fixture up --detach --build --wait
pilot ps --all
curl --noproxy '*' --fail http://127.0.0.1:18090/healthz
```

预期五个长驻服务健康，`postgres-init` 以 `0` 退出，`llm-fixture` 健康；只有 Go 发布宿主机端口 `18090`。

## 验收命令

脚本只使用公网/内部 HTTP 接口。唯一的数据库直写是给刚注册的合成管理员提升角色；Redis ACL 临时禁用命令后总会在 `finally` 中恢复。

```bash
cd ai-service
uv run --frozen python ../scripts/orbstack_smoke.py init
uv run --frozen python ../scripts/orbstack_smoke.py prime
uv run --frozen python ../scripts/orbstack_smoke.py search
uv run --frozen python ../scripts/orbstack_smoke.py faults
uv run --frozen python ../scripts/orbstack_smoke.py presence
uv run --frozen python ../scripts/orbstack_smoke.py baseline
```

状态与凭据仅保存在被 Git 忽略的 `.local/shareo/orbstack-clean-20261123/`，权限为 `0600`/`0700`。重复执行 `prime` 会创建新的合成账号，不影响旧试跑。

## 本次实际结果

2026-11-23 在本机 Apple Silicon、OrbStack `linux/arm64` 上执行上述命令，结果如下。样本数量和时间条件随行记录，不把结果外推为并发、吞吐或真实模型质量。

| 验收项 | 实际结果 |
|---|---|
| 服务与初始化 | 五个核心服务 healthy；`postgres-init` 退出码 `0`；固定响应服务 healthy；旧 `18080` 仍 healthy |
| 空库 | 用户仅 `shareo_bot` 1 个；帖子、消息、outbox、图片/正文向量均为 0 |
| 角色隔离 | `shareo_app` 不能写 `ai` schema，`shareo_ai` 不能写 `public` schema |
| 待审核可见性 | 发帖后公开详情返回 `404`，检索不到，图片代理拒绝；审核后正文检索、图片检索和图片读取均通过 |
| 索引 | 合成帖子审核后，`ai.post_chunk_embeddings` 与 `ai.image_embeddings` 各 1 条 |
| Bot 引用 | 固定响应服务返回一条带已审核帖子引用的回复；DM 持久化与 WebSocket 下发通过 |
| Redis 发布失败 | 禁用 `XADD` 后 outbox 保持 `pending` 且 `attempts` 增长、无回复；恢复后补发成功 |
| 重复回调 | 同一消息再次回调返回相同回复 ID 与内容，`bot_replies` 只有 1 行 |
| 内部 `401` | 使用错误 AI 内部令牌时任务留在 Redis pending 且无回复；恢复令牌后重领，只有 1 条带引用回复 |
| 最终 outbox | 4 条任务全部 `published`，`pending` 为 0；4 条业务回复各 1 行 |

普通私聊列表在用户未连接 WebSocket、没有引用预览、单用户顺序请求条件下，每种规模预热 1 次后请求 20 次。

| 返回会话数 | Redis `EXISTS`（20 次） | Redis `MGET`（20 次） | 估算往返次数 | 中位耗时 | p95 |
|---:|---:|---:|---:|---:|---:|
| 1 | 21 | 0 | 21 | 2.472 ms | 2.927 ms |
| 10 | 210 | 0 | 210 | 3.208 ms | 6.552 ms |
| 100 | 2103 | 0 | 2103 | 10.142 ms | 13.174 ms |

往返次数按应用层一条 `EXISTS` 对应一次 Redis 请求/响应估算，不是抓包 RTT。`EXISTS` 数约为 `请求次数 × 会话数`，多出的少量来自预热或并发后台读取，因此只用于说明随会话数线性增长，不作为精确单请求上限。WebSocket 本地 Hub 命中时，本次请求没有产生 `MGET`。

## 3 分钟面试演示

1. **30 秒，架构。** 浏览器只访问 Go；Go 是 PostgreSQL `public` schema 的唯一写者，Python 只写 `ai` schema/pgvector，并通过内部令牌读取 Go 的严格载荷。共识是“数据所有权清楚，异步链路可恢复”。
2. **30 秒，发帖审核。** 上传合成 PNG 并发布，演示待审核帖子在公开详情、检索和图片代理中不可见；管理员审核后立即进入公开检索。强调这不是前端隐藏，而是服务端可见性条件。
3. **45 秒，检索和引用。** 用中文正文搜到帖子，再用语义搜图命中同一张图；向 `shareo_bot` 提问，展示带帖子预览和图片的引用回复，点击引用核对来源。说明 Python 只产生候选，Go 再按 `approved AND is_deleted=0` 复核。
4. **45 秒，故障恢复。** 暂时禁止 Redis `XADD`，发一条 Bot 消息，展示 PostgreSQL outbox 仍为 `pending` 且重试数增加；恢复后自动补发。再用同一 `source_message_id` 重复回调，说明返回同一回复、数据库只有一条业务行。随后可补一句错误内部令牌导致 pending、修正后 Worker 重领。
5. **30 秒，边界。** 明确这是固定响应 LLM 的链路/权限/幂等验证，不代表真实模型回答质量；Redis 发布失败期间受保护请求仍按会话 fail-closed 处理，outbox 只保证任务最终可补发。

## 停止

只停止本轮项目，保留卷和已停止容器，便于下次复验：

```bash
pilot stop
```

不要执行 `down -v`，也不要删除任何卷。旧 `shareo-orbstack` 项目和本机 Homebrew 服务不在本演示的清理范围内。
