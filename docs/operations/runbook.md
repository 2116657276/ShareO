# ShareO 运行手册

## 环境与配置

需要 Go 1.25.1+、Python 3.12、uv 和 Docker Compose（或 Colima）。复制 `config.yaml.example` 为 `config.yaml`；密钥优先通过环境变量覆盖。

Docker 运行时至少需要 **3GB 内存**。Colima 用户启动前确认：

```bash
colima start --cpu 2 --memory 3 --disk 40
```

`compose.yaml` 已为六服务配置内存上限（详见文末资源预算表），关闭了 AI 模型启动预热以降低空闲内存占用。

必需的内部一致配置：

```bash
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
```

真实 DeepSeek演示额外配置：

```bash
export SHAREO_AI_LLM_BASE_URL='https://api.deepseek.com'
export SHAREO_AI_LLM_API_KEY='...'
export SHAREO_AI_LLM_MODEL='deepseek-chat'
```

不要把真实 Key写进 `.env` 模板、命令输出、截图或评测报告。

## 冷启动

```bash
cp config.yaml.example config.yaml
make up
docker compose ps
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8000/healthz
```

首次启动需要下载 Chinese-CLIP 和 FastEmbed模型。基础 health通过后，仍需使用内部 token检查：

```bash
curl -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/image-search
curl -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/rag
```

没有 LLM配置时 `/readyz/rag` 返回503是预期降级，不影响社区、普通私聊和语义搜图。

## 日常命令

| 命令 | 作用 |
|---|---|
| `make up` | 构建并等待六服务启动 |
| `make down` | 停止服务，不删除数据卷 |
| `make reset CONFIRM=YES` | 删除卷并重建，具有破坏性 |
| `make logs` | 跟踪Go与AI日志 |
| `make check` | Go/Python/Shell/文档检查 |
| `make test-integration` | 真实MySQL/Redis集成，需要显式测试DSN |
| `make test-image-e2e` | 隔离图片索引E2E |
| `make test-ai-e2e` | mock LLM跨服务Bot E2E |
| `make test-degradation` | 独立 Compose 项目验证五类故障和恢复，不打断 Demo 环境 |
| `make backfill-index` | 投递全部approved帖子 |
| `make reconcile-index` | 默认dry-run索引对账 |
| `make demo-seed` | 初始化 Demo 帖子和图片数据 |
| `make eval-ai` | 运行搜图与 RAG 统一评测 |

## Phase 7C 故障验证

自动验证使用独立项目 `shareo-degradation` 和独立端口，默认结束时删除临时容器、网络和数据卷；不会停止当前 `shareo` Demo 环境。若已有模型缓存，可显式复用已存在的 Docker volume：

```bash
SHAREO_DEGRADATION_NO_BUILD=1 \
SHAREO_DEGRADATION_MODEL_CACHE_VOLUME=shareo_hf_cache \
make test-degradation
```

脚本按“基线通过 → 注入单项故障 → 验证社区/全文/私聊/搜图/Bot → 恢复服务 → readiness 和业务基线再次通过”执行 AI、Qdrant、MinIO、Redis 和测试 LLM provider 五类场景。结果归档在 [`docs/evidence/phase7c/degradation-automated.log`](../evidence/phase7c/degradation-automated.log)。

真实 DeepSeek 断开只做一次人工复核：在当前终端临时保存原有值，使用临时环境变量覆盖 `SHAREO_AI_LLM_BASE_URL` 或 API Key，使 `/readyz/rag` 进入不可用状态，发送一条 Bot 消息确认固定兜底，再恢复原值并重启/等待 readiness。不要修改 `.env`，不要把变量值、Authorization header 或完整响应写入日志；复核结束前确认普通私聊仍可落库。

如果要保护现有 Demo 数据，先导出 MySQL、MinIO 和 Qdrant 卷到工作区外的临时目录，再执行 `make reset CONFIRM=YES`。恢复时只对明确的目标卷操作，禁止把备份归档进 Git。

## 模型缓存

Compose默认使用 `hf_cache` volume。可通过 `SHAREO_AI_MODEL_CACHE_SOURCE` 指向宿主机目录或其他volume。模型revision和名称必须与评测报告一致；升级模型需要重新索引并记录实验版本。

## 日志定位

```bash
docker compose logs --tail=200 app
docker compose logs --tail=200 ai-service
docker compose logs --tail=100 redis qdrant minio mysql
```

图片索引日志应能区分载荷、下载、模型加载、编码、删除和写入耗时。RAG日志应区分embedding、检索、LLM和总耗时。禁止记录 API Key、Authorization header或完整私聊历史。

## 常见故障

| 现象 | 首查 | 处理 |
|---|---|---|
| 页面图片404 | MinIO health、object key、Go日志 | 恢复MinIO；数据库记录存在不代表对象服务可用 |
| 搜图503 | `/readyz/image-search` | 等待预热或检查Qdrant/consumer |
| Bot不回复 | `/readyz/rag`、`bot_tasks` pending | 检查LLM配置、consumer和回调日志 |
| 普通消息已落库但无推送 | Redis、Hub与WS认证 | REST数据仍是真相，重连后用`after_id`恢复 |
| 索引数量不一致 | `make reconcile-index` | 先dry-run；确认后再 `APPLY=1` |
| collection维度错误 | AI启动日志 | 不自动覆盖；备份或删除错误collection后重建 |

## 数据重置

只使用 `make reset CONFIRM=YES` 重建完整Compose数据。执行前确认当前目录、Compose项目名和目标volume；不要直接删除MinIO内部目录或复用历史Homebrew数据路径。

## 容器重建

修改 `compose.yaml`（包括内存限制、启动参数、环境变量）后，需要重建容器使变更生效：

```bash
docker compose down          # 停止但不删数据卷
docker compose up -d --build --wait   # 重建镜像并启动
```

仅重启容器（`docker compose restart`）不会应用 `mem_limit`、`command` 或 `build` 变更。

`make up` 已包含 `--build`，首次启动或修改 compose 后直接用 `make up` 即可。

## 资源预算

compose.yaml 为六个服务设置了内存硬上限，基于 Colima 3GB 分配，目标日常 ~1.1GB、200 倍 demo 数据量峰值 ~2.3GB：

| 服务 | mem_limit | 优化措施 |
|------|-----------|---------|
| MySQL | 400m | buffer-pool 128M、perf-schema OFF、skip-log-bin |
| Redis | 80m | maxmemory 48mb、allkeys-lru、关闭 RDB |
| MinIO | 300m | — |
| Qdrant | 400m | — |
| Go App | 256m | — |
| AI Service | 1.2g | embedding_warmup=false（模型延迟加载） |
| **合计** | **2.64g** | |

AI Service 的内存大头是两个固定大小的 embedding 模型（Chinese-CLIP ~400MB + BGE-small ~100MB），不随帖子数量增长。MySQL buffer pool 和 Qdrant 索引在 5,200 帖规模内不会成为瓶颈。
