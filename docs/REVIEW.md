# ShareO 完工冻结审查

> 审查基准日期：2026-07-30
>
> 本文面向毕业答辩、面试和项目交接，记录 ShareO 的完工状态、核心链路、已有证据和已接受的证据缺口。本文不代表继续整改计划，也不改变业务代码、模型、Prompt、数据集或检索参数。

## 一、完工结论

ShareO 的功能实现已经完成，当前作为“Go + Python 可评测引用式 RAG 社区系统”冻结。项目包含图文社区、一对一实时私聊、中文语义搜图、正文混合搜贴、带可访问引用的 RAG Bot，以及同一私聊入口中的受限只读知识 Agent。

本项目的完工结论基于代码、自动化测试、真实依赖集成、当前语料评测和故障降级证据。部分发布级证据没有在当前工作区形成可复核归档，包括干净提交上的最终复评、完整浏览器人工演示和空缓存源码冷启动。这些属于证据边界，不改变已经实现的功能范围，也不作为继续整改事项。

项目不宣称通用 Agent、多 Agent 系统、写入型或外部工具 Agent、生产级高可用、吞吐量上限、线上 SLA 或公共克隆可直接复现真实图片质量评测。

## 二、最终定位与范围

ShareO 是一个本地优先的 Go + Python 全栈项目。Go 主服务负责认证、权限、社区业务、聊天、WebSocket、MySQL 写入和对 AI 结果的最终可见性复核；Python FastAPI 单进程负责 embedding、Qdrant、RAG、LangGraph Agent 以及 Redis Streams consumer。MinIO 凭证只由 Go 持有，Python 通过 Go 的图片代理读取图片。

保留范围包括注册登录、资料、图文发布与审核、Feed、点赞、关注、评论、通知、私人收藏、一对一私聊、中文混合搜贴、中文语义搜图、RAG Bot 和受限只读 Agent。转帖、话题、群聊、写入型 Agent、外部工具、长期记忆、多 Agent、独立 Bot 页面、音视频和生产级多租户不在范围内。

默认 `shareo_bot` 消息仍使用固定 RAG 流程。用户显式开启深度分析后，才进入 LangGraph `StateGraph`；Agent 只能使用语义检索、关键词检索、帖子读取和图片检索四类只读工具。

## 三、关键端到端链路

### 1. 图文发布、审核与检索索引

```text
用户发布图文
  → Go 写入 MySQL，状态为 pending
  → 管理员审核通过
  → Go 发布 index_post Redis Stream 任务
  → Python consumer 获取 Go 的严格索引载荷
  → Chinese-CLIP 写入 images，FastEmbed 写入 post_chunks
  → Qdrant 提供图片/正文向量检索
  → Go 对公开结果执行 approved AND is_deleted=0 复核
  → 页面返回 Feed、混合搜贴或语义搜图结果
```

正文搜贴融合 BGE/Qdrant 语义候选与 MySQL ngram FULLTEXT/LIKE 关键词候选；搜图只使用图片向量，两类结果不交叉计分。编辑、驳回和删除会通过幂等事件清理或更新派生索引。

### 2. 普通私聊与 RAG Bot

```text
用户发送私聊消息
  → Go 在 MySQL 事务中落消息并创建 bot_tasks
  → Redis Streams 投递给 Python Bot consumer
  → Python 调用 Go 内部接口获取严格 Bot 上下文
  → BGE/Qdrant 检索 approved 正文并调用 LLM
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

Agent 不获得 MySQL、MinIO 或 Qdrant 写权限，不保存长期记忆，不展示思维链、完整 Prompt 或原始工具参数。默认 `ai_mode=rag` 不进入 Agent 状态图。

### 4. 故障降级与恢复

| 故障依赖 | 仍保持可用的路径 | 受控降级 | 恢复方式 |
|---|---|---|---|
| AI service | 社区、普通私聊、关键词搜贴 | 语义搜图返回 503，Bot 任务进入等待或兜底 | AI readiness 恢复后继续处理 |
| Qdrant | 社区、普通私聊、关键词搜贴 | 语义搜图和 RAG 受控不可用 | Qdrant 恢复后 readiness 重新通过 |
| MinIO | 文字路径和正文 RAG | 图片代理失败，新图片索引暂停 | MinIO 恢复后重新处理索引任务 |
| Redis | MySQL 业务写入和普通消息落库 | 缓存、实时推送和异步处理降级 | consumer 重建消费组并继续消费 |
| LLM provider | 社区、普通私聊和非 LLM 搜索 | Bot/RAG 失败后返回固定兜底 | provider 恢复并重新预热 |

## 四、数据与历史结果

下表中的结果绑定本次运行号、当前 47 条本机帖子和真实 Provider，不代表线上质量、通用模型能力或第三方评测结论。旧的 26 条 Demo 冻结输入和旧报告已经退役，不再作为当前测试输入。

| 数据或结果 | 数量/指标 | 证据等级 | 说明 |
|---|---:|---|---|
| 当前本机帖子与图片 | 47 条 | B/C | 当前 manifest、索引状态和本次运行参数见 [Final Freeze](evidence/final-freeze/README.md) |
| 当前搜图开发评测 | 34 条 | C | Recall@5 `0.9222`、Recall@10 `1.0`、MRR `0.9750`，见 [质量汇总](evidence/final-freeze/quality-summary.json) |
| 当前 RAG 机器评测 | 30 条 | C | 来源命中率 `0.8333`、引用可访问率 `100%`、虚假引用 `0`，见 [质量汇总](evidence/final-freeze/quality-summary.json) |
| 当前 Agent 机器评测 | 36 条 | C | 来源命中率和必需工具选择率 `1.0`、引用可访问率 `100%`、安全门禁均为 `0`，见 [质量汇总](evidence/final-freeze/quality-summary.json) |
| 当前搜贴开发评测 | 32 条 | C | hybrid Recall@5 `0.8906`、MRR `0.9688`、nDCG@5 `0.9158`，见 [质量汇总](evidence/final-freeze/quality-summary.json) |

## 五、证据等级说明

- **A：代码事实。** 可以从当前代码、数据库 schema、路由契约或稳定设计文档直接确认的行为，例如服务边界、API 路径、权限校验和工具白名单。
- **B：仓库归档的工程证据。** 包括单元测试、race、真实依赖集成、跨服务 E2E、故障矩阵和已保存的执行记录。
- **C：本机或历史评测结果。** 结果绑定特定数据集、模型、设备、环境或人工评分方式，适合说明当次验证，不适合外推为线上结论。
- **D：未形成可复核归档的声明。** 可以作为演示或口头说明的背景，但不应写成最终发布证据或性能承诺。

当前评测报告中的 `git_dirty=true`、运行结果与干净提交不完全绑定等限制，属于 C 级本机证据的适用边界。

## 六、已接受的证据缺口

以下内容已经明确记录为“未补证”，不再作为当前项目的整改任务：

1. 评测报告没有全部绑定到干净提交 SHA、实际运行时 LLM 模型和完整源码指纹，不能据此宣称干净提交上的最终复评。
2. RAG 评测没有完整拆分队列等待、embedding、Qdrant 检索和 LLM 的分段 P50/P95，也没有独立的超时类型统计。
3. 仓库内没有完整的浏览器人工演示归档，尤其是 WebSocket 实时下行、引用实际点击和完整讲解节奏的逐项记录。
4. 空模型缓存、全新公共克隆和源码 Compose 冷启动没有形成完整可复核证据；已有验证可能复用了镜像或模型缓存。
5. 项目没有压力测试，因此不对吞吐量、并发用户数、容量上限、成本或线上 SLA 作承诺。
6. 当前 47 条本机真实图片不进入 Git，公共克隆无法直接复现完整图片质量评测。
7. 本次运行没有第三方盲评或人工评分；机器指标不等同于第三方评审结论。

这些缺口表示“证据没有归档或不能外推”，不表示相应业务链路在代码中不存在。现有功能、工程测试和历史运行记录仍以对应证据文件为准。

## 七、简历与答辩口径

### 推荐表述

> 独立设计并实现 Go + Python 的可评测引用式 RAG 社区系统：Go 负责社区业务、权限和最终可见性校验，Python 负责 Chinese-CLIP/BGE 向量检索、Qdrant、RAG 和 LangGraph 只读 Agent；通过 Redis Streams 实现图文索引与 Bot 任务的异步幂等处理，并使用冻结数据集、工程测试和故障矩阵验证检索质量、引用安全和依赖降级行为。

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
- [Final Freeze 自动化证据](evidence/final-freeze/README.md)：本机服务、真实 Provider、当前 34/32/30/36 评测和脱敏运行汇总；Compose E2E 与故障注入明确标记为未运行。
