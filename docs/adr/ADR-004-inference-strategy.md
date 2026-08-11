# ADR-004: 模型推理策略 —— 本地 Embedding + OpenAI-compatible LLM

- 日期：2026-07-19
- 状态：已接受；发布取舍以 [ADR-007](ADR-007-provider-evaluation.md) 为准

> 本 ADR 保留本地 embedding 与 OpenAI-compatible LLM 的技术方向。早期把 RTX 4060、Ollama 和多 Provider 对比设为强制部署/验收条件的部分已被 ADR-007 替代。

## 当前有效决定

- 图片使用 Chinese-CLIP（ViT-B/16，512 维），正文使用 `BAAI/bge-small-zh-v1.5`，embedding 在本地运行并按 CUDA → MPS → CPU 选择设备。
- Bot 使用 OpenAI-compatible chat completions；真实运行所用 Provider 和模型由受控环境变量注入，并随 Final Freeze 运行清单记录，不把某次运行的模型名称固化为项目默认能力。
- API Key 通过 `SHAREO_AI_LLM_API_KEY` 注入；mock Provider 仅用于确定性 E2E，不计入质量报告。
- Ollama、RTX 4060 和其他 Provider 可以作为论文或性能实验，但不是发布前置条件。

## 历史取舍

混合方案保留本地模型工程能力，同时避免把低容量本地 LLM 作为 Bot 主力。早期文档曾把 4060 作为最终部署目标，并要求 Ollama 与云 API 做完整质量/延迟/成本对比；该部分属于可选实验，不应出现在当前任务门禁中。

## 后果与验证边界

- 本地 embedding 仍需要模型下载、缓存和设备性能记录；模型 revision、维度、设备和本地检索 P95 必须写入评测报告。
- DeepSeek 网络波动记录端到端 P50/P95、可取得的分段耗时与超时率，不改变主站、普通私聊和非 LLM 能力的可用性要求。
- 当前报告中的 38/32/30/36 指标、Provider 和运行边界以 ADR-007 与 [评测规范](../eval/README.md) 为准；质量答案采用两轮 AI judge，10 条低分或不确定样本进入人工复核交接清单，不表述为人工评分。

当前证据边界：Final Freeze 已记录实际 Provider、模型、数据集 SHA、Git/dirty 状态和可取得的 embedding、检索、工具及 LLM 阶段耗时；队列等待、postprocess、callback 或缺少样本的阶段明确标记为 `unavailable`。当前运行仍是 dirty 工作区，10 条 AI judge 样本待人工复核。
