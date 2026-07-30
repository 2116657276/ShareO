# 当前语料评测规范

> 当前唯一有效的质量评测入口是 [`docs/evidence/final-freeze/README.md`](../evidence/final-freeze/README.md)。旧的 26 条 Demo 冻结输入和旧报告已退役，不再作为当前命令的默认输入。

## 当前评测数据

当前评测绑定本机 `local-photo-seed` 生成的 47 条 approved 帖子和对应向量。评测数据保存在 Git 忽略目录 `.local/shareo/eval/`，由脚本按当前 manifest 生成，不把个人照片或完整私聊内容放入仓库。

| 集合 | 数量 | 当前文件 | 用途 |
|---|---:|---|---|
| 搜图 | 34 条 | `image_search_local_v1.jsonl` | graded relevance 开发诊断 |
| 搜贴 | 32 条 | `post_search_v1.jsonl` | keyword、semantic、hybrid 对比 |
| RAG | 30 条 | `rag_qa_current_v1.jsonl` | 当前语料来源和引用机器门禁 |
| Agent | 36 条 | `agent_tasks_current_v1.jsonl` | 当前语料工具、安全和引用机器门禁 |

生成和运行当前集合：

```bash
make prepare-search-eval
make prepare-current-ai-eval
make eval-post-search DATASET=.local/shareo/eval/post_search_v1.jsonl
make eval-image-search-local DATASET=.local/shareo/eval/image_search_local_v1.jsonl
SHAREO_IMAGE_DATASET=.local/shareo/eval/image_search_local_v1.jsonl \
SHAREO_RAG_DATASET=.local/shareo/eval/rag_qa_current_v1.jsonl \
  make eval-ai MACHINE_ONLY=1
SHAREO_AGENT_DATASET=.local/shareo/eval/agent_tasks_current_v1.jsonl \
  make eval-agent MACHINE_ONLY=1
```

统一归档使用 `make final-evidence`。它记录数据集 SHA256、Git SHA、dirty 状态、运行时模型和指标；完整原始输出只保存在本机 `.local/shareo/final-freeze/<run_id>/`。

## 指标与当前结果

搜图记录 Recall@5、Recall@10、MRR、nDCG@5、Precision@3、无匹配准确率、重复结果和端到端延迟。搜贴分别记录 keyword-only、semantic-only 和 hybrid 的上述排序指标及延迟。

RAG 记录来源命中率、引用可访问率、虚假引用、失败请求、无答案准确率、平均引用数和端到端 P50/P95。人工评分不由机器报告代替；当前运行没有生成或宣称人工评分。

Agent 记录来源命中率、必需工具选择率、引用可访问率、禁止工具、预算越界、提示词注入成功、失败、超时、Provider 重试和端到端 P50/P95。Agent 题目的 expected tool 按当前问题意图标注：精确短语使用关键词检索，多来源比较使用关键词检索加帖子读取；这不是调低阈值。

当前语料已运行两次真实 Provider：`20260730T140713Z` 与 `20260730T143316Z`，两次本机原始包均保留。最新运行 `143316Z` 作为现行基线，两次对照见 [保留运行记录](../evidence/final-freeze/retained-runs.json)；最新完整结果见 [质量汇总](../evidence/final-freeze/quality-summary.json)、[延迟汇总](../evidence/final-freeze/latency-summary.json) 和 [运行清单](../evidence/final-freeze/run-manifest.json)。

## 运行规则

- 只使用当前 47 条帖子对应的本机集合，不回退到旧文件。
- 不修改 Prompt、模型、数据标签、检索参数或质量阈值来制造通过结果。
- 单个命令只执行一次；失败保留退出码、失败类别和脱敏摘要。
- 结果统一使用 `pass`、`fail`、`blocked`、`not_run`。
- 不记录 API Key、Token、Cookie、Authorization header、完整 Provider 请求、完整私聊内容或个人照片。
- 当前结果属于特定本机语料和真实 Provider 的诊断证据，不外推为线上 SLA、通用准确率或第三方评审。

## 证据边界

当前运行的代码树和工作区状态记录在 Final Freeze 清单中；由于本次工作区为 dirty，结果不能被表述为干净提交上的可重建发布复评。Compose 故障注入、空缓存冷启动和完整浏览器人工演示仍按最终证据中的状态处理。
