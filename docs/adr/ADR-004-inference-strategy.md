# ADR-004: 模型推理策略 —— 本地 Embedding + OpenAI-compatible LLM

- 日期：2026-07-19
- 状态：已接受；发布取舍以 [ADR-007](ADR-007-provider-evaluation.md) 为准

> 本 ADR 保留本地 embedding 与 OpenAI-compatible LLM 的技术方向。早期把 RTX 4060、Ollama 和多 provider 对比设为强制部署/验收条件的部分已被 ADR-007 替代。

## 当前有效决定

- 图片使用 Chinese-CLIP（ViT-B/16，512 维），正文使用 `BAAI/bge-small-zh-v1.5`，embedding 在本地运行并按 CUDA → MPS → CPU 选择设备。
- Bot 使用 OpenAI-compatible chat completions；真实运行所用 Provider 和模型由受控环境变量注入，并随 Final Freeze 运行清单记录，不把某次运行的模型名称固化为项目默认能力。
- API Key 通过 `SHAREO_AI_LLM_API_KEY` 注入；mock provider 仅用于确定性 E2E，不计入质量报告。
- Ollama、RTX 4060 和其他 provider 可以作为论文或性能实验，但不是发布前置条件。

## 历史取舍

混合方案保留本地模型工程能力，同时避免把低容量本地 LLM 作为 Bot 主力。早期文档曾把 4060 作为最终部署目标，并要求 Ollama 与云 API 做完整质量/延迟/成本对比；该部分属于可选实验，不应出现在当前任务门禁中。

## 后果与验证边界

- 本地 embedding 仍需要模型下载、缓存和设备性能记录；模型 revision、维度、设备和本地检索 P95 必须写入评测报告。
- DeepSeek 网络波动只记录 P50/P95 与超时率，不改变主站、普通私聊和非 LLM 能力的可用性要求。
- 40 条搜图和 30 条 RAG 的发布指标、人工评分和真实 provider 证据以 ADR-007 与 [评测规范](../eval/README.md) 为准。

当前实现说明：截至 2026-07-26，`eval_ai` 只记录 RAG 端到端总延迟，没有输出本地 embedding/检索分段 P95 或独立超时率。该实现缺口不改变本 ADR 的目标，但在补齐前不能宣称对应性能证据已完成。
