package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
)

func setupAuthService() (auth.Service, auth.TokenManager, *auth.FakeRepository) {
	repo := auth.NewFakeRepository()
	tm := auth.NewTokenManager(auth.DefaultTokenConfig("test-secret-key-32-bytes-long-1234"), nil)
	svc := auth.NewService(repo, tm, nil)
	return svc, tm, repo
}

func TestAuthService_Register(t *testing.T) {
	svc, _, _ := setupAuthService()
	ctx := context.Background()

	t.Run("successful registration enforces customer role", func(t *testing.T) {
		res, err := svc.Register(ctx, auth.RegisterRequest{
			Email:    "newuser@quickcart.in",
			Password: "Password123!",
			FullName: "Ananya Sharma",
			Phone:    "9876543210",
		})
		require.NoError(t, err)
		assert.Equal(t, "newuser@quickcart.in", res.User.Email)
		assert.Equal(t, auth.RoleCustomer, res.User.Role, "public registration MUST assign RoleCustomer")
		assert.NotEmpty(t, res.Tokens.AccessToken)
		assert.NotEmpty(t, res.Tokens.RefreshToken)
	})

	t.Run("duplicate email rejected with conflict", func(t *testing.T) {
		_, err := svc.Register(ctx, auth.RegisterRequest{
			Email:    "newuser@quickcart.in",
			Password: "Password456!",
			FullName: "Duplicate User",
			Phone:    "9876543211",
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, auth.ErrUserAlreadyExists)
	})
}

func TestAuthService_Login(t *testing.T) {
	svc, _, _ := setupAuthService()
	ctx := context.Background()

	_, err := svc.Register(ctx, auth.RegisterRequest{
		Email:    "loginuser@quickcart.in",
		Password: "SecurePassword123",
		FullName: "Vikram Malhotra",
		Phone:    "9876543212",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		req         auth.LoginRequest
		expectedErr error
	}{
		{
			name: "successful login with correct credentials",
			req: auth.LoginRequest{
				Email:    "loginuser@quickcart.in",
				Password: "SecurePassword123",
			},
			expectedErr: nil,
		},
		{
			name: "failed login with incorrect password",
			req: auth.LoginRequest{
				Email:    "loginuser@quickcart.in",
				Password: "WrongPassword!",
			},
			expectedErr: auth.ErrInvalidCredentials,
		},
		{
			name: "failed login with non-existent user",
			req: auth.LoginRequest{
				Email:    "nonexistent@quickcart.in",
				Password: "Password123",
			},
			expectedErr: auth.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.Login(ctx, tt.req)
			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.expectedErr)
				assert.Nil(t, res)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, res.Tokens.AccessToken)
				assert.NotEmpty(t, res.Tokens.RefreshToken)
			}
		})
	}
}

func TestAuthService_RefreshTokenRotation(t *testing.T) {
	svc, _, _ := setupAuthService()
	ctx := context.Background()

	regRes, err := svc.Register(ctx, auth.RegisterRequest{
		Email:    "rotate@quickcart.in",
		Password: "Password123!",
		FullName: "Rohan Verma",
		Phone:    "9876543213",
	})
	require.NoError(t, err)

	originalRefresh := regRes.Tokens.RefreshToken

	// 1. First rotation must succeed and issue a new token pair
	refreshRes, err := svc.Refresh(ctx, auth.RefreshRequest{RefreshToken: originalRefresh})
	require.NoError(t, err)
	assert.NotEmpty(t, refreshRes.Tokens.AccessToken)
	assert.NotEmpty(t, refreshRes.Tokens.RefreshToken)
	assert.NotEqual(t, originalRefresh, refreshRes.Tokens.RefreshToken, "new refresh token must differ from old")

	// 2. Replay attack attempt: Reusing the already rotated token must be REJECTED immediately
	replayRes, err := svc.Refresh(ctx, auth.RefreshRequest{RefreshToken: originalRefresh})
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrRefreshTokenReuseBlocked)
	assert.Nil(t, replayRes)
}

func TestAuthService_ConcurrentRefreshTokenRotation(t *testing.T) {
	svc, _, _ := setupAuthService()
	ctx := context.Background()

	regRes, err := svc.Register(ctx, auth.RegisterRequest{
		Email:    "concur_rotate@quickcart.in",
		Password: "Password123!",
		FullName: "Pooja Hegde",
		Phone:    "9876543214",
	})
	require.NoError(t, err)

	refreshToken := regRes.Tokens.RefreshToken

	// Run two concurrent rotation attempts on the identical refresh token
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var rejectCount atomic.Int32

	startBarrier := make(chan struct{})

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier
			_, err := svc.Refresh(ctx, auth.RefreshRequest{RefreshToken: refreshToken})
			if err == nil {
				successCount.Add(1)
			} else {
				rejectCount.Add(1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	// Exactly ONE must succeed, and the concurrent replay must be rejected
	assert.Equal(t, int32(1), successCount.Load(), "exactly one rotation must succeed")
	assert.Equal(t, int32(1), rejectCount.Load(), "concurrent rotation attempt must be rejected")
}

func TestRBAC_Middleware_Enforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, tm, repo := setupAuthService()
	ctx := context.Background()

	// Create customer and admin users
	custUser, err := repo.CreateUser(ctx, auth.User{
		Email: "customer@quickcart.in",
		Role:  auth.RoleCustomer,
	})
	require.NoError(t, err)
	custTokens, err := tm.GenerateTokenPair(*custUser)
	require.NoError(t, err)

	adminUser, err := repo.CreateUser(ctx, auth.User{
		Email: "admin@quickcart.in",
		Role:  auth.RoleAdmin,
	})
	require.NoError(t, err)
	adminTokens, err := tm.GenerateTokenPair(*adminUser)
	require.NoError(t, err)

	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(auth.Authenticate(tm))
	{
		// Customer endpoint (accessible by both customer and admin)
		v1.GET("/customer/orders", auth.RequireRole(auth.RoleCustomer, auth.RoleAdmin), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Admin-only endpoint
		v1.POST("/admin/products", auth.RequireRole(auth.RoleAdmin), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "created"})
		})
	}

	// 1. Unauthenticated request -> 401 Unauthorized
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/customer/orders", nil)
	wNoAuth := httptest.NewRecorder()
	r.ServeHTTP(wNoAuth, reqNoAuth)
	assert.Equal(t, http.StatusUnauthorized, wNoAuth.Code)

	// 2. Customer accessing Customer endpoint -> 200 OK
	reqCustOk := httptest.NewRequest(http.MethodGet, "/api/v1/customer/orders", nil)
	reqCustOk.Header.Set("Authorization", "Bearer "+custTokens.AccessToken)
	wCustOk := httptest.NewRecorder()
	r.ServeHTTP(wCustOk, reqCustOk)
	assert.Equal(t, http.StatusOK, wCustOk.Code)

	// 3. Customer accessing Admin-only endpoint -> 403 Forbidden
	reqCustAdmin := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", nil)
	reqCustAdmin.Header.Set("Authorization", "Bearer "+custTokens.AccessToken)
	wCustAdmin := httptest.NewRecorder()
	r.ServeHTTP(wCustAdmin, reqCustAdmin)
	assert.Equal(t, http.StatusForbidden, wCustAdmin.Code)

	// 4. Admin accessing Admin-only endpoint -> 200 OK
	reqAdminOk := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", nil)
	reqAdminOk.Header.Set("Authorization", "Bearer "+adminTokens.AccessToken)
	wAdminOk := httptest.NewRecorder()
	r.ServeHTTP(wAdminOk, reqAdminOk)
	assert.Equal(t, http.StatusOK, wAdminOk.Code)

	_ = svc
}
