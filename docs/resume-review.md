# ShareO 简历项目审查

> 审查日期：2026-08-08 | 本文不固定分支或提交数量；具体代码基线以提交时的 `git status`、提交 SHA 和评测报告为准。

## 推荐定位

ShareO 适合表述为“Go + Python 的可评测引用式 RAG 应用工程项目”，并可补充“同一私聊入口中的受限只读知识 Agent”。当前仍不是通用 Agent：没有写入工具、外部工具、长期记忆、多 Agent 或独立 Agent 页面。默认 `shareo_bot` 消息仍是固定 RAG；只有用户显式打开深度分析后，才进入 LangGraph 状态图并在四个只读工具之间进行受限多步调用。

简历、项目介绍和面试讲解可以强调 RAG、受限 Agent 工具调用、LangGraph 状态图、可见性与引用安全、异步幂等、质量评测和故障降级；不得把项目描述成“自主 Agent 平台”“多 Agent 系统”或“生产级通用智能体”。当前证据目录的最新报告运行 `20260808T151704Z` 使用本机 47 条语料和真实 Provider，工作区为 dirty；该报告的机器质量门禁通过，AI judge 仍有 10 条答案需要人工复核，不能替代干净提交复评或人工评分。当前结果和证据边界见 [`docs/evidence/final-freeze/README.md`](evidence/final-freeze/README.md)。

## 已由代码和测试支持的能力

| 简历可用表述 | 主要事实依据 |
|---|---|
| 设计 Go 业务服务与 Python AI 服务的数据所有权边界 | PostgreSQL `public` 运行时只由 Go 写，`ai`/pgvector 只由 Python 写，MinIO 凭证只在 Go 侧 |
| 实现帖子正文 RAG 与可访问引用 | 400/80 字符分块、BGE 512 维向量、pgvector `ai.post_chunk_embeddings`、结构化回答和两级引用白名单 |
| 实现中文语义搜图 | Chinese-CLIP、pgvector `ai.image_embeddings`、候选过取、按帖去重和 Go 可见性复核 |
| 实现受限只读社区知识 Agent | LangGraph `StateGraph`、原生 tool calling、语义/关键词/帖子读取/图片四个只读工具、步骤轨迹和最终引用过滤；工程测试、真实集成和当前机器门禁均有证据 |
| 实现异步且幂等的 Bot 工作流 | Redis Streams at-least-once、重试、`XAUTOCLAIM`、`NOGROUP` 恢复、`source_message_id` 唯一约束 |
| 建立 AI 质量评测 | 当前报告使用 38 条搜图（含 8 条 no-match）、32 条搜贴、30 条 v1 RAG 和 36 条 v1 Agent 机器评测；搜图 Recall@5 0.9222 且 no-match accuracy 1.0，RAG 完整来源覆盖率 0.9333、引用精度 0.9000，Agent 完整来源覆盖率 0.9722、额外工具调用 0；另有双轮 AI judge 和 10 条人工复核交接包中的待复核题 |
| 验证依赖故障降级 | 代码和本机 readiness 定义了 AI、PostgreSQL/pgvector、MinIO、Redis 和 LLM Provider 的降级边界；旧 Compose 路径已退役 |

上述指标只适用于当前本机语料和当次本地环境，不代表通用检索效果、线上 SLA 或第三方独立测评。AI judge 评分不等于人工评分，10 条待复核题见 [Final Freeze 人工复核交接包](evidence/final-freeze/ai-judge-human-handoff.json)。

## 审查发现与证据等级

### 当前审查结论

1. 产品默认是固定 RAG Bot，同时提供显式开启的受限只读 Agent；文档和简历必须区分默认 RAG、受限 Agent 与通用 Agent，并保留证据边界。
2. 当前报告记录了实际模型、数据集 SHA、Git SHA、dirty 状态和 dirty diff 指纹，但尚未完成干净提交复评。
3. 当前 47 条本机真实图片未进入 Git。公共克隆可运行不依赖照片的工程测试，但完整质量评测需要另行准备本机图片语料。
4. 本轮已完成一次本机人工浏览器 smoke，但没有形成完整浏览器 E2E、移动端真实视口、实时 WebSocket 下行和引用点击的自动化证据。
5. 两轮 AI judge 已完成，但 10 条低分、低置信或来源异常答案仍需人工复核；AI 评分不等于人工或第三方评审。
6. 当前没有压力测试、容量报告或成本评估，因此不能承诺生产级高可用、吞吐量、并发上限、成本或线上 SLA。

### 尚未证明的能力

- 通用 Agent、写入型或外部工具、长期记忆、rerank、混合检索或 GraphRAG；当前已实现的四个只读工具和受限状态图不能扩写成上述能力。
- 生产级高可用、水平扩展、容量上限、吞吐量、并发用户数、成本和线上 SLA。
- 从全新公共克隆、无本地图片和无模型缓存开始的一键完整复现。
- 完整浏览器 E2E、移动端真实视口、实时 WebSocket 下行、10 条 AI judge 待复核题的人工结论和独立评审者质量评分。

## 简历表述建议

推荐使用如下口径：

> 独立设计并实现 Go + Python 的社区 RAG 系统：通过 Redis Streams 异步构建 Chinese-CLIP 图片索引与 BGE 正文索引，使用 PostgreSQL pgvector 完成向量检索；为私聊 RAG Bot 增加任务幂等、失败重试、引用白名单和 Go 侧可见性复核，并在同一入口实现显式开启的 LangGraph 只读知识 Agent，以四个受限工具完成多步检索和脱敏轨迹记录；用版本化评测数据集、机器门禁和独立故障矩阵验证质量与降级行为。

不要使用“打造自主 Agent 平台”“支持多工具协作”“生产级高并发”“一键完整复现”或“第三方评测达到 5.0”等无法由当前仓库证明的描述。

## 当前未完成项

1. 用户人工复核 AI judge 交接包中的题目，以及浏览器视觉/WebSocket 实时下行、引用点击和页面错误状态。
2. 干净提交上的当前 38/32/30/36 评测复评，以及与运行时模型、源码指纹的完整绑定。
3. 版权清晰、可公开分发的 Demo 图片素材；公共克隆目前只能复现不依赖个人照片的工程测试。

## 已批准的后续定位

项目已实现 RAG + Agent 主线。旧评测报告和过期评分草稿已退役；当前没有人工评分结论，运行结果统一见 [`docs/evidence/final-freeze/README.md`](evidence/final-freeze/README.md)。只读社区知识 Agent 位于同一私聊入口：默认走当前 RAG，用户显式开启深度分析后，LangGraph 状态图可选择语义检索、关键词检索、帖子读取和图片检索工具，并在最终消息中展示脱敏步骤卡片。

该规划只有在真实代码、Agent 版本化评测数据集、自动化安全门禁、用户人工验收和浏览器演示全部完成后，才可写成最终收口口径。公开仓库只发布代码，不包含个人照片；因此公开克隆不能被描述成可直接复现完整真实图片质量评测。
