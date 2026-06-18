package cli

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/ca7ai/glasswall/internal/db"
	"github.com/ca7ai/glasswall/internal/sandbox"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run \"<command>\"",
	Short: "Executes a command inside the sandbox and logs changes",
	Args:  cobra.ExactArgs(1),
	Run:   executeRun,
}

func init() {
	RootCmd.AddCommand(runCmd)
}

func generateRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("run-%s-%x", timestamp, b)
}

// sanitizePath removes ANSI escape sequences and control characters from file paths
func sanitizePath(path string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1 // remove
		}
		return r
	}, path)
}

func executeRun(cmd *cobra.Command, args []string) {
	cmdStr := args[0]

	// 1. Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize DB
	database, err := db.InitDB(DbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// 3. Generate run ID and setup mirror workspace
	runID := generateRunID()
	mirrorDir, err := sandbox.CreateMirror(cwd, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating sandbox workspace: %v\n", err)
		os.Exit(1)
	}
	defer sandbox.CleanupMirror(mirrorDir)

	// 4. Select sandboxing driver
	var driver sandbox.SandboxDriver
	if DriverName == "docker" {
		driver = sandbox.NewDockerSandboxDriver(DockerImage)
	} else {
		driver = sandbox.NewMacSandboxDriver()
	}

	// 5. Run command in sandbox
	opts := sandbox.RunOptions{
		Dir:     mirrorDir,
		Network: AllowNetwork,
		Timeout: 5 * time.Minute,
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()
	res, err := driver.Run(ctx, cmdStr, opts)
	endTime := time.Now()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Execution engine error: %v\n", err)
		os.Exit(1)
	}

	// 6. Compute file modifications
	changes, err := sandbox.DiffMirror(cwd, mirrorDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing workspace diff: %v\n", err)
		// We proceed to save metrics even if diff fails
	}

	// 7. Save metrics to SQLite
	caller := os.Getenv("USER")
	if caller == "" {
		caller = os.Getenv("USERNAME") // Windows fallback
	}
	if caller == "" {
		caller = "unknown"
	}

	record := &db.RunRecord{
		ID:        runID,
		Command:   cmdStr,
		Dir:       cwd,
		Driver:    DriverName,
		Caller:    caller,
		StartedAt: startTime,
		EndedAt:   endTime,
		ExitCode:  res.ExitCode,
		Stdout:    res.Stdout,
		Stderr:    res.Stderr,
		Changes:   changes,
	}

	if err := database.SaveRun(record); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to save run history: %v\n", err)
	}

	// 8. Output results based on formatting flags
	printOutput(record, res.ExitCode)
}

func printOutput(record *db.RunRecord, exitCode int) {
	duration := record.EndedAt.Sub(record.StartedAt).Round(time.Millisecond)

	if JSONOutput {
		outBytes, _ := json.MarshalIndent(record, "", "  ")
		fmt.Println(string(outBytes))
		os.Exit(exitCode)
	}

	if CompactMode {
		fmt.Printf("GlassWall Run ID: %s\n", record.ID)
		fmt.Printf("Status: ExitCode=%d, Duration=%s, Files: Created=%d, Modified=%d, Deleted=%d\n",
			exitCode, duration, len(record.Changes.Created), len(record.Changes.Modified), len(record.Changes.Deleted))
		os.Exit(exitCode)
	}

	// Default Markdown Output
	if record.Stdout != "" {
		fmt.Println("--- SANDBOX STDOUT ---")
		fmt.Print(record.Stdout)
		if !strings.HasSuffix(record.Stdout, "\n") {
			fmt.Println()
		}
	}
	if record.Stderr != "" {
		fmt.Println("--- SANDBOX STDERR ---")
		fmt.Print(record.Stderr)
		if !strings.HasSuffix(record.Stderr, "\n") {
			fmt.Println()
		}
	}

	fmt.Println("--- GLASSWALL SUMMARY ---")
	fmt.Printf("| Run ID | %s |\n", record.ID)
	fmt.Printf("| Command | `%s` |\n", record.Command)
	fmt.Printf("| Exit Code | %d |\n", exitCode)
	fmt.Printf("| Duration | %s |\n", duration)
	fmt.Printf("| Driver | %s |\n", record.Driver)

	fmt.Println("\n### File Mutations")
	if len(record.Changes.Created) == 0 && len(record.Changes.Modified) == 0 && len(record.Changes.Deleted) == 0 {
		fmt.Println("No file changes detected.")
	} else {
		if len(record.Changes.Created) > 0 {
			fmt.Println("\n**Created:**")
			for _, file := range record.Changes.Created {
				fmt.Printf("- `+` %s\n", sanitizePath(file))
			}
		}
		if len(record.Changes.Modified) > 0 {
			fmt.Println("\n**Modified:**")
			for _, file := range record.Changes.Modified {
				fmt.Printf("- `~` %s\n", sanitizePath(file))
			}
		}
		if len(record.Changes.Deleted) > 0 {
			fmt.Println("\n**Deleted:**")
			for _, file := range record.Changes.Deleted {
				fmt.Printf("- `-` %s\n", sanitizePath(file))
			}
		}
	}

	os.Exit(exitCode)
}
