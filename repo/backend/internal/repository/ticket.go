package repository

import (
	"database/sql"
	"fmt"
	"time"

	"campusrec/internal/models"
)

type TicketRepository struct {
	db *sql.DB
}

func NewTicketRepository(db *sql.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

// Create inserts a new ticket with SLA deadlines.
func (r *TicketRepository) Create(t *models.Ticket) error {
	return r.db.QueryRow(`
		INSERT INTO tickets (type, ticket_number, subject, description, status, priority,
		    created_by, related_entity_type, related_entity_id,
		    sla_response_deadline, sla_resolution_deadline)
		VALUES ($1, $2, $3, $4, 'open', $5, $6, $7, $8, $9, $10)
		RETURNING id, status, created_at, updated_at
	`, t.Type, t.TicketNumber, t.Subject, t.Description, t.Priority,
		t.CreatedBy, t.RelatedEntityType, t.RelatedEntityID,
		t.SLAResponseDeadline, t.SLAResolutionDeadline,
	).Scan(&t.ID, &t.Status, &t.CreatedAt, &t.UpdatedAt)
}

// FindByID returns a ticket by ID with creator and assignee info.
func (r *TicketRepository) FindByID(id string) (*models.Ticket, error) {
	t := &models.Ticket{}
	err := r.db.QueryRow(`
		SELECT t.id, t.ticket_number, t.type, t.subject, t.description, t.status, t.priority,
		       t.assigned_to, t.created_by, t.related_entity_type, t.related_entity_id,
		       t.sla_response_deadline, t.sla_resolution_deadline,
		       t.sla_response_met, t.sla_resolution_met,
		       t.responded_at, t.resolved_at, t.closed_at,
		       t.created_at, t.updated_at,
		       cu.username, cu.display_name,
		       au.username, au.display_name
		FROM tickets t
		JOIN users cu ON cu.id = t.created_by
		LEFT JOIN users au ON au.id = t.assigned_to
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.TicketNumber, &t.Type, &t.Subject, &t.Description, &t.Status, &t.Priority,
		&t.AssignedTo, &t.CreatedBy, &t.RelatedEntityType, &t.RelatedEntityID,
		&t.SLAResponseDeadline, &t.SLAResolutionDeadline,
		&t.SLAResponseMet, &t.SLAResolutionMet,
		&t.RespondedAt, &t.ResolvedAt, &t.ClosedAt,
		&t.CreatedAt, &t.UpdatedAt,
		&t.CreatedByUsername, &t.CreatedByDisplayName,
		&t.AssignedToUsername, &t.AssignedToDisplayName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find ticket: %w", err)
	}
	return t, nil
}

// ListByUser returns paginated tickets created by a user.
func (r *TicketRepository) ListByUser(userID string, page, pageSize int, status, ticketType string) ([]models.Ticket, int, error) {
	baseQuery := `FROM tickets t
		JOIN users cu ON cu.id = t.created_by
		LEFT JOIN users au ON au.id = t.assigned_to
		WHERE t.created_by = $1`
	args := []interface{}{userID}
	argIdx := 2

	if status != "" {
		baseQuery += fmt.Sprintf(` AND t.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if ticketType != "" {
		baseQuery += fmt.Sprintf(` AND t.type = $%d`, argIdx)
		args = append(args, ticketType)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.ticket_number, t.type, t.subject, t.description, t.status, t.priority,
		       t.assigned_to, t.created_by, t.related_entity_type, t.related_entity_id,
		       t.sla_response_deadline, t.sla_resolution_deadline,
		       t.sla_response_met, t.sla_resolution_met,
		       t.responded_at, t.resolved_at, t.closed_at,
		       t.created_at, t.updated_at,
		       cu.username, cu.display_name,
		       au.username, au.display_name
		%s ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	tickets, err := r.scanTickets(selectQuery, args)
	return tickets, total, err
}

// ListAll returns paginated tickets for staff/admin.
func (r *TicketRepository) ListAll(page, pageSize int, status, ticketType, priority, assignedTo string) ([]models.Ticket, int, error) {
	baseQuery := `FROM tickets t
		JOIN users cu ON cu.id = t.created_by
		LEFT JOIN users au ON au.id = t.assigned_to
		WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if status != "" {
		baseQuery += fmt.Sprintf(` AND t.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}
	if ticketType != "" {
		baseQuery += fmt.Sprintf(` AND t.type = $%d`, argIdx)
		args = append(args, ticketType)
		argIdx++
	}
	if priority != "" {
		baseQuery += fmt.Sprintf(` AND t.priority = $%d`, argIdx)
		args = append(args, priority)
		argIdx++
	}
	if assignedTo != "" {
		baseQuery += fmt.Sprintf(` AND t.assigned_to = $%d`, argIdx)
		args = append(args, assignedTo)
		argIdx++
	}

	var total int
	if err := r.db.QueryRow(`SELECT COUNT(*) `+baseQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tickets: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT t.id, t.ticket_number, t.type, t.subject, t.description, t.status, t.priority,
		       t.assigned_to, t.created_by, t.related_entity_type, t.related_entity_id,
		       t.sla_response_deadline, t.sla_resolution_deadline,
		       t.sla_response_met, t.sla_resolution_met,
		       t.responded_at, t.resolved_at, t.closed_at,
		       t.created_at, t.updated_at,
		       cu.username, cu.display_name,
		       au.username, au.display_name
		%s ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argIdx, argIdx+1)
	args = append(args, pageSize, (page-1)*pageSize)

	tickets, err := r.scanTickets(selectQuery, args)
	return tickets, total, err
}

// Assign sets the assigned_to field and records responded_at if first assignment.
func (r *TicketRepository) Assign(ticketID, assignedTo string) error {
	_, err := r.db.Exec(`
		UPDATE tickets SET
		    assigned_to = $2,
		    status = CASE WHEN status = 'open' THEN 'assigned' ELSE status END,
		    responded_at = CASE WHEN responded_at IS NULL THEN NOW() ELSE responded_at END,
		    sla_response_met = CASE
		        WHEN responded_at IS NULL AND sla_response_deadline IS NOT NULL
		        THEN NOW() <= sla_response_deadline
		        ELSE sla_response_met
		    END,
		    updated_at = NOW()
		WHERE id = $1
	`, ticketID, assignedTo)
	if err != nil {
		return fmt.Errorf("assign ticket: %w", err)
	}
	return nil
}

// UpdateStatus transitions a ticket status.
func (r *TicketRepository) UpdateStatus(ticketID, status string) error {
	query := `UPDATE tickets SET status = $2, updated_at = NOW()`

	switch status {
	case "resolved":
		query += `,
			resolved_at = CASE WHEN resolved_at IS NULL THEN NOW() ELSE resolved_at END,
			sla_resolution_met = CASE
			    WHEN resolved_at IS NULL AND sla_resolution_deadline IS NOT NULL
			    THEN NOW() <= sla_resolution_deadline
			    ELSE sla_resolution_met
			END`
	case "closed":
		query += `, closed_at = CASE WHEN closed_at IS NULL THEN NOW() ELSE closed_at END`
	}

	query += ` WHERE id = $1`
	_, err := r.db.Exec(query, ticketID, status)
	if err != nil {
		return fmt.Errorf("update ticket status: %w", err)
	}
	return nil
}

// AddComment inserts a ticket comment.
func (r *TicketRepository) AddComment(comment *models.TicketComment) error {
	return r.db.QueryRow(`
		INSERT INTO ticket_comments (ticket_id, user_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, comment.TicketID, comment.UserID, comment.Content).Scan(&comment.ID, &comment.CreatedAt)
}

// GetComments returns all comments for a ticket with user info.
func (r *TicketRepository) GetComments(ticketID string) ([]models.TicketComment, error) {
	rows, err := r.db.Query(`
		SELECT c.id, c.ticket_id, c.user_id, c.content, c.created_at,
		       u.username, u.display_name
		FROM ticket_comments c
		JOIN users u ON u.id = c.user_id
		WHERE c.ticket_id = $1
		ORDER BY c.created_at ASC
	`, ticketID)
	if err != nil {
		return nil, fmt.Errorf("get comments: %w", err)
	}
	defer rows.Close()

	var comments []models.TicketComment
	for rows.Next() {
		var c models.TicketComment
		if err := rows.Scan(&c.ID, &c.TicketID, &c.UserID, &c.Content, &c.CreatedAt,
			&c.Username, &c.DisplayName); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// MarkBreachedSLAResponse marks tickets that breached response SLA.
func (r *TicketRepository) MarkBreachedSLAResponse() (int, error) {
	result, err := r.db.Exec(`
		UPDATE tickets SET sla_response_met = false, updated_at = NOW()
		WHERE status IN ('open', 'assigned')
		    AND sla_response_deadline < NOW()
		    AND responded_at IS NULL
		    AND (sla_response_met IS NULL OR sla_response_met = true)
	`)
	if err != nil {
		return 0, fmt.Errorf("mark breached SLA response: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// MarkBreachedSLAResolution marks tickets that breached resolution SLA.
func (r *TicketRepository) MarkBreachedSLAResolution() (int, error) {
	result, err := r.db.Exec(`
		UPDATE tickets SET sla_resolution_met = false, updated_at = NOW()
		WHERE status NOT IN ('resolved', 'closed')
		    AND sla_resolution_deadline < NOW()
		    AND resolved_at IS NULL
		    AND (sla_resolution_met IS NULL OR sla_resolution_met = true)
	`)
	if err != nil {
		return 0, fmt.Errorf("mark breached SLA resolution: %w", err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// GetConfigInt reads an integer config value with a default.
func (r *TicketRepository) GetConfigInt(key string, defaultVal int) int {
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

// CalculateSLAResponseDeadline returns the response deadline based on business hours.
// Business hours: Mon-Fri 9:00-18:00, configured response hours from config.
// CalculateSLAResponseDeadline calculates the SLA response deadline based on business hours.
func CalculateSLAResponseDeadline(createdAt time.Time, responseHours int) time.Time {
	return calculateSLADeadline(createdAt, responseHours)
}

func (r *TicketRepository) CalculateSLAResponseDeadline(createdAt time.Time, responseHours int) time.Time {
	return calculateSLADeadline(createdAt, responseHours)
}

func calculateSLADeadline(createdAt time.Time, responseHours int) time.Time {
	remaining := time.Duration(responseHours) * time.Hour
	current := createdAt

	for remaining > 0 {
		// Advance to the next business day if weekend
		for current.Weekday() == time.Saturday || current.Weekday() == time.Sunday {
			current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
		}

		businessStart := time.Date(current.Year(), current.Month(), current.Day(), 9, 0, 0, 0, current.Location())
		businessEnd := time.Date(current.Year(), current.Month(), current.Day(), 18, 0, 0, 0, current.Location())

		// If before business hours, move to start
		if current.Before(businessStart) {
			current = businessStart
		}

		// If after business hours, move to next business day
		if !current.Before(businessEnd) {
			current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
			continue
		}

		// Time left in today's business hours
		availableToday := businessEnd.Sub(current)
		if remaining <= availableToday {
			return current.Add(remaining)
		}

		remaining -= availableToday
		current = time.Date(current.Year(), current.Month(), current.Day()+1, 9, 0, 0, 0, current.Location())
	}

	return current
}

func (r *TicketRepository) scanTickets(query string, args []interface{}) ([]models.Ticket, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tickets: %w", err)
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(
			&t.ID, &t.TicketNumber, &t.Type, &t.Subject, &t.Description, &t.Status, &t.Priority,
			&t.AssignedTo, &t.CreatedBy, &t.RelatedEntityType, &t.RelatedEntityID,
			&t.SLAResponseDeadline, &t.SLAResolutionDeadline,
			&t.SLAResponseMet, &t.SLAResolutionMet,
			&t.RespondedAt, &t.ResolvedAt, &t.ClosedAt,
			&t.CreatedAt, &t.UpdatedAt,
			&t.CreatedByUsername, &t.CreatedByDisplayName,
			&t.AssignedToUsername, &t.AssignedToDisplayName,
		); err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}
