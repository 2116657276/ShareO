# 当前语料评测规范

> 当前证据归档入口是 [`docs/evidence/final-freeze/README.md`](../evidence/final-freeze/README.md)。旧的 26 条 Demo 冻结输入和旧报告已退役，不再作为当前命令的默认输入。

本页严格区分两种状态：当前证据目录的最新报告运行仍绑定 v1 RAG/Agent 数据集；代码默认评测输入为 v2，已按新的来源支持审计规则生成，但尚未完成三轮复评和新的证据归档，因此不能用 v2 改写已归档指标。

## 当前评测数据

当前评测绑定本机 `local-photo-seed` 生成的 47 条 approved 帖子和对应向量。评测数据保存在 Git 忽略目录 `.local/shareo/eval/`，由脚本按当前 manifest 生成，不把个人照片或完整私聊内容放入仓库。

| 集合 | 数量 | 报告绑定文件 | 用途 |
|---|---:|---|---|
| 搜图 | 38 条 | `image_search_local_v1.jsonl` | graded relevance 与 8 条 no-match 拒答诊断 |
| 搜贴 | 32 条 | `post_search_v1.jsonl` | keyword、semantic、hybrid 对比 |
| RAG | 30 条 | `rag_qa_current_v1.jsonl` | 当前报告运行使用的来源和引用机器门禁输入 |
| Agent | 36 条 | `agent_tasks_current_v1.jsonl` | 当前报告运行使用的工具、安全和引用机器门禁输入 |

生成和运行当前集合：

```bash
make prepare-search-eval
make prepare-current-ai-eval VERSION=v2  # 生成当前默认的待归档数据集
make eval-post-search DATASET=.local/shareo/eval/post_search_v1.jsonl
make eval-image-search-local DATASET=.local/shareo/eval/image_search_local_v1.jsonl
SHAREO_IMAGE_DATASET=.local/shareo/eval/image_search_local_v1.jsonl \
SHAREO_RAG_DATASET=.local/shareo/eval/rag_qa_current_v2.jsonl \
  make eval-ai MACHINE_ONLY=1
SHAREO_AGENT_DATASET=.local/shareo/eval/agent_tasks_current_v2.jsonl \
  make eval-agent MACHINE_ONLY=1
```

对指定的机器报告，可用已配置的真实 Provider 执行两轮 AI judge；当前报告运行已经完成这一步，但因低分条目仍需人工复核，质量门禁没有通过：

```bash
set -a; . ./.env; set +a
make eval-ai-judge \
  RAG_REPORT=.local/shareo/eval/rag-image.json \
  AGENT_REPORT=.local/shareo/eval/agent.json \
  OUTPUT=.local/shareo/eval/quality-judgement.json
```

AI judge 只读取脱敏评测行，不修改题目、答案、数据集或阈值；完整 Final Freeze 会自动加载 `.env` 中的非空 Provider 配置，且不会把密钥写入证据。

当前证据报告使用 v1 数据集；v2 题目增加了来源支持审计和更严格的多来源覆盖检查，是当前代码默认的待归档输入。只有完成新的三轮复评并更新证据归档后，v2 才能替代 v1 的报告指标。

统一归档使用 `make final-evidence`。它记录数据集 SHA256、Git SHA、dirty 状态、运行时模型和指标；完整原始输出只保存在本机 `.local/shareo/final-freeze/<run_id>/`。

## 指标与当前结果

搜图记录 Recall@5、Recall@10、MRR、nDCG@5、Precision@3、无匹配准确率、重复结果和端到端延迟。搜贴分别记录 keyword-only、semantic-only 和 hybrid 的上述排序指标及延迟。

RAG 记录来源命中率、完整来源覆盖率、引用 precision、额外引用、引用可访问率、虚假引用、失败请求、超时分类、无答案准确率、平均引用数和端到端 P50/P95。当前报告运行执行了两轮 `llm_judge`：模型、Prompt SHA、分数、分歧、置信度和人工复核条目均单独归档；AI 评分不表述为人工评分。

Agent 记录来源命中率、完整来源覆盖率、引用 precision、必需/允许/禁止工具、额外工具、预算越界、提示词注入拒答、失败、超时、Provider 重试和端到端 P50/P95。Agent 运行时会按问题意图收窄工具面：明确图片问题才暴露 `search_images`，多来源比较优先 `keyword_search_posts → read_posts`。题目标注的 required/allowed/forbidden tool 不是调低阈值，而是用于审计不必要调用。

证据目录最新报告运行是 `20260808T151704Z`：RAG 机器完整来源覆盖率 `0.9333`、引用精度 `0.9000`、P95 `5.13s`；Agent 机器完整来源覆盖率 `0.9722`、引用精度 `0.9583`、P95 `8.17s`。AI judge RAG 平均 `4.1667`/最低 `1`、Agent 平均 `4.8056`/最低 `1`，共 10 条进入 [人工复核交接包](../evidence/final-freeze/ai-judge-human-handoff.json)，质量门禁因此保持未通过。该运行使用 v1 RAG/Agent 数据集；完整结果见 [质量汇总](../evidence/final-freeze/quality-summary.json)、[延迟汇总](../evidence/final-freeze/latency-summary.json)、[运行清单](../evidence/final-freeze/run-manifest.json) 和 [保留运行记录](../evidence/final-freeze/retained-runs.json)。`retained-runs.json` 的权威运行指针尚未与该最新报告收口。

## 运行规则

- 只使用当前 47 条帖子对应的本机集合，不回退到旧文件。
- 不修改 Prompt、模型、数据标签、检索参数或质量阈值来制造通过结果。
- 单个命令只执行一次；失败保留退出码、失败类别和脱敏摘要。
- 结果统一使用 `pass`、`fail`、`blocked`、`needs_human_review`、`not_run`；`needs_human_review` 表示自动评分已完成但按规则必须人工复核，不等于通过。
- 不记录 API Key、Token、Cookie、Authorization header、完整 Provider 请求、完整私聊内容或个人照片。
- 当前结果属于特定本机语料和真实 Provider 的诊断证据，不外推为线上 SLA、通用准确率或第三方评审。

## 证据边界

当前运行的代码树和工作区状态记录在 Final Freeze 清单中；由于本次工作区为 dirty，结果不能被表述为干净提交上的可重建发布复评。Compose 故障注入、空缓存冷启动、浏览器自动化通过证据和 10 条 AI judge 人工复核仍按最终证据中的状态处理；已有人工 smoke 不替代浏览器自动化。
