# ShareO OrbStack 隔离演示

本文记录从干净提交启动隔离试跑、用合成数据验收，以及 3 分钟面试演示的固定步骤。运行时事实和交付边界以 [`docs/REVIEW.md`](../REVIEW.md) 为准；本文不宣称真实 LLM 质量、容量或 SLA。

## 本次验收条件

- 审核基线：`origin/orbstack` 的 `79b54f2`；本轮可靠性修改提交见 PR #1。
- 主机：Apple Silicon，OrbStack `linux/arm64`。本轮用 `shareo-orbstack-fresh-20261123`（`127.0.0.1:18092`）做空库和完整链路复验，用 `shareo-orbstack-reliable-20261123`（`127.0.0.1:18091`）做错误目标、presence 降级与基线采样复验；原有试跑继续使用 `18080`。
- 数据：独立 PostgreSQL 17/pgvector、Redis、MinIO、模型缓存卷，以及新生成的数据库/JWT/内部令牌/MinIO 凭据。
- LLM：仓库内 `app.testing.mock_llm` 固定响应服务，仅用于验证程序链路、引用权限和恢复语义。
- 模型缓存：从本机已有模型缓存卷只读复制，具体来源和大小以复验记录为准；模型权重没有重新下载。
- 镜像：复用既有隔离镜像；具体镜像 ID、创建时间和复用范围以复验记录为准。

## 从零启动

先确认旧试跑和 Homebrew 服务的状态，不停止它们：

```bash
docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}'
brew services list
```

准备独立 env。下面的 `SHAREO_*` 值只作为本地示例，请自行生成随机凭据，并保持文件权限为 `0600`：

```bash
umask 077
mkdir -p .local/shareo/shareo-orbstack-fresh-20261123
if [ ! -e .local/shareo/shareo-orbstack-fresh-20261123/.env ]; then
  install -m 0600 deploy/env.orbstack.example \
    .local/shareo/shareo-orbstack-fresh-20261123/.env
fi
```

至少修改：

```dotenv
SHAREO_ORBSTACK_APP_PORT=18092
SHAREO_ORBSTACK_VOLUME_PREFIX=shareo-orbstack-fresh-20261123
SHAREO_ORBSTACK_APP_IMAGE=shareo-orbstack-reliable-20261123-app:baseline
SHAREO_ORBSTACK_AI_IMAGE=shareo-orbstack-reliable-20261123-ai:baseline
SHAREO_ORBSTACK_MINIO_IMAGE=shareo-orbstack-minio:latest
SHAREO_AI_LLM_BASE_URL=http://llm-fixture:8000/v1
SHAREO_AI_LLM_API_KEY=fixture-key
SHAREO_AI_LLM_MODEL=fixture-model
```

如果本机已有模型缓存卷，可复制到新卷；没有缓存时跳过复制，首次索引会下载模型：

```bash
docker volume create shareo-orbstack-fresh-20261123-ai-model-cache
docker run --rm \
  -v shareo-orbstack-reliable-20261123-ai-model-cache:/from:ro \
  -v shareo-orbstack-fresh-20261123-ai-model-cache:/to \
  alpine:3.22 sh -lc 'cp -a /from/. /to/'
```

解析并启动。`fixture` profile 只在显式启用时加入本地固定响应服务：

```bash
pilot() {
  docker --context orbstack compose \
    --project-name shareo-orbstack-fresh-20261123 \
    --env-file .local/shareo/shareo-orbstack-fresh-20261123/.env \
    -f deploy/compose.orbstack.yaml "$@"
}

pilot config --quiet
pilot --profile fixture up --detach --wait
pilot ps --all
curl --noproxy '*' --fail http://127.0.0.1:18092/healthz
```

预期五个长驻服务健康，`postgres-init` 以 `0` 退出，`llm-fixture` 健康；只有 Go 发布宿主机端口 `18092`。

## 验收命令

脚本只使用公网/内部 HTTP 接口。数据库直写仅包括给刚注册的合成管理员提升角色，以及查询基线所需的合成会话数据；Redis 只写入带 TTL 的合成在线键，ACL 临时禁用命令后总会在 `finally` 中恢复。任何写入前，脚本先校验 base URL 是 loopback、端口与指定 Compose 项目的 Go 发布端口一致，并拒绝原 `shareo-orbstack` 项目和未隔离的状态目录。

```bash
cd ai-service
uv run --frozen python ../scripts/orbstack_smoke.py init --demo
uv run --frozen python ../scripts/orbstack_smoke.py prime --demo
uv run --frozen python ../scripts/orbstack_smoke.py search --demo
uv run --frozen python ../scripts/orbstack_smoke.py faults --demo
uv run --frozen python ../scripts/orbstack_smoke.py presence --demo
uv run --frozen python ../scripts/orbstack_smoke.py baseline --demo
```

`--demo` 要求 `ai-service` 的 LLM 地址精确指向 `http://llm-fixture:8000/v1`，固定响应服务不发布宿主机端口。状态与凭据仅保存在被 Git 忽略的 `.local/shareo/shareo-orbstack-fresh-20261123/`，权限为 `0600`/`0700`。重复执行 `prime` 会创建新的合成账号，不影响旧试跑。

## 复验记录

本节记录新隔离项目的实际复验结果。旧的 `shareo-orbstack-clean-20261123` 结果保留在 [工程审查](../REVIEW.md#2026-11-23-首轮优化与-orbstack-验证)，不与本轮结果混写。样本数量和时间条件随行记录，不把结果外推为并发、吞吐或真实模型质量。

| 验收项 | 实际结果 |
|---|---|
| 目标防串线 | 错误端口、非 loopback、原项目名和未配置项目均在状态写入前失败；配置有效时校验端口、容器项目标签和 fixture 地址 |
| 服务与初始化 | `shareo-orbstack-fresh-20261123` 五个核心服务均 `healthy`；`postgres-init` 为 `exited` 且退出码 `0`；`llm-fixture` 健康且未发布宿主机端口；只有 Go 发布 `127.0.0.1:18092` |
| 空库 | 新 PostgreSQL 卷：用户 1 个（仅 `shareo_bot`），帖子、消息、outbox、图片/正文向量均为 0 |
| 角色隔离 | `shareo_app` 不能写 `ai` schema，`shareo_ai` 不能写 `public` schema |
| 待审核可见性 | 合成 PNG 发帖后公开详情为 `404`，检索不可见，图片代理拒绝；审核后正文检索、语义搜图和图片读取均通过 |
| Bot 引用 | 固定响应服务返回带已审核帖子引用的回复；同一新库中引用预览和图片可访问 |
| Redis 发布失败 | 禁用 `XADD` 后 outbox 保持 `pending` 且 `attempts` 增长、无回复；恢复后补发一条回复 |
| 重复回调 | 同一 HTTP 回调返回原回复 ID 与内容，响应体 `code=0`，`bot_replies` 只有 1 行 |
| 内部 `401` | 按本次 `source_message_id` 确认 Stream 条目 pending 且无回复；恢复令牌后重领并只提交 1 条带引用回复 |
| Presence | 无 WebSocket 时合成 Redis 键令对端在线；禁用 `EXISTS` 后该对端按离线处理且顺序不变；恢复后在线；Hub 命中与断线清除均通过 |

普通私聊列表在单用户、无 WebSocket、无引用预览、顺序请求条件下，每种规模预热 1 次后请求 20 次；命令计数和延迟使用同一正式采样窗口。

| 返回会话数 | Redis `EXISTS`（20 次） | Redis `MGET`（20 次） | 估算往返次数 | 中位耗时 | p95 |
|---:|---:|---:|---:|---:|---:|
| 1 | 20 | 0 | 20 | 1.929 ms | 3.010 ms |
| 10 | 200 | 0 | 200 | 2.693 ms | 4.582 ms |
| 100 | 2000 | 0 | 2000 | 10.911 ms | 15.319 ms |

往返次数按应用层一条 Redis 命令对应一次请求/响应估算，不是抓包 RTT。界面耗时包含查询准备和 JSON 序列化，不只是 Redis 往返。本轮未测 `MGET` 优化版本，不据此宣称延迟改善。

复验条件：2026-11-23，Apple Silicon、OrbStack `linux/arm64`；新项目 `shareo-orbstack-fresh-20261123`，端口 `127.0.0.1:18092`；Go/AI 镜像为 `shareo-orbstack-reliable-20261123-{app,ai}:baseline`（Go `ac9e3433d580`，AI `e68758310af0`）；模型缓存从 `shareo-orbstack-reliable-20261123-ai-model-cache` 只读复制，约 1.5 GiB、35 个文件；MinIO 复用 `shareo-orbstack-minio:latest`（`8745ee867819`），PostgreSQL/Redis 复用官方镜像。旧 `18080` 栈和 Homebrew 服务全程保持原状态。

## 3 分钟面试演示

1. **30 秒，架构。** 浏览器只访问 Go；Go 是 PostgreSQL `public` schema 的唯一写者，Python 只写 `ai` schema/pgvector，并通过内部令牌读取 Go 的严格载荷。共识是“数据所有权清楚，异步链路可恢复”。
2. **30 秒，发帖审核。** 上传合成 PNG 并发布，演示待审核帖子在公开详情、检索和图片代理中不可见；管理员审核后立即进入公开检索。强调这不是前端隐藏，而是服务端可见性条件。
3. **45 秒，检索和引用。** 用中文正文搜到帖子，再用语义搜图命中同一张图；向 `shareo_bot` 提问，展示带帖子预览和图片的引用回复，点击引用核对来源。说明 Python 只产生候选，Go 再按 `approved AND is_deleted=0` 复核。
4. **45 秒，故障恢复。** 暂时禁止 Redis `XADD`，发一条 Bot 消息，展示 PostgreSQL outbox 仍为 `pending` 且重试数增加；恢复后自动补发。再用同一 `source_message_id` 重复回调，说明返回同一回复、数据库只有一条业务行。随后可补一句错误内部令牌导致 pending、修正后 Worker 重领。
5. **30 秒，边界。** 明确这是固定响应 LLM 的链路/权限/幂等验证，不代表真实模型回答质量；Redis 发布失败期间受保护请求仍按会话 fail-closed 处理，outbox 只保证任务最终可补发。

## 停止

只停止本轮项目，保留卷和已停止容器，便于下次复验：

```bash
pilot --profile fixture stop
```

不要执行 `down -v`，也不要删除任何卷。旧 `shareo-orbstack` 项目和本机 Homebrew 服务不在本演示的清理范围内。
