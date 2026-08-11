# ADR-002: 任务队列 —— Redis Streams

- 日期: 2026-07-19
- 状态: 已接受

## 背景

图片向量化必须异步（模型编码耗时不能阻塞发帖/审核路径），Bot 任务同理。生产者是 Go，消费者是 Python，需要一个跨语言、可靠投递的队列。

## 候选方案

1. RabbitMQ —— 经典 AMQP 中间件
2. Kafka —— 分布式日志
3. Redis Streams —— 复用现有 Redis
4. PostgreSQL 表轮询 —— 最土办法

## 决定

选 Redis Streams：两条流 `shareo:stream:index_post`、`shareo:stream:bot_tasks`，消费组 `ai-workers`；消费成功才 XACK，worker 启动时 XAUTOCLAIM 接管 pending 消息，全部消费逻辑幂等（at-least-once）。Bot 任务额外使用 PostgreSQL `bot_task_outbox`：消息和 pending 任务同事务写入，Go publisher 成功发布后标记完成，Redis 故障或 Go 重启不会丢失任务。

## 理由

- **零新增组件**：Redis 已在栈内，Go（go-redis）和 Python（redis-py）客户端都成熟。
- **事务边界清晰**：outbox 只解决 Go 数据库事务到 Redis 发布之间的间隙，不替代 Redis Streams 的消费确认和重试。
- Consumer group 原生提供确认、pending 列表、重领——毕设规模需要的可靠性语义全够。
- RabbitMQ/Kafka 的运维成本花在了毕设不考察的地方；本项目答辩重心在 AI 侧，架构上"用对"比"用重"加分。
- 轮询方案无推送语义、污染业务库，不取。

## 后果与代价

- at-least-once 要求消费幂等 → 向量点 ID 确定性生成（image_id / post_id+chunk_no），天然幂等。
- 无死信队列 → 约定初次处理失败后最多重试 3 次（共 4 次尝试）；仍失败的消息记录日志并 XACK 跳过，靠回填命令兜底。
- Redis 单点 → 与现状一致，毕设可接受；论文局限性提一句即可。
