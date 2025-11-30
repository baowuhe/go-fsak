package core

import (
	"fmt"

	"github.com/baowuhe/go-fsak/util"
	"github.com/spf13/cobra"
)

// hashCmd represents the hash command
var hashCmd = &cobra.Command{
	Use:   "hash [file]",
	Short: "Calculate MD5 and Blake3 hash values of a file",
	Long:  `Calculate MD5 and Blake3 hash values of a file with a single read operation`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]

		blake3Val, md5Val, err := util.FileBlake3MD5(filePath)
		if err != nil {
			fmt.Printf("[×] Error calculating hashes: %v\n", err)
			return
		}

		fmt.Printf("[√] MD5:    %s\n", md5Val)
		fmt.Printf("[√] Blake3: %s\n", blake3Val)
	},
}

func init() {
	rootCmd.AddCommand(hashCmd)
}
