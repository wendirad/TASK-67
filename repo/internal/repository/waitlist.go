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
	Position     int `json:"position"`
	TotalWaiting int `json:"total_waiting"`
}

// GetPosition returns the user's waitlist position for a session.
// Returns nil if the user is not on the waitlist.
func (r *WaitlistRepository) GetPosition(userID, sessionID string) (*WaitlistPosition, error) {
	var position int
	err := r.db.QueryRow(`
		SELECT position FROM waitlist
		WHERE user_id = $1 AND session_id = $2 AND status = 'waiting'
	`, userID, sessionID).Scan(&position)
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

	return &WaitlistPosition{Position: position, TotalWaiting: totalWaiting}, nil
}
