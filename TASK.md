# TASK.md — 当前任务

> 更新时间：2026-07-23 | 当前阶段：Phase 7 进行中

## Phase 7A：Demo 数据与冻结评测集

- [ ] 实现可重复的 `make demo-seed`，固定 Demo 用户、管理员、`shareo_bot`、帖子、图片和正文。
- [ ] 完成 40 条搜图标注：物体 8、场景 8、颜色 6、风格 6、构图 4、人像 4、运动 2、无匹配 2。
- [ ] 新增并完成 30 条 RAG 标注：事实 10、建议 8、比较 5、多来源 4、无答案 3。
- [ ] 全新卷连续执行两次 seed 结果一致，所有引用指向 approved、未删除帖子。

## Phase 7B：真实 Provider 与质量评测

- [ ] 使用 DeepSeek 完成一次非 mock 全链路演示。
- [ ] 实现 `make eval-ai` 并归档搜图、RAG、人工评分和延迟报告。
- [ ] 达到 [Phase 7 质量门禁](docs/phases/phase-7-demo-evaluation-release.md#退出标准)。

## Phase 7C：发布与演示收口

- [ ] 运行自动检查、真实集成、race、图片 E2E、Bot E2E、评测和全新卷冷启动。
- [ ] 验证 AI、Qdrant、MinIO、Redis、DeepSeek 故障时的受控降级。
- [ ] 完成 README、运行手册、五分钟 Demo、截图和最终证据矩阵。
- [ ] 清理无效脚本、旧术语、未引用文件和意外工作区文件。

## 当前阻塞

- 40 条搜图数据仍为空标签，不能作为质量证据。
- 30 条 RAG 数据集、Demo seed 和统一评测命令尚未实现。
- 真实 DeepSeek 验收需要由环境变量提供 API Key；密钥不得写入仓库或证据文件。

## 证据入口

- [Phase 0–7 阶段索引](docs/phases/README.md)
- [当前执行计划](docs/plan.md)
- [架构与降级矩阵](docs/architecture.md)
- [AI 评测规范](docs/eval/README.md)
- [五分钟演示规范](docs/demo.md)
