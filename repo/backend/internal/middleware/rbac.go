package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole returns middleware that restricts access to the specified roles.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		role := GetUserRole(c)
		if !roleSet[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "Insufficient permissions",
			})
			return
		}
		c.Next()
	}
}
