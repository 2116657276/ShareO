# 设计文档：IM 私聊与邀请制群组

> 更新时间: 2026-07-21 | 状态: **后端/接口已实现并通过终端验收；浏览器与前端验收暂缓** | 上游: [architecture.md](../architecture.md) · [plan.md](../plan.md)

## 1. 目标与边界

Phase 1 提供文本私聊、邀请制群组、实时下行、准确未读、在线状态和断线恢复。数据库是消息唯一真相源：发送走 REST，WebSocket 只负责下行通知。

本阶段不实现公开加入、邀请码、申请审批、群主转让、踢人、禁言、消息撤回/编辑、图片消息、端到端加密和音视频。群主不能作为普通成员退出，只能解散群组。

## 2. 核心协议

```text
发送：浏览器 -> POST message -> 成员校验 -> MySQL 事务落库/更新会话/推进发送者读标记 -> Hub 下行
接收：Hub -> WebSocket new_message -> 客户端按 message.id 去重 -> 当前会话推进读标记
恢复：WebSocket 重连 -> GET messages?after_id=<最后渲染ID> -> ID 升序追加，直到不足 100 条
历史：GET messages?before_id=<最早渲染ID> -> 向前分页
```

`before_id` 与 `after_id` 不能同时出现。REST POST 响应和 WS 回推可能包含同一消息，客户端必须以全局消息 ID 去重。

## 3. 数据与事务边界

`009_chat.sql` 创建 `conversations`、`conversation_members`、`messages`。已应用迁移不改写；`010_chat_hardening.sql` 负责前向加固：

- 迁移前检查成员、消息、群主引用的孤儿记录；发现异常即 `SIGNAL`，错误文本就是定位孤儿的 SELECT 查询。
- `conversation -> members/messages` 使用 `ON DELETE CASCADE`。
- `member.user_id`、`message.sender_id`、`conversation.owner_id` 使用 `ON DELETE RESTRICT`。
- DM 的 `owner_id` 从旧值 `0` 迁为 `NULL`。

关键事务：

- `EnsureDM`：`dm_key=min(uid):max(uid)` 唯一；并发插入用冲突忽略后回查；同一事务幂等补齐两名成员。
- 建群：先去重、验证正常用户和 50 人上限，再一次提交会话与成员。
- 邀请：锁定 conversation 行，再重查现有成员和上限，避免并发邀请共同越过 50 人。
- 发消息：同一事务插入消息、更新 `conversation.updated_at`，并以 `GREATEST` 推进发送者读标记；成员消失时整笔回滚。
- 解散：仅群主；事务内删除消息、成员和会话。即使数据库有级联约束，显式顺序仍便于审计和兼容已迁移环境。

准确未读定义：当前成员读标记之后、且发送者不是自己的消息数。会话列表逐会话返回准确 `unread_count`，不使用最后消息差值近似。

`MarkRead` 只接受目标会话中真实存在的消息，并以 `GREATEST(last_read_message_id, message_id)` 保证不能倒退。

## 4. 输入与权限

| 项目 | 规则 |
|------|------|
| 消息 | trim 后 1–2000 Unicode 字符 |
| 群名 | trim 后 1–100 Unicode 字符 |
| 群成员 | 含群主最多 50 人 |
| 用户搜索 | 非空关键词，最多 20 条；排除调用者和封禁用户 |
| 邀请 | 仅群主；重复邀请幂等成功 |
| 退出 | 仅普通群成员；群主返回 409 |
| 解散 | 仅群主；DM 不可解散 |

错误映射固定为：参数 400、非成员/非群主 403、不存在 404、状态冲突 409、未知数据库故障 500。数据库错误原文不得返回客户端。

## 5. API

全部 REST 路由使用完整登录校验。

| 方法 | 路径 | 请求/查询 | 说明 |
|------|------|-----------|------|
| GET | `/api/v1/conversations` | — | 最近更新优先；返回准确未读、成员；DM 额外返回 `online` |
| POST | `/api/v1/conversations` | `{"user_id":2}` | 幂等创建/获取 DM |
| POST | `/api/v1/conversations` | `{"title":"组名","member_ids":[2,3]}` | 创建邀请制群组 |
| GET | `/api/v1/conversations/:id/messages` | `before_id` 或 `after_id`，`limit<=100` | 历史或断线补偿；`after_id` 按 ID 升序 |
| POST | `/api/v1/conversations/:id/messages` | `{"content":"..."}` | 事务发送文本消息 |
| PUT | `/api/v1/conversations/:id/read` | `{"message_id":123}` | 单调推进读标记 |
| GET | `/api/v1/conversations/unread-count` | — | 全部会话准确未读总数 |
| POST | `/api/v1/conversations/:id/members` | `{"user_ids":[2,3]}` | 群主直接邀请，幂等 |
| DELETE | `/api/v1/conversations/:id/members/me` | — | 普通成员退出 |
| DELETE | `/api/v1/conversations/:id` | — | 群主解散 |
| GET | `/api/v1/users/search` | `q`、`limit<=20` | 返回 `id/username/avatar_url` |
| GET | `/ws` | Cookie 或 Bearer | WebSocket 握手 |

不存在 `POST .../:id/join` 和旧 `POST .../:id/leave` 兼容路由。

下行消息：

```json
{"type":"new_message","data":{"message":{"id":123,"conversation_id":9,"sender_id":2,"content":"你好"}}}
```

## 6. 身份、Origin、CSRF 与会话吊销

- WS 握手复用 HTTP 的 JWT 解析与 Redis 登录缓存比对，不接受只验证 JWT、已被替换/登出的旧会话。
- 同源 Origin 必须同时匹配 scheme 和 host；跨源只接受 `server.trusted_origins` / `SHAREO_TRUSTED_ORIGINS` 的精确 `http(s)://host[:port]`，禁止 `*`、路径、查询和片段。
- 全站浏览器 POST/PUT/PATCH/DELETE 由 Go 1.25 `http.CrossOriginProtection` 保护。CLI、Bearer 和内部客户端在没有浏览器跨站头时按标准库行为兼容，不引入 CSRF Token 库或 bypass pattern。
- Web 登出改为带确认的 `POST /logout`。帖子浏览计数由 GET 查询移到 `POST /api/v1/posts/:id/view`。
- 登出、修改密码、管理员封禁成功后，Hub 主动关闭该用户全部 WS 连接。

## 7. 在线状态与连接管理

- 每个用户可有多个连接；Hub 以 user ID -> connection set 管理。
- 注册时写 `ws:online:<uid>`；每 30 秒 ping 时刷新 60 秒 TTL。
- 关闭连接先从 Hub 注销；只有最后一个连接消失时才删除 Redis key。
- 写缓冲满视为慢连接并主动关闭；ping 30 秒，pong 超时 90 秒。
- DM 会话列表返回对端 `online`。页面每 30 秒刷新列表，收到消息和重连成功时立即刷新。

## 8. 页面行为

- `/chat?conv=<id>` 在会话列表加载后自动打开该会话。
- 发送响应和 WS 推送统一走 `appendMsg`，按 `message.id` 去重。
- 重连采用 1/2/4/8/16 秒指数退避；成功后调用 `after_id` 增量接口。
- 页面提供建群、搜索/选择成员、群主邀请和解散、普通成员退出入口。

## 9. 自动化与手工验收

自动化覆盖：群权限、消息限制、错误映射、未读委托、MarkRead 归属、分页冲突、Hub 多连接/慢连接、CSRF 同源/跨源矩阵、Python 消费重试/重启。真实 MySQL 集成文件覆盖并发 DM、消息回滚、准确未读、读标记单调、消息顺序、邀请上限、解散和 010 外键；真实 Redis 集成覆盖 pending 重领和最终 ACK。2026-07-21 终端验收已通过：`scripts/test_chat.sh` 全部通过，通用 `scripts/test_api.sh` 为 34/34。

当前策略：后端和接口成熟前，终端 curl/Shell、单元测试、真实 MySQL/Redis 与 Compose 是正式验收手段；浏览器交互、断网恢复和页面可用性清单暂不执行。暂缓不代表删除，发布前仍需按下列清单补验。

双浏览器清单（全部勾选后才能把状态改为“已实现/已验收”）：

- [ ] 两用户实时互发；发送者页面每条消息只出现一次
- [ ] B 离线，A 发送；B 重连后消息按序补齐且不重复
- [ ] 未读徽章在接收、打开、MarkRead 后准确变化
- [ ] DM 在线点随最后连接上线/离线正确变化
- [ ] 密码修改、退出和管理员封禁后，旧 WS 立即断开且不能重连
- [ ] 群外用户无法公开加入；群主可邀请；重复邀请不新增成员
- [ ] 普通成员可退出；群主退出返回 409；群主解散后各成员不可访问
- [ ] `/chat?conv=<id>` 能自动打开目标会话
- [ ] 可信跨源按配置工作，未配置跨站浏览器写请求返回 403

最终命令与证据矩阵见 [进度审计](../reviews/2026-07-21-progress-audit.md)。
