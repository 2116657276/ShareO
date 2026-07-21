# ShareO 开发规范

> 更新时间: 2026-07-21 | 状态: 生效 | 文档规范另见 [docs/README.md](README.md)

规范只写"会被检查的"：能进 `make check` 的进 `make check`，进不了的写成评审清单。

## 1. 分层与依赖规则（Go 主服务）

```
Handler → Service → Repository → DB/Redis
   │          │
   └──────────┴──→ pkg/（jwt/response/upload/queue/aiclient，工具不含业务）
```

- 单向依赖，禁止反向；Handler 不碰 GORM，Service 不碰 `gin.Context`，Repository 不含业务判断。
- **v2 新代码两条硬规矩**（老代码不强改，改到哪补到哪）：
  1. Repository/Service 新方法第一个参数是 `context.Context`；
  2. 新 Service 用构造函数注入依赖（`NewChatService(repo, hub, queue)`），不在方法里 `repository.NewXxx()`。
- WebSocket Hub 归 `internal/ws`，只做连接管理与投递，不写业务。

## 2. Go 编码规范

- **格式**: gofmt 强制，`make check` 里 `gofmt -l` 非空即失败。不额外引 linter，`go vet` 必须干净。
- **错误处理**: 错误必须处理或显式记录，禁止 `_` 丢弃（评审 SR-06 教训）；包装用 `fmt.Errorf("...: %w", err)`；跨层判定用 sentinel error（`service.ErrPostNotFound` 模式）+ `errors.Is`，不做字符串匹配。
- **日志**: `log/slog` 结构化输出，键统一 `user_id` / `post_id` / `conv_id` / `err`；禁止在日志里输出 token、密码哈希、完整 SQL。
- **命名**: 与现有代码保持一致（Repo 后缀、Service 后缀、Handler 后缀；表名蛇形复数）。
- **数据库**: 涉及"写记录 + 同步计数"的操作必须同一事务（like_repo.Toggle 为标准模板）；新表一律在 migrations/ 递增编号，迁移必须可重复执行（`IF NOT EXISTS` 等）。

## 3. Python 编码规范（ai-service）

- **工具链**: Python 固定 3.12；uv dependency group 管理 dev 依赖（lock 文件提交）；CI 和检查用 locked/frozen；`ruff check` + `ruff format` + `pytest` 必须可离线跑。
- **类型**: 全量 type hints；对外数据结构一律 pydantic v2 model，不裸传 dict。
- **结构**: `api/`（路由）→ `core/`（模型/存储客户端）→ `rag/`（管线）→ `workers/`（消费者）；模型加载懒加载 + 进程内单例。
- **配置**: pydantic-settings 读 `SHAREO_AI_*` 环境变量，禁止硬编码密钥/地址。

## 4. API 设计规范

- 路径 `/api/v1/...`；资源复数；动作用 POST 子路径（`/posts/:id/like`）。
- 分页参数统一 `page` / `page_size`，**服务端 clamp（1 ≤ page，1 ≤ page_size ≤ 50）**，响应统一 `PageResponse`（评审 SR-01 教训：任何除法前保证分母合法）。
- 业务错误码分段：`0` 成功；`1001~1007` 现有通用段（1007 为 conflict）；未来模块扩展码在 `response` 包集中定义并注释。
- 对外错误信息不得包含 DB/内部错误原文（SR-07 教训）；handler 层统一兜底话术，详情进日志。
- **内部接口** `/internal/*`：仅内网监听 + `X-Internal-Token` 校验；不复用对外错误码，直接 HTTP 语义。
- 游标分页用于消息流：`before_id` 向前分页，`after_id` 断线补偿，两者互斥；偏移分页用于普通列表页。

## 5. Git 规范

- **提交信息**: Conventional Commits——`feat:` / `fix:` / `docs:` / `refactor:` / `test:` / `chore:`，subject 中文可；修复评审项带编号（`fix: 搜索分页除零 panic (SR-01)`）。
- **粒度**: 一个逻辑变更一个提交；修复清单逐项提交，不打包。
- **分支**: 日常直接 main（单人项目）；每个 Phase 的大改动开 `phase1-im` 风格分支，完成后合回；main 时刻保持 `make check` 绿。
- **禁止入库**: config.yaml、密钥、二进制产物（bin/）、个人照片/数据（resources/）、模型权重文件。

## 6. 测试规范

- 新功能最低要求：Service 层核心逻辑单测 + `scripts/test_*.sh` API 冒烟用例。
- 后端成熟前，终端 curl/Shell 是正式行为验收方式：使用 `scripts/test_api.sh`、`scripts/test_chat.sh` 覆盖登录、资源写入、IM 权限/未读/恢复等流程；它不能替代发布前的浏览器可用性与双会话验收，但不会被视为低于手工点击的证据。
- 浏览器与前端验收安排在接口和数据一致性稳定后执行，避免把页面不稳定性混入后端基线；暂缓项必须记录在 `TASK.md` 和对应设计文档，不能被误标为完成。
- 涉及并发的代码（ws Hub、队列消费）必须过 `go test -race`。
- 无外部服务的门禁统一 `make check`；真实 MySQL/Redis 放 `make test-integration`。该目标强制要求 `SHAREO_TEST_MYSQL_DSN`（库名以 `_test` 结尾）与 `SHAREO_TEST_REDIS_URL`，避免把 skip 算作通过。
- AI 效果不写断言式单测，走 `docs/eval/` 评测集 + 指标（见 [eval/README.md](eval/README.md)）；工程部分（编码维度、upsert 幂等、检索 top-k 形状）写常规单测。
- 修 bug 先写复现用例再修（能自动化的自动化，不能的记入手测清单）。

## 7. 安全规范

- 密钥只经 `SHAREO_*` / `SHAREO_AI_*` 环境变量注入；新增敏感配置同步更新 config.yaml.example（占位符）与 README 表格。
- 新端点默认挂 `AuthRequired`，例外（公开读）需在路由处注释理由。
- 所有用户输入进 DB 前有校验（长度/格式/枚举），进模板渲染依赖自动转义，禁止新增 `template.HTML` 直出（现有 renderHashtags 是唯一豁免，改它需评审）。
- 全站浏览器 POST/PUT/PATCH/DELETE 由 Go 标准库 `http.CrossOriginProtection` 保护，不新增 Token 库或 bypass pattern；CLI/Bearer/internal 客户端保持标准库默认兼容。
- WebSocket Origin 只接受同源或 `server.trusted_origins`/`SHAREO_TRUSTED_ORIGINS` 中的精确 origin；release 禁止通配符。
- GET 必须无副作用；登出、浏览计数等写行为使用 POST。
- 写操作保持既有限流覆盖；Web 表单与 API 同源同限（SR-05 教训）。
- 上线部署前检查单：改默认管理员密码、`mode: release`、HTTPS、内部服务不暴露公网。

## 8. 何时写文档（联动 docs/README.md 三条硬规矩）

选型 → 先 ADR；Phase 开工 → 先设计文档；功能合入 → 更新 features.md；实验 → 当天记 eval。提交里包含行为变化的，检查 README/features 是否需要同步——文档滞后是 v1 的主要债务来源。
