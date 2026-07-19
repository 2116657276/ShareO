# 设计文档: IM 私聊 + 群组

> 更新时间: 2026-07-20 | 状态: 草稿 | 上游: [architecture.md](../architecture.md) · [plan.md](../plan.md)

## 1. 背景与目标

ShareO 当前是纯异步的摄影社区（发帖/评论/点赞），缺少用户间实时沟通能力。"私信作者"是摄影社区的高频需求——看到喜欢的作品想联系摄影师，现在除了公开评论没有别的途径。

做完后用户能：和任意用户一对一私聊、创建/加入群组、实时收到消息推送、查看历史消息、看到对方在线状态。

## 2. 非目标

- ❌ 消息已读回执（仅做会话级"已读到第 N 条"，不做每条消息的双勾）
- ❌ 图片/文件消息（v1 仅文本，后续可扩展）
- ❌ 消息撤回/编辑/删除
- ❌ 群组管理功能（转让群主、踢人、禁言）—— Phase 1 只做建群+加群+退群
- ❌ 端到端加密
- ❌ 语音/视频通话

## 3. 总体方案

**核心设计：发消息走 REST POST，WebSocket 只做下行推送。**

```
发送消息:
  用户A ──► POST /api/v1/conversations/:id/messages ──► Go 校验成员 → 落 MySQL → WS Hub 推给在线成员

接收消息:
  Go WS Hub ──► WebSocket ──► 用户B 浏览器实时展示

重连恢复:
  用户B 重连 WS → 前端按 last_read_message_id 调 REST GET /messages?before_id= 拉增量
```

时序：
```
发送侧（REST）          接收侧（WebSocket）
  A ──POST──► Go              Go ──WS──► B（实时）
               │               
               └──MySQL（持久化）
```

**为什么选这个模型？**
- REST 先落库 → 消息不丢（WS 断线也能发）
- 数据库是唯一真相源，WS 只是通知通道
- 天然复用现有 AuthRequired 中间件和限流
- 对比"消息走 WS 双向"：断了就丢了，需要在客户端做本地队列 + 重试，复杂度高

## 4. 数据模型

### 4.1 新表（migrations/009_chat.sql）

```sql
-- 会话表：私聊(dm) 或 群组(group)
CREATE TABLE conversations (
    id         BIGINT PRIMARY KEY AUTO_INCREMENT,
    type       VARCHAR(10)  NOT NULL DEFAULT 'dm',       -- 'dm' | 'group'
    title      VARCHAR(100) DEFAULT '',                   -- 群名（dm 为空，前端用对方用户名）
    owner_id   BIGINT       DEFAULT 0,                    -- 群主 user_id（dm 为 0）
    dm_key     VARCHAR(50)  DEFAULT '',                   -- 私聊唯一键: "小uid:大uid"
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_dm_key (dm_key)                      -- dm_key 为 '' 时 MySQL 不检查唯一性
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 会话成员表
CREATE TABLE conversation_members (
    id                  BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id     BIGINT NOT NULL,
    user_id             BIGINT NOT NULL,
    role                VARCHAR(10) DEFAULT 'member',      -- 'owner' | 'member'
    last_read_message_id BIGINT DEFAULT 0,                  -- 该用户在此会话中最后已读的消息 ID
    joined_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_conv_user (conversation_id, user_id),
    INDEX idx_user_conv (user_id, conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 消息表
CREATE TABLE messages (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    conversation_id BIGINT       NOT NULL,
    sender_id       BIGINT       NOT NULL,
    content         TEXT         NOT NULL,
    meta            JSON         DEFAULT NULL,              -- 扩展字段（Bot 引用等）
    created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_conv_msg (conversation_id, id),               -- 游标分页：WHERE conv_id=? AND id < before_id
    INDEX idx_sender (sender_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 4.2 关键设计决策

**dm_key 唯一约束**：双方同时发第一条私信时，如果只用应用层去重会有竞态条件。
`dm_key = CONCAT(LEAST(uid1, uid2), ':', GREATEST(uid1, uid2))` + UNIQUE INDEX 兜底：
先插入的请求成功，后插入的拿 duplicate key error → 重试 `SELECT WHERE dm_key=?` 取到已有会话。

**游标分页而非 offset**：消息列表需要实时追加。offset 分页在插入新消息后会导致重复/遗漏。
`WHERE conversation_id=? AND id < before_id ORDER BY id DESC LIMIT N` 是游标分页，
新消息插入不影响已有页的结果。

**meta JSON 列**：预留扩展点。Phase 3 Bot 的引用标记（`{citations: [{post_id, chunk_no}]}`）存这里，
不污染核心消息表结构。

### 4.3 与现有表的关系

- `messages.sender_id` → `users.id`：发消息的用户
- `messages.sender_id = Bot 的 user_id`（Phase 3）：Bot 消息自然复用
- `conversation_members.user_id` → `users.id`

## 5. 接口设计

### 5.1 WebSocket

| 项目 | 值 |
|------|-----|
| 端点 | `GET /ws` |
| 认证 | Cookie `token` 或 query `?token=xxx`（WebSocket 不支持自定义 Header） |
| Origin 校验 | 开发环境 `localhost:*`；生产白名单见配置 |
| 心跳 | 30s ping/pong，超时 90s 断开 |
| 下行格式 | `{"type":"new_message","data":{"message":{...}}}` |
| 在线状态 | `{"type":"user_online","data":{"user_id":1,"online":true}}`（仅当前会话相关用户） |

### 5.2 REST API

全部需要 AuthRequired，在 `authAPI` 组下。

| 方法 | 路径 | 入参 | 出参 | 说明 |
|------|------|------|------|------|
| `GET` | `/api/v1/conversations` | — | `[{conv, last_message, unread_count, members}]` | 会话列表，按最近消息时间排序 |
| `POST` | `/api/v1/conversations` | `{user_id}` (dm) 或 `{title, member_ids}` (group) | `{conversation}` | 创建 DM 或群组。DM 幂等：已存在则返回已有会话 |
| `GET` | `/api/v1/conversations/:id/messages` | `?before_id=&limit=30` | `{messages, has_more}` | 游标分页查历史。before_id 为空则取最新 |
| `POST` | `/api/v1/conversations/:id/messages` | `{content}` | `{message}` | 发送消息。校验 sender 是该会话成员 |
| `PUT` | `/api/v1/conversations/:id/read` | `{message_id}` | — | 标记该会话已读到 message_id |
| `GET` | `/api/v1/conversations/unread-count` | — | `{total}` | 所有会话未读总数（Header 徽章用） |
| `POST` | `/api/v1/conversations/:id/join` | — | — | 加入群组（仅 group 类型） |
| `POST` | `/api/v1/conversations/:id/leave` | — | — | 退出群组（群主不能退，需先转让或解散） |

### 5.3 Internal API（ai-service → Go）

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/internal/conversations/:id/context` | 取近 20 条消息作为 LLM 上下文（Phase 3） |
| `POST` | `/internal/bot/reply` | Bot 回调：`{conversation_id, content, meta}`（Phase 3） |

## 6. 关键流程

### 6.1 发送消息（正常路径）

```
1. 用户A POST /api/v1/conversations/:id/messages {content}
2. AuthRequired 解析 JWT 得 user_id = A
3. Service.SendMessage(A, convID, content):
   a. 查 conversation_members WHERE conv_id=? AND user_id=? → 确认 A 是成员
   b. INSERT INTO messages (conv_id, sender_id, content)
   c. 查 conversation_members 得到所有成员 user_id 列表
   d. Hub.SendToUsers(memberIDs, {type:"new_message", data:{message}})
   e. 返回 message 对象
4. Hub 遍历 memberIDs，对每个在线用户的每个连接 writeJSON(message)
5. 用户B（在线）浏览器收到 WS 消息 → 追加到当前会话（如果正在看）或更新未读计数
```

### 6.2 异常路径 1：接收方离线

```
发送方 POST → 消息落库成功（REST 返回 200）
Hub.SendToUsers 发现用户B不在线 → 跳过（无 WS 连接）
用户B 下次上线：重连 WS 时前端调 GET /messages?before_id=last_read_message_id 拉增量
  → 消息不丢
```

### 6.3 异常路径 2：DM 并发创建

```
用户A 和 用户B 同时给对方发第一条私信
两个 POST /api/v1/conversations {user_id: 对方}
  → Service.EnsureDM(A, B):
    1. dm_key = "min(A,B):max(A,B)"
    2. SELECT WHERE dm_key=? → 都查到不存在
    3. INSERT INTO conversations (type='dm', dm_key=...) → 一个成功，一个 duplicate key error
    4. 失败的那个 catch 到 duplicate → 重试 SELECT WHERE dm_key=? → 拿到已有会话
  → 双方拿到同一个 conversation_id，不会创建两个 DM 会话
```

### 6.4 异常路径 3：WS 断线重连

```
用户B WS 断线
  → 前端检测 onclose 事件
  → 启动指数退避重连：1s → 2s → 4s → 8s → 16s（上限）
  → 重连成功后：
    1. 调 GET /conversations → 获取会话列表 + 最后一条消息
    2. 对当前打开的会话：调 GET /messages?before_id=last_rendered_msg_id → 拉增量
  → 消息不丢不重
```

## 7. 风险与权衡

### CSRF 评估

WebSocket 握手：
- 认证：复用 Cookie `token` 解析 JWT（与现有 Web 页面一致）
- Origin 校验：`r.Header().Get("Origin")` 白名单（localhost + 生产域名）
- **不引入新的 CSRF 攻击面**：WS 握手不改变服务端状态，浏览器同源策略阻止恶意站点发起 WS 连接

REST 消息发送：
- 受现有 `SameSite=Lax` Cookie 保护
- 消息发送需要有效的 JWT token（Cookie 或 Authorization header）
- **已知遗留问题**：全局 CSRF Token 中间件将在本 Phase 后续统一添加（覆盖所有 POST/PUT/DELETE 表单）

### 性能预估
- 每条消息：1 INSERT + 1 SELECT（查成员）+ Hub 扇出（O(成员数) 次 writeJSON）
- 10 人在线群组发一条消息：~10ms（MySQL） + ~5ms（Redis 扇出）
- WS 连接数上限：单机 ~1000（受 goroutine 和内存限制，对摄影社区足够）

### 放弃了什么
- 消息 ID 不是全局递增（按会话独立），跨会话排序需要 created_at
- 不做消息同步到其他设备（多端登录各自拉历史即可）

## 8. 评测与测试方案

### 功能验收清单
- [ ] 两个浏览器互发 DM，消息实时可见
- [ ] 一个浏览器关闭，另一个发消息 → 重连后消息拉回
- [ ] DM 并发创建：双方同时发第一条私信 → 只有一个会话
- [ ] 群组：创建 → 邀请 → 群内发消息 → 退群
- [ ] 未读徽章：收到消息时未读数正确
- [ ] 在线状态：对方在线/离线状态正确
- [ ] `make check` 全绿

### 自动化测试
- `chat_repo_test.go`：EnsureDM 并发测试、游标分页测试
- `chat_service_test.go`：SendMessage 权限校验、消息落库
- `ws/hub_test.go`：竞态检测 `go test -race`

### 冒烟脚本 `scripts/test_chat.sh`
```bash
# 注册两个测试用户 → 登录 → 创建 DM → 互发消息 → 查历史 → 验证消息数
```
