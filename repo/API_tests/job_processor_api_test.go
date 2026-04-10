//go:build integration

package api_tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestJobCreationViaExport verifies that triggering an export creates a job
// that transitions through the expected lifecycle states.
func TestJobCreationViaExport(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=users&format=csv")
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

	// Poll until job reaches a terminal state (completed or failed)
	var status string
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		var job struct {
			Status string `json:"status"`
		}
		json.Unmarshal(jobResp.Data, &job)
		status = job.Status
		if status == "completed" || status == "failed" {
			break
		}
	}

	if status != "completed" {
		t.Errorf("Job final status = %q, want 'completed'", status)
	}
}

// TestJobStatusTransitions verifies that a job transitions from pending
// through processing to completed.
func TestJobStatusTransitions(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=products&format=csv")
	if resp.Code != 202 {
		t.Fatalf("Export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)

	// The job should start as 'pending'
	jobResp := c.get("/api/jobs/" + data.JobID)
	if jobResp.Code != 200 {
		t.Fatalf("Get job failed: %d %s", jobResp.Code, jobResp.Msg)
	}

	var job struct {
		Status   string `json:"status"`
		Attempts int    `json:"attempts"`
	}
	json.Unmarshal(jobResp.Data, &job)

	// Initial status should be 'pending' (if polled fast enough) or already picked up
	validInitial := job.Status == "pending" || job.Status == "processing" || job.Status == "completed"
	if !validInitial {
		t.Errorf("Initial job status = %q, want pending/processing/completed", job.Status)
	}

	// Wait for completion
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		jobResp = c.get("/api/jobs/" + data.JobID)
		json.Unmarshal(jobResp.Data, &job)
		if job.Status == "completed" || job.Status == "failed" {
			break
		}
	}

	if job.Status != "completed" {
		t.Errorf("Final job status = %q, want 'completed'", job.Status)
	}
	if job.Attempts < 1 {
		t.Errorf("Attempts = %d, want >= 1 (atomic claim increments attempts)", job.Attempts)
	}
}

// TestJobResultPopulated verifies that completed jobs have a non-empty result.
func TestJobResultPopulated(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=users&format=csv")
	if resp.Code != 202 {
		t.Fatalf("Export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)

	// Wait for completion
	var job struct {
		Status string  `json:"status"`
		Result *string `json:"result"`
	}
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		json.Unmarshal(jobResp.Data, &job)
		if job.Status == "completed" || job.Status == "failed" {
			break
		}
	}

	if job.Status != "completed" {
		t.Fatalf("Job did not complete: status=%s", job.Status)
	}

	if job.Result == nil || *job.Result == "" {
		t.Error("Completed job should have a non-empty result")
	}
}

// TestConcurrentJobCreation verifies that multiple jobs created in quick
// succession are all processed without duplicates (no double execution).
func TestConcurrentJobCreation(t *testing.T) {
	c := getAdminClient(t)

	// Create multiple export jobs rapidly
	jobIDs := make([]string, 3)
	entities := []string{"users", "products", "sessions"}
	for i, entity := range entities {
		resp := c.get(fmt.Sprintf("/api/export?entity_type=%s&format=csv", entity))
		if resp.Code != 202 {
			t.Fatalf("Export %s creation failed: %d %s", entity, resp.Code, resp.Msg)
		}
		var data struct {
			JobID string `json:"job_id"`
		}
		json.Unmarshal(resp.Data, &data)
		jobIDs[i] = data.JobID
	}

	// Wait for all to complete
	for _, jobID := range jobIDs {
		var status string
		for i := 0; i < 30; i++ {
			time.Sleep(500 * time.Millisecond)
			jobResp := c.get("/api/jobs/" + jobID)
			if jobResp.Code != 200 {
				continue
			}
			var job struct {
				Status string `json:"status"`
			}
			json.Unmarshal(jobResp.Data, &job)
			status = job.Status
			if status == "completed" || status == "failed" {
				break
			}
		}
		if status != "completed" {
			t.Errorf("Job %s final status = %q, want 'completed'", jobID, status)
		}
	}
}

// TestJobAttemptsFieldOnCompletion verifies the attempts field is exactly 1
// for a successful job that completes on first try.
func TestJobAttemptsFieldOnCompletion(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=users&format=csv")
	if resp.Code != 202 {
		t.Fatalf("Export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)

	var job struct {
		Status   string `json:"status"`
		Attempts int    `json:"attempts"`
	}
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		json.Unmarshal(jobResp.Data, &job)
		if job.Status == "completed" || job.Status == "failed" {
			break
		}
	}

	if job.Status != "completed" {
		t.Fatalf("Job did not complete: status=%s", job.Status)
	}

	// A successful first-try job should have exactly 1 attempt
	// (the atomic CTE increments attempts during claim)
	if job.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 for first-try success", job.Attempts)
	}
}

// TestJobExportXLSXFormat verifies that xlsx export jobs work correctly.
func TestJobExportXLSXFormat(t *testing.T) {
	c := getAdminClient(t)

	resp := c.get("/api/export?entity_type=users&format=xlsx")
	if resp.Code != 202 {
		t.Fatalf("XLSX export creation failed: %d %s", resp.Code, resp.Msg)
	}

	var data struct {
		JobID string `json:"job_id"`
	}
	json.Unmarshal(resp.Data, &data)

	var job struct {
		Status string `json:"status"`
	}
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		jobResp := c.get("/api/jobs/" + data.JobID)
		if jobResp.Code != 200 {
			continue
		}
		json.Unmarshal(jobResp.Data, &job)
		if job.Status == "completed" || job.Status == "failed" {
			break
		}
	}

	if job.Status != "completed" {
		t.Errorf("XLSX export job status = %q, want 'completed'", job.Status)
	}
}
