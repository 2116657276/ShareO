# ShareO 当前状态

> 更新时间：2026-08-11 | 状态：PostgreSQL/pgvector 迁移和 47 张本机图片 seed 已完成；目标库与 MinIO 已通过最终核对

本轮按 [工程审查](docs/REVIEW.md) 完成入口修复、业务完整性、Bot outbox、评测门禁、搜图拒答阈值、Agent 安全拒答、本机浏览器 smoke，以及 Homebrew PostgreSQL 17 + pgvector 迁移。项目不做 Docker 交付、生产部署、高可用、压测或容量承诺。

已完成的关键修复包括：恢复 `start.sh` 与被删除的 Agent 数据集完整性测试；统一私聊未读字段为 `data.total`；校验评论父级同帖、可见和一级关系并级联软删除；帖子正文、图片数量和上传 URL 服务层校验；消息与 Bot outbox 同事务落库、启动补发和失败退避；登录后吊销旧 WebSocket；搜图默认置信度阈值；RAG/Agent 完整来源覆盖、引用精度、工具审计和注入拒答指标。

2026-08-10 纠正了本轮审查确认的四项问题：敏感请求现在只返回固定拒答，不再拼接未经验证的模型内容；评论列表和精选评论均复核所属帖子为 `approved AND is_deleted=0`；`.playwright-cli/` 被纳入忽略和源码指纹排除范围；失败或未达严格门禁的 Final Freeze 运行仍会保留，但不会替换既有权威运行指针。新增了对应的 Agent、证据工具和 PostgreSQL 集成回归测试。

2026-08-11 完成数据库方言替换：Go GORM 使用 PostgreSQL，Repository 移除 MySQL 全文/排序/自增方言；Python 通过 Psycopg 3 异步池使用 `ai.image_embeddings` 与 `ai.post_chunk_embeddings`，均为 `vector(512)` + HNSW cosine；Compose、MySQL、Qdrant 启动路径退役。新增受 `CONFIRM=YES` 保护的 MinIO 快照/清空和 47 张图片 seed，文案按 SHA-256 固化在未提交的 `.local/shareo/local-photo-seed/captions.json`。

## 当前基线

- 当前本机语料：47 条帖子、38 条搜图（含 8 条 no-match）、32 条搜贴、30 条 RAG、36 条 Agent。
- 历史 AI 报告运行号：`20260808T151704Z`；该报告的搜图、RAG、Agent 结果绑定 v1 数据集 SHA256、模型版本和运行时版本。它是在本次 PostgreSQL/pgvector 迁移前采集的诊断证据，不能当作当前迁移后的三轮复评。
- 当前结果是特定本机语料和真实 Provider 下的诊断证据，不代表线上 SLA、通用准确率或第三方评测。
- 2026-08-11 网络中断后重新验收：完整 `make test-api` 通过社区/API 回归 48 项和聊天、RAG、Agent 回归 15 项；`SHAREO_RUNTIME=local bash scripts/test_integration_current.sh` 通过 Go PostgreSQL 集成与 Python Redis/pgvector 集成 4 项；`go test -race -count=1 ./...` 和 `SHAREO_SKIP_CURRENT_EVIDENCE=1 make check` 均通过。完整 `make check` 仍会因历史 Final Freeze 权威运行号、三轮 AI 复评和稳定源码指纹未收口而阻塞，这不影响本次迁移验收。
- 当前本机最终数据：`shareo_test`、`shareo_bot` 两个用户；47 条 `approved AND is_deleted=0` 帖子且每条恰有一张图片；图片/正文向量各 47 条、均为 `vector(512)`，HNSW `vector_cosine_ops` 索引 2 个；评论、点赞、收藏、关注、通知、聊天、Bot outbox、系统日志均为 0；MinIO 当前 141 个对象（47 个原图及派生缩略图）；项目 `qdrant/` 目录已删除。固定测试用户密码和数据库密码只存在未跟踪的本机 `.env`，未写入仓库。

## 本轮剩余边界

干净提交复评、空缓存源码冷启动、完整移动端/实时 WebSocket 浏览器自动化、10 条 AI judge 待复核题的人工结论和压力测试尚未完成。AI 评分不表述为人工评分，dirty 工作区结果也不表述为发布复现结果。

## 证据入口

- [工程审查](docs/REVIEW.md)
- [用户使用指南](docs/USER_GUIDE.md)
- [Final Freeze 证据归档](docs/evidence/final-freeze/README.md)
- [当前评测规范](docs/eval/README.md)
- [当前运行记录](docs/eval/experiments.md)
- [AI judge 人工复核交接包](docs/evidence/final-freeze/ai-judge-human-handoff.json)
