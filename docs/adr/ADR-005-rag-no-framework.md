# ADR-005：轻量 RAG 组件组合

- 日期：2026-07-22
- 状态：已接受

## 决定

RAG 使用 FastEmbed + Qdrant Client + `httpx` OpenAI-compatible 调用 + 自研受限 pipeline，不引入 LangChain、LlamaIndex、RAGFlow 或 Agent 框架。

## 理由

- 帖子正文规模小，流程固定为分块、检索、Prompt、回答和引用校验。
- `BAAI/bge-small-zh-v1.5` 可由 FastEmbed 以较轻的 ONNX 运行时加载。
- 自研 pipeline 可以直接保留确定性 chunk ID、帖子可见性和引用白名单。
- 项目目标是练习 Go 与 AI 服务集成，而不是展示编排框架配置。

## 约束

- LLM 只通过环境变量配置的 OpenAI-compatible 接口调用。
- 模型输出的 chunk ID 必须属于本次检索结果。
- 所有引用在写入聊天消息前由 Go 验证。
- 不实现 rerank、GraphRAG、Agent 或工具循环。
