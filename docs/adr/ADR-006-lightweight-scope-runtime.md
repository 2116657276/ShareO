# ADR-006：轻量范围与 AI 单进程运行时

- 日期：2026-07-23
- 状态：已接受

## 背景

早期路线同时规划群聊、Agent和独立AI Worker，导致功能与运维范围超过独立开发和五分钟演示所需。

## 决定

- 最终产品只保留社区、一对一私聊、语义搜图和私聊RAG Bot。
- 收藏、转帖、话题、群聊、Agent和工具调用从最终范围移除。
- Python AI API、`index_post` consumer和 `bot_tasks` consumer运行在同一FastAPI进程中。
- 根目录Compose是唯一完整运行方式，Uvicorn固定单worker。
- Go写MySQL，Python写Qdrant；AI不持有MySQL或MinIO凭证。

## 理由

单进程能共享模型和连接，减少内存、镜像和启动方式；轻量范围仍完整展示Go事务、WebSocket、异步任务、跨模态检索和RAG安全。

## 后果

- AI进程故障同时影响搜图和Bot，但不得影响社区和普通私聊。
- 水平扩展前必须重新设计consumer所有权，当前不支持多Uvicorn worker。
- 被删除能力不作为论文或路线图中的待实现功能，只能作为范围权衡说明。
