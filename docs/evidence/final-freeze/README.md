# Final Freeze 证据归档

本目录保存 ShareO 本轮本机自动化测试的脱敏汇总。统一入口为：

```bash
make final-evidence
```

原始命令输出和完整评测 JSON 保存于 `.local/shareo/final-freeze/<run_id>/`，不进入仓库证据目录。仓库内只保留运行清单、runtime snapshot、命令结果、质量指标、延迟摘要和失败矩阵。`run-manifest.json` 还包含四条关键链路的脱敏场景记录：状态、时间窗口、引用数量、工具序列、重试、失败类别和最终断言；临时帖子、消息和任务 ID 不进入仓库汇总。

本目录中的 AI 质量报告是在 2026-08-11 PostgreSQL/pgvector 迁移前采集的历史证据，运行时为 Homebrew MySQL/Redis/MinIO 与本地 Qdrant；这些报告用于保留模型和评测边界，不代表当前运行时仍访问旧服务。历史报告使用当前 47 条本机帖子语料：38 条搜图开发集（含 8 条 no-match）、32 条搜贴开发集、30 条 v1 RAG 题和 36 条 v1 Agent 题。迁移后的 PostgreSQL/pgvector、API 和 seed 重验结果见 [`docs/REVIEW.md`](../../REVIEW.md) 与 [`TASK.md`](../../../TASK.md)。Compose E2E 与故障注入不属于本轮范围，并在失败矩阵中标记为 `not_run`。

当前证据目录的最新报告运行是 `20260808T151704Z`，使用当前代码树、v1 的 38/32/30/36 四组数据集和真实 Provider；14 个自动化步骤通过，AI judge 步骤完成但状态为 `needs_human_review`。其中采集前的工程检查为避免复用旧证据，跳过了当前 Final Freeze 证据门禁；这不等同于完整 `make check` 已通过。运行结果绑定 Git SHA、dirty 状态、数据集 SHA256、模型和运行时版本。`retained-runs.json` 当前仍指向较早的 `20260808T091649Z`，因此最新报告尚未成为一致的权威归档；该归档也尚未完成计划中的三轮 AI 复评和稳定源码指纹收口，不能表述为干净提交上的最终复现结果。此前运行已作为历史对照保留，运行记录见 [`retained-runs.json`](retained-runs.json)。

本次机器结果为：搜图 Recall@5 `0.9222`、no-match accuracy `1.0`；混合搜贴 Recall@5 `0.8906`；RAG 完整来源覆盖率 `0.9333`、引用精度 `0.9000`、引用可访问率 `100%`、额外引用 `7`、虚假引用 `0`、P95 `5.13s`；Agent 完整来源覆盖率 `0.9722`、引用精度 `0.9583`、额外工具调用 `0`、注入拒答失败 `0`、P95 `8.17s`。两轮 AI judge 的 RAG 平均 `4.1667`、最低 `1`，Agent 平均 `4.8056`、最低 `1`；10 条答案进入 [AI judge 人工复核交接包](ai-judge-human-handoff.json)，因此 AI 质量门禁尚未通过。

## 关键链路

```text
历史（迁移前）：发布 → 审核 → index_post → Redis Streams → Chinese-CLIP/FastEmbed → Qdrant → 搜索 → Go 可见性复核

当前（迁移后）：发布 → 审核 → index_post → Redis Streams → Chinese-CLIP/FastEmbed → PostgreSQL `ai` schema / pgvector → 搜索 → Go 可见性复核

私聊 → 消息 + bot_task_outbox 同事务 → 重试发布 bot_tasks → RAG → 引用白名单 → Go 二次复核 → Bot 回复 → WebSocket

ai_mode=agent → LangGraph → 四类只读工具 → 脱敏轨迹 → 引用复核 → 回复
```

自动化记录测试状态、运行参数、数据集 SHA256、模型和 revision、Git SHA/dirty 状态、引用数量、工具序列、重试、失败类别以及可取得的 P50/P95。不得把 API Key、Token、Cookie、Authorization header、完整私聊正文或原始 Provider 请求写入本目录。

`latency-summary.json` 会区分逐题评测结果和从 AI 结构化日志中提取的运行窗口聚合结果。当前 Agent 工具步骤耗时和 embedding/retrieval/LLM 可用性标记已归档；Agent 单独 LLM 分段样本、队列等待、postprocess 和 callback 没有可靠字段时记录为 `unavailable`，不进行估算。AI judge 自身记录模型、Prompt SHA、批次耗时和两轮差异。

本次普通用户登录后的社区、评论点赞、私聊未读、Bot 引用卡片、Agent 步骤、空状态和双主题浏览器 smoke 摘要见 [`manual-ui-check.md`](manual-ui-check.md)。应用内浏览器自动化因本机连接隔离未形成通过证据，因此只保留人工浏览器 smoke 作为补充证据；不宣称覆盖完整浏览器 E2E。工作区 dirty 文件清单见 [`worktree-manifest.json`](worktree-manifest.json)，未知范围未自动删除或提交。
