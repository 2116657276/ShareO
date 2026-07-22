# Phase 2 — 中文语义搜图

> 工程状态：已完成；12 条查询质量评测等待阶段 7 的固定 Demo 数据。

## 范围

- approved 帖子图片通过 `index_post` 异步进入 Qdrant `images`。
- Chinese-CLIP 固定 revision，Go 提供图片代理和最终可见性过滤。
- Python AI 进程同时承载 API 与后台 consumer，共享模型状态。

## 退出条件

- 审核后图片可被中文查询命中，删除或取消审核后不可见。
- 重复事件幂等，worker readiness 可检查，阶段日志能定位失败点。
- 12 条演示查询 Recall@5 ≥ 0.70。

## 工程证据

- `make check`：Go/Python 单测、静态检查、Shell 与文档链接通过。
- `make test-image-e2e`：真实 Chinese-CLIP、MySQL、Redis、MinIO 和 Qdrant 下完成审核后命中、重复投递收敛、FastAPI 重启后 45 秒内重领以及删除后十秒内不可见。
- `/readyz/image-search` 同时检查模型、Qdrant schema 和图片索引消费者；E2E 还会断言各分段耗时日志实际输出。
