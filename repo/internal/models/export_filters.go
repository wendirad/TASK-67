package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExportFilters holds parsed filter values for scoped exports.
type ExportFilters struct {
	Status   string `json:"status"`
	Role     string `json:"role"`
	Category string `json:"category"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

// ParseExportFilters parses a JSON filter string. An empty string returns zero-value filters.
func ParseExportFilters(raw string) (*ExportFilters, error) {
	if raw == "" {
		return &ExportFilters{}, nil
	}
	var f ExportFilters
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return nil, fmt.Errorf("invalid filter JSON: %w", err)
	}
	return &f, nil
}

// ValidateExportFilters checks that filter values are permitted for the given entity type.
// Returns a user-facing error message or "" if valid.
func ValidateExportFilters(entityType string, filters *ExportFilters) string {
	if filters == nil {
		return ""
	}

	if filters.Status != "" {
		allowed := exportStatusValues(entityType)
		if !allowed[filters.Status] {
			return fmt.Sprintf("Invalid status filter '%s' for %s", filters.Status, entityType)
		}
	}

	if filters.Role != "" && entityType != "users" {
		return "Filter 'role' is only valid for users export"
	}
	if filters.Role != "" {
		validRoles := map[string]bool{"member": true, "staff": true, "moderator": true, "admin": true}
		if !validRoles[filters.Role] {
			return fmt.Sprintf("Invalid role filter '%s'", filters.Role)
		}
	}

	if filters.Category != "" && entityType != "products" {
		return "Filter 'category' is only valid for products export"
	}

	if filters.Type != "" && entityType != "tickets" {
		return "Filter 'type' is only valid for tickets export"
	}
	if filters.Type != "" {
		validTypes := map[string]bool{"seat_exception": true, "delivery_exception": true, "payment_issue": true, "general": true, "moderation_appeal": true}
		if !validTypes[filters.Type] {
			return fmt.Sprintf("Invalid type filter '%s'", filters.Type)
		}
	}

	if filters.Priority != "" && entityType != "tickets" {
		return "Filter 'priority' is only valid for tickets export"
	}
	if filters.Priority != "" {
		validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
		if !validPriorities[filters.Priority] {
			return fmt.Sprintf("Invalid priority filter '%s'", filters.Priority)
		}
	}

	if filters.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", filters.DateFrom); err != nil {
			return "Invalid date_from format, expected YYYY-MM-DD"
		}
	}
	if filters.DateTo != "" {
		if _, err := time.Parse("2006-01-02", filters.DateTo); err != nil {
			return "Invalid date_to format, expected YYYY-MM-DD"
		}
	}

	return ""
}

func exportStatusValues(entityType string) map[string]bool {
	switch entityType {
	case "users":
		return map[string]bool{"active": true, "banned": true, "suspended": true, "inactive": true}
	case "products":
		return map[string]bool{"active": true, "inactive": true, "out_of_stock": true}
	case "sessions":
		return map[string]bool{"draft": true, "open": true, "closed": true, "in_progress": true, "completed": true, "canceled": true}
	case "orders":
		return map[string]bool{"created": true, "pending_payment": true, "paid": true, "processing": true, "shipped": true, "delivered": true, "completed": true, "closed": true, "refund_pending": true, "refunded": true}
	case "registrations":
		return map[string]bool{"pending": true, "approved": true, "rejected": true, "registered": true, "waitlisted": true, "canceled": true, "completed": true, "no_show": true}
	case "tickets":
		return map[string]bool{"open": true, "assigned": true, "in_progress": true, "resolved": true, "closed": true}
	}
	return nil
}
