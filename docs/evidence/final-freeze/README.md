# Final Freeze 自动化证据

本目录保存 ShareO 本轮本机自动化测试的脱敏汇总。统一入口为：

```bash
make final-evidence
```

原始命令输出和完整评测 JSON 保存于 `.local/shareo/final-freeze/<run_id>/`，不进入仓库证据目录。仓库内只保留运行清单、runtime snapshot、命令结果、质量指标、延迟摘要和失败矩阵。`run-manifest.json` 还包含四条关键链路的脱敏场景记录：状态、时间窗口、引用数量、工具序列、重试、失败类别和最终断言；临时帖子、消息和任务 ID 不进入仓库汇总。

本轮范围是本机 Go/Python 服务、Homebrew MySQL/Redis/MinIO、本地 Qdrant 和当前配置的真实 Provider。评测使用当前 47 条本机帖子语料：34 条搜图开发集、32 条搜贴开发集、30 条当前语料 RAG 题和 36 条当前语料 Agent 题；历史输入不参与本轮。Compose E2E 与故障注入不属于本轮范围，并在失败矩阵中标记为 `not_run`。

当前语料已完成两次真实 Provider 运行：`20260730T140713Z` 和 `20260730T143316Z`。两次使用相同数据集 SHA 和当前 47 条帖子；两次本机原始包均保留，最新 `143316Z` 作为现行基线，运行对照记录见 [`retained-runs.json`](retained-runs.json)。

## 关键链路

```text
发布 → 审核 → index_post → Redis Streams → Chinese-CLIP/FastEmbed → Qdrant → 搜索 → Go 可见性复核

私聊 → bot_tasks → RAG → 引用白名单 → Go 二次复核 → Bot 回复 → WebSocket

ai_mode=agent → LangGraph → 四类只读工具 → 脱敏轨迹 → 引用复核 → 回复
```

自动化记录测试状态、运行参数、数据集 SHA256、模型和 revision、Git SHA/dirty 状态、引用数量、工具序列、重试、失败类别以及可取得的 P50/P95。不得把 API Key、Token、Cookie、Authorization header、完整私聊正文或原始 Provider 请求写入本目录。

`latency-summary.json` 会区分逐题评测结果和从 AI 结构化日志中提取的运行窗口聚合结果；队列等待和回调耗时没有可靠日志字段时记录为 `unavailable`，不进行估算。
