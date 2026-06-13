# TASK.md — 手动测试反馈修复计划

> 基于 2026-06-13 手动测试的 4 项反馈编写 | `go build` ✅ 为修改前提

---

## 问题 1: 最新/热门排序切换无区别

**文件**: `internal/repository/post_repo.go:73-76`, `internal/service/feed_service.go:47`

**根因分析**:

`FeedRepo.Feed()` 中热门排序逻辑：
```go
orderClause := "created_at DESC"
if q.Sort == model.SortHot {
    orderClause = "like_count DESC, created_at DESC"
}
```

当所有帖子的 `like_count` 均为 0 时，`like_count DESC, created_at DESC` 降级为纯 `created_at DESC`，与 "最新" 排序结果完全一致。这是测试数据缺乏差异性导致的，但生产环境中新帖也可能遇到同样问题。

**修改方案**: 热门排序增加 `view_count` 和 `comment_count` 作为加权二级排序，确保即使点赞数为 0 也能区分热度：

```go
// post_repo.go Feed() 方法中修改 orderClause:
if q.Sort == model.SortHot {
    // 加权热度: like(3) + comment(2) + view(1)，确保有区分度
    orderClause = "(like_count * 3 + comment_count * 2 + view_count) DESC, created_at DESC"
} else {
    orderClause = "created_at DESC"
}
```

**改动范围**: `internal/repository/post_repo.go:73-76`（1 处）

---

## 问题 2: 搜索功能点击无响应

**文件**: `web/templates/feed/feed.html:12-22`

**根因分析**:

搜索栏的 Alpine.js 作用域嵌套冲突：

```html
<!-- 父作用域: feedPage 组件，包含 doSearch() 等方法 -->
<div x-data="feedPage({{ .Page }}, {{ .Total }}, '{{ .Sort }}')">

    <!-- 子作用域: x-data="{ q: '' }" 创建独立作用域 -->
    <div class="search-bar mb-4" x-data="{ q: '' }">
        <input @keyup.enter="doSearch(q)" ...>   <!-- doSearch 在父作用域 -->
        <button @click="doSearch(q)" ...>搜索</button>
    </div>
</div>
```

Alpine.js 3.x 中，嵌套 `x-data` 会通过原型链合并父作用域数据，`doSearch` 理论上可见。但 `doSearch` 内部使用 `this.searchActive`、`this.renderCards()` 等引用 `feedPage` 组件属性。从子作用域的 `@click` 表达式调用时，Alpine 的 `this` 绑定可能指向子作用域对象，导致 `this.searchActive = true` 写入子作用域（无效），而父作用域的 `hasMore`、`searchHasMore` 等绑定 UI 的属性未更新，页面无反应。

同时，搜索按钮用 `x-show="q.length > 0"` 在输入为空时隐藏，用户必须先输入文字才能看到按钮，Enter 键也因作用域问题同样失效。

**修改方案**: 消除嵌套 `x-data`，将搜索状态统一纳入 `feedPage` 组件管理：

```html
<!-- 修改后: 移除内层 x-data，搜索状态由 feedPage 统一管理 -->
<div class="search-bar mb-4">
    <div class="input-group">
        <span class="input-group-text bg-white border-end-0 rounded-start-pill">
            <i class="bi bi-search text-muted"></i>
        </span>
        <input type="text" class="form-control border-start-0 rounded-end-pill"
               placeholder="搜索作品..."
               x-model="searchInput"
               @keyup.enter="doSearch(searchInput)"
               style="border-left:0">
        <button class="btn btn-primary rounded-pill ms-2 px-4"
                @click="doSearch(searchInput)"
                x-show="searchInput.length > 0">搜索</button>
    </div>
    <!-- ... 搜索状态提示保持不变 ... -->
</div>
```

同时在 `feedPage()` 函数中增加 `searchInput: ''` 属性，`doSearch()` 使用 `this.searchInput` 替代参数 `q`：

```javascript
function feedPage(currentPage, totalCount, sort) {
    return {
        // ... 现有属性 ...
        searchInput: '',   // 新增: 搜索输入框绑定

        async doSearch() {
            const q = this.searchInput.trim();
            if (!q) return this.clearSearch();
            this.searchActive = true; this.searchQuery = q; this.searchPage = 1;
            // ... 其余逻辑不变 ...
        },

        clearSearch() {
            this.searchActive = false; this.searchQuery = ''; this.searchTotal = 0;
            this.searchInput = '';  // 清空输入框
            this.page = 1;
            location.href = '/home?sort=' + this.tab;
        },
        // ...
    }
}
```

**改动范围**: `web/templates/feed/feed.html:12-22`（HTML 结构）、`feed.html` 中 `feedPage()` 函数（新增 1 属性 + 调整 `doSearch`/`clearSearch`）

---

## 问题 3: 通知铃铛缺少文字标签

**文件**: `web/templates/layout/header.html:28-33`

**根因分析**:

导航栏其他按钮均为 "图标 + 文字" 形式（"浏览"、"发布"、"管理"、"主页"、"退出"），唯独通知按钮只有铃铛图标无文字，视觉不统一，用户难以识别。

```html
<a class="nav-link position-relative" href="/notifications">
    <i class="bi bi-bell"></i>       <!-- 只有图标 -->
    <span class="badge ...">0</span>
</a>
```

**修改方案**: 在铃铛图标后添加"通知"文字，保持与其他导航项一致的格式：

```html
<a class="nav-link position-relative" href="/notifications">
    <i class="bi bi-bell"></i> 通知
    <span class="badge position-absolute top-0 start-100 translate-middle bg-danger rounded-pill"
          style="font-size:0.55rem;display:none" x-init="$el.style.display=''">0</span>
</a>
```

同时检查导航栏 CSS 是否因新增文字导致间距不均。当前导航使用 `justify-content-around`（Bootstrap flex），文字增加后自动等间距分布。

**改动范围**: `web/templates/layout/header.html:30`（1 行）

---

## 问题 4: 快速下滑图片频繁白屏，加载慢

**文件**: `web/templates/feed/feed.html:39-42`, `web/templates/feed/feed.html:89-90`

**根因分析**:

三方面原因叠加导致白屏：

**原因 A — 图片代理链路过长**:
```
浏览器 → Go Server (/api/v1/images/...) → MinIO → Go Server → 浏览器
```
每次图片请求走完整代理链路，无 CDN 加速。`ServeImage` 使用 `c.Stream` 手动 32KB 缓冲区循环传输，对并发请求的处理效率不如直接静态文件服务。

**原因 B — `loading="lazy"` 加载延迟**:
```html
<!-- 仅前 6 张 eager，其余全部 lazy -->
<img src="..." {{ if lt $idx 6 }}loading="eager"{{ else }}loading="lazy"{{ end }}>
```
浏览器 `loading="lazy"` 的阈值是距视口约 1 屏时开始加载。Feed 页快速滚动时，图片滚入视口才开始发起网络请求（走完整 MinIO 代理链路），此时用户看到的仍是 `skeleton-bg` 闪烁动画 = "白屏"。

LazyLoad 动态插入的卡片 100% 使用 `loading="lazy"`：
```javascript
// renderPostCard() 中:
<img src="${thumbUrl(post.cover_image)}" loading="lazy" ...>
```
快速连续加载多页时，大量图片同时触发 lazy load → 竞争带宽 → 更慢。

**原因 C — 图片尺寸未预留**:
缩略图容器用 `padding-top: 100%` 创建 1:1 占位，但 `<img>` 缺少 `width`/`height` 属性，浏览器无法在图片加载前计算精确布局，Layout Shift + skeleton 闪烁进一步加剧 "白屏" 感知。

**修改方案**:

**4a. 图片代理增加强缓存头** (`internal/handler/upload_handler.go:110-111`):
当前已有 `Cache-Control: public, max-age=86400`（24h），确认生效。追加 `immutable` 指令避免 `If-Modified-Since` 验证请求：
```go
c.Header("Cache-Control", "public, max-age=86400, immutable")
```

**4b. 扩大 eager 加载范围** (`feed.html` 模板):
前 6 张 → 前 12 张（约首屏 + 第二屏），减少首屏 lazy 等待：
```html
{{ if lt $idx 12 }}loading="eager"{{ else }}loading="lazy"{{ end }}
```

**4c. LazyLoad 动态卡片增加预加载** (`feed.html` renderPostCard 函数):
将动态插入 `<img>` 的 `loading="lazy"` 改为 `loading="eager"`，因为用户主动点击"加载更多"说明这些图片即将进入视口：
```javascript
// renderPostCard 中:
<img src="${thumbUrl(post.cover_image)}" loading="eager" decoding="async" ...>
```

**4d. 增加 `fetchpriority` 提示** (`feed.html` 模板):
前 4 张缩略图标记为高优先级，告诉浏览器优先下载：
```html
<img src="{{ thumbURL $p.CoverImage }}" 
     {{ if lt $idx 4 }}fetchpriority="high"{{ end }}
     ...>
```

**4e. 增加 `decoding="async"` 确保所有图片异步解码**:
模板中已对 eager 图片设置 `decoding="async"`，确认 LazyLoad 动态卡片同样设置。

**改动范围**:
| 文件 | 行号 | 改动 |
|------|------|------|
| `internal/handler/upload_handler.go` | 111 | Cache-Control 追加 `, immutable` |
| `web/templates/feed/feed.html` | 40 | eager 范围 6→12, 前4张加 fetchpriority |
| `web/templates/feed/feed.html` | 89 | renderPostCard 中 loading 改为 eager |

---

## 修改约束

1. **仅修改以上列出的文件和行号范围**
2. **禁止新增 `go.mod` require 依赖**
3. **每次修改后必须 `go build -o bin/shareo cmd/server/main.go` 确认零错误**
4. **禁止在代码中硬编码密码、token、密钥**
5. **修改完成后在 `feedback.md` 底部标注修复状态**
