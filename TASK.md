# TASK.md — 当前任务

> 更新时间：2026-07-28 | 当前阶段：Phase 8B 进行中 | 暂缓：浏览器验收与 Docker/Compose 打包

## 当前状态

当前主线是：本机自动回归 → 用户人工验收 → Docker/Compose 最终打包。后端 API、RAG、Agent 和 Go Template 页面已经完成工程实现；浏览器人工验收、源码冷启动和最终发布复核仍未完成。

- [x] API 边界、聊天/RAG/Agent 自动化、readiness、源码指纹和失败语义已补齐。
- [x] 当前本机环境的 36 条 Agent 评测曾取得工具选择率 `0.9167`，其余机器门禁通过。该报告位于仓库外，工作区为 dirty，不作为干净提交的最终发布证据。
- [x] Go Template 页面和独立 `/search/images` 页面已实现，并通过模板、Handler、静态契约和本机自动检查。
- [x] 本机人工样本已扩展为 47 条 approved 帖子；图片和正文向量各 47 条，索引 pending 为 0。该语料只用于人工体验，不替代冻结评测集。
- [ ] 用户完成浏览器人工测试，并抽查 10 条 Agent 答案。
- [ ] 人工验收通过后，执行干净源码 Compose 冷启动、故障恢复、最终质量复核和发布证据归档。

## 固定边界

Go 对外提供 `GET /api/v1/search/images`；AI 内部使用 `POST /v1/search/images` 和 `POST /v1/rag/answer`；Agent 继续通过 `shareo_bot` 私聊中的 `ai_mode=agent` 触发，不新增公开 RAG/Agent API。

日常开发使用宿主机 Go/Python、Homebrew MySQL/Redis/MinIO 和本地 Qdrant。Docker/Compose 只在用户人工验收通过后恢复。不得重置本机数据、提交 `.env`、配置凭证、个人照片、模型缓存、原始响应或未脱敏报告。

## 已完成阶段

### Phase 7A：Demo 与冻结评测集

- [x] `make demo-seed` 幂等创建 26 条 Demo 帖子和图片索引。
- [x] 接入 26 张真实 JPEG 照片，完成 40 条搜图与 30 条 RAG 标注集。
- [x] 冻结 40 条搜图与 30 条 RAG 数据集，并完成静态、可见性和工程校验。

### Phase 7B：质量评测

- [x] 完成搜图/RAG 机器评测和 30 条人工相关性评分；历史平均分 `4.8667`，最低 `4`。
- [x] 历史报告的准确性、引用和无答案门禁通过；报告仍受 dirty workspace 和旧运行时元数据限制。

### Phase 7C：发布与演示

- [x] 历史工程门禁、真实集成、故障矩阵和 API 冒烟已有脱敏记录。
- [ ] 源码冷启动、浏览器五分钟演示、干净提交复评和最终发布证据待补。

### Phase 8A：只读 Agent 工程门禁

- [x] 多 tool call 闭合、Provider 失败轨迹、受控重试、预算限制、工具白名单、引用复核和源码指纹已实现并有测试。
- [x] 测试 Provider、Go/Python 单测、race、真实集成、本机 API 回归、RAG 40/30 回归和本机 Agent 机器评测已有证据。
- [x] 历史真实 DeepSeek 轮次中的 `0.6944`、`0.7778`、`0.8889` 保留为实验记录；最新本机工作区轮次为 `0.9167`，但仍需干净提交复评。

## 下一项工作

1. 用户按 [`frontend-manual-acceptance.md`](docs/evidence/phase8-agent/frontend-manual-acceptance.md) 完成页面、WebSocket、引用跳转、降级行为和 10 条 Agent 答案抽查。
2. 根据人工结果修复必要问题并重跑本机自动门禁；不得用人工评分掩盖机器失败。
3. 人工验收通过后恢复 Docker/Compose，完成源码构建、冷启动、幂等 seed、故障矩阵和最终脱敏证据。

## 证据入口

- 当前执行顺序：[`docs/plan.md`](docs/plan.md)
- 阶段边界：[`docs/phases/README.md`](docs/phases/README.md)
- API 与运行规则：[`docs/reference/api.md`](docs/reference/api.md)、[`docs/operations/runbook.md`](docs/operations/runbook.md)
- 评测规则与历史实验：[`docs/eval/README.md`](docs/eval/README.md)、[`docs/eval/experiments.md`](docs/eval/experiments.md)
- 人工验收清单：[`docs/evidence/phase8-agent/frontend-manual-acceptance.md`](docs/evidence/phase8-agent/frontend-manual-acceptance.md)
