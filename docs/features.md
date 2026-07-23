# ShareO 最终功能矩阵

> 更新时间：2026-07-23 | 状态：Phase 7 发布收口中

## 功能能力

| 能力 | 用户入口 | 核心依赖 | 当前状态 | 主要证据 |
|---|---|---|---|---|
| 注册、登录、资料、改密、退出 | `/login`、`/register`、`/settings` | Go、MySQL、Redis JWT缓存 | 已完成 | `make check`、认证单测 |
| 图文发布与编辑 | `/post/create`、`/post/:id/edit` | MySQL、MinIO | 已完成 | 上传/帖子测试 |
| Feed与帖子详情 | `/home`、`/post/:id` | MySQL、Redis缓存 | 已完成 | 社区回归 |
| 全文搜索 | `/api/v1/search` | MySQL FULLTEXT/LIKE | 已完成 | 搜索回归 |
| 管理员审核和封禁 | `/admin` | MySQL、登录吊销 | 已完成 | 管理员/API测试 |
| 点赞、关注、评论 | 帖子与用户页面 | MySQL事务 | 已完成 | 社交事务测试 |
| 通知 | `/notifications` | MySQL | 已完成 | 通知单测 |
| 一对一私聊 | `/chat`、`/ws` | MySQL、Redis、WebSocket | 已完成 | 私聊集成、Hub测试 |
| 中文语义搜图 | `/api/v1/search/images` | MinIO、Chinese-CLIP、Qdrant | 工程完成 | 真实图片E2E；质量待7B |
| 正文向量检索 | 内部能力 | FastEmbed、Qdrant | 已完成 | 真实Qdrant round-trip |
| 私聊 RAG Bot | `/chat` 中 `shareo_bot` | Redis Streams、RAG、LLM | mock闭环完成 | `make test-ai-e2e`；真实DeepSeek待7B |

## 内容可见性

- 公共社区、语义搜图、正文索引和 Bot 引用只接受 `approved AND is_deleted=0`。
- 编辑已审核帖子会重新进入 pending 并删除旧图文向量。
- 驳回、用户删除和管理员删除都会发布幂等 delete 事件。
- Go 在公开搜索结果和 Bot回复落库前执行最终可见性过滤。

## 私聊约束

- 会话固定为两个不同用户，`dm_key` 保证并发幂等。
- 发送走 REST并先落 MySQL；WebSocket只负责下行。
- 消息以 ID去重，`before_id` 用于历史，`after_id` 用于断线补偿。
- 只有普通用户私聊固定 `shareo_bot` 才产生 AI任务。

## AI约束

- 图片模型固定 Chinese-CLIP revision，正文模型固定 `BAAI/bge-small-zh-v1.5`。
- Python不持有 MySQL或 MinIO凭证。
- LLM输出必须经过 chunk白名单和 Go帖子可见性二次校验。
- mock provider只用于测试；DeepSeek是 Phase 7B默认真实验收 provider。

## 明确排除

收藏、转帖、话题、群聊、Agent、工具调用、独立 Bot页面、消息撤回/编辑、音视频和生产级多租户均不在最终范围，也不作为后续路线图。

## 状态入口

当前工作只在 [TASK.md](../TASK.md) 维护，阶段完成定义见 [Phase 0–7](phases/README.md)。
