# 前端静态依赖

这些文件由 ShareO 自身的 `/static/vendor/` 路径提供，浏览器运行时不需要访问外部 CDN。
仅收录当前页面直接使用的固定版本，不包含开发工具或服务端依赖。

| 依赖 | 版本 | 文件 | SHA-256 | 许可证 |
| --- | --- | --- | --- | --- |
| Bootstrap | 5.3.3 | `bootstrap/css/bootstrap.min.css` | `3c8f27e6009ccfd710a905e6dcf12d0ee3c6f2ac7da05b0572d3e0d12e736fc8` | MIT，见 `bootstrap/LICENSE` |
| Bootstrap | 5.3.3 | `bootstrap/js/bootstrap.bundle.min.js` | `0833b2e9c3a26c258476c46266e6877fc75218625162e0460be9a3a098a61c6c` | MIT，见 `bootstrap/LICENSE` |
| Bootstrap Icons | 1.11.3 | `bootstrap-icons/font/bootstrap-icons.css` | `4ffa6bea4304d2eda418683f56261685ed47bf00995039f27e5ad62d53938d2d` | MIT，见 `bootstrap-icons/LICENSE` |
| Bootstrap Icons | 1.11.3 | `bootstrap-icons/font/fonts/bootstrap-icons.woff` | `bb1de989b83970f6f4e54de1cd974c5cba55b73582da5e1b225a6d0edf029483` | MIT，见 `bootstrap-icons/LICENSE` |
| Bootstrap Icons | 1.11.3 | `bootstrap-icons/font/fonts/bootstrap-icons.woff2` | `476adf42b40325098fcfa8b36ab3e769186bb4f6ce6a249753e2e1a9c22bf99e` | MIT，见 `bootstrap-icons/LICENSE` |
| Alpine.js | 3.14.1 | `alpinejs/alpine.min.js` | `358d9afbb1ab5befa2f48061a30776e5bcd7707f410a606ba985f98bc3b1c034` | MIT，见 `alpinejs/LICENSE` |

升级依赖时应同时更新文件、许可证、版本号和 SHA-256，并运行模板契约测试。不要在模板中恢复
`cdn.jsdelivr.net`、`unpkg.com` 等运行时外部引用。
