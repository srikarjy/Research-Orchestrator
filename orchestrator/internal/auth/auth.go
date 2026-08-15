// Package auth provides user accounts, API keys, and per-identity rate
// limiting for the gateway.
//
// Two credential kinds, matching how a research platform is actually used:
//   - JWT bearer tokens (HS256), issued by POST /api/v1/auth/login, for the
//     browser UI. Short-lived (24h), carry the user id and email.
//   - API keys ("ro_" + 64 hex chars), issued by POST /api/v1/auth/keys, for
//     scripts and programmatic access. Only the SHA-256 of a key is stored;
//     the plaintext is shown once at creation and cannot be recovered.
//
// Auth is enabled iff a JWT secret is configured. When it isn't, the
// middleware passes everything through and logs a prominent warning at
// startup — explicit and loud, never silently open in a deployed
// environment where the secret would be set.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyPrefix distinguishes API keys from JWTs in Authorization headers.
	APIKeyPrefix = "ro_"

	tokenTTL = 24 * time.Hour
)

// Identity is the authenticated caller, set on the request context by the
// middleware.
type Identity struct {
	UserID string
	Email  string
	// Via is "jwt" or "api_key" — useful in audit logs.
	Via string
}

// HashPassword returns the bcrypt hash of a password.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword reports whether password matches the stored bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// NewAPIKey generates a fresh API key, returning the plaintext (shown to the
// user exactly once) and the SHA-256 hex digest that gets stored.
func NewAPIKey() (plaintext, digest string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	plaintext = APIKeyPrefix + hex.EncodeToString(raw)
	return plaintext, HashAPIKey(plaintext), nil
}

// HashAPIKey returns the SHA-256 hex digest of an API key. Lookup by digest
// is constant-time with respect to the key material by construction: the
// digest is computed first and used as an exact index, so no byte-by-byte
// comparison against stored secrets ever happens.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// IsAPIKey reports whether a bearer credential looks like an API key rather
// than a JWT.
func IsAPIKey(credential string) bool {
	return strings.HasPrefix(credential, APIKeyPrefix)
}

// SignToken issues a JWT for the user.
func SignToken(secret []byte, userID, email string, now time.Time) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"iat":   now.Unix(),
		"exp":   now.Add(tokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

// VerifyToken parses and validates a JWT, returning the identity it carries.
func VerifyToken(secret []byte, token string) (Identity, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return Identity{}, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return Identity{}, fmt.Errorf("invalid token")
	}
	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if sub == "" {
		return Identity{}, fmt.Errorf("token missing sub claim")
	}
	return Identity{UserID: sub, Email: email, Via: "jwt"}, nil
}
