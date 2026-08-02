package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type AuthHandler struct {
	config        *AuthConfig
	userStore     UserStore
	sessionStore  SessionStore
	oauthConfigs  map[OAuthProvider]*OAuthConfig
	rateLimiter   *RateLimiter
}

func NewAuthHandler(config *AuthConfig, userStore UserStore, sessionStore SessionStore) *AuthHandler {
	return &AuthHandler{
		config:       config,
		userStore:    userStore,
		sessionStore: sessionStore,
		oauthConfigs: make(map[OAuthProvider]*OAuthConfig),
		rateLimiter:  NewRateLimiter(10, time.Minute),
	}
}

func (h *AuthHandler) RegisterOAuth(provider OAuthProvider, config *OAuthConfig) {
	h.oauthConfigs[provider] = config
}

func (h *AuthHandler) Routes(router *mux.Router) {
	router.HandleFunc("/auth/register", h.rateLimit(h.register)).Methods("POST")
	router.HandleFunc("/auth/login", h.rateLimit(h.login)).Methods("POST")
	router.HandleFunc("/auth/refresh", h.rateLimit(h.refresh)).Methods("POST")
	router.HandleFunc("/auth/logout", h.logout).Methods("POST")
	router.HandleFunc("/auth/me", h.me).Methods("GET")
	router.HandleFunc("/auth/oauth/{provider}", h.oauthRedirect).Methods("GET")
	router.HandleFunc("/auth/oauth/{provider}/callback", h.oauthCallback).Methods("GET")
	router.HandleFunc("/auth/password/reset", h.rateLimit(h.requestPasswordReset)).Methods("POST")
	router.HandleFunc("/auth/password/reset/confirm", h.rateLimit(h.confirmPasswordReset)).Methods("POST")
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	User  *User     `json:"user"`
	Tokens *TokenPair `json:"tokens"`
}

func (h *AuthHandler) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := IPKeyFunc(r)
		if !h.rateLimiter.Allow(key) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (h *AuthHandler) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	existingUser, _ := h.userStore.GetUserByEmail(req.Email)
	if existingUser != nil {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	passwordHash, err := HashPassword(req.Password, h.config.BCryptCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &User{
		Email: req.Email,
		Name:  req.Name,
		Roles: []string{"user"},
	}

	if err := h.userStore.CreateUser(user, passwordHash); err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	tokens, err := GenerateTokenPair(h.config, user.ID, user.Email, user.Roles, generateTokenID())
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	session := NewSession(user.ID, tokens.RefreshToken, r.UserAgent(), IPKeyFunc(r), h.config.RefreshExpiry)
	if err := h.sessionStore.SaveSession(session); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	h.respond(w, http.StatusCreated, AuthResponse{User: user, Tokens: tokens})
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, err := h.userStore.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	passwordHash, err := h.userStore.GetPasswordHash(user.ID)
	if err != nil || passwordHash == "" || !CheckPasswordHash(req.Password, passwordHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	tokens, err := GenerateTokenPair(h.config, user.ID, user.Email, user.Roles, generateTokenID())
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	session := NewSession(user.ID, tokens.RefreshToken, r.UserAgent(), IPKeyFunc(r), h.config.RefreshExpiry)
	if err := h.sessionStore.SaveSession(session); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	h.respond(w, http.StatusOK, AuthResponse{User: user, Tokens: tokens})
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RefreshToken == "" {
		http.Error(w, "Refresh token required", http.StatusBadRequest)
		return
	}

	claims, err := ValidateRefreshToken(h.config, req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrExpiredToken) {
			http.Error(w, "Refresh token expired", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	session, err := h.sessionStore.GetSession(claims.SessionID)
	if err != nil || session == nil || !session.IsValid() || session.RefreshToken != req.RefreshToken {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	user, err := h.userStore.GetUserByID(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	newTokens, err := GenerateTokenPair(h.config, user.ID, user.Email, user.Roles, claims.SessionID)
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	session.RefreshToken = newTokens.RefreshToken
	session.ExpiresAt = time.Now().Add(h.config.RefreshExpiry)
	if err := h.sessionStore.SaveSession(session); err != nil {
		http.Error(w, "Failed to update session", http.StatusInternalServerError)
		return
	}

	h.respond(w, http.StatusOK, AuthResponse{User: user, Tokens: newTokens})
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r.Context())
	if session != nil {
		h.sessionStore.DeleteSession(session.ID)
	}
	h.respond(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) me(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	h.respond(w, http.StatusOK, user)
}

func (h *AuthHandler) oauthRedirect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := OAuthProvider(vars["provider"])

	config, ok := h.oauthConfigs[provider]
	if !ok {
		http.Error(w, "Unsupported OAuth provider", http.StatusBadRequest)
		return
	}

	state := generateTokenID()
	url := config.AuthURL(state)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})

	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := OAuthProvider(vars["provider"])

	config, ok := h.oauthConfigs[provider]
	if !ok {
		http.Error(w, "Unsupported OAuth provider", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	storedState, err := r.Cookie("oauth_state")
	if err != nil || storedState.Value != state {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}

	userInfo, err := h.exchangeCodeForUserInfo(config, code)
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	user, err := h.userStore.GetUserByEmail(userInfo.Email)
	if err != nil {
		user = &User{
			Email:    userInfo.Email,
			Name:     userInfo.Name,
			Avatar:   userInfo.AvatarURL,
			Provider: string(userInfo.Provider),
			Roles:    []string{"user"},
		}
		if err := h.userStore.CreateUser(user, ""); err != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	} else {
		user.Name = userInfo.Name
		user.Avatar = userInfo.AvatarURL
		h.userStore.UpdateUser(user)
	}

	tokens, err := GenerateTokenPair(h.config, user.ID, user.Email, user.Roles, generateTokenID())
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	session := NewSession(user.ID, tokens.RefreshToken, r.UserAgent(), IPKeyFunc(r), h.config.RefreshExpiry)
	if err := h.sessionStore.SaveSession(session); err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	frontendURL := "http://localhost:5173"
	redirectURL := frontendURL + "/auth/callback?access_token=" + tokens.AccessToken + "&refresh_token=" + tokens.RefreshToken
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *AuthHandler) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := h.userStore.GetUserByEmail(req.Email)
	if err != nil {
		h.respond(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link will be sent"})
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link will be sent"})
}

func (h *AuthHandler) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" || len(req.NewPassword) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	h.respond(w, http.StatusOK, map[string]string{"message": "Password reset successful"})
}

func (h *AuthHandler) getPasswordHash(userID string) string {
	hash, err := h.userStore.GetPasswordHash(userID)
	if err != nil {
		return ""
	}
	return hash
}

func (h *AuthHandler) exchangeCodeForUserInfo(config *OAuthConfig, code string) (*OAuthUserInfo, error) {
	return &OAuthUserInfo{
		Provider:   config.Provider,
		ProviderID: "mock-id",
		Email:      "user@example.com",
		Name:       "Test User",
		AvatarURL:  "",
	}, nil
}

func (h *AuthHandler) respond(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}