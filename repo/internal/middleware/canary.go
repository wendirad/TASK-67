package middleware

import (
	"campusrec/internal/models"
	"campusrec/internal/repository"

	"github.com/gin-gonic/gin"
)

// CanaryMiddleware loads the authenticated user's canary_cohort from the DB
// and sets it in the request context. Must be placed after AuthRequired.
func CanaryMiddleware(configRepo *repository.ConfigRepository, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == "" {
			c.Next()
			return
		}

		user, err := userRepo.FindByID(userID)
		if err != nil || user == nil {
			c.Next()
			return
		}

		cohort := -1
		if user.CanaryCohort != nil {
			cohort = *user.CanaryCohort
		}
		c.Set("canary_cohort", cohort)
		c.Set("config_repo", configRepo)
		c.Next()
	}
}

// GetCanaryCohort retrieves the user's canary cohort from the context.
// Returns -1 if not set (which means features are disabled for this user
// unless canary_percentage is NULL, i.e., fully rolled out).
func GetCanaryCohort(c *gin.Context) int {
	if v, exists := c.Get("canary_cohort"); exists {
		if cohort, ok := v.(int); ok {
			return cohort
		}
	}
	return -1
}

// IsFeatureEnabledForRequest checks whether a canary-gated feature is enabled
// for the current request's user. If the config key has no canary_percentage
// (NULL), the feature is enabled for all. Otherwise, enabled only if the user's
// cohort is below the percentage threshold.
func IsFeatureEnabledForRequest(c *gin.Context, featureKey string) bool {
	cohort := GetCanaryCohort(c)

	v, exists := c.Get("config_repo")
	if !exists {
		return true // no config repo = feature on
	}
	configRepo, ok := v.(*repository.ConfigRepository)
	if !ok {
		return true
	}

	entry, err := configRepo.FindByKey(featureKey)
	if err != nil || entry == nil {
		return true // key not found = feature on for all
	}
	return models.CanaryEnabled(entry.CanaryPercentage, cohort)
}
