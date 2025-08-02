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
	version    = flag.Bool("version", false, "Show version information")
)

const (
	AppName    = "crucible-monitor"
	AppVersion = "1.0.0"
)

func main() {
	flag.Parse()

	// Show version and exit if requested
	if *version {
		fmt.Printf("%s version %s\n", AppName, AppVersion)
		os.Exit(0)
	}

	// Initialize logger
	logger, err := logging.NewLogger(fmt.Sprintf("/tmp/%s.log", AppName))
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Starting Crucible Monitoring Agent", "version", AppVersion)

	// Load configuration
	config, err := monitor.LoadConfig(*configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
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
	monitorAgent := agent.NewAgent(config, logger)

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
