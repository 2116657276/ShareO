# 数据模型参考

> 结构事实源：`migrations/postgres/001_schema.sql` 与 `migrations/postgres/002_vector.sql`

## PostgreSQL 17 业务表

| 表 | 用途 | 关键约束 |
|---|---|---|
| `users` | 用户、管理员和固定Bot | username唯一；`is_bot`；Bot密码不可登录 |
| `posts` | 帖子与审核状态 | 作者级联；审核人删除置空；正文使用安全 `LIKE` 回退搜索 |
| `post_images` | 帖子图片顺序和尺寸 | 帖子删除级联 |
| `comments` | 评论与回复及热度计数 | 帖子、作者、父评论级联；回复用户删除置空；`like_count` 为评论点赞明细的同步计数 |
| `comment_likes` | 评论点赞 | `(user_id,comment_id)`唯一；评论或用户删除级联 |
| `likes` | 帖子点赞 | `(user_id,post_id)`唯一 |
| `favorites` | 私人帖子收藏 | `(user_id,post_id)`唯一；仅收藏者本人查询 |
| `follows` | 用户关注 | `(follower_id,followee_id)`唯一 |
| `notifications` | 点赞、评论、关注、审核通知 | 接收者和操作者删除级联 |
| `system_logs` | 管理审计 | 用户删除置空 |
| `conversations` | 一对一会话 | `dm_key`唯一 |
| `conversation_members` | 两名成员与读游标 | `(conversation_id,user_id)`唯一；会话/用户删除级联 |
| `messages` | 文本消息和引用meta | 会话/发送者删除级联；按会话和ID索引 |
| `bot_replies` | Bot任务幂等映射 | source和reply消息ID分别唯一 |
| `bot_task_outbox` | Bot任务事务外发箱 | message唯一；pending/published 状态与退避时间；会话/消息删除级联 |

## 核心关系

```text
users ──< posts ──< post_images
  │         ├────< comments
  │         │       └────< comment_likes
  │         └────< likes
  │         └────< favorites
  ├────< follows >──── users
  ├────< notifications
  └────< conversation_members >──── conversations ──< messages
                                                     └─ bot_replies
                                                     └─ bot_task_outbox
```

`conversations.dm_key` 由两名用户ID排序组成。业务层必须保证一个会话恰有两个不同成员，schema唯一约束负责并发收敛。

`messages.meta` 保存Bot引用等结构化附加信息。引用不是内容真相；每次对外返回前仍需按帖子状态过滤。对外响应可为每条可见引用临时补充 `preview`（首张图片、作者和正文摘要），该字段不作为 AI 写入的持久化内容。

## 事务不变量

- 创建帖子与图片关系、互动明细与计数、消息与会话时间、Bot 消息与幂等记录必须在各自事务中完成。
- 收藏是私人关系，不写帖子计数、不产生通知；收藏列表仍执行帖子可见性过滤。
- `last_read_message_id` 只能推进到同会话真实消息，并通过最大值语义防止倒退。
- `messages` 与 `bot_task_outbox` 在同一事务中写入；发布成功后 outbox 才标记 `published`。
- `bot_replies.source_message_id` 保证一条用户来源消息最多产生一条 Bot 回复。
- 用户编辑 approved帖子后状态回到 pending，并发布图文向量删除事件。

## pgvector 派生数据

AI 账号只访问 `ai` schema。两张派生表都使用 `vector(512)`，并建立 `vector_cosine_ops` HNSW 索引；图片帖子正文为空时不生成文本向量。

### `ai.image_embeddings`

- 图片 ID、帖子 ID、对象键、模型版本和 `embedding vector(512)`。
- `(post_id, image_id)` 唯一，`post_id` 建普通索引。
- 余弦距离通过 PostgreSQL `<=>` 运算符查询，upsert 和按帖子删除均幂等。

### `ai.post_chunk_embeddings`

- 文本块 ID、帖子 ID、文本、模型版本和 `embedding vector(512)`。
- 文本块 ID 使用 `post_id:chunk_no` 的 UUIDv5，支持帖子级替换和删除。
- `post_id` 建普通索引，余弦排序使用 `<=>` 与 HNSW `vector_cosine_ops`。

应用启动时校验向量列确实为 `vector(512)`，不自动覆盖不兼容 schema。两张表都能由 approved 帖子通过回填重建。

## Redis

- `shareo:stream:index_post`：`action=upsert|delete`、`post_id`。
- `shareo:stream:bot_tasks`：来源消息任务。
- 登录、Feed缓存和在线状态使用TTL，不能替代 PostgreSQL 真相。
