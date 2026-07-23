# ADR-001: 总体架构 —— Go 单体扩展 + Python AI 服务

- 日期：2026-07-19
- 状态：已接受；运行时细节以 [ADR-006](ADR-006-lightweight-scope-runtime.md) 为准

> 本 ADR 的 Go/Python 服务边界和单写者原则仍有效；早期“AI API 与 Worker 分为两个进程”的决定已被 ADR-006 替代。下文把历史取舍和当前仍有效的约束分开记录，避免把历史方案当作实现要求。

## 当前有效决定

- Go 模块化单体承载全部业务，包括认证、审核、社区、私聊和 WebSocket，保持 Handler → Service → Repository 分层。
- `ai-service/` 使用 Python 3.12、FastAPI 和 uv；API、同时处理图片与正文的 `index_post` consumer、以及 `bot_tasks` consumer 在同一 FastAPI 进程内运行，Uvicorn 固定单 worker。
- Go 与 Python 通过内部 HTTP/JSON 和 `X-Internal-Token` 通信；AI 服务不对公网暴露。
- Go 是运行时 MySQL 业务数据的唯一写者（包括 Bot 回复），Python 通过内部接口读取载荷并只写 Qdrant。

## 历史取舍

早期选择 Go 主站 + Python AI 服务，是为了保留本地 embedding、RAG 和模型工程能力，同时让 AI 进程故障不阻塞主站和普通私聊。选择 HTTP/JSON 而不是 gRPC，是为了让跨服务链路便于终端调试和证据复核。

以下决定不再生效：AI API 与 Worker 拆成两个进程，以及把 Agent 作为产品路线。前者由 ADR-006 的单进程方案替代，后者已从最终范围移除。

## 后果与验证边界

- 单进程共享模型和连接，降低本地内存与启动复杂度；多 Uvicorn worker 或水平扩展前必须重新设计 consumer 所有权。
- AI 服务停止时，社区、全文搜索和普通私聊保持可用，语义搜图和 Bot 按 readiness 与降级矩阵处理。
- 具体服务拓扑、数据所有权和降级行为以 [架构文档](../architecture.md) 为准。
