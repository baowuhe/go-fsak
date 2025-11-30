package core

import (
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize file information",
	Long:  `Commands for synchronizing file information to database.`,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}
