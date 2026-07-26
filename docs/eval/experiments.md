# AI 实验记录

> 当前状态：工程 E2E 与 Phase 7B 40/30 真实质量基线已有证据；Phase 7C 故障和演示实验待完成

| 日期 | 实验 | 数据集/配置 | 结果 | 结论 |
|---|---|---|---|---|
| 2026-07-22 | 语义搜图真实链路 | 2个隔离图文帖；Chinese-CLIP固定revision；CPU；512维 | 审核、命中、重复投递、重启重领、删除通过 | 只证明工程闭环，不是质量评测 |
| 2026-07-22 | 文本索引真实链路 | 2个隔离正文；BGE small zh；CPU；512维；400/80 | Qdrant写入、检索、删除和双索引事件通过 | 只证明检索工程能力 |
| 2026-07-23 | RAG Bot 端到端 | DeepSeek V4 Flash；20篇Demo帖；25条标注 | 富士XT5查询正确引用post_id=1 | 真实Provider验证通过 |
| 2026-07-26 | 40 条搜图质量 | `image_search_v1`；26 张真实 JPEG；Chinese-CLIP revision `36e679e65c2a2fead755ae21162091293ad37834`；CPU | Recall@5 0.8125；MRR 0.7771；重复帖子 0；P50/P95 58.3/64.8 ms | 搜图质量门禁通过 |
| 2026-07-26 | 30 条 RAG 质量 | `rag_qa_v1`；DeepSeek `deepseek-v4-flash`；BGE `BAAI/bge-small-zh-v1.5`；`rag-v1`；top_k 15；max_sources 10 | 来源命中 1.0000；引用可访问 100%；虚假引用 0；失败 0；P50/P95 3102.2/6174.7 ms；超时 0/30 | 机器门禁通过 |
| 2026-07-26 | RAG 人工相关性 | 同上；30 条完整评分 | 26 条 5 分、4 条 4 分；平均 4.8667/5 | 人工门禁通过 |

## Phase 7B 最终实验记录

- 评测代码 SHA：`aad134cf78ecaa6484f3fb5db21d7dd9cce0a586`；日期：2026-07-26；实际设备：CPU；网络环境：`unspecified`。
- `image_search_v1` SHA256：`525c871c8f385d1d2bf3822e7edfc2aa9e4fec866cc6999b118671c79b3be8c7`；`rag_qa_v1` SHA256：`19e92621377ba919f1a7c55e0b6253030a27aec630cc91d8cb4e7e91d7d2ea44`。
- 最终报告：[`phase7b_final.json`](results/phase7b_final.json)；评分表：[`phase7b_human_scoring.md`](results/phase7b_human_scoring.md)。报告未包含 API Key、Token、Cookie 或敏感请求头。
- 人工反馈指出：回答总体准确；部分回答参数较多、语言不够自然，“是什么”类问题相对简略。后续可在 Phase 7C 记录 Prompt 表达风格实验，但不得用风格调整替代本轮已通过的准确性和引用门禁。
