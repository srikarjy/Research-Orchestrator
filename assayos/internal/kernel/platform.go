package kernel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/srikarjy/research-orchestrator/assayos/internal/api"
	gozap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Platform struct {
	Config         *Config
	Logger         *gozap.Logger
	DB             *pgxpool.Pool
	Redis          *redis.Client
	EventBus       *EventBus
	PluginMgr      *PluginManager
	Authz          *AuthzEngine
	Cache          *CacheManager
	WorkflowEngine api.WorkflowEngineService
	BiolabMCP      api.BiolabMCPService
	Aletheia       api.AletheiaGatewayService

	mu       sync.RWMutex
	services map[string]Service
	running  bool
}

type Config struct {
	Environment string `mapstructure:"ENVIRONMENT" default:"development"`
	
	Server struct {
		Host string `mapstructure:"SERVER_HOST" default:"0.0.0.0"`
		Port int    `mapstructure:"SERVER_PORT" default:"8080"`
	} `mapstructure:"server"`

	Database struct {
		DSN             string `mapstructure:"DATABASE_DSN" default:"postgres://assayos:assayos@localhost:5432/assayos?sslmode=disable"`
		MaxConns        int    `mapstructure:"DB_MAX_CONNS" default:"25"`
		MinConns        int    `mapstructure:"DB_MIN_CONNS" default:"5"`
		MaxConnLifetime time.Duration `mapstructure:"DB_MAX_CONN_LIFETIME" default:"1h"`
	} `mapstructure:"database"`

	Redis struct {
		Addr     string `mapstructure:"REDIS_ADDR" default:"localhost:6379"`
		Password string `mapstructure:"REDIS_PASSWORD" default:""`
		DB       int    `mapstructure:"REDIS_DB" default:"0"`
	} `mapstructure:"redis"`

	Auth struct {
		OPAEndpoint string `mapstructure:"OPA_ENDPOINT" default:"http://localhost:8181/v1/data/assayos/authz"`
		Enabled     bool   `mapstructure:"AUTH_ENABLED" default:"false"`
	} `mapstructure:"auth"`

	Planes struct {
		WorkflowsEnabled bool `mapstructure:"WORKFLOWS_ENABLED" default:"true"`
		BiolabEnabled    bool `mapstructure:"BIOLAB_ENABLED" default:"true"`
		AletheiaEnabled  bool `mapstructure:"ALETHEIA_ENABLED" default:"true"`
		AletheiaEndpoint string `mapstructure:"ALETHEIA_ENDPOINT" default:"http://localhost:8000"`
	} `mapstructure:"planes"`

	PluginDir string `mapstructure:"PLUGIN_DIR" default:"./plugins"`
}

type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) error
}

func NewPlatform(cfg *Config) (*Platform, error) {
	logger, err := newLogger(cfg.Environment)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	db, err := newDB(*cfg)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	redisClient, err := newRedis(*cfg)
	if err != nil {
		return nil, fmt.Errorf("redis: %w", err)
	}

	eventBus := NewEventBus(logger.Named("eventbus"))
	pluginMgr := NewPluginManager(cfg.PluginDir, logger.Named("plugins"))
	authz := NewAuthzEngine(*cfg, logger.Named("authz"))
	cache := NewCacheManager(redisClient, logger.Named("cache"))

	p := &Platform{
		Config:     cfg,
		Logger:     logger,
		DB:         db,
		Redis:      redisClient,
		EventBus:   eventBus,
		PluginMgr:  pluginMgr,
		Authz:      authz,
		Cache:      cache,
		services:   make(map[string]Service),
		running:    false,
	}

	return p, nil
}

func (p *Platform) RegisterService(svc Service) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.services[svc.Name()] = svc
	p.Logger.Info("Service registered", gozap.String("service", svc.Name()))
}

func (p *Platform) GetService(name string) (Service, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	svc, ok := p.services[name]
	return svc, ok
}

func (p *Platform) Start(ctx context.Context) error {
	if p.running {
		return fmt.Errorf("platform already running")
	}

	p.Logger.Info("Starting AssayOS platform", 
		gozap.String("environment", p.Config.Environment),
		gozap.String("version", "0.1.0"),
	)

	// Initialize database schema
	if err := p.initSchema(ctx); err != nil {
		return fmt.Errorf("schema init: %w", err)
	}

	// Start event bus
	if err := p.EventBus.Start(ctx); err != nil {
		return fmt.Errorf("event bus: %w", err)
	}

	// Load plugins
	if err := p.PluginMgr.LoadAll(ctx); err != nil {
		p.Logger.Warn("Plugin load failed", gozap.Error(err))
	}

	// Start registered services in dependency order
	order := []string{"workflow-engine", "biolab-mcp", "aletheia-gateway"}
	for _, name := range order {
		if svc, ok := p.services[name]; ok {
			p.Logger.Info("Starting service", gozap.String("service", name))
			if err := svc.Start(ctx); err != nil {
				return fmt.Errorf("start %s: %w", name, err)
			}
		}
	}

	p.running = true
	p.Logger.Info("AssayOS platform started successfully")
	return nil
}

func (p *Platform) Stop(ctx context.Context) error {
	if !p.running {
		return nil
	}

	p.Logger.Info("Stopping AssayOS platform...")

	// Stop services in reverse order
	p.mu.RLock()
	services := make([]Service, 0, len(p.services))
	for _, svc := range p.services {
		services = append(services, svc)
	}
	p.mu.RUnlock()

	for i := len(services) - 1; i >= 0; i-- {
		p.Logger.Info("Stopping service", gozap.String("service", services[i].Name()))
		if err := services[i].Stop(ctx); err != nil {
			p.Logger.Error("Service stop failed", gozap.String("service", services[i].Name()), gozap.Error(err))
		}
	}

	// Stop infrastructure
	p.EventBus.Stop(ctx)
	p.PluginMgr.UnloadAll(ctx)
	
	if p.DB != nil {
		p.DB.Close()
	}
	if p.Redis != nil {
		p.Redis.Close()
	}
	p.Logger.Sync()

	p.running = false
	p.Logger.Info("AssayOS platform stopped")
	return nil
}

func (p *Platform) Health(ctx context.Context) map[string]string {
	health := map[string]string{
		"platform": "healthy",
		"database": "unknown",
		"redis":    "unknown",
		"eventbus": "unknown",
	}

	if err := p.DB.Ping(ctx); err != nil {
		health["database"] = "unhealthy: " + err.Error()
	} else {
		health["database"] = "healthy"
	}

	if err := p.Redis.Ping(ctx).Err(); err != nil {
		health["redis"] = "unhealthy: " + err.Error()
	} else {
		health["redis"] = "healthy"
	}

	if p.EventBus.IsHealthy() {
		health["eventbus"] = "healthy"
	} else {
		health["eventbus"] = "unhealthy"
	}

	// Check registered services
	p.mu.RLock()
	for name, svc := range p.services {
		if err := svc.Health(ctx); err != nil {
			health[name] = "unhealthy: " + err.Error()
		} else {
			health[name] = "healthy"
		}
	}
	p.mu.RUnlock()

	return health
}

func (p *Platform) initSchema(ctx context.Context) error {
	schema := `
	-- Core platform tables
	CREATE TABLE IF NOT EXISTS platform_workflows (
		id UUID PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		plane TEXT NOT NULL, -- 'workflow', 'biolab', 'aletheia'
		status TEXT NOT NULL,
		input JSONB,
		output JSONB,
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW(),
		completed_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS platform_events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		workflow_id UUID REFERENCES platform_workflows(id),
		step_id TEXT,
		type TEXT NOT NULL,
		payload JSONB,
		dedup_key TEXT UNIQUE,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_events_workflow ON platform_events(workflow_id);
	CREATE INDEX IF NOT EXISTS idx_events_dedup ON platform_events(dedup_key);
	CREATE INDEX IF NOT EXISTS idx_events_created ON platform_events(created_at);

	-- Plugin registry
	CREATE TABLE IF NOT EXISTS platform_plugins (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name TEXT UNIQUE NOT NULL,
		version TEXT,
		type TEXT, -- 'tool', 'agent', 'visualizer', 'executor'
		manifest JSONB,
		enabled BOOLEAN DEFAULT true,
		loaded_at TIMESTAMPTZ
	);

	-- Audit log for compliance
	CREATE TABLE IF NOT EXISTS platform_audit (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		actor TEXT,
		action TEXT,
		resource_type TEXT,
		resource_id TEXT,
		metadata JSONB,
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_audit_resource ON platform_audit(resource_type, resource_id);
	`

	_, err := p.DB.Exec(ctx, schema)
	return err
}

func newLogger(env string) (*gozap.Logger, error) {
	var cfg gozap.Config
	if env == "production" {
		cfg = gozap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = gozap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return cfg.Build()
}

func newDB(cfg Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxConns)
	poolCfg.MinConns = int32(cfg.Database.MinConns)
	poolCfg.MaxConnLifetime = cfg.Database.MaxConnLifetime
	return pgxpool.NewWithConfig(context.Background(), poolCfg)
}

func newRedis(cfg Config) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func DefaultConfig() *Config {
	return &Config{
		Environment: "development",
		Server: struct {
			Host string `mapstructure:"SERVER_HOST" default:"0.0.0.0"`
			Port int    `mapstructure:"SERVER_PORT" default:"8080"`
		}{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: struct {
			DSN             string `mapstructure:"DATABASE_DSN" default:"postgres://assayos:assayos@localhost:5432/assayos?sslmode=disable"`
			MaxConns        int    `mapstructure:"DB_MAX_CONNS" default:"25"`
			MinConns        int    `mapstructure:"DB_MIN_CONNS" default:"5"`
			MaxConnLifetime time.Duration `mapstructure:"DB_MAX_CONN_LIFETIME" default:"1h"`
		}{
			DSN:             "postgres://assayos:assayos@localhost:5432/assayos?sslmode=disable",
			MaxConns:        25,
			MinConns:        5,
			MaxConnLifetime: time.Hour,
		},
		Redis: struct {
			Addr     string `mapstructure:"REDIS_ADDR" default:"localhost:6379"`
			Password string `mapstructure:"REDIS_PASSWORD" default:""`
			DB       int    `mapstructure:"REDIS_DB" default:"0"`
		}{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Auth: struct {
			OPAEndpoint string `mapstructure:"OPA_ENDPOINT" default:"http://localhost:8181/v1/data/assayos/authz"`
			Enabled     bool   `mapstructure:"AUTH_ENABLED" default:"false"`
		}{
			OPAEndpoint: "http://localhost:8181/v1/data/assayos/authz",
			Enabled:     false,
		},
		Planes: struct {
			WorkflowsEnabled bool   `mapstructure:"WORKFLOWS_ENABLED" default:"true"`
			BiolabEnabled    bool   `mapstructure:"BIOLAB_ENABLED" default:"true"`
			AletheiaEnabled  bool   `mapstructure:"ALETHEIA_ENABLED" default:"true"`
			AletheiaEndpoint string `mapstructure:"ALETHEIA_ENDPOINT" default:"http://localhost:8000"`
		}{
			WorkflowsEnabled: true,
			BiolabEnabled:    true,
			AletheiaEnabled:  true,
			AletheiaEndpoint: "http://localhost:8000",
		},
		PluginDir: "./plugins",
	}
}