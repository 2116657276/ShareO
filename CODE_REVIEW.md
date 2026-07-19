# ShareO 代码审查报告

> 审查时间: 2026-06-19 | 审查人: 高级开发工程师视角
> 
> 范围: handler(9), service(10), repository(12), middleware(3), pkg(3), templates(17)
> 
> 关注重点: 功能正确性、数据一致性、安全风险、错误处理、代码健壮性

---

## 修复状态 (2026-06-19)

| 等级 | 编号 | 问题 | 状态 |
|------|------|------|------|
| P0 | P0-1 | Topic post_count 持续膨胀 | ✅ 已修复 |
| P1 | P1-1 | DB.Save 级联写入 | ✅ 已修复 |
| P1 | P1-2 | FindOrCreateWithTx 破坏事务隔离 | ✅ 已修复 |
| P1 | P1-3 | resolveTopicIDsInTx 丢弃错误 | ✅ 已修复 |
| P1 | P1-4 | 计数同步错误静默忽略 | ✅ 已修复 |
| P1 | P1-5 | 帖子创建与话题关联不在同一事务 | ✅ 已修复 |
| P2 | P2-1 | 转帖选取照片无响应 | ✅ 已修复(此前) |
| P2 | P2-2 | Like/Favorite Toggle 无事务包裹 | ✅ 已修复 |
| P2 | P2-3 | Redis 缓存完整 Post 对象 | ✅ 已修复 |
| P2 | P2-4 | 查看自己帖子增加 view_count | ✅ 已修复 |
| P2 | P2-5 | Admin 强制删帖不通知作者 | ✅ 已修复 |
| P2 | P2-6 | SocialService 分页缺少参数校验 | ✅ 已修复 |
| P2 | P2-7 | strconv.Atoi 丢弃错误 | ✅ 已修复 |
| P2 | P2-8 | TopicService Feed 错误忽略 | ✅ 已修复 |
| P3 | P3-1 | 多处 Count 查询忽略 Error | ✅ 已修复 |
| P3 | P3-2 | UpdateProfile 空字符串清空已有值 | ⚠️ 已知设计选择 |
| P3 | P3-3 | reviewed_at 从未设置 | ✅ 已修复 |
| P3 | P3-4 | TopicPage ID=0 无效查询 | ✅ 已修复 |
| P3 | P3-5 | PostService.Update 返回旧对象 | ✅ 已修复 |
| P3 | P3-6 | Auth 中间件 Redis nil 处理 | ⚠️ 误判(当前实现正确) |
| P3 | P3-7 | 错误消息硬编码中文 | ⚠️ 需系统性重构 |

**修复总计**: 18/21 项已修复，3 项标记为已知设计选择/误判/需大重构。

---

## P0 — 致命问题 (1 项)

### P0-1: 编辑帖子时 Topic post_count 持续膨胀，从不减少

**文件**: `internal/repository/topic_repo.go:117-128` (ReplacePostTopics)
**调用链**: `PostService.Update` → `ReplacePostTopics` (在事务内)

**问题描述**:
每次编辑帖子时，`ReplacePostTopics` 会：
1. 删除当前帖子的所有 `topic_posts` 关联记录 (第 118 行)
2. 为新 topic 创建关联并 `post_count + 1` (第 125 行)

但旧 topic 的 `post_count` **从未减 1**。`TopicRepo.DecrementPostCount` 方法已存在 (第 59-62 行) 但从未被调用。

**复现步骤**:
- 发帖 `#A #B` → Topic A count=1, Topic B count=1
- 编辑改为 `#C` → Topic A count 仍=1, Topic B count 仍=1, Topic C count=1
- 再次编辑改为 `#A #D` → Topic A count=2, Topic B count=1, Topic C count=1, Topic D count=1
- 实际只有 1 个帖子关联了 Topic A，但计数为 2

**修复建议**: 在删除旧关联前收集旧 topic_id 列表，循环调用 `DecrementPostCount`。


---

## P1 — 高危问题 (5 项)

### P1-1: PostService.Update 使用 DB.Save 导致级联写入关联表

**文件**: `internal/repository/post_repo.go:61-63` (Update 方法)
```go
func (r *PostRepo) Update(post *model.Post) error {
    return DB.Save(post).Error
}
```

**问题描述**:
`PostService.Update` (第 85 行) 调用 `repo.Update(post)`，此时 `post` 对象通过 `FindByID` 获取，已 Preload 了 `User`、`Images`、`Topics`、`RepostOf`、`RepostOf.User`、`RepostOf.Images`。GORM 的 `Save` 会级联保存所有非 nil 的关联对象，可能：
- 意外更新 `users` 表的字段
- 删除并重建 `post_images` 记录
- 修改 `RepostOf` 原始帖子的数据

**修复建议**: 使用 `DB.Model(post).Updates(map[string]interface{}{"content": ..., "status": ...})` 代替 `DB.Save(post)`，只更新需要的字段。


### P1-2: FindOrCreateWithTx 在事务外执行 FindByName，破坏事务隔离

**文件**: `internal/repository/topic_repo.go:100-114`
```go
func (r *TopicRepo) FindOrCreateWithTx(tx *gorm.DB, name string) (*model.Topic, bool, error) {
    topic, err := r.FindByName(name)  // ← 使用全局 DB，不是 tx！
    ...
    if err := tx.Create(topic).Error; err != nil { ... }  // ← 使用 tx
}
```

**问题描述**: `FindByName` 使用全局 `DB` 实例而非传入的 `tx`，绕过了事务隔离。虽然 `ParseHashtags` 做了去重，但在并发场景下两个请求同时对同一个新话题调用时，两者都看到"不存在"，都尝试 `tx.Create`，其中一个会遇到 duplicate key 错误。

**修复建议**: 在事务内直接用 `tx.Where("LOWER(name) = LOWER(?)", name).First(&topic)` 替代 `r.FindByName(name)`。


### P1-3: resolveTopicIDsInTx 丢弃 FindOrCreateWithTx 的错误返回值

**文件**: `internal/service/post_service.go:107`
```go
topic, _, _ := s.topicRepo.FindOrCreateWithTx(tx, tag)
```

**问题描述**: 三个返回值中 `created` bool 和 `error` 都被丢弃。如果 `FindOrCreateWithTx` 因数据库错误失败 (如 duplicate key)，话题被静默跳过，帖子创建成功但没有关联话题。用户看到帖子发布成功，但 `#标签` 没有生效。

**修复建议**: 检查 error 并至少记录日志，或在事务中 abort。


### P1-4: 点赞/收藏/评论计数同步错误被静默忽略

**文件**:
- `internal/repository/like_repo.go:24-25, 35-36`
- `internal/repository/favorite_repo.go:20-21, 30-31`
- `internal/repository/comment_repo.go:25-26, 70-71`

**问题描述**: 所有计数同步操作 (`UpdateColumn + COUNT(*)` 子查询) 的 error 被完全忽略。如果在高负载下计数同步失败 (如连接池耗尽、死锁重试超限)，`like_count` / `favorite_count` / `comment_count` 将永久不一致。主操作已成功，用户看不到错误。

**修复建议**: 至少用 `log.Printf` 记录错误；考虑异步队列重试。


### P1-5: 帖子创建成功但话题关联失败时没有回滚帖子

**文件**: `internal/service/post_service.go:63-68`
```go
if err := repository.DB.Transaction(func(tx *gorm.DB) error {
    topicIDs := s.resolveTopicIDsInTx(tx, req.Content, req.TopicIDs)
    return s.topicRepo.ReplacePostTopics(tx, post.ID, topicIDs)
}); err != nil {
    log.Printf("PostService.Create: failed to associate topics for post %d: %v", post.ID, err)
}
```

**问题描述**: 帖子已在事务外创建成功 (第 58 行)，话题关联失败只记日志不报错，返回给用户的是"成功创建"。但帖子没有任何话题关联。

**修复建议**: 将 `postRepo.Create(post)` 也放入同一个事务内，失败时整体回滚。


---

## P2 — 中危问题 (8 项)

### P2-1: 转帖选取照片后无响应 (已修复)

**文件**: `web/templates/post/post_detail.html:159-162`

**问题描述**: `handleRepostUpload` 函数中 `reader.readAsDataURL(f)` 放在了 `await` 之后，导致 FileReader 从未启动，Promise 永远挂起，既不上传图片也不显示缩略图。

**修复状态**: ✅ 已修复 (2026-06-19)，将 `readAsDataURL` 移到 `await` 之前。


### P2-2: 点赞/收藏/关注 Toggle 操作不在 DB 事务中

**文件**:
- `internal/repository/like_repo.go:15-40` (Toggle)
- `internal/repository/favorite_repo.go:14-35` (Toggle)
- `internal/repository/follow_repo.go:16-36` (Toggle)

**问题描述**: 每个 Toggle 包含两个操作：(a) INSERT/DELETE 记录，(b) 同步 post_count。两者之间没有事务包裹。如果在 (a) 成功后、(b) 之前进程崩溃，计数永久不一致。

**修复建议**: 用 `DB.Transaction` 包裹两个操作，或使用数据库触发器自动维护计数（已有 003_triggers.sql，但触发器可能不覆盖所有场景）。


### P2-3: Feed Redis 缓存存储完整 Post 对象(含 Preloaded 关联)

**文件**: `internal/service/feed_service.go:113-124`

**问题描述**: 缓存的 `[]model.Post` 包含 `User`、`Images`、`Topics`、`RepostOf` 等完整关联数据：
- 用户信息 (头像/用户名) 在 2 分钟 TTL 内是过时的
- 单条缓存可能 100KB+，高并发下 Redis 内存压力大
- isLiked/isFavorited 在缓存命中后被重新填充，但其他关联数据不刷新

**修复建议**: 缓存仅存 post ID 列表 + total，命中后从 DB 重新查询完整数据；或缩短 TTL 至 30s。


### P2-4: 查看自己帖子也增加 view_count

**文件**: `internal/service/post_service.go:193`
```go
s.postRepo.IncrementView(postID)  // 无条件自增
```

**问题描述**: 作者每次查看自己的帖子都会增加浏览量。这会导致浏览量被人为膨胀 (作者可能反复查看)。

**修复建议**: 加判断 `if post.UserID != currentUserID` 再增量。


### P2-5: Admin 强制删帖不通知作者

**文件**: `internal/service/admin_service.go:36-42`
**对比**: 同文件 `ReviewPost` (第 44-57 行) 会通知作者审核结果。

**问题描述**: `DeletePost` 强制删除帖子后没有调用 `notifSvc.Send`，作者只能在尝试访问帖子时才发现被删除。而审核通过/驳回都有通知，逻辑不一致。

**修复建议**: 在 `DeletePost` 中增加通知发送。


### P2-6: SocialService 分页方法缺少参数校验

**文件**:
- `internal/service/social_service.go:53-54` (GetLikedPosts)
- `internal/service/social_service.go:67-68` (GetFavorites)
- `internal/service/social_service.go:85-90` (GetFollowing/GetFollowers)

**问题描述**: 这些方法直接透传 page/pageSize 到 repository，没有做边界检查和 clamp。对比 `FeedService.GetFeed` (第 36-40 行) 和 `NotificationService.List` (第 56-61 行) 都有 clamp。

**风险**: pageSize=0 导致 LIMIT 0 (空结果)，pageSize=999 导致大查询。


### P2-7: GetFollowing/GetFollowers handler 使用 strconv.Atoi 丢弃错误

**文件**: `internal/handler/social_handler.go:94,105`
```go
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
```

**问题描述**: 错误被丢弃，且 pageSize 硬编码为 20 不暴露给 API 调用方。如果用户传 `page=-1`，offset 计算为负值。


### P2-8: TopicService.GetTopicPage Feed 查询错误被忽略

**文件**: `internal/service/topic_service.go:50`
```go
posts, total, _ := s.postRepo.Feed(q)
```

**问题描述**: Feed 查询的 error 被完全忽略。如果 DB 异常，返回 `posts=nil, total=0`，上层 handler 认为话题存在但无帖子，而不是返回错误。


---

## P3 — 低危问题 (7 项)

### P3-1: 多处 Count 查询忽略 Error

**文件**:
- `internal/repository/like_repo.go:42-46` (CountTotal), `48-52` (IsLiked)
- `internal/repository/favorite_repo.go:50-54` (IsFavorited)
- `internal/repository/follow_repo.go:38-42` (IsFollowing)
- `internal/repository/user_repo.go:65-75` (CountByRole, CountByStatus)
- `internal/repository/comment_repo.go:14-18` (CountNonDeleted)

**问题描述**: `DB.Model(...).Count(&count)` 的 `Error` 返回值被丢弃。DB 故障时静默返回 0，Admin Dashboard 可能显示全 0 而不报错。


### P3-2: UpdateProfile 空字符串会清空已有值

**文件**: `internal/service/auth_service.go:120-128`
```go
updates := map[string]interface{}{
    "avatar_url": avatarURL,  // 空字符串直接覆盖
    "bio":        bio,
    "email":      email,
}
```

**问题描述**: Web 表单提交时未填写的字段变成空字符串，会覆盖数据库中已有的值。例如用户只改 bio 不填 email，email 被清空。

**修复建议**: 只更新非空字段，或区分"未提供"与"清空"的语义。


### P3-3: reviewed_at 时间戳从未被设置

**文件**: `internal/repository/post_repo.go:131-141` (UpdateStatus)

**问题描述**: `Post` 模型有 `ReviewedAt *time.Time` 字段，但 `UpdateStatus` 从未设置它。审核时间无法追溯。


### P3-4: 无意义的 DB 查询 — TopicPage ID=0

**文件**: `internal/service/topic_service.go:28`
```go
if id := parseInt64(idOrName); id > 0 {
    topic, err := s.topicRepo.FindByID(id)
```

**问题描述**: `parseInt64("landscape")` 返回 0，虽然 `FindByID(0)` 会返回 nil（无实际危害），但对 `/topic/0` 或非数字参数会做一次无效的 DB 查询。代码可读性也受影响。


### P3-5: PostService.Update 返回 pre-modification 的 post 对象

**文件**: `internal/service/post_service.go:98`
```go
return post, nil  // UpdatedAt 是旧的
```

**问题描述**: `post` 在内存中修改后调用 `DB.Save`，GORM 会自动更新 `updated_at`，但返回的 `post.UpdatedAt` 仍是旧值。


### P3-6: Auth 中间件对 Redis nil 响应处理不一致

**文件**: `internal/middleware/auth.go:44-50`

**问题描述**: `GetLoginToken` 对 `redis.Nil` (缓存过期) 返回错误，正确拒绝。但如果 `GetLoginToken` 内部逻辑变更不返回 redis.Nil 而返回其他错误，用户会被错误地踢出。


### P3-7: 错误消息硬编码为中文，无 i18n 支持

**影响文件**: `auth_service.go`, `post_service.go`, `social_service.go`, `admin_service.go` 等
**问题描述**: 所有 `errors.New("...")` 返回中文错误消息，国际化和错误码标准化困难。


---

## 安全审查

| 项目 | 状态 | 说明 |
|------|------|------|
| JWT 签名 | ✅ | HMAC-SHA256，72h 过期，secret 通过环境变量注入 |
| 密码存储 | ✅ | bcrypt + DefaultCost |
| XSS 防护 | ✅ | 模板函数 renderHashtags 分段转义 |
| SQL 注入 | ✅ | 全部使用 GORM 参数化查询 |
| CSRF | ⚠️ | SameSite Lax Cookie 提供部分保护，无 CSRF Token |
| 文件上传 | ✅ | 魔数检测 + 50MB 限制 + 类型白名单 |
| 限流 | ✅ | Redis Lua 滑动窗口，fail-open |
| 权限控制 | ✅ | 4 层中间件 (Optional/Required/Admin/Redirect) |
| 暴力破解 | ⚠️ | 登录无限流，无账号锁定机制 |
| HTTPS | ⚠️ | Cookie Secure 标志仅在 ReleaseMode 设为 true |
| 图片代理 | ⚠️ | `/api/v1/images/*` 开放访问，无 Referer 校验 |

---

## 架构评估

### 优点
1. **分层清晰**: Handler → Service → Repository 单向依赖，职责边界明确
2. **统一响应格式**: `response.Success/Error/Page` 规范 JSON 输出
3. **业务错误码与 HTTP 状态码解耦**: 7 组错误码独立管理
4. **中间件设计合理**: 认证 4 层 + 限流 fail-open + NoCache
5. **高可用考虑**: RateLimit 和 Auth Redis 依赖均 fail-open

### 改进建议
1. **事务范围过小**: Post Create/Update 的核心写操作未与话题关联在同一事务内
2. **缓存策略粗糙**: 仅缓存首页最新 Feed，热门排序、搜索均无缓存
3. **计数器维护方式不统一**: 同时存在应用层 COUNT(*)、触发器、直接 +1 三种方式
4. **缺少集成测试**: 85 个测试用例均为单元测试或 API 测试，缺少 DB 事务回滚、Redis 故障等场景覆盖
5. **错误处理不统一**: 有的记日志继续，有的返回错误，有的丢弃错误

---

## 统计汇总

| 严重等级 | 数量 | 关键影响 |
|----------|------|---------|
| P0 (致命) | 1 | Topic post_count 持续膨胀 (数据完整性) |
| P1 (高危) | 5 | 级联写入、事务隔离破坏、错误丢弃、计数不一致 |
| P2 (中危) | 8 | 事务缺失、缓存过时、view_count 不准、校验缺失 |
| P3 (低危) | 7 | 错误忽略、边界 case、代码质量 |
| 安全问题 | 3 | 暴力破解无限流、CSRF 无 Token、图片代理无 Referer |

---

> 审查结论: 项目整体架构合理，核心功能流正常运转。主要风险集中在 **数据一致性** 领域——Topic 计数膨胀 (P0)、DB.Save 级联写入 (P1)、事务范围不足 (P1) 等问题在生产环境中会逐步累积数据脏乱。建议优先修复 P0 和 P1 级别问题。
