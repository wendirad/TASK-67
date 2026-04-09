package models

import "time"

type CheckIn struct {
	ID                string     `json:"id"`
	RegistrationID    string     `json:"registration_id"`
	UserID            string     `json:"user_id"`
	SessionID         string     `json:"session_id"`
	SeatID            string     `json:"seat_id"`
	SeatNumber        int        `json:"seat_number,omitempty"`
	Status            string     `json:"status"`
	Method            string     `json:"method"`
	ConfirmedBy       string     `json:"confirmed_by"`
	BreakCount        int        `json:"break_count"`
	TotalBreakMinutes int        `json:"total_break_minutes"`
	LastBreakStart    *time.Time `json:"last_break_start,omitempty"`
	CheckedInAt       time.Time  `json:"checked_in_at"`
	CheckedOutAt      *time.Time `json:"checked_out_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Joined fields
	SessionTitle *string `json:"session_title,omitempty"`
	Username     *string `json:"username,omitempty"`
	DisplayName  *string `json:"display_name,omitempty"`
}

type CheckInRequest struct {
	RegistrationID     string `json:"registration_id" binding:"required"`
	KioskDeviceToken   string `json:"kiosk_device_token" binding:"required"`
	BluetoothConfirmed bool   `json:"bluetooth_confirmed"`
	BluetoothBeaconID  string `json:"bluetooth_beacon_id"`
}

type QRCodeResponse struct {
	QRContent  string    `json:"qr_content"`
	ValidUntil time.Time `json:"valid_until"`
}
