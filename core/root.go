package core

import (
	"github.com/baowuhe/go-fsak/util"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:               "fsak",
	Short:             "File System Swiss Army Knife",
	Long:              `A command-line tool for enhanced file management operations.`,
	CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Print the version number of fsak.`,
	Run: func(cmd *cobra.Command, args []string) {
		util.PrintSuccess("fsak v0.1.0")
	},
}
