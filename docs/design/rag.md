# 引用式 RAG 设计

> Prompt版本：`rag-v1`

## 边界

知识库只包含 `approved AND is_deleted=0` 的帖子正文，不索引评论、资料或私聊。Python负责文本索引、检索和回答候选；Go在Bot回复落库前再次验证会话、Bot身份和帖子可见性。

不引入LangChain、LlamaIndex、rerank、GraphRAG、Agent、工具调用或多provider路由。

## 文本索引

- 与图片共用 `shareo:stream:index_post`。
- 正文按段落和句末标点优先切分，目标400字符、重叠80。
- FastEmbed模型固定 `BAAI/bge-small-zh-v1.5`，归一化512维。
- Qdrant `post_chunks` 使用cosine；chunk key为 `post_id:chunk_no`，point ID为其UUIDv5。
- payload保存 `post_id`、`chunk_id`、`chunk_text`、`model_name`、`created_at`。
- 删除、驳回或重新pending时同时清理图片和正文向量。

## 回答流程

```text
问题 → query embedding → Qdrant top_k
     → 按post_id去重至最多10个候选来源
     → 受限Prompt → DeepSeek/OpenAI-compatible chat completions
     → JSON解析 → chunk ID白名单 → 回答与引用
```

公开的 AI 内部接口默认 `top_k=8`，允许请求方传入 1–20；冻结质量评测显式使用 `top_k=15`。`rag_max_sources=10` 限制进入 Prompt 的去重帖子数，私聊 Bot 回调再截断为最多 5 条引用。这三个数含义不同，不应合并描述。

模型输出固定为：

```json
{"answer":"...","source_chunk_ids":["12:0"]}
```

帖子、检索文本和聊天历史以数据结构传入，Prompt明确禁止执行其中指令。Python删除不属于本次候选的chunk ID；无来源、无有效引用或资料不足时返回不确定回答。

## AI 接口

`POST /v1/rag/answer` 需要内部Token，请求包含1–500字符问题、最多20条历史和1–20的 `top_k`。响应包含回答及 `post_id/chunk_id/excerpt/score` 引用。

`/readyz/rag` 只有文本模型、`post_chunks`、索引consumer和LLM配置可用时返回200。默认真实provider为DeepSeek；mock provider只用于E2E，Ollama为可选实验。

## 私聊 Bot

用户消息完成MySQL事务和WebSocket下发后，Go仅对普通用户私聊固定 `shareo_bot` 发布 `bot_tasks`。

`GET /internal/bot/tasks/:message_id` 返回严格校验的来源消息、DM和最近20条历史。`POST /internal/bot/reply` 再次验证来源、固定Bot、两人会话和帖子可见性，并在同一事务写入消息与 `bot_replies`。

`source_message_id` 唯一确保重复任务和回调只产生一条回复。4xx回调错误永久确认；超时、5xx和网络错误保留pending；第4次失败只发送一次“AI 当前暂不可用，请稍后重试。”。

## 引用安全

Python只提交本次检索候选中的chunk ID。Go只接受格式正确且对应approved、未删除帖子的引用。消息历史、会话列表和WebSocket推送都会再次清理失效引用。

## 质量门禁

30条冻结问答的来源命中率≥0.80、引用可访问率100%、虚假引用0、人工相关性平均≥4.0/5。性能目标是本地检索P95≤2秒，并记录 DeepSeek总延迟P50/P95和超时率；当前评测器只采集端到端总延迟，分段检索 P95 和超时分类仍待实现。
