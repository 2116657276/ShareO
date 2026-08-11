# ShareO 高级工程审查与当前证据

> 审查基准日期：2026-08-11
>
> 本文面向简历项目审查、面试和项目交接，记录本轮发现的问题、已实施修复、当前指标和证据边界。历史 AI 质量报告来自迁移前运行；2026-08-11 的 PostgreSQL/pgvector、API 和本机数据集重验结果单独列出。

## 一、审查结论

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
| Python AI 服务 | 社区、普通私聊、关键词搜贴 | 语义搜图返回 503，Bot 任务进入等待或兜底 | AI readiness 恢复后继续处理 |
| PostgreSQL/pgvector | 社区、普通私聊、关键词搜贴 | 语义搜图和 RAG 受控不可用 | PostgreSQL/pgvector 恢复后 readiness 重新通过 |
| MinIO | 文字路径和正文 RAG | 图片代理失败，新图片索引暂停 | MinIO 恢复后重新处理索引任务 |
| Redis | PostgreSQL 业务写入和普通消息落库 | 缓存、实时推送和异步处理降级 | consumer 重建消费组并继续消费 |
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
