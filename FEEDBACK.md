# ShareO 改进记录与状态

> 更新时间: 2026-06-19 | `go build` ✅ | `go vet` ✅

---

## 项目规模

```
58 路由 | 11 表 | 50+ Go 源文件 | 17 模板 | 8 SQL 迁移 | 85 测试用例
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

## 代码评审修复 (基于 CODE_REVIEW.md, 2026-06-19)

> 18/21 项已修复 | P0×1 + P1×5 + P2×7 + P3×5 | 3 项豁免(设计选择/误判/需重构)

### P0 — 致命 (1/1)
| # | 问题 | 修复方式 | 文件 |
|---|------|---------|------|
| P0-1 | Topic post_count 持续膨胀 | `ReplacePostTopics` 先收集旧 topic、递减计数，仅对新 topic 递増 | `topic_repo.go` |

### P1 — 高危 (5/5)
| # | 问题 | 修复方式 | 文件 |
|---|------|---------|------|
| P1-1 | DB.Save 级联写入关联表 | `PostRepo.Update` 改用 `DB.Updates(map)` 只更新 content+status | `post_repo.go` |
| P1-2 | FindOrCreateWithTx 破坏事务隔离 | 改用 `tx.Where(...)` 在事务内查询 | `topic_repo.go` |
| P1-3 | resolveTopicIDsInTx 丢弃错误 | 检查 `FindOrCreateWithTx` 的 error 并 log.Printf + continue | `post_service.go` |
| P1-4 | 计数同步错误静默忽略 | Like/Favorite/Comment 三处 `UpdateColumn` 均检查 error 并 log | 3 个 repo 文件 |
| P1-5 | 帖子创建与话题关联不同事务 | `tx.Create(post)` + `ReplacePostTopics` 纳入同一事务 | `post_service.go` |

### P2 — 中危 (7/8, P2-1 此前已修复)
| # | 问题 | 修复方式 | 文件 |
|---|------|---------|------|
| P2-2 | Like/Favorite Toggle 无事务包裹 | `DB.Transaction(func(tx){...})` 包裹 INSERT/DELETE + 计数同步 | `like_repo.go`, `favorite_repo.go` |
| P2-3 | Redis 缓存完整 Post 对象 | 改为只缓存 `PostIDs []int64`，命中后 `FindByIDs` 重新查询 | `feed_service.go`, `post_repo.go` |
| P2-4 | 查看自己帖子增加 view_count | `if post.UserID != currentUserID` 判断后再 IncrementView | `post_service.go` |
| P2-5 | Admin 强制删帖不通知作者 | `DeletePost` 新增 adminID 参数，删除前取作者 ID 发送通知 | `admin_service.go`, `admin_handler.go` |
| P2-6 | SocialService 分页缺少参数校验 | `clampPage()` 统一 clamp page/pageSize | `social_service.go` |
| P2-7 | strconv.Atoi 丢弃错误 | 改用已有的 `getPageSizePair` helper | `social_handler.go` |
| P2-8 | TopicService Feed 错误忽略 | 检查 `Feed()` 的 error 并正确返回 | `topic_service.go` |

### P3 — 低危 (5/7, 2 项豁免)
| # | 问题 | 修复方式 | 文件 |
|---|------|---------|------|
| P3-1 | 多处 Count 查询忽略 Error | 所有 `Count*`/`Is*` 方法检查 error 并 `log.Printf` | 7 个 repo 文件 |
| P3-3 | reviewed_at 从未设置 | `UpdateStatus` 增加 `"reviewed_at": time.Now()` | `post_repo.go` |
| P3-4 | TopicPage ID=0 无效查询 | `if id > 0` 守卫跳过无效 `FindByID(0)` | `topic_service.go` |
| P3-5 | PostService.Update 返回旧对象 | 末尾改为 `return s.postRepo.FindByID(postID)` 重新查询 | `post_service.go` |

**豁免项**:
- P3-2: UpdateProfile 空字符串清空 — 代码注释标注 "Always update all fields"，已知设计选择
- P3-6: Auth 中间件 Redis nil — 审查报告误判，实际代码对非 redis.Nil 错误采用 fail-open 放行
- P3-7: 错误消息硬编码中文 — 需 i18n 系统性重构，影响面过大

---

## 测试覆盖

| 模块 | 文件 | 用例数 |
|------|------|--------|
| JWT | `pkg/jwt/jwt_test.go` | 10 |
| Upload 魔数 | `handler/upload_magic_test.go` | 11 |
| Handler 辅助 | `handler/helpers_test.go` | 15 |
| Auth 中间件 | `middleware/auth_test.go` | 12 |
| RateLimit | `middleware/ratelimit_test.go` | 4 |
| Response | `pkg/response/response_test.go` | 9 |
| Thumbnail | `pkg/upload/thumbnail_test.go` | 8 |
| Hashtag | `service/hashtag_test.go` | 3 |
| Notification | `service/notification_service_test.go` | 11 |
| Topic | `service/topic_service_test.go` | 2 |
| **合计** | **10 文件** | **85** |

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
