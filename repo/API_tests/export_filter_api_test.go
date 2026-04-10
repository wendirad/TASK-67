//go:build integration

package api_tests

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestExportFilterValidationRejectsInvalidStatus(t *testing.T) {
	c := getAdminClient(t)

	filters := url.QueryEscape(`{"status":"nonexistent"}`)
	resp := c.get("/api/export?entity_type=users&format=csv&filters=" + filters)
	if resp.Code != 400 {
		t.Errorf("Expected 400 for invalid status filter, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportFilterValidationRejectsWrongEntityFilter(t *testing.T) {
	c := getAdminClient(t)

	// role filter is only valid for users, not orders
	filters := url.QueryEscape(`{"role":"member"}`)
	resp := c.get("/api/export?entity_type=orders&format=csv&filters=" + filters)
	if resp.Code != 400 {
		t.Errorf("Expected 400 for role filter on orders, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportFilterValidationRejectsInvalidJSON(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=users&format=csv&filters=" + url.QueryEscape(`{bad`))
	if resp.Code != 400 {
		t.Errorf("Expected 400 for invalid JSON filters, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportFilterValidationRejectsBadDateFormat(t *testing.T) {
	c := getAdminClient(t)

	filters := url.QueryEscape(`{"date_from":"01/01/2026"}`)
	resp := c.get("/api/export?entity_type=users&format=csv&filters=" + filters)
	if resp.Code != 400 {
		t.Errorf("Expected 400 for invalid date format, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportFilterValidationAcceptsValidFilters(t *testing.T) {
	c := getAdminClient(t)

	filters := url.QueryEscape(`{"status":"active","role":"admin"}`)
	resp := c.get("/api/export?entity_type=users&format=csv&filters=" + filters)
	if resp.Code != 202 {
		t.Errorf("Expected 202 for valid filters, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportWithoutFiltersStillWorks(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=products&format=csv")
	if resp.Code != 202 {
		t.Errorf("Expected 202 for export without filters, got %d: %s", resp.Code, resp.Msg)
	}
}

func TestExportFilteredResultContainsOnlyMatchingRows(t *testing.T) {
	c := getAdminClient(t)

	// Export only active products — there are seeded products with status=active
	filters := url.QueryEscape(`{"status":"active"}`)
	resp := c.get("/api/export?entity_type=products&format=csv&filters=" + filters)
	if resp.Code != 202 {
		t.Fatalf("Export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)
	if data.JobID == "" {
		t.Fatal("Job ID should not be empty")
	}

	// Wait for the job to complete (worker processes every few seconds)
	var jobResult struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		json.Unmarshal(jobResp.Data, &jobResult)
		if jobResult.Status == "completed" {
			break
		}
	}
	if jobResult.Status != "completed" {
		t.Fatalf("Export job did not complete, status=%s", jobResult.Status)
	}

	// Parse the result to verify row count and data
	var exportResult struct {
		RowCount int    `json:"row_count"`
		Format   string `json:"format"`
		CSVData  string `json:"csv_data"`
	}
	if err := json.Unmarshal([]byte(jobResult.Result), &exportResult); err != nil {
		// Result is stored as a JSON string, try double-decode
		var resultStr string
		json.Unmarshal(jobResult.Result, &resultStr)
		json.Unmarshal([]byte(resultStr), &exportResult)
	}

	if exportResult.RowCount == 0 {
		t.Error("Filtered export should contain at least one active product")
	}

	// Verify the CSV only contains active products
	if exportResult.CSVData != "" {
		lines := splitCSVLines(exportResult.CSVData)
		// First line is header, rest are data
		for i, line := range lines {
			if i == 0 {
				continue // header
			}
			if line == "" {
				continue
			}
			// Status is the last column in products export
			// headers: id, name, category, price_cents, stock_quantity, is_shippable, status
			fields := parseCSVLine(line)
			if len(fields) >= 7 && fields[6] != "active" {
				t.Errorf("Row %d has status=%q, expected 'active' (filtered export)", i, fields[6])
			}
		}
	}
}

func TestExportFilteredNoResults(t *testing.T) {
	c := getAdminClient(t)

	// Export with a status that no product has (out_of_stock — seed data has all active)
	filters := url.QueryEscape(`{"status":"out_of_stock"}`)
	resp := c.get("/api/export?entity_type=products&format=csv&filters=" + filters)
	if resp.Code != 202 {
		t.Fatalf("Export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)

	var jobResult struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
	}
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		json.Unmarshal(jobResp.Data, &jobResult)
		if jobResult.Status == "completed" {
			break
		}
	}
	if jobResult.Status != "completed" {
		t.Fatalf("Export job did not complete, status=%s", jobResult.Status)
	}

	var exportResult struct {
		RowCount int `json:"row_count"`
	}
	if err := json.Unmarshal([]byte(jobResult.Result), &exportResult); err != nil {
		var resultStr string
		json.Unmarshal(jobResult.Result, &resultStr)
		json.Unmarshal([]byte(resultStr), &exportResult)
	}

	if exportResult.RowCount != 0 {
		t.Errorf("Expected 0 rows for out_of_stock filter, got %d", exportResult.RowCount)
	}
}

// splitCSVLines splits CSV data into lines, handling \r\n and \n.
func splitCSVLines(data string) []string {
	var lines []string
	current := ""
	for _, ch := range data {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else if ch != '\r' {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// parseCSVLine splits a single CSV line by commas (simple, no quoted fields with commas).
func parseCSVLine(line string) []string {
	var fields []string
	current := ""
	inQuote := false
	for _, ch := range line {
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ',' && !inQuote:
			fields = append(fields, current)
			current = ""
		default:
			current += string(ch)
		}
	}
	fields = append(fields, current)
	return fields
}
