/* Shared feed-card renderer and horizontal media carousel. */
(function () {
    'use strict';

    function escapeHTML(value) {
        const node = document.createElement('div');
        node.textContent = value == null ? '' : String(value);
        return node.innerHTML;
    }

    function displayImageURL(url) {
        return (url || '').replace(/\/posts\/(original|thumb)\//, '/posts/medium/');
    }

    function imageList(post) {
        const images = Array.isArray(post && post.images) ? post.images : [];
        const urls = images.map(image => image && (image.image_url || image.url)).filter(Boolean);
        if (urls.length) return urls;
        return post && post.cover_image ? [post.cover_image] : ['/static/img/placeholder.svg'];
    }

    function imageHTML(post, loading) {
        const images = imageList(post);
        const slides = images.map((url, index) => {
            const safeURL = escapeHTML(displayImageURL(url));
            const eager = index === 0 && loading !== 'lazy' ? 'eager' : 'lazy';
            return `<div class="post-media-slide" data-carousel-slide>
                <a href="/post/${post.id}" aria-label="查看帖子图片 ${index + 1}">
                    <img src="${safeURL}" alt="帖子图片 ${index + 1}" loading="${eager}" decoding="async"
                         onerror="this.parentElement.classList.add('img-error')">
                </a>
            </div>`;
        }).join('');
        const controls = images.length > 1 ? `<button type="button" class="post-carousel-arrow post-carousel-prev" data-carousel-prev aria-label="上一张图片"><i class="bi bi-chevron-left"></i></button>
            <button type="button" class="post-carousel-arrow post-carousel-next" data-carousel-next aria-label="下一张图片"><i class="bi bi-chevron-right"></i></button>
            <div class="post-carousel-dots" aria-label="图片页码">${images.map((_, index) => `<button type="button" data-carousel-index="${index}" aria-label="查看第 ${index + 1} 张图片"${index === 0 ? ' aria-current="true"' : ''}></button>`).join('')}</div>` : '';
        return `<div class="post-media-carousel" data-carousel data-carousel-count="${images.length}">
            <div class="post-media-track">${slides}</div>${controls}
        </div>`;
    }

    function renderFeaturedComment(comment, postID) {
        if (!comment) return '';
        const avatar = escapeHTML(comment.user && comment.user.avatar_url ? comment.user.avatar_url : '/static/img/default-avatar.svg');
        const username = escapeHTML(comment.user && comment.user.username ? comment.user.username : '用户');
        const content = escapeHTML(comment.content || '');
        const count = Number(comment.like_count) || 0;
        return `<a class="featured-comment" href="/post/${comment.post_id || postID || ''}#comments">
            <img src="${avatar}" alt="" loading="lazy" onerror="this.src='/static/img/default-avatar.svg'">
            <span class="featured-comment-label">高赞</span>
            <span class="featured-comment-copy"><strong>${username}</strong><span>${content}</span></span>
            <span class="featured-comment-count"><i class="bi bi-heart-fill"></i> ${count}</span>
        </a>`;
    }

    function renderPostCard(post, options) {
        options = options || {};
        const author = escapeHTML(post && post.user && post.user.username ? post.user.username : '社区成员');
        const avatar = escapeHTML(post && post.user && post.user.avatar_url ? post.user.avatar_url : '/static/img/default-avatar.svg');
        const content = escapeHTML(post && post.content ? post.content : '无标题');
        const liked = post && post.is_liked;
        const likeCount = Number(post && post.like_count) || 0;
        const commentCount = Number(post && post.comment_count) || 0;
        const postID = Number(post && post.id) || 0;
        return `<div class="feed-item">
            <article class="feed-card">
                <header class="feed-card-author">
                    <a href="/user/${post && post.user ? post.user.id : ''}" class="feed-author-link">
                        <img src="${avatar}" class="feed-author-avatar" alt="" loading="lazy" onerror="this.src='/static/img/default-avatar.svg'">
                        <strong>${author}</strong>
                    </a>
                    <a href="/post/${postID}" class="feed-card-more" aria-label="查看帖子"><i class="bi bi-three-dots"></i></a>
                </header>
                ${imageHTML(post || {}, options.loading || 'lazy')}
                <div class="feed-card-body">
                    <div class="post-card-actions">
                        <a href="/post/${postID}" class="post-action-link ${liked ? 'is-liked' : ''}"><i class="bi ${liked ? 'bi-heart-fill' : 'bi-heart'}"></i><span>${likeCount}</span></a>
                        <a href="/post/${postID}#comments" class="post-action-link"><i class="bi bi-chat"></i><span>${commentCount}</span></a>
                    </div>
                    <a href="/post/${postID}" class="post-caption">${content}</a>
                    ${renderFeaturedComment(post && post.featured_comment, postID)}
                </div>
            </article>
        </div>`;
    }

    function updateCarousel(carousel) {
        const track = carousel.querySelector('.post-media-track');
        if (!track) return;
        const width = track.clientWidth || carousel.clientWidth || 1;
        const index = Math.max(0, Math.min(Number(carousel.dataset.carouselCount || 1) - 1, Math.round(track.scrollLeft / width)));
        carousel.querySelectorAll('[data-carousel-index]').forEach((dot, dotIndex) => {
            dot.toggleAttribute('aria-current', dotIndex === index);
        });
    }

    function init(root) {
        (root || document).querySelectorAll('[data-carousel]').forEach(carousel => {
            const track = carousel.querySelector('.post-media-track');
            if (!track || track.dataset.bound === 'true') return;
            track.dataset.bound = 'true';
            track.addEventListener('scroll', () => updateCarousel(carousel), {passive: true});
            updateCarousel(carousel);
        });
    }

    document.addEventListener('click', event => {
        const control = event.target.closest('[data-carousel-prev], [data-carousel-next], [data-carousel-index]');
        if (!control) return;
        event.preventDefault();
        event.stopPropagation();
        const carousel = control.closest('[data-carousel]');
        const track = carousel && carousel.querySelector('.post-media-track');
        if (!carousel || !track) return;
        const total = Number(carousel.dataset.carouselCount || 1);
        const current = Math.round(track.scrollLeft / (track.clientWidth || 1));
        const target = control.hasAttribute('data-carousel-prev') ? Math.max(0, current - 1) :
            control.hasAttribute('data-carousel-next') ? Math.min(total - 1, current + 1) :
                Math.max(0, Math.min(total - 1, Number(control.dataset.carouselIndex) || 0));
        track.scrollTo({left: target * track.clientWidth, behavior: 'smooth'});
    });

    window.ShareOPostCards = {escapeHTML, displayImageURL, renderPostCard, init};
    document.addEventListener('DOMContentLoaded', () => init(document));
})();
