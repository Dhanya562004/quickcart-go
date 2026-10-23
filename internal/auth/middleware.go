package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	ContextUserID    = "auth_user_id"
	ContextUserEmail = "auth_user_email"
	ContextUserRole  = "auth_user_role"
	ContextClaims    = "auth_claims"
)

// Authenticate extracts and validates JWT access tokens from HTTP requests.
// Supported Token Transports:
// 1. Standard Authorization header: "Bearer <token>"
// 2. Query parameter: "?token=<token>" (Required for native browser EventSource SSE connections)
//
// Security Invariant:
// Client-supplied headers (e.g. X-User-ID, X-User-Role) and unauthenticated query params
// are NEVER trusted for authorization. All identity is strictly derived from verified JWT signatures.
func Authenticate(tokenMgr TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// 1. Check Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				tokenStr = strings.TrimSpace(parts[1])
			}
		}

		// 2. Fallback to query parameter (for browser EventSource)
		if tokenStr == "" {
			tokenStr = strings.TrimSpace(c.Query("token"))
		}

		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized; authentication token required",
			})
			return
		}

		// 3. Validate token signature, claims, and expiry
		claims, err := tokenMgr.ValidateAccessToken(tokenStr)
		if err != nil {
			if errors.Is(err, ErrExpiredToken) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "unauthorized; token has expired",
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized; invalid or malformed token",
			})
			return
		}

		// 4. Inject authenticated server-side identity into request context
		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUserEmail, claims.Email)
		c.Set(ContextUserRole, claims.Role)
		c.Set(ContextClaims, claims)

		c.Next()
	}
}

// RequireRole enforces role-based access control (RBAC).
// Returns HTTP 401 if unauthenticated and HTTP 403 if the user lacks the required role.
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		roleSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		roleVal, exists := c.Get(ContextUserRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized; authentication required",
			})
			return
		}

		userRole, ok := roleVal.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "forbidden; invalid user role context",
			})
			return
		}

		if _, allowed := roleSet[userRole]; !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "forbidden; you do not have permission to perform this action",
			})
			return
		}

		c.Next()
	}
}

// GetUserID retrieves the authenticated UserID from Gin context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(ContextUserID)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// MustGetUserID retrieves the authenticated UserID or returns uuid.Nil.
func MustGetUserID(c *gin.Context) uuid.UUID {
	id, _ := GetUserID(c)
	return id
}

// GetUserRole retrieves the authenticated user role from Gin context.
func GetUserRole(c *gin.Context) string {
	val, exists := c.Get(ContextUserRole)
	if !exists {
		return ""
	}
	role, _ := val.(string)
	return role
}
