# ShareO 功能需求与实现清单

## 项目概述
ShareO 是基于 Go 的拍摄与作品管理 Web 系统，支持用户发帖分享、社交互动（类小红书/朋友圈），管理员审核管理。

## 技术栈
Go + Gin + GORM + MySQL + Redis + MinIO + Bootstrap 5 + Alpine.js

## 已实现功能

### 1. 认证系统
- [x] 用户注册（用户名/密码/邮箱，bcrypt加密）
- [x] 用户登录（JWT token签发，72h过期）
- [x] 登录页支持「用户登录/管理员登录」Tab切换
- [x] 已登录用户访问 `/` 自动分流（admin→/admin，user→/home）
- [x] 获取个人信息 `GET /api/v1/auth/me`
- [x] 更新个人资料（bio/email/avatar）`PUT /api/v1/auth/profile`
- [x] 退出登录
- [x] XSS防护：用户名拒绝 `<>"'\&/` 字符
- [x] 封禁用户拒绝登录

### 2. Feed流
- [x] 首页Feed（12列卡片网格，需登录）
- [x] 最新排序（created_at DESC）
- [x] 热门排序（like_count DESC）
- [x] 加载更多按钮（非传统分页）
- [x] Redis缓存首页最新Feed（2min TTL，发帖/删帖/审核自动失效）
- [x] gzip压缩（HTML/CSS/JSON ~67%压缩率）
- [x] 骨架屏加载占位 + 首屏6张eager预加载
- [x] 悬停图片放大1.2倍 + 卡片浮起阴影

### 3. 帖子管理
- [x] 创建帖子（多图上传+文字+话题标签）
- [x] 编辑帖子（仅作者，修改后重新进入审核）
- [x] 删除帖子（软删除，仅作者）
- [x] 帖子详情页（图片轮播+作者信息+操作栏）

### 4. 图片上传
- [x] MinIO对象存储
- [x] 支持 JPG/PNG/WebP/GIF，单文件最大50MB
- [x] 服务端+客户端双重大小校验
- [x] 上传后验证对象完整性
- [x] 图片代理服务（HEAD+GET，含Content-Length）
- [x] 多图上传（一帖多图，首张为封面）
- [x] 拖拽上传

### 5. 社交互动
- [x] 点赞切换（Toggle，防重复）
- [x] 收藏切换（Toggle，防重复）
- [x] 评论（支持多级回复 parent_id）
- [x] 评论列表（含嵌套children）
- [x] 关注/取关切换
- [x] 关注列表/粉丝列表
- [x] 收藏列表 `GET /api/v1/favorites`
- [x] 点赞不存在帖返回 "帖子不存在"（已修复外键错误暴露）
- [x] 删除不存在评论返回 "评论不存在或无权删除"（已修复空删除）
- [x] 关注自己返回 "cannot follow yourself"

### 6. 转帖系统（Repost）
- [x] 转帖API `POST /api/v1/posts/:id/repost`
- [x] 支持纯文字转帖
- [x] 支持带新图片转帖
- [x] Feed转帖卡片差异化渲染（X风格）：
  - 顶部显示 "用户名 转帖了"
  - 主标题：转帖文字（加粗）
  - 嵌入原帖卡片（浅黄底色+原作者+内容）
  - 双图时画中画：大图(转帖)全幅 + 小图(原帖)叠右下角毛玻璃边框
- [x] 转帖详情页显示 "查看原文" 跳转链接
- [x] 原帖 share_count 自动递增

### 7. 搜索
- [x] 搜索API `GET /api/v1/search?q=关键词`
- [x] MySQL LIKE 子串匹配（content字段）
- [x] 支持分页

### 8. 用户主页
- [x] 头像 + 用户名 + 简介
- [x] 统计信息（作品数/关注数/粉丝数）
- [x] 3列作品墙
- [x] 关注/取关按钮

### 9. 管理员功能
- [x] 仪表盘（统计卡片：用户/帖子/待审/点赞/评论/封禁）
- [x] 可折叠面板（审核/用户管理/系统日志），点击展开不跳转
- [x] 帖子审核（通过/驳回+理由）
- [x] 用户管理（封禁/解封）
- [x] 系统日志查看
- [x] 管理员删除帖子 `DELETE /api/v1/admin/posts/:id`
- [x] 管理员浏览Feed（导航栏"浏览"按钮，不切换账号）

### 10. 权限模型（3层中间件）
- [x] OptionalAuth：登录可选，已登录则注入上下文
- [x] AuthRequired：必须登录
- [x] AdminRequired：必须管理员角色

### 11. 页面优化
- [x] 全局NoCache（HTTP头+meta标签）
- [x] 导航栏等间距分布
- [x] 汉堡菜单(Offcanvas)含设置入口+GitHub链接
- [x] 账号设置页（编辑邮箱/简介）
- [x] Cammate暖黄色调主题（#E8A840主色/#FFF8F0奶油背景）

### 12. 数据库
- [x] 10张业务表 + 触发器自动维护计数 + 视图 + 存储过程
- [x] 3NF规范化
- [x] 软删除（posts.is_deleted, comments.is_deleted）
- [x] B+Tree索引（时间排序/热度排序/外键）

## 路由总览（48条）

### 页面路由
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | / | public | 首页（登录页，已登录分流） |
| GET | /login | public | 登录页 |
| GET | /register | public | 注册页 |
| GET | /home | user | Feed主页 |
| GET | /post/create | user | 发布作品 |
| GET | /post/:id | user | 帖子详情 |
| GET | /post/:id/edit | user | 编辑帖子 |
| GET | /user/:id | user | 用户主页 |
| GET | /settings | user | 账号设置 |
| GET | /logout | user | 退出 |
| GET | /admin/ | admin | 管理仪表盘 |
| GET | /admin/review | admin | 审核页 |
| GET | /admin/users | admin | 用户管理页 |
| GET | /admin/logs | admin | 日志页 |

### API路由
| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | /api/v1/auth/register | public | 注册 |
| POST | /api/v1/auth/login | public | 登录 |
| GET | /api/v1/auth/me | user | 个人信息 |
| POST | /api/v1/auth/logout | user | 退出 |
| PUT | /api/v1/auth/profile | user | 更新资料 |
| GET | /api/v1/feed | public | Feed流 |
| GET | /api/v1/search | public | 搜索 |
| GET | /api/v1/posts/:id | public | 帖子详情 |
| POST | /api/v1/posts | user | 创建帖子 |
| PUT | /api/v1/posts/:id | user | 编辑帖子 |
| DELETE | /api/v1/posts/:id | user | 删除帖子 |
| POST | /api/v1/posts/:id/repost | user | 转帖 |
| POST | /api/v1/posts/:id/like | user | 点赞切换 |
| POST | /api/v1/posts/:id/favorite | user | 收藏切换 |
| GET | /api/v1/favorites | user | 收藏列表 |
| POST | /api/v1/posts/:id/comments | user | 发评论 |
| GET | /api/v1/posts/:id/comments | public | 评论列表 |
| DELETE | /api/v1/comments/:cid | user | 删评论 |
| POST | /api/v1/users/:id/follow | user | 关注切换 |
| GET | /api/v1/users/:id/following | public | 关注列表 |
| GET | /api/v1/users/:id/followers | public | 粉丝列表 |
| POST | /api/v1/upload | user | 上传图片 |
| GET | /api/v1/images/* | public | 图片代理 |
| GET | /api/v1/admin/stats | admin | 仪表盘统计 |
| GET | /api/v1/admin/pending-posts | admin | 待审核列表 |
| DELETE | /api/v1/admin/posts/:id | admin | 强制删帖 |
| POST | /api/v1/admin/posts/:id/approve | admin | 通过审核 |
| POST | /api/v1/admin/posts/:id/reject | admin | 驳回审核 |
| GET | /api/v1/admin/users | admin | 用户列表 |
| PUT | /api/v1/admin/users/:id/status | admin | 封禁/解封 |
| GET | /api/v1/admin/logs | admin | 系统日志 |

## 预设账号
- 管理员：admin / admin123
- 普通用户：需自行注册
