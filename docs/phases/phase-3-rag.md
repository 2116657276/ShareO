# Phase 3 — 私聊 RAG Bot

> 详细协议见 [引用式 RAG 设计](../design/rag.md)。

> 当前状态：文本索引、检索、LLM provider 与引用白名单已完成；私聊 Bot 和最终质量评测待完成。

## 范围

- approved 正文按 400 字符、80 字符重叠分块，FastEmbed `BAAI/bge-small-zh-v1.5` 写入 `post_chunks`。
- 用户私聊 `shareo_bot` 才触发任务；不支持群聊和 Agent。
- 检索 top 8，按帖子去重后最多提供 5 个来源。
- LLM 只可选择实际检索到的 chunk ID，Go 再次验证引用可见性。

## 退出条件

- 重复任务只产生一条回复，LLM 错误只产生一次兜底消息。
- 15 条问答来源命中率 ≥ 0.80，引用可访问率 100%，虚假引用为 0。
- 缺少云端 Key 时普通社区和私聊继续可用。
