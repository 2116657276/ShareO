# Phase 7 — Demo、评测与发布

> 状态：7A、7B 与 7C 工程门禁已完成；人工演示、源码重建和干净复评待补证 | 顺序：7A → 7B → 7C

## 目标

把已有工程能力收口为可重复数据、可信质量报告和五分钟稳定演示。

## 当前基线

图片与 Bot 工程 E2E、Demo seed、40/30 数据集、统一评测命令、真实 provider 质量测试和故障降级已有证据。现有 5.08 秒记录是 API 自动化冒烟，不是人工浏览器五分钟演示；源码镜像构建也因网络卡住而改用已有镜像。两份最终评测报告均来自 dirty workspace，Phase 7C 的 LLM 模型元数据还未从被测容器采集。

## 范围与非目标

只做数据、评测、文档、故障验证和演示收口；不新增业务功能、前端框架、模型训练、Agent或多 provider 路由。

## 依赖

依赖 Phase 0–6工程闭环，以及由环境变量提供的 DeepSeek配置。

## 工作包

- 7A：固定 Demo 数据，完成 40 条搜图和 30 条 RAG标注。
- 7B：真实 DeepSeek、统一评测、人工评分和性能记录。
- 7C：完整工程门禁、独立 Compose 故障降级、README、运行手册、脱敏证据和人工五分钟演示。

## 接口与数据流

不新增公开 API。`make demo-seed` 只准备演示数据；`make eval-ai` 通过现有搜图和 RAG接口采集结果并输出不含密钥的报告。

## 数据一致性

全新卷两次 seed 必须收敛到相同数据；数据集标签只引用 approved、未删除帖子；评测报告记录数据集、模型和 Prompt 版本。

## 安全边界

DeepSeek API Key只读环境变量；日志、报告、截图和 Git不得包含密钥、Authorization header或完整敏感响应。

## 失败模式

数据标签漂移、模型版本变化、公网延迟和依赖故障必须被报告；不得通过修改阈值或删除失败查询制造通过结果。

## 测试矩阵

- 7A：幂等 seed、类别分布、标签存在性与可见性。
- 7B：全新 Compose 数据卷初始化、两次幂等 seed、40 条搜图、30 条 RAG、真实 provider、人工评分、引用验证和本地 P95。
- 7C：检查、集成、race、两套 E2E、源码构建冷启动、五类故障、API 冒烟和人工浏览器演示。

## 退出标准

- 搜图 Recall@5 ≥ 0.70、MRR ≥ 0.55；无重复或不可见帖子。
- RAG 来源命中率 ≥ 0.80、引用可访问率 100%、虚假引用 0、人工相关性平均 ≥ 4.0/5。
- 图片搜索预热后 MPS P95 ≤ 2 秒、CPU P95 ≤ 4 秒；RAG 本地检索 P95 ≤ 2 秒。
- DeepSeek 总延迟记录 P50/P95与超时率但不设硬门禁。
- 代码、自动测试、真实环境、评测、降级、源码构建和人工演示证据全部齐全。

当前质量指标门禁已通过，但性能证据尚不满足上述定义：`eval_ai` 只记录端到端总延迟和失败数，没有分离本地检索 P95 或超时率。

## 提交与环境证据

Phase 7A/7B 交付物：
- `scripts/demo_seed.sh` — 可重复 Demo 数据初始化（支持 26 张真实照片素材）
- `scripts/validate_eval_dataset.py` — 评测数据集校验
- `docs/eval/image_search_v1.jsonl` — 40 条搜图查询（基于 26 张真实照片全量标注）
- `docs/eval/rag_qa_v1.jsonl` — 30 条 RAG 问答（基于 26 张真实照片全量标注）
- `ai-service/app/commands/eval_ai.py` — 统一评测命令
- `.env` — DeepSeek V4 Flash 配置（不入库）

已有工程基线：`bb9732f`、`0f1785d`、`17b54ec`、`0aff22e`、`56e30f4`、`aad134c`。

本轮 7B 证据（2026-07-26）：
- 六服务 Compose 启动并通过健康检查；图片和 RAG readiness 均为 ready，实际设备为 CPU，图片与正文 Qdrant collection 各有 26 条索引。
- 两次 `make demo-seed` 收敛到 26 篇 approved 可见帖子、26 个帖子图片和原图/中图/缩略图各 26 个 MinIO 对象；40/30 数据集静态和数据库可见性校验通过。
- `docs/eval/results/phase7b_final.json` 为最终机器报告，`docs/eval/results/phase7b_human_scoring.md` 为人工评分表；报告不含 API Key、Token、Cookie 或敏感请求头。
- 报告记录的基线 SHA 为 `aad134cf78ecaa6484f3fb5db21d7dd9cce0a586`，同时标记 `git_dirty=true`，因此不能把该 SHA 当作可精确重建的评测代码树；搜图数据集 SHA256 为 `525c871c8f385d1d2bf3822e7edfc2aa9e4fec866cc6999b118671c79b3be8c7`，RAG 数据集 SHA256 为 `19e92621377ba919f1a7c55e0b6253030a27aec630cc91d8cb4e7e91d7d2ea44`。报告记录 top_k 15、最大来源 10、Prompt `rag-v1` 和 `deepseek-v4-flash`。
- 质量指标门禁通过：Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、失败请求 0、无答案准确率 1.0000、人工平均分 4.8667（26 条 5 分、4 条 4 分）。图片 P50/P95 为 58.3/64.8 ms，RAG 端到端总延迟 P50/P95 为 3102.2/6174.7 ms；报告没有独立超时率字段。
- 人工复核反馈：回答总体准确；部分回答参数较多、语言不够自然，“是什么”类问题相对简略。该项作为 Phase 7C 的可选 Prompt/表达风格优化记录，不改变本轮质量门禁结论。

本轮 Phase 7C 证据（2026-07-26）：

- `make check`、真实 MySQL/Redis/Qdrant 集成、`go test -race ./...`、图片 E2E 和 Bot E2E 均通过；独立项目 `shareo-degradation` 完成 AI、Qdrant、MinIO、Redis、测试 LLM provider 五类故障注入与恢复，最终基线再次通过。
- 按运行手册完成一次真实 DeepSeek provider 断开复核：AI health 保持 200，RAG readiness 和请求受控返回 503，普通私聊 HTTP 200，Bot 固定兜底成功；恢复后 readiness 再次为 200。
- 使用已有镜像启动清理后的新卷，完成六服务健康、图片/RAG readiness、两次成功 seed 和对象/索引对账：26 篇帖子、26 个图片记录、78 个 MinIO 对象、`images`/`post_chunks` 各 26 点。源码镜像构建未通过，不能称为完整冷启动复现。
- 运行时代码修复了 Redis Stream consumer 在 Redis 重启后收到 `NOGROUP` 时自动重建消费组，并重新运行 40/30 评测。数据集和 Prompt 未修改；“模型版本未改变”无法由现有报告证明。
- 复核报告 [`phase7c_revalidated_final.json`](../eval/results/phase7c_revalidated_final.json) 指标为 Recall@5 0.8125、MRR 0.7771、来源命中率 0.9667、引用可访问率 100%、虚假引用 0、失败请求 0、人工平均 5.0；总延迟 P50/P95 为 3095/5195.8 ms。
- API 计时冒烟耗时 5.08 秒，Bot 回复含 2 条引用；它没有覆盖浏览器人工操作、WebSocket 实时下行和引用点击。脱敏证据索引为 [`evidence-matrix.md`](../evidence/phase7c/evidence-matrix.md)，记录为 [`demo-run.md`](../evidence/phase7c/demo-run.md)。
- Phase 7B 基线提交为 `a3b02e9e51004ef5cf4bb25f1d33d5bb92298768`；Phase 7C 工程收口提交为 `91d470feadf5b735cdbba4479e621474ff360c8f`，尚未推送远程。

## 遗留项

- 完成人工浏览器五分钟演示并归档脱敏证据。
- 在不复用业务镜像的环境完成源码构建冷启动。
- 修正评测器的被测运行时元数据采集，在干净提交上复跑 40/30。
- 增加 RAG embedding、检索、队列与 LLM 分段延迟及超时分类，补齐性能目标证据。
- 回答长度与“是什么/为什么/怎么样”风格差异只作为后续 Prompt 优化候选，不扩展产品范围。
