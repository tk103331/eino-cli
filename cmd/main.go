package cmd

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tk103331/eino-cli/logger"
)

// Main is the shared entry point for the `eino-cli` and `eino` binaries.
// It initializes logging, handles signals and executes the root command.
func Main() {
	// The command name follows the binary name, so `eino --help` shows `eino`.
	// Keep the optional positional working directory in the usage line.
	RootCmd.Use = filepath.Base(os.Args[0]) + " [directory]"

	// Initialize logging
	if err := logger.Init(); err != nil {
		// If logging fails, still run the app but log to stderr
		os.Stderr.WriteString("Warning: Failed to initialize logging: " + err.Error() + "\n")
	}

	// Ensure logger is closed on exit
	defer logger.Close()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("MAIN", "Received signal, shutting down: "+sig.String())
		logger.Close()
		os.Exit(0)
	}()

	logger.Info("MAIN", "Eino CLI starting")

	// Execute the main command
	Execute()

	logger.Info("MAIN", "Eino CLI finished")
}
