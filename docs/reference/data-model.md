# 数据模型参考

> 结构事实源：`migrations/001_init.sql`

## MySQL 13 表

| 表 | 用途 | 关键约束 |
|---|---|---|
| `users` | 用户、管理员和固定Bot | username唯一；`is_bot`；Bot密码不可登录 |
| `posts` | 帖子与审核状态 | 作者级联；审核人删除置空；正文FULLTEXT |
| `post_images` | 帖子图片顺序和尺寸 | 帖子删除级联 |
| `comments` | 评论与回复 | 帖子、作者、父评论级联；回复用户删除置空 |
| `likes` | 帖子点赞 | `(user_id,post_id)`唯一 |
| `favorites` | 私人帖子收藏 | `(user_id,post_id)`唯一；仅收藏者本人查询 |
| `follows` | 用户关注 | `(follower_id,followee_id)`唯一 |
| `notifications` | 点赞、评论、关注、审核通知 | 接收者和操作者删除级联 |
| `system_logs` | 管理审计 | 用户删除置空 |
| `conversations` | 一对一会话 | `dm_key`唯一 |
| `conversation_members` | 两名成员与读游标 | `(conversation_id,user_id)`唯一；会话/用户删除级联 |
| `messages` | 文本消息和引用meta | 会话/发送者删除级联；按会话和ID索引 |
| `bot_replies` | Bot任务幂等映射 | source和reply消息ID分别唯一 |

## 核心关系

```text
users ──< posts ──< post_images
  │         ├────< comments
  │         └────< likes
  │         └────< favorites
  ├────< follows >──── users
  ├────< notifications
  └────< conversation_members >──── conversations ──< messages
                                                     └─ bot_replies
```

`conversations.dm_key` 由两名用户ID排序组成。业务层必须保证一个会话恰有两个不同成员，schema唯一约束负责并发收敛。

`messages.meta` 保存Bot引用等结构化附加信息。引用不是内容真相；每次对外返回前仍需按帖子状态过滤。

## 事务不变量

- 创建帖子与图片关系、互动明细与计数、消息与会话时间、Bot消息与幂等记录必须在各自事务中完成。
- 收藏是私人关系，不写帖子计数、不产生通知；收藏列表仍执行帖子可见性过滤。
- `last_read_message_id` 只能推进到同会话真实消息，并通过最大值语义防止倒退。
- `bot_replies.source_message_id` 保证一条用户来源消息最多产生一条Bot回复。
- 用户编辑 approved帖子后状态回到 pending，并发布图文向量删除事件。

## Qdrant 派生数据

### `images`

- 512维 cosine，point ID为 `image_id`。
- payload：`post_id`、`image_id`、`object_key`、`created_at`、`model_revision`。
- `post_id` 建 payload index。

### `post_chunks`

- 512维 cosine，point ID为 `post_id:chunk_no` 的 UUIDv5。
- payload：`post_id`、`chunk_id`、`chunk_text`、`model_name`、`created_at`。
- `post_id` 和 `chunk_id` 建 payload index。

Qdrant collection 不兼容时启动明确失败，不自动覆盖已有数据。两者都能由 approved帖子通过回填重建。

## Redis

- `shareo:stream:index_post`：`action=upsert|delete`、`post_id`。
- `shareo:stream:bot_tasks`：来源消息任务。
- 登录、Feed缓存和在线状态使用TTL，不能替代MySQL真相。
