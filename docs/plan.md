# ShareO 当前执行计划

> 更新时间：2026-07-23 | 状态事实源：[TASK.md](../TASK.md)

1. 文档范围冻结：统一轻量项目定位和完成边界。
2. 数据库与业务裁剪：合并 12 表 schema，删除收藏、转帖、话题和群聊。
3. 运行时轻量化：合并 AI API/worker，统一 Compose 与 Makefile。
4. 语义搜图修复：worker 预热、readiness、阶段日志和真实 E2E。
5. 文本索引与 RAG：FastEmbed、`post_chunks`、LLM 和引用过滤。
6. 私聊 Bot：拆为 6A 核心链路和 6B 页面与 mock LLM E2E，阶段 6 已通过评审。
   - 6A：Go/Redis/Python Worker 闭环、私聊触发、内部 token、幂等回复、引用二次校验和最终兜底。
   - 6B：聊天页 AI 助手入口、引用卡片、测试 mock provider，以及索引、回复、重复任务、普通私聊、删除来源和超时降级 E2E。
7. Demo 与清理：真实 DeepSeek/Ollama provider 演示、12 条搜图和 15 条 RAG 评测、Demo seed、README 收口和历史无效文件删除。

每阶段独立提交，并以测试结果和 `TASK.md` 的简短评审作为进入下一阶段的条件。
