# ADR-006：轻量范围与 AI 单进程运行时

- 日期：2026-07-23
- 状态：已接受，收藏范围由 ADR-009 部分取代

## 背景

早期路线同时规划群聊、Agent 和独立 AI Worker，导致功能与运维范围超过独立开发和五分钟演示所需。

## 决定

- 早期范围收窄阶段只保留社区、一对一私聊、语义搜图和私聊 RAG Bot；当前功能矩阵中的私人收藏和受限只读 Agent 分别由 ADR-009、ADR-008 恢复或修正。
- 收藏、转帖、话题、群聊、Agent 和工具调用从当时的阶段范围移除；私人收藏后来由 ADR-009 恢复，Agent 范围由 ADR-008 修正。
- 当前版本的范围例外由 ADR-008 定义：只允许同一 Bot 私聊入口中的只读 Agent，不恢复写入型 Agent、外部工具或多 Agent。
- Python AI API、`index_post` consumer 和 `bot_tasks` consumer 运行在同一 FastAPI 进程中。
- 运行时只支持宿主机 PostgreSQL 17、Redis、MinIO、Go/Python 与 Homebrew pgvector，旧 Compose 目标明确退役；Uvicorn 固定单 worker。
- Go 写运行时业务数据到 PostgreSQL `public` schema，Python 写同一实例的 `ai` schema；初始化脚本只负责 schema 与固定 Bot 引导记录，AI 不持有 `public` 表或 MinIO 凭证。

## 理由

单进程能共享模型和连接，减少内存、镜像和启动方式；轻量范围仍完整展示 Go 事务、WebSocket、异步任务、跨模态检索和 RAG 安全。

## 后果

- AI 进程故障同时影响搜图和 Bot，但不得影响社区和普通私聊。
- 水平扩展前必须重新设计 consumer 所有权，当前不支持多 Uvicorn worker。
- 被删除能力不作为论文或路线图中的待实现功能，只能作为范围权衡说明。
