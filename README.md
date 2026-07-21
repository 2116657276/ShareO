# ShareO — 摄影社区 + AI 能力演进中

Go Web 摄影作品管理与社区分享平台（类小红书产品形态），当前作为毕业设计向 v2 演进：**IM 私聊群组 / 图片语义搜索 / RAG Bot / Agent**。

当前状态：**Phase 0 本地完成 / Phase 1 后端候选 / Phase 2 后端进行中**。项目分为后端能力线和发布验收线；实时状态见 [TASK.md](TASK.md)，分阶段执行文档见 [docs/phases/](docs/phases/README.md)。

- **架构与决策**: [docs/architecture.md](docs/architecture.md) · [docs/adr/](docs/adr/)
- **里程碑与详细计划**: [docs/roadmap.md](docs/roadmap.md) · [docs/plan.md](docs/plan.md)
- **当前任务**: [TASK.md](TASK.md)

## 技术栈

| 层级 | 现状 (v1) | v2 增量（规划见 ADR） |
|------|-----------|----------------------|
| 后端 | Go 1.25.1+ + Gin 1.12 + gorilla/websocket | IM 已实现，Phase 2+ 继续增量 |
| 数据 | MySQL 8.0 (GORM) + Redis + MinIO + Redis Streams | Qdrant 已纳入依赖编排，向量业务在 Phase 2 |
| AI 服务 | Python 3.12 + FastAPI + uv lock | `/healthz`、`/readyz` 与 Streams Worker 已实现；模型能力在 Phase 2+ |
| 认证 | JWT (golang-jwt/v5) + bcrypt + Redis 登录缓存 | 复用 |
| 前端 | Go Templates + Bootstrap 5.3 + Alpine.js | 复用（不引 SPA） |
| 配置 | Viper + `SHAREO_*` 环境变量覆盖 | + `SHAREO_AI_*` |

## 快速开始

```bash
git clone <repo-url> && cd ShareO
cp config.yaml.example config.yaml
make dev-config                    # 校验 Compose
make dev-up                        # MySQL/Redis/MinIO/Qdrant/AI API/Worker
make dev-ready                     # 验证 AI liveness/readiness
make seed                          # 可选：开发管理员与演示数据（不会自动执行）
make run                           # 启动 Go 主服务
```

Homebrew MySQL/Redis/MinIO + Docker Qdrant 的推荐混合开发方式：

```bash
make brew-minio-ready
make vector-up && make vector-ready
export SHAREO_INTERNAL_TOKEN=shareo-dev-internal
make ai-run                        # 终端 1
make ai-worker                     # 终端 2
make run                           # 终端 3
make ai-search-ready
```

环境要求：Go 1.25.1+、Python 3.12、uv、Docker Compose。没有 Docker 时可用 `./start.sh` 启动本机 MySQL/Redis/MinIO，但 Qdrant/AI readiness 仍需单独提供。终端 API/IM 验收不依赖浏览器：启动 Go 服务后执行 `bash scripts/test_api.sh` 与 `bash scripts/test_chat.sh`。

本机端口冲突时可在命令前覆盖 `SHAREO_MYSQL_PORT`、`SHAREO_REDIS_PORT`、`SHAREO_MINIO_PORT`、`SHAREO_MINIO_CONSOLE_PORT`、`SHAREO_QDRANT_HTTP_PORT`、`SHAREO_QDRANT_GRPC_PORT`、`SHAREO_AI_PORT`；容器内部地址不变。

常用命令：`make check`、`make test-integration`（需 `SHAREO_TEST_MYSQL_DSN` 与 `SHAREO_TEST_REDIS_URL`）、`make dev-reset`、`make dev-clean-data`、`make help`。

本机 Homebrew 路径使用 `$HOME/minio_data` 的 MinIO，默认 API/Console 端口为 `9000/9001`；执行 `make start` 或 `make brew-minio-ready`。Homebrew MinIO 当前不是 `brew services` 可调度服务，详见 [本地存储说明](docs/operations/local-storage.md)。清空本机旧数据使用 `make brew-reset-data`，该命令会清除 MySQL `shareo`、Redis DB 0 和 MinIO `shareo` bucket。

## 项目结构

```
ShareO/
├── cmd/
│   ├── server/main.go        # 入口：初始化 DB/Redis/MinIO/JWT → 启动 Gin
│   └── genhash/              # bcrypt 哈希生成工具（重置密码用）
├── internal/                 # Handler → Service → Repository 单向分层
│   ├── config/  model/  repository/  service/  handler/
│   ├── middleware/           # Auth / CrossOriginProtection / RateLimit / NoCache
│   ├── router/router.go      # 全部路由注册
│   ├── ws/                   # WebSocket Hub / Client
│   └── pkg/                  # jwt / response / upload / queue
├── ai-service/               # FastAPI + Worker + uv.lock + Dockerfile
├── web/
│   ├── templates/            # Go HTML 模板（auth/feed/post/user/admin/layout 分目录）
│   └── static/               # CSS / JS / 图片
├── migrations/               # SQL 迁移（001~010；002/004 不是自动结构迁移）
├── scripts/                  # API/IM 冒烟与辅助脚本
├── docs/                     # 文档体系（规范见 docs/README.md）
│   ├── architecture.md  roadmap.md  plan.md  standards.md  features.md
│   ├── phases/  adr/  design/  eval/  reviews/
├── config.yaml.example       # 配置模板（config.yaml 已 gitignore）
├── Makefile  start.sh        # 构建与一键启动
└── TASK.md                   # 任务看板
```

> `bin/`（构建产物）与 `resources/`（本地测试素材）不入版本库。

## 配置与安全

`config.yaml` 不入库；敏感值优先用环境变量注入：

| 环境变量 | 覆盖项 |
|----------|--------|
| `SHAREO_DB_PASSWORD` | 数据库密码 |
| `SHAREO_JWT_SECRET` | JWT 签名密钥 |
| `SHAREO_MINIO_ACCESS_KEY` / `SHAREO_MINIO_SECRET_KEY` | MinIO 凭证 |
| `SHAREO_REDIS_PASSWORD` | Redis 密码 |
| `SHAREO_TRUSTED_ORIGINS` | 逗号分隔的精确浏览器/WS origin |
| `SHAREO_CONFIG` | 可选的 YAML 配置路径；未设置时读取 `config.yaml` |
| `SHAREO_INTERNAL_TOKEN` / `SHAREO_AI_INTERNAL_TOKEN` | Go 与 AI 服务共享的内部令牌 |
| `SHAREO_AI_MODEL_CACHE_DIR` | Chinese-CLIP 模型缓存目录 |
| `SHAREO_AI_EMBEDDING_CONCURRENCY` | embedding 并发数，默认 1 |
| `SHAREO_AI_IMAGE_COLLECTION` | Qdrant 图片 collection，默认 `images` |

## 功能与设计

- **功能清单**（14 模块 × 分层文件索引）: [docs/features.md](docs/features.md)
- **数据库**: 14 张业务表（原 11 张 + conversations / conversation_members / messages）
- **关键设计决策**（v1 精选，完整版见 features.md 与 ADR）:
  - 审核流：新帖 pending → 管理员 approve/reject → Feed 仅展示 approved
  - Feed 缓存：Redis 仅存 ID 列表（2min TTL），命中后回源补全，写操作主动失效
  - 限流：Redis Lua **固定窗口计数**（登录 10/min、发帖 30/min、上传 20/min），Redis 不可用时 fail-open
  - 登录态：JWT + Redis 登录缓存 30min 滑动续期，支持单设备强踢
  - IM：REST 事务写消息，WebSocket 下行，精确未读，邀请制群组，`after_id` 断线恢复
  - 浏览器写安全：Go 标准库 CrossOriginProtection + 精确 trusted origins；退出为 POST
  - 上传：魔数检测 + Lanczos 三档缩略图（thumb 300 / medium 1200 / original）
  - 计数一致性：Toggle 类操作事务内"写记录 + COUNT 同步"双写

## 开发指南

- **开发规范**（分层规则 / Go / Python / API / Git / 测试 / 安全）: [docs/standards.md](docs/standards.md)
- **文档规范**（何时写什么，与论文章节的映射）: [docs/README.md](docs/README.md)
- **评审记录**: [docs/reviews/](docs/reviews/)（最新：2026-07-19 全量评审，24/24 项已修复）
- **CI**: GitHub Actions；本地门禁为 `make check`
- **AI 服务**: Python FastAPI API + Streams Worker: `ai-service/`

Phase 2 后端语义搜图通过 `GET /api/v1/search/images?q=...&limit=...` 提供 API。Go 负责图片代理和帖子可见性过滤，Python worker 只通过 Go 读取图片，不需要 MinIO 凭证。Go 与 ai-service 必须共享 `SHAREO_INTERNAL_TOKEN`；Compose worker 默认通过 `host.docker.internal:8080` 访问本机 Go 服务。

真实全链路验收使用固定隔离资源（`shareo_e2e`、Redis DB 15、`shareo-e2e`、`images-e2e`），执行 `make test-image-search-e2e`。脚本会等待模型预热，验证索引/搜索/删除、重复回填和 worker 重启后的 pending 重领，并只清理这些隔离资源。首次运行需提前准备约数 GB 的模型缓存空间。
- **测试**: 单元测试不依赖外部服务；真实 MySQL/Redis 用 `make test-integration`；API 行为见 `scripts/test_api.sh` 与 `scripts/test_chat.sh`
