# TASK.md — 开发任务跟踪

> 更新时间: 2026-07-21 | 当前阶段: **Phase 0 本地门禁完成、Phase 1 后端加固中 / Phase 2 语义搜图后端垂直切片进行中** | 详细计划: [docs/plan.md](docs/plan.md)
>
> 本文件与 `docs/plan.md` 是任务状态的事实来源。GitHub Issues 仅用于可选协作，不决定阶段完成度。

## Phase 0 收口

- [x] Redis Streams：失败不 ACK；30 秒 idle 后由 `XAUTOCLAIM` 重领；最多 4 次处理；最终记录上下文并 ACK，不设死信队列
- [x] Python 3.12 + uv dev dependency group + `uv.lock`；CI locked/frozen；Ruff、pytest、Redis `aclose()` 已收口
- [x] `make check-go` / `check-python` / `check-shell`；集成测试独立为 `make test-integration`
- [x] `scripts/test_api.sh` 临时图片生成与清理修复；所有 Shell 脚本可做统一语法检查
- [x] Compose 只自动执行结构迁移；开发凭证与示例配置一致；提供 config/up/ready/reset/clean 命令
- [x] AI `/healthz` 与 `/readyz` 分离；Compose 使用 readiness healthcheck
- [x] 业务层、仓储层和 WS 裸 `log.Printf/log.Println` 已迁移到 `slog`
- [x] 在 Docker Compose 主机完成镜像拉取、启动、ready/reset 验收，并记录 MinIO/Qdrant/uv 基础镜像 digest（本机固定端口冲突时使用端口覆盖）
- [x] 在真实 Redis DB 15 执行 pending 重领/最终 ACK 集成测试（2 项通过）

## Phase 1 加固

- [x] 新增前向迁移 `010_chat_hardening.sql`：孤儿预检、清理查询、会话级联和用户限制外键
- [x] `ChatRepository` / `PresenceStore` / `MessageHub` 接口化；Chat Handler 全部传递请求 context
- [x] DM、建群、发消息、解散事务化；并发邀请成员上限通过行锁保护
- [x] 消息/群名/成员数/用户搜索限制；统一 400/403/404/409 与数据库错误脱敏
- [x] 精确未读 SQL；MarkRead 验证本会话消息并使用 `GREATEST`
- [x] 删除公开 join/旧 leave；实现群主邀请、普通成员退出、群主解散、用户搜索
- [x] WS 握手复用 JWT + Redis 登录缓存；登出、改密、封禁主动断开连接
- [x] 精确 Origin 白名单；Redis 在线 TTL；最后连接断开删除在线 key
- [x] 前端消息 ID 去重、`after_id` 断线补偿、`?conv=` 自动打开、30 秒刷新、群组操作入口
- [x] Go 1.25.1+ `http.CrossOriginProtection`；`POST /logout`；浏览计数改为受保护的 POST
- [x] 单元测试与可选 MySQL/Redis 集成测试骨架；扩展 `scripts/test_chat.sh`
- [x] 在临时迁移的 `*_test` MySQL 库与真实 Redis DB 15 执行 `make test-integration`，通过后自动清理
- [x] 运行终端 API/IM 冒烟脚本；双浏览器和前端手工清单按当前决策暂缓，不作为后端成熟前的工作项

## 最终门禁

- [x] `make check`
- [x] `make test-integration`（真实 MySQL/Redis，不以 skip 代替通过）
- [x] `bash -n` 全部 Shell 脚本
- [x] Compose config/up/ready/reset 验证
- [x] `scripts/test_api.sh`（34/34）与 `scripts/test_chat.sh`
- [x] `go test -race ./...`
- [ ] 双浏览器手工清单全部勾选（按当前决策暂缓，后端/接口成熟后再执行）
- [x] 文档链接检查、工作区意外文件检查

只有上面门禁都有证据后，才把 Phase 0 标为 100%、Phase 1 标为完成。当前证据见 [2026-07-21 进度审计](docs/reviews/2026-07-21-progress-audit.md)。

## Phase 2–4

Phase 2 语义搜图后端已开始：

- [x] Chinese-CLIP 懒加载、CUDA → MPS → CPU 设备选择、512 维归一化输出
- [x] Qdrant `images` collection、索引载荷和 token 保护的 AI 搜索 API
- [x] 审核/删除事件、Go 图片代理 worker、幂等 upsert/delete、`make backfill-index`
- [x] Go `GET /api/v1/search/images` 可见性过滤和统一错误映射
- [x] Python 单测、worker 单测和 `scripts/test_image_search.sh` 终端验收脚本
- [ ] Homebrew MinIO + Qdrant + Chinese-CLIP 实际上传→审核→搜索→删除端到端证据
- [ ] 搜索页面、浏览器验收、评测集与 4060 吞吐实验（按当前范围暂缓）

任务和时间线保留在 `docs/plan.md` 与 `docs/roadmap.md`；没有真实 Qdrant/模型端到端证据前，Phase 2 不标记完成。
