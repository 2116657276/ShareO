# ShareO 高级工程审查与当前证据

> 最新文档复核日期为 2026-11-23。2026-08-12 章节为历史审查快照。
>
> 本文面向简历项目审查、面试和项目交接，记录历史审查、当前事实和证据边界。当前状态与下一轮执行计划以 [`TASK.md`](../TASK.md) 为准；当前复核评分为 7.4/10。历史 AI 质量报告来自迁移前运行；2026-08-11 的 PostgreSQL/pgvector、API 和本机数据集重验结果单独列出。

## 2026-11-23 只读评审

本次以 `ffd195c` 和已有工作区为基线，沿启动脚本、认证、Bot 队列和会话列表检查正常入口。6 项 Go 定向测试、55 项 Python 单元测试和两个启动脚本的语法检查通过。下表记录修复前的状态，后续实测见 [OrbStack 验证](#2026-11-23-首轮优化与-orbstack-验证)。

| 优先级 | 发现与影响 |
| --- | --- |
| P1 | `SHAREO_APP_PORT` 已被脚本用于探活和默认回调地址，但 Go 监听端口仍从 YAML 读取。仅修改环境变量会造成监听与探活不一致。 |
| P1 | Bot 内部取任务或回调遇到 `401` 被作为永久失败，正常返回后 Redis consumer 会 ACK。未回复的 `published` outbox 仍占用用户额度。Redis 在发布后丢失 Stream 数据也有待处理记录悬挂的问题。 |
| P2 | 注册先创建账户，再向 Redis 写会话。Redis 写失败会返回通用 `503`，账户实际已经存在，用户按提示重试注册会得到用户名冲突。 |
| P2 | 会话、成员、最新消息与未读数已有批量查询。在线状态的本地 Hub 未命中后仍逐会话查 Redis，带引用的预览也会单独复核。基线需分别计数。 |

OrbStack 试跑复用 Go 与 Python 的镜像构建骨架及 PostgreSQL 初始迁移。旧 Compose 仍连接 MySQL/Qdrant。Go 镜像原构建上下文包含未排除的本机 `.env`、`.local` 和缓存目录，容器构建前须缩小上下文范围。

## 2026-11-23 首轮优化与 OrbStack 验证

Go 启动时校验并优先使用 `SHAREO_APP_PORT`，未设置时使用 YAML。API 与 Web 注册在账户已落库、会话写 Redis 失败时返回 `503` 和“账号已创建，登录服务暂不可用，请稍后直接登录”，且不设置会话 Cookie。Bot 取任务、回调和终止兜底遇到内部 `401` 时保留 Redis pending 条目供重领。

`ffd195c` 加本轮工作区改动通过 230 项 Go 测试、157 项 Python 非集成测试、`go vet`、Shell 语法和 Compose 配置验证。四项 Python 集成测试未纳入该批次。

独立 `shareo-orbstack` 项目在 OrbStack `linux/arm64` 启动 Go、Python、PostgreSQL 17.12（pgvector 0.8.7）、Redis 和 MinIO。空卷初始化退出码为 0，两个应用角色各自只能向所属 schema 插入。五个长驻服务报告健康，Go `/healthz` 和 Python `/readyz` 均返回 `200`。MinIO 镜像改用官方源码固定标签构建，避免依赖本次拉取被拒的旧镜像。操作见 [本地运行手册](operations/runbook.md)。

合成数据通过注册、上传 PNG、发帖审核、公开图片读取、私聊持久化、WebSocket 实时下发和中文语义搜图。单独禁止试跑 Redis 的 `SET` 后，API 与 Web 注册均返回约定的 `503`，恢复权限后两个已创建的账号均能直接登录。

正文与图片使用实际 embedding 模型写入 pgvector。中文正文查询返回审核通过的合成帖子，Bot 通过同网络固定响应服务生成带该帖子引用的回复。模型加载后，RAG 和 Agent readiness 均返回 `200`，两者报告 Bot consumer 正在运行。

故障验证覆盖以下场景。

- 暂时禁止 Redis `XADD`，观察 PostgreSQL outbox 保持 `pending` 且重试次数增长。恢复后任务发布并回复。对同一消息再次回调时，回复 ID 和内容保持一致。
- 使用不匹配的内部令牌重建 Python，确认 Bot 任务进入 Redis pending 且无回复。恢复令牌并重建 Worker 后，任务被重领，PostgreSQL 只保存一条带引用的回复。
- 重启五个服务后，帖子与 Bot 回复仍在 PostgreSQL，MinIO 代理返回的图片 SHA-256 一致，Redis AOF 保留测试前的 3 条 Bot Stream 记录。
- 分别验证管理员撤审和作者删除。两种操作后，对应帖子均从公开搜贴和历史回复返回的引用中消失，公开图片返回 `404`。

普通私聊列表的查询基线如下。对每种规模请求一次 `GET /api/v1/conversations`，用户未连接 WebSocket，列表中没有引用预览。PostgreSQL 计入请求窗口内的 Go 角色 `SELECT`（含认证，排除后台 Bot outbox 轮询），Redis 取该窗口内 `INFO commandstats` 的 `EXISTS` 差值。

| 返回会话数 | PostgreSQL `SELECT` | Redis `EXISTS` |
| --- | --- | --- |
| 1 | 8 | 1 |
| 10 | 8 | 10 |
| 100 | 8 | 100 |

实测覆盖本次普通会话列表的命令数。批量加载维持固定数量的 SQL；在线状态走 Redis 逐项检查，调用数随返回会话数增长。

固定响应服务只用于验证程序链路和引用权限。本轮未测真实 LLM 回答质量、检索评测指标、延迟与吞吐，也未切换 Final Freeze 的权威证据。Redis 已发布记录在整个 Stream 丢失时如何从 PostgreSQL 重放，仍待处理。

### 干净检出复验

从 `origin/orbstack` 的 `5888b63` 建立独立 worktree，支持提交 `d445a34` 只增加试跑参数化、固定响应服务 profile 和可重复执行脚本；业务代码未变。另起项目 `shareo-orbstack-clean-20261123` 和 `127.0.0.1:18090`，使用新生成的凭据、独立 PostgreSQL/Redis/MinIO/模型缓存卷，旧 `shareo-orbstack` 栈与 `18080` 未改动。

Go 和 AI 镜像由该提交构建；MinIO 复用同一提交此前构建的镜像，没有重新构建。模型缓存从已有卷复制，新卷约 1.5 GiB，未重新下载权重。五个核心服务健康，`postgres-init` 退出码为 `0`，固定响应服务健康。空库只有 `shareo_bot`，帖子、消息、outbox 和两张向量表均为 0；两个数据库角色无法跨 schema 写入。

合成 PNG 发帖在待审核时公开详情为 `404`、检索不可见、图片代理拒绝；审核后正文检索、语义搜图和引用回复均通过。禁用 Redis `XADD` 后 outbox 保持 `pending` 且重试增长，恢复后补发；重复回调返回相同回复 ID 和内容，数据库只有一条业务回复；错误内部令牌使任务留在 pending，恢复后重领并只提交一条带引用回复。最终 4 条 outbox 全部 `published`，4 个 `source_message_id` 各一条回复。

普通私聊列表在未连接 WebSocket、无引用预览、单用户顺序请求的条件下，每种规模预热 1 次后请求 20 次：1/10/100 个会话分别产生 21/210/2103 次 Redis `EXISTS` 和 0 次 `MGET`，中位耗时为 2.472/3.208/10.142 ms，p95 为 2.927/6.552/13.174 ms。按一条命令一次请求/响应估算，往返次数同量级；这不是抓包 RTT。完整步骤、条件和演示提纲见 [OrbStack 隔离演示](operations/orbstack-demo.md)。

本轮未验证真实 LLM 质量、并发吞吐、100 会话以上规模、Stream 整体丢失后的全量重放和最终冻结证据。

### 验收可靠性复验

审核基线仍为 `79b54f2`。本轮只修改 `scripts/orbstack_smoke.py`、运行手册和本文档，没有改变业务代码；PR #1 保持草稿。

脚本现在会在任何状态写入、HTTP 写请求、SQL 写入或 Redis ACL 修改前校验目标：base URL 必须是 loopback、端口必须等于指定 Compose 项目的 Go 发布端口、容器必须属于该项目、状态目录和卷前缀必须与项目同名，并拒绝原 `shareo-orbstack` 项目。演示阶段还要求 `ai-service` 的 LLM 地址精确指向 `http://llm-fixture:8000/v1`。实测错误端口、非 loopback 和原项目名均在副作用前失败。

新项目 `shareo-orbstack-fresh-20261123` 使用全新 PostgreSQL/Redis/MinIO 卷和从既有缓存只读复制的模型缓存，在 `127.0.0.1:18092` 完成空库初始化、审核检索、固定响应 Bot 引用、`XADD` 失败补发、重复回调、内部 `401` 重领和 presence 复验。五个核心服务健康，`postgres-init` 退出码为 `0`，fixture 健康且未发布宿主机端口。重复回调同时校验 HTTP 响应体和数据库唯一行；`401` 按本次 `source_message_id` 确认 Stream 条目仍 pending。

PR 基线使用 `EXISTS`。普通私聊列表在单用户、无 WebSocket、无引用预览、顺序请求条件下，每种规模预热 1 次后请求 20 次；命令计数与延迟使用同一正式采样窗口。1/10/100 个会话分别产生 20/200/2000 次 `EXISTS`、0 次 `MGET`，估算往返为 20/200/2000，中位耗时为 1.929/2.693/10.911 ms，p95 为 3.010/4.582/15.319 ms。该结果是基线复验，不是 `MGET` 优化收益。

未验证真实 LLM 质量、并发吞吐、100 会话以上规模、Stream 整体丢失后的全量重放和最终冻结证据。本地性能分支 `perf/chat-presence-batch`（`63b50d6`）保持独立，本轮未推送、未合并、未宣称延迟改善。

### 分支提交前复验

从 Git 暂存区导出不含本机数据和凭据的源码快照后，Go 格式检查、`go vet`、Go 测试和构建、Ruff、Shell 语法及示例环境的 `compose config --quiet` 均通过。Python 为 155 项通过、6 项跳过，其中 2 项依赖本地评测集、4 项依赖集成服务配置。文档结构检查通过，完整 Final Freeze 门禁仍报告权威运行指针、三轮评测和源码指纹三项缺口。

## 2026-08-12 全面工程与面试官复审（历史快照）

### 结论摘要

ShareO 已经明显超过普通课程 Demo：Go/Python 数据所有权、PostgreSQL outbox + Redis Streams、Bot 幂等回调、Go 最终可见性复核、受限只读 Agent 和版本化 AI 评测，足以体现中高级全栈工程能力。当时整体质量约为 **7.5/10**，已经具备简历项目的技术深度，但还没有达到可以不加说明地称为“完整生产级收口”的状态；当前复核评分见本文顶部摘要。

本次复审不改变项目范围，也不建议引入微服务、Kafka、SPA 或复杂计费系统。最需要优先处理的是安全边界和运行事实一致性：Feed 排序参数反射型 XSS、固定内部令牌与全接口监听、Redis 会话撤销 fail-open、公开图片绕过审核、图片解码/存储资源预算，以及搜索和 Bot 缺少跨请求额度。

截至本文“当前工作区的分阶段改进进度”一节所记录的状态，Feed XSS、固定令牌与默认全接口监听、Redis 基础设施故障 fail-open、持久化会话撤销、公开图片可见性、上传对象基础生命周期、像素解码风险、Bot 原子准入和 AI 搜索并发边界均已完成第一轮修复并有回归测试；搜索/聊天入口和 WebSocket 断线补偿也已补上基础边界。全局 LLM 计费额度、通知事务 outbox、构造注入的全面迁移、完整浏览器 E2E 和 Portfolio 资产仍是明确边界，不能把本轮修复夸大为完整生产级治理。

### 当前架构判断

浏览器通过 Go 服务端模板、Bootstrap、Alpine.js 和少量原生 JavaScript 访问 Gin 模块化单体；Go 负责认证、授权、审核、社区、聊天、WebSocket、`public` schema 和 MinIO；Python FastAPI 单进程负责 `ai` schema/pgvector、embedding、RAG、LangGraph 只读 Agent 和两个 Redis Streams consumer。Go 是业务数据唯一写者，Python 只写派生向量，MinIO 凭证不离开 Go。这种拆分与数据所有权和模型运行方式一致，当前规模不需要进一步拆分服务。

前端选择服务端渲染是合理的，但还没有形成 TypeScript 或完整浏览器构建/回归链；后端分层意图清晰，但大量 Service/Repository 仍创建全局 Repository 并依赖全局 DB/Redis，测试隔离性和错误契约的一致性不足。

### 主要优点

- Bot 任务具备事务 outbox、重试、重领、最终兜底、ACK 和 `source_message_id` 幂等，能够支撑事务边界、at-least-once 和故障恢复追问。
- Go/Python/MinIO 的信任边界清楚，Agent 只拥有语义检索、关键词检索、帖子读取和图片检索四类只读工具。
- 公开帖子与 Bot 引用由 Go 按 `approved AND is_deleted=0` 最终复核，聊天成员校验、Bot 回调关系校验和引用格式校验较完整。
- AI 评测不仅看回答文本，还记录 Recall、MRR、nDCG、no-match、引用精度、来源覆盖率、工具选择、注入拒答和延迟，并明确区分本机诊断结果与通用保证。
- README、ADR、API、数据模型、运行手册和证据边界比普通 Portfolio 项目完整，且没有把受限 Agent 夸大为通用或多 Agent 平台。

### 主要问题与面试风险

下面保留的是复审时的原始风险清单；方括号状态以当前工作区为准，详细改动和测试见“当前工作区的分阶段改进进度”。

#### P0：优先修复

1. **[已修复]** `web/templates/feed/feed.html` 将未枚举校验的 `sort` 放入 Alpine `x-data` JavaScript 字符串，HTML 实体解码后仍可突破表达式上下文，构成登录用户可触发的反射型 XSS。
2. **[已修复]** `scripts/local_runtime.sh` 在未配置时使用公开固定值 `shareo-local-internal`，而 `cmd/server/main.go` 默认监听 `:port`。局域网客户端可能带已知令牌访问 `/internal`，这与“本机优先”定位不构成充分安全边界。

#### P1：对质量和简历价值影响最大

1. **[已修复（基础会话治理）]** Redis 非 `redis.Nil` 错误时会话认证 fail-open；注销、改密和封禁撤销失败只记录日志，旧 JWT 仍可能访问 REST 写接口，且 REST 与 WebSocket 的 active 状态语义不一致。现在 Redis 故障 fail-closed，JWT 带 `session_version`，注销、改密和状态变更在 PostgreSQL 持久化递增版本，REST/WebSocket 复用同一校验；Redis 不是撤销事实源，但当前仍是受保护请求的会话可用性门槛。
2. **[已修复]** 公开 API 和私聊读取接口直接序列化完整 `model.User`，Email、Role、Status 等非必要字段会随 Feed、评论、关注、会话和消息响应暴露。社区与聊天读接口现在映射为安全 DTO，认证资料使用独立的 `AuthUser`。
3. **[已修复（基础生命周期）]** 图片上传先写入 MinIO，公开图片代理只按对象键读取，不绑定帖子审核/删除状态；缺少用户配额、上传 owner、未绑定过期和孤儿回收。现在上传记录带 owner、批次、过期时间和帖子绑定，发帖事务校验归属，启动后定期回收未绑定记录并删除对象，公开读取仍绑定 `approved AND is_deleted=0`；当前仍是有界 best-effort 回收，不承诺完整对象存储盘点或用户级存储配额。
4. **[已修复（基础准入）]** 公开搜贴/搜图会触发模型推理但没有有界排队；Bot 私聊当前有入口限流和每用户 pending 软上限，但仍没有严格原子额度、会话预算或全局 LLM 预算。现在语义搜贴/搜图共享进程级并发闸门，Bot outbox 在 PostgreSQL 事务内按用户 advisory lock 做原子 pending 上限；全局 LLM 计费、跨实例配额和长期队列仍不在项目范围。
5. **[已修复]** 文档声称 WebSocket 断线补偿，但客户端重连后没有按最后消息 ID 执行 `after_id` 补拉，存在消息间隙。
6. **[已部分修复]** 输入校验和错误映射分散在 Handler、Web 表单和 Service；认证、注册并发唯一冲突、密码修改和管理员写操作现在使用稳定业务错误并对未知错误返回通用响应；其余历史 Handler 仍保留少量面向用户输入的 `err.Error()`，构造注入和全局 Repository 清理尚未全面完成。
7. **[已处理文档边界]** 当前文档明确 PostgreSQL/pgvector 是本机唯一运行时，`compose.yaml` 与 `deploy/config.docker.yaml` 已标注 RETIRED；根目录和 `ai-service/` 的 Dockerfile 仍是未删除的构建材料，但不属于支持的启动入口，也没有在本轮执行。Compose 冷启动证据仍需用户授权后单独形成。另已将遗留 GORM 模型类型标签从 MySQL 专用 `enum/json` 对齐为 PostgreSQL 兼容的 `varchar/jsonb`，避免未来 schema 工具误生成错误 DDL。

#### P2/P3：可维护性、体验和展示层

- **[已部分修复]** Chat 会话列表已增加生产仓储的批量预览查询（成员、最后消息和未读数），并保留轻量仓储回退；全局 Repository/DB/RDB 和 `context.Background()` 使 Service 测试隔离性不足仍是长期债务。
- Feed 首屏模板和动态 JavaScript 卡片仍有重复；前端 fetch、错误状态和 loading 逻辑重复，模板内联脚本较多。
- **[已部分修复]** 设置页可以清空邮箱/简介，通知已读操作在服务端失败时不再乐观修改本地状态，个人页增加分页和基础 tab/布局可访问性，根级 MIT LICENSE 已补充；README 产品截图和公开演示数据仍未收口。
- Final Freeze 证据体系对审计有价值，但当前权威运行号、三轮评测和稳定源码指纹尚未收口，完整 `make check` 因此仍会被文档门禁阻塞。

### 面试官视角

最能体现工程能力的是事务 outbox 与 at-least-once 消费、Go/Python 数据所有权、引用二次校验、Agent 工具白名单和版本化 AI 评测。当前仍像 Demo 的部分是模板内联 JavaScript、部分管理内部响应仍直接使用实体、前端缺少真正的浏览器自动化回归，以及实际 PostgreSQL 运行时和遗留 Compose 配置不一致；社区和聊天读取接口的主要 DTO 泄露问题已在当前工作区收口。

面试官最可能追问：Redis 或 Python 故障时各链路如何降级；Bot 如何避免重复回复；帖子审核/删除后向量如何最终一致；为什么 Agent 没有写权限；RAG 引用为什么由 Go 再校验；WebSocket 断线如何补拉；为什么不用 React、Kafka 或微服务；以及为什么图片 URL 不能作为授权本身。

### 安全审阅摘要

本次标准安全审阅由两条独立静态审阅线完成，并由主审重新验证 source-to-sink 与反证，覆盖 Go、Python、模板、PostgreSQL、Redis、MinIO、内部令牌和本机启动路径。共确认 11 项高置信问题：2 项高、7 项中、2 项低。未发现可报告的 SQL 注入、聊天 IDOR、普通跨站 mutation 绕过、SSRF、任意工具调用或 Bot 跨会话回调。

完整机器生成报告和 canonical artifacts 位于本机安全扫描目录；本仓库只在本文记录稳定结论，不把临时目录路径当作项目契约。安全报告未执行容器或真实服务攻击测试，符合当前禁止 Docker/OrbStack 的约束。

### 本次基线检查（2026-08-12）

| 检查 | 结果 | 说明 |
|---|---|---|
| `go vet ./...` | 通过 | 与 `make check` 同步执行 |
| Go 单元测试 | 通过 | 所有 Go 测试包通过 |
| Ruff lint / format | 通过 | Python 60 个文件格式检查通过 |
| Python 非集成测试 | 通过 | 149 passed，4 个 integration tests deselected |
| Shell 语法检查 | 通过 | `start.sh` 与 `scripts`/`deploy` 脚本通过 |
| `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check` | 通过 | 代码、Python、Shell、测试门禁通过 |
| 原样 `make check` | 阻塞 | Final Freeze 权威运行号、三轮评测和稳定源码指纹尚未收口 |
| Docker/OrbStack/Compose | 未运行 | 用户明确禁止，后续需单独授权 |

### 当前工作区的分阶段改进进度（2026-08-12）

本轮已经按“先建立回归断言，再做最小实现”的方式完成了九个逻辑批次。以下状态只描述当前工作区已经落地并有测试覆盖的代码，不把尚未实现的长期治理项写成已完成：

| 批次 | 已实施内容 | 状态 |
|---|---|---|
| 安全与 HTTP 边界 | `sort` 白名单与非执行数据属性、随机内部令牌与 0600 本地状态文件、回环监听默认值、Gin 可信代理关闭、HTTP 超时与头部/请求体上限、搜索/聊天限流 | 已完成 |
| 会话与 API 契约 | Redis 会话基础设施错误 fail-closed 并返回 503；JWT 持久化 session version；认证资料使用 `AuthUser`，社区与聊天读接口使用安全 DTO/公共帖子、评论和通知读模型；认证/管理员未知错误不透传数据库原文；GORM 模型类型标签与 PostgreSQL 对齐 | 已完成（全局注入仍有边界） |
| 图片输入与生命周期 | 图片代理关联 `approved AND is_deleted=0` 帖子、对象键校验、`DecodeConfig` 尺寸/总像素预算、损坏图片拒绝、owner/批次/TTL/帖子绑定、定期回收和失败清理 | 已完成（配额与跨存储重试仍有边界） |
| 搜索、Bot 与实时体验 | 搜贴/搜图入口与 Bot 消息入口限流；共享 AI 并发闸门；PostgreSQL advisory lock 原子 pending 上限；WebSocket 重连按 `after_id` 补拉并去重 | 已完成（跨实例/计费额度仍有边界） |
| Chat 热点查询与前端状态 | 会话列表批量加载成员、最后消息和未读数；设置字段支持清空；通知已读失败不再错误更新本地状态 | 已完成（注入和脚本去重仍有边界） |
| 文档与最终验证 | 同步任务、审查、运行手册、数据模型和 API 说明；已完成本机全量代码检查、race、构建和最终整体复审 | 已完成 |

本轮新增或修改的回归覆盖包括 Feed 排序输入、内部启动契约、HTTP server 配置、Redis 会话故障、持久化 session version、缓存 fail-closed、公共用户/帖子/评论/聊天 DTO、可信代理、请求体上限、图片对象键/像素预算/生命周期、Bot pending 原子准入、AI 并发闸门、聊天批量预览，以及聊天页面的断线补偿契约。当前仍未处理跨实例额度与全局 LLM 预算、通知事务 outbox、全面构造注入、完整浏览器 E2E 和 README 截图/公开演示数据等问题；这些属于下一阶段，不应在面试中表述为已解决。

本轮已实际运行的验证为 `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check`、`go test -race -count=1 ./...`、`go build -o /tmp/shareo-server-check ./cmd/server`、Ruff lint/format、149 个 Python 非集成测试和 Shell 语法检查。原样 `make check` 仍只会被 `scripts/check_docs.py` 的 Final Freeze 权威运行号、三轮 AI 复评和稳定源码指纹门禁阻塞；这些既有证据收口问题不应通过修改实现代码绕过。Docker、Docker Compose 和 OrbStack 仍未运行。

### 分阶段改进计划

1. **已完成 Batch 1–5。** 安全边界、API DTO、图片输入、入口限流、WebSocket 补偿和文档/验证收口已落地。
2. **已完成 Batch 6。** JWT `session_version` 持久化撤销，补充 RowsAffected 检查和认证/管理员错误映射。
3. **已完成 Batch 7。** 上传 owner/批次/TTL、帖子事务绑定、定期回收和失败清理；仍不承诺全局对象盘点。
4. **已完成 Batch 8。** Bot 原子 pending 准入与语义搜贴/搜图共享并发闸门；跨实例和计费额度不在范围。
5. **已完成 Batch 9。** Chat 会话列表批量查询、设置字段清空、通知已读失败回滚和相关测试。
6. **后续只保留低优先级收口。** 全面构造注入、通知事务 outbox、完整可访问性、README 截图、公开演示数据，以及新的 Final Freeze 证据。
7. **Docker/Compose 验证。** 必须在代码改进和最终汇报完成后，由用户明确授权；本轮不执行任何 Docker、Docker Compose 或 OrbStack 操作。


## 一、当前状态摘要（截至 2026-08-28）

本节汇总当前代码边界、已有本机证据和未完成项；其中带具体运行号或历史日期的结果仍以当次环境为准，不代表本轮重新执行。

ShareO 已具备可用于简历展示的“Go + Python 可评测引用式 RAG 社区系统”基础。本轮最大风险集中在入口可验证性、异步任务耐久性、评论关系完整性、搜图拒答和评测口径，而不是功能数量；主要代码修复和自动化验证已完成，但当前证据仍受 dirty 工作区、评测版本切换和权威指针收口限制。项目包含图文社区、一对一实时私聊、中文语义搜图、正文混合搜贴、带可访问引用的 RAG Bot，以及同一私聊入口中的受限只读知识 Agent。

本轮结论基于当前代码、自动化测试、真实依赖集成、真实 Provider 机器评测、两轮 AI judge 和本机浏览器 smoke。AI judge 已完成，但 RAG 8 条、Agent 2 条答案因低分、低置信或来源问题进入人工复核交接包，不能写成“人工评分完成”或质量门禁通过；干净提交复评、空缓存冷启动、完整移动端/实时 WebSocket 浏览器自动化和 Compose 故障注入也未形成证据。

项目不宣称通用 Agent、多 Agent 系统、写入型或外部工具 Agent、生产级高可用、吞吐量上限、线上 SLA 或公共克隆可直接复现真实图片质量评测。

### 本次本机浏览器 smoke

2026-08-08 在本地原生运行时执行 `SHAREO_OPEN_BROWSER=0 make up`，使用新建普通测试账号完成注册登录、Feed、评论点赞、私聊未读徽标与清除、Bot 引用卡片、Agent 步骤轨迹、空状态和黑白/琥珀主题切换验收。详细摘要见 [人工浏览器 smoke 记录](evidence/final-freeze/manual-ui-check.md)。Codex 应用内浏览器尝试连接同一服务时受运行环境隔离影响，未形成自动化通过证据；因此保留人工浏览器 smoke，不宣称完整浏览器 E2E。

## 二、项目定位与范围

ShareO 是一个本地优先的 Go + Python 全栈项目。Go 主服务负责认证、权限、社区业务、聊天、WebSocket、PostgreSQL `public` schema 写入和对 AI 结果的最终可见性复核；Python FastAPI 单进程负责 embedding、PostgreSQL `ai` schema/pgvector、RAG、LangGraph Agent 以及 Redis Streams consumer。MinIO 凭证只由 Go 持有，Python 通过 Go 的图片代理读取图片。

保留范围包括注册登录、资料、图文发布与审核、Feed、点赞、关注、评论、通知、私人收藏、一对一私聊、中文混合搜贴、中文语义搜图、RAG Bot 和受限只读 Agent。转帖、话题、群聊、写入型 Agent、外部工具、长期记忆、多 Agent、独立 Bot 页面、音视频和生产级多租户不在范围内。

默认 `shareo_bot` 消息仍使用固定 RAG 流程。用户显式开启深度分析后，才进入 LangGraph `StateGraph`；Agent 只能使用语义检索、关键词检索、帖子读取和图片检索四类只读工具。

## 三、关键端到端链路

### 1. 图文发布、审核与检索索引

```text
用户发布图文
  → Go 写入 PostgreSQL，状态为 pending
  → 管理员审核通过
  → Go 发布 index_post Redis Stream 任务
  → Python consumer 获取 Go 的严格索引载荷
  → Chinese-CLIP 写入 ai.image_embeddings，FastEmbed 写入 ai.post_chunk_embeddings
  → PostgreSQL pgvector 提供图片/正文向量检索
  → Go 对公开结果执行 approved AND is_deleted=0 复核
  → 页面返回 Feed、混合搜贴或语义搜图结果
```

正文搜贴融合 BGE/pgvector 语义候选与 PostgreSQL `LIKE` 关键词候选；搜图只使用图片向量，两类结果不交叉计分。编辑、驳回和删除会通过幂等事件清理或更新派生索引。

### 2. 普通私聊与 RAG Bot

```text
用户发送私聊消息
  → Go 在 PostgreSQL 事务中落消息并写入 bot_task_outbox
  → 单进程 outbox publisher 重试发布 Redis Streams bot_tasks
  → Python 调用 Go 内部接口获取严格 Bot 上下文
  → BGE/pgvector 检索 approved 正文并调用 LLM
  → Python 生成回答候选和引用
  → Go 再次校验会话、来源消息和帖子可见性
  → Go 事务写入 Bot 回复
  → WebSocket 下行消息，客户端按 message ID 去重
```

Bot 使用 `source_message_id` 等幂等约束处理 at-least-once 投递；失败时经过受控重试后返回固定兜底，不影响普通私聊和社区写入。

### 3. 显式只读 Agent

```text
搜索工作台“问问小O”
  → 打开 shareo_bot 专属会话并设置 ai_mode=agent
  → LangGraph StateGraph 选择只读工具
  → semantic_search / keyword_search / read_posts / search_images
  → Go 内部接口返回已做可见性校验的观察结果
  → Agent 继续检索或生成最终总结
  → 输出脱敏步骤轨迹和结构化引用
  → Go 复核引用并通过既有 Bot 消息链路返回
```

Agent 不获得 PostgreSQL `public`、MinIO 或 pgvector 写权限，不保存长期记忆，不展示思维链、完整 Prompt 或原始工具参数。默认 `ai_mode=rag` 不进入 Agent 状态图。

### 4. 故障降级与恢复

| 故障依赖 | 仍保持可用的路径 | 受控降级 | 恢复方式 |
|---|---|---|---|
| Python AI 服务 | 社区、普通私聊、关键词搜贴 | 语义搜图返回 503，Bot 任务进入等待或兜底 | 对应 AI readiness 恢复后继续处理 |
| PostgreSQL 业务库 | 依赖数据库的社区读取、私聊和写入失败 | 不应伪装为 HTTP 200；outbox 也无法持久化 | PostgreSQL 恢复并重新建立连接 |
| `ai` schema/pgvector（业务库仍正常） | 社区、普通私聊、关键词搜贴 | 语义搜图、RAG/Agent 受控不可用 | AI 数据库恢复后 readiness 重新通过 |
| MinIO | 文字路径和正文 RAG | 图片代理失败，新图片索引暂停 | MinIO 恢复后重新处理索引任务 |
| Redis | PostgreSQL 公开读在依赖正常时可继续 | 受保护 REST/WS 会话校验 fail-closed；outbox 发布、实时推送和异步处理延迟 | Redis 恢复后登录校验、consumer 和 publisher 恢复 |
| LLM Provider | 社区、普通私聊和非 LLM 搜索 | Bot/RAG 失败后返回固定兜底 | Provider 恢复并重新预热 |

## 四、数据与历史结果

下表中的结果绑定本次报告运行号、当前 47 条本机帖子和真实 Provider；RAG/Agent 数字来自 v1 输入，不代表线上质量、通用模型能力或第三方评测结论。旧的 26 条 Demo 冻结输入和旧报告已经退役，不再作为当前测试输入。

| 数据或结果 | 数量/指标 | 证据等级 | 说明 |
|---|---:|---|---|
| 当前本机帖子与图片 | 47 条 | B/C | 当前 manifest、索引状态和本次运行参数见 [Final Freeze](evidence/final-freeze/README.md) |
| 当前搜图开发评测 | 38 条 | C | Recall@5 `0.9222`、Recall@10 `1.0`、MRR `0.9750`、no-match accuracy `1.0`；公开 Go 链路使用阈值 `0.39` |
| 当前搜贴开发评测 | 32 条 | C | hybrid Recall@5 `0.8906`、MRR `0.9688`、nDCG@5 `0.9158` |
| 当前报告 RAG 机器评测 | 30 条 v1 | C | 完整来源覆盖率 `0.9333`、引用精度 `0.9000`、引用可访问率 `100%`、额外引用 `7`、虚假引用 `0`、P95 `5.13s` |
| 当前报告 Agent 机器评测 | 36 条 v1 | C | 完整来源覆盖率 `0.9722`、引用精度 `0.9583`、必需工具选择率 `1.0`、禁止/额外工具 `0`、注入拒答失败 `0`、P95 `8.17s` |
| 本次 AI judge（待人工复核） | RAG 30 / Agent 36 | C | RAG 平均 `4.1667`、最低 `1`、8 条待复核；Agent 平均 `4.8056`、最低 `1`、2 条待复核；模型、Prompt SHA 和逐题理由见 [质量汇总](evidence/final-freeze/quality-summary.json)，待复核条目见 [人工复核交接包](evidence/final-freeze/ai-judge-human-handoff.json) |

## 五、证据等级说明

- **A：代码事实。** 可以从当前代码、数据库 schema、路由契约或稳定设计文档直接确认的行为，例如服务边界、API 路径、权限校验和工具白名单。
- **B：仓库归档的工程证据。** 包括单元测试、race、真实依赖集成、跨服务 E2E、故障矩阵和已保存的执行记录。
- **C：本机或历史评测结果。** 结果绑定特定数据集、模型、设备、环境或人工评分方式，适合说明当次验证，不适合外推为线上结论。
- **D：未形成可复核归档的声明。** 可以作为演示或口头说明的背景，但不应写成最终发布证据或性能承诺。

当前评测报告中的 `git_dirty=true`、运行结果与干净提交不完全绑定等限制，属于 C 级本机证据的适用边界。搜图 no-match 指标依赖当前 Go API 的 `0.39` 置信度阈值，不应外推为跨模型拒答阈值。

## 六、已接受的证据缺口

以下内容是本轮仍未形成的证据或能力边界：

1. 当前报告记录了实际运行时模型、数据集 SHA、Git SHA、dirty 状态和 dirty diff 指纹，但没有绑定到干净提交，不能据此宣称干净提交上的最终复评。
2. 当前报告已经记录 RAG 的分段耗时可取得性、Agent 工具步骤和 embedding/retrieval/LLM 阶段可取得性；Agent 单独 LLM 样本、队列等待、postprocess 和 callback 没有可靠字段时明确为 `unavailable`，不能外推完整链路性能。
3. 已有浏览器 smoke 覆盖登录、评论、未读、引用卡片和主题，但没有完整移动端、实时 WebSocket 下行和全量交互演示归档。
4. 空模型缓存、全新公共克隆和源码 Compose 冷启动没有形成完整可复核证据；已有验证可能复用了镜像或模型缓存。
5. 项目没有压力测试，因此不对吞吐量、并发用户数、容量上限、成本或线上 SLA 作承诺。
6. 当前 47 条本机真实图片不进入 Git，公共克隆无法直接复现完整图片质量评测。
7. RAG/Agent 的 AI judge 已完成，但 10 条低分、低置信或来源异常答案尚需人工复核；AI 指标不等同于人工或第三方评审结论。

### 2026-08-11 PostgreSQL/pgvector 本机迁移

本轮按全新本机图片数据集方案完成 Homebrew PostgreSQL 17 与 pgvector 迁移：Go GORM 改用 PostgreSQL，Python 用 Psycopg 3 异步连接池访问 `ai` schema；旧 MySQL/Qdrant/Compose 启动路径退役。受 `CONFIRM=YES` 保护的 seed 已在本机目标环境执行：快照并清空 MinIO bucket 后上传 47 张图片，创建 47 条自动审核帖子和对应图片/文本向量；最终核对确认固定 Bot/测试用户、47 条可见帖子、互动/聊天/outbox/日志为空，图片代理、Feed、语义搜图和对账均通过。图片文案清单保存在未提交的 `.local/shareo/local-photo-seed/captions.json`，按 SHA-256 幂等复用。

### 2026-08-11 网络中断后重验

网络恢复后重新执行了本机原生运行时和数据集验收。`make test-api` 的社区/API 回归为 48/48，聊天、默认 RAG、显式只读 Agent 和内部鉴权回归为 15/15；`SHAREO_RUNTIME=local bash scripts/test_integration_current.sh` 的 Go PostgreSQL 集成全部通过，Python Redis/pgvector 集成 4/4 通过；`go test -race -count=1 ./...` 与 `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check` 均通过。测试期间产生的临时用户、帖子、对象和聊天记录已按精确范围清理，未进入最终数据集。

### 2026-08-12 最终复审与验证补充

最后一轮针对“PostgreSQL/pgvector 是唯一运行时”重新核对 GORM 模型声明，发现并修正 4 个遗留的 MySQL 专用类型标签：枚举字段改为 `varchar`，消息结构化字段改为 `jsonb`，并新增模型标签回归测试，避免未来 schema 工具生成错误 DDL。复审没有新增 P0/P1 阻塞项；跨实例/计费额度、通知事务 outbox、全面依赖注入、完整浏览器 E2E、README 展示素材和 Final Freeze 权威证据仍按 P2/P3 边界保留。

本次最终本机验证结果：`SHAREO_SKIP_CURRENT_EVIDENCE=1 make check` 通过；`go test -race -count=1 ./...`、`go build -o /tmp/shareo-server-check ./cmd/server`、`go mod verify` 和 `git diff --check` 通过。未跳过证据门禁的 `python3 scripts/check_docs.py` 仍只报告三项已知 Final Freeze 缺口：权威运行指针未切换、当前归档不足三轮 AI 复评、源码指纹尚未稳定；没有出现新的代码、测试或文档结构错误。本轮没有执行 Docker、Docker Compose 或 OrbStack。

最终重验状态如下：业务库只有 `shareo_test` 与 `shareo_bot` 两个用户，47 条帖子全部由固定用户发布并满足 `approved AND is_deleted=0`，47 条 `post_images` 一对一对应；`ai.image_embeddings` 与 `ai.post_chunk_embeddings` 各 47 条，向量列均为 `vector(512)`，两张 HNSW `vector_cosine_ops` 索引存在；MinIO bucket 有 141 个本次对象（47 个原图及派生对象），图片代理、Feed、语义搜图和对账通过；评论、点赞、收藏、关注、通知、聊天、Bot outbox 和系统日志均为空；项目内 `qdrant/` 目录已删除。固定密码只通过未跟踪 `.env` 注入，仓库未写入凭据。

### 2026-08-10 纠正与验收

本次复核修复了四项可由代码路径和回归断言确认的问题。Agent 对敏感请求现在丢弃模型生成内容并返回固定拒答，同时清除该请求的引用；公开评论列表与精选评论查询都加入帖子 `approved AND is_deleted=0` 联结条件；源码指纹工具和仓库忽略规则共同排除 Playwright CLI 生成的 `.playwright-cli/`；Final Freeze 采集器会保留失败运行的完整包，但只有严格质量门禁通过时才推进 `authoritative_run_id`。

Agent 非集成测试、证据工具测试和 Python 全量测试在迁移前通过；PostgreSQL/pgvector 单测、真实 PostgreSQL 集成、MinIO 清空上传和 47 条 API seed 已在本机完成，密码只保存在未跟踪的 `.env`。

这些缺口表示“证据没有归档或不能外推”，不表示相应业务链路在代码中不存在。现有功能、工程测试和历史运行记录仍以对应证据文件为准。

## 七、简历与答辩口径

### 推荐表述

> 独立设计并实现 Go + Python 的可评测引用式 RAG 社区系统：Go 负责 PostgreSQL 业务、权限和最终可见性校验，Python 负责 Chinese-CLIP/BGE + pgvector 向量检索、RAG 和 LangGraph 只读 Agent；通过 PostgreSQL outbox + Redis Streams 实现 Bot 任务的事务耐久和异步幂等处理，并使用带 no-match 的版本化评测数据集、工程测试和工具/引用安全门禁验证检索质量。

### 面试中应主动说明

- 默认 Bot 是固定流程 RAG；只有用户显式开启深度分析后，才进入受限只读 Agent。
- Agent 只有语义检索、关键词检索、帖子读取和图片检索四类工具，没有写入、外部调用、长期记忆或多 Agent 能力。
- 引用先由 Python 生成候选，再由 Go 按 `approved AND is_deleted=0` 做最终复核。
- 评测结果是特定本地数据集和历史运行环境下的证据，不延伸为线上 SLA、通用准确率或第三方评测。

### 不应使用的表述

“通用 Agent 平台”“自主多 Agent 系统”“生产级高并发”“已完成源码一键复现”“第三方评测达到 5 分”均超出当前证据范围。

## 八、证据入口

- [项目架构](architecture.md)：服务关系、数据所有权和降级矩阵。
- [功能矩阵](features.md)：稳定功能、可见性和非目标范围。
- [API 参考](reference/api.md)：公开、内部和 AI 接口边界。
- [AI 评测规范](eval/README.md)：数据集、指标、门禁和评测限制。
- [简历项目审查](resume-review.md)：简历可用表述和历史证据限制。
- [Final Freeze 自动化证据归档](evidence/final-freeze/README.md)：本机服务、真实 Provider、报告中的 v1 38/32/30/36 评测、AI judge、人工复核交接包和 dirty 运行汇总；Compose E2E、浏览器自动化和人工评分明确标记边界。
