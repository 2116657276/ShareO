# 中文语义搜图设计

> 能力边界：后端索引、公开 API、可见性过滤和本机工程测试已实现；页面层复用本接口，当前证据见 [`docs/REVIEW.md`](../REVIEW.md)

## 边界

审核通过的帖子图片异步写入 PostgreSQL pgvector `ai.image_embeddings`。Go 是图片代理、权限和最终可见性入口；Python 不配置 MinIO 凭证。编辑进入 pending、驳回或删除时清理该帖向量。

## 搜索接口

公网 `GET /api/v1/search/images?q=&limit=`：

- `q` trim 后为 1–200 字符，`limit` 为 1–20。
- Go 以 8 秒超时调用内部 AI 搜索，再过滤 `approved AND is_deleted=0`。
- Go 默认只返回分数不低于 `0.39` 的结果，可用 `SHAREO_IMAGE_SEARCH_SCORE_THRESHOLD` 在本机实验中覆盖；当前阈值由 8 条 no-match 负例校准。
- 同一帖子只保留最高分图片。
- 返回 `post_id`、`image_id`、`image_url`、`score` 和帖子，不暴露 object key。
- AI 或 pgvector 不可用时返回 503，关键词 `LIKE` 与 Feed 不受影响。

内部 `POST /v1/search/images` 使用 `X-Internal-Token`，返回KNN候选和内部object key。`GET /v1/meta/image-search` 返回模型、revision、device、维度和collection信息。

## 索引数据流

```text
审核/编辑/驳回/删除/回填
  → shareo:stream:index_post
  → GET /internal/posts/:id/index-payload
  → GET /api/v1/images/<medium object>
  → Chinese-CLIP
  → PostgreSQL pgvector ai.image_embeddings
```

不存在、非 approved 或软删除帖子对索引载荷返回 404，worker 收到后幂等删除旧向量。

## 模型与 collection

- 模型：`OFA-Sys/chinese-clip-vit-base-patch16`。
- revision：`36e679e65c2a2fead755ae21162091293ad37834`。
- 设备：CUDA → MPS → CPU。
- 向量：归一化 512 维，cosine。
- point ID：`image_id`。
- payload：`post_id`、`image_id`、`object_key`、`created_at`、`model_revision`。
- `post_id` 建payload index；维度或距离不兼容时拒绝启动，不覆盖数据。

## 幂等与恢复

每次 upsert 先按 `post_id` 删除旧点，再按稳定 `image_id` 写入。重复 upsert/delete 安全。Stream 失败最多处理四次，重启通过 `XAUTOCLAIM` 接管；最终失败记录后 ACK，由回填和对账修复。

## Readiness 与日志

`/readyz/image-search` 只有图片模型已加载、`images` schema 有效且索引 consumer 运行时返回 200。日志分别记录载荷获取、下载、模型加载、编码、删除和写入耗时。

## 质量门禁

当前质量使用 38 条本机搜图开发集（含 8 条 no-match），记录 Recall@5、Recall@10、MRR、nDCG@5、Precision@3、无匹配准确率、重复结果和延迟。最新运行结果与数据集 SHA256 见 [Final Freeze](../evidence/final-freeze/README.md)；该集合用于当前语料诊断，不外推为公共基准。
