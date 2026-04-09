package services

import (
	"encoding/json"
	"fmt"
	"log"

	"campusrec/internal/models"
	"campusrec/internal/repository"
)

type ConfigService struct {
	configRepo *repository.ConfigRepository
}

func NewConfigService(configRepo *repository.ConfigRepository) *ConfigService {
	return &ConfigService{configRepo: configRepo}
}

// ListConfig returns all config entries.
func (s *ConfigService) ListConfig() ([]models.ConfigEntry, error) {
	entries, err := s.configRepo.ListAll()
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.ConfigEntry{}
	}
	return entries, nil
}

// UpdateConfig updates a config entry with audit logging.
func (s *ConfigService) UpdateConfig(key, value string, canaryPercentage *int, userID, ipAddress string) (*models.ConfigEntry, int, string) {
	// Find existing
	existing, err := s.configRepo.FindByKey(key)
	if err != nil {
		log.Printf("Error finding config %s: %v", key, err)
		return nil, 500, "Internal server error"
	}
	if existing == nil {
		return nil, 404, "Configuration key not found"
	}

	// Don't allow modifying internal scheduler keys
	if len(key) > 10 && key[:10] == "scheduler." {
		return nil, 403, "Cannot modify internal scheduler keys"
	}

	if value == "" {
		return nil, 400, "Value is required"
	}

	// Validate canary percentage
	if canaryPercentage != nil && (*canaryPercentage < 0 || *canaryPercentage > 100) {
		return nil, 400, "Canary percentage must be between 0 and 100"
	}

	// Build audit old/new values
	oldJSON, _ := json.Marshal(map[string]interface{}{
		"value":             existing.Value,
		"canary_percentage": existing.CanaryPercentage,
	})
	newJSON, _ := json.Marshal(map[string]interface{}{
		"value":             value,
		"canary_percentage": canaryPercentage,
	})
	oldStr := string(oldJSON)
	newStr := string(newJSON)

	// Update
	if err := s.configRepo.Update(key, value, canaryPercentage, userID); err != nil {
		log.Printf("Error updating config %s: %v", key, err)
		return nil, 500, "Internal server error"
	}

	// Audit log
	if err := s.configRepo.CreateAuditLog("config", existing.ID, "config_update", &oldStr, &newStr, userID, ipAddress); err != nil {
		log.Printf("Warning: failed to create audit log for config %s: %v", key, err)
	}

	// Return updated entry
	updated, err := s.configRepo.FindByKey(key)
	if err != nil {
		log.Printf("Error finding updated config %s: %v", key, err)
		return nil, 500, "Internal server error"
	}

	log.Printf("Config updated: %s by=%s", key, userID)
	return updated, 200, ""
}

// ListCanary returns all canary-enabled configs.
func (s *ConfigService) ListCanary() ([]models.ConfigEntry, error) {
	entries, err := s.configRepo.ListCanary()
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []models.ConfigEntry{}
	}
	return entries, nil
}

// ListAuditLogs returns recent config audit logs.
func (s *ConfigService) ListAuditLogs(limit int) ([]models.AuditLog, error) {
	logs, err := s.configRepo.ListAuditLogs("config", limit)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}
	return logs, nil
}

// IsFeatureEnabled checks if a feature is enabled for a user's canary cohort.
func IsFeatureEnabled(userCohort int, featureKey string, configRepo *repository.ConfigRepository) bool {
	entry, err := configRepo.FindByKey(featureKey)
	if err != nil || entry == nil {
		return true // No config = enabled for all
	}
	if entry.CanaryPercentage == nil {
		return true
	}
	return userCohort < *entry.CanaryPercentage
}

// ComputeCanaryCohort deterministically assigns a cohort (0-99) from a user ID.
func ComputeCanaryCohort(userID string) int {
	hash := 0
	for _, c := range userID {
		hash = (hash*31 + int(c)) % 100
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// GetFeatureStatus returns the canary status for a feature for a specific user.
func (s *ConfigService) GetFeatureStatus(userID, featureKey string) (bool, error) {
	cohort := ComputeCanaryCohort(userID)
	entry, err := s.configRepo.FindByKey(featureKey)
	if err != nil {
		return false, fmt.Errorf("find feature config: %w", err)
	}
	if entry == nil || entry.CanaryPercentage == nil {
		return true, nil
	}
	return cohort < *entry.CanaryPercentage, nil
}
