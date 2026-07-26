# TASK.md — 当前任务

> 更新时间：2026-07-26 | 当前阶段：Phase 7C 进行中

## Phase 7A：Demo 数据与冻结评测集 ✅ 已完成

- [x] 实现可重复的 `make demo-seed`，固定 Demo 用户、管理员、`shareo_bot`、26 篇真实图片帖子正文。
- [x] 接入 26 张真实 JPEG 照片（`resources/static/pictures/`），重构 40 条搜图与 30 条 RAG 标注集。
- [x] `make check` 全部通过（Go tests, Python tests, ruff, shell syntax, doc check）。
- [x] RAG Bot 端到端通过 DeepSeek V4 Flash 验证，引用准确。

## Phase 7B：真实 Provider 与质量评测 ✅ 已完成

- [x] DeepSeek V4 Flash 已配置（`.env`，不入库），`/readyz/rag` 返回 ready。
- [x] 实现 `make eval-ai` 统一评测命令（搜图 + RAG）。
- [x] 修正评测报告、人工评分模板和质量门禁失败语义，补齐必要单元测试。
- [x] 使用当前 Compose 配置全新重建六个服务与数据库，并完成健康和 readiness 检查。
- [x] 连续运行两次 `make demo-seed`，并使用 `validate_eval_dataset.py` 校验数据和数据库可见性。
- [x] 运行完整搜图与 RAG 评测 `make eval-ai` 并记录质量指标。
- [x] 完成人工相关性评分（30 条，1–5 分，平均 4.8667）。
- [x] 达到 [Phase 7 质量门禁](docs/phases/phase-7-demo-evaluation-release.md#退出标准)：Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、失败请求 0。

## Phase 7C：发布与演示收口

- [ ] 运行自动检查、真实集成、race、图片 E2E、Bot E2E、评测和全新卷冷启动。
- [ ] 验证 AI、Qdrant、MinIO、Redis、DeepSeek 故障时的受控降级。
- [ ] 完成 README、运行手册、五分钟 Demo、截图和最终证据矩阵。
- [ ] 清理无效脚本、旧术语、未引用文件和意外工作区文件。

## 环境备忘

- `compose.yaml` 与 Colima 配置已经调整；用户已授权清空 Compose 项目数据并全新重建容器和数据库。
- 2026-07-26 已确认 Colima 正常运行，本机使用独立 `docker-compose`，Makefile 会自动回退。
- 已完成 Compose 六服务健康检查；MySQL 26 篇 approved 可见帖子、26 个帖子图片、MinIO 26/26/26 原图/中图/缩略图对象、Qdrant 图片/正文索引各 26 条，图片与 RAG readiness 均为 ready。保留源码修改、`.env` 和被忽略的本地真实照片。

## 证据入口

- [Phase 0–7 阶段索引](docs/phases/README.md)
- [当前执行计划](docs/plan.md)
- [架构与降级矩阵](docs/architecture.md)
- [AI 评测规范](docs/eval/README.md)
- [Phase 7B 最终机器报告](docs/eval/results/phase7b_final.json)
- [Phase 7B 人工评分表](docs/eval/results/phase7b_human_scoring.md)
- [五分钟演示规范](docs/demo.md)
