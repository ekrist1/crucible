package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"crucible/internal/logging"
	"crucible/internal/monitor"
	"crucible/internal/monitor/agent"
)

var (
	configPath = flag.String("config", "", "Path to configuration file")
	debug      = flag.Bool("debug", false, "Enable debug logging")
	versionFlag = flag.Bool("version", false, "Show version information")
)

// Build information - can be set via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

const (
	AppName = "crucible-monitor"
)

func main() {
	flag.Parse()

	// Show version and exit if requested
	if *versionFlag {
		fmt.Printf("%s version %s\n", AppName, version)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Git commit: %s\n", gitCommit)
		os.Exit(0)
	}

	// Initialize temporary logger (fallback to temp file initially)
	tempLogPath := fmt.Sprintf("/tmp/%s.log", AppName)
	logger, err := logging.NewLogger(tempLogPath)
	if err != nil {
		fmt.Printf("Failed to initialize temporary logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting Crucible Monitoring Agent", "version", version, "build_time", buildTime, "git_commit", gitCommit)

	// Load configuration
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Reinitialize logger with configured path if different from temp
	if config.Agent.LogFile != "" && config.Agent.LogFile != tempLogPath {
		logger.Info("Reinitializing logger with configured path", "old_path", tempLogPath, "new_path", config.Agent.LogFile)
		if err := logger.ReinitializeWithPath(config.Agent.LogFile); err != nil {
			logger.Warn("Failed to reinitialize logger with configured path, continuing with temporary path",
				"error", err, "temp_path", tempLogPath, "config_path", config.Agent.LogFile)
		}
	}

	// Override debug setting from command line
	if *debug {
		config.Agent.Debug = true
	}

	logger.Info("Configuration loaded successfully",
		"listen_addr", config.Agent.ListenAddr,
		"data_retention", config.Agent.DataRetention,
		"collect_interval", config.Agent.CollectInterval,
	)

	// Create monitoring agent
	monitorAgent, err := agent.NewAgent(config, logger)
	if err != nil {
		logger.Error("Failed to create monitoring agent", "error", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the agent in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := monitorAgent.Start(); err != nil {
			errChan <- err
		}
	}()

	logger.Info("Monitoring agent started successfully")

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig.String())
	case err := <-errChan:
		logger.Error("Agent startup failed", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	logger.Info("Shutting down monitoring agent...")
	if err := monitorAgent.Stop(); err != nil {
		logger.Error("Error during shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Monitoring agent stopped successfully")
}
