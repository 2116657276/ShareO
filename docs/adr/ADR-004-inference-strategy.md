# ADR-004: 模型推理策略 —— 本地 Embedding + 云 LLM，4060 为部署与对比实验平台

- 日期: 2026-07-19
- 状态: 已接受（强制 4060/多 provider 对比已被 [ADR-007](ADR-007-provider-evaluation.md) 替代）

> 本地 embedding + OpenAI-compatible LLM 的基本方向仍有效；当前发布只强制一次 DeepSeek 真实验收，Ollama 与 4060 对比不阻塞项目完成。

## 背景

可用硬件：开发机 MacBook Pro（M1 Pro，16GB 统一内存）；另有一台 RTX 4060（8GB 显存）机器。需要决定 embedding 与 LLM 分别跑在哪。约束：Bot 对话质量直接决定 demo 体验；毕设预算有限；需要积累真实的模型工程经验。

## 候选方案

1. 全云 API（embedding + LLM 都调 API）
2. 全本地（4060 跑一切）
3. 混合：本地 embedding + 云 LLM

## 决定

选方案 3：

- **Embedding 本地跑**：图文用 Chinese-CLIP（ViT-B/16，512 维），文本用 BGE 系列（bge-small-zh-v1.5 起步）。开发期在 M1 Pro（CPU/MPS）上随 worker 运行；部署期迁至 4060（CUDA）。
- **LLM 走云 API**：DeepSeek / Qwen 系（经 OpenAI 兼容接口），在 ai-service 内封装为可切换 provider。
- **4060 定位**：①最终 demo 部署目标机；②对比实验平台——Ollama 跑 Qwen2.5-7B-Instruct（Q4，约 4.7GB 恰好入 8G 显存），与云 API 在评测集上做质量/延迟/成本三维对比。

## 理由

- Embedding 模型小（Chinese-CLIP ViT-B/16 约 1.8 亿参数），M1 Pro 跑异步任务绰绰有余；本地跑才能积累模型加载、批处理、MPS/CUDA 部署的真实工程经验，这是 AI 毕设的核心含金量。
- LLM 质量差距肉眼可见：7B 本地模型作主力会拖累 Bot 体验；云 API 全项目费用约数十元，性价比压倒性。
- "硬件不强"转化为论文素材：M1 CPU vs 4060 CUDA 吞吐对比、本地 7B vs 云 API 对比，都是现成的实验章节。
- 全云方案（1）丢掉模型工程经验且 AI 含量降低；全本地方案（2）的对话质量不可接受。

## 后果与代价

- 依赖网络与 API 账号 → provider 抽象 + 本地 Ollama 作为降级备份，密钥走 `SHAREO_AI_*` 环境变量。
- 两种硬件后端（MPS/CUDA）→ 模型封装层统一 device 选择逻辑，属可控复杂度。
