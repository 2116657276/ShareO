# Phase 2 语义搜图后端设计（v1）

> 状态：后端/API 与真实索引 E2E 已完成；搜索页面和 12 条质量评测在阶段 7 收口。

## 边界

审核通过的帖子图片异步写入 Qdrant `images` collection。Go 是图片代理和可见性过滤的唯一入口；Python worker 不配置 MinIO 凭证，通过 Go 的 `/api/v1/images/...` 读取 medium 图片。删除、驳回或重新编辑进入 pending 时发布删除事件。

## API

### 用户搜索

`GET /api/v1/search/images?q=山景&limit=10`

- `q` trim 后 1–200 字符，`limit` 为 1–20，参数错误返回 400。
- Go 调用 AI `POST /v1/search/images`，带 `X-Internal-Token`，再用 MySQL 过滤 `approved AND is_deleted=0` 的帖子并补齐作者、图片和主题。
- AI 不可用返回 503；不会把 Python/Qdrant/数据库原文错误暴露给客户端。

### AI 搜索

`POST /v1/search/images`（仅内部 token）请求 `{ "query": "山景", "limit": 10 }`，返回：

```json
{"results":[{"image_id":12,"post_id":7,"object_key":"posts/medium/2026/01/a.jpg","score":0.91}]}
```

### 索引载荷

`GET /internal/posts/:id/index-payload`（仅内部 token）只返回 approved 且未删除的帖子：

```json
{"data":{"post_id":7,"status":"approved","content":"...","created_at":"...","images":[{"image_id":12,"object_key":"posts/medium/2026/01/a.jpg","image_url":"/api/v1/images/posts/medium/2026/01/a.jpg","created_at":"..."}]}}
```

不存在、待审、驳回或软删除帖子返回 404。worker 遇到 404 会幂等清理该帖向量。

## 事件与幂等

Redis Streams 使用 `shareo:stream:index_post`，字段为 `action=upsert|delete` 和 `post_id`。审核、删除和回填由 Go 发布；worker 先按 `post_id` 删除旧点，再按 `image_id` upsert 新点。重复事件安全，失败遵循现有四次尝试、XAUTOCLAIM 和最终 ACK 规则。

## 向量与设备

Chinese-CLIP `OFA-Sys/chinese-clip-vit-base-patch16` 固定 revision `36e679e65c2a2fead755ae21162091293ad37834`，启动后后台预热，设备顺序 CUDA → MPS → CPU；输出归一化为 512 维。Qdrant 使用 cosine 距离并为 `post_id` 建 payload index，payload 含 `post_id`、`image_id`、`object_key`、`created_at`、`model_revision`。`/readyz/image-search` 只在模型、collection 与图片索引消费者均可用时返回 200。

上面的 `object_key` 只存在于 Go↔AI 内部协议和 Qdrant payload。公网 `GET /api/v1/search/images` 每项固定返回 `post_id`、`image_id`、`image_url`、`score`、`post`，不会暴露对象键；同一帖子仅保留最高分图片。

## 本轮不做

搜索页面和浏览器验收等待真实闭环及质量评测通过；4060 吞吐只记录实验，不阻塞本地门禁。当前阶段细节以 [Phase 2 文档](../phases/phase-2-image-search.md) 为准。
