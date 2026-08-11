# 引用式 RAG 设计

> Prompt 版本：`rag-v1`

## 边界

知识库只包含 `approved AND is_deleted=0` 的帖子正文，不索引评论、资料或私聊。Python 负责文本索引、检索和回答候选；Go 在 Bot 回复落库前再次验证会话、Bot 身份和帖子可见性。

默认 RAG 路径不引入 LangChain、LlamaIndex、rerank、GraphRAG、Agent、工具调用或多 Provider 路由；受限只读 Agent 是独立链路，见 [Agent 设计](agent.md)。

## 文本索引

- 与图片共用 `shareo:stream:index_post`。
- 正文按段落和句末标点优先切分，目标 400 字符、重叠 80。
- FastEmbed 模型固定 `BAAI/bge-small-zh-v1.5`，归一化 512 维。
- PostgreSQL pgvector `ai.post_chunk_embeddings` 使用 cosine；chunk key 为 `post_id:chunk_no`，chunk ID 为其 UUIDv5。
- payload 保存 `post_id`、`chunk_id`、`chunk_text`、`model_name`、`created_at`。
- 删除、驳回或重新进入 pending 时同时清理图片和正文向量。

## 回答流程

```text
问题 → query embedding → pgvector `<=>` top_k
     → 按 post_id 去重至最多 10 个候选来源
     → 受限 Prompt → DeepSeek/OpenAI-compatible chat completions
     → JSON解析 → chunk ID白名单 → 回答与引用
```

公开的 AI 内部接口默认 `top_k=8`，允许请求方传入 1–20；当前报告的 v1 质量评测显式使用 `top_k=15`。`rag_max_sources=10` 限制进入 Prompt 的去重帖子数，私聊 Bot 回调再截断为最多 5 条引用。这三个数含义不同，不应合并描述。

模型输出固定为：

```json
{"answer":"...","source_chunk_ids":["12:0"]}
```

帖子、检索文本和聊天历史以数据结构传入，Prompt 明确禁止执行其中指令。Python 删除不属于本次候选的 chunk ID；无来源、无有效引用或资料不足时返回不确定回答。

## AI 接口

`POST /v1/rag/answer` 需要内部 Token，请求包含 1–500 字符问题、最多 20 条历史和 1–20 的 `top_k`。响应包含回答及 `post_id/chunk_id/excerpt/score` 引用。

`/readyz/rag` 只有文本模型、`post_chunks`、索引 consumer 和 LLM 配置可用时返回 200。示例配置使用 DeepSeek；实际 Provider 由 `SHAREO_AI_LLM_BASE_URL`、`SHAREO_AI_LLM_MODEL` 和对应密钥注入。mock Provider 只用于 E2E，Ollama 为可选实验。

## 私聊 Bot

用户消息与 `bot_task_outbox` 在同一 PostgreSQL 事务中完成；事务提交后 publisher 才向 Redis Streams 发布 `bot_tasks`，发布失败按退避重试，启动时补发 pending 任务。

`GET /internal/bot/tasks/:message_id` 返回严格校验的来源消息、DM 和最近 20 条历史。`POST /internal/bot/reply` 再次验证来源、固定 Bot、两人会话和帖子可见性，并在同一事务写入消息与 `bot_replies`。

`source_message_id` 唯一确保重复任务和回调只产生一条回复。4xx 回调错误永久确认；超时、5xx 和网络错误保留 pending；第 4 次失败只发送一次“AI 当前暂不可用，请稍后重试。”。

## 引用安全

Python 只提交本次检索候选中的 chunk ID。Go 只接受格式正确且对应 approved、未删除帖子的引用。消息历史、会话列表和 WebSocket 推送都会再次清理失效引用。

## 质量门禁

当前 30 条 RAG 评测题的来源命中率和完整来源覆盖率应 ≥ 0.80、引用可访问率 100%、虚假引用 0；两轮 AI judge 平均分应 ≥ 4.0/5 且最低分 ≥ 3.0，低分或不确定答案转人工复核。性能目标是本地检索 P95 ≤ 2 秒；当前报告同时记录端到端 P50/P95、可取得的 embedding/检索/LLM 阶段耗时和超时分类，无法可靠取得的队列、postprocess 或 callback 字段明确标记为 `unavailable`。
