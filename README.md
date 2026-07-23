# ShareO — Go 社区与 AI 检索项目

ShareO 是面向简历展示与毕业答辩的轻量全栈项目，完整主线包括图文社区、一对一实时私聊、中文语义搜图和带帖子引用的 RAG Bot。项目强调可复现启动、清晰数据所有权、受控降级和量化评测。

> 当前状态：Phase 0–3、5–6 已完成；Phase 4 工程闭环完成但质量评测待验；Phase 7 Demo、评测与发布收口进行中。实时状态见 [TASK.md](TASK.md)。

## 核心能力

- 注册登录、图文发布、Feed、全文搜索和管理员审核。
- 点赞、关注、评论与通知。
- WebSocket 一对一私聊、准确未读和断线恢复。
- Chinese-CLIP + Qdrant 中文语义搜图。
- FastEmbed + Qdrant + DeepSeek/OpenAI-compatible LLM 的引用式 RAG Bot。

收藏、转帖、话题、群聊和 Agent 已从最终范围移除。

## 架构

Go 负责全部业务和 MySQL写入，Python FastAPI单进程负责模型、Qdrant和两个Redis Streams consumer。MinIO凭证只由Go持有，Bot引用由Python候选白名单和Go可见性二次校验。

详细说明见 [架构文档](docs/architecture.md) 和 [单上下文](CONTEXT.md)。

## 快速开始

环境要求：Go 1.25.1+、Python 3.12、uv、Docker Compose。

```bash
cp config.yaml.example config.yaml
export SHAREO_INTERNAL_TOKEN='replace-with-a-random-value'
make up
docker compose ps
```

首次启动会下载 Chinese-CLIP 和 FastEmbed模型。检查基础服务：

```bash
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8000/healthz
curl --fail -H "X-Internal-Token: $SHAREO_INTERNAL_TOKEN" \
  http://127.0.0.1:8000/readyz/image-search
```

没有 LLM Key时 RAG readiness返回503是预期行为，不影响社区、普通私聊或语义搜图。DeepSeek配置和故障排查见 [运行手册](docs/operations/runbook.md)。

## 已可用命令

| 命令 | 说明 |
|---|---|
| `make up` / `make down` | 启停六服务 |
| `make reset CONFIRM=YES` | 删除数据卷并重建 |
| `make check` | Go、Python、Shell和文档门禁 |
| `make test-integration` | 真实MySQL/Redis集成 |
| `make test-image-e2e` | 真实图片索引E2E |
| `make test-ai-e2e` | mock LLM私聊Bot跨服务E2E |
| `make backfill-index` | approved帖子索引回填 |
| `make reconcile-index` | 图文索引对账，默认dry-run |

`make demo-seed` 和 `make eval-ai` 是 Phase 7交付；当前目标会明确返回“尚未实现”，不能作为已有能力使用。

## 项目文档

- [当前任务](TASK.md)与[执行计划](docs/plan.md)
- [Phase 0–7](docs/phases/README.md)
- [API参考](docs/reference/api.md)与[数据模型](docs/reference/data-model.md)
- [AI评测](docs/eval/README.md)与[五分钟演示](docs/demo.md)
- [技术决策](docs/adr/)

## 演示主线

用户发布图文并经管理员审核，`index_post` 异步建立图片和正文索引；用户可以用中文描述搜图，也可以私聊 `shareo_bot`，由Bot根据已审核正文回答并返回可访问帖子引用。
