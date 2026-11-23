# ShareO：AI 原生社区后端与多模态 RAG

ShareO 是一个 Go 主导的 AI 原生社区后端与多模态 RAG 项目。它把图文社区、一对一实时私聊、中文搜贴、中文语义搜图、带可访问引用的固定 RAG Bot 和显式开启的只读深度分析能力收在同一套业务链路中，重点展示事务、异步恢复、权限复核和检索评测，而不是把 Agent 框架作为主角。

日常启动、关闭和 Make 用法见 [用户使用指南](docs/USER_GUIDE.md)。工程结论、证据等级和剩余边界见 [工程审查](docs/REVIEW.md)；脱敏运行结果见 [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)。项目不宣称通用 Agent、多 Agent、生产级高并发、线上 SLA 或公共克隆一键复现完整图片质量评测。

## 项目范围

已实现的业务能力包括：

- 注册登录、资料、图文发布、管理员审核、Feed、点赞、关注、评论、通知和私人收藏。
- WebSocket 一对一私聊，支持消息落库、未读、断线补偿和 Bot 回复推送。
- JWT 使用 PostgreSQL 持久化 `session_version` 做注销、改密和封禁撤销；Redis 保存当前登录令牌并参与每次受保护请求的会话校验，但不是真正的撤销事实源。Redis 不可用时，受保护 REST 和 WebSocket 握手会 fail-closed 并返回受控错误。
- 图片上传记录 owner、批次和过期时间，发帖在同一事务内绑定对象；未绑定对象由本机定时任务有界回收。
- 正文混合搜贴：BGE/pgvector 语义召回与 PostgreSQL `LIKE` 关键词召回独立融合。
- Chinese-CLIP + pgvector 中文语义搜图，公开结果由 Go 做最终可见性复核。
- 默认私聊 Bot 固定走可预测的 RAG 管线；只有用户显式开启深度分析后，才进入 LangGraph 只读 Agent。Agent 是增强能力，不是默认主链路。
- Agent 仅有语义检索、关键词检索、帖子读取和图片检索四类工具，不具备写入、外部调用、长期记忆或多 Agent 能力。

转帖、话题、群聊、写入型 Agent、外部工具、独立 Bot 页面、音视频和生产级多租户不在范围内。

## 架构

```text
浏览器
  → Go 服务 → PostgreSQL / Redis / MinIO / WebSocket
          → 内部 HTTP + Token → Python FastAPI
                                  → Chinese-CLIP / FastEmbed
                                  → PostgreSQL `ai` schema / pgvector
                                  → RAG / LangGraph Agent / LLM
```

Go 是业务数据和 PostgreSQL `public` schema 的唯一写者，负责认证、权限、审核、社区、聊天和 Bot 回复；Python 只写同一实例 `ai` schema 的 pgvector 派生表，负责 embedding、向量检索、RAG、Agent 和 Redis Streams consumer。MinIO 凭证只由 Go 持有，Python 通过 Go 图片代理读取图片。所有公开帖子和引用最终都必须满足 `approved AND is_deleted=0`。

关键链路如下：

```text
发布 → 审核 → index_post → Redis Streams → 图片/正文索引 → pgvector → 搜索 → Go 可见性复核
私聊 → PostgreSQL 消息 + outbox 同事务 → outbox 重试发布 bot_tasks → RAG → 引用白名单 → Go 二次校验 → Bot 落库 → WebSocket
ai_mode=agent → LangGraph → 四类只读工具 → 脱敏轨迹 → 引用复核 → 回复展示
```

## 简历与技术面试展示重点

面试可以从数据所有权、异步恢复、请求准入、Agent 权限和检索评测五方面展开。

- Go 是 `public` 业务数据和权限的唯一写者，Python 只负责 `ai` schema 的派生向量；跨服务返回的帖子和引用由 Go 再做 `approved AND is_deleted=0` 可见性复核。
- 私聊 Bot 使用 PostgreSQL 事务 outbox、Redis Streams 的 at-least-once 消费和 `source_message_id` 幂等，能解释故障重试、重领、兜底和重复回调。
- Bot pending 准入在 PostgreSQL 事务内按用户 advisory lock 原子执行；语义搜贴和搜图共享有限并发闸门，避免模型请求无界放大。
- Agent 只有四类只读工具，工具参数、调用次数、观察长度、总时限和引用格式都有边界；默认私聊仍是 RAG，不把受限 Agent 宣称为通用或多 Agent 平台。
- 检索质量同时记录 Recall/MRR/nDCG、no-match、引用精度、来源覆盖、工具选择、注入拒答和延迟，并明确区分本机诊断结果与线上保证。

面试演示建议沿“发布/审核/索引 → 搜索 → 引用复核 → Bot 回复 → WebSocket 补偿”走一条完整链路，再主动说明 Redis、Python、MinIO 和 LLM 故障时的降级策略。更完整的问题清单、已知限制和不应使用的夸大表述见 [工程审查](docs/REVIEW.md) 与 [简历项目审查](docs/resume-review.md)。

## 快速开始

默认开发入口为 Homebrew 原生环境。Apple Silicon 用户可以按 [OrbStack 隔离试跑](docs/operations/runbook.md#orbstack-隔离试跑) 使用独立端口、卷和合成数据。

### Homebrew 原生启动

首次配置和依赖安装见 [用户使用指南](docs/USER_GUIDE.md)。

```bash
cp config.yaml.example config.yaml
cp .env.example .env
make bootstrap-postgres
make up
```

`make up` 会检测并启动本机依赖、启动 Go/AI 服务、执行 readiness 检查，并默认打开网页。不需要自动打开浏览器时使用 `SHAREO_OPEN_BROWSER=0 make up`。

默认 Go/AI 端口分别为 `8080` 和 `8000`。本机脚本把 `SHAREO_APP_PORT` 传给 Go，并用于探活和默认 AI 回调地址。直接启动 Go 时，显式 `SHAREO_APP_PORT` 优先于 `config.yaml` 的 `server.port`。若配置了显式 `SHAREO_AI_BASE_URL` 或 `SHAREO_AI_GO_BASE_URL`，请将它们与实际监听地址同步。

宿主机的全新图片数据集使用 `CONFIRM=YES make local-photo-seed`，该命令会先校验空业务库、备份并清空指定 MinIO bucket，再为本地图片目录中的 47 张图片创建固定测试用户帖子。

### OrbStack 隔离试跑

启动配置、健康检查和停止步骤见 [运行手册](docs/operations/runbook.md#orbstack-隔离试跑)。试跑已验证空库初始化、合成数据的发布与检索、私聊推送、Bot 故障恢复和重启持久性。Bot 使用固定响应服务，真实模型质量仍待复评。详见 [试跑记录](docs/REVIEW.md#2026-11-23-首轮优化与-orbstack-验证)。

### 验证

以下 API 和集成测试使用原生环境。

```bash
SHAREO_SKIP_CURRENT_EVIDENCE=1 make check
make test-integration-auto
make test-api
```

`make check` 还包含 Final Freeze 文档门禁；在没有新的权威运行号、三轮 AI 复评和稳定源码指纹时，它会按设计阻塞。代码改动的日常回归使用上面的 `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check`，不要通过删改证据文件绕过门禁。

## 文档

- [用户使用指南](docs/USER_GUIDE.md)：面向日常使用者的配置、`make up`、`make down` 和常见问题。
- [工程审查](docs/REVIEW.md)：问题清单、修复范围、当前指标和剩余边界。
- [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)：当前报告运行与历史对照的脱敏归档入口；权威运行指针和三轮复评仍待收口。
- [架构](docs/architecture.md)、[功能矩阵](docs/features.md)、[API 参考](docs/reference/api.md)、[数据模型](docs/reference/data-model.md)。
- [当前评测规范](docs/eval/README.md)与[运行记录](docs/eval/experiments.md)。
- [运行手册](docs/operations/runbook.md)、[ADR](docs/adr/)。
- [简历项目审查](docs/resume-review.md)：简历和答辩中的推荐表述。
- [MIT License](LICENSE)：项目开源许可。

历史过程由 Git 提交记录追溯；当前文档只保留稳定设计、当前状态和可复核证据。
