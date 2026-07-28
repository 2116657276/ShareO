# ShareO 文档索引

## 事实与执行

- [单上下文](../CONTEXT.md)
- [当前任务](../TASK.md)
- [当前执行计划](plan.md)
- [交付路线图](roadmap.md)
- [Phase 0–8](phases/README.md)

## 架构与参考

- [已实现架构](architecture.md)
- [最终功能矩阵](features.md)
- [API参考](reference/api.md)
- [数据模型](reference/data-model.md)
- [运行手册](operations/runbook.md)
- [五分钟演示](demo.md)
- [简历项目审查](resume-review.md)

## 设计、决策与评测

- [一对一私聊](design/im.md)
- [中文语义搜图](design/image-search.md)
- [引用式RAG](design/rag.md)
- [只读社区知识 Agent](design/agent.md)
- [ADR](adr/)
- [AI评测](eval/README.md)

`TASK.md` 是唯一实时状态源。阶段文档维护边界、门禁和证据；设计文档不维护任务状态；Git历史承担已删除旧评审的追溯职责。

本文档中的“Agent”指产品能力；当前产品默认仍实现固定 RAG Bot，并在显式深度分析模式下提供受限只读 Agent，阶段状态以 `TASK.md` 和阶段证据为准。开发代理的协作规则统一由根目录 `AGENTS.md` 管理。
