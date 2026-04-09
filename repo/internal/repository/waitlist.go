package repository

import (
	"database/sql"
	"fmt"
)

type WaitlistRepository struct {
	db *sql.DB
}

func NewWaitlistRepository(db *sql.DB) *WaitlistRepository {
	return &WaitlistRepository{db: db}
}

type WaitlistPosition struct {
	Position     int    `json:"position"`
	TotalWaiting int    `json:"total_waiting"`
	SessionTitle string `json:"session_title,omitempty"`
}

// GetPosition returns the user's waitlist position for a session.
// Returns nil if the user is not on the waitlist.
func (r *WaitlistRepository) GetPosition(userID, sessionID string) (*WaitlistPosition, error) {
	var position int
	var sessionTitle string
	err := r.db.QueryRow(`
		SELECT w.position, s.title FROM waitlist w
		JOIN sessions s ON s.id = w.session_id
		WHERE w.user_id = $1 AND w.session_id = $2 AND w.status = 'waiting'
	`, userID, sessionID).Scan(&position, &sessionTitle)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get waitlist position: %w", err)
	}

	var totalWaiting int
	err = r.db.QueryRow(`
		SELECT COUNT(*) FROM waitlist
		WHERE session_id = $1 AND status = 'waiting'
	`, sessionID).Scan(&totalWaiting)
	if err != nil {
		return nil, fmt.Errorf("count waiting: %w", err)
	}

	return &WaitlistPosition{Position: position, TotalWaiting: totalWaiting, SessionTitle: sessionTitle}, nil
}

// GetActivePosition returns the user's most recent active waitlist position across all sessions.
// Returns nil if the user is not on any waitlist.
func (r *WaitlistRepository) GetActivePosition(userID string) (*WaitlistPosition, error) {
	var position int
	var sessionTitle string
	var sessionID string
	err := r.db.QueryRow(`
		SELECT w.position, s.title, w.session_id FROM waitlist w
		JOIN sessions s ON s.id = w.session_id
		WHERE w.user_id = $1 AND w.status = 'waiting'
		ORDER BY w.joined_at DESC
		LIMIT 1
	`, userID).Scan(&position, &sessionTitle, &sessionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active waitlist position: %w", err)
	}

	var totalWaiting int
	err = r.db.QueryRow(`
		SELECT COUNT(*) FROM waitlist
		WHERE session_id = $1 AND status = 'waiting'
	`, sessionID).Scan(&totalWaiting)
	if err != nil {
		return nil, fmt.Errorf("count waiting: %w", err)
	}

	return &WaitlistPosition{Position: position, TotalWaiting: totalWaiting, SessionTitle: sessionTitle}, nil
}
