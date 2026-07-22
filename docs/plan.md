# ShareO 当前执行计划

> 更新时间：2026-07-22 | 状态事实源：[TASK.md](../TASK.md)

1. 文档范围冻结：统一轻量项目定位和完成边界。
2. 数据库与业务裁剪：合并 12 表 schema，删除收藏、转帖、话题和群聊。
3. 运行时轻量化：合并 AI API/worker，统一 Compose 与 Makefile。
4. 语义搜图修复：worker 预热、readiness、阶段日志和真实 E2E。
5. 文本索引与 RAG：FastEmbed、`post_chunks`、LLM 和引用过滤。
6. 私聊 Bot：Bot 身份、任务发布、幂等回复和引用 UI。
7. Demo 与清理：固定数据、搜索页、评测和无关文件删除。

每阶段独立提交，并以测试结果和 `TASK.md` 的简短评审作为进入下一阶段的条件。
