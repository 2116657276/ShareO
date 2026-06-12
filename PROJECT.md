# ShareO — 拍摄与作品管理系统

## 项目定位

基于 Go 语言构建的 Web 端摄影作品管理与社区分享平台，类比小红书/朋友圈的产品形态，支持普通用户发帖互动、管理员审核管理。

## 技术栈

| 层级 | 选型 | 版本 |
|------|------|------|
| 后端语言 | Go | 1.22+ |
| Web框架 | Gin | 1.12 |
| ORM | GORM + MySQL驱动 | 1.31 |
| 数据库 | MySQL | 8.0 (Homebrew) |
| 缓存 | Redis | 8.2 (Homebrew) |
| 对象存储 | MinIO | 最新版 (Homebrew) |
| 认证 | JWT (golang-jwt/v5) | — |
| 密码加密 | bcrypt | — |
| 前端 | Go Templates + Bootstrap 5.3 + Alpine.js 3.14 | — |
| 配置管理 | Viper | — |

## 项目结构

```
ShareO/
├── cmd/server/main.go          # 入口：初始化 DB/Redis/MinIO → 启动 Gin
├── config.yaml                 # MySQL/Redis/MinIO/JWT 配置
├── Makefile                    # start/run/build/migrate/seed
├── start.sh                    # 一键启动脚本(自动检测并拉起三大组件+编译+打开浏览器)
├── test_api.sh                 # 全量 API 测试脚本(70+ 用例)
├── migrations/
│   ├── 001_init.sql            # 10 张表 + 视图 + 存储过程 + 默认 admin
│   ├── 002_seed.sql            # 50 用户 + ~130 帖子的测试数据
│   └── 003_triggers.sql        # 点赞/评论自动更新计数器触发器
├── internal/
│   ├── config/config.go        # Viper 配置加载
│   ├── model/                  # GORM 模型 (10 个文件)
│   ├── repository/             # DAO 层 + DB/Redis 连接初始化
│   ├── service/                # 业务逻辑层 (5 个 service)
│   ├── handler/                # HTTP 处理器 (6 个 handler + helpers)
│   ├── middleware/auth.go      # JWT 认证 + Admin 权限 + 可选认证
│   ├── router/router.go        # 路由注册 (43 条路由)
│   └── pkg/{jwt,response,upload}/
└── web/
    ├── templates/
    │   ├── layout/header.html  # 导航栏 + 汉堡菜单 Offcanvas
    │   ├── layout/footer.html
    │   ├── auth/{login,register}.html
    │   ├── feed/feed.html      # Feed 主页(搜索+卡片网格+加载更多)
    │   ├── post/{create_post,post_detail,edit_post}.html
    │   ├── user/user_profile.html
    │   └── admin/{dashboard,review,users,logs}.html
    └── static/{css,js,img}/
```

## 数据库设计 (10 张表)

| 表名 | 用途 | 关键字段 |
|------|------|---------|
| `users` | 用户 | username, password_hash, role(user/admin), status(1正常/0封禁) |
| `posts` | 帖子 | user_id, content, cover_image, status(pending/approved/rejected), like_count |
| `post_images` | 帖子图片 | post_id, image_url, sort_order (一帖多图) |
| `comments` | 评论 | post_id, user_id, parent_id (多级回复), content |
| `likes` | 点赞 | user_id, post_id (UNIQUE防重复) |
| `favorites` | 收藏 | user_id, post_id (UNIQUE) |
| `topics` | 话题/挑战赛 | name, description, status |
| `topic_posts` | 话题-帖子关联 | topic_id, post_id (多对多) |
| `follows` | 关注 | follower_id, followee_id (UNIQUE) |
| `system_logs` | 审计日志 | user_id, action, detail |

触发器: `likes` 表 INSERT/DELETE 自动更新 `posts.like_count` + 写审计日志
视图: `view_feed_stream` 脱敏 Feed 流(隐藏密码等敏感字段)
索引: 时间排序、热度排序、外键列均有 B+Tree 索引

## 路由体系

### 页面路由 (服务端渲染)
```
GET  /                    → 首页(登录页，已登录自动分流)
GET  /login               → 登录页(用户/管理员模式切换)
GET  /register            → 注册页
GET  /home                → Feed流主页(需登录)
GET  /post/create         → 发布作品页
GET  /post/:id            → 帖子详情页
GET  /post/:id/edit       → 编辑帖子页(仅作者)
GET  /user/:id            → 用户主页
GET  /logout              → 退出登录
GET  /admin/              → 管理仪表盘(仅admin)
GET  /admin/review        → 帖子审核
GET  /admin/users         → 用户管理
GET  /admin/logs          → 系统日志
```

### API 路由
```
公开(无需登录):
  POST /api/v1/auth/register
  POST /api/v1/auth/login

公开查询(可选认证):
  GET  /api/v1/feed              # Feed流(分页, latest/hot)
  GET  /api/v1/search            # 搜索帖子(q=关键词)
  GET  /api/v1/posts/:id
  GET  /api/v1/posts/:id/comments
  GET  /api/v1/users/:id/following
  GET  /api/v1/users/:id/followers

需登录:
  GET  /api/v1/auth/me
  POST /api/v1/auth/logout
  PUT  /api/v1/auth/profile
  POST /api/v1/posts             # 创建帖子
  PUT  /api/v1/posts/:id         # 编辑帖子
  DELETE /api/v1/posts/:id       # 删除帖子
  POST /api/v1/posts/:id/like    # 点赞切换
  POST /api/v1/posts/:id/favorite # 收藏切换
  POST /api/v1/posts/:id/comments
  DELETE /api/v1/comments/:cid
  POST /api/v1/users/:id/follow  # 关注切换
  POST /api/v1/upload            # MinIO 图片上传

管理员(需 admin 角色):
  GET    /api/v1/admin/stats
  GET    /api/v1/admin/pending-posts   # 待审核帖子列表
  DELETE /api/v1/admin/posts/:id       # 管理员强制删除帖子
  POST   /api/v1/admin/posts/:id/approve
  POST   /api/v1/admin/posts/:id/reject
  GET    /api/v1/admin/users           # 用户列表
  PUT    /api/v1/admin/users/:id/status
  GET    /api/v1/admin/logs            # 系统日志列表

图片服务:
  GET  /api/v1/images/*objectName  # MinIO 图片代理
```

## 核心业务流程

### 普通用户主链路
注册 → 登录(自动跳转/home) → 浏览Feed(搜索+最新/热门+加载更多) → 发布作品(多图上传MinIO+文字+话题) → 帖子待审核 → 收到点赞/评论/收藏 → 个人主页(作品墙+关注/粉丝) → 关注其他用户

### 管理员链路
登录(admin/admin123) → 自动跳转/admin → 仪表盘统计 → 审核帖子(通过/驳回+理由) → 管理用户(封禁/解封) → 查看系统日志

### 登录入口设计
- 首页 `/` 显示登录页，顶部支持「用户登录/管理员登录」Tab切换
- 用户登录后跳 `/home`，管理员登录后跳 `/admin`
- 已登录用户访问 `/` 自动分流

## 前端设计

- 主题：Cammate 暖黄色调 (#E8A840 主色, #FFF8F0 奶油背景)
- 导航栏：毛玻璃效果，左侧 Logo + Feed/热门/发布，右侧头像下拉菜单
- 汉堡菜单(Offcanvas)：用户信息区 + 用户设置按钮 + GitHub链接 + 退出
- Feed：12列卡片网格，搜索栏，最新/热门Tab，"加载更多"按钮(非传统分页)
- 帖子详情：图片轮播 + 操作栏(点赞/收藏/评论) + 评论表单
- 个人主页：头像 + 统计(作品/关注/粉丝) + 3列作品墙
- 管理后台：统计卡片 + 表格列表

## 关键设计决策

1. **模板架构**: 使用 `{{ define "header" }}`/`{{ define "footer" }}` 组件模式，非继承模式，避免 Go html/template 多页面 define 冲突
2. **Feed 缓存**: 首页第一页最新 Feed 缓存到 Redis (2min TTL)，后续请求命中缓存
3. **权限模型**: 三层中间件 — `OptionalAuth()`(可选), `AuthRequired()`(必须), `AdminRequired()`
4. **点赞/计数**: MySQL 触发器自动维护 `like_count`/`comment_count`，避免 Go 层并发计数问题
5. **用户上下文注入**: `userData(c, gin.H{...})` 辅助函数统一向模板注入 user_id/username/role/avatar
6. **软删除**: 帖子和评论使用 `is_deleted` 标记而非物理删除
7. **帖子审核流**: 新建帖默认 `pending` → 管理员 `approve` 或 `reject` → 仅 `approved` 帖在 Feed 展示

## 预设账号与启动

```bash
cd ShareO
./start.sh          # 一键启动(检测MySQL/Redis/MinIO→编译→启动→打开浏览器)

# 或手动:
make start          # 同上
make run            # 仅启动服务(组件需已运行)

# 管理员: admin / admin123
# 普通用户: 注册后登录
# MinIO Console: http://localhost:9001 (minioadmin / minioadmin)
```

## 测试规范

**强制规定：所有测试操作必须通过 API 调用模拟用户/管理员行为，严禁直接修改数据库。**

### 用户模拟测试流程
```bash
# 1. 注册用户
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"test_user","password":"test123"}'

# 2. 登录获取 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"test_user","password":"test123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")

# 3. 上传图片到 MinIO
curl -X POST http://localhost:8080/api/v1/upload \
  -b "token=$TOKEN" -F "file=@real_photo.jpg"

# 4. 发布帖子
curl -X POST http://localhost:8080/api/v1/posts \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"作品描述","images":["/api/v1/images/posts/..."],"topic_ids":[1]}'

# 5. 社交互动
curl -X POST http://localhost:8080/api/v1/posts/1/like -b "token=$TOKEN"
curl -X POST http://localhost:8080/api/v1/posts/1/favorite -b "token=$TOKEN"
curl -X POST http://localhost:8080/api/v1/posts/1/comments -b "token=$TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"好作品！"}'
curl -X POST http://localhost:8080/api/v1/users/2/follow -b "token=$TOKEN"
```

### 管理员模拟测试流程
```bash
# 管理员登录
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}' \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['data']['token'])")

# 查看仪表盘
curl -b "token=$ADMIN_TOKEN" http://localhost:8080/api/v1/admin/stats

# 审核帖子
curl -X POST http://localhost:8080/api/v1/admin/posts/1/approve -b "token=$ADMIN_TOKEN"
curl -X POST http://localhost:8080/api/v1/admin/posts/2/reject -b "token=$ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"comment":"含违规内容"}'

# 管理用户
curl -X PUT http://localhost:8080/api/v1/admin/users/5/status -b "token=$ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"status":0}'

# 删除帖子
curl -X DELETE http://localhost:8080/api/v1/admin/posts/3 -b "token=$ADMIN_TOKEN"
```

### 禁止事项
- **禁止** 直接执行 `INSERT/UPDATE/DELETE` SQL 语句修改业务数据
- **禁止** 直接操作 MinIO 文件系统
- **禁止** 直接修改 Redis 缓存（测试前清空缓存除外）
- 数据库迁移(`migrations/`)和种子数据(`002_seed.sql`)仅用于初始化，不得用于测试模拟

### 为什么
直接修改数据库绕过了：
- 业务逻辑校验（权限、字段验证、XSS过滤）
- 触发器（like_count自动更新、审计日志）
- 缓存失效机制（Feed Redis缓存清除）
- 真实用户路径的端到端验证

只有通过 API 测试才能真正验证系统行为是否符合预期。

## 仓库

```
git@github.com:2116657276/ShareO.git
```

## 当前状态

- 48 条路由全部实现并验证
- 转帖系统(Repost): 文字转帖 + 图片转帖 + X风格差异化卡片 + PIP画中画
- Feed 骨架屏 + eager/lazy 混合加载 + gzip 压缩
- 管理员控制台: 可折叠面板(审核/用户/日志)，无需跳转
- 已通过完整 E2E 测试: 5 用户 + 12 帖(4原创+4文字转帖+4图片转帖) + 点赞/评论/关注
