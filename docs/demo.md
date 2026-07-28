# ShareO 五分钟演示

> 本文维护后续人工验收所需的演示脚本和安全要求；执行时序与当前状态见 [`TASK.md`](../TASK.md)。

前端自动化阶段完成后的逐项人工验收步骤见 [`frontend-manual-acceptance.md`](evidence/phase8-agent/frontend-manual-acceptance.md)。该清单由用户执行后，才可进入 Docker/Compose 最终打包。

## 演示前检查

- 本机 Go、AI、Homebrew 基础服务和 Qdrant 状态正常；六服务 Compose 检查属于最终打包阶段。
- `/readyz/image-search` 返回200。
- API `GET /api/v1/search/images` 可用；独立 `/search/images` 页面已实现，完成非浏览器回归后纳入人工演示。
- 配置DeepSeek时 `/readyz/rag` 返回200。
- Demo seed已执行，管理员、普通用户、`shareo_bot`、图片和正文素材存在。
- 浏览器不展示API Key、内部token或后台管理密码。

## 五分钟脚本

| 时间 | 操作 | 要证明的能力 |
|---|---|---|
| 0:00–0:40 | 登录并浏览Feed | 社区基线、审核后内容和图片代理 |
| 0:40–1:30 | 发布图文并由管理员审核 | MySQL事务、MinIO、审核流和异步索引触发 |
| 1:30–2:20 | 在独立搜图页面用中文描述搜索图片 | Chinese-CLIP、Qdrant、每帖去重和公开可见性过滤 |
| 2:20–3:10 | 两名普通用户私聊 | REST落库、WebSocket、未读和实时下行 |
| 3:10–4:20 | 私聊 `shareo_bot` 提问 | 正文检索、DeepSeek回答、帖子引用和异步链路 |
| 4:20–5:00 | 打开引用并说明降级 | 引用可访问、Go二次校验、AI失败不影响普通路径 |

## 讲解主线

1. MySQL是业务真相，Qdrant是可重建派生索引。
2. Go负责业务与最终权限，Python负责模型与向量。
3. 两条Redis Stream把慢AI任务移出业务事务。
4. 模型输出不能直接成为引用，Python和Go执行双重白名单。
5. 项目用真实E2E和40/30评测证明能力，不以页面存在代替完成。

## 降级演示

- DeepSeek不可用：展示固定兜底，同时发送一条普通私聊证明IM正常。
- AI或Qdrant不可用：语义接口返回503，Feed和FULLTEXT仍可用。
- MinIO不可用：说明图片代理失败与数据库记录的区别，不删除数据伪装修复。

如果现场网络不稳定，允许使用已归档真实DeepSeek结果讲解质量，但不能把mock回答称为真实provider证据。

## Phase 7C 已有自动化记录

2026-07-26 在现有六服务 Compose 环境完成一次 API 驱动的计时冒烟，总耗时 5.08 秒。它覆盖登录、Feed、临时图文发布与管理员审核、语义搜图 API、普通私聊、Bot 问答和降级说明，Bot 回复包含 2 条引用，临时发布内容已清理。

该记录只证明接口链路和 Bot 等待时间，不证明独立搜图页面、浏览器页面操作、WebSocket 实时下行、引用点击可用性或人工讲解节奏，不能作为“五分钟人工演示已完成”的证据。自动化记录见 [`docs/evidence/phase7c/demo-run.md`](evidence/phase7c/demo-run.md)。

## 证据要求

最终演练记录日期、提交SHA、Compose镜像、模型revision、DeepSeek模型、数据集版本、各门禁结果和总时长。截图不得包含密钥、token、Cookie或敏感请求头。

人工演示完成后还应记录：浏览器与窗口环境、每个步骤的开始/结束时间、WebSocket 实时消息是否可见、Bot 引用是否实际点击并打开 approved 帖子，以及是否使用归档报告替代现场 provider。

## Phase 8 Agent 演示（代码已实现，门禁后演示）

演示时先展示默认 RAG，再在同一 `shareo_bot` 私聊中打开“深度分析”，提出需要多来源比较的问题。展示最终消息中的折叠步骤卡片、工具命中数量、耗时、引用点击和页面刷新后的轨迹恢复；随后演示无答案或 Provider 故障时的受控结果。真实 Provider 质量、故障矩阵和人工评分证据形成前，不把本节作为已完成发布证据。不要展示思维链、完整 Prompt、原始工具参数、Token 或任何个人图片。
