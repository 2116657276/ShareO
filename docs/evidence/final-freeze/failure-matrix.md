# Final Freeze 自动化结果

> 归档属性：历史运行快照（对应旧的本机采集批次），不是当前运行结果。表中的 `check=pass` 仅表示采集当时的命令状态；当前文档门禁仍以 `python3 scripts/check_docs.py` 的实时输出为准。原始命令输出位于 `.local/shareo/final-freeze/<run_id>/`，不进入仓库证据目录。

| 场景 | 状态 | 退出码 | 说明 |
|---|---|---:|---|
| `local-doctor` | `pass` | 0 |  |
| `up` | `pass` | 0 |  |
| `warm-ai` | `pass` | 0 |  |
| `prepare-current-ai-eval` | `pass` | 0 |  |
| `check` | `pass` | 0 |  |
| `go-race` | `pass` | 0 |  |
| `integration` | `pass` | 0 |  |
| `local-stack` | `pass` | 0 |  |
| `api` | `pass` | 0 |  |
| `post-search-eval` | `pass` | 0 |  |
| `image-search-local-eval` | `pass` | 0 |  |
| `eval-ai-real-provider` | `pass` | 0 |  |
| `eval-agent-real-provider` | `pass` | 0 |  |
| `ai-judge` | `needs_human_review` | 2 |  |
| `worktree-manifest` | `pass` | 0 |  |
| `compose-image-e2e` | `not_run` | — | local-only scope；Compose 图片链路未纳入本轮 |
| `compose-bot-e2e` | `not_run` | — | local-only scope；Compose Bot 链路未纳入本轮 |
| `compose-agent-e2e` | `not_run` | — | local-only scope；Compose Agent 链路未纳入本轮 |
| `compose-fault-injection` | `not_run` | — | local-only scope；Compose 故障注入未纳入本轮 |
