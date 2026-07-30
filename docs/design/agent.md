# 只读社区知识 Agent 设计

> 状态：只读 Agent 的工程与安全设计已实现；机器门禁和发布证据边界以 [`docs/REVIEW.md`](../REVIEW.md) 为准。实现边界以 [ADR-008](../adr/ADR-008-readonly-agent.md) 为准。

## 目标

在现有 `shareo_bot` 私聊中增加可选的深度分析模式，让模型能够根据问题自主选择多个只读工具，完成检索、读取、比较和带引用总结。默认 RAG 路径保持不变，Agent 不拥有社区写权限。

## 运行边界

用户发送消息时可选 `ai_mode=rag|agent`，缺省为 `rag`。`agent` 只对固定 Bot 私聊生效。Python 不能访问 MySQL 或 MinIO 凭证；工具通过受保护 Go 内部接口取得 approved、未删除帖子数据，最终 Bot 引用仍由 Go 事务落库前复核。

## 状态图

```text
START → model_decide → tool_execute ─┐
          │                          │
          └────────── finalize ◄─────┘
```

模型决策节点只能返回结束文本或白名单 function call。工具节点验证名称、JSON Schema、数量、字符数和预算；结果以不可信观察注入下一轮。最终节点只能引用本次已验证工具结果中的帖子和 chunk。

## 工具

| 工具 | 数据来源 | 作用 | 上限 |
|---|---|---|---:|
| `semantic_search_posts` | Python embedding/Qdrant + Go 可见性复核 | 处理语义问题 | 10 条 |
| `keyword_search_posts` | Go/MySQL 全文或 LIKE | 处理精确名词 | 10 条 |
| `read_posts` | Go 内部批量只读接口 | 读取帖子正文和必要元数据 | 10 个 ID |
| `search_images` | Chinese-CLIP/Qdrant + Go 可见性复核 | 处理图片相关问题 | 8 条 |

不提供创建、编辑、删除、评论、点赞、关注、发送消息、外部 HTTP、文件系统或任意 SQL 工具。

## 预算与安全

最多 4 个决策轮次、6 次工具调用、每轮最多 2 个并行调用、总观察文本 12,000 字符、全链路 45 秒。未知工具、参数越界、重复恶意 ID 和提示词注入均拒绝执行。步骤卡片只保存工具事实和耗时，不保存思维链、完整 Prompt 或原始正文。

## 交付门禁

必须通过 Go/Python 单测、mock provider E2E、真实 MySQL/Redis/Qdrant E2E、故障恢复、36 条 Agent 冻结评测和 30 条人工评分。来源命中率 ≥ 0.80、必需工具选择率 ≥ 0.85、引用可访问率 100%、虚假引用/越权/预算越界/注入成功均为 0；人工平均分 ≥ 4.0/5，且不能有低于 3 分的答案。
