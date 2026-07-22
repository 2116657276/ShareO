# 实验记录

> 更新时间: 2026-07-22 | 状态: Phase 2 真实 E2E 未通过，评测等待索引收敛

| 日期 | 实验 | 数据集版本 | 关键配置 | 指标结果 | 结论/备注 |
|------|------|------------|----------|----------|-----------|
| 2026-07-21 | 评测框架建立 | image_search_v1（待标注） | Chinese-CLIP revision `36e679e65c2a2fead755ae21162091293ad37834`，512 维 cosine | 未运行 | 数据库已清空；禁止在空标签下生成质量结论 |
| 2026-07-22 | Phase 2 阶段 0 隔离 E2E | image_search_v1（未标注） | 真实 MySQL/Redis/Qdrant/MinIO；approved 图片搜索轮询 180 秒 | 未命中；Recall/MRR 未运行 | Go/Python/Shell/Markdown 门禁通过；worker 未在窗口内完成索引收敛，先定位日志和 upsert 失败点 |
