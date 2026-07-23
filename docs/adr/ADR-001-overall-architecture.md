# ADR-001: 总体架构 —— Go 单体扩展 + Python AI 服务

- 日期: 2026-07-19
- 状态: 已接受（Python 双进程部分已被 [ADR-006](ADR-006-lightweight-scope-runtime.md) 替代）

> 历史决定中的 Go/Python 双服务与单写者原则仍有效；“AI API 与 Worker 两个进程”和远期 Agent 规划不再有效。当前运行时以 ADR-006 为准。

## 背景

v2 要引入 IM、语义搜图、Bot/RAG、远期 Agent。核心矛盾：现有主站是成熟的 Go 单体（分层清晰、58 路由、有审核流），而 AI 生态（PyTorch / transformers / 本地模型部署）在 Python 侧。开发者为 AI 专业本科生，独立开发，此项目为毕业设计。

## 候选方案

1. 纯 Go，AI 能力全部调云 API
2. Go 主服务 + Python AI 服务（双服务）
3. 推倒重写为完整微服务架构

## 决定

选方案 2：

- Go 单体继续承载**全部业务**（含 IM/WebSocket），按现有 Handler→Service→Repository 分层扩展。
- 新增 `ai-service/`（Python 3.12 + FastAPI + uv）：一份代码库、两个进程——API 进程（编码/搜索/RAG）与 Worker 进程（消费队列）。
- 服务间通信：内部 HTTP/JSON + 共享 token（`X-Internal-Token`），ai-service 不对公网暴露。
- **单写者原则**：MySQL 只由 Go 写（含 Bot 回复落库，Python 经内部接口回调）；Qdrant 只由 Python 写。

## 理由

- AI 专业对口：本地模型、RAG、Agent 的生态与学习价值都在 Python，方案 1 会把毕设的 AI 深度砍掉大半。
- 稳定性解耦：ai-service 崩溃时主站与 IM 完全可用，搜索/Bot 优雅降级。
- 方案 3 收益为零：现有单体规模完全健康，重写只烧时间。
- 通信选 HTTP/JSON 而非 gRPC：可调试性优先，接口面收窄后未来切 gRPC 是小工作量练习（届时新增 ADR）。

## 后果与代价

- 维护两套运行时与依赖 → Phase 0 用 docker-compose 统一编排。
- 跨服务调试成本 → 接口保持少而稳定，全部列入 architecture.md §7。
- IM 留在单体意味着单实例扩展上限 → 设计时预留 Redis Pub/Sub 扇出接口，多实例演进作为论文展望。
