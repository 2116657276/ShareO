# Phase 2 — 语义搜图

> 更新时间: 2026-07-21 | 状态: 后端进行中

## 目标与当前基线

让 approved 帖子图片异步进入 Chinese-CLIP/Qdrant，并通过自然语言稳定检索。初始后端切片提交为 `7a96feb`，真实模型/Qdrant 闭环尚未取得证据。

## 工作包

1. **真实闭环**：独立 Qdrant 命令、固定模型 revision、后台预热、搜索 readiness、schema 校验、payload index、隔离 E2E。
2. **可靠性**：回填、索引对账、故障注入、结构化耗时/计数日志；暂不引入 outbox。
3. **质量**：每帖最高分去重、40 条中文评测集、FULLTEXT 基线、Recall/MRR/延迟报告。
4. **发布**：质量门禁后实现关键词/语义最小页面和降级状态。

## 接口与数据流

- `GET /api/v1/search/images?q=&limit=`：公开搜索，每项返回 post/image/image_url/score/post，不暴露 object key。
- `POST /v1/search/images`：内部编码和 KNN，要求 internal token。
- `GET /readyz/search`：模型与 collection 就绪。
- `GET /v1/meta/image-search`：内部返回 model/revision/device/dimension/collection。
- 审核通过发布 upsert；驳回、删除、编辑 pending 发布 delete；worker 通过 Go medium 图片代理读取。

## 失败模式

模型预热期间语义搜索返回 503；Redis/Qdrant/MinIO 任一失败只影响语义能力，Feed、关键词搜索和 IM 保持正常。重复事件幂等，丢失事件由回填和对账修复。

## 测试矩阵

- 模型 revision、设备、512 维、归一化、预热、并发和超时。
- collection 维度/距离不兼容、payload index、重复 upsert/delete。
- 隔离 E2E、worker 重启、Redis/Qdrant/MinIO 故障、回填和对账。
- 40 条查询覆盖物体、场景、颜色、风格、组合语义、模糊与无匹配。

## 退出标准

- 健康环境删除 10 秒内不可见，worker 重启 45 秒内收敛。
- Recall@10 ≥ 0.75，MRR ≥ 0.55，无不可见帖子或重复帖子。
- 预热后 CPU P95 ≤ 4 秒、MPS P95 ≤ 2 秒；4060 只记录实验。
- API、真实依赖、评测、性能、降级、页面和文档证据齐全。

## 证据矩阵

| 交付 | 提交 | 自动证据 | 环境证据 |
|------|------|----------|----------|
| 初始索引/搜索切片 | `7a96feb` | Python 20 项非集成测试、Go/race | 待真实 Qdrant/模型 E2E |
| 成熟化代码与分阶段文档 | `5c6e087` | Python 27 项非集成测试、Go、Shell、61 个 Markdown 链接 | 隔离 E2E 脚本已建立，待真实运行 |

## 暂缓项

搜索页面与浏览器验收必须等待真实闭环和质量评测通过。
