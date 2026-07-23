# Phase 7 — Demo、评测与发布

> 状态：7A 基本完成，7B 进行中 | 顺序：7A → 7B → 7C

## 目标

把已有工程能力收口为可重复数据、可信质量报告和五分钟稳定演示。

## 当前基线

图片与 Bot 工程 E2E 已通过；Demo seed、30 条 RAG 数据、统一评测命令、真实 DeepSeek报告和最终降级证据尚未完成。

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
- 7B：40 条搜图、30 条 RAG、真实 provider、人工评分和本地 P95。
- 7C：检查、集成、race、两套 E2E、冷启动、五类故障和演示计时。

## 退出标准

- 搜图 Recall@5 ≥ 0.70、MRR ≥ 0.55；无重复或不可见帖子。
- RAG 来源命中率 ≥ 0.80、引用可访问率 100%、虚假引用 0、人工相关性平均 ≥ 4.0/5。
- 图片搜索预热后 MPS P95 ≤ 2 秒、CPU P95 ≤ 4 秒；RAG 本地检索 P95 ≤ 2 秒。
- DeepSeek 总延迟记录 P50/P95与超时率但不设硬门禁。
- 代码、自动测试、真实环境、评测、降级和文档证据全部齐全。

## 提交与环境证据

Phase 7A 交付物：
- `scripts/demo_seed.sh` — 可重复 Demo 数据初始化
- `scripts/validate_eval_dataset.py` — 评测数据集校验
- `docs/eval/image_search_v1.jsonl` — 40 条搜图查询（标注待真实图片）
- `docs/eval/rag_qa_v1.jsonl` — 30 条 RAG 问答（25 条已标注）
- `ai-service/app/commands/eval_ai.py` — 统一评测命令
- `.env` — DeepSeek V4 Flash 配置（不入库）

已有工程基线：`bb9732f`、`0f1785d`、`17b54ec`、`0aff22e`、`56e30f4`。

## 遗留项

- 40 条搜图质量评测依赖真实图片素材（非多模态大模型无法采集）
- 5 条 RAG portrait/motion 标注待补齐对应帖子
- 完成后只保留可选论文材料整理，不再扩展产品范围。
