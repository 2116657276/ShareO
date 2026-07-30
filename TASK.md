# TASK.md — 当前任务

> 更新时间：2026-07-30 | 状态：完工冻结 | 任务：功能实现已完成，证据缺口已接受

## 当前状态

项目功能实现已完成并冻结。正文搜贴已完成“语义为主、词面加分”的混合排序；搜贴只读取正文，搜图只读取图片向量，两类结果不交叉融合。后续不再安排业务整改、模型调参或发布补证，完工口径、证据等级和已接受缺口统一见 [完工冻结审查](docs/REVIEW.md)。

- [x] API 边界、聊天/RAG/Agent 自动化、readiness、源码指纹和失败语义已补齐。
- [x] 本次当前语料 Final Freeze 已完成：36 条 Agent 机器评测来源命中率和必需工具选择率均为 `1.0`，结果入口见 [Final Freeze 自动化证据](docs/evidence/final-freeze/README.md)。
- [x] Go Template 页面和 `/search/images` 统一搜索工作台功能已实现，并通过模板、Handler、静态契约和本机自动检查。
- [x] 公共壳层、移动/桌面导航、Feed、关注页、统一搜索和各业务页面已完成全量 UI 统一重构。
- [x] 当前本机语料已收敛为 47 条 approved 帖子；图片和正文向量各 47 条，索引 pending 为 0，并作为本次当前评测的真实输入。
- [x] 用户已完成一轮浏览器人工验收；核心功能总体可用，但指出新功能与旧页面缺少统一 UI，部分页面视觉不协调。
- [x] UI 重构后的模板/路由契约、`make check`、Go race、本机 API 52 项和真实 MySQL/Redis/Qdrant 集成测试通过。
- [x] 旧 Demo、旧评测输入和旧报告已清理，当前命令不再回退到这些文件。
- [x] 当前 47 条本机语料已建立为唯一现行评测范围：34 条搜图、32 条搜贴、30 条 RAG 和 36 条 Agent；最新运行搜图 Recall@5 `0.9222`、RAG 来源命中率 `0.8333`、Agent 来源命中率与工具选择率均为 `1.0`，完整结果见 [Final Freeze 自动化证据](docs/evidence/final-freeze/README.md)。
- [x] 五项主导航、关注动态、私聊列表、高清中图、页脚和毛玻璃视觉优化已实现；新上传中图使用 JPEG 90% 质量，本机 API 回归 41 项、聊天与 AI API 回归 15 项通过；完整浏览器演示记录作为已接受证据缺口归档。
- [x] 内置浏览器实测确认 1200px 中图均可正常加载；Bootstrap、Bootstrap Icons 与 Alpine.js 已按现用版本本地化，页面基础样式、图标和交互不再依赖外部 CDN。
- [x] Feed 图片悬停缩放增强为轻量 `1.045`；评论作者可进入个人主页；关注页已移除“全部关注”展开入口。
- [x] 搜索页增加“问问小O”入口并直接打开 `shareo_bot` Agent 会话；私聊改为严格的“会话列表 → 单一会话 → 返回列表”信息架构，列表只显示已有消息的会话。
- [x] 私人收藏完成数据库迁移、幂等接口、帖子详情按钮与“我的”收藏 Tab；收藏仅本人可见、不通知作者、不公开数量。
- [x] 通知列表只展示关注、点赞、评论事件；评论未读只在“我的”显示红点，私聊未读只在“私聊”显示数量。
- [x] 已完成语义搜图实现审计与准确率提升调研，形成分级实验建议；未修改模型、Prompt、数据集、检索参数或阈值。
- [x] 第二轮 UI 的模板契约、Go 全量与 race、`make check`、真实本机集成、API 48 项和聊天/AI 15 项通过；内置浏览器已验证桌面与 390px 移动布局，不替代用户人工验收。
- [x] 第三轮 UI 复验整改：“我的”内容 Tab 与布局切换同排；桌面侧栏删除重复账户头像；普通私聊列表隐藏 `shareo_bot`，仅允许“问问小O”入口打开专属 Agent 会话；Feed 悬停缩放调整为 `1.09`；已完成模板、路由契约和本机页面复核。
- [x] 用户确认第三轮四项视觉整改效果符合预期；这只关闭对应 UI 问题，不替代最终全功能人工验收。
- [x] 完成 BGE/Qdrant 正文语义召回、MySQL ngram/LIKE 关键词召回、固定权重融合、可见性复核和关键词降级；公开接口仍为 `GET /api/v1/search`。
- [x] 建立 32 条本机搜贴开发集并对比三路：hybrid Recall@5 `0.8906`、MRR `0.9688`、nDCG@5 `0.9158`、Precision@3 `0.5833`、无匹配准确率 `1.0`，首版门禁通过。该集合参与过阈值检查，不冒充最终盲测集。
- [x] 为 47 图建立 34 条本机 graded relevance 开发集；零阈值基线 Recall@5 `0.9222`、MRR `0.9750`、nDCG@5 `0.9419`、Precision@3 `0.3778`、无匹配准确率 `0`。
- [x] 离线比较图片分数阈值 `0.36–0.40`：`0.38/0.39` 可使开发集无匹配准确率达到 `1.0` 且相关查询指标不变，但相关查询 nDCG@5 未提升 `0.03`，因此本轮不改生产搜图阈值。
- [x] 本轮 `go test -race ./...`、`make check`、`make warm-ai` 与 `make test-integration-auto` 通过。
- [x] Docker/Compose 冷启动、故障注入和完整浏览器演示不属于本次本机冻结范围，证据限制见 [完工冻结审查](docs/REVIEW.md)。
- [x] UI 重构后的浏览器人工逐项记录、10 条 Agent 答案抽查、干净源码 Compose 冷启动和最终发布复评不再作为当前整改任务，统一记录为已接受证据缺口。

## 固定边界

Go 对外提供 `GET /api/v1/search/images`；AI 内部使用 `POST /v1/search/images` 和 `POST /v1/rag/answer`；Agent 继续通过 `shareo_bot` 私聊中的 `ai_mode=agent` 触发，不新增公开 RAG/Agent API。

日常开发使用宿主机 Go/Python、Homebrew MySQL/Redis/MinIO 和本地 Qdrant。Docker/Compose 仅作为可选的最终打包与复核方式，不构成当前完工结论的必要条件。不得重置本机数据、提交 `.env`、配置凭证、个人照片、模型缓存、原始响应或未脱敏报告。

## 已完成内容

## 历史结果

开发阶段的实现、评测和门禁结果已经收口；旧数据集与旧报告不再作为当前输入。需要追溯过程时使用 Git 历史，不在当前任务中恢复阶段计划或旧测试入口。

## 完工冻结后的不执行事项

以下事项不再作为 ShareO 当前版本的执行计划：源码冷启动、完整浏览器人工演示、干净提交复评、RAG 分段延迟补测、搜图继续调参和 Docker/Compose 最终发布复核。它们的证据边界及对简历表述的影响见 [完工冻结审查](docs/REVIEW.md)。

## 证据入口

- 完工冻结结论：[`docs/REVIEW.md`](docs/REVIEW.md)
- API 与运行规则：[`docs/reference/api.md`](docs/reference/api.md)、[`docs/operations/runbook.md`](docs/operations/runbook.md)
- 当前评测规则与运行记录：[`docs/eval/README.md`](docs/eval/README.md)、[`docs/eval/experiments.md`](docs/eval/experiments.md)
