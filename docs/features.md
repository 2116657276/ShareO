# ShareO 功能清单

> 更新时间: 2026-07-21 | 版本: v2 Phase 1 后端加固中、Phase 2 语义搜图后端进行中 | 路由以 `internal/router/router.go` 为准 · 14 张业务表

---

## 1. 认证系统

**功能**: 注册、登录、POST 登出、修改密码、JWT 72h、个人信息、资料更新、封禁拦截、Redis 登录缓存；登出/改密/封禁会主动断开 WS

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | 13 条认证路由 |
| Handler | `internal/handler/auth_handler.go` | `Register`, `Login`, `Logout`, `ChangePassword`, `Me`, `UpdateProfile`, Web handlers |
| Service | `internal/service/auth_service.go` | `Register`, `Login`, `ChangePassword`, `GetProfile`, `UpdateProfile` |
| Repository | `internal/repository/user_repo.go` | `Create`, `FindByID`, `FindByUsername`, `UpdateFields` |
| JWT | `internal/pkg/jwt/jwt.go` | `Init`, `GenerateToken`, `ParseToken`, `ExpireDuration` |
| Login Cache | `internal/repository/auth_cache.go` | `CacheLoginToken`, `GetLoginToken`, `DeleteLoginToken`, `RefreshLoginToken` |
| Model | `internal/model/user.go` | `User` struct |
| Templates | `web/templates/auth/login.html`, `register.html`; `web/templates/user/settings.html` | |
| Migration | `migrations/001_init.sql` | `users` 表 |

---

## 2. Feed 流

**功能**: 卡片网格、最新/热门排序、"加载更多"分页、Redis 首页缓存(2min TTL)、骨架屏、gzip

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `GET /api/v1/feed`, `GET /home` |
| Handler | `internal/handler/feed_handler.go` | `GetFeed`, `HomePage`, `Search` |
| Service | `internal/service/feed_service.go` | `GetFeed`, `fillUserInteraction`, `cacheFeed`, `InvalidateCache`, `getCachedFeed` |
| Repository | `internal/repository/post_repo.go` | `Feed` (加权热度排序: `like*3+comment*2+view`) |
| Model | `internal/model/constants.go` | `SortLatest`, `SortHot` |
| Templates | `web/templates/feed/feed.html` | Alpine.js `feedPage` 组件 + LazyLoad |
| CSS | `web/static/css/style.css` | Feed 卡片样式 |

---

## 3. 帖子管理

**功能**: 创建(多图+文字+话题)、编辑(仅作者, 事务重关联)、软删除、详情页(图片轮播)

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | 8 条帖子路由 |
| Handler | `internal/handler/post_handler.go` | `Create`, `Update`, `Delete`, `GetByID`, `RecordView`, `Repost`, Web handlers |
| Service | `internal/service/post_service.go` | `Create`, `Update`, `Delete`, `GetByID`, `RecordView`, `Repost`, `resolveTopicIDsInTx` |
| Repository | `internal/repository/post_repo.go` | `Create`, `FindByID`, `FindByIDLight`, `Update`, `SoftDelete`, `IncrementView`, `IncrementShare` |
| Model | `internal/model/post.go`, `internal/model/post_image.go` | `Post`, `PostImage` |
| Templates | `web/templates/post/create_post.html`, `edit_post.html`, `post_detail.html` | |
| Migration | `migrations/001_init.sql`, `migrations/005_repost.sql` | `posts`, `post_images` 表 |

---

## 4. 图片上传与缩略图

**功能**: MinIO 存储、魔数检测(防 Content-Type 伪造)、Lanczos 三档缩略图(thumb 300 / medium 1200 / original)、图片代理(24h 缓存)、拖拽上传

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `POST /api/v1/upload`, `ANY /api/v1/images/*` |
| Handler | `internal/handler/upload_handler.go` | `UploadImage` , `ServeImage` |
| Upload Pkg | `internal/pkg/upload/minio.go` | `Init`, `MaxUploadSize`, `UploadImage`, `GetImage` |
| Thumbnail | `internal/pkg/upload/thumbnail.go` | `ProcessAndUpload`, `squareCrop`, `decodeImage`, `encodeJPEG` |
| Test | `internal/handler/upload_magic_test.go` | 11 项魔数测试 |
| Model | `internal/model/post_image.go` | `PostImage` |

---

## 5. 社交互动

**功能**: 点赞/收藏/关注(Toggle 模式)、评论(多级回复)、计数同步(COUNT(*) 子查询, 天然幂等)

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | 11 条社交路由 |
| Handler | `internal/handler/social_handler.go` | `ToggleLike`, `ToggleFavorite`, `ToggleFollow`, `CreateComment`, `DeleteComment` |
| Service | `internal/service/social_service.go` | `ToggleLike` , `ToggleFavorite`, `ToggleFollow`, `CreateComment` |
| Repository (Like) | `internal/repository/like_repo.go` | `Toggle`, `IsLiked`, `GetUserLikedPostIDs`, `GetUserLikedPosts` |
| Repository (Favorite) | `internal/repository/favorite_repo.go` | `Toggle`, `IsFavorited`, `GetUserFavoritedPostIDs`, `GetUserFavorites` |
| Repository (Follow) | `internal/repository/follow_repo.go` | `Toggle`, `IsFollowing`, `GetFollowing`, `GetFollowers` |
| Repository (Comment) | `internal/repository/comment_repo.go` | `Create`, `FindByPostID`, `SoftDelete` |
| Models | `internal/model/like.go`, `favorite.go`, `follow.go`, `comment.go` | |
| Migration | `migrations/001_init.sql` | `likes`, `favorites`, `follows`, `comments` 表 |

---

## 6. 转帖系统

**功能**: 纯文字/图片转帖、Feed PIP 画中画、详情页"查看原文"、share_count 计数

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `POST /api/v1/posts/:id/repost` |
| Handler | `internal/handler/post_handler.go` | `Repost` |
| Service | `internal/service/post_service.go` | `Repost` (设置 `IsRepost=1`, `RepostOfID`, 递增 share_count, 发送通知) |
| Repository | `internal/repository/post_repo.go` | `IncrementShare` |
| Model | `internal/model/post.go,30` | `Post.IsRepost`, `Post.RepostOfID`, `Post.RepostOf` |
| Migration | `migrations/005_repost.sql` | 添加 repost 字段 |

---

## 7. 搜索

**功能**: FULLTEXT ngram 全文搜索(中文友好)、启动时检测索引可用性、无索引时自动降级 LIKE

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `GET /api/v1/search` |
| Handler | `internal/handler/feed_handler.go` | `Search` |
| Service | `internal/service/feed_service.go` | `Search` |
| Repository | `internal/repository/post_repo.go` | `Search` (FULLTEXT, LIKE 降级) |
| FULLTEXT检测 | `internal/repository/post_repo.go` | `DetectFulltext` (查询 `INFORMATION_SCHEMA`) |
| Migration | `migrations/006_fulltext.sql` | `FULLTEXT INDEX idx_posts_content_ft` (ngram) |

---

## 8. 用户主页

**功能**: 头像+简介+统计、作品墙(3 列)、关注按钮、isOwnProfile/isFollowing 状态

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `GET /user/:id` |
| Handler | `internal/handler/user_handler.go` | `ProfilePage` |
| Service | `internal/service/user_service.go` | `GetProfile` |
| Repository | `internal/repository/user_repo.go,81-87` | `FindByID`, `GetFollowCounts` |
| Model | `internal/model/user.go` | `User` |
| Template | `web/templates/user/user_profile.html` | Alpine.js `profilePage` 组件 |

---

## 9. 管理员功能

**功能**: 仪表盘统计、帖子审核(通过/驳回+通知)、用户管理(封禁/解封)、系统日志、强制删帖

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | 12 条管理员路由 (中间件: `AuthRequired` + `AdminRequired`) |
| Handler | `internal/handler/admin_handler.go` | `GetStats`, `GetPendingPosts`, `ApprovePost`, `RejectPost`, `DeletePost`, `GetUsers`, `UpdateUserStatus`, `GetLogs` |
| Service | `internal/service/admin_service.go` | `GetDashboardStats`, `GetPendingPosts`, `ReviewPost` , `ListUsers`, `UpdateUserStatus`, `GetLogs` |
| Repository (Post) | `internal/repository/post_repo.go` | `UpdateStatus`, `AdminSoftDelete`, `CountByStatus`, `CountTotal` |
| Repository (User) | `internal/repository/user_repo.go` | `List`, `UpdateStatus`, `CountByRole`, `CountByStatus` |
| Repository (Log) | `internal/repository/log_repo.go` | `List` |
| Templates | `web/templates/admin/admin_dashboard.html`, `admin_review.html`, `admin_users.html`, `admin_logs.html` | |
| Migration | `migrations/001_init.sql`, `migrations/003_triggers.sql` | `system_logs` 表, 触发器 |

---

## 10. 通知系统

**功能**: 5 类通知(like/comment/follow/repost/review)、列表、单条已读、全部已读、未读数、铃铛徽章

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | 5 条通知路由 |
| Handler | `internal/handler/notification_handler.go` | `List`, `MarkRead`, `MarkAllRead`, `UnreadCount`, `NotificationsPage` |
| Service | `internal/service/notification_service.go` | `Send` , `List`, `MarkRead`, `MarkAllRead`, `UnreadCount`, `Stats` |
| Repository | `internal/repository/notification_repo.go` | `Create`, `List`, `MarkRead`, `MarkAllRead`, `UnreadCount` |
| Model | `internal/model/notification.go,14` | `NotifType*` 常量, `Notification` struct |
| Template | `web/templates/notifications.html` | Alpine.js 渲染 + 铃铛徽章在 `layout/header.html` |
| 触发点 | `social_service.go,123,126,76`; `post_service.go`; `admin_service.go` | |
| Migration | `migrations/007_notifications.sql` | `notifications` 表 |

---

## 11. 话题系统

**功能**: `#话题` 正则提取、自动创建/关联、编辑重关联、聚合页(ID/名称双通)、模板 `#链接` 渲染

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Router | `internal/router/router.go` | `GET /topic/:id` |
| Handler | `internal/handler/topic_handler.go` | `TopicPage` |
| Service | `internal/service/topic_service.go` | `GetTopicPage` |
| Hashtag 解析 | `internal/service/hashtag.go` | `ParseHashtags` |
| Repository | `internal/repository/topic_repo.go` | `FindByID`, `FindByName`, `FindOrCreate`, `ReplacePostTopics` |
| 模板函数 | `cmd/server/main.go` | `renderHashtags` (分段转义防 XSS) |
| Models | `internal/model/topic.go`, `internal/model/topic_post.go` | `Topic`, `TopicPost` |
| Template | `web/templates/topic.html` | |
| Migration | `migrations/001_init.sql` | `topics`, `topic_posts` 表 |

---

## 12. 安全与中间件

**功能**: 完整登录缓存校验、Go 标准库跨源写保护、精确 Origin、Redis 限流、NoCache、魔数校验、SameSite Cookie、错误脱敏、业务错误码

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Auth 中间件 | `internal/middleware/auth.go` | `AuthRequired` , `OptionalAuth`, `AdminRequired`, `RedirectIfAuth` |
| CSRF | `internal/middleware/cross_origin.go` | `http.CrossOriginProtection` 全局保护浏览器写操作 |
| RateLimit | `internal/middleware/ratelimit.go` | `RateLimit` |
| NoCache | `internal/middleware/nocache.go` | `NoCache` |
| 响应格式 | `internal/pkg/response/response.go` | 1001–1007，含 409 Conflict |
| 限流配置 | `internal/router/router.go` | 登录 10/min, 发帖 30/min, 上传 20/min |

---

## 13. 配置与启动

**功能**: YAML 配置加载、环境变量覆盖、精确 trusted origins 校验、Graceful Shutdown(10s 超时)、Compose 生命周期命令

| 层 | 文件 | 关键函数 |
|----|------|---------|
| Entry | `cmd/server/main.go` | `main` (初始化顺序: Config→DB→Redis→MinIO→JWT→LoginCache→Router) |
| Config | `internal/config/config.go` | `Load`, `applyEnvOverrides` , `Validate` |
| DB | `internal/repository/db.go` | `InitDB` (pool 配置) |
| Redis | `internal/repository/redis.go` | `InitRedis` |
| 工具函数 | `internal/handler/helpers.go` | `userData` (头像缓存), `getPageSizePair`, `calcPages`, `respondPage` |
| 配置模板 | `config.yaml.example` | |
| Compose | `deploy/docker-compose.yml` | MySQL/Redis/MinIO/Qdrant/AI API/Worker；仅自动结构迁移 |

---

## 14. 前端 UI

**功能**: Cammate 暖黄色调(#E8A840)、毛玻璃导航栏、汉堡菜单 Offcanvas、响应式布局(768px/480px 断点)

| 类别 | 文件 |
|------|------|
| 布局 | `web/templates/layout/header.html`, `footer.html` |
| CSS | `web/static/css/style.css` |
| CDN 依赖 | Bootstrap 5.3.3 + Bootstrap Icons 1.11.3 + Alpine.js 3.14.1 |
| 静态资源 | `web/static/img/default-avatar.svg`, `placeholder.svg` |

---

## 15. IM 私聊与邀请制群组

**功能**: 并发安全 DM、事务发消息、准确未读、邀请制群组、群主解散、普通成员退出、用户搜索、WebSocket 下行、在线状态、断线补偿和消息 ID 去重

| 层 | 文件 | 关键能力 |
|----|------|----------|
| Router/Handler | `internal/router/router.go`, `internal/handler/chat_handler.go` | 完整 Chat API、错误映射、WS 认证/Origin |
| Service | `internal/service/chat_service.go` | 权限、限制、精确未读、presence、Hub 扇出 |
| Repository | `internal/repository/chat_repo.go`, `presence.go` | 事务、行锁、游标 SQL、Redis TTL |
| WS | `internal/ws/hub.go`, `client.go` | 多连接、ping/pong、慢连接断开、会话吊销 |
| Model/Migration | `internal/model/chat.go`, `009_chat.sql`, `010_chat_hardening.sql` | 3 表、级联/限制外键、孤儿预检 |
| UI/Test | `web/templates/chat/chat.html`, `scripts/test_chat.sh` | 去重、恢复、群组操作、API 冒烟 |

---

## 数据库完整表清单

| 表 | Model | Repository | Migration |
|----|-------|-----------|-----------|
| `users` | `model/user.go` | `user_repo.go` | `001_init.sql` |
| `posts` | `model/post.go` | `post_repo.go` | `001_init.sql` |
| `post_images` | `model/post_image.go` | — (随 Post Preload) | `001_init.sql` |
| `comments` | `model/comment.go` | `comment_repo.go` | `001_init.sql` |
| `likes` | `model/like.go` | `like_repo.go` | `001_init.sql` |
| `favorites` | `model/favorite.go` | `favorite_repo.go` | `001_init.sql` |
| `follows` | `model/follow.go` | `follow_repo.go` | `001_init.sql` |
| `topics` | `model/topic.go` | `topic_repo.go` | `001_init.sql` |
| `topic_posts` | `model/topic_post.go` | — (随 Topic) | `001_init.sql` |
| `notifications` | `model/notification.go` | `notification_repo.go` | `007_notifications.sql` |
| `system_logs` | `model/system_log.go` | `log_repo.go` | `001_init.sql` |
| `conversations` | `model/chat.go` | `chat_repo.go` | `009_chat.sql`, `010_chat_hardening.sql` |
| `conversation_members` | `model/chat.go` | `chat_repo.go` | `009_chat.sql`, `010_chat_hardening.sql` |
| `messages` | `model/chat.go` | `chat_repo.go` | `009_chat.sql`, `010_chat_hardening.sql` |

---

## 路由总览

### API 路由
| 方法 | 路径 | Handler | 权限 | 限流 |
|------|------|---------|------|------|
| POST | `/api/v1/auth/register` | `authH.Register` | public | 10/min |
| POST | `/api/v1/auth/login` | `authH.Login` | public | 10/min |
| GET | `/api/v1/feed` | `feedH.GetFeed` | public | — |
| GET | `/api/v1/search` | `feedH.Search` | public | — |
| GET | `/api/v1/search/images?q=&limit=` | `imageSearchH.Search` | public | AI timeout 5s |
| GET | `/api/v1/posts/:id` | `postH.GetByID` | public | — |
| POST | `/api/v1/posts/:id/view` | `postH.RecordView` | public | — |
| GET | `/api/v1/posts/:id/comments` | `socialH.GetComments` | public | — |
| GET | `/api/v1/users/:id/following` | `socialH.GetFollowing` | public | — |
| GET | `/api/v1/users/:id/followers` | `socialH.GetFollowers` | public | — |
| POST | `/api/v1/auth/logout` | `authH.Logout` | user | — |
| GET | `/api/v1/auth/me` | `authH.Me` | user | — |
| PUT | `/api/v1/auth/profile` | `authH.UpdateProfile` | user | — |
| PUT | `/api/v1/auth/password` | `authH.ChangePassword` | user | — |
| POST | `/api/v1/posts` | `postH.Create` | user | 30/min |
| PUT | `/api/v1/posts/:id` | `postH.Update` | user | — |
| DELETE | `/api/v1/posts/:id` | `postH.Delete` | user | — |
| POST | `/api/v1/posts/:id/repost` | `postH.Repost` | user | — |
| POST | `/api/v1/posts/:id/like` | `socialH.ToggleLike` | user | — |
| POST | `/api/v1/posts/:id/favorite` | `socialH.ToggleFavorite` | user | — |
| GET | `/api/v1/favorites` | `socialH.GetFavorites` | user | — |
| GET | `/api/v1/likes` | `socialH.GetLikes` | user | — |
| POST | `/api/v1/posts/:id/comments` | `socialH.CreateComment` | user | — |
| DELETE | `/api/v1/comments/:cid` | `socialH.DeleteComment` | user | — |
| POST | `/api/v1/users/:id/follow` | `socialH.ToggleFollow` | user | — |
| GET | `/api/v1/users/search` | `chatH.SearchUsers` | user | — |
| POST | `/api/v1/upload` | `uploadH.UploadImage` | user | 20/min |
| GET | `/api/v1/notifications` | `notifH.List` | user | — |
| PUT | `/api/v1/notifications/:id/read` | `notifH.MarkRead` | user | — |
| PUT | `/api/v1/notifications/read-all` | `notifH.MarkAllRead` | user | — |
| GET | `/api/v1/notifications/unread-count` | `notifH.UnreadCount` | user | — |
| GET/HEAD | `/api/v1/images/*objectName` | `uploadH.ServeImage` | public | — |
| GET/POST | `/api/v1/conversations`, `/api/v1/conversations` | `chatH.ListConversations/CreateConversation` | user | — |
| GET/POST | `/api/v1/conversations/:id/messages` | `chatH.GetMessages/SendMessage` | user | — |
| PUT | `/api/v1/conversations/:id/read` | `chatH.MarkRead` | user | — |
| GET | `/api/v1/conversations/unread-count` | `chatH.UnreadCount` | user | — |
| POST | `/api/v1/conversations/:id/members` | `chatH.InviteMembers` | owner | — |
| DELETE | `/api/v1/conversations/:id/members/me` | `chatH.LeaveConversation` | member | — |
| DELETE | `/api/v1/conversations/:id` | `chatH.DissolveConversation` | owner | — |
| GET | `/api/v1/admin/stats` | `adminH.GetStats` | admin | — |
| GET | `/api/v1/admin/pending-posts` | `adminH.GetPendingPosts` | admin | — |
| DELETE | `/api/v1/admin/posts/:id` | `adminH.DeletePost` | admin | — |
| POST | `/api/v1/admin/posts/:id/approve` | `adminH.ApprovePost` | admin | — |
| POST | `/api/v1/admin/posts/:id/reject` | `adminH.RejectPost` | admin | — |
| GET | `/api/v1/admin/users` | `adminH.GetUsers` | admin | — |
| PUT | `/api/v1/admin/users/:id/status` | `adminH.UpdateUserStatus` | admin | — |
| GET | `/api/v1/admin/logs` | `adminH.GetLogs` | admin | — |

语义搜图的 Python 内部接口 `POST /v1/search/images`、`GET /readyz/search`、`GET /v1/meta/image-search`，以及 Go 索引载荷接口 `GET /internal/posts/:id/index-payload`、`GET /internal/posts/index-payloads` 均要求 `X-Internal-Token`，不属于公网 API。索引事件由审核通过、驳回、编辑、删除和 `make backfill-index` 发布到 Redis Streams。公网 `GET /api/v1/search/images` 返回去重后的 `post_id/image_id/image_url/score/post`，不暴露 MinIO `object_key`。

### Web 页面路由
| 方法 | 路径 | Handler | 权限 |
|------|------|---------|------|
| GET | `/` | `rootRedirect` | public (已登录分流) |
| GET | `/login` | `authH.LoginPage` | public |
| GET | `/register` | `authH.RegisterPage` | public |
| POST | `/login` | `authH.WebLogin` | public |
| POST | `/register` | `authH.WebRegister` | public |
| GET | `/home` | `feedH.HomePage` | user |
| GET | `/post/create` | `postH.CreatePage` | user |
| POST | `/post/create` | `postH.WebCreate` | user |
| GET | `/post/:id` | `postH.DetailPage` | user |
| GET | `/post/:id/edit` | `postH.EditPage` | user |
| POST | `/post/:id/edit` | `postH.WebUpdate` | user |
| POST | `/post/:id/comment` | `socialH.WebCreateComment` | user |
| GET | `/user/:id` | `userH.ProfilePage` | user |
| GET | `/settings` | `authH.SettingsPage` | user |
| POST | `/settings` | `authH.WebSettings` | user |
| POST | `/logout` | `authH.WebLogout` | user |
| GET | `/notifications` | `notifH.NotificationsPage` | user |
| GET | `/topic/:id` | `topicH.TopicPage` | user |
| GET | `/chat` | `chatH.ChatPage` | user |
| GET | `/admin/` | `adminH.Dashboard` | admin |
| GET | `/admin/review` | `adminH.Review` | admin |
| GET | `/admin/users` | `adminH.UsersPage` | admin |
| GET | `/admin/logs` | `adminH.LogsPage` | admin |

### 其他
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 健康检查 |
| GET | `/ws` | WebSocket，完整登录缓存与 Origin 校验 |
| Static | `/static/*` | 静态文件服务 |

---

## 迁移文件清单（10 个）

| 文件 | 内容 |
|------|------|
| `migrations/001_init.sql` | 10 张表 + 触发器 + 视图 + 存储过程 + 默认 admin 账号 |
| `migrations/002_seed.sql` | 可选开发管理员 + 50 用户 + 演示帖子；Compose 不自动执行 |
| `migrations/003_triggers.sql` | 系统日志触发器 (like/unlike/comment/post) |
| `migrations/004_clean_demo_posts.sql` | 清理演示占位数据 |
| `migrations/005_repost.sql` | 转帖系统 (is_repost, repost_of_id, repost_text) |
| `migrations/006_fulltext.sql` | `posts.content` FULLTEXT ngram 索引 |
| `migrations/007_notifications.sql` | `notifications` 通知表 |
| `migrations/008_reply_to_uid_index.sql` | `comments.reply_to_uid` 索引 |
| `migrations/009_chat.sql` | conversations / conversation_members / messages |
| `migrations/010_chat_hardening.sql` | 孤儿预检、owner NULL 化、级联/限制外键 |
