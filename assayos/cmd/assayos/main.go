package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/srikarjy/research-orchestrator/assayos/internal/kernel"
	"github.com/srikarjy/research-orchestrator/assayos/internal/services"
	"github.com/srikarjy/research-orchestrator/assayos/internal/gateway"
)

var (
	Version = "0.1.0"
	Commit  = "dev"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "assayos",
		Short:   "AssayOS - Unified Platform for Agentic Biotech Research",
		Version: Version,
		RunE:    runServer,
	}

	rootCmd.Flags().String("config", "", "Config file path")
	rootCmd.Flags().String("env", "development", "Environment (development|production)")
	rootCmd.Flags().Int("port", 8080, "Server port")

	if err := viper.BindPFlags(rootCmd.Flags()); err != nil {
		panic(err)
	}

	viper.SetEnvPrefix("ASSAYOS")
	viper.AutomaticEnv()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	env := viper.GetString("env")
	
	// Load config with defaults
	cfg := kernel.DefaultConfig()
	cfg.Environment = env
	cfg.Server.Port = viper.GetInt("port")
	
	// Override from environment variables (viper with ASSAYOS_ prefix)
	cfg.Database.DSN = viper.GetString("database_dsn")
	cfg.Redis.Addr = viper.GetString("redis_addr")
	cfg.Planes.WorkflowsEnabled = viper.GetBool("workflows_enabled")
	cfg.Planes.BiolabEnabled = viper.GetBool("biolab_enabled")
	cfg.Planes.AletheiaEnabled = viper.GetBool("aletheia_enabled")
	cfg.Planes.AletheiaEndpoint = viper.GetString("aletheia_endpoint")
	
	// Override from config file if provided
	if cfgFile := viper.GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
		if err := viper.Unmarshal(cfg); err != nil {
			return err
		}
	}

	// Create platform
	platform, err := kernel.NewPlatform(cfg)
	if err != nil {
		return err
	}

	// Register plane services
	platform.WorkflowEngine = services.NewWorkflowEngineService(platform.Logger)
	platform.BiolabMCP = services.NewBiolabMCPService(platform.Logger, nil)
	platform.Aletheia = services.NewAletheiaService(platform.Logger, &services.AletheiaConfig{
		Endpoint: cfg.Planes.AletheiaEndpoint,
		Timeout:  60 * time.Second,
	})

	platform.RegisterService(platform.WorkflowEngine)
	platform.RegisterService(platform.BiolabMCP)
	platform.RegisterService(platform.Aletheia)

	// Create and start gateway
	gw := gateway.NewGateway(platform)

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		platform.Logger.Info("Shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		if err := gw.Stop(shutdownCtx); err != nil {
			platform.Logger.Error("Gateway shutdown failed", zap.Error(err))
		}
		if err := platform.Stop(shutdownCtx); err != nil {
			platform.Logger.Error("Platform shutdown failed", zap.Error(err))
		}
	}()

	// Start platform first
	ctx := context.Background()
	if err := platform.Start(ctx); err != nil {
		return err
	}

	// Start gateway (blocks)
	return gw.Start()
}