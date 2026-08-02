package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token expired")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
)

type Claims struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	SessionID string   `json:"session_id"`
	jwt.RegisteredClaims
}

type AuthConfig struct {
	AccessSecret       string
	RefreshSecret      string
	AccessExpiry       time.Duration
	RefreshExpiry      time.Duration
	Issuer             string
	Audience           string
	BCryptCost         int
}

func DefaultConfig() *AuthConfig {
	return &AuthConfig{
		AccessSecret:  generateSecret(32),
		RefreshSecret: generateSecret(32),
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
		Issuer:        "research-orchestrator",
		Audience:      "research-orchestrator-api",
		BCryptCost:    12,
	}
}

func generateSecret(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func GenerateTokenPair(config *AuthConfig, userID, email string, roles []string, sessionID string) (*TokenPair, error) {
	now := time.Now()
	accessClaims := Claims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(config.AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{config.Audience},
			ID:        generateTokenID(),
		},
	}

	refreshClaims := Claims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(config.RefreshExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{config.Audience},
			ID:        generateTokenID(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(config.AccessSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(config.RefreshSecret))
	if err != nil {
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int(config.AccessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func ValidateAccessToken(config *AuthConfig, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.AccessSecret), nil
	}, jwt.WithIssuer(config.Issuer), jwt.WithAudience(config.Audience), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func ValidateRefreshToken(config *AuthConfig, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.RefreshSecret), nil
	}, jwt.WithIssuer(config.Issuer), jwt.WithAudience(config.Audience), jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func HashPassword(password string, cost int) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateTokenID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", ErrInvalidToken
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidToken
	}
	return parts[1], nil
}

type OAuthProvider string

const (
	OAuthGoogle   OAuthProvider = "google"
	OAuthGitHub   OAuthProvider = "github"
	OAuthMicrosoft OAuthProvider = "microsoft"
)

type OAuthConfig struct {
	Provider    OAuthProvider
	ClientID    string
	ClientSecret string
	RedirectURL string
	Scopes      []string
}

type OAuthUserInfo struct {
	Provider   OAuthProvider `json:"provider"`
	ProviderID string        `json:"provider_id"`
	Email      string        `json:"email"`
	Name       string        `json:"name"`
	AvatarURL  string        `json:"avatar_url"`
	RawData    map[string]interface{} `json:"-"`
}

func (o *OAuthConfig) AuthURL(state string) string {
	baseURLs := map[OAuthProvider]string{
		OAuthGoogle:   "https://accounts.google.com/o/oauth2/v2/auth",
		OAuthGitHub:   "https://github.com/login/oauth/authorize",
		OAuthMicrosoft: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
	}
	baseURL := baseURLs[o.Provider]
	params := map[string]string{
		"client_id":     o.ClientID,
		"redirect_uri":  o.RedirectURL,
		"response_type": "code",
		"scope":         strings.Join(o.Scopes, " "),
		"state":         state,
		"access_type":   "offline",
		"prompt":        "consent",
	}
	query := ""
	for k, v := range params {
		if query != "" {
			query += "&"
		}
		query += k + "=" + v
	}
	return baseURL + "?" + query
}

type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	RefreshToken string    `json:"refresh_token"`
	UserAgent    string    `json:"user_agent"`
	IP           string    `json:"ip"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Revoked      bool      `json:"revoked"`
}

func NewSession(userID, refreshToken, userAgent, ip string, expiry time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:           generateTokenID(),
		UserID:       userID,
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IP:           ip,
		CreatedAt:    now,
		ExpiresAt:    now.Add(expiry),
		Revoked:      false,
	}
}

func (s *Session) IsValid() bool {
	return !s.Revoked && time.Now().Before(s.ExpiresAt)
}

func (s *Session) Revoke() {
	s.Revoked = true
}