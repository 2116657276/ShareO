# ShareO — 项目架构说明

## 项目定位
Go Web 端摄影作品管理与社区分享平台，类小红书/朋友圈产品形态。

## 技术栈

| 层级 | 选型 |
|------|------|
| 后端语言 | Go |
| Web 框架 | Gin 1.12 |
| ORM | GORM + MySQL 驱动 |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis |
| 对象存储 | MinIO |
| 认证 | JWT (golang-jwt/v5) + bcrypt |
| 前端 | Go Templates + Bootstrap 5.3 + Alpine.js |
| 配置 | Viper + 环境变量覆盖 |

---

## 项目结构

```
ShareO/
├── cmd/server/main.go           # 入口：初始化 DB/Redis/MinIO/JWT → 启动 Gin
├── config.yaml.example          # 配置文件模板（不含密钥，提交 Git）
├── Makefile                     # start / run / build / migrate / seed / clean
├── start.sh                     # 一键启动（检测并拉起 MySQL/Redis/MinIO → 编译 → 打开浏览器）
├── migrations/
│   ├── 001_init.sql             # 10 张表 + 触发器 + 视图 + 存储过程 + 默认 admin
│   ├── 002_seed.sql             # 50 用户 + ~130 帖子测试数据
│   ├── 003_triggers.sql         # 点赞/评论自动更新计数器触发器
│   ├── 006_fulltext.sql         # posts.content FULLTEXT ngram 索引
│   └── 007_notifications.sql    # notifications 通知表
├── internal/
│   ├── config/config.go         # Viper 加载 + 环境变量覆盖 + Validate()
│   ├── model/                   # GORM 模型
│   │   ├── constants.go         # 状态/排序/角色/话题/通知 10 组常量
│   │   ├── user.go / post.go / post_image.go / comment.go
│   │   ├── like.go / favorite.go / follow.go
│   │   ├── topic.go / topic_post.go / system_log.go
│   │   └── notification.go      # 通知模型
│   ├── repository/              # DAO 层 + DB/Redis 连接
│   │   ├── db.go / redis.go
│   │   └── *_{repo}.go          # 12 个 Repository
│   ├── service/                 # 业务逻辑层
│   │   ├── auth_service.go / post_service.go / feed_service.go
│   │   ├── social_service.go / admin_service.go / user_service.go
│   │   │   ├── notification_service.go   # 通知发送+查询
│   │   ├── topic_service.go          # 话题查询聚合
│   │   └── hashtag.go                # #话题解析
│   ├── handler/                 # HTTP 处理器
│   │   ├── auth_handler.go / post_handler.go / feed_handler.go
│   │   ├── social_handler.go / admin_handler.go / user_handler.go
│   │   ├── upload_handler.go / helpers.go
│   │   ├── notification_handler.go   # 通知 API + 页面
│   │   └── topic_handler.go          # 话题聚合页
│   ├── middleware/
│   │   ├── auth.go              # JWT 认证（3 层：Optional / Required / Admin）
│   │   ├── ratelimit.go         # Redis 滑动窗口限流
│   │   └── nocache.go           # HTTP 禁用缓存
│   ├── router/router.go         # 54 条路由注册
│   └── pkg/
│       ├── jwt/jwt.go           # JWT 生成/解析
│       ├── response/response.go # 统一响应格式
│       └── upload/
│           ├── minio.go         # MinIO 上传/获取
│           └── thumbnail.go     # 缩略图生成（3 档尺寸）
└── web/
    ├── templates/               # Go HTML 模板
    │   ├── layout/              # header.html / footer.html
    │   ├── auth/                # login.html / register.html
    │   ├── feed/                # feed.html
    │   ├── post/                # create_post / post_detail / edit_post
    │   ├── user/                # user_profile / settings
    │   ├── admin/               # dashboard / review / users / logs
    │   ├── notifications.html   # 通知列表页
    │   └── topic.html           # 话题聚合页
    └── static/                  # CSS / JS / 图片
```

---

## 数据库（11 张表）

| 表 | 用途 |
|----|------|
| `users` | 用户（username, password_hash, role, status） |
| `posts` | 帖子（content, cover_image, status, is_deleted, like_count 等） |
| `post_images` | 帖子图片（一帖多图, sort_order） |
| `comments` | 评论（parent_id 多级回复, is_deleted） |
| `likes` | 点赞（UNIQUE user_id+post_id） |
| `favorites` | 收藏（UNIQUE user_id+post_id） |
| `follows` | 关注（UNIQUE follower+followee） |
| `topics` | 话题（name, status, post_count） |
| `topic_posts` | 话题-帖子关联（多对多） |
| `notifications` | 通知（user_id, type, actor_id, target_id, is_read） 🆕 |
| `system_logs` | 审计日志（触发器自动写入） |

**触发器**: likes/comments/posts 表 INSERT/DELETE 自动更新计数 + 写审计日志
**FULLTEXT**: posts.content 含 ngram 全文索引

---

## 路由总览

- **页面路由**: 16 条（服务端渲染 HTML）
- **API 路由**: 38 条（RESTful JSON）
- **总计**: 54 条

### 权限分级
| 级别 | 中间件 | 说明 |
|------|--------|------|
| 公开 | — | 无需登录 |
| 可选认证 | `OptionalAuth()` | 已登录则注入上下文，未登录也可访问 |
| 需登录 | `AuthRequired()` | 必须有效 JWT |
| 管理员 | `AuthRequired()` + `AdminRequired()` | 需 admin 角色 |

### API 限流
| 端点 | 限制 |
|------|------|
| 登录/注册 | 10 次/分钟/IP |
| 发帖 | 30 次/分钟 |
| 上传 | 20 次/分钟 |

---

## 关键设计决策

1. **模板**: `{{ define "header" }}` 组件模式，避免 Go template 多页面 define 冲突
2. **敏感配置**: 密钥通过 `SHAREO_*` 环境变量注入，config.yaml 不提交
3. **配置校验**: `Validate()` 启动时检查，避免运行时崩溃
4. **Feed 缓存**: Redis 首页最新 Feed（2min TTL），发帖/删帖/审核时主动失效
5. **权限模型**: 三层中间件 Optional → Required → Admin
6. **限流**: Redis INCR 滑动窗口，fail-open（Redis 不可用时放行）
7. **上传安全**: 魔数检测 + io.MultiReader 保数据完整 + 错误脱敏
8. **Cookie 安全**: SameSite=Lax + HttpOnly，防 CSRF
9. **点赞计数**: MySQL 触发器维护，避免 Go 层并发竞态
10. **头像缓存**: `userData()` 请求级 context 缓存，避免重复 DB 查询
11. **搜索**: FULLTEXT ngram 全文索引（中文友好），无索引时降级 LIKE
12. **软删除**: 帖子和评论 is_deleted 标记
13. **审核流**: 新帖 pending → admin approve/reject → Feed 仅展示 approved
14. **通知系统**: 5 类事件（like/comment/follow/repost/review）自动触发，API 列表+已读+未读数
15. **话题解析**: 正文 `#标签` 正则提取 + FindOrCreate 自动建 Topic + 编辑时重新关联
16. **图片缩略图**: Lanczos 3 档（thumb 300 / medium 1200 / original），Feed thumb、详情 medium
17. **并发上传**: Promise.all 替代串行 for...of，预览先行
18. **常量**: 状态/排序/角色/话题/通知 10 组统一在 `model/constants.go`
19. **Graceful Shutdown**: SIGINT/SIGTERM 信号监听 + http.Server.Shutdown + 10s 超时
20. **业务错误码**: 7 组码(0+1001~1006)与 HTTP 状态码解耦，API 消费者只检查 code==0
21. **事务保护**: Post Create/Update 中话题解析+关联均在同一事务内，避免孤儿 Topic
22. **应用层双写**: Like/Favorite/Comment 操作后同步计数，与 DB 触发器兼容共存

---

## 预设账号与启动

```bash
cd ShareO
cp config.yaml.example config.yaml   # 首次：编辑密码
./start.sh                           # 一键启动

# 管理员: admin / admin123
# MinIO Console: http://localhost:9001 (minioadmin / minioadmin)
# 生产环境: export SHAREO_DB_PASSWORD=xxx 等环境变量覆盖
```

## 测试规范

所有测试通过 API 模拟用户/管理员行为，严禁直接 SQL/Redis/MinIO 操作。

```
测试用户: test01~test10, 密码 256500
管理员: admin / admin123
```

## 仓库

```
git@github.com:2116657276/ShareO.git
```
