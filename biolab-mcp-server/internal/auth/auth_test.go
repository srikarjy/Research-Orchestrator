package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidateTokenPair(t *testing.T) {
	config := DefaultConfig()
	userID := "user-123"
	email := "test@example.com"
	roles := []string{"user", "researcher"}
	sessionID := "session-456"

	pair, err := GenerateTokenPair(config, userID, email, roles, sessionID)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, "Bearer", pair.TokenType)
	assert.Equal(t, int(config.AccessExpiry.Seconds()), pair.ExpiresIn)

	claims, err := ValidateAccessToken(config, pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, roles, claims.Roles)
	assert.Equal(t, sessionID, claims.SessionID)
}

func TestValidateExpiredToken(t *testing.T) {
	config := &AuthConfig{
		AccessSecret:  "test-secret",
		RefreshSecret: "test-refresh",
		AccessExpiry:  -time.Hour,
		Issuer:        "test",
		Audience:      "test",
	}

	pair, err := GenerateTokenPair(config, "user", "test@test.com", []string{"user"}, "sess")
	require.NoError(t, err)

	_, err = ValidateAccessToken(config, pair.AccessToken)
	assert.Error(t, err)
	assert.Equal(t, ErrExpiredToken, err)
}

func TestValidateRefreshToken(t *testing.T) {
	config := DefaultConfig()
	pair, err := GenerateTokenPair(config, "user-1", "test@test.com", []string{"user"}, "sess")
	require.NoError(t, err)

	claims, err := ValidateRefreshToken(config, pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
}

func TestHashAndCheckPassword(t *testing.T) {
	password := "secure-password-123"
	hash, err := HashPassword(password, 12)
	require.NoError(t, err)
	assert.NotEqual(t, password, hash)

	assert.True(t, CheckPasswordHash(password, hash))
	assert.False(t, CheckPasswordHash("wrong-password", hash))
}

func TestExtractBearerToken(t *testing.T) {
	token, err := ExtractBearerToken("Bearer abc123")
	require.NoError(t, err)
	assert.Equal(t, "abc123", token)

	_, err = ExtractBearerToken("")
	assert.Error(t, err)

	_, err = ExtractBearerToken("Basic abc123")
	assert.Error(t, err)

	_, err = ExtractBearerToken("Bearer")
	assert.Error(t, err)
}

func TestMemoryUserStore(t *testing.T) {
	store := NewMemoryUserStore()

	user := &User{
		Email:    "test@test.com",
		Name:     "Test User",
		Roles:    []string{"user"},
		PasswordHash: "hashed",
	}
	err := store.CreateUser(user, "hashed")
	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	retrieved, err := store.GetUserByEmail("test@test.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrieved.ID)
	assert.Equal(t, user.Email, retrieved.Email)

	_, err = store.GetUserByEmail("nonexistent@test.com")
	assert.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)
}

func TestMemorySessionStore(t *testing.T) {
	store := NewMemorySessionStore()
	session := NewSession("user-1", "refresh-token", "Mozilla/5.0", "127.0.0.1", time.Hour)

	err := store.SaveSession(session)
	require.NoError(t, err)

	retrieved, err := store.GetSession(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.UserID, retrieved.UserID)
	assert.True(t, retrieved.IsValid())

	err = store.DeleteSession(session.ID)
	require.NoError(t, err)

	_, err = store.GetSession(session.ID)
	assert.Error(t, err)
}

func TestOAuthConfig(t *testing.T) {
	config := &OAuthConfig{
		Provider:    OAuthGoogle,
		ClientID:    "client-id",
		ClientSecret: "secret",
		RedirectURL: "http://localhost/callback",
		Scopes:      []string{"openid", "email", "profile"},
	}

	url := config.AuthURL("state123")
	assert.Contains(t, url, "accounts.google.com")
	assert.Contains(t, url, "client-id")
	assert.Contains(t, url, "state123")
	assert.Contains(t, url, "openid")
	assert.Contains(t, url, "email")
	assert.Contains(t, url, "profile")
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)

	assert.True(t, limiter.Allow("key1"))
	assert.True(t, limiter.Allow("key1"))
	assert.True(t, limiter.Allow("key1"))
	assert.False(t, limiter.Allow("key1"))

	limiter.Cleanup()
}