# TASK.md — 开发任务跟踪

> 更新时间: 2026-07-20 | 当前阶段: **Phase 0 完成 → Phase 1 待启动**（详细步骤见 [docs/plan.md](docs/plan.md)）

## 当前待办（Phase 0 基本完成，遗留项排入下阶段）

- [x] 0.0 结构清理脚本 `bash scripts/cleanup_v2_restructure.sh`（2026-07-20 执行完毕）
- [x] 0.1 评审修复 SR-01~SR-22（20 项，SR-07/SR-12/SR-23 排入 Phase 1 前）
- [x] 0.2.1 Makefile `fmt` / `check` 目标
- [x] 0.2.2 slog 结构化日志迁移（handler/middleware 层完成，repository/service 层后续渐进）
- [ ] 0.2.3 密码修改功能（排入 Phase 0 收尾）
- [x] 0.2.4 CI 工作流文件（Go + Python）
- [x] 0.3 deploy/docker-compose.yml（MySQL/Redis/MinIO/Qdrant）
- [x] 0.4 ai-service 脚手架（FastAPI + healthz + pytest）
- [x] 0.5 队列骨架打通（Go Streams → Python consumer）

## 中难度待排期（不阻塞 Phase 0 退出）

| 来源 | 项目 | 说明 | 建议排期 |
|------|------|------|----------|
| SR-07 | 搜索 FULLTEXT 语法错误降级 + 错误脱敏 | 见评审报告 | Phase 0 末或 Phase 2 搜索改造时一并做 |
| SR-12 | 上传并发 semaphore（原 TASK P2） | io.ReadAll 全内存，防 OOM | Phase 1 前 |
| SR-23 | 评论 reply_to_uid 校验 | 防定向通知骚扰 | Phase 1（做 IM 时统一"用户存在性"校验） |
| 旧 P3 | i18n 错误消息 | 随 standards.md 错误码分段渐进，不做大重构 | v2 期间渐进 |
| 旧 P2 | 依赖注入 | 老代码不强改；v2 新模块已按 standards.md 强制注入 | 已转为规范 |

## 已完成（归档）

- [x] 2026-07-20: Phase 0 基本完成——结构清理、20 项评审修复（P1×4 + P2×6 + P3×10）、Makefile check、slog 迁移、CI workflows、Docker Compose、ai-service 脚手架、队列骨架
- [x] 2026-07-19: v2 立项——架构/ADR×5/路线图/详细计划/开发规范/文档体系（docs/）；全量代码评审（4 P1 + 8 P2 + 12 P3）
- [x] 2026-06-19: 上轮评审 18/21 项修复 + 文档同步
