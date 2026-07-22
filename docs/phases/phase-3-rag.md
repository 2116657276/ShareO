# Phase 3 — 私聊 RAG Bot

> 详细协议见 [引用式 RAG 设计](../design/rag.md)。

> 当前状态：6A/6B 技术链路已实现并通过 mock LLM E2E；真实 provider、质量评测、Demo seed 和最终清理待第七阶段完成。

## 范围

- approved 正文按 400 字符、80 字符重叠分块，FastEmbed `BAAI/bge-small-zh-v1.5` 写入 `post_chunks`。
- 用户私聊 `shareo_bot` 才触发任务；不支持群聊和 Agent。
- 检索 top 8，按帖子去重后最多提供 5 个来源。
- LLM 只可选择实际检索到的 chunk ID，Go 再次验证引用可见性。
- Bot 任务使用 `shareo:stream:bot_tasks`，普通消息先落库和 WebSocket 下发，再异步发布；Redis 或 AI 故障不阻塞普通私聊。
- `GET /internal/bot/tasks/:message_id` 返回严格校验的 Bot 私聊上下文，`POST /internal/bot/reply` 在事务中验证会话、Bot 身份和来源帖子可见性，并以 `source_message_id` 幂等。
- Worker 对 4xx 回调永久确认，对超时、5xx 和网络错误保留 pending；第 4 次失败发送一次“AI 当前暂不可用，请稍后重试。”兜底消息。
- 消息历史、会话列表和 WebSocket 推送都会清理已删除或取消审核帖子的引用；聊天页只渲染后端返回的可见引用卡片。

## 退出条件

- 阶段 6 已满足：重复任务只产生一条回复；普通私聊不触发；Bot 不可登录且不自触发；来源删除后新回复无失效引用；AI 故障不影响普通社区和私聊；mock LLM E2E 通过。
- `make check`、真实 MySQL/Redis/Qdrant 集成测试、Python 集成测试、`go test -race ./...` 和 `make test-ai-e2e` 已通过。
- 阶段 7 继续完成：真实 provider 演示、15 条问答来源命中率 ≥ 0.80、12 条搜图 Recall@5 ≥ 0.70、引用可访问率 100%、虚假引用为 0，以及 Demo seed。
