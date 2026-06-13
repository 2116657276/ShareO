# ShareO - 拍摄与作品管理系统

## 项目性质
Go + Gin Web 应用，后端 API + 服务端渲染 HTML。数据库 MySQL，缓存 Redis，对象存储 MinIO。

## 核心铁律

### 测试 Agent 规则（不可违反）
1. **禁止阅读任何 .go 源代码文件** — 你是真实用户/管理员，不是开发者
2. **禁止阅读 internal/ 目录下任何文件**
3. **禁止阅读 migrations/ 目录下的 SQL 文件**
4. **禁止直接连接 MySQL/Redis/MinIO** — 所有操作必须通过 HTTP API
5. **禁止执行任何 SQL 语句** — 包括 SELECT/INSERT/UPDATE/DELETE
6. 只能通过 `curl` 调用 `http://localhost:8080` 的 API 完成测试
7. 测试用户命名规则: `test01`, `test02`, `test03`... 密码统一 `256500`
8. 管理员账号: `admin` / `admin123`
9. 可阅读的文件: `FEATURES.md`, `TEST_CHECKLIST.md`, `PROJECT.md`, `feedback.md`
10. 测试结束后必须通过管理员 API 删除测试数据，保留原有帖子

### 修复 Agent 规则
1. 仅修改 `TASK.md` 中明确指定的文件和行
2. 不要新增依赖包（`go.mod` 中的 require）
3. 修改后运行 `go build -o bin/shareo cmd/server/main.go` 确认零错误
4. 不要在代码中硬编码密码、token、密钥
5. 修复完成后更新 `feedback.md` 底部标注修复状态
6. 不要修改 HTML 模板、CSS、SQL 迁移文件（除非 TASK.md 明确指定）

## 环境检查顺序
启动服务前确认：
1. MySQL 运行中: `mysql -u root -p"${MYSQL_PASS}" -e "SELECT 1"`
2. Redis 运行中: `redis-cli PING`
3. MinIO 运行中: `lsof -i :9000`
4. 编译: `go build -o bin/shareo cmd/server/main.go`
5. 启动: `./bin/shareo &`
6. 验证: `curl http://localhost:8080/healthz`

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

### 上传图片
```bash
curl -s -X POST http://localhost:8080/api/v1/upload \
  -b "token=$TOKEN" \
  -F "file=@/path/to/image.jpeg"
```

### 发帖
```bash
curl -s -X POST http://localhost:8080/api/v1/posts \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"content":"帖子内容","images":["/api/v1/images/posts/..."]}'
```

### 转帖
```bash
# 纯文字转帖
curl -s -X POST http://localhost:8080/api/v1/posts/:id/repost \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"转发文字"}'

# 带图片转帖
curl -s -X POST http://localhost:8080/api/v1/posts/:id/repost \
  -b "token=$TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"text":"转发文字","images":["/api/v1/images/posts/..."]}'
```

### 点赞/收藏/评论/关注（均为 Toggle 模式）
```bash
curl -s -X POST http://localhost:8080/api/v1/posts/:id/like -b "token=$TOKEN"
curl -s -X POST http://localhost:8080/api/v1/posts/:id/favorite -b "token=$TOKEN"
curl -s -X POST http://localhost:8080/api/v1/posts/:id/comments -b "token=$TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"评论内容"}'
curl -s -X POST http://localhost:8080/api/v1/users/:id/follow -b "token=$TOKEN"
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
POST /api/v1/auth/register         注册
POST /api/v1/auth/login            登录
GET  /api/v1/auth/me               个人信息
PUT  /api/v1/auth/profile          更新资料（bio, email, avatar_url）
POST /api/v1/upload                上传图片（需登录，支持JPG/PNG/WebP/GIF，≤50MB）
POST /api/v1/posts                 发帖 {content, images[], topic_ids[]}
PUT  /api/v1/posts/:id             编辑帖子
DELETE /api/v1/posts/:id           删除帖子（仅作者）
POST /api/v1/posts/:id/repost      转帖 {text?, images?}
POST /api/v1/posts/:id/like        点赞切换
POST /api/v1/posts/:id/favorite    收藏切换
GET  /api/v1/favorites             收藏列表
POST /api/v1/posts/:id/comments    发评论 {content, parent_id?, reply_to_uid?}
GET  /api/v1/posts/:id/comments    评论列表
DELETE /api/v1/comments/:cid       删评论（仅作者）
POST /api/v1/users/:id/follow      关注切换
GET  /api/v1/users/:id/following   关注列表
GET  /api/v1/users/:id/followers   粉丝列表
GET  /api/v1/feed                  Feed流 (?sort=latest|hot&page=&page_size=)
GET  /api/v1/search                搜索 (?q=关键词&page=&page_size=)
GET  /api/v1/admin/stats           管理员统计
GET  /api/v1/admin/pending-posts   待审核帖
POST /api/v1/admin/posts/:id/approve  通过审核
POST /api/v1/admin/posts/:id/reject   驳回审核
DELETE /api/v1/admin/posts/:id        管理员强制删帖
GET  /api/v1/admin/users              用户列表
PUT  /api/v1/admin/users/:id/status   封禁/解封 {status: 0|1}
GET  /api/v1/admin/logs               系统日志
```

## 测试数据清理
测试完成后必须通过管理员 API 清理：
1. `GET /api/v1/feed?page_size=999` 获取全部帖子
2. 筛选出测试创建的帖子（排除测试前已存在的 PRESERVED_IDS）
3. `DELETE /api/v1/admin/posts/:id` 逐个删除
4. `redis-cli DEL feed:latest:page1` 清缓存
5. 验证 `GET /api/v1/feed` 帖子数恢复测试前水平
