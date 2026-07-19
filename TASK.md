# TASK.md — 开发任务跟踪

> 更新时间: 2026-07-20 | 当前阶段: **Phase 0 完成 → Phase 1 待启动**（详细步骤见 [docs/plan.md](docs/plan.md)）

## 当前待办（Phase 0 全部完成 ✅，下一步 Phase 1 IM）

- [x] 0.0 结构清理脚本（2026-07-20 执行完毕）
- [x] 0.1 评审修复 SR-01~SR-22, SR-24（20 项全部完成，含 SR-07/12/23）
- [x] 0.2.1 Makefile `fmt` / `check` 目标
- [x] 0.2.2 slog 结构化日志迁移（handler/middleware/service 层完成，repository 层后续渐进）
- [x] 0.2.3 密码修改功能（API + Web 表单）
- [x] 0.2.4 CI 工作流文件（Go + Python）
- [x] 0.3 deploy/docker-compose.yml（MySQL/Redis/MinIO/Qdrant）
- [x] 0.4 ai-service 脚手架（FastAPI + healthz + pytest）
- [x] 0.5 队列骨架打通（Go Streams → Python consumer）

## 遗留项（排入 v2 后续阶段）

| 来源 | 项目 | 说明 | 排期 |
|------|------|------|------|
| 旧 P3 | i18n 错误消息 | 随 standards.md 错误码分段渐进 | v2 期间渐进 |
| 旧 P2 | 依赖注入 | 老代码不强改；v2 新模块已按 standards.md 强制注入 | 已转为规范 |
| 旧 P2 | CSRF Token 防护 | 当前仅 SameSite=Lax；Phase 1 IM 表单前统一加 | Phase 1 设计阶段 |
| slog | repository 层 ~25 处 log.Printf | 格式串需逐文件转为 slog key=value | 随手改 |

## 已完成（归档）

- [x] 2026-07-20: Phase 0 全部完成——结构清理、22 项评审修复（P1×4 + P2×8 + P3×10）、密码修改、Makefile check、slog 迁移、CI workflows、Docker Compose、ai-service 脚手架、队列骨架、SR-07/12/23 收尾
- [x] 2026-07-19: v2 立项——架构/ADR×5/路线图/详细计划/开发规范/文档体系（docs/）；全量代码评审（4 P1 + 8 P2 + 12 P3）
- [x] 2026-06-19: 上轮评审 18/21 项修复 + 文档同步
