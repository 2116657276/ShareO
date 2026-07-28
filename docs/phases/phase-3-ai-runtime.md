# Phase 3 — AI 单进程运行时

> 状态：已完成

## 目标

用一个可观测、可重启的 Python 进程承载 AI API、图片索引和 Bot 任务消费。

## 当前基线

FastAPI lifespan 启动共享模型、Qdrant client 和两个 Redis Streams consumer。日常开发可使用本机服务；Compose 作为最终完整运行方式和发布复核环境。

## 范围与非目标

保持单实例 Uvicorn 和显式 pipeline；不拆独立 worker、Celery、Kafka、微服务或编排框架。

## 依赖

依赖 Redis、Qdrant、Go 内部接口，以及 Phase 0 统一 Compose。

## 工作包

- 单进程共享 Chinese-CLIP、FastEmbed 和 Qdrant 连接。
- 消费 `index_post` 与 `bot_tasks`。
- 四次处理、pending 重领、最终记录并 ACK。
- `/healthz`、基础 `/readyz` 和能力 readiness。
- 结构化分段耗时、错误类别和 consumer 状态。

## 接口与数据流

Go 发布 Stream 事件；Python consumer 读取 Go 内部载荷，写 Qdrant或回调 Bot 回复；HTTP API提供检索、回答和能力元信息。

## 数据一致性

消息采用 at-least-once；向量 ID、删除条件和 `bot_replies.source_message_id` 唯一约束确保幂等；无 DLQ，依靠回填和对账恢复。

## 安全边界

内部 HTTP 强制 `X-Internal-Token`；AI 不持有 MySQL 或 MinIO 凭证；LLM Key只存在于环境变量。

## 失败模式

模型预热、Qdrant schema、consumer 停止和 LLM 缺配置分别体现在 capability readiness；AI 失败不阻塞主站业务写入。

## 测试矩阵

生命周期、单 consumer、重试、重领、最终 ACK、模型预热、依赖降级、冷启动和容器重启。

## 退出标准

六服务冷启动可复现；两个 consumer 各一份；重启后 pending 收敛；能力 readiness 不误报。

## 提交与环境证据

- `b942643`：统一 AI 单进程和 Compose。
- 历史阶段记录曾完成清空卷启动、两个 Stream consumer 正常且无遗留 pending；该记录不代表 Phase 7C/8 当前源码变更已经通过独立源码重建。
- `make check`、Compose 配置和 Redis 集成测试通过。

## 遗留项

Phase 7C 已补齐独立 Compose 故障注入证据；源码构建冷启动仍待补证。
