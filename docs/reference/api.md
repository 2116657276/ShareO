# API 参考

> 路由事实源：`internal/router/router.go` 与 `ai-service/app/main.py`。本文件只维护稳定接口契约；阶段状态见 [`TASK.md`](../../TASK.md)。

本文件描述已存在的 HTTP 接口，不承诺每个接口都已有浏览器入口。接口审计以本文件和代码路由为对照；验证统一使用 curl、Go/Python 集成测试和页面内 fetch，不把浏览器直接打开 JSON 时的 `ERR_BLOCKED_BY_CLIENT` 当作应用路由错误。

## 通用约定

Go JSON API 成功响应为：

```json
{"code":0,"message":"success","data":{}}
```

业务错误码：`1001` 参数错误、`1002` 未登录、`1003` 禁止、`1004` 不存在、`1005` 内部错误、`1006` 限流、`1007` 冲突。HTTP状态仍分别使用 400、401、403、404、500、429、409。

浏览器登录态使用 JWT Cookie 和 Redis登录缓存；API也可使用 Bearer。浏览器写操作受同源保护。Go↔AI内部接口要求 `X-Internal-Token`，不得暴露给浏览器。

## 公开与可选登录 API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/auth/register` | 注册 |
| POST | `/api/v1/auth/login` | 登录 |
| GET | `/api/v1/feed` | approved Feed |
| GET | `/api/v1/search` | FULLTEXT/LIKE关键词搜索 |
| GET | `/api/v1/search/images?q=&limit=` | 中文语义搜图 API；AI/Qdrant 不可用时503；页面可复用此接口 |
| GET | `/api/v1/posts/:id` | 可见帖子详情 |
| POST | `/api/v1/posts/:id/view` | 显式记录浏览 |
| GET | `/api/v1/posts/:id/comments` | 可见评论 |
| GET | `/api/v1/users/:id/following` | 关注列表 |
| GET | `/api/v1/users/:id/followers` | 粉丝列表 |
| GET | `/api/v1/users/:id/likes` | 用户点赞帖子 |
| GET/HEAD | `/api/v1/images/*objectName` | Go图片代理 |

## 登录 API

| 域 | 接口 |
|---|---|
| 账号 | `GET /auth/me`、`POST /auth/logout`、`PUT /auth/profile`、`PUT /auth/password` |
| 帖子 | `POST /posts`、`PUT /posts/:id`、`DELETE /posts/:id` |
| 互动 | `POST /posts/:id/like`、`GET /likes`、`POST /posts/:id/comments`、`DELETE /comments/:cid`、`POST /users/:id/follow` |
| 上传 | `POST /upload` |
| 用户搜索 | `GET /users/search?q=&limit=`，返回正常用户并排除调用者；固定Bot可被搜索 |
| 通知 | `GET /notifications`、`PUT /notifications/:id/read`、`PUT /notifications/read-all`、`GET /notifications/unread-count` |
| 私聊 | `GET/POST /conversations`、`GET/POST /conversations/:id/messages`、`PUT /conversations/:id/read`、`GET /conversations/unread-count` |

以上路径均位于 `/api/v1`。创建会话只接受目标 `user_id`；群组字段和群组路由不存在。消息 trim 后为 1–2000字符；`before_id` 与 `after_id` 不能同时出现。

WebSocket 使用 `GET /ws`。握手复用登录校验，下行新消息包含稳定 message ID，客户端必须用该 ID去重。

## 管理员 API

所有路径位于 `/api/v1/admin`，同时要求登录和管理员角色：

- `GET /stats`
- `GET /pending-posts`
- `POST /posts/:id/approve`
- `POST /posts/:id/reject`
- `DELETE /posts/:id`
- `GET /users`
- `PUT /users/:id/status`
- `GET /logs`

## Go 内部 API

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/internal/health` | 内部Go状态 |
| GET | `/internal/posts/index-payloads` | 分页获取可索引帖子 |
| GET | `/internal/posts/:id/index-payload` | 获取单帖正文与图片载荷 |
| GET | `/internal/bot/tasks/:message_id` | 获取严格校验的Bot上下文 |
| POST | `/internal/bot/reply` | 幂等写入Bot回复与引用 |
| GET | `/internal/posts/agent/search` | Agent 关键词/语义帖子检索，内部 Token |
| POST | `/internal/posts/agent/read` | Agent 批量读取帖子，内部 Token |

索引载荷对不存在、非 approved或软删除帖子返回404。Bot回复接口再次验证 DM、固定Bot身份、来源消息和帖子可见性。

## AI 内部 API

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/healthz` | 进程存活 |
| GET | `/readyz` | Redis/Qdrant基础就绪 |
| GET | `/readyz/image-search` | 图片模型、collection和consumer就绪 |
| GET | `/readyz/rag` | 文本模型、collection、consumer和LLM配置就绪 |
| GET | `/v1/meta/image-search` | 模型、revision、device和collection信息 |
| POST | `/v1/search/images` | 文本编码与图片KNN |
| POST | `/v1/rag/answer` | 受限RAG回答 |

除 `/healthz` 和基础 `/readyz` 外，能力接口要求内部 token。RAG问题为1–500字符，历史最多20条，`top_k` 为1–20。AI 接口不直接对浏览器公开；Go 负责公开 API、权限和引用最终校验。

## API 契约审计与测试边界

API 阶段必须覆盖每条公开、管理员、Go 内部和 AI 内部接口的成功路径、非法参数、未登录、越权、资源不存在、依赖不可用和敏感信息不泄露。公开语义搜图保持 `GET /api/v1/search/images`，Go 内部调用 AI 的 `POST /v1/search/images`；除非测试证明现有路径、方法或响应与本文件不一致，否则不重命名、不迁移、不增加公开 RAG/Agent 路由。

`make test-api` 是内部开发验证入口，使用本机服务、唯一测试用户、临时合成图片和本轮可回收数据，不重置现有 Demo 数据。API 测试通过后才进入独立 `/search/images` 页面开发。
