package services

import (
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type PostService struct {
	postRepo   *repository.PostRepository
	userRepo   *repository.UserRepository
	configRepo *repository.ConfigRepository
}

func NewPostService(postRepo *repository.PostRepository, userRepo *repository.UserRepository, configRepo *repository.ConfigRepository) *PostService {
	return &PostService{postRepo: postRepo, userRepo: userRepo, configRepo: configRepo}
}

// CreatePost creates a post with rate limiting and ban checks.
// canaryCohort is the user's cohort from middleware context (-1 if unset).
func (s *PostService) CreatePost(userID, title, content string, canaryCohort int) (*models.Post, int, string) {
	// Check ban status
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		log.Printf("Error finding user %s: %v", userID, err)
		return nil, 500, "Internal server error"
	}
	if user == nil {
		return nil, 404, "User not found"
	}
	if user.Status == "banned" {
		return nil, 403, "Your account is banned and cannot create posts."
	}

	// Validate input
	if title == "" {
		return nil, 400, "Title is required"
	}
	if len(title) > 300 {
		return nil, 400, "Title must be at most 300 characters"
	}
	if content == "" {
		return nil, 400, "Content is required"
	}
	if len(content) > 5000 {
		return nil, 400, "Content must be at most 5000 characters"
	}

	// Check rate limit (canary-aware via cohort from middleware context)
	rateLimit := s.configRepo.GetIntForCohort("post.rate_limit_per_hour", 5, canaryCohort)
	recentCount, err := s.postRepo.CountRecentByUser(userID, 60)
	if err != nil {
		log.Printf("Error counting recent posts: %v", err)
		return nil, 500, "Internal server error"
	}
	if recentCount >= rateLimit {
		return nil, 429, "Posting limit reached. Maximum 5 posts per hour."
	}

	post := &models.Post{
		UserID:  userID,
		Title:   title,
		Content: content,
	}
	if err := s.postRepo.Create(post); err != nil {
		log.Printf("Error creating post: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Post created: %s user=%s", post.ID, userID)
	return post, 201, ""
}

// ListPosts returns paginated posts based on user role.
func (s *PostService) ListPosts(userID, role string, page, pageSize int, status string) ([]models.Post, int, error) {
	if role == "member" {
		return s.postRepo.ListForMember(userID, page, pageSize, status)
	}
	return s.postRepo.ListAll(page, pageSize, status)
}

// ReportPost creates a report on a post.
// canaryCohort is the reporter's cohort from middleware context (-1 if unset).
func (s *PostService) ReportPost(postID, userID, reason string, canaryCohort int) (*models.PostReport, int, string) {
	if reason == "" || len(reason) > 500 {
		return nil, 400, "Reason is required (max 500 characters)"
	}

	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		log.Printf("Error finding post %s: %v", postID, err)
		return nil, 500, "Internal server error"
	}
	if post == nil {
		return nil, 404, "Post not found"
	}
	if post.UserID == userID {
		return nil, 400, "Cannot report own post"
	}

	already, err := s.postRepo.HasReported(postID, userID)
	if err != nil {
		log.Printf("Error checking duplicate report: %v", err)
		return nil, 500, "Internal server error"
	}
	if already {
		return nil, 409, "You have already reported this post"
	}

	// Auto-flag threshold is canary-gated via cohort from middleware context
	autoFlagThreshold := s.configRepo.GetIntForCohort("post.auto_flag_report_count", 3, canaryCohort)
	report, err := s.postRepo.Report(postID, userID, reason, autoFlagThreshold)
	if err != nil {
		log.Printf("Error reporting post: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Post reported: post=%s by=%s", postID, userID)
	return report, 201, ""
}

// ListModerationQueue returns posts needing moderation.
func (s *PostService) ListModerationQueue(page, pageSize int, status string) ([]models.Post, int, error) {
	return s.postRepo.ListModerationQueue(page, pageSize, status)
}

// MakeDecision creates a moderation decision on a post.
func (s *PostService) MakeDecision(postID, moderatorID, action, reason string) (*models.ModerationDecision, int, string) {
	validActions := map[string]bool{
		"approve": true, "reject": true, "remove": true, "ban_user": true, "warn_user": true,
	}
	if !validActions[action] {
		return nil, 400, "Action must be one of: approve, reject, remove, ban_user, warn_user"
	}
	if len(reason) < 10 {
		return nil, 400, "Reason must be at least 10 characters"
	}
	if len(reason) > 1000 {
		return nil, 400, "Reason must be at most 1000 characters"
	}

	post, err := s.postRepo.FindByID(postID)
	if err != nil {
		log.Printf("Error finding post %s: %v", postID, err)
		return nil, 500, "Internal server error"
	}
	if post == nil {
		return nil, 404, "Post not found"
	}

	decision, err := s.postRepo.MakeDecision(postID, moderatorID, action, reason)
	if err != nil {
		log.Printf("Error making decision on post %s: %v", postID, err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Moderation decision: post=%s action=%s moderator=%s", postID, action, moderatorID)
	return decision, 201, ""
}
