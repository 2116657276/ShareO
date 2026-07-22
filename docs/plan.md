# ShareO 当前执行计划

> 更新时间: 2026-07-22 | 当前主线: Phase 2 真实 E2E 门禁 | 状态事实源: [TASK.md](../TASK.md)

## 执行顺序

1. **当前阻塞：Phase 2 worker 收敛**：补齐 worker readiness、预热和索引失败日志，定位隔离 E2E 未命中的首个失败点。
2. **Phase 2 真实闭环**：模型预热、Qdrant schema、隔离 E2E、回填与恢复。
3. **Phase 2 可靠性和评测**：索引对账、结构化指标、结果去重、40 条查询集与性能报告。
4. **发布验收线**：评测通过后实现最小搜索页面，同时完成 Phase 1 浏览器验收。
5. **Phase 3 Bot + RAG**：先检索与引用，再接 LLM 和 IM Bot。
6. **Phase 4 Agent**：仅作扩展目标，Phase 3 达标后启动。

## 当前执行状态

2026-07-22 阶段 0 评审未通过：真实依赖已启动，Go/Python/Shell/Markdown 本地门禁通过，但 approved 图片未在 180 秒内完成 worker 索引并命中搜索。当前不进入 Phase 3 运行时代码；阶段证据和后续动作见 [Phase 2 门禁评审](reviews/2026-07-22-phase2-gate-review.md)。

## 阶段文档

| 阶段 | 文档 | 当前状态 | 核心门禁 |
|------|------|----------|----------|
| Phase 0 基建 | [phase-0-foundation.md](phases/phase-0-foundation.md) | 本地完成，远端 CI 待确认 | 可复现构建、真实依赖、可靠队列 |
| Phase 1 IM | [phase-1-im.md](phases/phase-1-im.md) | 后端候选，发布验收待后 | 事务一致性、恢复、安全、双浏览器 |
| Phase 2 语义搜图 | [phase-2-image-search.md](phases/phase-2-image-search.md) | 真实 E2E 门禁阻塞 | 真实闭环、对账、质量/性能评测 |
| Phase 3 RAG | [phase-3-rag.md](phases/phase-3-rag.md) | 未启动，等待 Phase 2 | 引用正确率、降级、成本/质量报告 |
| Phase 4 Agent | [phase-4-agent.md](phases/phase-4-agent.md) | 扩展目标 | 工具安全、多步任务评测 |

## 跨阶段统一门禁

- `make check`
- `make test-integration`（真实依赖，不以 skip 代替）
- `go test -race ./...`
- Shell 语法、Compose/Qdrant readiness、文档链接和工作区检查
- 计划项必须同时具备代码提交、自动测试、真实环境证据和文档记录

前端恢复条件固定为：Phase 2 真实闭环完成，Recall@10 ≥ 0.75、MRR ≥ 0.55，且降级测试通过。
