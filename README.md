# ShareO — 摄影社区 + AI 能力演进中

Go Web 摄影作品管理与社区分享平台（类小红书产品形态），当前作为毕业设计向 v2 演进：**IM 私聊群组 / 图片语义搜索 / RAG Bot / Agent**。

- **架构与决策**: [docs/architecture.md](docs/architecture.md) · [docs/adr/](docs/adr/)
- **里程碑与详细计划**: [docs/roadmap.md](docs/roadmap.md) · [docs/plan.md](docs/plan.md)
- **当前任务**: [TASK.md](TASK.md)

## 技术栈

| 层级 | 现状 (v1) | v2 增量（规划见 ADR） |
|------|-----------|----------------------|
| 后端 | Go 1.25 + Gin 1.12 | + gorilla/websocket（IM） |
| 数据 | MySQL 8.0 (GORM) + Redis + MinIO | + Qdrant（向量库）+ Redis Streams（队列） |
| AI 服务 | — | Python 3.12 + FastAPI + uv；Chinese-CLIP / BGE 本地 embedding；LLM 云 API |
| 认证 | JWT (golang-jwt/v5) + bcrypt + Redis 登录缓存 | 复用 |
| 前端 | Go Templates + Bootstrap 5.3 + Alpine.js | 复用（不引 SPA） |
| 配置 | Viper + `SHAREO_*` 环境变量覆盖 | + `SHAREO_AI_*` |

## 快速开始

```bash
git clone <repo-url> && cd ShareO
cp config.yaml.example config.yaml   # 填入本地 MySQL/Redis/MinIO 配置
./start.sh                           # 检测并拉起 MySQL/Redis/MinIO → 编译 → 启动
curl http://localhost:8080/healthz   # → {"status":"ok","service":"ShareO"}
```

> 环境要求: Go 1.25+ / MySQL 8.0 / Redis / MinIO。Phase 0 已提供 `deploy/docker-compose.yml` 一键编排（含 Qdrant），推荐以 compose 为首选路径。

常用命令：`make help`（start / run / build / migrate / seed / reset-db / clean）。

## 项目结构

```
ShareO/
├── cmd/
│   ├── server/main.go        # 入口：初始化 DB/Redis/MinIO/JWT → 启动 Gin
│   └── genhash/              # bcrypt 哈希生成工具（重置密码用）
├── internal/                 # Handler → Service → Repository 单向分层
│   ├── config/  model/  repository/  service/  handler/
│   ├── middleware/           # Auth(4层) / RateLimit / NoCache
│   ├── router/router.go      # 全部路由注册
│   └── pkg/                  # jwt / response / upload 工具包
├── web/
│   ├── templates/            # Go HTML 模板（auth/feed/post/user/admin/layout 分目录）
│   └── static/               # CSS / JS / 图片
├── migrations/               # SQL 迁移（001~008，新迁移递增编号）
├── scripts/                  # read_config.py / test_api.sh 等辅助脚本
├── docs/                     # 文档体系（规范见 docs/README.md）
│   ├── architecture.md  roadmap.md  plan.md  standards.md  features.md
│   ├── adr/  design/  eval/  reviews/
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

## 功能与设计

- **功能清单**（14 模块 × 分层文件索引）: [docs/features.md](docs/features.md)
- **数据库**: 11 张表（users / posts / post_images / comments / likes / favorites / follows / topics / topic_posts / notifications / system_logs）
- **关键设计决策**（v1 精选，完整版见 features.md 与 ADR）:
  - 审核流：新帖 pending → 管理员 approve/reject → Feed 仅展示 approved
  - Feed 缓存：Redis 仅存 ID 列表（2min TTL），命中后回源补全，写操作主动失效
  - 限流：Redis Lua **固定窗口计数**（登录 10/min、发帖 30/min、上传 20/min），Redis 不可用时 fail-open
  - 登录态：JWT + Redis 登录缓存 30min 滑动续期，支持单设备强踢
  - 上传：魔数检测 + Lanczos 三档缩略图（thumb 300 / medium 1200 / original）
  - 计数一致性：Toggle 类操作事务内"写记录 + COUNT 同步"双写

## 开发指南

- **开发规范**（分层规则 / Go / Python / API / Git / 测试 / 安全）: [docs/standards.md](docs/standards.md)
- **文档规范**（何时写什么，与论文章节的映射）: [docs/README.md](docs/README.md)
- **评审记录**: [docs/reviews/](docs/reviews/)（最新：2026-07-19 全量评审，24/24 项已修复）
- **CI**: GitHub Actions（`make check` 本地等价）: `.github/workflows/go.yml`
- **AI 服务**: Python FastAPI 微服务（Phase 0 脚手架完成）: `ai-service/`
- **测试**: 业务行为测试一律走 API 模拟（见 `scripts/test_api.sh`），禁止直改 SQL/Redis/MinIO 构造状态
