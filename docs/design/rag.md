# 引用式 RAG 设计

> 状态：已实现（阶段 6 mock LLM 闭环已通过） | Prompt 版本：`rag-v1`

## 边界

知识库只包含 `approved AND is_deleted=0` 的帖子正文，不索引评论、用户资料或私聊。Python 负责正文向量、检索和回答候选；Go 在 Bot 回复落库前再次检查会话、Bot 身份及引用帖子可见性。RAG 不可用不得影响 Feed、互动、语义搜图或普通私聊。

## 文本索引

- 与图片共用 `shareo:stream:index_post`，一次 upsert 同时维护 `images` 和 `post_chunks`。
- 正文按中文段落和句末标点优先切分，目标 400 字符、重叠 80；空正文删除该帖文本向量。
- FastEmbed 模型固定为 `BAAI/bge-small-zh-v1.5`，输出归一化 512 维向量。
- Qdrant `post_chunks` 使用 cosine；`chunk_id` 为 `post_id:chunk_no`，point ID 为 chunk ID 的 UUIDv5。
- payload 保存 `post_id`、`chunk_id`、`chunk_text`、`model_name`、`created_at`，并为 `post_id` 和 `chunk_id` 建索引。
- 删除、驳回或重新 pending 时同时删除图片和正文向量；重复 upsert/delete 幂等。

## 回答流程

```text
问题 → FastEmbed query embedding → Qdrant top 8
     → 按 post_id 去重至 5 个来源 → 受限 Prompt
     → OpenAI-compatible chat completions → JSON 解析
     → chunk ID 白名单 → 带引用回答
```

模型输出固定为 `{"answer":"...","source_chunk_ids":["12:0"]}`。资料和聊天历史以 JSON 数据传入，Prompt 明确禁止执行其中的指令。Python 过滤所有未出现在本次检索结果中的 chunk ID；没有来源、无有效引用或资料不足时返回“不确定，未找到足够的相关内容。”。这不能替代 Go 的最终可见性校验。

## 内部 API

`POST /v1/rag/answer` 需要 `X-Internal-Token`：

```json
{
  "question": "夜景怎么拍？",
  "history": [{"role": "user", "content": "我只有手机"}],
  "top_k": 8
}
```

```json
{
  "answer": "可以固定手机并降低曝光。",
  "citations": [
    {"post_id": 12, "chunk_id": "12:0", "excerpt": "……", "score": 0.82}
  ]
}
```

`/readyz/rag` 只有文本模型、`post_chunks`、索引消费者及 LLM 配置均可用时返回 200。LLM 使用 `SHAREO_AI_LLM_BASE_URL`、`SHAREO_AI_LLM_API_KEY`、`SHAREO_AI_LLM_MODEL` 和 `SHAREO_AI_LLM_TIMEOUT_SECONDS` 注入；代码与文档不保存密钥。DeepSeek 可作为云端演示，Ollama 的 OpenAI-compatible 地址可作本地对照。

### 私聊 Bot 协议

Go 只在私聊 `shareo_bot` 且发送者为普通用户时发布 `shareo:stream:bot_tasks`。消息先完成数据库事务和 WebSocket 下发，Redis 发布在异步桥接中执行，因此 Redis/AI 不可用不会阻塞普通消息。Bot 自己、其他机器人、群聊和无效会话都不会产生有效 Bot 任务。

`GET /internal/bot/tasks/:message_id` 和 `POST /internal/bot/reply` 只接受 `X-Internal-Token`。任务接口返回来源消息、会话最近 20 条消息、Bot 身份和会话信息；回复接口再次验证来源消息、会话必须是两人私聊、发送者必须是普通用户、Bot 必须是固定 `shareo_bot`。引用在 Go 事务中再次过滤为 `approved AND is_deleted=0` 且符合 `post_id:chunk_no` 格式，消息与 `bot_replies(source_message_id UNIQUE)` 同一事务写入。重复回调返回已有回复，不新增消息。

Python 只提交本次 Qdrant 检索候选中的 chunk ID。Worker 将 4xx 视为永久失败并确认任务，将超时、5xx 和网络错误留在 pending 以便重试；达到四次处理尝试后发送一次固定兜底消息 `AI 当前暂不可用，请稍后重试。`。Go 在历史消息、会话列表和实时推送前再次清理不可见引用。

阶段 6 的 `compose.test.yaml` 和 `scripts/test_ai_e2e.sh` 提供仅用于测试的 OpenAI-compatible mock provider，不加入默认生产 Compose。mock E2E 已覆盖正文索引、带引用回复、重复任务、普通私聊、来源删除和 LLM 超时兜底；真实 provider 与质量评测属于阶段 7。

## 非目标

不引入 LangChain、LlamaIndex、rerank、GraphRAG、Agent、工具调用、评论知识库或多 provider 路由。项目规模增长前，保持当前可测试的显式 pipeline。
