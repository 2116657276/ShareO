# Phase 4 — 中文语义搜图

> 状态：工程闭环完成，质量评测待 Phase 7B

## 目标

让 approved 帖子图片通过中文自然语言检索，并在内容状态变化后及时从结果中消失。

## 当前基线

Chinese-CLIP 固定 revision、图片代理、Qdrant `images`、索引 consumer、公开搜索、可见性过滤和真实 E2E 已完成。

## 范围与非目标

返回每帖最高分图片和帖子信息；不公开 MinIO object key，不实现跨模态训练、重排模型或独立搜索前端框架。

## 依赖

依赖 Phase 1 审核内容、Phase 3 运行时、MinIO、Qdrant 和 Go 内部载荷接口。

## 工作包

- 图片下载、RGB 转换、512 维归一化编码。
- Qdrant collection/schema/payload index 校验。
- 审核、编辑、驳回、删除、回填和对账事件。
- 候选过取、Go 可见性过滤和按帖子去重。
- 40 条冻结查询质量与性能评测。

## 接口与数据流

审核事件 → `index_post` → Go 索引载荷/图片代理 → Chinese-CLIP → Qdrant。搜索时 Go 调 AI KNN，再回源 MySQL过滤和补全。

## 数据一致性

point ID 使用 `image_id`；每次 upsert 先按 `post_id` 删除旧点；重复 upsert/delete 幂等；回填和 dry-run/apply 对账修复事件丢失。

## 安全边界

AI 只通过内部 token 读取载荷；公网结果不含 object key；Go 最终过滤审核和软删除状态。

## 失败模式

预热返回 503；Qdrant、MinIO或 AI 不可用时只有语义搜图降级，Feed、全文搜索和私聊继续工作。

## 测试矩阵

revision、设备、维度、归一化、schema 拒绝覆盖、payload index、损坏图片、重复事件、重启重领、删除不可见和隔离 E2E。

## 退出标准

- 工程门禁：审核后命中、重复投递收敛、重启 45 秒内重领、删除 10 秒内不可见。
- 质量门禁：40 条查询 Recall@5 ≥ 0.70、MRR ≥ 0.55、无重复或不可见帖子。
- 性能门禁：预热后 MPS P95 ≤ 2 秒、CPU P95 ≤ 4 秒。

## 提交与环境证据

- `bb9732f`：完成真实语义搜图工程闭环及 Transformers 兼容修复。
- `make test-image-e2e` 已通过真实 Chinese-CLIP、MySQL、Redis、MinIO 和 Qdrant 链路。
- 质量和性能报告尚未生成，因此本阶段不标记完全完成。

## 遗留项

Phase 7A 完成标签，Phase 7B执行质量/性能门禁，Phase 7C归档最终证据。
