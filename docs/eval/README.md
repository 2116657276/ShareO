# AI 评测规范

> 本文只维护数据规格、指标、评测输入输出和复核规则；当前状态与阻塞以 [`TASK.md`](../../TASK.md) 为准。

## 数据集

### 搜图 `image_search_v1.jsonl`

固定40条：

| 类别 | 数量 |
|---|---:|
| object | 8 |
| scene | 10 |
| color | 8 |
| style | 6 |
| composition | 4 |
| motion | 2 |
| no-match | 2 |

普通查询格式：

```json
{"query":"海边日落","category":"scene","relevant_post_ids":[12,18]}
```

无匹配查询额外使用 `"expect_no_results":true`。普通查询不得以空数组冒充标注完成。

### RAG `rag_qa_v1.jsonl`

Phase 7A创建30条：

| 类别 | 数量 |
|---|---:|
| factual | 10 |
| advice | 8 |
| comparison | 5 |
| multi-source | 4 |
| no-answer | 3 |

格式包含 `question`、`category`、`expected_source_post_ids`、`reference_answer`；无答案项显式使用 `expect_no_answer`。

## 指标与门禁

### 搜图

- Recall@5 ≥ 0.70。
- 同时输出 Recall@10 和 MRR，MRR ≥ 0.55。
- 重复帖子率为0。
- 待审、驳回和软删除帖子为0。
- 预热后MPS P95 ≤ 2秒、CPU P95 ≤ 4秒。

### RAG

- 来源命中率 ≥ 0.80。
- 引用可访问率100%，虚假引用为0。
- 人工回答相关性按1–5评分，平均 ≥ 4.0。
- 本地 embedding+检索目标 P95 ≤ 2秒；必须与 LLM 和队列等待分段测量。
- DeepSeek 总延迟记录P50/P95和超时率，不设硬门禁。

当前 `eval_ai` 只记录从发送问题到 Bot 回复的总延迟和失败请求数，没有输出 embedding、Qdrant 检索、队列等待与 LLM 的分段 P95，也没有单独统计超时类型。因此现有报告不能证明“本地检索 P95 ≤ 2秒”，`failed_queries=0` 也不能严格替代超时率字段。这两项在评测器补充仪表前属于待补证性能目标。

## Phase 8 Agent 评测规格

Agent 数据集固定在 [`agent_tasks_v1.jsonl`](agent_tasks_v1.jsonl)，共 36 条，覆盖单次检索 10 条、多步检索 8 条、比较 6 条、图片辅助 4 条、无答案 4 条和提示词注入 4 条。每条记录包含 `question`、`category`、`expected_source_post_ids`、`expected_tools` 和 `expect_no_answer`；其中 30 条标记为人工评分，另外两条无答案题和四条注入题只进入机器安全门禁。安全项只进入自动门禁，不用回答风格分数掩盖越权行为。

Agent 机器门禁为来源命中率 ≥ 0.80、必需工具选择率 ≥ 0.85、引用可访问率 100%、虚假引用 0、未知/禁止工具调用 0、预算越界 0、提示词注入成功 0、正常基线失败请求 0，预热后 P95 总延迟 ≤ 50 秒。30 条正常任务人工评分平均 ≥ 4.0/5，且不得出现低于 3 分的答案。评分维度为正确性、完整性、引用恰当性、语言自然度和步骤效率。

`make eval-agent` 先只生成机器报告并判断机器门禁。只有完整 36 条结果、失败请求为 0、所有机器指标通过后，才生成新的 30 条评分模板；机器失败时不生成或复用旧模板。人工复核通过 `REPORT_INPUT` 与 `SCORING_INPUT` 复用机器结果，不重复调用 provider。评分缺失或评分非法时命令返回失败。RAG 40/30 评测仍必须单独运行并保持原门禁。

测试 Provider 的源码跨服务验证已通过 36 条 Agent 机器门禁：来源命中率 0.9722、必需工具选择率 0.9167、引用可访问率 100%、虚假引用 0、无答案准确率 100%、P95 1133.5 ms；这不是 DeepSeek 质量结论。历史本机真实 DeepSeek 报告曾为 0.8889、0.6944 和 0.7778。最近一次本机工作区复评为 36 条无失败/超时，来源命中率 1.0000，必需工具选择率 0.9167，引用可访问率 100%，无答案准确率 100%，安全门禁均为 0，P95 13.39 秒。该报告不在仓库内且工作区为 dirty，需在干净提交上复评；旧报告和人工评分均保留为历史证据。

### 人工验收前自动化顺序

环境预检、主动预热、三类 readiness、确定性 E2E、故障恢复、评测失败语义、多 tool call 闭合、失败 state 保留和 AI 源码指纹一致性均已完成。Provider 对超时、网络错误、限流、服务端错误和结构化响应错误最多进行一次受控重试；最终失败仍计入 `failed_queries`，并记录脱敏失败类别和重试次数。有效真实评测已通过，后续进入 Go Template 前端补齐；浏览器和 Docker 仍不属于本阶段。

机器报告必须保留全部 36 条逐题证据，包括期望工具、实际工具、工具命中、期望来源、实际引用、失败类别、步骤、Provider 次数和耗时；同时分别输出全部任务和已完成任务工具选择率，以及未满足期望工具的任务 ID。机器门禁失败时人工模板仍不得生成。

机器门禁未全部通过时，只保存失败报告，不生成或复用人工评分模板。有效本机机器门禁已通过，但前端和非浏览器自动回归完成前不开始新的人工浏览器验收；最终阶段从新报告抽查 10 条 Agent 答案。上一轮 30 条人工评分平均 4.8667、最低 4，仍只作历史证据；完整历史评分表为 [`phase8a_native_agent_human_scoring_final.md`](results/phase8a_native_agent_human_scoring_final.md)，评分输入为 [`phase8a_native_agent_human_scores.json`](results/phase8a_native_agent_human_scores.json)。人工评分不能用机器指标替代；原模板的答案展示上限问题已作为评测器修复记录。

## 运行记录

每次评测记录提交SHA、数据集版本、Demo seed版本、模型/revision、Prompt版本、设备、DeepSeek模型、日期和网络环境。原始结果和汇总追加到 [实验记录](experiments.md)，不得包含API Key或敏感请求头。

mock provider只证明协议和幂等，不计入真实质量指标。修改查询、标签、Demo数据、模型、检索参数或Prompt时必须升版或新增实验记录。

## 评测命令

### 本机优先运行环境

日常评测默认使用宿主机 Go/Python，不构建 app/AI 镜像。运行前先执行：

```bash
make local-doctor
make local-infra-up
make dev-local
make warm-ai
```

本机使用 Homebrew 的 `mysql@8.0`、Redis、MinIO，以及从仓库下 `qdrant/` 目录启动的
Qdrant。Go/uv 依赖和 Hugging Face 模型优先复用宿主机缓存；代理仅用于缺失依赖或模型
下载，`127.0.0.1` 和本机基础服务不经过代理。完整 Compose 评测仍用于集成、故障矩阵和
源码冷启动，不能用本机通过替代 Compose 证据。

RAG 评测首次运行可以生成机器报告和人工评分模板；人工评分未完成时该次命令预期返回非零状态。Agent 评测必须先使用机器模式：

```bash
make eval-ai \
  OUTPUT=docs/eval/results/phase7b_raw.json \
  SCORING_OUTPUT=docs/eval/results/phase7b_human_scoring.md
```

填写 30 条评分后，只复用机器报告并复核全部质量门禁，不重复调用服务：

```bash
make eval-ai \
  REPORT_INPUT=docs/eval/results/phase7b_raw.json \
  SCORING_INPUT=docs/eval/results/phase7b_human_scoring.md \
  OUTPUT=docs/eval/results/phase7b_final.json
```

Agent 机器评测使用独立目标；该目标只判断机器门禁，不要求人工评分：

```bash
make eval-agent \
  MACHINE_ONLY=1 \
  OUTPUT=docs/eval/results/phase8a_agent_machine.json
```

本轮本机优先验证实际使用：

```bash
SHAREO_LOCAL_STATE_DIR="${SHAREO_LOCAL_STATE_DIR:-.local/shareo}" \
OUTPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_machine.json" \
  make eval-agent-machine

REPORT_INPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_machine.json" \
  OUTPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_machine_finalized.json" \
  SCORING_OUTPUT=docs/eval/results/phase8a_native_agent_human_scoring_final.md \
  make eval-agent
```

历史机器门禁通过后曾完成评分复核；该流程仅作为输入格式示例，不能作为当前验收结论：

```bash
REPORT_INPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_machine.json" \
  SCORING_INPUT=docs/eval/results/phase8a_native_agent_human_scores.json \
  OUTPUT="$SHAREO_LOCAL_STATE_DIR/phase8a_native_agent_final.json" \
  make eval-agent
```

`phase8_agent_raw.json`、`phase8_agent_human_scoring.md`、历史机器失败报告和历史评分均不是当前人工验收输入。当前代码重新取得机器门禁通过后，才生成新的验收材料；前端完成后用户抽查 10 条 Agent 答案。评分缺失、非法或机器报告被替换时，最终复核必须失败。

Agent 报告记录提交 SHA、数据集 SHA256、图片/正文模型与 revision、设备、工具检索候选数、Prompt/轨迹版本、DeepSeek 模型、P50/P95 和超时率。当前正文 embedding 没有单独锁定 revision，报告会诚实记录为 `unresolved`，不能将其写成已固定的模型 revision。

评分文件也可以是 `{"scores":[1,2,...]}` JSON；分数必须与报告中的已评测问题数量一致，且每项为 1–5 的整数。

Phase 7B 最终证据：机器报告为 `results/phase7b_final.json`，人工评分表为 `results/phase7b_human_scoring.md`。30 条回答中 26 条为 5 分、4 条为 4 分，平均 4.8667；机器门禁为 Recall@5 0.8125、MRR 0.7771、RAG 来源命中率 1.0000、引用可访问率 100%、虚假引用 0、失败请求 0。

Phase 7C 运行时变更后重新完成 40/30 评测，报告为 [`results/phase7c_revalidated_final.json`](results/phase7c_revalidated_final.json)，人工评分表为 [`results/phase7c_revalidated_human_scoring.md`](results/phase7c_revalidated_human_scoring.md)。本轮 Recall@5 0.8125、MRR 0.7771、来源命中率 0.9667、引用可访问率 100%、虚假引用 0、失败请求 0，30 条项目方人工评分均为 5 分。

两份 Phase 7C 最终报告均标记 `git_dirty=true`。此前报告的 `llm_model=deepseek-chat` 来自本地评测进程默认配置，与当时被测 Compose 环境的 `.env` 值 `deepseek-v4-flash` 不一致，因此这些报告只能证明当次质量结果，不能独立证明精确代码树或运行时模型版本。当前实现已增加 readiness 运行时元数据和源码指纹，但仍需在干净提交上重新评测后才能关闭该证据缺口。人工分数是项目方主观评分，不是盲评或第三方评测。
