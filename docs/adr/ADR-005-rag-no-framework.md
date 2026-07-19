# ADR-005: RAG 实现 —— 自研 pipeline，不引入 LangChain/LlamaIndex

- 日期: 2026-07-19
- 状态: 已接受

## 背景

Bot 的 RAG 能力（站内帖子检索增强问答）需要决定实现路径：用编排框架还是手写。这是毕业设计，评价标准是"理解深度 + 可论证性"，不是交付速度。

## 候选方案

1. LangChain / LlamaIndex 全家桶
2. 自研 pipeline：chunk → embed → retrieve → (rerank) → prompt → answer with citations

## 决定

选自研。ai-service 的 `rag/` 模块手写完整链路：中文友好的分块策略、BGE 向量检索、可选 rerank、prompt 组装、带帖子引用的回答生成。LLM 调用只依赖 OpenAI 兼容 SDK 这一层薄封装。

## 理由

- 答辩上"每一步为什么这么做"必须自己答；手写意味着分块粒度、top-k、prompt 结构、引用机制全部是自己的决策，全部可写进论文并做消融实验。
- 框架的抽象在毕设规模是负资产：调试隔着多层封装，版本变动快，且"调通了框架"在答辩里不构成贡献。
- 依赖面小：transformers + qdrant-client + openai SDK 即可，环境干净。

## 后果与代价

- 分块、重试、上下文窗口管理都要自己写 → 正是学习目标本身，且各组件保持函数级简单。
- 后期若要复杂编排（多步 Agent 工具循环），自研循环不够用时再评估引入轻量框架（届时新增 ADR）。
