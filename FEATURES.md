# ShareO 功能清单

> 更新时间: 2026-06-13 | 版本: v4 终版

## 项目概述
ShareO — Go + Gin 摄影作品管理与社区分享平台，支持用户发帖社交互动、管理员审核管理。

## 技术栈
Go + Gin + GORM + MySQL 8.0 + Redis + MinIO + Bootstrap 5 + Alpine.js

---

## 已实现功能

### 1. 认证系统
- 用户注册/登录（bcrypt 加密，JWT 72h 过期）
- 登录页 Tab 切换（用户/管理员模式）
- 已登录自动分流（admin→/admin，user→/home）
- 个人信息查询与更新（bio/email/avatar）
- XSS 防护：用户名拒绝 `<>"'&/\\` 字符
- 封禁用户拒绝登录

### 2. Feed 流
- 12 列卡片网格，最新/热门排序
- "加载更多" 按钮（非传统分页）
- Redis 缓存首页最新 Feed（2min TTL，发帖/删帖/审核自动失效）
- gzip 压缩，骨架屏 + 首屏 6 张 eager 预加载 + 悬停放大 1.2x

### 3. 帖子管理
- 创建帖子（多图 + 文字 + 话题标签）
- 编辑帖子（仅作者，修改后重新审核）
- 软删除帖子（仅作者）
- 帖子详情页（图片轮播 + 作者信息 + 操作栏）

### 4. 图片上传
- MinIO 对象存储
- 支持 JPG/PNG/WebP/GIF，单文件 ≤50MB
- 魔数（magic number）检测真实类型，防 Content-Type 伪造
- 服务端 + 客户端双重校验
- 图片代理（HEAD/GET，24h 浏览器缓存）
- 拖拽上传

### 5. 社交互动
- 点赞/收藏/关注（Toggle 模式，防重复）
- 应用层 + 触发器双层计数保护（like_count/favorite_count/comment_count）
- 评论（多级回复 parent_id + reply_to_uid）
- 评论嵌套 children
- 关注/粉丝列表，收藏列表
- 边界校验完善（自关注/不存在帖/不存在评论）
- 错误码分级：业务错误(400) vs 资源不存在(404)

### 6. 转帖系统
- 纯文字转帖 / 带图片转帖
- Feed 差异化卡片：转帖文字 + 嵌入原帖 + PIP 画中画（双图时）
- 详情页 "查看原文" 跳转
- share_count 自动递增

### 7. 搜索
- FULLTEXT 全文搜索（ngram 分词，中文友好）
- 启动时检测 FULLTEXT 索引可用性，缓存结果
- 无 FULLTEXT 索引时自动降级 LIKE
- 分页支持

### 8. 用户主页
- 头像 + 简介 + 统计（作品数/关注/粉丝）
- 3 列作品墙 + 关注按钮
- isOwnProfile / isFollowing 状态

### 9. 管理员功能
- 仪表盘统计（用户/帖子/待审/点赞/评论/封禁）
- 帖子审核（通过/驳回 + 理由）
- 用户管理（封禁/解封）
- 系统日志查看
- 强制删帖
- 可折叠面板，不跳转

### 10. 安全与权限
- 4 层中间件：OptionalAuth / AuthRequired / AdminRequired / RateLimit
- Redis 滑动窗口 API 限流（登录 10次/分, 发帖 30次/分, 上传 20次/分, Lua 原子操作）
- JWT Cookie SameSite=Lax + HttpOnly（防 XSS/CSRF）
- JWT 密钥 nil 保护（GenerateToken/ParseToken 双重检查）
- 上传魔数校验（Magic Number + WebP 手动检测 + io.MultiReader）
- 错误信息脱敏，不暴露内部实现
- 密钥环境变量覆盖（SHAREO_* 5 项），config.yaml 不提交 Git
- 启动时配置 Validate() + Graceful Shutdown (10s 超时)
- 业务错误码体系（0+1001~1006），与 HTTP 状态码解耦

### 11. 前端
- Cammate 暖黄色调主题（#E8A840）
- 毛玻璃导航栏 + 汉堡菜单 Offcanvas
- 全局 NoCache + 响应式布局（768px / 480px 断点）

### 12. 通知系统 🆕
- 5 类通知：点赞/评论/关注/转帖/审核结果
- API：列表(GET) + 标记已读(PUT) + 全部已读(PUT) + 未读数(GET)
- 导航栏铃铛图标 + 红色未读徽章
- 通知页面（Alpine.js 渲染，时间友好显示，加载更多分页）
- 5 个触发点自动集成（SocialService/PostService/AdminService）

### 13. 话题系统 🆕
- 正文 `#话题名` 正则自动提取（支持中/英/数字/下划线）
- 话题不存在时自动创建 + 自动关联帖子
- 编辑帖子时重新解析并更新话题关联
- 话题聚合页 `GET /topic/:id_or_name`（作品墙 + 作品数统计）
- 帖子正文 #话题 渲染为可点击链接

### 14. 图片缩略图 🆕
- 上传时自动生成 3 档尺寸：thumb(300x300) / medium(1200x1200) / original
- Feed 卡片使用 thumb，详情页使用 medium
- 正方裁剪（中心）+ Lanczos 重采样 + JPEG Q85 编码
- GIF 跳过 resize 保留动画

### 15. 数据库
- 11 张业务表（含 notifications）+ 触发器 + 视图 + 存储过程
- FULLTEXT ngram 索引（posts.content）
- B+Tree 索引（时间/热度/外键）
- 软删除 + 审计日志触发器

---

## 路由总览（54 条）

### 页面路由（16 条）
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | / | public | 登录页（已登录分流） |
| GET | /login | public | 登录页 |
| GET | /register | public | 注册页 |
| GET | /home | user | Feed 主页 |
| GET | /post/create | user | 发布作品 |
| GET | /post/:id | user | 帖子详情 |
| GET | /post/:id/edit | user | 编辑帖子 |
| GET | /user/:id | user | 用户主页 |
| GET | /settings | user | 账号设置 |
| GET | /notifications | user | 通知列表 🆕 |
| GET | /topic/:id | user | 话题聚合页 🆕 |
| GET | /logout | user | 退出 |
| GET | /admin/ | admin | 仪表盘 |
| GET | /admin/review | admin | 审核页 |
| GET | /admin/users | admin | 用户管理 |
| GET | /admin/logs | admin | 日志 |

### API 路由（38 条）
| 方法 | 路径 | 权限 | 限流 |
|------|------|------|------|
| POST | /api/v1/auth/register | public | 10/min |
| POST | /api/v1/auth/login | public | 10/min |
| GET | /api/v1/auth/me | user | — |
| POST | /api/v1/auth/logout | user | — |
| PUT | /api/v1/auth/profile | user | — |
| GET | /api/v1/feed | public | — |
| GET | /api/v1/search | public | — |
| GET | /api/v1/posts/:id | public | — |
| POST | /api/v1/posts | user | 30/min |
| PUT | /api/v1/posts/:id | user | — |
| DELETE | /api/v1/posts/:id | user | — |
| POST | /api/v1/posts/:id/repost | user | — |
| POST | /api/v1/posts/:id/like | user | — |
| POST | /api/v1/posts/:id/favorite | user | — |
| GET | /api/v1/favorites | user | — |
| POST | /api/v1/posts/:id/comments | user | — |
| GET | /api/v1/posts/:id/comments | public | — |
| DELETE | /api/v1/comments/:cid | user | — |
| POST | /api/v1/users/:id/follow | user | — |
| GET | /api/v1/users/:id/following | public | — |
| GET | /api/v1/users/:id/followers | public | — |
| POST | /api/v1/upload | user | 20/min |
| GET | /api/v1/images/* | public | — |
| GET | /api/v1/notifications | user | — 🆕 |
| PUT | /api/v1/notifications/:id/read | user | — 🆕 |
| PUT | /api/v1/notifications/read-all | user | — 🆕 |
| GET | /api/v1/notifications/unread-count | user | — 🆕 |
| GET | /api/v1/admin/stats | admin | — |
| GET | /api/v1/admin/pending-posts | admin | — |
| DELETE | /api/v1/admin/posts/:id | admin | — |
| POST | /api/v1/admin/posts/:id/approve | admin | — |
| POST | /api/v1/admin/posts/:id/reject | admin | — |
| GET | /api/v1/admin/users | admin | — |
| PUT | /api/v1/admin/users/:id/status | admin | — |
| GET | /api/v1/admin/logs | admin | — |
