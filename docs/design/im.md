# 一对一私聊设计

## 边界

系统只支持两个不同用户之间的文本私聊。发送走 REST 并以 PostgreSQL 为真相，WebSocket 只负责下行；不支持群聊、邀请、成员管理、撤回、编辑、图片消息、音视频或端到端加密。

固定 `shareo_bot` 复用同一会话、消息、未读和 WebSocket 管线，但 Bot 账号禁止登录。普通用户之间的 DM 不触发 AI。

## 会话与事务

`conversations.dm_key` 固定为 `min(uid):max(uid)` 并有唯一约束。并发创建通过冲突后回查收敛到同一会话，业务层保证恰有两名不同成员。

发送消息在同一事务中：

1. 验证发送者属于会话。
2. 插入 `messages`。
3. 更新 `conversations.updated_at`。
4. 以最大值推进发送者的 `last_read_message_id`。
5. 若目标是固定 Bot，同时写入 `bot_task_outbox` pending 记录。

任何一步失败整笔回滚。准确未读是读游标之后且发送者不是自己的消息数。

## REST 与恢复协议

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/v1/conversations` | 最近更新优先，返回对端、最后消息和准确未读；已建立但暂无消息的普通DM也可展示 |
| POST | `/api/v1/conversations` | `{"user_id":2}`，幂等获取或创建DM |
| GET | `/api/v1/conversations/:id/messages` | `before_id` 历史或 `after_id` 恢复，不能并用 |
| POST | `/api/v1/conversations/:id/messages` | `{"content":"..."}`，trim 后为 1–2000 字符 |
| PUT | `/api/v1/conversations/:id/read` | 只接受本会话真实消息，读标记不能倒退 |
| GET | `/api/v1/conversations/unread-count` | 全部DM准确未读总数 |
| GET | `/api/v1/users/search` | 搜索正常用户并排除调用者；固定 `shareo_bot` 也可被搜索 |
| GET | `/ws` | Cookie或Bearer登录态 |

历史分页向前读取；断线补偿以 `after_id` 按ID升序返回。客户端把REST发送响应和WebSocket回推统一按 message ID去重。

## WebSocket 与在线状态

握手复用JWT和Redis登录缓存，不接受已退出、改密或被封禁用户的旧Token。Hub允许一个用户多个连接，最后一个连接关闭后才删除在线状态；慢连接写缓冲满时主动断开。

连接断开后按指数退避重连，成功后立即使用最后渲染的 message ID调用 `after_id`。Redis在线状态不是消息真相，丢失时只影响在线展示。

## Bot 扩展

Go只在发送者为普通用户、目标为固定 `shareo_bot` 的有效DM中写入 outbox。消息事务提交后由单进程 publisher 按退避重试发布 `bot_tasks`；因此 Redis 或 AI 故障不阻塞普通消息，Go 重启后也能补发未完成任务。

Bot 回复协议和引用安全见 [RAG 设计](rag.md)。

## 安全与错误

- 所有会话接口验证成员身份，越权返回403。
- 参数错误400，不存在404，状态冲突409，未知故障500。
- Origin只接受同源或精确可信配置；浏览器写操作受跨源保护。
- 登出、改密和封禁后主动断开该用户全部连接。
- 数据库原文错误和内部Token不得返回客户端。

## 验收

并发DM幂等、事务回滚、准确未读、读游标单调、消息排序、越权、分页冲突、Hub多连接/慢连接和会话吊销均由自动化覆盖。人工浏览器演示仍需复核 WebSocket 实时下行，不恢复群聊场景。
