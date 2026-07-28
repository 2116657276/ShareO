# ShareO 交付路线图

> 更新时间：2026-07-28 | 目标：可复现、可量化、五分钟可稳定演示

| 阶段 | 核心交付 | 依赖 | 状态 |
|---|---|---|---|
| Phase 0 范围与基线 | 最终范围、12 表 schema、统一 Compose 和质量门禁 | 无 | 已完成 |
| Phase 1 社区核心 | 认证、图文、上传、Feed、搜索、审核、互动、通知 | Phase 0 | 已完成 |
| Phase 2 一对一私聊 | DM、REST 写入、WebSocket、未读和断线恢复 | Phase 1 | 已完成 |
| Phase 3 AI 运行时 | FastAPI 单进程、Streams、重试、readiness、数据所有权 | Phase 0 | 已完成 |
| Phase 4 语义搜图 | Chinese-CLIP、图片索引、公开 API、可见性过滤、真实 E2E、独立页面 | Phase 1、3 | 后端和独立页面已实现；浏览器验收待补 |
| Phase 5 RAG 检索 | 正文分块、FastEmbed、Qdrant、受限引用回答 | Phase 3 | 已完成 |
| Phase 6 私聊 Bot | 异步触发、幂等回复、引用二次校验、页面与 mock E2E | Phase 2、5 | 已完成 |
| Phase 7 Demo 与发布 | 固定数据、DeepSeek、40/30 评测、降级与演示 | Phase 4、6 | 工程能力和历史质量证据已形成；最终人工验收与 Docker 打包待补 |
| Phase 8 只读社区知识 Agent | LangGraph 受限工具循环、深度分析开关、步骤卡片、安全与 Agent 评测 | Phase 7、2、4、5、6 | 工程和机器门禁已通过；前端人工验收以 TASK 为准 |

当前执行顺序为：本机后端收口 → 后端自动验收 → Go Template 前端补齐与重构 → 本机自动回归 → 用户人工验收 → Docker/Compose 最终打包。浏览器操作和 Docker 工作在对应阶段前均搁置。群聊、写入型 Agent、外部工具和生产级平台能力不进入路线图。
