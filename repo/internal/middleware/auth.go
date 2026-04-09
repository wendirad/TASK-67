package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims stored in the JWT token.
type JWTClaims struct {
	Sub         string `json:"sub"`
	Role        string `json:"role"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token for the given user.
func GenerateToken(userID, role, username, displayName, secret string) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		Sub:         userID,
		Role:        role,
		Username:    username,
		DisplayName: displayName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// AuthRequired validates the JWT from the session_token cookie or Authorization header.
func AuthRequired(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "Authentication required",
			})
			return
		}

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": 401,
				"msg":  "Invalid or expired token",
			})
			return
		}

		c.Set("user_id", claims.Sub)
		c.Set("user_role", claims.Role)
		c.Set("username", claims.Username)
		c.Set("display_name", claims.DisplayName)
		c.Next()
	}
}

// extractToken reads the token from the session_token cookie first, then falls back to
// the Authorization: Bearer header.
func extractToken(c *gin.Context) string {
	if cookie, err := c.Cookie("session_token"); err == nil && cookie != "" {
		return cookie
	}
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

// GetUserID retrieves the authenticated user's ID from the context.
func GetUserID(c *gin.Context) string {
	id, _ := c.Get("user_id")
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}

// GetUserRole retrieves the authenticated user's role from the context.
func GetUserRole(c *gin.Context) string {
	role, _ := c.Get("user_role")
	if s, ok := role.(string); ok {
		return s
	}
	return ""
}
