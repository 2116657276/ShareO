# ShareO 交付路线图

> 更新时间：2026-07-26 | 目标：可复现、可量化、五分钟可稳定演示

| 阶段 | 核心交付 | 依赖 | 状态 |
|---|---|---|---|
| Phase 0 范围与基线 | 最终范围、12 表 schema、统一 Compose 和质量门禁 | 无 | 已完成 |
| Phase 1 社区核心 | 认证、图文、上传、Feed、搜索、审核、互动、通知 | Phase 0 | 已完成 |
| Phase 2 一对一私聊 | DM、REST 写入、WebSocket、未读和断线恢复 | Phase 1 | 已完成 |
| Phase 3 AI 运行时 | FastAPI 单进程、Streams、重试、readiness、数据所有权 | Phase 0 | 已完成 |
| Phase 4 语义搜图 | Chinese-CLIP、图片索引、可见性过滤、真实 E2E | Phase 1、3 | 已完成 |
| Phase 5 RAG 检索 | 正文分块、FastEmbed、Qdrant、受限引用回答 | Phase 3 | 已完成 |
| Phase 6 私聊 Bot | 异步触发、幂等回复、引用二次校验、页面与 mock E2E | Phase 2、5 | 已完成 |
| Phase 7 Demo 与发布 | 固定数据、DeepSeek、40/30 评测、降级与演示 | Phase 4、6 | 7B 已完成，7C 进行中 |

Phase 7 分为 7A 数据、7B 评测、7C 发布。项目完成后不继续规划 Agent、群聊或生产级平台能力。
