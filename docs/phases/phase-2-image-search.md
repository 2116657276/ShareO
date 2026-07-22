# Phase 2 — 中文语义搜图

## 范围

- approved 帖子图片通过 `index_post` 异步进入 Qdrant `images`。
- Chinese-CLIP 固定 revision，Go 提供图片代理和最终可见性过滤。
- Python AI 进程同时承载 API 与后台 consumer，共享模型状态。

## 退出条件

- 审核后图片可被中文查询命中，删除或取消审核后不可见。
- 重复事件幂等，worker readiness 可检查，阶段日志能定位失败点。
- 12 条演示查询 Recall@5 ≥ 0.70。
