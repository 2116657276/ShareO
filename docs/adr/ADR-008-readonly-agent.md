# ADR-008：私聊内只读社区知识 Agent

- 日期：2026-07-26
- 状态：已接受，Phase 8 实现中
- 取代：ADR-005 仅针对已实现 RAG 快速路径；ADR-006 关于“最终范围不包含 Agent”的条款由本 ADR 的受限例外部分取代

## 背景

当前 `shareo_bot` 是可靠的固定流程 RAG Bot，已经具备异步任务、引用白名单、Go 可见性复核和质量评测，但没有规划器、工具选择或行动—观察循环。为了作为 RAG + Agent 简历项目，需要增加可由代码、测试、指标和演示证明的 Agent 能力，同时不能破坏现有 RAG 和社区数据边界。

## 决定

- Agent 只存在于现有 `shareo_bot` 私聊，通过用户显式开启的 `ai_mode=agent` 触发；缺省仍为 `rag`。
- Python 使用 LangGraph `StateGraph` 编排有限状态图，保留自定义节点、预算和错误语义，不使用黑盒通用 Agent 运行时。
- 模型使用 OpenAI-compatible function calling；服务端仍执行工具名、JSON Schema、长度、数量、超时和可见性校验。
- 只提供 `semantic_search_posts`、`keyword_search_posts`、`read_posts`、`search_images` 四个只读工具；不提供写工具、外部工具、长期记忆、多 Agent 或独立页面。
- Go 仍是 MySQL 唯一业务写者，Python 仍是 Qdrant 唯一写者，MinIO 凭证仍只在 Go 侧；Agent 只能通过内部 Token 接口读取已审核数据。
- 最终 Bot 消息持久化脱敏 `agent_trace`，只展示完成后的步骤卡片，不保存思维链或完整工具上下文；不启用 LangGraph checkpoint。

## 预算与门禁

每次运行最多 4 个决策轮次、6 次工具调用、每轮最多 2 个并行调用、12,000 字符观察文本和 45 秒总时长。Agent 质量门禁为来源命中率 ≥ 0.80、必需工具选择率 ≥ 0.85、引用可访问率 100%、虚假引用/越权/预算越界/提示词注入成功均为 0；30 条人工评分平均 ≥ 4.0/5，且无低于 3 分的答案。现有 RAG 40/30 门禁必须保持通过。

## 后果

项目可以真实展示 RAG 与 Agent 的差异：RAG 负责低延迟固定路径，Agent 负责可审计的多步只读研究。代价是新增 provider tool-call 解析、内部批量读取接口、工具安全测试、LangGraph 依赖和 Agent 评测集；这些成本通过不引入写工具、持久 checkpoint 和独立页面控制在可演示范围内。
