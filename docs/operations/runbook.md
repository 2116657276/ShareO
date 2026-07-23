# ShareO 运行手册

## 环境与配置

需要 Go 1.25.1+、Python 3.12、uv 和 Docker Compose。复制 `config.yaml.example` 为 `config.yaml`；密钥优先通过环境变量覆盖。

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
| `make backfill-index` | 投递全部approved帖子 |
| `make reconcile-index` | 默认dry-run索引对账 |

`make demo-seed` 和 `make eval-ai` 是 Phase 7交付；在对应实现存在前不得作为已可用命令宣传。

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
