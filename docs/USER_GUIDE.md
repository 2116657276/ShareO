# ShareO 用户使用指南

这是一份面向日常使用者的本机运行说明。项目默认使用 macOS 宿主机服务，不会由 `make up` 自动启动 Docker 或 Colima。

## 第一次运行

先准备 Go 1.25.1+、Python 3.12、`uv`、Homebrew PostgreSQL 17（含 pgvector）、Redis 和 MinIO。然后在项目根目录执行：

```bash
brew install postgresql@17 pgvector redis minio
brew services start postgresql@17
brew services start redis
cp config.yaml.example config.yaml
cp .env.example .env
make bootstrap-postgres
```

至少检查以下配置：

- `config.yaml` 中的 PostgreSQL、MinIO 和 JWT 配置是否适合当前本机环境；
- `.env` 中的 `SHAREO_INTERNAL_TOKEN` 是否为 Go 和 Python 共用的随机值；
- 需要使用 RAG、Agent 或真实模型评测时，再填写 `SHAREO_AI_LLM_API_KEY` 等 Provider 配置。

不要把 `.env`、API Key、Token、个人图片或模型权重提交到 Git。

## 日常启动与关闭

### 一键启动

```bash
make up
```

`make up` 会依次完成：

1. 检查并按需启动本机 PostgreSQL 17、Redis 和 MinIO；
2. 编译并启动 Go 服务和 Python AI 服务；
3. 预热图片模型、文本模型，并检查图片搜索、RAG、Agent readiness；
4. 通过后使用系统默认浏览器打开 `http://127.0.0.1:8080/home`。

不希望自动打开浏览器时使用：

```bash
SHAREO_OPEN_BROWSER=0 make up
```

启动成功后也可以手动访问：<http://127.0.0.1:8080>。

### 一键关闭

```bash
make down
```

默认会停止 Go、Python，以及由 ShareO 自己启动的 MinIO 进程；Homebrew 管理的 PostgreSQL、Redis、MinIO 等服务会保留运行，避免影响其他本机项目。需要同时关闭 Homebrew 服务时，明确执行：

```bash
SHAREO_STOP_BREW_SERVICES=1 make down
```

关闭不会删除 PostgreSQL、MinIO 或模型缓存中的数据。

## 常用 Make 命令

| 命令 | 作用 |
|---|---|
| `make doctor` | 检查 Go、Python、uv、本机依赖、端口、模型缓存和服务状态，不负责完整启动。 |
| `make logs` | 查看最近的 Go 和 AI 服务日志。 |
| `make local-infra-up` | 只启动 PostgreSQL、Redis 和 MinIO 等本机依赖。 |
| `make local-infra-down` | 停止由 ShareO 管理的 MinIO；Homebrew 服务默认保留。 |
| `make dev-local` | 启动本机依赖、Go 和 AI 服务，但不执行完整模型预热，也不自动打开网页。 |
| `make warm-ai` | 在服务已启动后预热模型并检查 AI readiness。 |
| `make check` | 执行 Go、Python、Shell 和文档检查；当前证据归档尚未完成三轮收口时，文档证据门禁会明确报告阻塞原因。 |
| `make test-api` | 执行 HTTP API、聊天和 Agent 边界回归；测试账号和测试对象会按脚本清理。 |
| `make test-integration-auto` | 使用本机 PostgreSQL/pgvector、Redis 执行隔离集成测试，临时数据库保留供检查。 |
| `make reconcile-index` | 只读检查 PostgreSQL/MinIO 与 pgvector 派生索引是否一致。 |
| `make reconcile-index APPLY=1` | 将对账差异应用到索引；执行前先确认当前数据库和对象存储属于 ShareO。 |

## 评测与测试

日常使用只需要 `make up` 和 `make down`；修改代码或准备答辩演示时，再按需执行：

```bash
make check
make test-api
make test-integration-auto
```

需要生成当前本机图片语料时：

```bash
make local-photo-seed CONFIRM=YES
make prepare-search-eval
make prepare-current-ai-eval VERSION=v2
```

`local-photo-seed` 会修改本机数据库、对象存储和向量索引，只在确认当前环境是 ShareO 专用环境时执行。完整 RAG/Agent 评测和 Final Freeze 证据归档属于开发审查流程，详见[评测规范](eval/README.md)和[工程审查](REVIEW.md)。

本机图片 seed 使用固定账号 `shareo_test`，密码只从 `SHAREO_TEST_PASSWORD` 读取；数据库的 `shareo_app`、`shareo_ai` 密码同样只从环境变量读取，当前本机值保存在未跟踪的 `.env`，不要复制到仓库。seed 会先快照并清空指定 MinIO bucket，再上传 `resources/static/pictures` 中的 47 张图片；完成后 bucket 只保留这批图片及其缩略图/中图派生对象，旧对象快照留在 `.local/shareo/local-photo-seed/`。

## 旧运行时

MySQL、Qdrant、Colima、OrbStack 和 Docker Compose 已退役，不再由当前 Make 目标启动。旧存储可以人工保留，但新运行时完全不读取；项目内旧 `qdrant/` 源码目录已在迁移验收通过后删除。

## 常见问题

### `make up` 在 AI readiness 处失败

先执行 `make logs`，确认模型缓存、PostgreSQL/pgvector 和 LLM Provider 配置。没有可用 Provider 时，社区页面和普通私聊代码仍可运行，但完整 `make up` 的 RAG/Agent readiness 检查不会通过。

### 页面打不开

执行 `make doctor`，再确认 `127.0.0.1:8080` 没有被其他程序占用。也可以使用 `curl http://127.0.0.1:8080/healthz` 检查 Go 服务。

### Bot 没有回复

先确认 `make up` 完成、RAG readiness 为 ready，并查看 `make logs`。Bot 任务由 PostgreSQL outbox 和 Redis Streams 异步处理，重启 Go 服务后会继续补发，不要手工重复发送同一条消息。

## 项目边界

ShareO 是本机优先的简历项目，不提供生产部署、高可用、压力测试、线上 SLA 或通用 Agent 能力。默认 Bot 使用 RAG；只有用户显式开启深度分析时才使用受限只读 Agent。项目范围和当前证据以[工程审查](REVIEW.md)为准。
