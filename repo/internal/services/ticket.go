package services

import (
	"fmt"
	"log"
	"time"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type TicketService struct {
	ticketRepo *repository.TicketRepository
	userRepo   *repository.UserRepository
}

func NewTicketService(ticketRepo *repository.TicketRepository, userRepo *repository.UserRepository) *TicketService {
	return &TicketService{ticketRepo: ticketRepo, userRepo: userRepo}
}

// CreateTicket creates a ticket with SLA deadlines.
func (s *TicketService) CreateTicket(userID, ticketType, subject, description, priority string, relatedEntityType, relatedEntityID *string) (*models.Ticket, int, string) {
	// Validate type
	validTypes := map[string]bool{
		"general": true, "moderation_appeal": true,
	}
	if !validTypes[ticketType] {
		return nil, 400, "Type must be 'general' or 'moderation_appeal'"
	}

	if subject == "" {
		return nil, 400, "Subject is required"
	}
	if len(subject) > 500 {
		return nil, 400, "Subject must be at most 500 characters"
	}
	if description == "" {
		return nil, 400, "Description is required"
	}
	if len(description) > 5000 {
		return nil, 400, "Description must be at most 5000 characters"
	}

	// Validate priority
	if priority == "" {
		priority = "medium"
	}
	validPriorities := map[string]bool{
		"low": true, "medium": true, "high": true, "critical": true,
	}
	if !validPriorities[priority] {
		return nil, 400, "Priority must be one of: low, medium, high, critical"
	}

	// Calculate SLA deadlines
	now := time.Now()
	responseHours := s.ticketRepo.GetConfigInt("ticket.sla_response_hours", 4)
	resolutionDays := s.ticketRepo.GetConfigInt("ticket.sla_resolution_days", 3)

	slaResponseDeadline := s.ticketRepo.CalculateSLAResponseDeadline(now, responseHours)
	slaResolutionDeadline := now.Add(time.Duration(resolutionDays) * 24 * time.Hour)

	ticketNumber := fmt.Sprintf("TKT-%s-%05d", now.Format("20060102"), now.UnixNano()%100000)

	ticket := &models.Ticket{
		TicketNumber:          ticketNumber,
		Type:                  ticketType,
		Subject:               subject,
		Description:           description,
		Priority:              priority,
		CreatedBy:             userID,
		RelatedEntityType:     relatedEntityType,
		RelatedEntityID:       relatedEntityID,
		SLAResponseDeadline:   &slaResponseDeadline,
		SLAResolutionDeadline: &slaResolutionDeadline,
	}

	if err := s.ticketRepo.Create(ticket); err != nil {
		log.Printf("Error creating ticket: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Ticket created: %s type=%s user=%s", ticket.TicketNumber, ticketType, userID)
	return ticket, 201, ""
}

// ListTickets returns paginated tickets based on role.
func (s *TicketService) ListTickets(userID, role string, page, pageSize int, status, ticketType, priority, assignedTo string) ([]models.Ticket, int, error) {
	if role == "member" {
		return s.ticketRepo.ListByUser(userID, page, pageSize, status, ticketType)
	}
	return s.ticketRepo.ListAll(page, pageSize, status, ticketType, priority, assignedTo)
}

// GetTicket returns a ticket with comments, enforcing ownership for members.
func (s *TicketService) GetTicket(ticketID, userID, role string) (*models.Ticket, int, string) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		log.Printf("Error finding ticket %s: %v", ticketID, err)
		return nil, 500, "Internal server error"
	}
	if ticket == nil {
		return nil, 404, "Ticket not found"
	}

	// Members can only see their own tickets
	if role == "member" && ticket.CreatedBy != userID {
		return nil, 403, "Access denied"
	}

	// Load comments
	comments, err := s.ticketRepo.GetComments(ticketID)
	if err != nil {
		log.Printf("Error loading comments for ticket %s: %v", ticketID, err)
		return nil, 500, "Internal server error"
	}
	if comments == nil {
		comments = []models.TicketComment{}
	}
	ticket.Comments = comments

	return ticket, 200, ""
}

// AssignTicket assigns a ticket to a staff/admin user.
func (s *TicketService) AssignTicket(ticketID, assignedTo string) (int, string) {
	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		log.Printf("Error finding ticket %s: %v", ticketID, err)
		return 500, "Internal server error"
	}
	if ticket == nil {
		return 404, "Ticket not found"
	}
	if ticket.Status == "closed" {
		return 422, "Cannot assign a closed ticket"
	}

	// Validate the assigned user exists and is staff/admin
	assignee, err := s.userRepo.FindByID(assignedTo)
	if err != nil {
		log.Printf("Error finding assignee %s: %v", assignedTo, err)
		return 500, "Internal server error"
	}
	if assignee == nil {
		return 404, "Assignee user not found"
	}
	if assignee.Role != "staff" && assignee.Role != "admin" && assignee.Role != "moderator" {
		return 422, "Tickets can only be assigned to staff, moderator, or admin users"
	}

	if err := s.ticketRepo.Assign(ticketID, assignedTo); err != nil {
		log.Printf("Error assigning ticket %s: %v", ticketID, err)
		return 500, "Internal server error"
	}

	log.Printf("Ticket assigned: %s to=%s", ticket.TicketNumber, assignedTo)
	return 200, ""
}

// UpdateTicketStatus transitions a ticket's status.
func (s *TicketService) UpdateTicketStatus(ticketID, userID, role, newStatus string) (int, string) {
	validStatuses := map[string]bool{
		"in_progress": true, "resolved": true, "closed": true,
	}
	if !validStatuses[newStatus] {
		return 400, "Status must be one of: in_progress, resolved, closed"
	}

	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		log.Printf("Error finding ticket %s: %v", ticketID, err)
		return 500, "Internal server error"
	}
	if ticket == nil {
		return 404, "Ticket not found"
	}

	// Validate transition
	validTransitions := map[string]map[string]bool{
		"open":        {"in_progress": true, "closed": true},
		"assigned":    {"in_progress": true, "closed": true},
		"in_progress": {"resolved": true, "closed": true},
		"resolved":    {"closed": true},
	}
	if !validTransitions[ticket.Status][newStatus] {
		return 422, fmt.Sprintf("Cannot transition from '%s' to '%s'", ticket.Status, newStatus)
	}

	// Staff can only update tickets assigned to them (admin can update any)
	if role == "staff" || role == "moderator" {
		if ticket.AssignedTo == nil || *ticket.AssignedTo != userID {
			return 403, "You can only update tickets assigned to you"
		}
	}

	if err := s.ticketRepo.UpdateStatus(ticketID, newStatus); err != nil {
		log.Printf("Error updating ticket %s status: %v", ticketID, err)
		return 500, "Internal server error"
	}

	log.Printf("Ticket status updated: %s %s→%s by=%s", ticket.TicketNumber, ticket.Status, newStatus, userID)
	return 200, ""
}

// AddComment adds a comment to a ticket.
func (s *TicketService) AddComment(ticketID, userID, role, content string) (*models.TicketComment, int, string) {
	if content == "" {
		return nil, 400, "Content is required"
	}
	if len(content) > 5000 {
		return nil, 400, "Content must be at most 5000 characters"
	}

	ticket, err := s.ticketRepo.FindByID(ticketID)
	if err != nil {
		log.Printf("Error finding ticket %s: %v", ticketID, err)
		return nil, 500, "Internal server error"
	}
	if ticket == nil {
		return nil, 404, "Ticket not found"
	}
	if ticket.Status == "closed" {
		return nil, 422, "Cannot comment on a closed ticket"
	}

	// Members can only comment on their own tickets
	if role == "member" && ticket.CreatedBy != userID {
		return nil, 403, "Access denied"
	}

	comment := &models.TicketComment{
		TicketID: ticketID,
		UserID:   userID,
		Content:  content,
	}

	if err := s.ticketRepo.AddComment(comment); err != nil {
		log.Printf("Error adding comment to ticket %s: %v", ticketID, err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Comment added: ticket=%s by=%s", ticket.TicketNumber, userID)
	return comment, 201, ""
}
