# ShareO 运行手册

## 环境与配置

需要 Go 1.25.1+、Python 3.12、uv、Homebrew 服务和已编译的 Qdrant。日常开发与当前自动验收使用本机服务；Docker Compose（或 Colima）搁置到用户人工验收后，只用于最终源码打包、故障复核和冷启动。复制 `config.yaml.example` 为 `config.yaml`；密钥优先通过环境变量覆盖。

## 本机优先开发

日常开发使用宿主机 Go 和 `uv`，不构建 app/AI Docker 镜像。MySQL 使用 Homebrew
`mysql@8.0`，Redis 和 MinIO 使用 Homebrew 服务；Qdrant 必须在仓库下的 `qdrant/`
目录启动：

```bash
make local-doctor
make local-infra-up
make dev-local
```

日常使用也可以直接运行 `make start-local`。该入口会复用上述本机启动逻辑，按顺序等待基础服务、Go/AI 健康检查、图片检索/RAG/Agent readiness 和模型预热全部通过，最后调用 macOS 默认浏览器打开 `/home`。失败时不会打开网页，并会保留本机运行日志供排查；服务已在运行时会复用现有监听，不重复启动。
进行自动化或只想验证就绪状态时，可使用 `SHAREO_OPEN_BROWSER=0 make start-local`，此时不会打开浏览器。

`local-infra-up` 会按需启动已安装的 Homebrew 服务，并从
`qdrant/target/release/qdrant` 启动 Qdrant。Qdrant 的当前工作目录不能改成仓库根目录，
否则可能找不到 `static`、`storage` 或配置。脚本只管理自己启动的 Qdrant、Go 和 AI 进程，
不会停止用户已有的 Homebrew 服务，也不会删除数据库、对象或 Qdrant storage。

### 扩展本机人工测试样本

`make demo-seed` 仍只维护 26 张冻结 Demo/评测素材。若需要使用照片目录中的全部本机照片进行人工测试，运行：

```bash
make local-photo-seed CONFIRM=YES
```

该入口会软删除当前帖子、保留旧 MinIO 对象，交替使用 `demo_alice` 和 `demo_bob` 为每张照片创建一条
5–20 字短文案并审核通过，然后通过 `index_post`、AI worker 和 Qdrant 完成图片/正文向量化。执行清单位于
Git 忽略的 `.local/shareo/local-photo-seed/manifest.json`，支持中断后复用已完成帖子；它不是质量评测入口，
也不会更新冻结数据集报告。旧 MinIO 对象如需物理清理，必须另行核实，当前流程不会删除。

本机 AI 默认使用 loopback 地址访问 Redis、Qdrant 和 Go。模型优先从 Git 忽略的项目缓存
和宿主机用户缓存加载；本机运行脚本会优先选择已完整的缓存，不会因为缓存存在而重复下载。
Go/uv 依赖也优先复用项目缓存。`make local-doctor` 会检查服务、端口、缓存、readiness
和代理状态，但不输出 API Key、Token 或完整代理凭证。

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
export SHAREO_AI_LLM_MODEL='deepseek-v4-flash'
```

不要把真实 Key写进 `.env` 模板、命令输出、截图或评测报告。

Hugging Face endpoint 默认使用官方地址；网络受限时只在被忽略的 `.env` 中覆盖，不要把个人镜像地址写入 Compose：

```bash
export SHAREO_AI_HF_ENDPOINT='https://hf-mirror.com'
```

如果模型缓存为空且 Docker/Colima 不能直接访问 Hugging Face，可为 AI 容器临时配置运行时代理。代理地址必须是 Docker VM 可访问的地址，不能填宿主机的 `127.0.0.1`：

```bash
export SHAREO_AI_HTTP_PROXY='http://docker-host-gateway:7890'
export SHAREO_AI_HTTPS_PROXY='http://docker-host-gateway:7890'
export SHAREO_AI_ALL_PROXY='http://docker-host-gateway:7890'
```

Compose 会把这些变量注入 `ai-service` 运行环境，并将同一组标准代理参数传给 Go 和 AI 镜像构建阶段；`NO_PROXY` 默认包含 `localhost`、`127.0.0.1`、`::1`、Go、Redis 和 Qdrant 服务。模型缓存卷只保存 Hugging Face 模型，不能代替 Docker 的 Python 依赖构建缓存；如果依赖安装所在镜像层未成功完成，下次构建仍会重新下载 PyTorch 或 Go 模块。代理变更后执行 `make reload-ai` 或 `make up`，以构建层成功落盘和 `make doctor` 源码指纹一致为准。不要把真实代理凭证写入仓库。

仓库提供 [`.env.example`](../../.env.example) 作为无密钥配置清单。推荐先复制为 `.env`，再只在本地填写 API Key 和代理；Compose 会自动读取 `.env`，当前 Shell 不会自动导出其中的变量。构建前使用 `make doctor` 检查变量是否存在，构建后使用 `make warm-ai` 检查实际模型、revision、设备和 readiness。代理只用于外部依赖和模型下载，容器之间的 MySQL、Redis、Qdrant、MinIO、Go 和 AI 地址必须走 `NO_PROXY`。

提交前检查：`.env`、`config.yaml`、个人照片、模型缓存、原始响应、Token、Cookie、Authorization header 和 API Key 均必须处于忽略或未暂存状态；脱敏报告只保留指标、类别、post ID、模型 revision、设备、耗时和失败类别。

## 最终 Docker 冷启动（人工验收后）

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

启用 Agent 前额外检查只读 Agent readiness：

```bash
curl --fail \
  -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/agent
```

## 日常命令

| 命令 | 作用 |
|---|---|
| `make local-doctor` | 检查本机 Homebrew/Qdrant、缓存、代理、端口和 readiness |
| `make local-infra-up` | 启动本机 Homebrew 基础服务和 Qdrant |
| `make dev-local` | 使用宿主机 Go 与 `uv` 启动 Go/AI 服务 |
| `make start-local` | 一键启动并等待本机服务就绪后打开网页 |
| `make local-photo-seed CONFIRM=YES` | 重建 47 张本机人工测试照片样本并完成图片/正文向量化 |
| `make local-stop` | 停止本机脚本启动的 Go、AI、Qdrant 进程 |
| `make up` | 构建并等待六服务启动 |
| `make down` | 停止服务，不删除数据卷 |
| `make reset CONFIRM=YES` | 删除卷并重建，具有破坏性 |
| `make logs` | 跟踪Go与AI日志 |
| `make check` | Go/Python/Shell/文档检查 |
| `make test-api` | 本机 API 契约和功能回归；使用唯一临时身份与合成图片，不重置 Demo |
| `make test-integration` | 真实MySQL/Redis集成，需要显式测试DSN |
| `make test-image-e2e` | 隔离图片索引E2E |
| `make test-ai-e2e` | mock LLM跨服务Bot E2E |
| `make test-agent-e2e` | 独立 Compose 中验证 Agent/RAG readiness、深度分析和默认 RAG 回归；源码构建需要完整依赖下载 |
| `make test-degradation` | 独立 Compose 项目验证五类故障和恢复，不打断 Demo 环境 |
| `make test-integration-auto` | 依据当前环境自动选择本机或 Compose，运行真实 MySQL/Redis/AI 集成测试 |
| `make backfill-index` | 投递全部approved帖子 |
| `make reconcile-index` | 默认dry-run索引对账 |
| `make demo-seed` | 初始化 Demo 帖子和图片数据 |
| `make eval-ai` | 运行搜图与 RAG 统一评测 |
| `make eval-ai-machine` | 只运行 RAG 机器门禁，不要求人工评分 |
| `make eval-agent` | 运行 36 条 Agent 机器评测；只有机器门禁全部通过后才应生成 30 条人工评分模板 |
| `make eval-agent-machine` | 只运行 36 条 Agent 机器门禁，不生成人工评分模板 |

API 收口阶段运行 `make test-api`。该目标先通过 `scripts/test_api.sh` 覆盖认证、权限、资料、上传发帖审核、Feed、全文搜索、公开语义搜图、点赞、评论、关注、通知、分页边界和已移除路由 404，再通过 `scripts/test_api_ai.sh` 覆盖内部 Token、readiness、普通私聊、默认 RAG、显式 Agent 和 Agent 私聊边界。两个脚本都只通过 HTTP API 创建本轮测试用户、合成图片和帖子，并在结束时停用测试用户、删除测试帖子；不直接写 MySQL、Redis、MinIO 或 Qdrant，不执行 `make reset`。重复 Stream 投递、Provider 失败、依赖替身和真实集成继续由单测、集成测试、`make warm-ai`、专用 E2E 和机器评测分别验证。

公开语义搜图接口是 `GET /api/v1/search/images`，AI 内部接口是 `POST /v1/search/images`；RAG 接口和 Agent 工具仍为内部边界。浏览器直接导航 JSON API 时出现 `ERR_BLOCKED_BY_CLIENT` 属于浏览器工具限制，API 结果应通过 curl、集成测试或页面内 fetch 验证，不据此改动路由。

## Phase 7C 故障验证（最终 Docker 阶段）

自动验证使用独立项目 `shareo-degradation` 和独立端口，默认结束时删除临时容器、网络和数据卷；不会停止当前 `shareo` Demo 环境。若已有模型缓存，可显式复用已存在的 Docker volume：

```bash
SHAREO_DEGRADATION_NO_BUILD=1 \
SHAREO_DEGRADATION_MODEL_CACHE_VOLUME=shareo_hf_cache \
make test-degradation
```

该脚本在用户人工验收后恢复执行，按“基线通过 → 注入单项故障 → 验证社区/全文/私聊/搜图/Bot → 恢复服务 → readiness 和业务基线再次通过”覆盖 AI、Qdrant、MinIO、Redis 和测试 LLM provider 五类场景。历史结果归档在 [`docs/evidence/phase7c/evidence-matrix.md`](../evidence/phase7c/evidence-matrix.md) 和 [`docs/evidence/phase7c/degradation-automated.log`](../evidence/phase7c/degradation-automated.log)，不能替代最终源码复核。

真实 DeepSeek 断开只做一次人工复核：在当前终端临时保存原有值，使用临时环境变量覆盖 `SHAREO_AI_LLM_BASE_URL` 或 API Key，使 `/readyz/rag` 进入不可用状态，发送一条 Bot 消息确认固定兜底，再恢复原值并重启/等待 readiness。不要修改 `.env`，不要把变量值、Authorization header 或完整响应写入日志；复核结束前确认普通私聊仍可落库。

如果要保护现有 Demo 数据，先导出 MySQL、MinIO 和 Qdrant 卷到工作区外的临时目录，再执行 `make reset CONFIRM=YES`。恢复时只对明确的目标卷操作，禁止把备份归档进 Git。

## Phase 8 Agent 运行与排障

Agent 只在 `ai_mode=agent` 且目标为 `shareo_bot` 时启用；默认消息仍走 RAG。人工验收前的自动排障顺序固定为：文档事实收口 → 多 tool call 协议单测 → 状态图与失败轨迹 → AI 源码指纹一致性 → 配置与 Compose 检查 → 三类 readiness 和主动预热 → 确定性 E2E → 真实集成与故障矩阵 → RAG/Agent 机器评测。看到“预算耗尽”或“工具失败”时先查看脱敏错误类别、工具名、状态和耗时，不收集思维链或原始上下文。

评测报告中的 `provider_error`、兜底回复、缺失轨迹、失败状态和超时都属于失败任务，不能只按“收到了 Bot 消息”计为完成。机器门禁失败时不得开始人工评分，也不得复用旧评分模板；应保留失败报告并在实验记录中追加修复轮次。

OpenAI-compatible 多工具调用要求 assistant 消息中的每个 `tool_call_id` 都有对应 tool message。`max_parallel_tools` 只限制同时执行数量，不得截断待回复的调用；总预算不足时也要为预算外调用逐个返回受控错误。复用当前数据卷或源码 bind mount 时，必须重建 AI 服务并确认 readiness 中的进程源码指纹与工作区一致，单纯 healthz 通过不能证明进程已经加载最新源码。

停止 AI、Qdrant、Redis 或测试 Provider 后，必须确认社区、全文搜索、普通私聊和默认 RAG 的降级行为；恢复服务后重新检查 readiness、消费组和 Bot 回复幂等。Agent 故障验证不允许通过手工补写消息或直接写数据库伪造成功。

当前已有源码跨服务 Agent E2E、测试 Provider 门禁、真实集成和本机 API 回归证据，见 [`docs/evidence/phase8-agent/host-source-e2e.md`](../evidence/phase8-agent/host-source-e2e.md)。最近一次本机工作区复评为 36 条无失败/超时、工具选择率 0.9167、来源命中率 1.0000、引用可访问率 100%、P95 13.39 秒；由于报告在仓库外且工作区为 dirty，干净提交仍需复评。历史 0.6944/0.7778/0.8889 报告和人工评分仅作历史证据。

独立 E2E 可在已构建当前源码镜像且已有模型缓存时执行：

```bash
SHAREO_AGENT_E2E_NO_BUILD=1 \
SHAREO_AGENT_E2E_MODEL_CACHE_VOLUME=shareo_hf_cache \
make test-agent-e2e
```

模型缓存为空时，先为 Docker 构建配置 Docker 虚拟机可达的 HTTP/HTTPS 代理，再执行源码构建；不要把宿主机 loopback 地址直接写进容器或仓库配置。容器重建后必须主动触发图片和正文模型加载，再检查三类 readiness。人工评分只能使用本轮新生成的模板，浏览器演示和真实 Provider 复核只能在自动机器门禁通过后进行。

### Phase 8 人工评分交接（后续）

机器门禁通过后才生成新的评分材料。历史 [`docs/eval/results/phase8a_native_agent_human_scoring_final.md`](../eval/results/phase8a_native_agent_human_scoring_final.md) 只用于审计，不能作为当前验收输入；前端完成后用户抽查新报告中的 10 条 Agent 答案。

填写完成后只复用已保存的机器报告，不重新调用 DeepSeek：

```bash
SHAREO_LOCAL_STATE_DIR="${SHAREO_LOCAL_STATE_DIR:-.local/shareo}" \
REPORT_INPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_machine.json" \
  SCORING_INPUT=docs/eval/results/phase8a_native_agent_human_scores.json \
  OUTPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_final.json" \
  make eval-agent
```

如果复核时评分缺失或非法、机器报告被替换、平均分低于 4.0 或出现低于 3 分，必须保持 Phase 8 未完成。

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

2026-07-26 的新卷验证中，Docker 构建阶段因未继承 Go 代理而持续卡住；2026-07-27 已修复
Go/AI 构建代理传递，并完成当前环境的依赖下载与 AI 重建。空缓存、公共克隆的完整冷启动
仍需单独验证，不能把当前有缓存环境的成功写成最终发布证据。

## 资源预算

`compose.yaml` 为六个服务设置以下内存硬上限，配置总和为 2.64GB：

| 服务 | mem_limit | 优化措施 |
|------|-----------|---------|
| MySQL | 400m | buffer-pool 128M、perf-schema OFF、skip-log-bin |
| Redis | 80m | maxmemory 48mb、allkeys-lru、关闭 RDB |
| MinIO | 300m | — |
| Qdrant | 400m | — |
| Go App | 256m | — |
| AI Service | 1.2g | embedding_warmup=false（模型延迟加载） |
| **合计** | **2.64g** | |

这些数值是容器配置，不是容量测试结果。仓库没有归档 RSS 峰值、吞吐量、并发用户数或 5,200 帖规模压力测试，因此不得据此宣称生产容量或“若干倍 Demo 数据稳定运行”。向量索引和数据库内存仍会随数据量变化，扩容前必须重新测量。
