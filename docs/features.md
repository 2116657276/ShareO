# ShareO 最终功能矩阵

> 更新时间：2026-07-30 | 本文只维护稳定能力；完工状态和证据边界以 [REVIEW.md](REVIEW.md) 为准

## 功能能力

| 能力 | 用户入口 | 核心依赖 | 当前状态 | 主要证据 |
|---|---|---|---|---|
| 注册、登录、资料、改密、退出 | `/login`、`/register`、`/settings` | Go、MySQL、Redis JWT缓存 | 已完成 | `make check`、认证单测 |
| 图文发布与编辑 | `/post/create`、`/post/:id/edit` | MySQL、MinIO | 已完成 | 上传/帖子测试 |
| Feed与帖子详情 | `/home`、`/post/:id` | MySQL、Redis缓存 | 单/双列均使用 1200px 高清中图，新上传采用 JPEG 90% 质量；待浏览器复验 | 社区回归 |
| 混合搜贴 | `/api/v1/search` | BGE + Qdrant 正文语义召回、MySQL ngram FULLTEXT/LIKE 关键词召回 | 已完成固定权重融合、可见性复核和关键词降级；不读取图片向量 | 单测、集成与 32 条本机开发集 |
| 管理员审核和封禁 | `/admin` | MySQL、登录吊销 | 已完成 | 管理员/API测试 |
| 点赞、关注、评论 | 帖子与用户页面 | MySQL事务 | 已完成 | 社交事务测试 |
| 私人收藏 | 帖子详情、“我的”收藏 Tab | MySQL | 仅本人可见，不通知作者、不公开计数、不参与热门排序 | 收藏服务、路由与可见性测试 |
| 通知 | `/notifications` | MySQL | 关注、点赞、评论三类社区通知；评论未读映射到“我的”红点 | 通知单测 |
| 一对一私聊 | `/chat`、`/ws` | MySQL、Redis、WebSocket | 两级 WhatsApp 风格界面；列表只显示普通用户会话，导航独立展示私聊未读数量 | 私聊集成、Hub测试 |
| 中文语义搜图 | `/search/images`、`/api/v1/search/images` | MinIO、Chinese-CLIP、Qdrant | 工程闭环与统一工作台已完成；只读取图片向量 | 当前 47 图、34 条搜图评测；AI/Qdrant 故障受控 503 |
| 关注动态 | `/following`、`/api/v1/feed/following` | MySQL、现有 follows 与 Feed | 顶部有限关注头像栏和关注对象单列动态 | 关注动态接口、可见性与分页测试 |
| 正文向量检索 | 内部能力 | FastEmbed、Qdrant | 已完成 | 真实Qdrant round-trip |
| 私聊 RAG Bot | `/chat` 中 `shareo_bot` | Redis Streams、RAG、LLM | 已完成 | `make test-api`、当前 RAG 机器评测；来源命中率 0.8333、引用可访问率 100%、LLM 失败固定兜底，详见 Final Freeze |
| 只读社区知识 Agent | 搜索页“问问小O”进入专属 Agent 会话 | LangGraph、RAG、Qdrant、Go 内部只读接口 | Bot 不出现在普通私聊列表；复用既有会话与 `ai_mode=agent`，不新增公开 Agent API | 四个只读工具、步骤卡片、源码跨服务 E2E、真实集成和故障恢复测试 |

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
- mock provider只用于工程测试；本次 Final Freeze 使用当前配置的真实 Provider。
- 当前本机故障降级范围和未运行的 Compose 故障注入见 [`docs/REVIEW.md`](REVIEW.md) 与 [Final Freeze](evidence/final-freeze/README.md)。

## 明确排除

转帖、话题、群聊、写入型 Agent、外部工具、独立 Bot页面、消息撤回/编辑、音视频和生产级多租户均不在最终范围，也不作为后续路线图。私人收藏与同一私聊入口中的只读 Agent 是经 ADR 明确批准的范围例外。

默认 `shareo_bot` 消息仍是固定触发、固定检索—生成—回调管线；显式深度分析才进入受限 Agent。简历不得把该功能改称为通用 Agent。

## 状态入口

当前冻结状态只在 [TASK.md](../TASK.md) 和 [完工冻结审查](REVIEW.md) 维护。
