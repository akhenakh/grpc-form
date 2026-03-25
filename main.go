package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jhump/protoreflect/grpcreflect"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

const appName = "admin-form-gateway"

//go:embed index.html
var indexHTML []byte

//go:embed validate.css
var validateCSS []byte

// EnvConfig holds server-level configuration from environment variables.
type EnvConfig struct {
	LogLevel     string `env:"LOG_LEVEL" envDefault:"INFO"`
	HTTPPort     int    `env:"HTTP_PORT" envDefault:"8080"`
	ConfigPath   string `env:"CONFIG_PATH" envDefault:"config.yaml"`
	AllowDevMode bool   `env:"ALLOW_DEV_MODE" envDefault:"false"`
}

// YAMLConfig holds the dynamic routing and role rules for the gateway.
type YAMLConfig struct {
	Services []ServiceConfig `yaml:"services"`
}

type ServiceConfig struct {
	ID      string   `yaml:"id"`
	Name    string   `yaml:"name"`
	Target  string   `yaml:"target"`
	Methods []string `yaml:"methods"`
	Roles   []string `yaml:"roles"`
}

// Gateway encapsulates the state of our API Gateway.
type Gateway struct {
	services []ServiceConfig
	clients  map[string]*grpcreflect.Client
	channels map[string]*grpc.ClientConn
	logger   *slog.Logger
	devMode  bool
}

func main() {
	var envCfg EnvConfig
	if err := env.Parse(&envCfg); err != nil {
		fmt.Printf("failed to parse env config: %+v\n", err)
		os.Exit(1)
	}

	logger := createLogger(envCfg)
	slog.SetDefault(logger)

	// Read YAML routes config
	yamlData, err := os.ReadFile(envCfg.ConfigPath)
	if err != nil {
		logger.Error("failed to read yaml config", "path", envCfg.ConfigPath, "error", err)
		os.Exit(1)
	}
	var yamlCfg YAMLConfig
	if err := yaml.Unmarshal(yamlData, &yamlCfg); err != nil {
		logger.Error("failed to parse yaml config", "error", err)
		os.Exit(1)
	}

	// Setup Context and Signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(interrupt)

	g, gCtx := errgroup.WithContext(ctx)

	// Initialize Gateway state & gRPC clients
	gw, err := NewGateway(ctx, yamlCfg.Services, logger, envCfg.AllowDevMode)
	if err != nil {
		logger.Error("failed to initialize gateway", "error", err)
		os.Exit(1)
	}
	defer gw.Close()

	// Start HTTP Server
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", envCfg.HTTPPort),
		Handler: gw.router(),
	}

	g.Go(func() error {
		logger.Info("Admin Form Gateway listening", "port", envCfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server failed: %w", err)
		}
		return nil
	})

	// Wait for termination signal or an error from the server
	select {
	case <-interrupt:
		logger.Warn("received termination signal, starting graceful shutdown")
		cancel()
	case <-gCtx.Done():
		logger.Warn("context cancelled, starting graceful shutdown")
	}

	// Graceful Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Wait for all services in the errgroup to finish
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("server group returned an error", "error", err)
		os.Exit(2)
	}
}

func createLogger(cfg EnvConfig) *slog.Logger {
	var programLevel slog.Level
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		programLevel = slog.LevelDebug
	case "INFO":
		programLevel = slog.LevelInfo
	case "WARN":
		programLevel = slog.LevelWarn
	case "ERROR":
		programLevel = slog.LevelError
	default:
		programLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     programLevel,
		AddSource: programLevel <= slog.LevelDebug,
	}).WithAttrs([]slog.Attr{slog.String("app", appName)})

	return slog.New(handler)
}
