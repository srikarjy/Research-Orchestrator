package auth

import (
	"errors"
	"strings"
	"sync"
	"time"
)

type MemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*User
	emails map[string]string
}

func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users:  make(map[string]*User),
		emails: make(map[string]string),
	}
}

func (s *MemoryUserStore) GetUserByID(userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MemoryUserStore) GetUserByEmail(email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID, ok := s.emails[strings.ToLower(email)]
	if !ok {
		return nil, ErrUserNotFound
	}
	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (s *MemoryUserStore) GetPasswordHash(userID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[userID]
	if !ok {
		return "", ErrUserNotFound
	}
	return user.PasswordHash, nil
}

func (s *MemoryUserStore) CreateUser(user *User, passwordHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailKey := strings.ToLower(user.Email)
	if _, ok := s.emails[emailKey]; ok {
		return ErrUserExists
	}

	if user.ID == "" {
		user.ID = generateTokenID()
	}

	user.PasswordHash = passwordHash
	s.users[user.ID] = user
	s.emails[emailKey] = user.ID
	return nil
}

func (s *MemoryUserStore) UpdateUser(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[user.ID]; !ok {
		return ErrUserNotFound
	}

	s.users[user.ID] = user
	return nil
}

type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	userSessions map[string][]string
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions:     make(map[string]*Session),
		userSessions: make(map[string][]string),
	}
}

func (s *MemorySessionStore) GetSession(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (s *MemorySessionStore) SaveSession(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	s.userSessions[session.UserID] = append(s.userSessions[session.UserID], session.ID)
	return nil
}

func (s *MemorySessionStore) DeleteSession(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return errors.New("session not found")
	}
	delete(s.sessions, sessionID)
	
	sessions := s.userSessions[session.UserID]
	for i, id := range sessions {
		if id == sessionID {
			s.userSessions[session.UserID] = append(sessions[:i], sessions[i+1:]...)
			break
		}
	}
	return nil
}

func (s *MemorySessionStore) DeleteUserSessions(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionIDs := s.userSessions[userID]
	for _, id := range sessionIDs {
		delete(s.sessions, id)
	}
	delete(s.userSessions, userID)
	return nil
}

func (s *MemorySessionStore) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, session := range s.sessions {
		if !session.IsValid() || now.After(session.ExpiresAt) {
			delete(s.sessions, id)
			sessions := s.userSessions[session.UserID]
			for i, sid := range sessions {
				if sid == id {
					s.userSessions[session.UserID] = append(sessions[:i], sessions[i+1:]...)
					break
				}
			}
		}
	}
}