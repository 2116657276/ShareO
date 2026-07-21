# Phase 3 — Bot + RAG

> 更新时间: 2026-07-21 | 状态: 未启动，依赖 Phase 2 完成

## 目标与当前基线

基于站内 approved 内容回答问题并返回可访问引用。当前只有 Redis Streams worker 骨架，Bot handler 仍为 Phase 3 TODO。

## 工作包

- `011_bot.sql`：`users.is_bot`、消息 `meta` JSON 和 Bot 初始化数据。
- approved 帖子分块、固定 revision 的 BGE 中文 embedding、`post_chunks` collection。
- 先完成检索和引用评测，再接 OpenAI 兼容 LLM 与 IM Bot。
- 私聊 Bot 或群内明确提及时触发，普通聊天不等待 AI。

## 接口与数据流

Go 发布 bot task；worker 拉取近 20 条会话上下文，检索站内块，生成带帖子引用的回答，再通过内部 Bot 回复接口走正常消息管线。

## 失败模式

AI/LLM 超时返回可理解的兜底消息；引用帖子在返回前由 Go 校验可见性；停止 AI 服务不得影响普通 IM。

## 测试矩阵

分块边界、检索排序、引用过滤、mock LLM、Bot 权限、重复任务幂等、超时与降级；至少 30 条问答评测。

## 退出标准

引用正确率 ≥ 0.80，所有引用可访问，质量/延迟/成本报告归档，AI 停止时聊天正常。

## 证据矩阵

尚无实现证据；不得提前标记进行中。

## 暂缓项

rerank 和本地 7B 对比可在基本 RAG 达标后追加。
