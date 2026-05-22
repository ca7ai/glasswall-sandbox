package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	DbPath      string
	CompactMode bool
	JSONOutput  bool
	DriverName  string
	DockerImage string
	AllowNetwork bool
)

var RootCmd = &cobra.Command{
	Use:   "glasswall",
	Short: "GlassWall Sandbox is an agentic command-line execution sandbox",
	Long: `GlassWall Sandbox runs untrusted terminal commands inside isolated environments
(using macOS sandbox-exec or Docker containers), tracks file mutations, and logs
all metrics in a local SQLite database for AI agent consumption.`,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVar(&DbPath, "db-path", "", "Custom SQLite database file path (defaults to ~/.glasswall/runs.db)")
	RootCmd.PersistentFlags().BoolVar(&CompactMode, "compact", false, "Enable compact mode (returns minimal token-friendly outputs)")
	RootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Format stdout output as JSON")
	RootCmd.PersistentFlags().StringVar(&DriverName, "driver", "mac", "Sandboxing driver engine: 'mac' (sandbox-exec) or 'docker'")
	RootCmd.PersistentFlags().StringVar(&DockerImage, "image", "alpine:latest", "Docker image to run command inside (only used if --driver=docker)")
	RootCmd.PersistentFlags().BoolVar(&AllowNetwork, "network", false, "Enable outbound network access inside sandbox")
}
