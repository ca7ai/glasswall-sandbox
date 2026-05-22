package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ca7ai/glasswall/internal/db"
	"github.com/spf13/cobra"
)

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "Lists historical sandboxed runs",
	Args:  cobra.NoArgs,
	Run:   executeRuns,
}

func init() {
	RootCmd.AddCommand(runsCmd)
}

func executeRuns(cmd *cobra.Command, args []string) {
	database, err := db.InitDB(DbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	records, err := database.GetRuns()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving runs: %v\n", err)
		os.Exit(1)
	}

	if len(records) == 0 {
		if JSONOutput {
			fmt.Println("[]")
		} else {
			fmt.Println("No runs recorded yet.")
		}
		return
	}

	if JSONOutput {
		outBytes, _ := json.MarshalIndent(records, "", "  ")
		fmt.Println(string(outBytes))
		return
	}

	if CompactMode {
		for _, record := range records {
			duration := record.EndedAt.Sub(record.StartedAt).Round(time.Millisecond)
			fmt.Printf("%s | Cmd: %s | Exit: %d | Duration: %s | Driver: %s\n",
				record.ID, record.Command, record.ExitCode, duration, record.Driver)
		}
		return
	}

	// Default Markdown Output
	fmt.Printf("| Run ID | Started At | Driver | Command | Exit Code | Duration | Changes |\n")
	fmt.Printf("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, record := range records {
		duration := record.EndedAt.Sub(record.StartedAt).Round(time.Millisecond)
		timeStr := record.StartedAt.Format("2006-01-02 15:04:05")
		totalChanges := len(record.Changes.Created) + len(record.Changes.Modified) + len(record.Changes.Deleted)
		
		// Truncate command if too long
		cmdDisplay := record.Command
		if len(cmdDisplay) > 30 {
			cmdDisplay = cmdDisplay[:27] + "..."
		}
		
		fmt.Printf("| %s | %s | %s | `%s` | %d | %s | %+d |\n",
			record.ID, timeStr, record.Driver, cmdDisplay, record.ExitCode, duration, totalChanges)
	}
}
