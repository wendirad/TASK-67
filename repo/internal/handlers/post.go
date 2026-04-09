package handlers

import (
	"net/http"

	"campusrec/internal/middleware"
	"campusrec/internal/models"
	"campusrec/internal/services"

	"github.com/gin-gonic/gin"
)

// PostHandler handles post endpoints for members.
type PostHandler struct {
	postService *services.PostService
}

func NewPostHandler(postService *services.PostService) *PostHandler {
	return &PostHandler{postService: postService}
}

type createPostRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (h *PostHandler) Create(c *gin.Context) {
	var req createPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	post, code, msg := h.postService.CreatePost(userID, req.Title, req.Content)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Post created", post)
}

func (h *PostHandler) List(c *gin.Context) {
	p := ParsePagination(c)
	userID := middleware.GetUserID(c)
	role := middleware.GetUserRole(c)
	status := c.Query("status")

	posts, total, err := h.postService.ListPosts(userID, role, p.Page, p.PageSize, status)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if posts == nil {
		posts = []models.Post{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(posts, total, p))
}

type reportPostRequest struct {
	Reason string `json:"reason"`
}

func (h *PostHandler) Report(c *gin.Context) {
	postID := c.Param("id")
	var req reportPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	userID := middleware.GetUserID(c)
	report, code, msg := h.postService.ReportPost(postID, userID, req.Reason)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Post reported", report)
}

// ModerationHandler handles moderation endpoints for moderators/admins.
type ModerationHandler struct {
	postService *services.PostService
}

func NewModerationHandler(postService *services.PostService) *ModerationHandler {
	return &ModerationHandler{postService: postService}
}

func (h *ModerationHandler) ListQueue(c *gin.Context) {
	p := ParsePagination(c)
	status := c.Query("status")

	posts, total, err := h.postService.ListModerationQueue(p.Page, p.PageSize, status)
	if err != nil {
		Error(c, http.StatusInternalServerError, "Internal server error")
		return
	}
	if posts == nil {
		posts = []models.Post{}
	}

	Success(c, http.StatusOK, "OK", PaginatedResponse(posts, total, p))
}

type makeDecisionRequest struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func (h *ModerationHandler) MakeDecision(c *gin.Context) {
	postID := c.Param("id")
	var req makeDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	moderatorID := middleware.GetUserID(c)
	decision, code, msg := h.postService.MakeDecision(postID, moderatorID, req.Action, req.Reason)
	if code != http.StatusCreated {
		Error(c, code, msg)
		return
	}

	Created(c, "Decision recorded", decision)
}
