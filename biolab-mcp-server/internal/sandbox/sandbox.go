package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/shared"
	"go.uber.org/zap"
)

type SandboxConfig struct {
	BasePath       string            `json:"base_path"`
	MaxConcurrent  int               `json:"max_concurrent"`
	DefaultTimeout time.Duration     `json:"default_timeout"`
	ResourceLimits ResourceLimits    `json:"resource_limits"`
	Env            map[string]string `json:"env"`
}

type ResourceLimits struct {
	MaxCPUPercent  float64 `json:"max_cpu_percent"`
	MaxMemoryMB    int64   `json:"max_memory_mb"`
	MaxDiskMB      int64   `json:"max_disk_mb"`
	MaxProcesses   int     `json:"max_processes"`
	NetworkEnabled bool    `json:"network_enabled"`
}

type Sandbox struct {
	config   SandboxConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	sessions map[string]*Session
	sem      chan struct{}
}

type Session struct {
	ID          string                 `json:"id"`
	ExperimentID string                `json:"experiment_id"`
	WorkDir     string                 `json:"work_dir"`
	Status      SessionStatus          `json:"status"`
	Process     *exec.Cmd              `json:"-"`
	Stdout      *os.File               `json:"-"`
	Stderr      *os.File               `json:"-"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	ExitCode    int                    `json:"exit_code"`
	Resources   ResourceUsage          `json:"resources"`
	Artifacts   []Artifact             `json:"artifacts"`
	Logs        []LogEntry             `json:"logs"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type SessionStatus string

const (
	SessionStatusPending   SessionStatus = "pending"
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

type ResourceUsage struct {
	CPUTime    time.Duration `json:"cpu_time"`
	MaxMemory  int64         `json:"max_memory_bytes"`
	DiskUsed   int64         `json:"disk_used_bytes"`
	NetworkRX  int64         `json:"network_rx_bytes"`
	NetworkTX  int64         `json:"network_tx_bytes"`
}

type Artifact struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
}

func NewSandbox(config SandboxConfig, logger *zap.Logger) *Sandbox {
	if config.BasePath == "" {
		config.BasePath = "/tmp/research-sandbox"
	}
	if config.MaxConcurrent == 0 {
		config.MaxConcurrent = 10
	}
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Minute
	}
	if config.ResourceLimits.MaxMemoryMB == 0 {
		config.ResourceLimits.MaxMemoryMB = 4096
	}
	if config.ResourceLimits.MaxDiskMB == 0 {
		config.ResourceLimits.MaxDiskMB = 10240
	}

	os.MkdirAll(config.BasePath, 0755)

	return &Sandbox{
		config:   config,
		logger:   logger,
		sessions: make(map[string]*Session),
		sem:      make(chan struct{}, config.MaxConcurrent),
	}
}

func (s *Sandbox) CreateSession(ctx context.Context, experimentID string, metadata map[string]interface{}) (*Session, error) {
	sessionID := uuid.New().String()
	workDir := filepath.Join(s.config.BasePath, sessionID)
	
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}

	session := &Session{
		ID:           sessionID,
		ExperimentID: experimentID,
		WorkDir:      workDir,
		Status:       SessionStatusPending,
		StartTime:    time.Now(),
		Metadata:     metadata,
		Logs:         []LogEntry{},
		Artifacts:    []Artifact{},
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	s.log(sessionID, "info", "sandbox", "Session created")
	return session, nil
}

func (s *Sandbox) RunExperiment(ctx context.Context, sessionID string, spec shared.ExperimentSpec) (*Session, error) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	select {
	case s.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.sem }()

	session.Status = SessionStatusRunning
	s.log(sessionID, "info", "sandbox", "Experiment started: type=%s", spec.Type)

	timeout := spec.Timeout
	if timeout == 0 {
		timeout = s.config.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.Command[0], spec.Command[1:]...)
	cmd.Dir = session.WorkDir
	cmd.Env = append(os.Environ(), s.buildEnv(spec.Env)...)
	
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	session.Process = cmd

	if err := cmd.Start(); err != nil {
		session.Status = SessionStatusFailed
		session.ExitCode = -1
		now := time.Now()
		session.EndTime = &now
		s.log(sessionID, "error", "sandbox", "Failed to start: error=%v", err)
		return session, err
	}

	go s.captureOutput(sessionID, stdoutPipe, "stdout")
	go s.captureOutput(sessionID, stderrPipe, "stderr")

	err := cmd.Wait()
	
	now := time.Now()
	session.EndTime = &now
	session.ExitCode = cmd.ProcessState.ExitCode()
	
	if err != nil {
		session.Status = SessionStatusFailed
		s.log(sessionID, "error", "sandbox", "Experiment failed: error=%v", err)
	} else {
		session.Status = SessionStatusCompleted
		s.log(sessionID, "info", "sandbox", "Experiment completed")
	}

	s.collectArtifacts(session)
	s.collectResourceUsage(session)

	return session, err
}

func (s *Sandbox) captureOutput(sessionID string, pipe io.ReadCloser, source string) {
	buf := make([]byte, 4096)
	for {
		n, err := pipe.Read(buf)
		if n > 0 {
			msg := string(buf[:n])
			s.log(sessionID, "info", source, msg)
		}
		if err != nil {
			break
		}
	}
}

func (s *Sandbox) log(sessionID, level, source, message string, args ...interface{}) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !ok {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	s.mu.Lock()
	session.Logs = append(session.Logs, entry)
	s.mu.Unlock()
}

func (s *Sandbox) collectArtifacts(session *Session) {
	filepath.Walk(session.WorkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		
		relPath, _ := filepath.Rel(session.WorkDir, path)
		session.Artifacts = append(session.Artifacts, Artifact{
			Name:     relPath,
			Path:     path,
			Size:     info.Size(),
			MimeType: detectMimeType(relPath),
		})
		return nil
	})
}

func (s *Sandbox) collectResourceUsage(session *Session) {
	if session.Process != nil && session.Process.ProcessState != nil {
		session.Resources.CPUTime = session.Process.ProcessState.UserTime() + session.Process.ProcessState.SystemTime()
	}
}

func (s *Sandbox) buildEnv(customEnv map[string]string) []string {
	env := make([]string, 0, len(s.config.Env)+len(customEnv))
	for k, v := range s.config.Env {
		env = append(env, k+"="+v)
	}
	for k, v := range customEnv {
		env = append(env, k+"="+v)
	}
	return env
}

func (s *Sandbox) GetSession(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *Sandbox) ListSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, s := range s.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (s *Sandbox) CancelSession(sessionID string) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("session not found")
	}
	
	if session.Process != nil && session.Process.Process != nil {
		session.Process.Process.Kill()
		session.Status = SessionStatusCancelled
		now := time.Now()
		session.EndTime = &now
	}
	
	return nil
}

func (s *Sandbox) CleanupSession(sessionID string) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("session not found")
	}
	
	os.RemoveAll(session.WorkDir)
	
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
	
	return nil
}

func detectMimeType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".json":
		return "application/json"
	case ".csv", ".tsv":
		return "text/csv"
	case ".pdb", ".sdf", ".mol":
		return "chemical/x-pdb"
	case ".png", ".jpg", ".jpeg":
		return "image/" + ext[1:]
	case ".pdf":
		return "application/pdf"
	case ".txt", ".log":
		return "text/plain"
	case ".py":
		return "text/x-python"
	case ".sh":
		return "application/x-shellscript"
	default:
		return "application/octet-stream"
	}
}