# ShareO 功能矩阵

> 更新时间：2026-08-11 | 本文只维护稳定能力；当前状态和证据边界以 [REVIEW.md](REVIEW.md) 为准

## 功能能力

| 能力 | 用户入口 | 核心依赖 | 当前状态 | 主要证据 |
|---|---|---|---|---|
| 注册、登录、资料、改密、退出 | `/login`、`/register`、`/settings` | Go、PostgreSQL、Redis JWT缓存 | 已实现 | 认证单测与接口回归 |
| 响应式界面与主题 | 全部页面、“设置和动态” | 共享模板、CSS、浏览器本地偏好 | 已实现手机/桌面响应式布局；默认 ShareO 琥珀色，可切换经典黑白主题；两套主题保持低对比度 | 浏览器 smoke、模板契约与静态检查 |
| 图文发布与编辑 | `/post/create`、`/post/:id/edit` | PostgreSQL、MinIO | 已实现 | 上传/帖子测试 |
| Feed与帖子详情 | `/home`、`/post/:id` | PostgreSQL、Redis缓存 | 单/双列均使用 1200px 高清中图，新上传采用 JPEG 90% 质量；人工 smoke 已覆盖 Feed，完整移动端/实时 WebSocket仍未形成浏览器自动化证据 | 社区回归 |
| 混合搜贴 | `/api/v1/search` | BGE + pgvector 正文语义召回、PostgreSQL `LIKE` 关键词召回 | 已实现固定权重融合、可见性复核和关键词降级；不读取图片向量 | 单测、集成与 32 条本机开发集 |
| 管理员审核和封禁 | `/admin` | PostgreSQL、登录吊销 | 已实现 | 管理员/API测试 |
| 点赞、关注、评论 | 帖子与用户页面 | PostgreSQL 事务 | 已实现 | 社交事务测试 |
| 私人收藏 | 帖子详情、“我的”收藏 Tab | PostgreSQL | 仅本人可见，不通知作者、不公开计数、不参与热门排序 | 收藏服务、路由与可见性测试 |
| 通知 | `/notifications` | PostgreSQL | 关注、点赞、评论三类社区通知；评论未读映射到“我的”红点 | 通知单测 |
| 一对一私聊 | `/chat`、`/ws` | PostgreSQL、Redis、WebSocket | 两级私聊界面；列表包含空会话并展示对方资料、最后消息和未读数量；导航独立展示私聊未读数量 | 私聊集成、Hub测试 |
| 中文语义搜图 | `/search/images`、`/api/v1/search/images` | MinIO、Chinese-CLIP、PostgreSQL pgvector | 已实现工程闭环与统一工作台；Go 使用校准置信度阈值 `0.39` 过滤低置信度结果 | 当前 47 条帖子语料、38 条搜图评测；Recall@5 `0.9222`、no-match `1.0`；AI/pgvector 故障受控 503 |
| 关注动态 | `/following`、`/api/v1/feed/following` | PostgreSQL、现有 follows 与 Feed | 完整关注用户名单独显示；关注对象没有帖子时仍显示用户资料，动态使用同一帖子流 | 关注动态接口、可见性与分页测试 |
| 响应式帖子流与评论热度 | `/home`、`/following`、`/post/:id` | PostgreSQL、静态 CSS/JS | 双主题低对比度布局；帖子多图横向轮播；主页展示高赞评论；评论可点赞/取消点赞；网页评论失败显示稳定提示 | 静态模板、迁移和接口契约审查 |
| 正文向量检索 | 内部能力 | FastEmbed、Psycopg 3、pgvector | 已实现 | PostgreSQL pgvector round-trip |
| 私聊 RAG Bot | `/chat` 中 `shareo_bot` | PostgreSQL outbox、Redis Streams、RAG、LLM | 已实现；消息与任务 outbox 同事务，publisher 可重试补发；可见引用响应增加帖子图片、作者和摘要预览 | 集成测试、当前报告 v1 RAG 机器评测；完整来源覆盖率 `0.9333`、引用精度 `0.9000`、可访问率 `100%`，详见 Final Freeze |
| 只读社区知识 Agent | 搜索页“问问小O”进入专属 Agent 会话 | LangGraph、RAG、pgvector、Go 内部只读接口 | Bot 不出现在普通私聊列表；复用既有会话与 `ai_mode=agent`，按问题意图收窄工具面，不新增公开 Agent API | 当前报告 v1 的 36 条真实 Provider 评测；完整来源覆盖率 `0.9722`、引用精度 `0.9583`、禁止/额外工具 `0`、注入拒答失败 `0` |

## 内容可见性

- 公共社区、语义搜图、正文索引和 Bot 引用只接受 `approved AND is_deleted=0`。
- 编辑已审核帖子会重新进入 pending 并删除旧图文向量。
- 驳回、用户删除和管理员删除都会发布幂等 delete 事件。
- Go 在公开搜索结果和 Bot 回复落库前执行最终可见性过滤。

## 私聊约束

- 会话固定为两个不同用户，`dm_key` 保证并发幂等。
- 发送走 REST 并先落 PostgreSQL；WebSocket 只负责下行。
- 消息以 ID 去重，`before_id` 用于历史，`after_id` 用于断线补偿。
- 只有普通用户私聊固定 `shareo_bot` 才产生 AI任务。

## AI约束

- 图片模型固定 Chinese-CLIP revision，正文模型固定 `BAAI/bge-small-zh-v1.5`。
- Python 不持有 PostgreSQL `public` 或 MinIO 凭证，只使用 AI 账号访问 `ai` schema。
- LLM 输出必须经过 chunk 白名单和 Go 帖子可见性二次校验。
- mock Provider 只用于工程测试；本次报告使用当前配置的真实 Provider。
- 当前本机故障降级范围见 [`docs/REVIEW.md`](REVIEW.md) 与 [Final Freeze](evidence/final-freeze/README.md)。旧 Compose 路径已退役。

## 明确排除

转帖、话题、群聊、写入型 Agent、外部工具、独立 Bot页面、消息撤回/编辑、音视频和生产级多租户均不在当前范围，也不作为后续路线图。私人收藏与同一私聊入口中的只读 Agent 是经 ADR 明确批准的范围例外。

默认 `shareo_bot` 消息仍是固定触发、固定检索—生成—回调管线；显式深度分析才进入受限 Agent。简历不得把该功能改称为通用 Agent。

## 状态入口

当前状态只在 [TASK.md](../TASK.md) 和 [工程审查](REVIEW.md) 维护。
