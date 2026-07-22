# TASK.md — 当前任务与证据

> 更新时间: 2026-07-22 | 后端能力线: **Phase 2 真实 E2E 门禁阻塞** | 发布验收线: **Phase 1 后端候选，页面验收待后**
>
> 本文件是当前状态的唯一事实来源；执行顺序见 [docs/plan.md](docs/plan.md)，阶段细节见 [docs/phases/](docs/phases/README.md)。

## 已完成

- [x] Phase 0 本地工程基线：Go/Python/Shell/文档门禁、可靠 Streams、Compose、AI liveness/readiness、结构化日志（`57b9615`）
- [x] Phase 1 后端候选：事务 IM、邀请制群组、精确未读、WS 恢复/吊销、Origin/CSRF、真实 MySQL/Redis 与终端验收（`57b9615`）
- [x] Homebrew 本地数据重置、MinIO 运行固化、Phase 2 搜图后端初始切片（`7a96feb`）
- [x] 当前本地门禁：`make check`、`make test-integration`、`go test -race ./...`、API/IM curl、Compose 与文档链接均已有证据

## 进行中：Phase 2 后端闭环（阶段 0 门禁阻塞）

- [x] Chinese-CLIP 懒加载、设备选择、512 维向量和 Qdrant `images`
- [x] 审核/删除索引事件、Go 图片代理 worker、公开搜索 API、回填命令
- [x] 独立 Qdrant 本地命令与 AI API/worker 运行命令（`5c6e087`）
- [x] 固定模型 revision、后台预热、搜索专用 readiness 和元信息（`5c6e087`）
- [x] collection schema 校验与 `post_id` payload index（`5c6e087`）
- [x] 每帖最高分去重、稳定公开响应、搜索结构化日志（`5c6e087`）
- [ ] 隔离 E2E：上传→审核→索引→命中→删除→不可见
- [ ] worker 模型预热、索引失败日志和 180 秒内收敛证据
- [x] 索引对账 dry-run/apply 与 40 条评测模板/执行器（`5c6e087`；数据待人工标注）
- [ ] 真实依赖故障注入、评测集标注与质量/性能报告

## 发布验收线

- [ ] Phase 2 质量门禁通过后实现关键词/语义最小搜索页面
- [ ] Phase 1 双浏览器、断网恢复、WS 吊销、群组操作和部署 trusted origin
- [ ] Phase 2 页面空态、预热、降级和演示验收

## 阻塞与暂缓

- 远端 CI 结果尚未取得可复核证据；Phase 0 只标记“本地完成”。
- Phase 2 真实闭环必须在 Docker/Qdrant/Homebrew MinIO 均可复核运行时验收；环境依赖本轮已启动，但索引收敛证据仍缺失。
- 2026-07-22 已启动 Colima、Qdrant 和 MinIO 并修复 E2E 轮询的 503 提前退出；修复后 worker 仍未在 180 秒内完成图片命中，Phase 2 真实索引收敛待定位，Phase 3 暂停。
- Phase 3 必须等待 Phase 2 E2E、故障注入和评测门禁全部通过；Phase 4 Agent 是扩展目标，不影响核心完成判定。

## 下一步

先为 worker 增加可复核的 readiness/预热状态和索引失败日志，定位图片代理、模型编码、Qdrant upsert 或查询过滤中的首个失败点；修复后重新运行隔离 E2E、worker 重启重领和删除不可见验证。阶段 0 评审通过前不创建 Phase 3 运行时代码。

## 当前证据

- [2026-07-21 进度审计](docs/reviews/2026-07-21-progress-audit.md)
- [2026-07-22 Phase 2 门禁阶段评审](docs/reviews/2026-07-22-phase2-gate-review.md)
- [Phase 0 详细计划](docs/phases/phase-0-foundation.md)
- [Phase 1 详细计划](docs/phases/phase-1-im.md)
- [Phase 2 详细计划](docs/phases/phase-2-image-search.md)
