# AI 评测规范

> 状态：数据规格已冻结，标注和统一评测命令待 Phase 7A/7B

## 数据集

### 搜图 `image_search_v1.jsonl`

固定40条：

| 类别 | 数量 |
|---|---:|
| object | 8 |
| scene | 8 |
| color | 6 |
| style | 6 |
| composition | 4 |
| portrait | 4 |
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
