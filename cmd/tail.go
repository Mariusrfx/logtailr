package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var (
	filePath string
	level    string
	regex    string
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Tail and parse logs from a source",
	Long:  `Tail logs from a specific file with real-time parsing and filtering capabilities.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if filePath == "" {
			return fmt.Errorf("the --file flag is required")
		}

		fmt.Printf("Starting tail on file: %s | Level: %s | Regex: %s\n", filePath, level, regex)

		// Logic orchestration will go here
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tailCmd)

	// Local flags for the tail command
	tailCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to the log file to process")
	tailCmd.Flags().StringVarP(&level, "level", "l", "info", "Minimum log level (debug, info, warn, error)")
	tailCmd.Flags().StringVarP(&regex, "regex", "r", "", "Optional regex pattern to filter messages")
}
