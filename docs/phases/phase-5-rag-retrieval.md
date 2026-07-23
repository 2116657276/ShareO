# Phase 5 — 文本检索与 RAG

> 状态：已完成

## 目标

把已审核帖子正文建立为可追溯知识库，并生成只引用实际检索来源的受限回答。

## 当前基线

正文分块、FastEmbed、Qdrant `post_chunks`、OpenAI-compatible provider、结构化输出和双重引用白名单已经实现。

## 范围与非目标

知识库只含 approved、未删除帖子正文；不索引评论、用户资料或私聊，不引入 rerank、GraphRAG、框架或工具循环。

## 依赖

依赖 Phase 3 运行时、Go 索引载荷、Qdrant 和可选 LLM 配置。

## 工作包

- 中文段落/句末优先的 400/80 分块。
- `BAAI/bge-small-zh-v1.5` 512 维文本向量。
- `post_chunks` schema、确定性 point ID 和 payload index。
- top 8 检索、按帖子去重至 5 个来源。
- Prompt 数据隔离、JSON 解析和 chunk ID 白名单。

## 接口与数据流

`index_post` 同时维护图片和正文；问答经过 query embedding、Qdrant、来源去重、LLM、结构化解析和引用过滤。

## 数据一致性

chunk point ID 由 `post_id:chunk_no` 的 UUIDv5确定；状态变化同时清理图文向量；Go 在回复落库前再次验证帖子可见性。

## 安全边界

帖子、历史和检索内容作为数据而非指令传给 Prompt；模型只能选择本次候选 chunk；无来源时返回不确定回答。

## 失败模式

文本模型失败不能饿死图片索引；缺少 LLM Key 时 RAG readiness 为 503，但图搜、社区和普通私聊正常。

## 测试矩阵

分块边界、维度与归一化、collection、索引/删除、检索去重、Prompt 注入、JSON 错误、无来源和 provider 超时。

## 退出标准

正文索引可重复写删；无虚构 chunk ID；来源删除后不可再引用；缺少 provider 时受控降级。

## 提交与环境证据

- `0f1785d`：完成轻量正文索引与 RAG pipeline。
- 真实 FastEmbed 512 维、Qdrant round-trip 和 Compose 图文双索引 E2E通过。

## 遗留项

真实 DeepSeek 和 30 条质量评测由 Phase 7B完成。
