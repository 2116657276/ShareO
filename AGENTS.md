# AGENTS.md

本文件用于指导 AI 编程代理在 ShareO 中进行代码修改。开始工作前，先阅读
`README.md`、`TASK.md`、`docs/REVIEW.md` 以及与目标模块相关的文档；
不要仅根据文件名或任务描述猜测现有行为。

## 项目原则

- 优先保证正确性、可维护性、可验证性和现有功能不回归。
- 修改范围保持最小，只处理当前任务必需的内容，不顺便重构无关代码。
- 优先修复根因，复用已有实现和依赖，避免不必要的抽象、框架和第三方依赖。
- 任务描述与代码、测试或事实文档冲突时，先确认实际行为；以明确需求和可验证事实为依据，并说明冲突。
- 不恢复已经从最终范围移除的转帖、话题、群聊、写入型/外部工具 Agent 或独立 Bot 页面。私人收藏与同一 Bot 私聊中的只读 Agent 属于经 ADR 明确批准的范围例外。

## 架构与设计边界

- Go 是业务运行时和 PostgreSQL 的唯一写者，负责认证、权限、审核、社区、聊天和 Bot 回复。
- Python AI 服务通过同一 PostgreSQL 实例的 `ai` schema 写入 pgvector 派生数据，负责模型、向量检索、RAG 管线和两个 Redis Streams consumer；保持单 FastAPI 进程、单 Uvicorn worker 的设计。
- MinIO 凭证只由 Go 持有。Python 通过 Go 的图片代理读取图片，不直接获取对象存储凭证。
- 所有公开帖子和 Bot 引用都必须经过 Go 的可见性校验：`approved AND is_deleted=0`。
- Redis Streams 按 at-least-once 语义处理；保持现有重试、重领、最终记录并 ACK 的策略，不擅自引入死信队列。
- Go 主服务遵循 `Handler → Service → Repository → DB/Redis` 单向依赖。Handler 不直接访问 GORM，Service 不依赖 `gin.Context`，Repository 不承载业务判断；WebSocket Hub 只负责连接管理和消息投递。
- 新增或修改的 Repository、Service 方法优先以 `context.Context` 作为第一个参数；新 Service 使用构造函数注入依赖。
- Python 路由当前由 `app/main.py` 承载；其余代码遵循 `core/ → rag/ → workers/` 的职责边界。对外数据结构使用 Pydantic v2 model，模型加载保持懒加载和进程内单例。

分层、API、安全和测试约束以本文件、实际代码和 [`docs/adr/`](docs/adr/) 为准。

## 编码与接口规范

- Go 代码必须通过 `gofmt` 和 `go vet`；错误必须处理或显式记录，跨层错误使用 sentinel error 与 `errors.Is`，不要匹配错误字符串。
- Go 日志使用 `log/slog` 结构化输出，统一使用 `user_id`、`post_id`、`conv_id`、`err` 等键，不输出 token、密码哈希或完整 SQL。
- Python 固定使用 3.12 和 uv；依赖变更同步更新 `pyproject.toml` 与 `uv.lock`，检查使用 locked/frozen 模式。
- API 使用 `/api/v1/...`、复数资源名和统一分页参数；服务端必须校验并限制分页输入。对外错误不得泄露数据库或内部错误原文。
- 涉及写记录与同步计数的操作使用同一事务；GET 请求不得产生副作用；用户输入入库前必须校验长度、格式和枚举值。
- 新增敏感配置通过 `SHAREO_*` 或 `SHAREO_AI_*` 环境变量注入，并同步更新示例配置或文档；不得硬编码凭证。
- 注释只解释原因、限制和非显然行为，不保留注释掉的旧代码、临时调试代码或无意义日志。

## 禁止事项

- 不覆盖、回退或删除用户已有的未提交修改；修改前先检查工作区状态。
- 未经明确要求，不提交、推送、创建分支、修改提交历史或执行外部协作操作。
- 不提交 `config.yaml`、`.env`、私钥、令牌、密码、模型权重、二进制产物以及个人照片或数据。
- 不擅自升级依赖、混用 npm/pnpm/yarn/bun 等工具，或引入已有依赖和标准库可以替代的新依赖。
- 不修改生成文件；除非任务明确要求，不大规模调整目录、命名、格式或迁移策略。
- 不执行以下破坏性操作，除非用户明确授权且已确认目标：

  ```text
  git reset --hard
  git clean -fd
  git push --force
  rm -rf
  DROP DATABASE
  ```

- 不为了让测试通过而降低断言质量、删除有效测试或伪造评测结果；不把浏览器手工验收当作后端测试证据。

## 测试与交付

- 修改后先运行与改动直接相关的测试；核心逻辑需要覆盖正常路径、边界条件和失败路径。
- 在成本合理时运行 `make check`；并按影响范围补充 `make test-integration`、`go test -race ./...`、`make test-api` 或 `make eval-ai`。
- AI 效果通过 `docs/eval/` 中的版本化评测数据集和指标验证，不用普通单测替代质量评测；并发、队列和幂等逻辑必须有工程测试。
- 无法执行某项检查时，明确记录未运行的命令、原因和替代验证方式；不得声称运行过未实际运行的命令。
- 只有在需求实现、相关验证完成或明确说明限制、无明显临时文件和敏感信息、必要文档同步后，才可报告任务完成。
- 当前状态只更新 `TASK.md`，当前证据和边界写入 `docs/REVIEW.md` 与 `docs/evidence/final-freeze/`；技术选型写入 ADR，功能变更同步 `docs/features.md`。
