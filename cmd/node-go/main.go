package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/hteppl/remnawave-node-go/internal/api"
	"github.com/hteppl/remnawave-node-go/internal/config"
	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/version"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logLevel := logger.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = logger.LevelDebug
	case "warn":
		logLevel = logger.LevelWarn
	case "error":
		logLevel = logger.LevelError
	}

	log := logger.New(logger.Config{
		Level:  logLevel,
		Format: logger.FormatJSON,
	})

	log.Info(fmt.Sprintf("Starting remnawave-node-go version %s (%s)", version.Version, version.BuildTime))

	core := xray.NewCore(log)
	configMgr := xray.NewConfigManager(log)

	server, err := api.NewServer(cfg, log, core, configMgr)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create server: %v", err))
		os.Exit(1)
	}

	if err := server.Start(); err != nil {
		log.Error(fmt.Sprintf("Failed to start server: %v", err))
		os.Exit(1)
	}

	log.Info(fmt.Sprintf("Main HTTPS server listening on :%d", cfg.NodePort))
	log.Info(fmt.Sprintf("Internal HTTP server listening on 127.0.0.1:%d", cfg.InternalRestPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down servers...")

	if core.IsRunning() {
		log.Info("Stopping xray core...")
		if err := core.Stop(); err != nil {
			log.Error(fmt.Sprintf("Failed to stop xray core: %v", err))
		}
	}

	if err := server.Stop(); err != nil {
		log.Error(fmt.Sprintf("Failed to stop server: %v", err))
	}

	log.Info("Servers stopped gracefully")
}
