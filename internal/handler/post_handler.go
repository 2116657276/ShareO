package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zhoujianlin/ShareO/internal/pkg/response"
	"github.com/zhoujianlin/ShareO/internal/service"
)

type PostHandler struct {
	svc *service.PostService
}

func NewPostHandler() *PostHandler { return &PostHandler{svc: service.NewPostService()} }

func (h *PostHandler) Create(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req service.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	post, err := h.svc.Create(userID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) Update(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	post, err := h.svc.Update(userID, postID, req.Content)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) Repost(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	var req service.RepostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数格式错误")
		return
	}
	post, err := h.svc.Repost(userID, postID, req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, post)
}

func (h *PostHandler) Delete(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	if err := h.svc.Delete(userID, postID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *PostHandler) GetByID(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, post)
}

// --- Web page handlers ---

func (h *PostHandler) CreatePage(c *gin.Context) {
	c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品 - ShareO"}))
}

func (h *PostHandler) DetailPage(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil {
		c.HTML(http.StatusNotFound, "404.html", userData(c, gin.H{"title": "404"}))
		return
	}
	c.HTML(http.StatusOK, "post_detail.html", userData(c, gin.H{
		"title": post.Content,
		"Post":  post,
})) 
}

func (h *PostHandler) EditPage(c *gin.Context) {
	postID := getInt64Param(c, "id")
	currentUserID := c.GetInt64("user_id")
	post, err := h.svc.GetByID(postID, currentUserID)
	if err != nil || post.UserID != currentUserID {
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.HTML(http.StatusOK, "edit_post.html", userData(c, gin.H{
		"title": "编辑帖子 - ShareO",
		"Post":  post,
})) 
}

// WebCreate handles form-based post creation
func (h *PostHandler) WebCreate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	content := c.PostForm("content")
	imageURLs := c.PostFormArray("images")
	topicIDsStr := c.PostFormArray("topic_ids")

	var topicIDs []int64
	for _, s := range topicIDsStr {
		id, _ := strconv.ParseInt(s, 10, 64)
		if id > 0 {
			topicIDs = append(topicIDs, id)
		}
	}

	if len(imageURLs) == 0 {
		// Try single file upload
		_, err := c.FormFile("image")
		if err == nil {
			// Upload handled separately via /api/v1/upload
			c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": "请先上传图片"}))
			return
		}
		c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": "至少需要一张图片"}))
		return
	}

	req := service.CreatePostReq{
		Content:  content,
		Images:   imageURLs,
		TopicIDs: topicIDs,
	}
	_, err := h.svc.Create(userID, req)
	if err != nil {
		c.HTML(http.StatusOK, "create_post.html", userData(c, gin.H{"title": "发布作品", "Error": err.Error()}))
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func (h *PostHandler) WebUpdate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	postID := getInt64Param(c, "id")
	content := c.PostForm("content")
	_, err := h.svc.Update(userID, postID, content)
	if err != nil {
		c.HTML(http.StatusOK, "edit_post.html", userData(c, gin.H{"Error": err.Error()}))
		return
	}
	c.Redirect(http.StatusFound, "/post/"+c.Param("id"))
}
