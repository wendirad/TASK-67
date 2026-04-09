package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"

	"campusrec/internal/models"
	"campusrec/internal/repository"

	"github.com/xuri/excelize/v2"
)

type ImportExportService struct {
	jobRepo *repository.JobRepository
}

func NewImportExportService(jobRepo *repository.JobRepository) *ImportExportService {
	return &ImportExportService{jobRepo: jobRepo}
}

// ImportPayload is the JSON stored in job.payload for import jobs.
type ImportPayload struct {
	EntityType string     `json:"entity_type"`
	Filename   string     `json:"filename"`
	FileID     string     `json:"file_id"`
	Rows       [][]string `json:"rows"`
	Headers    []string   `json:"headers"`
	UserID     string     `json:"user_id"`
}

// ExportPayload is the JSON stored in job.payload for export jobs.
type ExportPayload struct {
	EntityType string `json:"entity_type"`
	Format     string `json:"format"`
	Filters    string `json:"filters"`
	UserID     string `json:"user_id"`
}

// ValidationError represents a row-level validation error.
type ValidationError struct {
	Row   int    `json:"row"`
	Field string `json:"field"`
	Error string `json:"error"`
}

// ImportValidationResult is the validation result for an import request.
type ImportValidationResult struct {
	Errors     []ValidationError `json:"errors"`
	ValidCount int               `json:"valid_count"`
	ErrorCount int               `json:"error_count"`
}

// Import validates and creates an import job.
func (s *ImportExportService) Import(userID, entityType string, file multipart.File, header *multipart.FileHeader) (*models.Job, *ImportValidationResult, int, string) {
	// Validate entity type
	validTypes := map[string]bool{
		"sessions": true, "products": true, "users": true, "registrations": true,
	}
	if !validTypes[entityType] {
		return nil, nil, 400, "Entity type must be one of: sessions, products, users, registrations"
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".csv" && ext != ".xlsx" {
		return nil, nil, 400, "File must be a CSV (.csv) or Excel (.xlsx) file"
	}

	// Check file size (10MB)
	if header.Size > 10*1024*1024 {
		return nil, nil, 400, "File must be less than 10MB"
	}

	// Read file content and compute fingerprint
	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading import file: %v", err)
		return nil, nil, 500, "Internal server error"
	}

	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Check for duplicate file
	isDuplicate, err := s.jobRepo.CheckDuplicateFile(hashStr)
	if err != nil {
		log.Printf("Error checking duplicate file: %v", err)
		return nil, nil, 500, "Internal server error"
	}
	if isDuplicate {
		return nil, nil, 409, "This file has already been imported"
	}

	// Parse file into records (headers + data rows)
	var records [][]string
	if ext == ".xlsx" {
		records, err = parseExcelFile(content)
		if err != nil {
			return nil, nil, 400, "Invalid Excel file: " + err.Error()
		}
	} else {
		reader := csv.NewReader(strings.NewReader(string(content)))
		records, err = reader.ReadAll()
		if err != nil {
			return nil, nil, 400, "Invalid CSV format: " + err.Error()
		}
	}

	if len(records) < 2 {
		return nil, nil, 400, "File must have a header row and at least one data row"
	}

	headers := records[0]
	dataRows := records[1:]

	// Validate headers
	if err := validateHeaders(entityType, headers); err != nil {
		return nil, nil, 400, err.Error()
	}

	// Validate each row
	var validationErrors []ValidationError
	for i, row := range dataRows {
		rowNum := i + 2 // 1-indexed, skip header
		rowErrors := validateRow(entityType, headers, row, rowNum)
		validationErrors = append(validationErrors, rowErrors...)
	}

	if len(validationErrors) > 0 {
		return nil, &ImportValidationResult{
			Errors:     validationErrors,
			ValidCount: len(dataRows) - len(validationErrors),
			ErrorCount: len(validationErrors),
		}, 400, "Validation failed"
	}

	// Create file record
	fileRecord := &models.FileRecord{
		Filename:   header.Filename,
		FileType:   "import",
		SHA256Hash: hashStr,
		SizeBytes:  int64(len(content)),
		UploadedBy: &userID,
	}
	if err := s.jobRepo.CreateFileRecord(fileRecord); err != nil {
		log.Printf("Error creating file record: %v", err)
		return nil, nil, 500, "Internal server error"
	}

	// Create import job
	payload := ImportPayload{
		EntityType: entityType,
		Filename:   header.Filename,
		FileID:     fileRecord.ID,
		Rows:       dataRows,
		Headers:    headers,
		UserID:     userID,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadStr := string(payloadJSON)

	job := &models.Job{
		Type:    "import_" + entityType,
		Payload: &payloadStr,
	}
	if err := s.jobRepo.CreateJob(job); err != nil {
		log.Printf("Error creating import job: %v", err)
		return nil, nil, 500, "Internal server error"
	}

	log.Printf("Import job created: %s type=%s rows=%d file=%s", job.ID, entityType, len(dataRows), header.Filename)
	return job, nil, 202, ""
}

// Export creates an export job.
func (s *ImportExportService) Export(userID, entityType, format, filters string) (*models.Job, int, string) {
	validTypes := map[string]bool{
		"sessions": true, "products": true, "users": true, "orders": true, "registrations": true, "tickets": true,
	}
	if !validTypes[entityType] {
		return nil, 400, "Invalid entity type for export"
	}

	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "xlsx" {
		return nil, 400, "Export format must be 'csv' or 'xlsx'"
	}

	payload := ExportPayload{
		EntityType: entityType,
		Format:     format,
		Filters:    filters,
		UserID:     userID,
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadStr := string(payloadJSON)

	job := &models.Job{
		Type:    "export_" + entityType,
		Payload: &payloadStr,
	}
	if err := s.jobRepo.CreateJob(job); err != nil {
		log.Printf("Error creating export job: %v", err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Export job created: %s type=%s format=%s", job.ID, entityType, format)
	return job, 202, ""
}

// GetJob returns a job by ID after verifying the requesting user is authorized.
// Staff and admin users can access any job; other users can only access their own.
func (s *ImportExportService) GetJob(jobID, requestingUserID, requestingUserRole string) (*models.Job, int, string) {
	job, err := s.jobRepo.FindByID(jobID)
	if err != nil {
		log.Printf("Error finding job %s: %v", jobID, err)
		return nil, 500, "Internal server error"
	}
	if job == nil {
		return nil, 404, "Job not found"
	}

	// Staff and admin can view any job
	if requestingUserRole == "staff" || requestingUserRole == "admin" {
		return job, 200, ""
	}

	// For other users, verify ownership via the payload's user_id
	if job.Payload != nil {
		var ownerInfo struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal([]byte(*job.Payload), &ownerInfo); err == nil {
			if ownerInfo.UserID == requestingUserID {
				return job, 200, ""
			}
		}
	}

	return nil, 403, "You do not have permission to view this job"
}

// validateHeaders checks that required columns exist for the entity type.
func validateHeaders(entityType string, headers []string) error {
	headerSet := make(map[string]bool)
	for _, h := range headers {
		headerSet[strings.TrimSpace(strings.ToLower(h))] = true
	}

	var required []string
	switch entityType {
	case "sessions":
		required = []string{"title", "facility_name", "start_time", "end_time", "total_seats"}
	case "products":
		required = []string{"name", "category", "price_cents", "stock_quantity"}
	case "users":
		required = []string{"username", "password", "role", "display_name"}
	case "registrations":
		required = []string{"username", "session_title"}
	}

	var missing []string
	for _, r := range required {
		if !headerSet[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}
	return nil
}

// validateRow validates a single row for the given entity type.
func validateRow(entityType string, headers, row []string, rowNum int) []ValidationError {
	var errors []ValidationError

	if len(row) != len(headers) {
		return []ValidationError{{Row: rowNum, Field: "*", Error: fmt.Sprintf("expected %d columns, got %d", len(headers), len(row))}}
	}

	// Build field map
	fields := make(map[string]string)
	for i, h := range headers {
		fields[strings.TrimSpace(strings.ToLower(h))] = strings.TrimSpace(row[i])
	}

	switch entityType {
	case "sessions":
		if fields["title"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "title", Error: "Title is required"})
		}
		if fields["facility_name"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "facility_name", Error: "Facility name is required"})
		}
		if fields["start_time"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "start_time", Error: "Start time is required"})
		}
		if fields["end_time"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "end_time", Error: "End time is required"})
		}
		if fields["total_seats"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "total_seats", Error: "Total seats is required"})
		}

	case "products":
		if fields["name"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "name", Error: "Name is required"})
		}
		if fields["category"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "category", Error: "Category is required"})
		}
		if fields["price_cents"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "price_cents", Error: "Price is required"})
		}

	case "users":
		if fields["username"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "username", Error: "Username is required"})
		}
		if fields["password"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "password", Error: "Password is required"})
		}
		validRoles := map[string]bool{"member": true, "staff": true, "moderator": true}
		if !validRoles[fields["role"]] {
			errors = append(errors, ValidationError{Row: rowNum, Field: "role", Error: "Role must be member, staff, or moderator"})
		}
		if fields["display_name"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "display_name", Error: "Display name is required"})
		}

	case "registrations":
		if fields["username"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "username", Error: "Username is required"})
		}
		if fields["session_title"] == "" {
			errors = append(errors, ValidationError{Row: rowNum, Field: "session_title", Error: "Session title is required"})
		}
	}

	return errors
}

// parseExcelFile reads an xlsx file from raw bytes and returns all rows from the first sheet.
func parseExcelFile(content []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
