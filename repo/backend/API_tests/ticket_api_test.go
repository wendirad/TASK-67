//go:build integration

package api_tests

import (
	"encoding/json"
	"testing"
)

func TestCreateTicket(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/tickets", map[string]interface{}{
		"subject":     "Test Ticket",
		"category":    "general",
		"priority":    "medium",
		"description": "This is a test ticket",
	})
	if resp.Code != 201 {
		t.Fatalf("Create ticket failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		ID           string `json:"id"`
		TicketNumber string `json:"ticket_number"`
		Status       string `json:"status"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.ID == "" {
		t.Error("Ticket ID is empty")
	}
	if data.Status != "open" {
		t.Errorf("Status = %q, want open", data.Status)
	}
}

func TestListTickets(t *testing.T) {
	c := getAdminClient(t)
	resp := c.get("/api/tickets")
	if resp.Code != 200 {
		t.Fatalf("List tickets failed: %d %s", resp.Code, resp.Msg)
	}
}

func TestCreateTicketMissingFields(t *testing.T) {
	c := getAdminClient(t)
	resp := c.post("/api/tickets", map[string]interface{}{
		"subject": "",
	})
	if resp.Code == 201 {
		t.Fatal("Expected ticket creation to fail with missing fields")
	}
}

func TestTicketRequiresAuth(t *testing.T) {
	c := newClient(t)
	resp := c.get("/api/tickets")
	if resp.Code != 401 {
		t.Errorf("Expected 401, got %d", resp.Code)
	}
}
