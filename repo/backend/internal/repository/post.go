package repository

import (
	"database/sql"
	"fmt"

	"campusrec/internal/models"
)

type PostRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) *PostRepository {
	return &PostRepository{db: db}
}

// Create inserts a new post.
func (r *PostRepository) Create(post *models.Post) error {
	return r.db.QueryRow(`
		INSERT INTO posts (user_id, title, content, status)
		VALUES ($1, $2, $3, 'pending_review')
		RETURNING id, status, reported_count, created_at, updated_at
	`, post.UserID, post.Title, post.Content).Scan(
		&post.ID, &post.Status, &post.ReportedCount, &post.CreatedAt, &post.UpdatedAt,
	)
}

// FindByID returns a post by ID.
func (r *PostRepository) FindByID(id string) (*models.Post, error) {
	p := &models.Post{}
	err := r.db.QueryRow(`
		SELECT p.id, p.user_id, p.title, p.content, p.status, p.reported_count,
		       p.created_at, p.updated_at, u.username, u.display_name
		FROM posts p JOIN users u ON u.id = p.user_id
		WHERE p.id = $1
	`, id).Scan(
		&p.ID, &p.UserID, &p.Title, &p.Content, &p.Status, &p.ReportedCount,
		&p.CreatedAt, &p.UpdatedAt, &p.Username, &p.DisplayName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find post: %w", err)
	}
	return p, nil
}

// CountRecentByUser counts posts by user in the last N minutes.
func (r *PostRepository) CountRecentByUser(userID string, minutes int) (int, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM posts
		WHERE user_id = $1 AND created_at > NOW() - ($2 || ' minutes')::interval
	`, userID, minutes).Scan(&count)
	return count, err
}

// ListForMember returns paginated posts: own posts + approved posts from others.
func (r *PostRepository) ListForMember(userID string, page, pageSize int, status string) ([]models.Post, int, error) {
	baseQuery := `FROM posts p JOIN users u ON u.id = p.user_id
		WHERE (p.user_id = $1 OR p.status = 'approved')`
	args := []interface{}{userID}
	argIdx := 2

	if status != "" {
		baseQuery += fmt.Sprintf(` AND p.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT p.id, p.user_id, p.title, p.content, p.status, p.reported_count,
		       p.created_at, p.updated_at, u.username, u.display_name
		%s ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	posts, err := r.scanPosts(selectQuery, args)
	return posts, total, err
}

// ListAll returns paginated posts for staff/moderator/admin (all posts).
func (r *PostRepository) ListAll(page, pageSize int, status string) ([]models.Post, int, error) {
	baseQuery := `FROM posts p JOIN users u ON u.id = p.user_id WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND p.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count posts: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT p.id, p.user_id, p.title, p.content, p.status, p.reported_count,
		       p.created_at, p.updated_at, u.username, u.display_name
		%s ORDER BY p.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	posts, err := r.scanPosts(selectQuery, args)
	return posts, total, err
}

// ListModerationQueue returns posts needing moderation (pending_review or flagged).
func (r *PostRepository) ListModerationQueue(page, pageSize int, status string) ([]models.Post, int, error) {
	if status == "" {
		status = "" // will use default filter below
	}

	baseQuery := `FROM posts p JOIN users u ON u.id = p.user_id WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND p.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	} else {
		baseQuery += ` AND p.status IN ('pending_review', 'flagged')`
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count moderation queue: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT p.id, p.user_id, p.title, p.content, p.status, p.reported_count,
		       p.created_at, p.updated_at, u.username, u.display_name
		%s ORDER BY p.reported_count DESC, p.created_at ASC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	posts, err := r.scanPosts(selectQuery, args)
	return posts, total, err
}

// Report creates a report and increments reported_count. Auto-flags if threshold reached.
func (r *PostRepository) Report(postID, reportedBy, reason string, autoFlagThreshold int) (*models.PostReport, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	report := &models.PostReport{}
	err = tx.QueryRow(`
		INSERT INTO post_reports (post_id, reported_by, reason)
		VALUES ($1, $2, $3)
		RETURNING id, post_id, reported_by, reason, created_at
	`, postID, reportedBy, reason).Scan(
		&report.ID, &report.PostID, &report.ReportedBy, &report.Reason, &report.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create report: %w", err)
	}

	// Increment reported_count and auto-flag if threshold reached
	_, err = tx.Exec(`
		UPDATE posts SET reported_count = reported_count + 1,
		    status = CASE WHEN reported_count + 1 >= $2 AND status NOT IN ('removed', 'rejected') THEN 'flagged' ELSE status END,
		    updated_at = NOW()
		WHERE id = $1
	`, postID, autoFlagThreshold)
	if err != nil {
		return nil, fmt.Errorf("update reported count: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return report, nil
}

// HasReported checks if a user has already reported a post.
func (r *PostRepository) HasReported(postID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM post_reports WHERE post_id = $1 AND reported_by = $2)
	`, postID, userID).Scan(&exists)
	return exists, err
}

// MakeDecision creates a moderation decision and updates post status.
func (r *PostRepository) MakeDecision(postID, moderatorID, action, reason string) (*models.ModerationDecision, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Create immutable decision record
	decision := &models.ModerationDecision{}
	err = tx.QueryRow(`
		INSERT INTO moderation_decisions (post_id, moderator_id, action, reason)
		VALUES ($1, $2, $3, $4)
		RETURNING id, post_id, moderator_id, action, reason, created_at
	`, postID, moderatorID, action, reason).Scan(
		&decision.ID, &decision.PostID, &decision.ModeratorID, &decision.Action, &decision.Reason, &decision.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create decision: %w", err)
	}

	// Update post status based on action
	var newStatus string
	switch action {
	case "approve", "warn_user":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
	case "remove", "ban_user":
		newStatus = "removed"
	}

	_, err = tx.Exec(`
		UPDATE posts SET status = $1, updated_at = NOW() WHERE id = $2
	`, newStatus, postID)
	if err != nil {
		return nil, fmt.Errorf("update post status: %w", err)
	}

	// Ban user if action is ban_user
	if action == "ban_user" {
		var authorID string
		if err := tx.QueryRow(`SELECT user_id FROM posts WHERE id = $1`, postID).Scan(&authorID); err != nil {
			return nil, fmt.Errorf("get post author: %w", err)
		}
		_, err = tx.Exec(`UPDATE users SET status = 'banned', updated_at = NOW() WHERE id = $1`, authorID)
		if err != nil {
			return nil, fmt.Errorf("ban user: %w", err)
		}
	}

	// Create warning ticket if warn_user
	if action == "warn_user" {
		var authorID string
		if err := tx.QueryRow(`SELECT user_id FROM posts WHERE id = $1`, postID).Scan(&authorID); err != nil {
			return nil, fmt.Errorf("get post author: %w", err)
		}
		ticketNumber := fmt.Sprintf("TKT-%d", decision.CreatedAt.UnixNano()%100000000)
		_, err = tx.Exec(`
			INSERT INTO tickets (ticket_number, subject, type, status, description, created_by, related_entity_type, related_entity_id)
			VALUES ($1, $2, 'general', 'open', $3, $4, 'post', $5)
		`, ticketNumber, "Moderation Warning: Your post requires revision", reason, authorID, postID)
		if err != nil {
			return nil, fmt.Errorf("create warning ticket: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return decision, nil
}

// GetReportsForPost returns all reports for a post.
func (r *PostRepository) GetReportsForPost(postID string) ([]models.PostReport, error) {
	rows, err := r.db.Query(`
		SELECT id, post_id, reported_by, reason, created_at
		FROM post_reports WHERE post_id = $1
		ORDER BY created_at ASC
	`, postID)
	if err != nil {
		return nil, fmt.Errorf("get reports: %w", err)
	}
	defer rows.Close()

	var reports []models.PostReport
	for rows.Next() {
		var r models.PostReport
		if err := rows.Scan(&r.ID, &r.PostID, &r.ReportedBy, &r.Reason, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// GetConfigInt reads an integer config value with a default.
func (r *PostRepository) GetConfigInt(key string, defaultVal int) int {
	var val string
	err := r.db.QueryRow(`SELECT value FROM config_entries WHERE key = $1`, key).Scan(&val)
	if err != nil {
		return defaultVal
	}
	var n int
	fmt.Sscanf(val, "%d", &n)
	if n <= 0 {
		return defaultVal
	}
	return n
}

func (r *PostRepository) scanPosts(query string, args []interface{}) ([]models.Post, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.Title, &p.Content, &p.Status, &p.ReportedCount,
			&p.CreatedAt, &p.UpdatedAt, &p.Username, &p.DisplayName,
		); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}
