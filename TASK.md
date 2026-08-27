# ShareO 当前状态

> 更新时间：2026-08-28 | 状态：Batch 1-9 核心改进、PostgreSQL/pgvector 迁移和本机验证已完成；文档审计已完成一轮；复审后的下一轮优化计划已建立，阶段 0 待执行

## 2026-08-12 复审与改进进度

本次评审确认项目具备 Go + Python 可评测引用式 RAG 社区系统的简历展示基础；**7.5/10 是 2026-08-12 历史审查评分，不是当前评分**。事务 outbox、Redis Streams、Bot 幂等、Go 最终可见性复核、受限只读 Agent 和版本化 AI 评测是主要工程亮点；当前剩余风险集中在跨实例/计费额度、通知事务 outbox、全面注入和 Portfolio/证据收口。

已完成：

- 将完整架构、面试官视角、P0-P3 问题、安全审阅摘要、基线检查和分阶段计划写入 [`docs/REVIEW.md`](docs/REVIEW.md)。
- 完成标准安全静态审阅：确认 11 项高置信问题（2 高、7 中、2 低），未发现 SQL 注入、聊天 IDOR、普通 CSRF 绕过、SSRF 或任意工具调用。
- 运行当前基线：`go vet`、Go 测试、Ruff、149 个 Python 非集成测试和 Shell 检查通过；带 `SHAREO_SKIP_CURRENT_EVIDENCE=1` 的完整代码检查通过，原样 `make check` 仍只被 Final Freeze 历史证据门禁阻塞。
- **Batch 1：安全与 HTTP 边界。** Feed `sort` 改为白名单并移出可执行 Alpine 表达式；本地运行时生成随机内部令牌并以 0600 保存，Go 默认只监听回环地址；HTTP 服务增加超时、请求头/请求体上限，并关闭未配置时的可信代理；搜索和聊天入口加入基础限流。
- **Batch 2：会话与 API 契约。** Redis 会话基础设施故障改为 fail-closed 并映射为 503；认证资料使用私有 `AuthUser`，Feed、帖子、评论、关注、收藏、通知和聊天读取接口统一映射为安全 DTO/公共读模型，避免把持久化用户模型直接暴露给客户端；新增 DTO 回归测试。
- **Batch 3：图片输入与可见性。** 图片代理必须关联已审核、未删除帖子；上传在完整解码前校验尺寸和总像素预算，拒绝损坏或超预算图片，并降低并发解码压力。
- **Batch 4：搜索、Bot 与实时体验。** 公开搜贴/搜图和 Bot 私聊消息加入入口限流；Bot 触发增加每用户 pending 任务软上限；WebSocket 重连后按最后消息 ID 使用 `after_id` 补拉并去重，降低消息间隙风险。

后续已完成：

- **Batch 5：前端可靠性与验证收口。** 已实现 `after_id` 断线补偿，修复设置清空、通知已读失败回滚，并同步本轮实现状态、实际检查结果和剩余边界；当前不运行 Docker、Docker Compose 或 OrbStack。
- **Batch 6：持久化会话与稳定错误契约。** JWT 增加 `session_version`，注销、改密和封禁递增 PostgreSQL 版本；认证与管理员写操作不再把未知数据库错误透传给客户端，并补充 RowsAffected 检查。
- **Batch 6 补充：PostgreSQL 模型标签。** 将遗留 GORM `enum/json` 类型标签改为 PostgreSQL 兼容的 `varchar/jsonb`，避免未来 schema 工具误生成 MySQL DDL。
- **Batch 7：上传对象生命周期。** 上传记录包含 owner、批次、过期和帖子绑定；发帖事务校验对象归属，启动后定期回收未绑定记录并删除对象，注册失败执行补偿清理。
- **Batch 8：Bot 与 AI 准入。** Bot outbox 在同一 PostgreSQL 事务内按用户 advisory lock 执行原子 pending 上限；语义搜贴和搜图共享进程级 AI 并发闸门。
- **Batch 9：聊天热点查询。** 会话列表批量加载成员、最后消息和未读数，生产仓储避免按会话放大的查询模式，同时保留轻量仓储回退路径。

剩余边界：

1. 跨实例用户/会话额度、全局 LLM 计费预算和通知事务 outbox；
2. 全面构造注入、完整键盘可访问性和模板脚本去重；
3. README 产品截图、公开演示数据，以及新的 Final Freeze 权威证据；
4. Compose/Docker 冷启动和故障注入验证必须等待用户明确授权。

Docker、Docker Compose 和 OrbStack 在上述改进完成、适用检查通过、最终整体 review 和状态汇报之前均不运行；即使静态判断适合验证，也需要用户另行明确授权。

本轮按 [工程审查](docs/REVIEW.md) 完成入口修复、业务完整性、Bot outbox、评测门禁、搜图拒答阈值、Agent 安全拒答、本机浏览器 smoke，以及 Homebrew PostgreSQL 17 + pgvector 迁移。项目不做 Docker 交付、生产部署、高可用、压测或容量承诺。

已完成的关键修复包括：恢复 `start.sh` 与被删除的 Agent 数据集完整性测试；统一私聊未读字段为 `data.total`；校验评论父级同帖、可见和一级关系并级联软删除；帖子正文、图片数量和上传 URL 服务层校验；消息与 Bot outbox 同事务落库、启动补发和失败退避；登录后吊销旧 WebSocket；搜图默认置信度阈值；RAG/Agent 完整来源覆盖、引用精度、工具审计和注入拒答指标。

2026-08-10 纠正了本轮审查确认的四项问题：敏感请求现在只返回固定拒答，不再拼接未经验证的模型内容；评论列表和精选评论均复核所属帖子为 `approved AND is_deleted=0`；`.playwright-cli/` 被纳入忽略和源码指纹排除范围；失败或未达严格门禁的 Final Freeze 运行仍会保留，但不会替换既有权威运行指针。新增了对应的 Agent、证据工具和 PostgreSQL 集成回归测试。

2026-08-11 完成数据库方言替换：Go GORM 使用 PostgreSQL，Repository 移除 MySQL 全文/排序/自增方言；Python 通过 Psycopg 3 异步池使用 `ai.image_embeddings` 与 `ai.post_chunk_embeddings`，均为 `vector(512)` + HNSW cosine；Compose、MySQL、Qdrant 启动路径退役。新增受 `CONFIRM=YES` 保护的 MinIO 快照/清空和 47 张图片 seed，文案按 SHA-256 固化在未提交的 `.local/shareo/local-photo-seed/captions.json`。

## 当前基线

- 当前本机语料：47 条帖子、38 条搜图（含 8 条 no-match）、32 条搜贴、30 条 RAG、36 条 Agent。
- 历史 AI 报告运行号：`20260808T151704Z`；该报告的搜图、RAG、Agent 结果绑定 v1 数据集 SHA256、模型版本和运行时版本。它是在本次 PostgreSQL/pgvector 迁移前采集的诊断证据，不能当作当前迁移后的三轮复评。
- 当前结果是特定本机语料和真实 Provider 下的诊断证据，不代表线上 SLA、通用准确率或第三方评测。
- 2026-08-11 网络中断后重新验收：完整 `make test-api` 通过社区/API 回归 48 项和聊天、RAG、Agent 回归 15 项；`SHAREO_RUNTIME=local bash scripts/test_integration_current.sh` 通过 Go PostgreSQL 集成与 Python Redis/pgvector 集成 4 项；`go test -race -count=1 ./...` 和 `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check` 均通过。完整 `make check` 仍会因历史 Final Freeze 权威运行号、三轮 AI 复评和稳定源码指纹未收口而阻塞，这不影响本次迁移验收。
- 2026-08-12 最终复审补充：修正遗留 GORM `enum/json` 类型标签为 PostgreSQL 兼容的 `varchar/jsonb`，并新增模型标签回归测试；`SHAREO_SKIP_CURRENT_EVIDENCE=1 make check`、`go test -race -count=1 ./...`、服务构建、`go mod verify` 和 `git diff --check` 均通过。原样文档门禁仍只剩 Final Freeze 权威运行指针、三轮 AI 复评和稳定源码指纹三项已知证据缺口；未运行 Docker、Docker Compose 或 OrbStack。
- 当前本机最终数据：`shareo_test`、`shareo_bot` 两个用户；47 条 `approved AND is_deleted=0` 帖子且每条恰有一张图片；图片/正文向量各 47 条、均为 `vector(512)`，HNSW `vector_cosine_ops` 索引 2 个；评论、点赞、收藏、关注、通知、聊天、Bot outbox、系统日志均为 0；MinIO 当前 141 个对象（47 个原图及派生缩略图）；项目 `qdrant/` 目录已删除。固定测试用户密码和数据库密码只存在未跟踪的本机 `.env`，未写入仓库。

## 本轮剩余边界

干净提交复评、空缓存源码冷启动、完整移动端/实时 WebSocket 浏览器自动化、10 条 AI judge 待复核题的人工结论和压力测试尚未完成。AI 评分不表述为人工评分，dirty 工作区结果也不表述为发布复现结果。

## 证据入口

- [工程审查](docs/REVIEW.md)
- [用户使用指南](docs/USER_GUIDE.md)
- [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)
- [当前评测规范](docs/eval/README.md)
- [当前运行记录](docs/eval/experiments.md)
- [AI judge 人工复核交接包](docs/evidence/final-freeze/ai-judge-human-handoff.json)

## 2026-08-28 文档审计记录

本次只做文档和示例配置清理，没有修改业务代码、数据库数据或历史证据文件。已完成的校对包括：统一 Redis 会话的真实故障语义；拆分 PostgreSQL 业务库与 `ai`/pgvector 故障矩阵；标明 `/readyz/rag`、`/readyz/agent` 尚未覆盖 `bot_tasks` consumer 的已知缺口；补充 `SHAREO_APP_PORT` 与 `config.yaml.server.port` 的当前不一致限制；将旧 MySQL E2E 配置示例标记为 RETIRED；纠正历史 smoke 中已替换的迁移文件名；为 Final Freeze、简历审查和 Batch 1 设计/计划增加只读历史标识。

完整 `python3 scripts/check_docs.py` 仍会按设计报告三项 Final Freeze 证据缺口（权威运行指针、三轮 AI 复评、稳定源码指纹）；这不是本次文档链接或过时引用校对失败。带 `SHAREO_SKIP_CURRENT_EVIDENCE=1` 的文档检查通过，历史证据目录继续保留且不被伪装成当前发布结论。

## 2026-08-28 复审后优化执行计划（细化版）

本节是下一轮开发的主计划和状态事实源，依据当前工作区代码、配置、测试及[工程审查](docs/REVIEW.md)重新核对。上一轮 2026-08-12 的 Batch 1–9 记录保留为历史，不在本节重复排队；本节只安排当前仍有价值、尚未完全收口的工作。

### 计划约定与当前判断

- `[x]` 表示已有实现和证据，`[~]` 表示已有基础但仍有明确缺口，`[ ]` 表示尚未开始。
- 当前复审评分为 7.4/10；没有确认的 P0 阻塞项。P1 优先解决运行事实、认证失败语义和 AI 就绪误报，P2 再处理演进、维护性与交付证据。
- 每个任务必须同时补充实现、直接相关测试、必要文档和验收记录；接口或配置变化必须同步 README、`docs/USER_GUIDE.md` 或 API 参考。
- Docker、Docker Compose 和 OrbStack 仍不是默认验收路径。现有 Compose 已退役并有文档说明，不需要为完成本计划擅自删除、迁移或启动它；若要恢复容器交付，另行取得授权并建立独立计划。
- 当前工作区已有大量未提交修改。执行新任务前先记录基线，完成后只报告本轮新增的变更和证据，不把历史 dirty 结果描述为干净提交结果。

### 当前状态快照

| 领域 | 当前状态 | 已有基础 | 下一步缺口 |
|---|---|---|---|
| 运行入口 | `[~]` | 原生 PostgreSQL 17 + pgvector、Redis、MinIO 路径已记录，旧 Compose 已标记退役 | `SHAREO_APP_PORT` 与 Go 实际配置覆盖仍需统一，文档需做一次端到端验证 |
| 认证会话 | `[~]` | JWT `session_version`、Redis 故障 fail-closed、DTO 和错误映射已有实现 | 注册/登录写库成功后 Redis 缓存失败的客户端契约仍需定稿并测试 |
| AI Worker | `[~]` | 两个 Streams consumer 和运行时状态字典已存在 | `/readyz/rag`、`/readyz/agent` 尚未把 Bot consumer 纳入就绪判断 |
| API 与数据 | `[~]` | PostgreSQL 约束、DTO、上传生命周期、Bot outbox 和主要输入限制已存在 | 迁移执行记录、幂等状态 API、统一分页/错误契约和跨存储补偿仍未完成 |
| 前端 | `[~]` | SSR、Bootstrap/Alpine、自定义主题、响应式和基础可访问性已存在 | 内联脚本、帖子卡片双渲染来源和 CSS 覆盖层次仍影响维护 |
| 工程交付 | `[~]` | Go/Python/Shell 检查、race、集成和 AI 评测脚本已存在 | CI、SCA、统一日志指标和 Final Freeze 三轮复评尚未闭环 |

### 阶段 0：运行基线与交付入口收口（P1，阶段 1 的前置）

目标是让任何后续修复都建立在同一个可运行、可复查的入口上。

- [x] `BASE-01` 已在 `TASK.md`、`docs/REVIEW.md` 和 Final Freeze 入口记录当前工作区、运行环境、主要测试、数据集和证据缺口。每次后续代码改动前仍需追加新的基线记录。
- [x] `BASE-02` 已确定原生 PostgreSQL 17 + pgvector、Redis、MinIO 是当前支持路径，MySQL/Qdrant/Compose 仅作退役历史资料；本任务不再要求移动或删除文件。
- [ ] `BASE-03` 统一 `SHAREO_APP_PORT`、`SHAREO_SERVER_HOST`、`SHAREO_AI_PORT`、数据库配置和 Go/Python 内部令牌的加载规则。至少验证默认端口、一个非默认端口、直接启动 Go 和脚本启动四种组合。
- [~] `BASE-04` README、用户指南和 Make 目标已有启动/停止/检查说明，但需要围绕端口、AI readiness、失败清理和测试数据写一次实际链路校对，删除相互矛盾的示例。

阶段验收：按文档启动时，健康检查访问的端口必须等于 Go/AI 实际监听端口；默认和非默认配置都能完成健康检查；仓库中只有一个被标记为支持的本地入口。

本阶段收益是降低首次启动和现场演示失败率，为后续证据冻结建立可信基线。主要风险是旧脚本使用者受到配置变化影响，因此只做兼容性迁移说明，不删除数据或历史配置。

### 阶段 1：关键正确性、API 契约与安全边界（P1）

目标是处理异常依赖、重试、错误输入和公网部署前最容易暴露的问题；已有基础不重复实现，只补缺口。

- [~] `AUTH-01` 以 PostgreSQL `session_version` 作为撤销事实来源，明确 Redis 是强会话层还是可失效快速层。优先设计“用户已写库但缓存失败”时的注册/登录契约，再补 Redis 故障、重复注册、注销、改密和封禁测试；不得通过静默改变会话语义来掩盖故障。
- [~] `AI-01` 修正 `/readyz/rag` 和 `/readyz/agent`，同时检查 `index_post` 与 `bot_tasks` 两个 consumer；为停止、重启、重领、重试和恢复分别保留可复核测试。
- [~] `API-01` 在现有 DTO、上传和文本限制基础上，统一路径 ID、邮箱/URL/长度、枚举、分页上限和数据库错误映射；查询错误不能伪装为空列表，客户端不能看到数据库原文。
- [ ] `API-02` 将点赞、关注、评论点赞等状态操作收敛为幂等 `PUT/DELETE` 或带幂等键的命令；统一列表接口的 `page/page_size/meta`，限制深分页，并保留现有收藏接口行为不回归。
- [ ] `SEC-01` 确定 Go 或受控边缘层的安全响应头与 CSP 方案；发布模式拒绝已知占位密钥和开发默认凭证，并验证模板内联脚本/事件不会绕过新策略。
- [~] `SEC-02` 现有请求体、图片、搜索和 Bot 限制继续保留；补充 WebSocket 连接上限、跨请求资源预算和故障日志脱敏。全局 LLM 计费额度属于边界项，除非引入真实成本场景，否则只记录设计和测试假设。

阶段验收：自动化覆盖 Redis 不可用、任一 AI consumer 停止、重复请求、非法参数、超深分页和超限上传；公开写操作的 HTTP/业务错误契约稳定；就绪探针不会报告停止的关键 consumer；安全响应头和敏感配置策略有代码或运行证据。

本阶段收益是消除数据语义不一致、假健康和异常场景失控。主要风险是认证语义、CSP 和状态 API 会影响现有页面，必须先补契约测试，再逐步切换。

### 阶段 2：数据演进、跨存储一致性与后端结构（P2）

目标是让后续迭代可控，而不是引入超出项目规模的基础设施。

- [~] `DATA-01` 当前已有 `001–004` PostgreSQL 脚本，但缺少迁移执行记录、校验和和升级测试；补齐版本化执行入口，并验证从上一版数据逐步升级和重复执行。
- [~] `DATA-02` 上传 owner/批次/TTL 和基础回收已经存在；将通知发送、MinIO 删除失败和未绑定对象清理纳入轻量补偿记录，支持重试、状态查询和最终对账。
- [~] `ARCH-01` 新代码优先构造器注入并贯穿 `context.Context`，逐步替换全局 DB/Redis/Repository；消除 Handler 直接访问 Repository 的例外，不为旧代码做无关大重构。
- [~] `PERF-01` 会话列表已有成员、最后消息和未读数批量加载；补充游标/上限、批量在线状态和慢查询基线，再决定是否调整计数策略。
- [ ] `PERF-02` 为 Feed、聊天、搜索和 Bot 增加最小查询计数、P95 延迟、队列积压和 consumer 状态观测；只优化有测量证据的热点。

阶段验收：旧数据可按迁移序列升级，重复执行安全；外部副作用失败可重试且有状态；核心 Service 可替换依赖；会话和搜索查询有页大小、查询次数或延迟基线。

本阶段收益是降低数据升级、故障补偿和长期维护成本。主要风险是迁移与最终一致性增加排障复杂度，必须保留备份、幂等键和对账方式，禁止借机扩展业务范围。

### 阶段 3：前端可维护性与产品完成度（P2）

目标是保留 SSR + Bootstrap + Alpine 的合理方向，只治理重复和交互模块的维护成本，不进行全量 React/Vue 重写。

- [ ] `FE-01` 将 Feed、聊天、图片搜索和设置页的内联状态机拆为 ES Module，统一请求、loading、empty、error 和重试处理。
- [ ] `FE-02` 确定帖子卡片单一渲染来源，整理 CSS 主题变量、断点和覆盖顺序，删除失效规则；不改变已经验收的视觉主题。
- [ ] `FE-03` 只有在模块拆分确实需要类型和构建能力时，才在高交互区域引入 TypeScript/轻量构建链；SSR 页面仍应可独立理解和运行。
- [~] `FE-04` 已有基础跳转、焦点态、响应式和人工 smoke；补齐键盘路径、移动视口、实时 WebSocket 下行、引用点击和错误态的自动化回归。

阶段验收：关键页面不再依赖大段内联状态机；帖子卡片字段变更只需修改一个渲染契约；桌面/移动、键盘和异常状态均有回归记录；CSP 策略不会被新增内联代码重新破坏。

本阶段收益是保持已有商业化 UI 的同时降低迭代成本。主要风险是构建链增加本地步骤和 SSR/客户端边界复杂度，因此只在高交互模块渐进引入。

### 阶段 4：质量证明、可观测性与最终冻结（P2/P3）

目标是让质量声明能被第三方按文档复现，而不是只依赖本机口头演示。

- [ ] `ENG-01` 建立 CI：Go 格式、`go vet`、单测、race、构建；Python locked 测试与 Ruff；Shell、文档和敏感文件检查。
- [ ] `ENG-02` 增加依赖安全扫描和最小覆盖率基线；明确安全扫描、集成测试、浏览器 smoke、AI 评测的必跑门禁与定时任务边界。
- [ ] `OBS-01` 统一 Go/Python 结构化日志、请求/任务关联 ID、关键延迟、队列积压、consumer 状态和 Provider 错误指标；确认不记录 token、密码、Prompt 或完整用户内容。
- [~] `EVAL-01` 历史和本机 AI 评测脚本已存在；在干净提交上完成当前版本三轮评测，固定数据集 SHA、模型/Prompt 版本和源码指纹，并完成待复核题的人工结论。
- [~] `EVAL-02` 已有启动、测试和边界文档；补齐最小可复核演示包、无个人敏感素材的演示数据说明、关键 API/浏览器路径和故障降级证据。

阶段验收：干净工作区执行 `make check` 通过；测试、评测和文档产物可定位到同一源码/数据集版本；没有报告支持的性能、并发、成本或 SLA 不写入简历或发布文档。

本阶段收益是把工程能力转化为可信证据。主要风险是 CI、浏览器和 AI 评测的时间/偶发性增加，使用隔离数据、确定性 mock 和定时任务控制不稳定因素。

### 依赖顺序与完成定义

主关键路径为：`BASE-03/04 → AUTH-01 + AI-01 + API-01 → DATA-01/02 + ARCH-01 → FE-01/02 → ENG-01 + EVAL-01`。`SEC-01` 可与阶段 1 并行，`PERF-02` 可在阶段 2 后半段开始；`FE-03` 是条件性任务，不是强制的技术栈迁移。

任务只有同时满足“实现完成、直接相关测试通过、文档同步、无临时敏感产物、验收记录可定位”五项条件，才能从 `[~]` 或 `[ ]` 更新为 `[x]`。阶段完成后在本节补充日期、执行命令和证据入口，不用删除历史记录或覆盖失败运行。

本计划不改变 ShareO 的产品范围：继续保留 Go 业务单体、Python AI 派生服务、PostgreSQL/pgvector、Redis Streams、SSR 前端和受限只读 Agent；不新增群聊、转帖、写入型 Agent、外部工具、多 Agent 或生产级高可用承诺。
