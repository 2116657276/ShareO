# ShareO 普通用户测试报告

> 测试时间: 2026-06-12  
> 方式: 纯 API 模拟，使用 resources/static/pictures/ 真实照片  
> 流程: 上传 → 发帖(pending) → 管理员审批 → Feed可见 → 社交互动

---

## 结果: 66 项全部通过 ✅

---

## 一、发帖明细 (一图一帖，9张真实照片)

| 帖子ID | 用户 | 图片 | 大小 | 内容 |
|--------|------|------|------|------|
| 1 | test03 | IMG_0102.jpeg | 29.8MB | 清晨古镇石板路 🌅 #街拍 |
| 2 | test02 | IMG_0203.jpeg | 24.1MB | 雨后山间云雾 ⛰️ #山景 |
| 3 | test07 | IMG_0224.jpeg | 67.0MB | 城市霓虹夜景 🏙️ #夜景 |
| 4 | test05 | IMG_0284.jpeg | 63.7MB | 周末山顶徒步 🥾 #户外 |
| 5 | photofan_03 | IMG_0285.jpeg | 65.6MB | 晒太阳的小猫 🐱 #宠物 |
| 6 | photofan_01 | IMG_0432.jpeg | 34.9MB | 春日花卉微距 🌸 #花卉 |
| 7 | photofan_02 | IMG_6122.jpeg | 5.0MB | 海边日落 🌊 #海边 |
| 8 | test04 | IMG_9008.jpeg | 9.8MB | 老建筑记忆 📷 #建筑 |
| 9 | test06 | IMG_9009.jpeg | 34.9MB | 咖啡厅午后 ☕ #生活 |

---

## 二、审批流验证

```
用户上传图片 → POST /api/v1/upload → MinIO 存储 → 返回 /api/v1/images/... URL
     ↓
POST /api/v1/posts {cover_image, images, content, topic_tags} → status=pending
     ↓
管理员 POST /api/v1/admin/posts/:id/approve → status=approved
     ↓
Feed 可见性: 9/9 ✅
```

---

## 三、MinIO 图片内容验证

| 帖子 | 图片 | Content-Type | 大小 | 可渲染 |
|------|------|-------------|------|--------|
| 1 | IMG_0102.jpeg | image/jpeg | 30488KB | ✅ |
| 2 | IMG_0203.jpeg | image/jpeg | 24679KB | ✅ |
| 3 | IMG_0224.jpeg | image/jpeg | 68649KB | ✅ |
| 4 | IMG_0284.jpeg | image/jpeg | 65254KB | ✅ |
| 5 | IMG_0285.jpeg | image/jpeg | 67192KB | ✅ |
| 6 | IMG_0432.jpeg | image/jpeg | 35730KB | ✅ |
| 7 | IMG_6122.jpeg | image/jpeg | 5128KB | ✅ |
| 8 | IMG_9008.jpeg | image/jpeg | 10036KB | ✅ |
| 9 | IMG_9009.jpeg | image/jpeg | 35756KB | ✅ |

---

## 四、社交互动

| 类型 | 次数 |
|------|------|
| 点赞 | 49 (12用户交叉点赞) |
| 评论 | 25 (含7条二级回复) |
| 收藏 | 25 |
| 关注 | 24 |

---

## 五、功能验证

| 模块 | 测试 | 结果 |
|------|------|------|
| 登录 | 12用户+1管理员 | ✅ |
| 上传 | 9张(5-67MB)全部成功 | ✅ |
| 发帖 | 9篇, 初始status=pending | ✅ |
| 审批 | 9篇全部approve | ✅ |
| Feed可见 | 9/9 全部在Latest Feed中 | ✅ |
| Latest Feed | total=9, 时间降序 | ✅ |
| Hot Feed | total=9, 热度降序 | ✅ |
| 搜索 | "街拍"1条/"海边"1条 | ✅ |
| 收藏列表 | API正常 | ✅ |
| Bug1回归 | Feed无参数200 | ✅ |
| Bug2回归 | 点赞不存在帖400 | ✅ |
| Bug3回归 | 删不存在评论400 | ✅ |
| 权限 | 普通用户→admin API→403 | ✅ |
| 权限 | 编辑他人帖→400 | ✅ |

---

## 六、平台统计

| 指标 | 数值 |
|------|------|
| 用户总数 | 70 |
| 帖子总数 | 9 (全部approved) |
| 待审核 | 0 |
| 总点赞 | 49 |
| 总评论 | 32 |

---

## 七、结论

✅ 66项测试全部通过。9张真实照片经完整审批流后在Feed中正常展示，MinIO图片全部可渲染，社交互动正常，权限边界完整。
