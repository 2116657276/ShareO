# 中文语义搜图设计

> 能力边界：后端索引、公开 API、可见性过滤和 E2E 已实现；页面层复用本接口，阶段状态见 [`TASK.md`](../../TASK.md)

## 边界

审核通过的帖子图片异步写入 Qdrant `images`。Go是图片代理、权限和最终可见性入口；Python不配置MinIO凭证。编辑进入pending、驳回或删除时清理该帖向量。

## 搜索接口

公网 `GET /api/v1/search/images?q=&limit=`：

- `q` trim后1–200字符，`limit` 为1–20。
- Go以8秒超时调用内部AI搜索，再过滤 `approved AND is_deleted=0`。
- 同一帖子只保留最高分图片。
- 返回 `post_id`、`image_id`、`image_url`、`score` 和帖子，不暴露 object key。
- AI或Qdrant不可用时返回503，FULLTEXT与Feed不受影响。

内部 `POST /v1/search/images` 使用 `X-Internal-Token`，返回KNN候选和内部object key。`GET /v1/meta/image-search` 返回模型、revision、device、维度和collection信息。

## 索引数据流

```text
审核/编辑/驳回/删除/回填
  → shareo:stream:index_post
  → GET /internal/posts/:id/index-payload
  → GET /api/v1/images/<medium object>
  → Chinese-CLIP
  → Qdrant images
```

不存在、非approved或软删除帖子对索引载荷返回404，worker收到后幂等删除旧向量。

## 模型与 collection

- 模型：`OFA-Sys/chinese-clip-vit-base-patch16`。
- revision：`36e679e65c2a2fead755ae21162091293ad37834`。
- 设备：CUDA → MPS → CPU。
- 向量：归一化512维，cosine。
- point ID：`image_id`。
- payload：`post_id`、`image_id`、`object_key`、`created_at`、`model_revision`。
- `post_id` 建payload index；维度或距离不兼容时拒绝启动，不覆盖数据。

## 幂等与恢复

每次upsert先按 `post_id` 删除旧点，再按稳定 `image_id` 写入。重复upsert/delete安全。Stream失败最多处理四次，重启通过 `XAUTOCLAIM` 接管；最终失败记录后ACK，由回填和对账修复。

## Readiness 与日志

`/readyz/image-search` 只有图片模型已加载、`images` schema有效且索引consumer运行时返回200。日志分别记录载荷取取、下载、模型加载、编码、删除和写入耗时。

## 质量门禁

工程E2E已经覆盖审核后命中、重复投递、重启重领和删除不可见。最终质量使用40条冻结查询：Recall@5 ≥ 0.70、MRR ≥ 0.55，且无重复、待审、驳回或软删除结果。
