# ShareO 代码评审报告（2026-07-19）

> 审查时间: 2026-07-19 | 范围: 全部 Go 源码（handler 9 / service 10 / repository 12 / middleware 3 / pkg 3 / cmd 2）+ 模板抽查 + 构建与测试客观检查
>
> 前置: 2026-06-19 评审的 18/21 项修复已逐一核实，均确认修复到位（Topic 计数、DB.Save 级联、事务范围、ID-only 缓存、view_count 自增等）。本次聚焦上次评审后的改动与漏网问题。
>
> 客观检查: `go build` ✅ | `go vet` ✅ | `go test ./...` 全部通过 ✅ | `gofmt -l` **13 个文件未格式化** ❌

每项标注修复难度：**易**（<30 分钟）/ **中**（半天内）/ **大**（需专项规划）。所有"易"项已列入 [Phase 0 修复清单](../plan.md#phase-0)。

## 修复状态 (2026-07-20)

| 等级 | 编号 | 问题 | 状态 |
|------|------|------|------|
| P1 | SR-01 | 搜索 page_size=0 除零 panic | ✅ 已修复 |
| P1 | SR-02 | 不存在用户主页 nil 指针 panic | ✅ 已修复 |
| P1 | SR-03 | SettingsPage / WebSettings nil 解引用 | ✅ 已修复 |
| P1 | SR-04 | Web 设置保存清空头像 | ✅ 已修复 |
| P2 | SR-05 | Web 表单登录/注册无限流 | ✅ 已修复 |
| P2 | SR-06 | 取消收藏失败仍报成功 | ✅ 已修复 |
| P2 | SR-07 | 搜索 FULLTEXT 语法错误 + 泄漏 DB 错误 | ✅ 已修复（2026-07-20） |
| P2 | SR-08 | 可转发未过审帖子 | ✅ 已修复 |
| P2 | SR-09 | 评论计数不在事务内 | ✅ 已修复 |
| P2 | SR-10 | 管理员删不存在帖返回成功 | ✅ 已修复 |
| P2 | SR-11 | 图片代理接受任意 HTTP 方法 | ✅ 已修复 |
| P2 | SR-12 | 上传无并发上限 | ✅ 已修复（2026-07-20） |
| P3 | SR-13 | gofmt 13 文件 | ✅ 已修复 |
| P3 | SR-14 | JWT 算法白名单 | ✅ 已修复 |
| P3 | SR-15 | RedirectIfAuth 未挂载 | ✅ 已修复 |
| P3 | SR-16 | 死代码 | ✅ 已修复 |
| P3 | SR-17 | 限流文档纠偏 | ✅ 已修复 |
| P3 | SR-18 | GetLoginToken 注释 | ✅ 已修复 |
| P3 | SR-19 | Email 格式校验 | ✅ 已修复 |
| P3 | SR-20 | URL 转义 | ✅ 已修复 |
| P3 | SR-21 | 模板 glob fatal | ✅ 已修复 |
| P3 | SR-22 | HomePage error userData | ✅ 已修复 |
| P3 | SR-23 | reply_to_uid 校验 | ✅ 已修复（2026-07-20） |
| P3 | SR-24 | admin 封禁自保护 | ✅ 已修复 |

**修复总计**：24/24 项已修复（含原 20 易修项 + 3 中项 SR-07/12/23 + SR-24）。

---

## P1 — 高危（4 项，全部可稳定复现）

### SR-01: 搜索接口 page_size=0 触发除零 panic 【易】

`internal/handler/feed_handler.go:33`
`GET /api/v1/search?q=x&page_size=0` → `totalPages := (int(total)+pageSize-1)/pageSize` 除以 0 → panic（Gin recovery 兜成 500）。任何未登录用户可触发。
**修复**: Search 改用 `getPageSizePair` 并在 service 层 clamp（对齐 GetFeed）。

### SR-02: 访问不存在用户主页 nil 指针 panic 【易】

`internal/handler/user_handler.go:24` + `internal/service/user_service.go:35`
`UserRepo.FindByID` 未命中返回 `(nil, nil)` → `GetProfile` 返回 `(nil, nil)` → handler 判断 `err != nil || data.User == nil` 时 `data` 为 nil → 解引用 panic。`/user/999999` 稳定复现。
**修复**: handler 补 `data == nil` 判断；或 service 对 user==nil 返回明确错误。

### SR-03: SettingsPage / WebSettings 同类 nil 解引用 【易】

`internal/handler/auth_handler.go:174,196`
`user, _ := h.svc.GetProfile(userID)` 后直接 `user.Email`。用户记录缺失或 DB 错误时 panic。
**修复**: 判空 + 错误处理。

### SR-04: 网页版保存设置会清空头像 【易】

`internal/handler/auth_handler.go:186` + `web/templates/user/settings.html`
settings 表单**没有** `avatar_url` 字段，`c.PostForm("avatar_url")` 恒为空串，而 `UpdateProfile` 语义是"空串覆盖"→ 每次网页保存设置都会清掉通过 API 设置的头像。（另注：目前没有任何设置头像的 UI，头像功能是半成品。）
**修复**: Web 路径改为仅更新表单实际提交的字段；头像上传 UI 列入开发计划。

---

## P2 — 中危（8 项）

### SR-05: Web 表单登录/注册没有限流 【易】

`internal/router/router.go:123-124`
API 版 `/api/v1/auth/login` 有 10 次/分钟限流，但浏览器表单走的 `POST /login`、`POST /register` **没挂** limiter → 暴力破解绕行通道。
**修复**: 两条 Web 路由挂同一 `authLimiter`。

### SR-06: 取消收藏失败仍报成功 【易】

`internal/repository/favorite_repo.go:23`
`tx.Delete(&existing)` 的 error 被忽略（对比 `like_repo.go:24` 有检查）。删除失败时事务照常提交、返回"已取消收藏"，但记录还在。
**修复**: 对齐 like_repo 的错误检查。

### SR-07: 搜索词可触发 FULLTEXT 语法错误且泄漏 DB 错误信息 【中】

`internal/repository/post_repo.go:215` + `internal/handler/feed_handler.go:30`
`MATCH ... AGAINST(? IN BOOLEAN MODE)` 直接用用户输入，含未闭合引号等 BOOLEAN 语法时 MySQL 报错；handler 把 `err.Error()`（含 SQL 片段）原样返回给用户。
**修复**: ① MATCH 失败时降级 LIKE 重试或预清洗 BOOLEAN 操作符；② handler 统一脱敏为"搜索失败"。LIKE 分支同时转义 `%`/`_`（`post_repo.go:217`）。

### SR-08: 可转发未过审帖子，间接暴露 pending/rejected 内容 【易】

`internal/service/post_service.go:135-137`
`Repost` 只校验原帖存在且未删除，不校验 `Status == approved`。详情页对他人隐藏未过审帖，但通过转发（转发过审后 `RepostOf` 预加载渲染原帖）可绕过。
**修复**: Repost 增加 `original.Status == model.StatusApproved` 校验。

### SR-09: 评论计数同步不在事务内 【易】

`internal/repository/comment_repo.go:23-33, 55-80`
like/favorite 的 Toggle 已包 `DB.Transaction`，但评论 Create/SoftDelete 的"写记录 + 同步 comment_count"仍是两个独立操作，中间崩溃则计数漂移。
**修复**: 包进 `DB.Transaction`（照抄 like_repo 模式）。

### SR-10: 管理员删除不存在的帖子返回成功 【易】

`internal/service/admin_service.go:38-41`
`if err != nil || post == nil { return err }` — post 为 nil 且 err 为 nil 时返回 nil（成功）。
**修复**: post 为 nil 时返回"帖子不存在"。

### SR-11: 图片代理接受任意 HTTP 方法 【易】

`internal/router/router.go:104`
`api.Any("/images/*objectName", ...)` 允许 POST/PUT/DELETE 打到图片代理。
**修复**: 改为显式 `GET` + `HEAD` 两条注册。

### SR-12: 上传无并发上限，io.ReadAll 全量进内存 【中】

`internal/pkg/upload/thumbnail.go:37`
50MB × N 并发 = OOM 风险（TASK.md 已有此条 P2）。
**修复**: 全局 semaphore（如 buffered channel 容量 4）包住 ProcessAndUpload。

---

## P3 — 低危 / 质量（12 项）

| # | 问题 | 位置 | 难度 |
|---|------|------|------|
| SR-13 | 13 个文件未过 gofmt | `gofmt -l` 输出 | 易 |
| SR-14 | JWT 解析未限定签名算法（应加 `jwt.WithValidMethods(["HS256"])`） | `internal/pkg/jwt/jwt.go:55` | 易 |
| SR-15 | `RedirectIfAuth` 中间件定义后从未挂载（已登录用户仍可见登录页）——挂到 `GET /login`、`GET /register` 即补全功能 | `internal/middleware/auth.go:152`, `router.go:121-122` | 易 |
| SR-16 | 死代码群：`TopicRepo.FindOrCreate/AddPostToTopic/IncrementPostCount/DecrementPostCount/List/Update/Delete`、`AdminService.CreateTopic/UpdateTopic/DeleteTopic`、`NotificationService.Stats`、`upload.UploadImage`(被 ProcessAndUpload 取代，需二次确认) | topic_repo.go / admin_service.go / notification_service.go / minio.go | 易 |
| SR-17 | 限流实现是**固定窗口**（INCR+EXPIRE），README 与注释宣称"滑动窗口"，文实不符 | `internal/middleware/ratelimit.go:14` | 易(改文档) |
| SR-18 | `GetLoginToken` 注释说"不存在返回 ("", nil)"，实际返回 redis.Nil 错误（中间件依赖后者，注释误导） | `internal/repository/auth_cache.go:38` | 易 |
| SR-19 | 注册 Email 无格式校验（缺 `binding:"omitempty,email"`） | `internal/service/auth_service.go:26` | 易 |
| SR-20 | WebCreateComment 重定向把错误信息拼进 URL 未做 QueryEscape | `internal/handler/social_handler.go:165` | 易 |
| SR-21 | 模板 glob 错误被丢弃，模板目录缺失时静默启动、首次渲染才炸；应在模板数为 0 时 Fatal | `cmd/server/main.go:117-119` | 易 |
| SR-22 | HomePage 错误分支渲染时漏包 `userData()`，头部导航丢登录态 | `internal/handler/feed_handler.go:91` | 易 |
| SR-23 | 评论 `reply_to_uid` 不校验目标用户是否存在/是否参与讨论，可对任意用户定向刷通知 | `internal/service/social_service.go:140` | 中 |
| SR-24 | 管理员可封禁自己/其他管理员（无 role/self 检查） | `internal/service/admin_service.go:83` | 易 |

其它记录在案：自定义模板函数 `or` 覆盖了 html/template 内建 `or`（当前用法安全，新模板需注意，v2 建议改名 `coalesce`）；默认管理员密码 `admin123` 硬编码于 `migrations/001_init.sql:331`（演示可接受，对外部署前必须改，且"修改密码"功能缺失——已列入开发计划）；`log.Printf` 满天飞 → slog 迁移列入 Phase 0。

---

## 安全清单（对上次评审的增量更新）

| 项目 | 状态 | 变化 |
|------|------|------|
| 暴力破解 | ⚠️ | API 已限流，**Web 表单未限流（SR-05）** |
| 单设备登录/强踢 | ✅ | 新增的 Redis 登录缓存实现正确，fail-open 策略一致 |
| JWT | ⚠️ | 建议补算法白名单（SR-14） |
| CSRF | ⚠️ | 仍仅 SameSite=Lax，无 Token（v2 引入 IM 表单前评估，列入 Phase 1 设计考虑） |
| 错误信息泄漏 | ⚠️ | 搜索路径泄漏 DB 错误（SR-07）；多数 handler 把 service 错误原样透出，建议 v2 统一错误映射层 |
| 上传 | ✅ | 魔数检测/大小限制/类型白名单完备；并发内存是遗留项（SR-12） |

## 架构评估（面向 v2 的结论）

1. **分层纪律好**，v2 按现有模式扩展没有障碍。
2. **全局单例（DB/RDB/Client）+ handler 内 new service** 使单测依赖真实 DB 困难——v2 新模块（chat/search/bot）**从第一天用构造函数注入**，老代码不强推（ADR-001 原则：增量演进）。
3. **repo 层无 context 传递**，超时/取消不可控——v2 新代码全部带 `context.Context`，老代码遇改动顺手补。
4. 错误处理已有 sentinel error 雏形（`ErrPostNotFound`），v2 扩展这套模式而不是继续 `errors.New` 中文散弹。

## 汇总

| 等级 | 数量 | 其中"易修" |
|------|------|-----------|
| P1 | 4 | 4 |
| P2 | 8 | 6 |
| P3 | 12 | 10 |

**20 个易修项全部并入 [docs/plan.md](../plan.md) 的 Phase 0 修复清单**，按"先 P1 → P2 → P3"顺序执行，每项独立提交。
