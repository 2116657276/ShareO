# Phase 6 — 私聊 RAG Bot

> 状态：已完成

## 目标

让普通用户私聊固定 `shareo_bot` 时异步获得带帖子引用的回答，同时不影响普通私聊。

## 当前基线

任务发布、上下文读取、RAG 生成、Go 事务回复、引用二次校验、页面卡片、mock provider 和跨服务 E2E 已完成。

## 范围与非目标

只在一对一 `shareo_bot` 会话触发；不支持其他机器人、群聊、主动消息、独立 Bot 页面或 Agent 工具。

## 依赖

依赖 Phase 2 私聊、Phase 5 RAG、`bot_tasks` Stream 和 Go 内部回复接口。

## 工作包

- 普通用户消息落库后异步发布 Bot 任务。
- 严格校验来源消息、会话、Bot身份和最近 20 条上下文。
- Python 生成后回调 Go，事务写消息与 `bot_replies`。
- 重复任务/回调幂等、4xx 永久确认、可重试错误四次处理。
- 页面 AI 助手入口、引用卡片和固定兜底消息。

## 接口与数据流

用户消息 → MySQL/WebSocket → `bot_tasks` → Go 上下文 → RAG → Go 回复事务 → WebSocket。普通私聊不经过 AI。

## 数据一致性

`bot_replies.source_message_id` 和 `reply_message_id` 均唯一；重复回调返回现有回复；消息和幂等记录同一事务。

## 安全边界

Bot 不能登录或自触发；Python 只提交候选 chunk ID；Go 再次过滤引用并清理历史、列表和推送中的失效引用。

## 失败模式

超时、5xx 和网络错误保留 pending；第 4 次失败发送一次固定兜底。Redis 或 AI 故障不回滚用户消息。

## 测试矩阵

Bot触发边界、重复任务、重复回调、普通私聊、其他机器人、来源删除、4xx、超时、5xx、兜底和页面引用。

## 退出标准

每条来源消息最多一条 Bot 回复；普通私聊不触发；失效来源不展示；mock E2E覆盖成功、重复与降级。

## 提交与环境证据

- `17b54ec`：连接私聊 Bot 核心链路。
- `0aff22e`：增加 Bot 页面与跨服务 E2E。
- `56e30f4`：关闭阶段评审发现，未遗留影响幂等、认证、可见性和普通私聊的 P0/P1/P2。
- `make test-ai-e2e`、真实集成、Python 集成和 Go race通过。

## 遗留项

真实 provider 和 30 条质量评测已由 Phase 7 完成；现有 5.08 秒记录是 API 冒烟，人工浏览器五分钟演示仍待补证。
