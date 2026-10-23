package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for user authentication and session management.
type Handler struct {
	svc Service
}

// NewHandler constructs an auth HTTP Handler.
func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes attaches authentication endpoints to the provided router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	authGroup := rg.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/refresh", h.Refresh)
		authGroup.POST("/logout", h.Logout)

		// Protected endpoints
		if authMiddleware != nil {
			authGroup.GET("/me", authMiddleware, h.Me)
		}
	}
}

// Register handles user signup.
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registration request: " + err.Error()})
		return
	}

	res, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "user with this email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, res)
}

// Login handles user authentication and token issuance.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid login credentials payload"})
		return
	}

	res, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// Refresh handles atomic refresh token rotation.
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token is required"})
		return
	}

	res, err := h.svc.Refresh(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenReuseBlocked) || errors.Is(err, ErrRefreshTokenRevoked) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token is invalid or has already been used"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token refresh failed"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// Logout revokes the given refresh token.
func (h *Handler) Logout(c *gin.Context) {
	var req RefreshRequest
	_ = c.ShouldBindJSON(&req)

	_ = h.svc.Logout(c.Request.Context(), req)
	c.JSON(http.StatusOK, gin.H{"message": "successfully logged out"})
}

// Me returns the profile of the authenticated user.
func (h *Handler) Me(c *gin.Context) {
	userID, ok := GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	user, err := h.svc.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}
