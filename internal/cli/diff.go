package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ca7ai/glasswall/internal/db"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff <run-id>",
	Short: "Shows file changes for a specific run",
	Args:  cobra.ExactArgs(1),
	Run:   executeDiff,
}

func init() {
	RootCmd.AddCommand(diffCmd)
}

func executeDiff(cmd *cobra.Command, args []string) {
	runID := args[0]

	database, err := db.InitDB(DbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	record, err := database.GetRunByID(runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving run: %v\n", err)
		os.Exit(1)
	}

	if record == nil {
		fmt.Fprintf(os.Stderr, "Error: Run ID '%s' not found.\n", runID)
		os.Exit(1)
	}

	if JSONOutput {
		outBytes, _ := json.MarshalIndent(record.Changes, "", "  ")
		fmt.Println(string(outBytes))
		return
	}

	fmt.Printf("### File Mutations for Run: %s\n", record.ID)
	fmt.Printf("Command: `%s` | Driver: %s\n", record.Command, record.Driver)

	if len(record.Changes.Created) == 0 && len(record.Changes.Modified) == 0 && len(record.Changes.Deleted) == 0 {
		fmt.Println("\nNo file changes detected.")
		return
	}

	if len(record.Changes.Created) > 0 {
		fmt.Println("\n**Created:**")
		for _, file := range record.Changes.Created {
			fmt.Printf("- `+` %s\n", file)
		}
	}
	if len(record.Changes.Modified) > 0 {
		fmt.Println("\n**Modified:**")
		for _, file := range record.Changes.Modified {
			fmt.Printf("- `~` %s\n", file)
		}
	}
	if len(record.Changes.Deleted) > 0 {
		fmt.Println("\n**Deleted:**")
		for _, file := range record.Changes.Deleted {
			fmt.Printf("- `-` %s\n", file)
		}
	}
}
