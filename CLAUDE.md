# ShareO — 拍摄与作品管理系统

## 项目性质
Go + Gin Web 应用，后端 API + 服务端渲染 HTML。
数据库 MySQL，缓存 Redis，对象存储 MinIO。
前端 Bootstrap 5.3 + Alpine.js 3.14。

## Agent 角色与规则

本项目定义了三种 Agent 角色，各自权限和职责严格分离：

### 1. 测试 Agent（黑盒用户视角）

**身份**: 你是真实用户/管理员，不是开发者。不知道系统内部实现。

**不可违反的铁律**:
1. 禁止阅读任何 `.go` 源码文件
2. 只读白名单文档: `CLAUDE.md`, `FEATURES.md`, `TEST_CHECKLIST.md`, `PROJECT.md`, `feedback.md`
3. 禁止直接连接 MySQL / Redis / MinIO
4. 禁止执行任何 SQL 语句
5. 所有操作通过 `curl http://localhost:8080` 的 HTTP API
6. 测试账号: `test01`~`test10` 密码 `256500`，管理员 `admin` / `admin123`
7. 每次 API 调用检查 HTTP 状态码和 JSON `code` 字段（0=成功）
8. 注意 API 限流：登录/注册 10次/min，发帖 30次/min，上传 20次/min。429 状态码需等待 60s 重试
9. 测试结束后通过管理员 API 删除测试帖子，保留原有数据
10. Web 页面需验证 HTTP 状态码（200/403/401）

### 2. 修复 Agent（代码修改者）

**触发条件**: 存在 `TASK.md` 文件时方可工作。

**必须遵守**:
1. 仅修改 `TASK.md` 中明确指定的文件和行号
2. 禁止新增 `go.mod` 依赖包（require 列表不变）
3. 修改后必须 `go build -o bin/shareo cmd/server/main.go` 确认零错误
4. 禁止在代码中硬编码密码、token、密钥
5. 修复完成后在 `feedback.md` 底部标注修复状态
6. 禁止修改 HTML 模板、CSS、SQL 迁移文件（除非 TASK.md 明确指定）

### 3. 审查 Agent（代码评审者）

**触发条件**: 用户明确要求 "review" 或 "审查"。

**职责**:
1. 审查全量或指定范围的 Go 源码 + 模板 + CSS
2. 发现问题分为 P0(致命)/P1(高危)/P2(中危)/P3(低危)
3. 输出报告到 `CODE_REVIEW.md`，包含：问题描述、代码位置、修复建议
4. 可附带架构评估、性能分析、安全审计

## 配置说明

### 首次配置
```bash
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入本地数据库/Redis/MinIO 密码
```

### 敏感信息
生产环境通过环境变量覆盖（优先级 > config.yaml）：

| 环境变量 | 覆盖项 |
|----------|--------|
| `SHAREO_DB_PASSWORD` | 数据库密码 |
| `SHAREO_JWT_SECRET` | JWT 签名密钥 |
| `SHAREO_MINIO_ACCESS_KEY` | MinIO AccessKey |
| `SHAREO_MINIO_SECRET_KEY` | MinIO SecretKey |
| `SHAREO_REDIS_PASSWORD` | Redis 密码 |

`config.yaml` 已加入 `.gitignore`，禁止提交到版本控制。

## 项目规模速览

```
54 路由 | 11 表 | 50+ Go 源文件 | 17 模板 | 7 迁移 | 66 测试用例
```

## 模块架构

### 分层架构

```
cmd/server/main.go          ← 入口
      ↓
router/router.go            ← 54 条路由注册
      ↓
handler/ (9 个)              ← HTTP 请求/响应，调用 Service
      ↓
service/ (10 个)             ← 业务逻辑，调用 Repository
      ↓
repository/ (12 个)          ← 数据访问，封装 GORM 查询
      ↓
model/ (12 个)               ← GORM 模型定义 + 常量
```

**铁律**: Handler → Service → Repository，单向依赖，禁止反向调用。

### Handler 层 (9 个)

| Handler | 文件 | 职责 |
|---------|------|------|
| AuthHandler | `auth_handler.go` | 注册/登录/登出/个人信息/设置(Web+API) |
| PostHandler | `post_handler.go` | 帖子 CRUD + 转帖 + Web 页面 |
| FeedHandler | `feed_handler.go` | Feed 流/搜索/首页渲染 |
| SocialHandler | `social_handler.go` | 点赞/收藏/关注/评论(含 Web 表单) |
| AdminHandler | `admin_handler.go` | 仪表盘/审核/用户管理/日志 |
| UserHandler | `user_handler.go` | 用户主页 |
| UploadHandler | `upload_handler.go` | 图片上传+魔数检测 / 图片代理 |
| NotificationHandler | `notification_handler.go` | 通知列表/已读/未读数/通知页面 |
| TopicHandler | `topic_handler.go` | 话题聚合页(ID/名称双通) |
| `helpers.go` | — | userData(头像缓存), 分页辅助, 共享工具 |

### Service 层 (10 个)

| Service | 职责 |
|---------|------|
| AuthService | 注册/登录(bcrypt+JWT), 资料更新 |
| PostService | 帖子创建(含#话题解析)/编辑(事务重关联)/删除/转帖 |
| FeedService | Feed 流(Redis 缓存)+搜索(FULLTEXT), isLiked/isFavorited 批量填充 |
| SocialService | 点赞/收藏/关注 Toggle + 评论(多级回复+双通知) |
| AdminService | 审核(含通知)/统计(走 Repo)/用户管理/日志 |
| UserService | 用户主页数据聚合(ProfileData) |
| NotificationService | 通知发送(5 事件)/列表/已读/未读计数(自过滤) |
| TopicService | 话题 ID/Name 双查 + 帖子聚合 |
| `hashtag.go` | #Unicode 正则提取 + 去重 + ToLower |

### Repository 层 (12 个)

每个 entity 一个 Repo，封装所有 GORM 操作。命名规范: `*_repo.go`。

| Repo | 关键方法 |
|------|---------|
| UserRepo | CRUD, UpdateFields, CountByRole/Status, GetFollowCounts |
| PostRepo | CRUD, Feed(分页+排序+过滤), Search(FULLTEXT→LIKE降级), CountByStatus/Total |
| LikeRepo | Toggle, IsLiked, GetUserLikedPostIDs(批量), CountTotal |
| FavoriteRepo | Toggle, IsFavorited, GetUserFavoritedPostIDs(批量), GetUserFavorites |
| FollowRepo | Toggle, IsFollowing, GetFollowing/Followers(ORDER BY FIELD) |
| CommentRepo | CRUD, FindByPostID(分页+嵌套), SoftDelete, CountNonDeleted |
| TopicRepo | CRUD, FindByName(LOWER), FindOrCreate(ToLower), CountByStatus |
| NotificationRepo | Create, List(分页+unreadOnly), MarkRead, MarkAllRead, UnreadCount |
| LogRepo | List(分页+过滤) |

### Middleware 层 (3 个)

| 文件 | 功能 |
|------|------|
| `auth.go` | 4 层认证: OptionalAuth / AuthRequired / AdminRequired / RedirectIfAuth |
| `ratelimit.go` | Redis Lua 滑动窗口限流, fail-open |
| `nocache.go` | HTTP 禁用缓存(headers+meta) |

### Model 层 (12 个)

| 模型 | 表 | 说明 |
|------|-----|------|
| User | `users` | 用户 |
| Post | `posts` | 帖子(含软删除, 审核状态, 计数) |
| PostImage | `post_images` | 帖子图片(一帖多图) |
| Comment | `comments` | 评论(多级 parent_id, 软删除) |
| Like | `likes` | 点赞(UNIQUE user+post) |
| Favorite | `favorites` | 收藏(UNIQUE user+post) |
| Follow | `follows` | 关注(UNIQUE follower+followee) |
| Topic | `topics` | 话题 |
| TopicPost | `topic_posts` | 话题-帖子关联 |
| Notification | `notifications` | 通知(5 类型, is_read) |
| SystemLog | `system_logs` | 审计日志 |
| `constants.go` | — | 10 组常量(Status/Sort/Role/UserStatus/TopicStatus/NotifType) |

### pkg 包

| 包 | 文件 | 功能 |
|----|------|------|
| jwt | `jwt.go` | HMAC-SHA256 JWT 生成/解析, 72h 过期 |
| response | `response.go` | 统一 JSON 响应(业务错误码0+1001~1006,Success/Error/Page) |
| upload | `minio.go` | MinIO 连接/上传/代理 |
| upload | `thumbnail.go` | 缩略图生成(Lanczos 3 档+squareCrop+智能降级) |

## 环境检查顺序

```bash
# 0. 配置
test -f config.yaml || cp config.yaml.example config.yaml

# 1-3. 三大组件
mysql -u root -p"${MYSQL_PASS}" -e "SELECT 1"
redis-cli PING
lsof -i :9000   # MinIO

# 4-6. 编译+启动+验证
go build -o bin/shareo cmd/server/main.go
./bin/shareo &
curl http://localhost:8080/healthz  # → {"status":"ok","service":"ShareO"}
```

## API 调用模板

### 注册
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"test01","password":"256500","email":"test01@shareo.com"}'
```

### 登录并获取 Token
```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"test01","password":"256500"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")
```

### 上传图片（自动生成缩略图）
```bash
curl -s -X POST http://localhost:8080/api/v1/upload \
  -b "token=$TOKEN" \
  -F "file=@/path/to/image.jpeg"
# 返回: {"code":0,"data":{"url":".../medium/...","urls":{"thumb":"...","medium":"...","original":"..."}}}
```

### 发帖（#话题自动解析）
```bash
curl -s -X POST http://localhost:8080/api/v1/posts \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"#风光 晨曦中的老街","images":["/api/v1/images/posts/..."]}'
# 正文中的 #风光 会被自动解析、创建 Topic 并关联帖子
```

### 转帖
```bash
curl -s -X POST http://localhost:8080/api/v1/posts/:id/repost \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"转发文字","images":["/api/v1/images/posts/..."]}'
```

### 点赞/收藏/评论/关注（均为 Toggle 模式，自动触发通知）
```bash
curl -s -X POST http://localhost:8080/api/v1/posts/:id/like -b "token=$TOKEN"
curl -s -X POST http://localhost:8080/api/v1/posts/:id/favorite -b "token=$TOKEN"
curl -s -X POST http://localhost:8080/api/v1/posts/:id/comments -b "token=$TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"评论内容"}'
curl -s -X POST http://localhost:8080/api/v1/users/:id/follow -b "token=$TOKEN"
```

### 通知操作
```bash
curl -s -b "token=$TOKEN" "http://localhost:8080/api/v1/notifications?unread_only=true"
curl -s -b "token=$TOKEN" http://localhost:8080/api/v1/notifications/unread-count
curl -s -X PUT -b "token=$TOKEN" http://localhost:8080/api/v1/notifications/1/read
curl -s -X PUT -b "token=$TOKEN" http://localhost:8080/api/v1/notifications/read-all
```

### 管理员操作
```bash
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")

curl -s -b "token=$ADMIN_TOKEN" http://localhost:8080/api/v1/admin/stats
curl -s -b "token=$ADMIN_TOKEN" http://localhost:8080/api/v1/admin/pending-posts
curl -s -X POST http://localhost:8080/api/v1/admin/posts/:id/approve -b "token=$ADMIN_TOKEN"
curl -s -X POST http://localhost:8080/api/v1/admin/posts/:id/reject -b "token=$ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"comment":"违规原因"}'
curl -s -X DELETE http://localhost:8080/api/v1/admin/posts/:id -b "token=$ADMIN_TOKEN"
curl -s -b "token=$ADMIN_TOKEN" "http://localhost:8080/api/v1/admin/users?page_size=50"
curl -s -X PUT http://localhost:8080/api/v1/admin/users/:id/status -b "token=$ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"status":0}'
```

## 关键 API 端点速查

```
POST /api/v1/auth/register             注册
POST /api/v1/auth/login                登录
GET  /api/v1/auth/me                   个人信息
PUT  /api/v1/auth/profile              更新资料（bio, email, avatar_url）
POST /api/v1/upload                    上传图片（自动 thumb/medium/original 三档）
POST /api/v1/posts                     发帖 {content, images[], topic_ids[]}  #话题自动解析
PUT  /api/v1/posts/:id                 编辑帖子
DELETE /api/v1/posts/:id               删除帖子（仅作者）
POST /api/v1/posts/:id/repost          转帖 {text?, images?}
POST /api/v1/posts/:id/like            点赞切换（触发通知）
POST /api/v1/posts/:id/favorite        收藏切换
GET  /api/v1/favorites                 收藏列表
POST /api/v1/posts/:id/comments        发评论 {content, parent_id?, reply_to_uid?}（触发通知）
GET  /api/v1/posts/:id/comments        评论列表
DELETE /api/v1/comments/:cid           删评论（仅作者）
POST /api/v1/users/:id/follow          关注切换（触发通知）
GET  /api/v1/users/:id/following       关注列表
GET  /api/v1/users/:id/followers       粉丝列表
GET  /api/v1/feed                      Feed流 (?sort=latest|hot&page=&page_size=)
GET  /api/v1/search                    搜索 (?q=关键词) FULLTEXT ngram+LIKE降级
GET  /api/v1/notifications             通知列表 (?unread_only=true&page=&page_size=)
PUT  /api/v1/notifications/:id/read    标记单条已读
PUT  /api/v1/notifications/read-all    全部已读
GET  /api/v1/notifications/unread-count 未读数量
GET  /api/v1/admin/stats               管理员统计
GET  /api/v1/admin/pending-posts       待审核帖
POST /api/v1/admin/posts/:id/approve   通过审核（触发通知）
POST /api/v1/admin/posts/:id/reject    驳回审核（触发通知）
DELETE /api/v1/admin/posts/:id         管理员强制删帖
GET  /api/v1/admin/users               用户列表
PUT  /api/v1/admin/users/:id/status    封禁/解封 {status: 0|1}
GET  /api/v1/admin/logs                系统日志
ANY  /api/v1/images/*objectName        图片代理（GET+HEAD, 24h 缓存）
```

## Web 页面验证

```bash
# 公开页面 → 200
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/login
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/register

# 需登录 → 200
curl -s -b "token=$TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/home
curl -s -b "token=$TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/post/create
curl -s -b "token=$TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/settings
curl -s -b "token=$TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/notifications
curl -s -b "token=$TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/topic/1

# 管理员 → 200
curl -s -b "token=$ADMIN_TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/admin/

# 权限验证
curl -s -b "token=$USER_TOKEN" -o /dev/null -w "%{http_code}" http://localhost:8080/admin/  # → 403
```

## 测试数据清理

1. `GET /api/v1/feed?page_size=999` 记录测试前 PRESERVED_IDS
2. `DELETE /api/v1/admin/posts/:id` 逐个删除测试帖
3. `redis-cli DEL feed:latest:page1` 清缓存
4. `GET /api/v1/feed` 验证帖子数恢复

## 文档索引

| 文档 | 用途 |
|------|------|
| `CLAUDE.md` | 本文档 — Agent 规则 + API 模板 + 架构速览 |
| `FEATURES.md` | 完整功能清单 + 54 路由表 |
| `PROJECT.md` | 项目架构说明 + 设计决策 |
| `TEST_CHECKLIST.md` | 分步 E2E 测试用例 |
| `feedback.md` | 改进记录 + 测试状态 |
| `CODE_REVIEW.md` | 最新审查报告 |
