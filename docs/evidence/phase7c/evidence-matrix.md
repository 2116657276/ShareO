# Phase 7C 已有证据矩阵

| 场景 | 命令/入口 | 预期行为 | 实际结果 | 恢复方式 | 证据 |
|---|---|---|---|---|---|
| 自动代码与静态门禁 | `make check` | Go、Python、Shell、文档检查通过 | 通过；85 个 Python 非集成测试通过 | — | 最终门禁记录 |
| Go 并发安全 | `go test -race ./...` | 无 race 或失败 | 通过 | — | 最终门禁记录 |
| 真实依赖集成 | `make test-integration` | MySQL、Redis、Qdrant 真实路径通过 | Go 与 Python 集成测试通过 | 临时测试库已删除 | 最终门禁记录 |
| 图片 Compose E2E | `make test-image-e2e` | 索引、重复投递、重启重领、删除收敛 | 通过 | 隔离项目自动清理 | 最终门禁记录 |
| Bot Compose E2E | `make test-ai-e2e` | 引用、幂等、LLM 失败兜底、普通私聊隔离 | 通过 | 隔离项目自动清理 | 最终门禁记录 |
| AI service 停止 | `make test-degradation` | 社区/全文正常，语义搜图受控失败 | HTTP 200 / 200 / 503 | 启动 AI，readiness 恢复 | 本表脱敏摘要 |
| Qdrant 停止 | `make test-degradation` | 普通路径正常，搜图/RAG 受控失败 | HTTP 200 / 200 / 503 / 503 | 启动 Qdrant，readiness 恢复 | 本表脱敏摘要 |
| MinIO 停止 | `make test-degradation` | 文字路径正常，图片代理失败且不伪造成功 | 全文 HTTP 200，代理 HTTP 404；恢复后 HTTP 200 | 启动 MinIO | 本表脱敏摘要 |
| Redis 停止 | `make test-degradation` | 业务消息落库不回滚，实时路径降级 | 社区/全文/普通消息 HTTP 200；恢复后 readiness 通过 | 启动 Redis；consumer 自动恢复消费组 | 本表脱敏摘要 |
| 测试 LLM provider 停止 | `make test-degradation` | 普通私聊正常，Bot 固定兜底 | 普通消息 HTTP 200，Bot 兜底通过 | 启动 mock provider，readiness 恢复 | 本表脱敏摘要 |
| 真实 DeepSeek provider 断开 | 临时环境变量覆盖 provider 地址 | AI 进程健康但 RAG 受控失败，普通私聊不受影响 | health HTTP 200，RAG readiness/请求 HTTP 503，普通消息 HTTP 200，Bot 固定兜底成功 | 恢复未覆盖的 Compose 配置并重新预热 | [`deepseek-disconnect.md`](deepseek-disconnect.md) |
| 新卷启动与幂等 seed | `make reset CONFIRM=YES`、两次 `make demo-seed` | 六服务健康，数据和索引收敛 | 26 帖子、26 图片记录、78 对象、两类索引各 26；源码构建未通过 | 使用已验证镜像启动新卷 | [`cold-start.md`](cold-start.md) |
| 40/30 质量复核 | `make eval-ai` | Phase 7B 门禁保持通过 | Recall@5 0.8125，MRR 0.7771，来源命中率 0.9667，引用可访问率 100%，虚假引用 0，人工平均 5.0 | — | [`phase7c_revalidated_final.json`](../../eval/results/phase7c_revalidated_final.json) |
| API 计时冒烟 | 自动 HTTP 流程 | 登录、审核、搜图、普通私聊和 Bot 链路可达 | 5.08s，Bot 返回 2 条引用 | 临时演示数据已清理 | [`demo-run.md`](demo-run.md) |
| 人工浏览器五分钟演示 | `docs/demo.md` | 页面操作、WebSocket 实时下行、引用点击和讲解在 5 分钟内完成 | 待补证 | — | 尚无证据文件 |

## 安全复核

归档内容不包含 API Key、Token 值、Cookie、Authorization header、个人照片、模型权重或模型缓存。评测报告只保留版本、参数、指标和脱敏运行元数据；模型缓存和容器数据备份留在仓库外。

## 证据限制

- Phase 7B/7C 最终评测报告均记录 `git_dirty=true`，不能仅凭 `git_sha` 精确重建被测代码树。
- Phase 7C 报告的 LLM 模型来自本地评测进程配置，不是被测 AI 容器元数据；报告中的 `deepseek-chat` 与实际 `.env` 的 `deepseek-v4-flash` 不一致。
- 新卷数据收敛使用已有镜像，不能证明从公共克隆完成源码构建。
- 5.08 秒记录是 API 自动化，不是人工浏览器演示。
