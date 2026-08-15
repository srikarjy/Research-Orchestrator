package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// fakeStore is an in-memory Store for middleware/handler tests.
type fakeStore struct {
	users  map[string]User   // by email
	keys   map[string]string // digest -> userID
	nextID int
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[string]User{}, keys: map[string]string{}}
}

func (f *fakeStore) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	if _, exists := f.users[email]; exists {
		return User{}, ErrDuplicateEmail
	}
	f.nextID++
	u := User{ID: fmt.Sprintf("user-%d", f.nextID), Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}
	f.users[email] = u
	return u, nil
}

func (f *fakeStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	u, ok := f.users[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) CreateAPIKey(ctx context.Context, userID, name, keyDigest string) (APIKey, error) {
	f.keys[keyDigest] = userID
	return APIKey{ID: "key-1", UserID: userID, Name: name, CreatedAt: time.Now()}, nil
}

func (f *fakeStore) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	return nil, nil
}

func (f *fakeStore) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	return nil
}

func (f *fakeStore) GetIdentityByKeyDigest(ctx context.Context, digest string) (Identity, error) {
	userID, ok := f.keys[digest]
	if !ok {
		return Identity{}, ErrNotFound
	}
	return Identity{UserID: userID, Via: "api_key"}, nil
}

const testSecret = "test-secret-for-auth-tests"

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !CheckPassword(hash, "correct horse battery") {
		t.Error("correct password rejected")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("wrong password accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Error("expected error for a password under 8 characters")
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	plaintext, digest, err := NewAPIKey()
	if err != nil {
		t.Fatalf("NewAPIKey: %v", err)
	}
	if !IsAPIKey(plaintext) {
		t.Errorf("generated key %q lacks the %q prefix", plaintext, APIKeyPrefix)
	}
	if HashAPIKey(plaintext) != digest {
		t.Error("digest does not match HashAPIKey of the plaintext")
	}
	other, _, _ := NewAPIKey()
	if other == plaintext {
		t.Error("two generated keys are identical")
	}
}

func TestJWTSignVerify(t *testing.T) {
	now := time.Now()
	token, err := SignToken([]byte(testSecret), "user-1", "a@b.com", now)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	id, err := VerifyToken([]byte(testSecret), token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if id.UserID != "user-1" || id.Email != "a@b.com" || id.Via != "jwt" {
		t.Errorf("unexpected identity %+v", id)
	}

	if _, err := VerifyToken([]byte("other-secret"), token); err == nil {
		t.Error("token verified under the wrong secret")
	}

	expired, _ := SignToken([]byte(testSecret), "user-1", "a@b.com", now.Add(-48*time.Hour))
	if _, err := VerifyToken([]byte(testSecret), expired); err == nil {
		t.Error("expired token accepted")
	}

	if _, err := VerifyToken([]byte(testSecret), "not-a-token"); err == nil {
		t.Error("garbage accepted as a token")
	}
}

func protectedRouter(mw *Middleware) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", mw.Require(), func(c *gin.Context) {
		id, _ := IdentityFrom(c)
		c.JSON(http.StatusOK, gin.H{"user": id.UserID, "via": id.Via})
	})
	return r
}

func TestMiddlewareRejectsMissingAndBadCredentials(t *testing.T) {
	mw := NewMiddleware(testSecret, newFakeStore(), nil, zap.NewNop())
	r := protectedRouter(mw)

	for name, header := range map[string]string{
		"no credentials": "",
		"garbage token":  "Bearer nonsense",
		"unknown key":    "Bearer " + APIKeyPrefix + strings.Repeat("0", 64),
	} {
		req := httptest.NewRequest("GET", "/protected", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", name, w.Code)
		}
	}
}

func TestMiddlewareAcceptsJWTAndAPIKey(t *testing.T) {
	store := newFakeStore()
	mw := NewMiddleware(testSecret, store, nil, zap.NewNop())
	r := protectedRouter(mw)

	token, _ := SignToken([]byte(testSecret), "user-7", "x@y.com", time.Now())
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "user-7") {
		t.Errorf("JWT: got %d %s", w.Code, w.Body.String())
	}

	plaintext, digest, _ := NewAPIKey()
	store.keys[digest] = "user-9"
	for _, place := range []string{"Authorization", "X-API-Key"} {
		req := httptest.NewRequest("GET", "/protected", nil)
		if place == "Authorization" {
			req.Header.Set(place, "Bearer "+plaintext)
		} else {
			req.Header.Set(place, plaintext)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "user-9") {
			t.Errorf("API key via %s: got %d %s", place, w.Code, w.Body.String())
		}
	}
}

func TestMiddlewareDisabledPassesThrough(t *testing.T) {
	mw := NewMiddleware("", newFakeStore(), nil, zap.NewNop())
	if mw.Enabled() {
		t.Fatal("middleware with empty secret reports enabled")
	}
	r := protectedRouter(mw)
	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("disabled auth: got %d, want 200 passthrough", w.Code)
	}
}

func doJSON(r *gin.Engine, method, path, body, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRegisterLoginFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newFakeStore()
	h := NewHandlers(testSecret, store, zap.NewNop())
	r := gin.New()
	r.POST("/register", h.Register)
	r.POST("/login", h.Login)

	if w := doJSON(r, "POST", "/register", `{"email":"a@b.com","password":"longenough"}`, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "token") {
		t.Fatalf("register: got %d %s", w.Code, w.Body.String())
	}
	if w := doJSON(r, "POST", "/register", `{"email":"a@b.com","password":"longenough"}`, ""); w.Code != http.StatusConflict {
		t.Errorf("duplicate register: got %d, want 409", w.Code)
	}
	if w := doJSON(r, "POST", "/login", `{"email":"a@b.com","password":"longenough"}`, ""); w.Code != http.StatusOK {
		t.Errorf("login: got %d %s", w.Code, w.Body.String())
	}
	// Wrong password and unknown email must be indistinguishable.
	wrongPw := doJSON(r, "POST", "/login", `{"email":"a@b.com","password":"wrongwrong"}`, "")
	unknown := doJSON(r, "POST", "/login", `{"email":"nobody@b.com","password":"wrongwrong"}`, "")
	if wrongPw.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
		t.Errorf("bad logins: got %d and %d, want 401 for both", wrongPw.Code, unknown.Code)
	}
	if wrongPw.Body.String() != unknown.Body.String() {
		t.Error("wrong-password and unknown-email responses differ — enables account probing")
	}
}

// TestRateLimit needs a reachable Redis (the docker-compose one on
// localhost:6379); skipped automatically otherwise.
func TestRateLimit(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	mw := NewMiddleware(testSecret, newFakeStore(), rdb, zap.NewNop())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Unique limiter name per run so leftover counters can't interfere.
	name := fmt.Sprintf("test-%d", time.Now().UnixNano())
	r.GET("/limited", mw.RateLimit(name, 2, time.Minute), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	codes := make([]int, 3)
	for i := range codes {
		req := httptest.NewRequest("GET", "/limited", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		codes[i] = w.Code
	}
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK || codes[2] != http.StatusTooManyRequests {
		t.Errorf("got %v, want [200 200 429]", codes)
	}
}
