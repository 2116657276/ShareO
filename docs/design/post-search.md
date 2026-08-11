# 混合搜贴设计

## 目标与边界

公开入口继续使用 `GET /api/v1/search`。搜贴只使用帖子正文，不读取图片向量；搜图继续只使用图片向量。首版面向本机中小规模语料，候选窗口最多 200 条，不引入 Elasticsearch、OpenSearch、LLM 查询改写或大型重排模型。

## 数据流

```text
浏览器
  → Go 公开搜贴
      ├─ PostgreSQL LIKE 关键词候选
      └─ AI 内部正文语义检索
           → BGE query embedding
           → PostgreSQL pgvector ai.post_chunk_embeddings
      → Go 按 post_id 合并
      → approved AND is_deleted=0 复核
      → 混合排序、分页和帖子补全
```

AI/pgvector 不可用时只使用关键词候选并返回 HTTP 200。关键词链路也失败时返回受控错误。内部语义接口必须使用 `X-Internal-Token`，不暴露为公开 API。

## 排序

每路候选先按自身相关性排序，再转换为：

```text
rank_score = 1 / log2(rank + 1)
```

文本规范化只执行 Unicode 小写、移除空白与标点。查询长度大于一时使用字符 bigram，小于等于一时使用 unigram；词面重合使用 Sørensen–Dice。最终分数固定为：

```text
0.60 × semantic_rank
+ 0.20 × keyword_rank
+ 0.15 × lexical_overlap
+ 0.05 × exact_phrase
```

缺少某一路候选时对应分量为零；同分时按 `created_at DESC, post_id DESC`。分数只用于内部排序和评测，公开响应保持现有帖子分页结构。

本机 32 条基线显示，相关查询的语义 Top1 最低约为 `0.566`，无匹配查询最高约为 `0.518`。首版采用保守阈值 `0.54`，并允许通过 `SHAREO_POST_SEARCH_SEMANTIC_THRESHOLD` 在独立实验中覆盖。关键词候选只有在完整短语命中或 bigram Dice 不低于 `0.20` 时进入融合，避免 ngram union 因常见短语产生无关结果。

## 评测

文本评测集与 RAG 问答集隔离，覆盖精确关键词、同义表达、自然语言、多条件、短文案和无匹配。每条使用 `2/1/0` 三级相关性，分别运行 keyword-only、semantic-only 和 hybrid，输出 Recall@5、MRR、nDCG@5、Precision@3、无匹配准确率和 P50/P95。

首版门禁：Recall@5 ≥ 0.80、MRR ≥ 0.65、nDCG@5 ≥ 0.70、无匹配准确率 ≥ 0.80；同时要求关键词降级和帖子可见性复核通过。
