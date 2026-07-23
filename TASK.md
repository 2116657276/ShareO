# TASK.md — 当前任务

> 更新时间：2026-07-23 | 当前阶段：Phase 7 进行中

## Phase 7A：Demo 数据与冻结评测集 ✅ 基本完成

- [x] 实现可重复的 `make demo-seed`，固定 Demo 用户、管理员、`shareo_bot`、帖子、图片和正文。
- [ ] 完成 40 条搜图标注：因无真实图片素材，Chinese-CLIP 语义搜图质量评测推迟。已生成合成占位图，查询框架和数据集已就绪。
- [x] 新增并完成 30 条 RAG 标注：25 条已标注（factual 9、advice 6、comparison 4、multi-source 3、no-answer 3），5 条 portrait/motion 类因对应帖子未创建暂挂。
- [x] `make check` 全部通过（Go tests, Python tests, ruff, shell syntax, doc check）。
- [x] `make demo-seed` 成功创建 20 篇 approved 帖子，索引完成。
- [x] RAG Bot 端到端通过 DeepSeek V4 Flash 验证，引用准确。

## Phase 7B：真实 Provider 与质量评测 🔄 进行中

- [x] DeepSeek V4 Flash 已配置（`.env`，不入库），`/readyz/rag` 返回 ready。
- [x] 实现 `make eval-ai` 统一评测命令（搜图 + RAG）。
- [x] RAG 评测框架就绪，25 条已标注数据可运行。
- [ ] 运行完整 RAG 评测并记录质量指标。
- [ ] 人工相关性评分（1-5 scale, target avg ≥ 4.0）。
- [ ] 达到 [Phase 7 质量门禁](docs/phases/phase-7-demo-evaluation-release.md#退出标准)。

## Phase 7C：发布与演示收口

- [ ] 运行自动检查、真实集成、race、图片 E2E、Bot E2E、评测和全新卷冷启动。
- [ ] 验证 AI、Qdrant、MinIO、Redis、DeepSeek 故障时的受控降级。
- [ ] 完成 README、运行手册、五分钟 Demo、截图和最终证据矩阵。
- [ ] 清理无效脚本、旧术语、未引用文件和意外工作区文件。

## 当前阻塞

- 40 条搜图标注和评测依赖真实图片素材（非多模态模型无法采集合适图片）。
- 5 条 RAG portrait/motion 标注等待补齐对应 Demo 帖子。
- 搜图质量门禁（Recall@5 ≥ 0.70, MRR ≥ 0.55）需真实图片才能评测。

## 已完成

- `make demo-seed` 可运行，幂等，创建 20 篇 approved 帖子
- 25/30 RAG 标注完成，数据集校验通过
- `make check` 全部门禁通过
- DeepSeek V4 Flash 接入，RAG Bot 端到端验证通过
- 统一评测命令 `make eval-ai` 已实现

## 证据入口

- [Phase 0–7 阶段索引](docs/phases/README.md)
- [当前执行计划](docs/plan.md)
- [架构与降级矩阵](docs/architecture.md)
- [AI 评测规范](docs/eval/README.md)
- [五分钟演示规范](docs/demo.md)
