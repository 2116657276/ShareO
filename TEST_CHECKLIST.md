# ShareO 自动化测试清单

> 本清单供 AI Agent 执行端到端测试，所有操作必须通过 API 模拟用户/管理员行为，**严禁直接修改数据库**。

## 环境信息
- **Base URL**: `http://localhost:8080`
- **测试用户命名**: `test01` ~ `test10`（按需递增）
- **测试用户密码**: `256500`
- **管理员**: `admin` / `admin123`
- **图片**: 使用 `resources/static/pictures/` 下 JPEG 文件

## 测试流程总览

```
Phase 1: 用户测试（test01-test05 注册→发帖→转帖→社交）
Phase 2: 管理员审批+管理
Phase 3: 全功能交叉验证
Phase 4: 数据清理（管理员API删除测试数据，保留原有帖子）
```

---

## Phase 1: 用户端测试

### 1.1 用户注册
- [ ] `POST /api/v1/auth/register` test01 (pw=256500, email=test01@shareo.com)
- [ ] `POST /api/v1/auth/register` test02 (pw=256500, email=test02@shareo.com)
- [ ] `POST /api/v1/auth/register` test03 (pw=256500, email=test03@shareo.com)
- [ ] `POST /api/v1/auth/register` test04 (pw=256500, email=test04@shareo.com)
- [ ] `POST /api/v1/auth/register` test05 (pw=256500, email=test05@shareo.com)
- 预期: 全部返回 `code=0`, 含 token 和 user 对象, role=user

### 1.2 登录验证
- [ ] `POST /api/v1/auth/login` test01/256500 → 返回 token
- [ ] `POST /api/v1/auth/login` test01/wrong → 400 "用户名或密码错误"
- [ ] `POST /api/v1/auth/login` nonexist/xxx → 400
- [ ] XSS用户名注册 `username=<script>alert(1)</script>` → 400 "用户名包含非法字符"

### 1.3 个人信息
- [ ] `GET /api/v1/auth/me` (带test01 token) → 返回 id/username/email/role/bio
- [ ] `PUT /api/v1/auth/profile` 更新 test01 的 bio="摄影爱好者，记录生活美好" + email="test01_new@shareo.com" → 200
- [ ] `GET /api/v1/auth/me` 验证 bio 和 email 已更新
- [ ] 无 token 访问 `/auth/me` → 401 "请先登录"
- [ ] 错误 token 访问 → 401

### 1.4 图片上传（由 test01 执行）
- [ ] `POST /api/v1/upload` 上传任意一张 resources/static/pictures/ 下的 JPEG → 返回 MinIO URL
- [ ] `HEAD <返回的URL>` → 200, Content-Type: image/jpeg, Content-Length > 0
- [ ] `GET <返回的URL>` → 200, 返回图片二进制数据
- [ ] 上传非图片文件 → 400 "不支持的图片格式"
- [ ] 不选文件 → 400

### 1.5 发帖
- [ ] test01 发帖: content="晨曦中的老街", 使用上传的1张图片 → 返回 post_id, status=pending
- [ ] test02 发帖: content="雨后山间云雾", 使用上传的1张图片 → status=pending
- [ ] test03 发帖: content="城市夜景霓虹", 使用上传的1张图片 → status=pending
- [ ] test04 发帖: content="海边的日落", 使用上传的1张图片 → status=pending
- [ ] 空内容发帖(无图) → 400 "至少需要上传一张图片"

### 1.6 帖子详情
- [ ] `GET /api/v1/posts/:id` → 含 user/images/is_liked/is_favorited 字段
- [ ] 查看不存在的帖子 → 404

---

## Phase 2: 管理员测试

### 2.1 管理员登录
- [ ] `POST /api/v1/auth/login` admin/admin123 → role=admin
- [ ] `GET /api/v1/admin/stats` → 含 total_users/total_posts/pending_posts 等
- [ ] `GET /api/v1/admin/pending-posts` → 返回 test01-test04 的 4 篇待审核帖

### 2.2 帖子审核
- [ ] `POST /api/v1/admin/posts/:id/approve` 逐个通过 test01-test04 的帖子
- [ ] 审核后 Feed 可见性: `GET /api/v1/feed` 确认 4 篇 status=approved 可见

### 2.3 管理员操作
- [ ] `GET /api/v1/admin/users` → 查看用户列表
- [ ] `GET /api/v1/admin/logs` → 查看系统日志
- [ ] 普通用户访问 admin API → 403 "需要管理员权限"

---

## Phase 3: 社交功能交叉测试

### 3.1 转帖
- [ ] test02 纯文字转帖 test01 的帖子: `POST /api/v1/posts/:id/repost` {"text":"这组晨光太棒了"}
- [ ] test03 纯文字转帖 test02 的帖子
- [ ] test04 图片转帖 test03 的帖子: 先上传一张新图，再调用 repost
- [ ] test05 图片转帖 test04 的帖子
- [ ] 验证转帖: `is_repost=1`, `repost_of_id` 指向原帖, `repost_of` 含原帖完整数据
- [ ] 验证原帖 `share_count` 增加

### 3.2 点赞
- [ ] test03 点赞所有4篇原创帖 `POST /api/v1/posts/:id/like` → liked=true
- [ ] test03 重复点赞同一帖 → liked=false（取消）
- [ ] test04 点赞 test01、test02 的帖和转帖
- [ ] test05 点赞所有图片转帖
- [ ] test01 点赞 test02 的帖
- [ ] 点赞不存在帖(pid=99999) → 400 "帖子不存在"

### 3.3 收藏
- [ ] test01 收藏 test02 的帖子 → favorited=true
- [ ] test02 收藏 test03 的帖子
- [ ] test03 收藏 test01 的帖子
- [ ] `GET /api/v1/favorites` (test01) → 含已收藏帖
- [ ] 重复收藏同一帖 → favorited=false（取消）

### 3.4 评论
- [ ] test02 评论 test01 的帖: content="构图太棒了"
- [ ] test04 评论 test01 的帖: content="这光线绝了"
- [ ] test01 评论 test02 的帖: content="人文气息浓厚"
- [ ] `GET /api/v1/posts/:id/comments` → 含评论列表+嵌套children
- [ ] 评论不存在帖 → 400
- [ ] 删除自己的评论 → 200
- [ ] 删除不存在的评论 → 400 "评论不存在或无权删除"

### 3.5 关注
- [ ] test01 关注 test02 → following=true
- [ ] test02 关注 test01（互关）
- [ ] test03 关注 test01
- [ ] test04 关注 test01、test02
- [ ] test05 关注 test03、test04
- [ ] 关注自己 → 400 "cannot follow yourself"
- [ ] `GET /api/v1/users/:id/following` → 关注列表
- [ ] `GET /api/v1/users/:id/followers` → 粉丝列表

### 3.6 搜索
- [ ] `GET /api/v1/search?q=晨光` → 返回匹配帖
- [ ] `GET /api/v1/search?q=不存在关键词` → total=0
- [ ] `GET /api/v1/search?q=` → 400 "搜索关键词不能为空"

### 3.7 Feed验证
- [ ] `GET /api/v1/feed?sort=latest` → 时间降序
- [ ] `GET /api/v1/feed?sort=hot` → 热度降序
- [ ] `GET /api/v1/feed?page=2&page_size=3` → 分页正常
- [ ] `GET /api/v1/feed` (无参数) → 200（已修复除零bug）

### 3.8 权限边界
- [ ] 普通用户编辑他人帖 → 400 "无权编辑"
- [ ] 普通用户删除他人帖 → 400 "无权删除"
- [ ] 普通用户访问 `/admin/` → 403
- [ ] 未登录访问 `/home` → 401

---

## Phase 4: 数据清理

> **关键：仅删除本次测试过程中新增的帖子，保留测试前已存在的帖子**

### 4.1 记录测试前状态
- [ ] `GET /api/v1/feed` 记录当前 Feed 中的帖子ID列表 → 记作 `PRESERVED_IDS`
- [ ] `GET /api/v1/admin/stats` 记录当前统计

### 4.2 删除测试数据（管理员API）
- [ ] 管理员登录获取 token
- [ ] `GET /api/v1/feed?page_size=999` 获取全部帖子
- [ ] 筛选出 `id NOT IN PRESERVED_IDS` 的帖子
- [ ] 对每个测试帖 `DELETE /api/v1/admin/posts/:id` → code=0
- [ ] 对每个测试用户所发的评论 `DELETE /api/v1/comments/:cid` → code=0

### 4.3 验证清理结果
- [ ] `GET /api/v1/feed` 确认仅保留 PRESERVED_IDS 的帖子
- [ ] `GET /api/v1/admin/stats` 确认帖子数正确
- [ ] `GET /api/v1/admin/users` 确认用户数（测试用户无需删除）

### 4.4 清理缓存
- [ ] `redis-cli DEL feed:latest:page1` 或 `redis-cli FLUSHALL`

---

## 测试判定标准

每个测试项的判定：
- ✅ **通过**: HTTP状态码和响应 body 符合预期
- ❌ **失败**: 状态码不符、响应缺失关键字段、业务逻辑错误
- ⚠️ **需调查**: 状态码正确但行为未达预期（记录详细错误信息）

## 重要提醒
1. **严禁直接执行 INSERT/UPDATE/DELETE SQL**
2. **严禁手动修改 Redis 缓存**（清理除外）
3. **严禁直接操作 MinIO 文件系统**
4. 所有操作必须通过 `curl` 调用 API 完成
5. 每次 API 调用需携带正确的 JWT token
6. 测试用户从 test01 开始，密码统一为 256500
7. Phase 4 清理时**仅删除测试创建的帖子**，保留测试前已存在的帖子
