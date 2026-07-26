# ShareO 五分钟演示

> 当前状态：演示规范已冻结；Phase 7C 五分钟演练已完成

## 演示前检查

- 六服务状态正常。
- `/readyz/image-search` 返回200。
- 配置DeepSeek时 `/readyz/rag` 返回200。
- Demo seed已执行，管理员、普通用户、`shareo_bot`、图片和正文素材存在。
- 浏览器不展示API Key、内部token或后台管理密码。

## 五分钟脚本

| 时间 | 操作 | 要证明的能力 |
|---|---|---|
| 0:00–0:40 | 登录并浏览Feed | 社区基线、审核后内容和图片代理 |
| 0:40–1:30 | 发布图文并由管理员审核 | MySQL事务、MinIO、审核流和异步索引触发 |
| 1:30–2:20 | 用中文描述搜索图片 | Chinese-CLIP、Qdrant、每帖去重和公开可见性过滤 |
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

## Phase 7C 实际演练结果

2026-07-26 在现有六服务 Compose 环境完成一次计时演练，总耗时 5.08 秒。演示覆盖登录、Feed、临时图文发布与管理员审核、中文语义搜图、普通私聊、Bot 问答和降级说明；Bot 回复包含 2 条可访问帖子引用，临时发布内容已清理。完整脱敏记录见 [`docs/evidence/phase7c/demo-run.md`](evidence/phase7c/demo-run.md)。

## 证据要求

最终演练记录日期、提交SHA、Compose镜像、模型revision、DeepSeek模型、数据集版本、各门禁结果和总时长。截图不得包含密钥、token、Cookie或敏感请求头。
