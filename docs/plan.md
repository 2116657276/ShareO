# ShareO 当前执行计划

> 更新时间：2026-07-26 | 状态事实源：[TASK.md](../TASK.md)

当前只执行 Phase 7，顺序固定为 7A → 7B → 7C。7A、7B、7C 均已形成自动测试、真实环境证据和文档记录；后续只处理明确的新需求或实验。

## 7A：Demo 数据与冻结评测集

1. 建立可重复的 Demo seed，数据 ID、对象键和审核状态可预测。
2. 准备覆盖完整类别的图文素材，并在全新卷中验证两次 seed 幂等。
3. 为 40 条搜图查询填写 `relevant_post_ids`；无匹配项显式标记。
4. 新建 30 条 RAG 问答，填写期望来源和参考答案。
5. 校验所有标注只引用 approved、未删除帖子。

退出门禁：`make demo-seed` 可运行，数据集数量和类别分布准确，普通查询不存在空标签。

## 7B：真实 DeepSeek 与质量评测

1. 修正统一评测命令的可信度问题：人工评分模板输出真实引用 ID，搜图门禁统一使用 Recall@5，门禁或请求失败时返回失败状态，并为来源命中、无答案、引用验证和门禁判断补充单元测试。
2. 使用当前源码和 Compose 配置执行 `make reset CONFIRM=YES`，清除项目数据卷并全新构建六个服务；保留 `.env`、源码修改和被忽略的本地照片。完成服务健康、图片检索 readiness 和真实 DeepSeek RAG readiness 检查。
3. 在全新数据库运行两次 `make demo-seed`，确认固定用户、Bot、26 篇帖子、图片对象和两类索引收敛且无重复；随后执行 40/30 数据集静态校验与数据库可见性校验。
4. 预热模型后运行完整 40 条搜图与 30 条 RAG 真实评测，输出脱敏 JSON 报告和人工评分表。报告记录提交 SHA、数据集版本、模型/revision、实际设备、检索参数、Prompt 版本、DeepSeek 模型、日期和网络环境。
5. 对 30 条 RAG 回答逐项进行 1–5 分人工相关性评分。未达门禁时只允许修正错误标签或素材映射、调整检索参数或 Prompt；不得降低阈值或删除失败查询，每轮变化和完整结果均写入实验记录。
6. 运行相关单元测试、数据校验和 `make check`，将最终报告归档到 `docs/eval/results/`。全部门禁通过且证据完整后，才在 `TASK.md` 和阶段文档中将 7B 标记完成。

退出门禁：搜图 Recall@5 ≥ 0.70、MRR ≥ 0.55；RAG 来源命中率 ≥ 0.80、引用可访问率 100%、虚假引用 0、人工相关性均分 ≥ 4.0/5。

当前执行状态：7B 已完成。六服务健康、图片和 RAG readiness、两次 seed 幂等、26/26/26 图片对象、两类 26 条索引、40/30 数据库可见性校验和完整评测均已核实；最终报告为 `docs/eval/results/phase7b_final.json`。人工评分 30 条，4 分 4 条、5 分 26 条，平均 4.8667。Phase 7C 复核报告见 `docs/eval/results/phase7c_revalidated_final.json`。

## 7C：发布与演示收口

1. 先运行 `make check`、真实集成、Go race、图片/Bot E2E，再进行冷启动和 readiness 复核。
2. 用独立 Compose 项目 `shareo-degradation` 逐项停止 AI、Qdrant、MinIO、Redis 和测试 LLM provider；每个场景都要求基线通过、注入故障、验证矩阵、恢复服务、基线再次通过。
3. 真实 DeepSeek 断开只按运行手册使用临时环境变量人工复核，不修改 `.env`，不输出 API Key；自动脚本使用确定性的测试 provider。
4. 若运行时代码、模型、数据集或 Prompt 变化，重新运行完整 40/30 评测；本次仅修复 Redis consumer 的 `NOGROUP` 恢复语义，因此已完成复核，未修改 Prompt、检索参数和阈值。
5. 按 [五分钟演示](demo.md) 完成计时演练，归档冷启动、故障、演示和质量报告的脱敏证据。
6. 更新 README、运行手册、架构、阶段索引、路线图、功能矩阵和实验记录，清理无效文件后创建唯一最终本地提交。

退出门禁：代码、自动测试、真实环境、40/30 评测、降级验证和文档证据全部齐全；Phase 4 与 Phase 7 同时标记完成。具体证据索引为 [`docs/evidence/phase7c/evidence-matrix.md`](evidence/phase7c/evidence-matrix.md)。

## Phase 7C 已验证结果

- 自动门禁、真实 MySQL/Redis/Qdrant 集成、Go race、图片 E2E、Bot E2E 均通过。
- 冷启动后六服务健康，图片/RAG readiness 为 HTTP 200；两次成功 seed 收敛为 26 篇帖子、26 个图片记录、78 个 MinIO 对象和两类各 26 条 Qdrant 索引。
- AI、Qdrant、MinIO、Redis、测试 LLM provider 五类故障均按矩阵验证，恢复后基线再次通过。
- 真实 DeepSeek provider 断开复核通过：AI health 仍为 200，RAG readiness/请求为 503，普通私聊为 200，Bot 固定兜底成功，恢复后 readiness 为 200。
- 40/30 复核保持 Recall@5 0.8125、MRR 0.7771、来源命中率 0.9667、引用可访问率 100%、虚假引用 0、失败请求 0、人工平均分 5.0。
- 五分钟演示实际耗时 5.08 秒，Bot 回复包含 2 条可访问引用。
