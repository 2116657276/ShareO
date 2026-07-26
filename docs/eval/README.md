# AI 评测规范

> 状态：数据规格、统一评测命令、Phase 7B 基线和 Phase 7C 复核报告均已收口

## 数据集

### 搜图 `image_search_v1.jsonl`

固定40条：

| 类别 | 数量 |
|---|---:|
| object | 8 |
| scene | 10 |
| color | 8 |
| style | 6 |
| composition | 4 |
| motion | 2 |
| no-match | 2 |

普通查询格式：

```json
{"query":"海边日落","category":"scene","relevant_post_ids":[12,18]}
```

无匹配查询额外使用 `"expect_no_results":true`。普通查询不得以空数组冒充标注完成。

### RAG `rag_qa_v1.jsonl`

Phase 7A创建30条：

| 类别 | 数量 |
|---|---:|
| factual | 10 |
| advice | 8 |
| comparison | 5 |
| multi-source | 4 |
| no-answer | 3 |

格式包含 `question`、`category`、`expected_source_post_ids`、`reference_answer`；无答案项显式使用 `expect_no_answer`。

## 指标与门禁

### 搜图

- Recall@5 ≥ 0.70。
- 同时输出 Recall@10 和 MRR，MRR ≥ 0.55。
- 重复帖子率为0。
- 待审、驳回和软删除帖子为0。
- 预热后MPS P95 ≤ 2秒、CPU P95 ≤ 4秒。

### RAG

- 来源命中率 ≥ 0.80。
- 引用可访问率100%，虚假引用为0。
- 人工回答相关性按1–5评分，平均 ≥ 4.0。
- 本地embedding+检索P95 ≤ 2秒。
- DeepSeek总延迟记录P50/P95和超时率，不设硬门禁。

## 运行记录

每次评测记录提交SHA、数据集版本、Demo seed版本、模型/revision、Prompt版本、设备、DeepSeek模型、日期和网络环境。原始结果和汇总追加到 [实验记录](experiments.md)，不得包含API Key或敏感请求头。

mock provider只证明协议和幂等，不计入真实质量指标。修改查询、标签、Demo数据、模型、检索参数或Prompt时必须升版或新增实验记录。

## 评测命令

首次运行同时生成机器报告和人工评分模板；人工评分未完成时该次命令预期返回非零状态：

```bash
make eval-ai \
  OUTPUT=docs/eval/results/phase7b_raw.json \
  SCORING_OUTPUT=docs/eval/results/phase7b_human_scoring.md
```

填写 30 条评分后，只复用机器报告并复核全部质量门禁，不重复调用服务：

```bash
make eval-ai \
  REPORT_INPUT=docs/eval/results/phase7b_raw.json \
  SCORING_INPUT=docs/eval/results/phase7b_human_scoring.md \
  OUTPUT=docs/eval/results/phase7b_final.json
```

评分文件也可以是 `{"scores":[1,2,...]}` JSON；分数必须与报告中的已评测问题数量一致，且每项为 1–5 的整数。

Phase 7B 最终证据：机器报告为 `results/phase7b_final.json`，人工评分表为 `results/phase7b_human_scoring.md`。30 条回答中 26 条为 5 分、4 条为 4 分，平均 4.8667；机器门禁为 Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、失败请求 0。

Phase 7C 运行时变更后重新完成 40/30 评测，报告为 [`results/phase7c_revalidated_final.json`](results/phase7c_revalidated_final.json)，人工评分表为 [`results/phase7c_revalidated_human_scoring.md`](results/phase7c_revalidated_human_scoring.md)。本轮 Recall@5 0.8125、MRR 0.7771、来源命中率 0.9667、引用可访问率 100%、虚假引用 0、失败请求 0，30 条人工评分均为 5 分；数据集、Prompt、检索参数和模型版本未改变。
