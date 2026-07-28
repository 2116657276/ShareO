# Phase 8 — 只读社区知识 Agent

> 阶段边界：受限只读 Agent 的工程、协议、安全和评测；当前阶段状态由 [`TASK.md`](../../TASK.md) 统一维护。

## 目标

在现有私聊 RAG Bot 中增加显式开启的只读社区知识 Agent，使用 LangGraph `StateGraph` 选择语义检索、关键词检索、帖子读取和图片检索工具，生成带 Go 可见性复核引用的回答，并在聊天内展示完成后的脱敏步骤卡片。

## 当前基线

Phase 7B 的 40/30 RAG 与搜图质量门禁已经形成历史报告；Phase 7C 工程证据与 Phase 8 自动化能力已有记录。2026-07-28 在本机 Qdrant ready、当前 AI 源码和本地模型缓存生效的环境中，完整 36 条真实 DeepSeek Agent 机器门禁已重新通过，下一阶段进入 Go Template 前端补齐和重构。人工浏览器验收与 Docker/Compose 仍未开始。

本阶段日常验证使用宿主机 Go/Python、Homebrew `mysql@8.0`/Redis/MinIO 和从
`qdrant/` 目录启动的本地 Qdrant。Docker/Compose 当前暂缓，仅保留为后续跨服务集成、故障恢复和源码冷启动复核，
不把容器构建网络耗时计入本机开发路径。

2026-07-28 最近一次本机工作区复评为 36 条无失败/超时，来源命中率 1.0000，必需工具选择率 0.9167，引用可访问率 100%，无答案准确率 1.0000，虚假引用、越权、预算越界和注入失败均为 0，P95 13.39 秒。报告在仓库外且工作区为 dirty，因此只作为当前开发证据；干净提交仍需重新评测。此前 0.6944、0.7778、0.8889 报告、30 条人工评分和结构化输出失败报告均保留为历史证据。

## Phase 8A：人工验收前自动化收口 ✅ 机器门禁已通过

Phase 8A 优先于人工评分和浏览器演示。环境预检、健康/readiness、主动预热、运行时模型元数据、Provider 错误分类、一次受控重试、失败门禁、确定性 E2E、真实集成和本机优先运行入口已经完成；有效机器评测已通过，当前转入前端补齐，不提前进行浏览器验收或 Docker 打包：

1. [x] 修复多 tool call 消息链：并行上限只限制并发，所有调用 ID 必须获得对应 tool message；预算外调用返回受控错误。
2. [x] Provider 失败转为状态图失败 state，保留故障前步骤、来源、累计次数和拒绝调用数；最终 JSON 重试不重复执行工具。
3. [x] 逐题报告记录期望工具、实际工具和工具命中，分别输出全部任务与已完成任务工具选择率。
4. [x] readiness 和 `doctor` 增加 AI 进程源码指纹一致性检查；复用数据卷时仍重建 AI 服务。
5. [x] 在恢复 Qdrant、重启 AI 并启用评测任务隔离后重新取得完整机器门禁；工具选择率 0.9167，本轮不修改 Prompt、数据集、检索参数或阈值。
6. [x] 上一轮机器门禁通过后完成过 30 条人工评分；最终历史表填写了分数、实际引用 post ID、引用详情和回答评价，平均 4.8667 且最低 4；该评分仍只作历史证据，前端完成后再抽查新报告中的 10 条答案。

## 范围与非目标

范围是 `ai_mode=agent`、LangGraph 状态图、四个只读工具、Go 内部帖子复核、步骤卡片、自动测试、36 条 Agent 评测、独立语义搜图页面和最终人工抽查。非目标是写入工具、外部网页/文件、长期记忆、checkpoint、多 Agent、独立 Agent 页面、模型训练和公开在线部署。

## 依赖

依赖 Phase 2 私聊、Phase 3 AI 运行时、Phase 4 图片检索、Phase 5 RAG、Phase 6 Bot 和 Phase 7 发布证据。必须锁定 LangGraph 与其直接依赖，扩展 DeepSeek/OpenAI-compatible provider 的 tool-call 协议，并保持现有 Go/Python 数据所有权。

## 工作包

1. [x] 冻结 `ai_mode`、消息 `meta`、`agent_trace`、内部只读接口、工具 schema、预算和错误语义。
2. [x] 实现 provider tool calls、LangGraph 状态图、四个工具、Go 可见性复核和最终引用过滤。
3. [x] 实现深度分析开关、完成后步骤卡片、刷新恢复、日志和 readiness。
4. [x] 增加 mock provider、Go/Python 单测、36 条数据集和 Agent 评测 CLI；源码跨服务 E2E、真实集成和故障恢复已有证据，Compose/源码冷启动按最终打包阶段复核。
5. [x] 前端自动整理完成并生成 [`frontend-manual-acceptance.md`](../evidence/phase8-agent/frontend-manual-acceptance.md)；用户在最终人工阶段抽查 10 条 Agent 答案，历史 30 条评分不作为当前机器门禁。
6. [x] 审计公开、管理员、Go 内部和 AI 内部 API，新增 `make test-api`；不因浏览器直接打开 JSON 被拦截而改变接口边界。
7. [x] 已实现独立 `/search/images` 页面并完成 Go Template 页面体系首轮整理；页面复用现有公开搜图 API，并通过 Go Handler、模板静态契约、本机 HTTP smoke、`make check` 和 Go race 验证。该页面不是 Agent 页面，也不新增公开 RAG/Agent API；浏览器验收仍待后续阶段。

## 接口与数据流

外部消息接口只新增可选 `ai_mode`，不新增 Agent 页面或公开 Agent API。Bot task 携带模式；Python 通过内部 Token 调用 Go 的关键词搜索和批量读取接口，向量工具复用 Qdrant 后再复核可见性；Bot reply 携带引用与 `agent_trace`，Go 在事务中落库并通过既有 WebSocket 下行。

## 数据一致性

用户消息和 Bot 回复沿用现有 MySQL 事务与 `bot_replies` 唯一约束；Redis Streams 继续 at-least-once 和重领。Agent 轨迹为消息元数据，不成为业务真相；Qdrant 仍为可重建派生数据。所有工具结果和最终引用都只能来自 approved、未删除帖子。

## 安全边界

工具白名单、JSON Schema、ID/数量/字符/时间预算和内部 Token 必须由代码校验。帖子正文、图片描述、历史消息均按不可信数据处理；拒绝其中的角色切换、工具指令和 Prompt 覆盖。日志和步骤卡片不得包含思维链、完整正文、Token、Cookie、Authorization 或 API Key。

## 失败模式

Provider 不可用、Qdrant 不可用、Go 内部接口失败、工具参数非法、未知工具、预算耗尽和没有来源时，均返回受控状态；有足够证据时总结已有结果，否则返回固定无答案/暂不可用。重复 Stream 任务必须只生成一条 Bot 回复；普通私聊和默认 RAG 不因 Agent 失败回滚。

## 测试矩阵

覆盖默认 RAG 回归、合法多步调用、单轮三个以上工具调用、并行分批、预算外调用闭合、关键词/语义/图片工具、无答案、引用过滤、提示词注入、未知工具、非法参数、预算耗尽、失败前轨迹保留、最终 JSON 重试、源码指纹不一致、provider/Qdrant/Redis/Go 故障、恢复后基线、消息刷新恢复、重复投递、真实 MySQL/Redis/Qdrant E2E 和 36 条冻结 Agent 评测。

## 退出标准

来源命中率 ≥ 0.80、必需工具选择率 ≥ 0.85、引用可访问率 100%、虚假引用 0、未知/禁止工具调用 0、预算越界 0、提示词注入成功 0、正常失败请求 0、预热后 P95 ≤ 50 秒；30 条人工平均 ≥ 4.0/5 且无低于 3 分；现有 RAG 门禁不回退。

## 提交与环境证据

实现后记录实际提交 SHA、uv.lock、Compose 配置摘要、模型与 revision、Prompt/工具协议版本、数据集 SHA、设备、P50/P95、token、超时率、工具轨迹统计、自动测试、人工评分和演示录像索引。公共仓库不包含个人照片，完整真实图片评测需要外部素材流程。

## 当前验证记录

已通过 Python 非集成测试、ruff、格式检查、Shell 和文档门禁、Go 全量测试与 race、模板解析和页面契约测试、真实集成、`make test-api`、`make local-doctor`、`make test-local-stack`、`make warm-ai` 和本机 RAG 40/30 机器回归。图片/Bot/Agent 的 Compose 专用 E2E、独立 Compose 故障矩阵和源码冷启动按当前主线暂缓，不把已有镜像验证写成当前源码冷启动通过。测试 Provider 的 Agent 机器门禁通过；最近一次本机真实 DeepSeek 工作区复评为 36 条无失败/超时、工具选择率 0.9167、来源命中率 1.0000、引用可访问率 100%、无答案准确率 1.0000、P95 13.39 秒，其他安全门禁均为 0。由于报告不在仓库且工作区为 dirty，干净提交仍需复评。此前结构化输出失败和工具选择率 0.7778 报告保留为历史证据。

## 遗留项

Phase 7C 的源码构建、人工浏览器演示和干净复评必须先补证。Phase 8 不承诺长期记忆、多 Agent、外部工具、在线服务或生产级容量；这些如未来需要，必须另立阶段和 ADR。当前报告来自 dirty workspace，不能作为最终可重建 SHA 的替代品。
