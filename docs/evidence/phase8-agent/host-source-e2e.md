# Phase 8 Agent 当前验证证据

> 日期：2026-07-28 | 证据类型：源码跨服务、独立 Compose 与本机优先验证 | 性质：历史证据；不承担当前门禁状态，当前结论见 [`TASK.md`](../../../TASK.md)

## 验证边界

本记录包含两次隔离验证。第一部分使用未提交源码启动临时 Go、AI 和测试 Provider 进程，连接已有 Compose 基础设施中的 MySQL、Redis、MinIO 和 Qdrant。第二部分使用当前源码构建的 `shareo-agent-e2e-*` 镜像启动独立 Compose 项目，复用只读模型缓存卷 `shareo_hf_cache`。两次验证均使用备用端口和测试数据，结束后临时进程、容器和卷已清理，现有六个 Demo 服务未被关闭或重建。

这不是空模型缓存的冷启动证据，也不是 DeepSeek 真实质量结论。独立 Compose 源码构建阶段通过临时 Docker build proxy 完成依赖下载；运行时复用了已有模型缓存，因此没有把网络下载耗时或空缓存模型下载误计入 Agent 业务 E2E。

## 跨服务源码 E2E

| 场景 | 预期 | 实际结果 |
|---|---|---|
| Go `/healthz` | 返回 200 | 通过 |
| AI `/readyz` | Redis、Qdrant 可用 | 通过 |
| `shareo_bot` 深度分析 | 消息标记 `ai_mode=agent`，完成后含允许工具步骤和可访问引用 | 通过；`semantic_search_posts → read_posts`，引用帖子 `[14, 11, 15]` |
| 默认私聊 | 仍走固定 RAG，不产生 Agent 轨迹 | 通过；`ai_mode=rag` 且没有 `agent_trace` |

## 独立 Compose 源码 E2E

执行 `SHAREO_AGENT_E2E_NO_BUILD=1 SHAREO_AGENT_E2E_MODEL_CACHE_VOLUME=shareo_hf_cache make test-agent-e2e`，使用本轮刚由当前源码构建的三个测试镜像，并挂载已有模型缓存。启动、RAG/Agent readiness、帖子索引、显式 Agent 深度分析、引用可访问性和默认 RAG 回归全部通过；退出状态为 0。脚本退出时自动清理 `shareo-agent-e2e` 项目资源，现有 Demo Compose 保持健康。

本次构建使用了临时 Docker build proxy，将宿主机代理转换为 Docker 虚拟机可达地址。该地址只属于当次本机运行环境，没有写入仓库配置；其他环境必须按自身 Docker 网络配置传入代理，不能假定固定端口或地址。

验证查询为“西湖夜景的灯光有什么特点？”。工具步骤只记录工具名、状态、结果数量和耗时，不记录思维链、完整 Provider 请求或凭证。

## 36 条 Agent 机器评测

数据集为 `docs/eval/agent_tasks_v1.jsonl`，SHA256 为 `95256fb8005826820f5279dcee0bc90378390343e44ece6001248b50780c694d`。本轮使用 Python 3.12.13、测试 Provider、当前 Agent 配置（`agent-v1`、`agent-trace-v1`、最大 4 轮、6 次工具调用、45 秒总超时）；报告记录的 HEAD 为 `91d470feadf5b735cdbba4479e621474ff360c8f` 且工作区为 dirty，因此不是最终发布 SHA。

| 指标 | 实际值 | 门禁 |
|---|---:|---:|
| 完成/失败/超时 | 36 / 0 / 0 | 通过 |
| 来源命中率 | 0.9722 | ≥ 0.80 |
| 必需工具选择率 | 0.9167 | ≥ 0.85 |
| 引用可访问率 | 1.0000 | 100% |
| 虚假引用 | 0 | 0 |
| 无答案准确率 | 1.0000（4 条） | 100% |
| 禁止动作 / 预算越界 / 注入成功 | 0 / 0 / 0 | 全部 0 |
| 延迟 P50 / P95 | 1103.5 / 1133.5 ms | P95 ≤ 50 s |

测试 Provider 机器门禁全部通过；该结果只证明协议、工具白名单、引用校验和预算安全，不是 DeepSeek 质量结论。2026-07-28 本机优先环境的真实 DeepSeek 历史报告曾为 0.8889 和 0.7778；最近一次本机工作区复评为 36 条无失败/超时，来源命中率 1.0000、必需工具选择率 0.9167、引用可访问率 100%、无答案准确率 100%、P95 13.39 秒。该报告不在仓库内且工作区为 dirty，不能替代干净提交复评。当前门禁状态以 [`TASK.md`](../../../TASK.md) 为准。

## 后续门禁

Agent 故障恢复矩阵和上述真实评测是历史验证；模型缓存为空时的完整 Compose 源码下载冷启动、人工浏览器演示和干净提交复评仍需单独验证。本机 dirty 报告不作为最终发布证据。
