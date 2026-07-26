# Phase 7 — Demo、评测与发布

> 状态：7A、7B 已完成，7C 进行中 | 顺序：7A → 7B → 7C

## 目标

把已有工程能力收口为可重复数据、可信质量报告和五分钟稳定演示。

## 当前基线

图片与 Bot 工程 E2E 已通过；Demo seed、40/30 评测数据集、统一评测命令和真实 DeepSeek 质量报告已完成；故障降级、五分钟演示和最终发布证据属于尚未完成的 Phase 7C。

## 范围与非目标

只做数据、评测、文档、故障验证和演示收口；不新增业务功能、前端框架、模型训练、Agent或多 provider 路由。

## 依赖

依赖 Phase 0–6工程闭环，以及由环境变量提供的 DeepSeek配置。

## 工作包

- 7A：固定 Demo 数据，完成 40 条搜图和 30 条 RAG标注。
- 7B：真实 DeepSeek、统一评测、人工评分和性能记录。
- 7C：完整门禁、故障降级、README、运行手册、截图和五分钟演示。

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
- 7C：检查、集成、race、两套 E2E、冷启动、五类故障和演示计时。

## 退出标准

- 搜图 Recall@5 ≥ 0.70、MRR ≥ 0.55；无重复或不可见帖子。
- RAG 来源命中率 ≥ 0.80、引用可访问率 100%、虚假引用 0、人工相关性平均 ≥ 4.0/5。
- 图片搜索预热后 MPS P95 ≤ 2 秒、CPU P95 ≤ 4 秒；RAG 本地检索 P95 ≤ 2 秒。
- DeepSeek 总延迟记录 P50/P95与超时率但不设硬门禁。
- 代码、自动测试、真实环境、评测、降级和文档证据全部齐全。

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
- 评测代码 SHA 为 `aad134cf78ecaa6484f3fb5db21d7dd9cce0a586`；搜图数据集 SHA256 为 `525c871c8f385d1d2bf3822e7edfc2aa9e4fec866cc6999b118671c79b3be8c7`，RAG 数据集 SHA256 为 `19e92621377ba919f1a7c55e0b6253030a27aec630cc91d8cb4e7e91d7d2ea44`；Chinese-CLIP revision 为 `36e679e65c2a2fead755ae21162091293ad37834`，BGE 为 `BAAI/bge-small-zh-v1.5`，检索参数为 top_k 15、最大来源 10、Prompt `rag-v1`，DeepSeek 模型为 `deepseek-v4-flash`。
- 质量门禁全部通过：Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、失败请求 0、无答案准确率 1.0000、人工平均分 4.8667（26 条 5 分、4 条 4 分）。图片 P50/P95 为 58.3/64.8 ms，RAG 总延迟 P50/P95 为 3102.2/6174.7 ms，超时率为 0/30。
- 人工复核反馈：回答总体准确；部分回答参数较多、语言不够自然，“是什么”类问题相对简略。该项作为 Phase 7C 的可选 Prompt/表达风格优化记录，不改变本轮质量门禁结论。

## 遗留项

- Phase 7C：运行完整自动检查、真实集成、race、图片/Bot E2E 和冷启动；验证 AI、Qdrant、MinIO、Redis、DeepSeek 故障时的受控降级；完成五分钟演示和最终证据矩阵。
- 回答长度与“是什么/为什么/怎么样”风格差异只作为后续 Prompt 优化候选，不扩展产品范围，也不提前宣称 Phase 7C 完成。
