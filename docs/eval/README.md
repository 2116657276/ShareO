# AI 评测

轻量项目只维护两份冻结数据集：

- `image_search_demo_v1.jsonl`：12 条中文搜图查询，记录期望帖子 ID。
- `rag_qa_demo_v1.jsonl`：15 条问题、期望来源帖子 ID 和参考答案。

`make eval-ai` 输出 Recall@5、来源命中率、引用可访问率以及检索、LLM、总耗时。至少运行一次真实云端 provider，但不做多 provider 成本对照。
