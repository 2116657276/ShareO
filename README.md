# ShareO

ShareO 是一个面向毕业答辩和简历展示的 Go + Python 全栈社区项目。它把图文社区、一对一实时私聊、中文搜贴、中文语义搜图、带可访问引用的 RAG Bot 和受限只读知识 Agent 收在同一套业务链路中。

日常启动、关闭和 Make 用法见 [用户使用指南](docs/USER_GUIDE.md)。工程结论、证据等级和剩余边界见 [工程审查](docs/REVIEW.md)；脱敏运行结果见 [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)。项目不宣称通用 Agent、多 Agent、生产级高并发、线上 SLA 或公共克隆一键复现完整图片质量评测。

## 项目范围

已实现的业务能力包括：

- 注册登录、资料、图文发布、管理员审核、Feed、点赞、关注、评论、通知和私人收藏。
- WebSocket 一对一私聊，支持消息落库、未读、断线补偿和 Bot 回复推送。
- 正文混合搜贴：BGE/pgvector 语义召回与 PostgreSQL `LIKE` 关键词召回独立融合。
- Chinese-CLIP + pgvector 中文语义搜图，公开结果由 Go 做最终可见性复核。
- 默认私聊 Bot 固定走 RAG；用户显式开启深度分析后，才进入 LangGraph 只读 Agent。
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

## 快速开始

环境要求和完整启动说明见 [用户使用指南](docs/USER_GUIDE.md)。首次运行可执行：

```bash
cp config.yaml.example config.yaml
cp .env.example .env
make bootstrap-postgres
make up
```

`make up` 会检测并启动本机依赖、启动 Go/AI 服务、执行 readiness 检查，并默认打开网页。不需要自动打开浏览器时使用 `SHAREO_OPEN_BROWSER=0 make up`。

本机运行只使用 Homebrew PostgreSQL 17、pgvector、Redis 和 MinIO；不会启动或读取旧 MySQL、Qdrant、Colima、OrbStack 或 Compose 数据。全新图片数据集使用 `CONFIRM=YES make local-photo-seed`，该命令会先校验空业务库、备份并清空指定 MinIO bucket，再为本地图片目录中的 47 张图片创建固定测试用户帖子。

基础检查：

```bash
make check
make test-integration-auto
make test-api
```

## 文档

- [用户使用指南](docs/USER_GUIDE.md)：面向日常使用者的配置、`make up`、`make down` 和常见问题。
- [工程审查](docs/REVIEW.md)：问题清单、修复范围、当前指标和剩余边界。
- [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)：当前报告运行与历史对照的脱敏归档入口；权威运行指针和三轮复评仍待收口。
- [架构](docs/architecture.md)、[功能矩阵](docs/features.md)、[API 参考](docs/reference/api.md)、[数据模型](docs/reference/data-model.md)。
- [当前评测规范](docs/eval/README.md)与[运行记录](docs/eval/experiments.md)。
- [运行手册](docs/operations/runbook.md)、[ADR](docs/adr/)。
- [简历项目审查](docs/resume-review.md)：简历和答辩中的推荐表述。

历史过程由 Git 提交记录追溯；当前文档只保留稳定设计、当前状态和可复核证据。
