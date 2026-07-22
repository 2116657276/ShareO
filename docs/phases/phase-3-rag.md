# Phase 3 — Bot + RAG

> 更新时间: 2026-07-22 | 状态: 未启动，Phase 2 真实 E2E 门禁未通过

## 目标与当前基线

基于站内 approved 内容回答问题并返回可访问引用。当前只有 Redis Streams worker 骨架，Bot handler 仍为 Phase 3 TODO。

## 当前状态与启动条件

Phase 3 尚未创建运行时代码，`docs/design/rag.md` 也尚未定稿。启动前必须满足 Phase 2 的全部前置门禁：真实上传→审核→索引→命中→删除→不可见闭环、worker 重启 45 秒内重领、依赖故障降级、Recall@10 ≥ 0.75、MRR ≥ 0.55，以及 40 条搜图评测和性能报告证据。

2026-07-22 阶段 0 评审发现 worker 未在 180 秒内完成 approved 图片命中，Phase 3 保持暂停。不得因为目录、接口草案或单元测试存在而将本阶段标记为“进行中”。

## 工作包

- `011_bot.sql`：`users.is_bot`、消息 `meta` JSON 和 Bot 初始化数据。
- approved 帖子分块、固定 revision 的 BGE 中文 embedding、`post_chunks` collection。
- 先完成检索和引用评测，再接 OpenAI 兼容 LLM 与 IM Bot。
- 私聊 Bot 或群内明确提及时触发，普通聊天不等待 AI。

## 接口与数据流

Go 发布 bot task；worker 拉取近 20 条会话上下文，检索站内块，生成带帖子引用的回答，再通过内部 Bot 回复接口走正常消息管线。

## 失败模式

AI/LLM 超时返回可理解的兜底消息；引用帖子在返回前由 Go 校验可见性；停止 AI 服务不得影响普通 IM。

## 测试矩阵

分块边界、检索排序、引用过滤、mock LLM、Bot 权限、重复任务幂等、超时与降级；至少 30 条问答评测。

## 退出标准

引用正确率 ≥ 0.80，所有引用可访问，质量/延迟/成本报告归档，AI 停止时聊天正常。

## 证据矩阵

尚无实现证据；不得提前标记进行中。

## 暂缓项

rerank 和本地 7B 对比可在基本 RAG 达标后追加。

## 分阶段开发与评审规则

Phase 2 门禁通过后，Phase 3 按以下顺序推进。每完成一阶段都必须独立提交、运行门禁、完成代码评审并回写文档，评审通过后才能进入下一阶段：

1. **设计冻结**：创建并定稿 `docs/design/rag.md`，冻结 BGE revision、400/80 分块参数、`post_chunks` schema、RAG API、引用约束和 30+ 条 `rag_qa_v1` 数据集。
2. **文本索引**：复用现有 `index_post` stream，只索引 approved 正文；加入 BGE、Qdrant schema、确定性 chunk ID、幂等 upsert/delete、backfill/reconcile。
3. **Bot/Go 管线**：新增 `011_bot.sql`、Bot 身份、私聊和群聊触发、内部上下文/回复接口、可见性校验与回复幂等。
4. **RAG 服务**：实现检索去重、受限 Prompt、OpenAI-compatible provider、虚假引用过滤和 `/readyz/rag`。
5. **Worker/E2E/评测**：接通 `bot_tasks`、重试与兜底消息，完成 IM E2E、故障降级、引用正确率、质量/延迟/成本评测。
6. **总体收口**：完成全量评审，修复 P0/P1 以及影响正确性、安全性、可见性和幂等性的 P2，并在进入 Phase 4 前更新全部证据。

每阶段评审文档使用 `docs/reviews/YYYY-MM-DD-phase3-stageN-review.md`，必须记录评审提交、测试和真实依赖、P0–P3 问题、修复与复测证据、遗留风险、通过结论及文档回写位置。真实实验当天同步写入 `docs/eval/experiments.md`。
