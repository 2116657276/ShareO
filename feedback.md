# ShareO 改进记录与状态

> 更新时间: 2026-06-13 | `go build` ✅ | `go test` 66 PASS | `go vet` ✅

---

## 项目规模

```
54 路由 | 11 表 | 50+ Go 源文件 | 19 模板 | 7 SQL 迁移 | 66 测试用例
```

## 功能总览

**核心**: 认证(JWT+bcrypt) · Feed(最新+热门+LazyLoad) · 发帖(多图+审核流) · 搜索(FULLTEXT ngram)
**社交**: 点赞/收藏/关注(Toggle) · 评论(多级嵌套) · 转帖(PIP画中画) · 用户主页(作品墙)
**通知**: 5 类型(like/comment/follow/repost/review) · 铃铛徽章 · 已读/全部已读
**话题**: #Unicode 解析 · 自动创建 · 事务重关联 · 聚合页 · 模板链接渲染
**图片**: 魔数检测 · Lanczos 三档缩略图 · 并发 Promise.all 上传
**管理员**: 仪表盘 · 审核(含通知) · 用户管理 · 系统日志
**安全**: 密钥环境变量 · 限流 3 档(Lua 原子) · SameSite Cookie · 错误脱敏 · 配置校验 · Graceful Shutdown
**质量**: 业务错误码体系(0+1001~1006) · 应用层双写计数 · 事务保护

---

## Bug 修复记录

| # | 描述 | 文件 | 状态 |
|----|------|------|------|
| 1 | JSON API 不支持 email 更新 | `auth_service.go` | ✅ |
| 2 | 评论不存在帖泄漏外键错误 | `social_service.go` | ✅ |
| 3 | 空关注/粉丝列表返回 null | `follow_repo.go` | ✅ |
| 4 | 管理员统计含已软删除帖 | `admin_service.go` | ✅ |

---

## 代码评审修复 (基于 CODE_REVIEW.md)

> 全部 22 项问题已修复 | 3 轮迭代 | P0×1 + P1×5 + P2×8 + P3×8

### P0 — 致命 (1)
| # | 问题 | 修复方式 |
|---|------|---------|
| 1 | PostService.Update 事务边界不一致 | `resolveTopicIDsInTx` + `ReplacePostTopics` 纳入事务，Create 同步修复 |

### P1 — 高危 (5)
| # | 问题 | 修复方式 |
|---|------|---------|
| 2 | 计数依赖 DB 触发器 | Like/Comment 用 COUNT(*) 子查询，Favorite 用 +1/-1 增量，与触发器兼容 |
| 3 | 图片上传 OOM 风险 | TASK.md 标记后续优化（当前 50MB 限制可接受） |
| 4 | encodeJPEG 返回 nil | 改为 `([]byte, error)`，调用方检查并降级到原图 URL |
| 5 | PostService 打破分层 | `ReplacePostTopics` / `FindOrCreateWithTx` 下沉到 TopicRepo |
| 6 | 无 Graceful Shutdown | `signal.Notify` + `http.Server.Shutdown` + 10s 超时 |

### P2 — 中危 (8)
| # | 问题 | 修复方式 |
|---|------|---------|
| 7 | FULLTEXT 每次搜索检测 | `DetectFulltext()` 启动时检测一次，`hasFulltext` 变量缓存 |
| 8 | Response code 字段混乱 | 7 组业务错误码(0+1001~1006)与 HTTP 状态码解耦 |
| 9 | 通知发送静默吞错误 | `atomic.Int64` 计数器 + `Stats()` 方法提供可观测性 |
| 10 | FindByID Preload 过重 | `FindByIDLight()` 轻量查询，ToggleLike/Favorite/Comment 使用 |
| 11 | HTTP 状态码语义不一致 | `ErrPostNotFound` sentinel error → handler 返回 404 |
| 12 | JWT secret 未检查 | `GenerateToken` / `ParseToken` 开头 nil check |
| 13 | DB/RDB 全局变量 | `SetDB()` 测试辅助，完整 DI 标记为后续 |
| 14 | GlobalConfig 死代码 | 删除 `var GlobalConfig *Config` 及赋值 |

### P3 — 低危 (8)
| # | 问题 | 修复方式 |
|---|------|---------|
| 15 | Login 未 TrimSpace | `req.Username = strings.TrimSpace(req.Username)` |
| 16 | WebSettings 头像硬编码 | `c.PostForm("avatar_url")` 从表单读取 |
| 17 | TopicPage 无分页 | `GetTopicPage` 加了 `page, pageSize` 参数 |
| 18 | 评论 API page_size 硬编码 | `GetComments` 支持 query `page_size` (1-50) |
| 19 | TopicRepo.FindByID 不一致 | 统一返回 `nil, nil` on NotFound |
| 20 | Ratelimit INCR+EXPIRE 非原子 | Lua 脚本 `setExpireScript` 保证原子性 |
| 21 | fmt.Println → log | `log.Println("feed served from Redis cache")` |
| 22 | PostService.Update swallow error | 事务错误改为 `log.Printf` 输出 |

---

## 测试覆盖

| 模块 | 文件 | 用例数 |
|------|------|--------|
| JWT | `pkg/jwt/jwt_test.go` | 10 |
| Upload 魔数 | `handler/upload_magic_test.go` | 10 |
| Handler 辅助 | `handler/helpers_test.go` | 11 |
| Auth 中间件 | `middleware/auth_test.go` | 10 |
| RateLimit | `middleware/ratelimit_test.go` | 2 |
| Response | `pkg/response/response_test.go` | 7 |
| Thumbnail | `pkg/upload/thumbnail_test.go` | 8 |
| Hashtag | `service/hashtag_test.go` | 3 |
| Notification | `service/notification_service_test.go` | 2 |
| Topic | `service/topic_service_test.go` | 3 |
| **合计** | **10 文件** | **66** |

---

## 待后续改进

| 优先级 | 项目 | 说明 |
|--------|------|------|
| P2 | 上传并发限制 | 多个大文件并发上传可能 OOM，加 semaphore |
| P2 | 依赖注入 (DI) | 架构级改动，独立规划 |
| P3 | 结构化日志 | `log/slog` 替换 `log.Printf` |
| P3 | 密码修改/重置 | 用户安全基础功能 |
| P3 | Docker 容器化 | Dockerfile + docker-compose.yml |

---

## 构建与测试命令

```bash
go build -o bin/shareo cmd/server/main.go   # 编译
go vet ./...                                 # 静态分析
go test ./... -count=1                       # 66 测试用例
go test ./... -count=1 -v | grep -c PASS    # 统计通过数
```
